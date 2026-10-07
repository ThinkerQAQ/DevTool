package code

import (
	"context"
	"encoding/json"
	"testing"

	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
	"github.com/thinkerqaq/devtool/sdk/codecontext"
	contract "github.com/thinkerqaq/devtool/sdk/contract"
	"github.com/thinkerqaq/devtool/sdk/service"
)

type fakeRegistrar struct {
	services map[string]service.Invoker
	tools    agentsdk.ToolProvider
}

func (r *fakeRegistrar) RegisterCommand(contract.CommandDescriptor) error     { return nil }
func (r *fakeRegistrar) RegisterResource(contract.ResourceDescriptor) error   { return nil }
func (r *fakeRegistrar) RegisterView(contract.ViewDescriptor) error           { return nil }
func (r *fakeRegistrar) RegisterFeature(contract.FeatureBinding) error        { return nil }
func (r *fakeRegistrar) RegisterNavigation(contract.NavigationItem) error     { return nil }
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
	tools, err := New().ListTools(t.Context(), agentsdk.Session{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 || tools[0].Name != "code_context" {
		t.Fatalf("tools = %+v; want only code_context", tools)
	}
}

func TestCodeContextDelegatesToConfiguredContextService(t *testing.T) {
	var method string
	var request codecontext.BuildRequest
	reg := &fakeRegistrar{services: map[string]service.Invoker{
		codecontext.ServiceName: service.Func(func(_ context.Context, gotMethod string, payload json.RawMessage) (json.RawMessage, error) {
			method = gotMethod
			if err := json.Unmarshal(payload, &request); err != nil {
				return nil, err
			}
			return json.Marshal(codecontext.BuildResponse{
				Objective: request.Objective,
				Indexed:   json.RawMessage("{\"results\":[]}"),
			})
		}),
	}}
	ext := New()
	if err := ext.Register(reg); err != nil {
		t.Fatal(err)
	}

	raw, err := ext.CallTool(
		t.Context(),
		agentsdk.Session{ProjectRoot: "/workspace", Workspaces: []string{"."}},
		"code_context",
		json.RawMessage("{\"objective\":\"understand routing\",\"symbol\":\"Gateway\",\"path\":\"core/agent/gateway.go\",\"limit\":7}"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if method != codecontext.MethodBuild {
		t.Fatalf("method = %q, want %q", method, codecontext.MethodBuild)
	}
	if request.Root != "/workspace" || request.Objective != "understand routing" || request.Symbol != "Gateway" || request.Path != "core/agent/gateway.go" || request.Limit != 7 {
		t.Fatalf("request = %#v", request)
	}
	if !json.Valid(raw) {
		t.Fatalf("tool result is not JSON: %s", raw)
	}
}

func TestCodeContextRequiresObjective(t *testing.T) {
	ext := New()
	ext.services = &fakeRegistrar{services: map[string]service.Invoker{}}
	if _, err := ext.CallTool(t.Context(), agentsdk.Session{}, "code_context", json.RawMessage("{}")); err == nil {
		t.Fatal("expected missing objective to fail")
	}
}
