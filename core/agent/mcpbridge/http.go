package mcpbridge

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
)

type HTTPProvider struct {
	endpoint      string
	authorization string
	client        *http.Client

	mu          sync.Mutex
	nextID      int64
	initialized bool
	sessionID   string
}

func NewHTTP(endpoint, authorization string) *HTTPProvider {
	return &HTTPProvider{
		endpoint:      strings.TrimSpace(endpoint),
		authorization: strings.TrimSpace(authorization),
		client:        &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *HTTPProvider) ListTools(ctx context.Context, _ agentsdk.Session) ([]agentsdk.Tool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.ensureInitializedLocked(ctx); err != nil {
		return nil, err
	}
	raw, err := p.requestLocked(ctx, "tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var result struct {
		Tools []json.RawMessage `json:"tools"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decode remote MCP tools/list: %w", err)
	}
	tools := make([]agentsdk.Tool, 0, len(result.Tools))
	for _, definition := range result.Tools {
		var value struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(definition, &value); err != nil {
			return nil, fmt.Errorf("decode remote MCP tool: %w", err)
		}
		tools = append(tools, agentsdk.Tool{
			Name:       value.Name,
			Definition: append(json.RawMessage(nil), definition...),
		})
	}
	return tools, nil
}

func (p *HTTPProvider) CallTool(ctx context.Context, _ agentsdk.Session, name string, args json.RawMessage) (json.RawMessage, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.ensureInitializedLocked(ctx); err != nil {
		return nil, err
	}
	var arguments any = map[string]any{}
	if len(args) != 0 {
		if err := json.Unmarshal(args, &arguments); err != nil {
			return nil, fmt.Errorf("decode remote MCP tool arguments: %w", err)
		}
	}
	return p.requestLocked(ctx, "tools/call", map[string]any{
		"name":      name,
		"arguments": arguments,
	})
}

func (p *HTTPProvider) ensureInitializedLocked(ctx context.Context) error {
	if p.initialized {
		return nil
	}
	if p.endpoint == "" {
		return fmt.Errorf("remote MCP endpoint is required")
	}
	if _, err := p.requestLocked(ctx, "initialize", map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "devtool-remote-mcp",
			"version": "1",
		},
	}); err != nil {
		return fmt.Errorf("initialize remote MCP: %w", err)
	}
	if err := p.notifyLocked(ctx, "notifications/initialized", map[string]any{}); err != nil {
		return fmt.Errorf("notify remote MCP initialized: %w", err)
	}
	p.initialized = true
	return nil
}

func (p *HTTPProvider) requestLocked(ctx context.Context, method string, params any) (json.RawMessage, error) {
	p.nextID++
	id := p.nextID
	payload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return nil, err
	}
	raw, err := p.postLocked(ctx, payload, true)
	if err != nil {
		return nil, err
	}

	var envelope struct {
		ID     json.RawMessage `json:"id,omitempty"`
		Result json.RawMessage `json:"result,omitempty"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decode remote MCP response: %w", err)
	}
	if envelope.Error != nil {
		return nil, fmt.Errorf("remote MCP %s error %d: %s", method, envelope.Error.Code, envelope.Error.Message)
	}
	if len(envelope.Result) == 0 {
		return json.RawMessage(`null`), nil
	}
	return append(json.RawMessage(nil), envelope.Result...), nil
}

func (p *HTTPProvider) notifyLocked(ctx context.Context, method string, params any) error {
	payload, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return err
	}
	_, err = p.postLocked(ctx, payload, false)
	return err
}

func (p *HTTPProvider) postLocked(ctx context.Context, payload []byte, expectResponse bool) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if p.authorization != "" {
		req.Header.Set("Authorization", p.authorization)
	}
	if p.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", p.sessionID)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("remote MCP POST: %w", err)
	}
	defer resp.Body.Close()

	if sessionID := strings.TrimSpace(resp.Header.Get("Mcp-Session-Id")); sessionID != "" {
		p.sessionID = sessionID
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		return nil, fmt.Errorf("remote MCP HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if !expectResponse || resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		return nil, err
	}
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		return extractSSEData(body)
	}
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return nil, fmt.Errorf("remote MCP returned empty response")
	}
	return body, nil
}

func extractSSEData(body []byte) ([]byte, error) {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		raw := []byte(data)
		if json.Valid(raw) {
			return raw, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("remote MCP SSE response contained no JSON data")
}
