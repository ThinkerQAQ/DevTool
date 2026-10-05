package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
	if err := reg.ProvideAgentTools("z.extension", fakeProvider{name: "z_tool"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.ProvideAgentTools("a.extension", fakeProvider{name: "a_tool"}); err != nil {
		t.Fatal(err)
	}
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"a_tool","arguments":{}}}`,
	}, "\n") + "\n"
	var out bytes.Buffer
	if err := NewGateway(reg, agentsdk.Session{ProjectRoot: "/tmp/example"}).Serve(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("responses = %d, want 3: %s", len(lines), out.String())
	}
	if !strings.Contains(lines[1], `"name":"a_tool"`) || !strings.Contains(lines[1], `"name":"z_tool"`) {
		t.Fatalf("tools/list response = %s", lines[1])
	}
	if !strings.Contains(lines[2], `"text":"ok"`) {
		t.Fatalf("tools/call response = %s", lines[2])
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
