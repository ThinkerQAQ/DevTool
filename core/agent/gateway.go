package agent

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/thinkerqaq/devtool/core/registry"
	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
	devtooltrace "github.com/thinkerqaq/devtool/sdk/trace"
)

// Gateway implements the project-aware engineering tool contract. The official
// MCP SDK owns protocol parsing, version negotiation and transport lifecycle.
type Gateway struct {
	registry *registry.Registry
	session  agentsdk.Session

	mu     sync.Mutex
	routes map[string]agentsdk.ToolProvider
	tools  []json.RawMessage
	policy agentsdk.ExecutionPolicy
}

func NewGateway(reg *registry.Registry, session agentsdk.Session) *Gateway {
	return &Gateway{registry: reg, session: session}
}

// WithPolicy connects a deterministic execution policy adapter.
func (g *Gateway) WithPolicy(policy agentsdk.ExecutionPolicy) *Gateway {
	g.policy = policy
	return g
}

func (g *Gateway) server(ctx context.Context) (*mcp.Server, error) {
	definitions, err := g.listTools(ctx)
	if err != nil {
		return nil, err
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "devtool-mcp", Version: "1"}, nil)
	for _, raw := range definitions {
		var tool mcp.Tool
		if err := json.Unmarshal(raw, &tool); err != nil {
			return nil, fmt.Errorf("decode MCP tool definition: %w", err)
		}
		if tool.Name == "" || tool.InputSchema == nil {
			return nil, fmt.Errorf("MCP tool name and input schema are required")
		}
		name := tool.Name
		server.AddTool(&tool, func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := request.Params.Arguments
			if len(args) == 0 {
				args = json.RawMessage(`{}`)
			}
			ctx, span := devtooltrace.Start(ctx, devtooltrace.Attributes{
				Name: "tools/call", Layer: "gateway", Tool: name,
				RequestBytes: len(args),
			})
			result, err := g.callTool(ctx, name, args)
			if err != nil {
				result = toolErrorResult(err)
			}
			span.End(len(result), err)
			var out mcp.CallToolResult
			if err := json.Unmarshal(result, &out); err != nil {
				return nil, fmt.Errorf("decode tool result %s: %w", name, err)
			}
			return &out, nil
		})
	}
	return server, nil
}

// Preflight checks the exact MCP tool registration that will be used by
// Serve/HTTPHandler, without opening any transport or executing tools.
func (g *Gateway) Preflight(ctx context.Context) (int, error) {
	if _, err := g.server(ctx); err != nil {
		return 0, err
	}
	definitions, err := g.listTools(ctx)
	if err != nil {
		return 0, err
	}
	return len(definitions), nil
}

// Serve uses the SDK's standard newline-delimited JSON stdio transport.
func (g *Gateway) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	server, err := g.server(ctx)
	if err != nil {
		return err
	}
	return server.Run(ctx, &mcp.IOTransport{
		Reader: io.NopCloser(in),
		Writer: writeOnlyCloser{Writer: out},
	})
}

type writeOnlyCloser struct{ io.Writer }

func (writeOnlyCloser) Close() error { return nil }

// HTTPHandler delegates the MCP wire protocol and session lifecycle to the SDK
// while keeping DevTool's bearer authorization and health endpoint.
func (g *Gateway) HTTPHandler(token string) http.Handler {
	server, err := g.server(context.Background())
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		})
	}
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{
		Stateless:           true,
		JSONResponse:        true,
		MaxRequestBodyBytes: 8 << 20,
	})
	token = strings.TrimSpace(token)
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = io.WriteString(w, "{\"status\":\"ok\"}\n")
	})
	mux.Handle("/mcp", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token != "" && !authorizedBearer(r.Header.Get("Authorization"), token) {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		handler.ServeHTTP(w, r)
	}))
	return mux
}

func authorizedBearer(header, token string) bool {
	expected := "Bearer " + token
	return len(header) == len(expected) && subtle.ConstantTimeCompare([]byte(header), []byte(expected)) == 1
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
			if _, ok := routes[name]; ok {
				return nil, fmt.Errorf("agent tool %q is exposed by multiple extensions", name)
			}
			if !json.Valid(tool.Definition) {
				return nil, fmt.Errorf("extension %s exposed invalid JSON definition for tool %s", entry.ExtensionID, name)
			}
			routes[name] = entry.Provider
			definitions = append(definitions, append(json.RawMessage(nil), tool.Definition...))
		}
	}
	if g.policy != nil {
		if _, conflict := routes["workspace_apply_patch"]; conflict {
			return nil, fmt.Errorf("workspace_apply_patch conflicts with a project agent tool")
		}
		definitions = append(definitions, g.policy.Tool())
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
	if g.policy != nil && name == "workspace_apply_patch" {
		return g.policy.Apply(ctx, args)
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
	result, err := provider.CallTool(ctx, g.session, name, args)
	if err == nil && g.policy != nil {
		result = g.policy.Attest(ctx, name, args, result)
	}
	return result, err
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

func toolErrorResult(err error) json.RawMessage {
	result, _ := json.Marshal(map[string]any{
		"content": []map[string]string{{"type": "text", "text": err.Error()}},
		"isError": true,
	})
	return result
}
