package mcpbridge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
)

func TestHTTPProviderListsAndCallsTools(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "remote-fake", Version: "1"}, nil)
	server.AddTool(&mcp.Tool{Name: "find_references", Description: "find references", InputSchema: map[string]any{"type": "object"}}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil
	})
	endpoint := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "token test" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		endpoint.ServeHTTP(w, r)
	}))
	defer httpServer.Close()
	provider := NewHTTP(httpServer.URL, "token test")
	defer provider.Close()
	tools, err := provider.ListTools(context.Background(), agentsdk.Session{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 || tools[0].Name != "find_references" {
		t.Fatalf("tools = %#v", tools)
	}
	raw, err := provider.CallTool(context.Background(), agentsdk.Session{}, "find_references", json.RawMessage(`{"symbol":"Foo"}`))
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &result); err != nil || len(result.Content) != 1 || result.Content[0].Text != "ok" {
		t.Fatalf("unexpected tool result: %s err=%v", raw, err)
	}
}
