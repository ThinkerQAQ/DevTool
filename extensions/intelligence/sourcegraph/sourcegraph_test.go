package sourcegraph

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thinkerqaq/devtool/core/agent/mcpbridge"
	"github.com/thinkerqaq/devtool/sdk/codeintelligence"
)

func TestQueryCallsSourcegraphMCP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			ID     json.RawMessage `json:"id,omitempty"`
			Method string          `json:"method"`
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		switch request.Method {
		case "initialize":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      request.ID,
				"result":  map[string]any{"protocolVersion": "2025-06-18"},
			})
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/call":
			if request.Params.Name != "find_references" {
				t.Fatalf("tool = %q", request.Params.Name)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      request.ID,
				"result":  map[string]any{"content": []map[string]string{{"type": "text", "text": "ok"}}},
			})
		default:
			t.Fatalf("unexpected method %q", request.Method)
		}
	}))
	defer server.Close()

	e := &Extension{
		endpoint: server.URL,
		remote:   mcpbridge.NewHTTP(server.URL, ""),
	}
	payload, err := json.Marshal(codeintelligence.GraphQuery{
		Tool: "sourcegraph_find_references",
		Args: json.RawMessage(`{"repo":"github.com/acme/repo","path":"main.go","symbol":"Foo"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := e.Invoke(context.Background(), codeintelligence.MethodQuery, payload)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(raw) {
		t.Fatalf("result is not JSON: %s", raw)
	}
}

func TestSourcegraphAuthorization(t *testing.T) {
	if got := sourcegraphAuthorization("abc"); got != "token abc" {
		t.Fatalf("authorization = %q", got)
	}
	if got := sourcegraphAuthorization(""); got != "" {
		t.Fatalf("empty authorization = %q", got)
	}
}
