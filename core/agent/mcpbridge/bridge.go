package mcpbridge

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"sync"

	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
)

type CommandFactory func(context.Context, agentsdk.Session) (*exec.Cmd, error)

type Provider struct {
	factory CommandFactory

	mu        sync.Mutex
	client    *client
	sessionID string
}

func New(factory CommandFactory) *Provider {
	return &Provider{factory: factory}
}

func (p *Provider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client == nil {
		return nil
	}
	err := p.client.close()
	p.client = nil
	p.sessionID = ""
	return err
}

func (p *Provider) ListTools(ctx context.Context, session agentsdk.Session) ([]agentsdk.Tool, error) {
	c, err := p.ensureClient(ctx, session)
	if err != nil {
		return nil, err
	}
	raw, err := c.request("tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var result struct {
		Tools []json.RawMessage `json:"tools"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decode child MCP tools/list: %w", err)
	}
	tools := make([]agentsdk.Tool, 0, len(result.Tools))
	for _, definition := range result.Tools {
		var value struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(definition, &value); err != nil {
			return nil, fmt.Errorf("decode child MCP tool: %w", err)
		}
		tools = append(tools, agentsdk.Tool{
			Name:       value.Name,
			Definition: append(json.RawMessage(nil), definition...),
		})
	}
	return tools, nil
}

func (p *Provider) CallTool(ctx context.Context, session agentsdk.Session, name string, args json.RawMessage) (json.RawMessage, error) {
	c, err := p.ensureClient(ctx, session)
	if err != nil {
		return nil, err
	}
	var arguments any = map[string]any{}
	if len(args) != 0 {
		if err := json.Unmarshal(args, &arguments); err != nil {
			return nil, fmt.Errorf("decode tool arguments: %w", err)
		}
	}
	return c.request("tools/call", map[string]any{
		"name":      name,
		"arguments": arguments,
	})
}

func (p *Provider) ensureClient(ctx context.Context, session agentsdk.Session) (*client, error) {
	keyBytes, _ := json.Marshal(session)
	key := string(keyBytes)

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client != nil && p.sessionID == key {
		return p.client, nil
	}
	if p.client != nil {
		_ = p.client.close()
		p.client = nil
	}
	cmd, err := p.factory(ctx, session)
	if err != nil {
		return nil, err
	}
	c, err := startClient(cmd)
	if err != nil {
		return nil, err
	}
	if err := c.initialize(); err != nil {
		_ = c.close()
		return nil, err
	}
	p.client = c
	p.sessionID = key
	return c, nil
}

type client struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	scanner *bufio.Scanner
	encoder *json.Encoder

	mu     sync.Mutex
	nextID int64
}

func startClient(cmd *exec.Cmd) (*client, error) {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open child MCP stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("open child MCP stdout: %w", err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("start child MCP: %w", err)
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	return &client{
		cmd:     cmd,
		stdin:   stdin,
		scanner: scanner,
		encoder: json.NewEncoder(stdin),
	}, nil
}

func (c *client) initialize() error {
	if _, err := c.request("initialize", map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "devtool-agent-bridge",
			"version": "1",
		},
	}); err != nil {
		return fmt.Errorf("initialize child MCP: %w", err)
	}
	return c.notify("notifications/initialized", map[string]any{})
}

func (c *client) request(method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.nextID++
	id := c.nextID
	if err := c.encoder.Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}); err != nil {
		return nil, fmt.Errorf("write child MCP request: %w", err)
	}

	for c.scanner.Scan() {
		line := append([]byte(nil), c.scanner.Bytes()...)
		var envelope struct {
			ID     json.RawMessage `json:"id,omitempty"`
			Method string          `json:"method,omitempty"`
			Result json.RawMessage `json:"result,omitempty"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error,omitempty"`
		}
		if err := json.Unmarshal(line, &envelope); err != nil {
			continue
		}
		if envelope.Method != "" {
			continue
		}
		var responseID int64
		if len(envelope.ID) == 0 || json.Unmarshal(envelope.ID, &responseID) != nil || responseID != id {
			continue
		}
		if envelope.Error != nil {
			return nil, fmt.Errorf("child MCP %s error %d: %s", method, envelope.Error.Code, envelope.Error.Message)
		}
		if len(envelope.Result) == 0 {
			return json.RawMessage(`null`), nil
		}
		return append(json.RawMessage(nil), envelope.Result...), nil
	}
	if err := c.scanner.Err(); err != nil {
		return nil, fmt.Errorf("read child MCP response: %w", err)
	}
	return nil, fmt.Errorf("child MCP exited while waiting for %s", method)
}

func (c *client) notify(method string, params any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.encoder.Encode(map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	})
}

func (c *client) close() error {
	_ = c.stdin.Close()
	if c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	err := c.cmd.Wait()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return nil
		}
	}
	return err
}

func parseID(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var number int64
	if json.Unmarshal(raw, &number) == nil {
		return strconv.FormatInt(number, 10)
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	return string(raw)
}
