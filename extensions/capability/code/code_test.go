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

func TestStableCodeToolSurface(t *testing.T) {
	e := New()
	tools, err := e.ListTools(context.Background(), agentsdk.Session{})
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(tools))
	for _, tool := range tools {
		got = append(got, tool.Name)
	}
	want := []string{"code_search", "code_symbols", "code_references", "code_diagnostics"}
	if len(got) != len(want) {
		t.Fatalf("tools = %v; want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("tools = %v; want %v", got, want)
		}
	}
}

func TestCodeSearchRoutesToIndexedSemanticService(t *testing.T) {
	var calledMethod string
	var request map[string]any
	reg := &fakeRegistrar{services: map[string]service.Invoker{
		"code-indexed": service.Func(func(_ context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
			calledMethod = method
			if err := json.Unmarshal(payload, &request); err != nil {
				return nil, err
			}
			return json.RawMessage(`{"matches":[]}`), nil
		}),
	}}
	e := New()
	if err := e.Register(reg); err != nil {
		t.Fatal(err)
	}
	raw, err := e.CallTool(context.Background(), agentsdk.Session{ProjectRoot: "/workspace"}, "code_search", json.RawMessage(`{"query":"Registry","limit":5}`))
	if err != nil {
		t.Fatal(err)
	}
	if calledMethod != "search" {
		t.Fatalf("method = %q; want search", calledMethod)
	}
	if request["query"] != "Registry" {
		t.Fatalf("query = %#v", request["query"])
	}
	if !json.Valid(raw) {
		t.Fatalf("tool result is not JSON: %s", raw)
	}
}
