package mcpbridge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
)

func TestHTTPProviderListsAndCallsTools(t *testing.T) {
	var sessionID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "token test" {
			t.Fatalf("Authorization = %q", got)
		}
		var request struct {
			ID     json.RawMessage `json:"id,omitempty"`
			Method string          `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		switch request.Method {
		case "initialize":
			sessionID = "session-test"
			w.Header().Set("Mcp-Session-Id", sessionID)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      request.ID,
				"result": map[string]any{
					"protocolVersion": "2025-06-18",
					"capabilities":    map[string]any{},
				},
			})
		case "notifications/initialized":
			if got := r.Header.Get("Mcp-Session-Id"); got != sessionID {
				t.Fatalf("notification session = %q", got)
			}
			w.WriteHeader(http.StatusAccepted)
		case "tools/list":
			if got := r.Header.Get("Mcp-Session-Id"); got != sessionID {
				t.Fatalf("tools/list session = %q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      request.ID,
				"result": map[string]any{
					"tools": []map[string]any{{
						"name":        "find_references",
						"description": "find references",
						"inputSchema": map[string]any{"type": "object"},
					}},
				},
			})
		case "tools/call":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      request.ID,
				"result": map[string]any{
					"content": []map[string]string{{"type": "text", "text": "ok"}},
				},
			})
		default:
			t.Fatalf("unexpected method %q", request.Method)
		}
	}))
	defer server.Close()

	provider := NewHTTP(server.URL, "token test")
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
	if !json.Valid(raw) {
		t.Fatalf("result is not JSON: %s", raw)
	}
}
