package agent

import (
	"bufio"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/thinkerqaq/devtool/core/registry"
	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
	devtooltrace "github.com/thinkerqaq/devtool/sdk/trace"
)

const protocolVersion = "2025-06-18"

type Gateway struct {
	registry *registry.Registry
	session  agentsdk.Session

	mu     sync.Mutex
	routes map[string]agentsdk.ToolProvider
	tools  []json.RawMessage
}

func NewGateway(reg *registry.Registry, session agentsdk.Session) *Gateway {
	return &Gateway{registry: reg, session: session}
}

func (g *Gateway) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	encoder := json.NewEncoder(out)

	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		var request rpcRequest
		if err := json.Unmarshal(line, &request); err != nil {
			if err := encoder.Encode(errorResponse(nil, -32700, "parse error")); err != nil {
				return err
			}
			continue
		}
		if request.ID == nil {
			continue
		}
		response := g.handle(ctx, request)
		if err := encoder.Encode(response); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func (g *Gateway) HTTPHandler(token string) http.Handler {
	token = strings.TrimSpace(token)
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "{\"status\":\"ok\"}\n")
	})
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if token != "" && !authorizedBearer(r.Header.Get("Authorization"), token) {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		defer r.Body.Close()
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8*1024*1024))
		var request rpcRequest
		if err := decoder.Decode(&request); err != nil {
			writeHTTPRPC(w, http.StatusBadRequest, errorResponse(nil, -32700, "parse error"))
			return
		}
		if request.ID == nil {
			// MCP notifications do not receive JSON-RPC responses.
			w.WriteHeader(http.StatusAccepted)
			return
		}
		writeHTTPRPC(w, http.StatusOK, g.handle(r.Context(), request))
	})
	return mux
}

func authorizedBearer(header, token string) bool {
	expected := "Bearer " + token
	if len(header) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(header), []byte(expected)) == 1
}

func writeHTTPRPC(w http.ResponseWriter, status int, response rpcResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response)
}

func (g *Gateway) handle(ctx context.Context, request rpcRequest) rpcResponse {
	switch request.Method {
	case "initialize":
		return successResponse(request.ID, map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities": map[string]any{
				"tools": map[string]any{"listChanged": false},
			},
			"serverInfo": map[string]any{
				"name":    "devtool-agent",
				"version": "1",
			},
		})
	case "ping":
		return successResponse(request.ID, map[string]any{})
	case "tools/list":
		tools, err := g.listTools(ctx)
		if err != nil {
			return errorResponse(request.ID, -32603, err.Error())
		}
		return successResponse(request.ID, map[string]any{"tools": tools})
	case "tools/call":
		var params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(request.Params, &params); err != nil {
			return errorResponse(request.ID, -32602, "invalid tools/call params")
		}
		ctx, span := devtooltrace.Start(ctx, devtooltrace.Attributes{
			Name:         "tools/call",
			Layer:        "gateway",
			Tool:         params.Name,
			RequestBytes: len(params.Arguments),
		})
		result, err := g.callTool(ctx, params.Name, params.Arguments)
		if err != nil {
			failure := toolErrorResult(err)
			span.End(len(failure), err)
			return successRawResponse(request.ID, failure)
		}
		span.End(len(result), nil)
		return successRawResponse(request.ID, result)
	default:
		return errorResponse(request.ID, -32601, fmt.Sprintf("method %q not found", request.Method))
	}
}

func (g *Gateway) listTools(ctx context.Context) ([]json.RawMessage, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.routes != nil {
		return cloneRawMessages(g.tools), nil
	}

	routes := map[string]agentsdk.ToolProvider{}
	var definitions []json.RawMessage
	for _, entry := range g.registry.AgentToolProviders() {
		tools, err := entry.Provider.ListTools(ctx, g.session)
		if err != nil {
			return nil, fmt.Errorf("list tools from %s: %w", entry.ExtensionID, err)
		}
		for _, tool := range tools {
			name := strings.TrimSpace(tool.Name)
			if name == "" {
				return nil, fmt.Errorf("extension %s exposed an unnamed tool", entry.ExtensionID)
			}
			if existing, ok := routes[name]; ok && existing != entry.Provider {
				return nil, fmt.Errorf("agent tool %q is exposed by multiple extensions", name)
			}
			if !json.Valid(tool.Definition) {
				return nil, fmt.Errorf("extension %s exposed invalid JSON definition for tool %s", entry.ExtensionID, name)
			}
			routes[name] = entry.Provider
			definitions = append(definitions, append(json.RawMessage(nil), tool.Definition...))
		}
	}
	sort.Slice(definitions, func(i, j int) bool {
		return toolName(definitions[i]) < toolName(definitions[j])
	})
	g.routes = routes
	g.tools = definitions
	return cloneRawMessages(definitions), nil
}

func (g *Gateway) callTool(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
	if _, err := g.listTools(ctx); err != nil {
		return nil, err
	}
	g.mu.Lock()
	provider := g.routes[name]
	g.mu.Unlock()
	if provider == nil {
		return nil, fmt.Errorf("unknown agent tool %q", name)
	}
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	return provider.CallTool(ctx, g.session, name, args)
}

func toolName(raw json.RawMessage) string {
	var value struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(raw, &value)
	return value.Name
}

func cloneRawMessages(values []json.RawMessage) []json.RawMessage {
	out := make([]json.RawMessage, len(values))
	for i := range values {
		out[i] = append(json.RawMessage(nil), values[i]...)
	}
	return out
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func successResponse(id json.RawMessage, result any) rpcResponse {
	raw, _ := json.Marshal(result)
	return successRawResponse(id, raw)
}

func successRawResponse(id json.RawMessage, result json.RawMessage) rpcResponse {
	return rpcResponse{JSONRPC: "2.0", ID: cloneID(id), Result: append(json.RawMessage(nil), result...)}
}

func errorResponse(id json.RawMessage, code int, message string) rpcResponse {
	return rpcResponse{JSONRPC: "2.0", ID: cloneID(id), Error: &rpcError{Code: code, Message: message}}
}

func cloneID(id json.RawMessage) json.RawMessage {
	if id == nil {
		return json.RawMessage(`null`)
	}
	return append(json.RawMessage(nil), id...)
}

func toolErrorResult(err error) json.RawMessage {
	raw, _ := json.Marshal(map[string]any{
		"content": []map[string]string{{"type": "text", "text": err.Error()}},
		"isError": true,
	})
	return raw
}
