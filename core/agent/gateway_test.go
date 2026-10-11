package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"testing"

	"github.com/thinkerqaq/devtool/core/registry"
	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
)

type fakeProvider struct {
	name string
}

func (f fakeProvider) ListTools(context.Context, agentsdk.Session) ([]agentsdk.Tool, error) {
	raw, _ := json.Marshal(map[string]any{
		"name":        f.name,
		"description": "fake tool",
		"inputSchema": map[string]any{"type": "object"},
	})
	return []agentsdk.Tool{{Name: f.name, Definition: raw}}, nil
}

func (f fakeProvider) CallTool(context.Context, agentsdk.Session, string, json.RawMessage) (json.RawMessage, error) {
	return json.RawMessage(`{"content":[{"type":"text","text":"ok"}]}`), nil
}

func TestGatewayDiscoversToolsFromRegistry(t *testing.T) {
	reg := registry.New()
	for _, name := range []string{"z_tool", "a_tool"} {
		if err := reg.ProvideAgentTools(name, fakeProvider{name: name}); err != nil {
			t.Fatal(err)
		}
	}
	server, err := NewGateway(reg, agentsdk.Session{ProjectRoot: "/tmp/example"}).server(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(t.Context(), serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	clientSession, err := mcp.NewClient(&mcp.Implementation{Name: "devtool-test", Version: "1"}, nil).Connect(t.Context(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()
	listed, err := clientSession.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Tools) != 2 || listed.Tools[0].Name != "a_tool" || listed.Tools[1].Name != "z_tool" {
		t.Fatalf("unexpected tool order: %+v", listed.Tools)
	}
	result, err := clientSession.CallTool(t.Context(), &mcp.CallToolParams{Name: "a_tool", Arguments: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError || len(result.Content) != 1 {
		t.Fatalf("tool call failed: %+v", result)
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok || text.Text != "ok" {
		t.Fatalf("unexpected tool result: %+v", result.Content)
	}
}

func TestGatewayHTTPTransport(t *testing.T) {
	reg := registry.New()
	if err := reg.ProvideAgentTools("fake.extension", fakeProvider{name: "fake_tool"}); err != nil {
		t.Fatal(err)
	}
	gateway := NewGateway(reg, agentsdk.Session{ProjectRoot: "/tmp/example"})
	handler := gateway.HTTPHandler("secret")

	unauthorized := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	unauthorized.Header.Set("Content-Type", "application/json")
	unauthorizedRecorder := httptest.NewRecorder()
	handler.ServeHTTP(unauthorizedRecorder, unauthorized)
	if unauthorizedRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, want %d", unauthorizedRecorder.Code, http.StatusUnauthorized)
	}

	request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`))
	request.Header.Set("Authorization", "Bearer secret")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("Mcp-Protocol-Version", "2025-06-18")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("tools/list status = %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"name":"fake_tool"`) {
		t.Fatalf("tools/list response = %s", recorder.Body.String())
	}

	notification := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`))
	notification.Header.Set("Authorization", "Bearer secret")
	notification.Header.Set("Content-Type", "application/json")
	notification.Header.Set("Accept", "application/json, text/event-stream")
	notification.Header.Set("Mcp-Protocol-Version", "2025-06-18")
	notificationRecorder := httptest.NewRecorder()
	handler.ServeHTTP(notificationRecorder, notification)
	if notificationRecorder.Code != http.StatusAccepted {
		t.Fatalf("notification status = %d, want %d", notificationRecorder.Code, http.StatusAccepted)
	}

	health := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	healthRecorder := httptest.NewRecorder()
	handler.ServeHTTP(healthRecorder, health)
	if healthRecorder.Code != http.StatusOK {
		t.Fatalf("health status = %d", healthRecorder.Code)
	}
}
