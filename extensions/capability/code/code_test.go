package code

import (
	"context"
	"encoding/json"
	"testing"

	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
	contract "github.com/thinkerqaq/devtool/sdk/contract"
	"github.com/thinkerqaq/devtool/sdk/service"
)

type fakeRegistrar struct {
	services map[string]service.Invoker
	tools    agentsdk.ToolProvider
}

func (r *fakeRegistrar) RegisterCommand(contract.CommandDescriptor) error { return nil }
func (r *fakeRegistrar) RegisterResource(contract.ResourceDescriptor) error { return nil }
func (r *fakeRegistrar) RegisterView(contract.ViewDescriptor) error { return nil }
func (r *fakeRegistrar) RegisterFeature(contract.FeatureBinding) error { return nil }
func (r *fakeRegistrar) RegisterNavigation(contract.NavigationItem) error { return nil }
func (r *fakeRegistrar) ProvideService(string, string, service.Invoker) error { return nil }
func (r *fakeRegistrar) Service(name string) (service.Invoker, bool) {
	value, ok := r.services[name]
	return value, ok
}
func (r *fakeRegistrar) ProvideAgentTools(_ string, provider agentsdk.ToolProvider) error {
	r.tools = provider
	return nil
}

func TestAgentSurfaceExposesOneIntentLevelCodeCapability(t *testing.T) {
	e := New()
	tools, err := e.ListTools(context.Background(), agentsdk.Session{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 || tools[0].Name != "code_context" {
		t.Fatalf("tools = %+v; want only code_context", tools)
	}
}

func TestCodeContextComposesIndexedAndRealtimeServices(t *testing.T) {
	var calls []string
	reg := &fakeRegistrar{services: map[string]service.Invoker{
		"code-indexed": service.Func(func(_ context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
			calls = append(calls, "indexed."+method)
			var request map[string]any
			if err := json.Unmarshal(payload, &request); err != nil {
				return nil, err
			}
			if request["query"] != "Registry" {
				t.Fatalf("indexed query = %#v", request["query"])
			}
			return json.RawMessage(`{"matches":[{"path":"core/registry/registry.go"}]}`), nil
		}),
		"code-realtime": service.Func(func(_ context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
			calls = append(calls, "realtime."+method)
			switch method {
			case "symbols":
				return json.RawMessage(`{"symbols":[{"name":"Registry"}]}`), nil
			case "references":
				return json.RawMessage(`{"references":[{"path":"core/host/project.go"}]}`), nil
			case "diagnostics":
				return json.RawMessage(`{"diagnostics":[]}`), nil
			default:
				t.Fatalf("unexpected realtime method %q", method)
				return nil, nil
			}
		}),
	}}
	e := New()
	if err := e.Register(reg); err != nil {
		t.Fatal(err)
	}

	raw, err := e.CallTool(
		context.Background(),
		agentsdk.Session{ProjectRoot: "/workspace"},
		"code_context",
		json.RawMessage(`{"objective":"understand registry routing","symbol":"Registry","path":"core/registry/registry.go","limit":5}`),
	)
	if err != nil {
		t.Fatal(err)
	}

	wantCalls := []string{"indexed.search", "realtime.symbols", "realtime.references", "realtime.diagnostics"}
	if len(calls) != len(wantCalls) {
		t.Fatalf("calls = %v; want %v", calls, wantCalls)
	}
	for i := range wantCalls {
		if calls[i] != wantCalls[i] {
			t.Fatalf("calls = %v; want %v", calls, wantCalls)
		}
	}
	if !json.Valid(raw) {
		t.Fatalf("tool result is not JSON: %s", raw)
	}
}
