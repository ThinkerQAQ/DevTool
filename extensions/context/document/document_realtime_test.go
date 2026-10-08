package document

import (
	"context"
	"encoding/json"
	"testing"

	document "github.com/thinkerqaq/devtool/sdk/document"
	realtime "github.com/thinkerqaq/devtool/sdk/documentrealtime"
	"github.com/thinkerqaq/devtool/sdk/service"
)

func TestRealtimeServiceUsesStableNormalizedContract(t *testing.T) {
	called := false
	services := map[string]service.Invoker{
		document.ServiceName: service.Func(func(ctx context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
			return json.Marshal(document.InspectResponse{Path: "doc.md", Format: "markdown"})
		}),
		realtime.ServiceName: service.Func(func(ctx context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
			var request realtime.AnalyzeRequest
			if err := json.Unmarshal(payload, &request); err != nil {
				return nil, err
			}
			if method != realtime.MethodAnalyze || request.Root != "/workspace" || request.Path != "doc.md" || request.Line != 7 || request.Column != 13 || !request.IncludeReferences {
				t.Fatalf("bad request: %+v", request)
			}
			called = true
			return json.Marshal(realtime.AnalyzeResponse{Provider: "test", Status: "ok", WorkspaceScope: "docs", Symbols: []realtime.Symbol{{Name: "Overview"}}})
		}),
	}
	e := New()
	if err := e.Register(&fakeRegistrar{services: services}); err != nil {
		t.Fatal(err)
	}
	raw, err := callContext(t, e, t.Context(), "/workspace", json.RawMessage(`{"objective":"review","path":"doc.md","realtime":true,"line":7,"column":13,"include_references":true}`))
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Realtime realtime.AnalyzeResponse `json:"realtime"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if !called || out.Realtime.Status != "ok" || len(out.Realtime.Symbols) != 1 {
		t.Fatalf("got=%s", raw)
	}
}

func TestRealtimeWithoutProviderDoesNotClaimSuccess(t *testing.T) {
	e := New()
	if err := e.Register(&fakeRegistrar{services: map[string]service.Invoker{
		document.ServiceName: service.Func(func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(`{"path":"doc.md","format":"markdown"}`), nil
		}),
	}}); err != nil {
		t.Fatal(err)
	}
	raw, err := callContext(t, e, t.Context(), "/workspace", json.RawMessage(`{"objective":"review","path":"doc.md","realtime":true}`))
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	realtimeField := v["realtime"].(map[string]any)
	if realtimeField["status"] != "unconfigured" {
		t.Fatalf("result=%s", raw)
	}
}
