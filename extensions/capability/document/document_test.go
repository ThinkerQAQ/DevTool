package document

import (
	"context"
	"encoding/json"
	"testing"

	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
	contract "github.com/thinkerqaq/devtool/sdk/contract"
	"github.com/thinkerqaq/devtool/sdk/documentcontext"
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

func TestAgentSurfaceExposesOneIntentLevelDocumentCapability(t *testing.T) {
	tools, err := New().ListTools(t.Context(), agentsdk.Session{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 || tools[0].Name != "document_context" {
		t.Fatalf("tools = %+v; want only document_context", tools)
	}
}

func TestDocumentContextDelegatesToConfiguredContextService(t *testing.T) {
	var method string
	var request documentcontext.BuildRequest
	reg := &fakeRegistrar{services: map[string]service.Invoker{
		documentcontext.ServiceName: service.Func(func(_ context.Context, gotMethod string, payload json.RawMessage) (json.RawMessage, error) {
			method = gotMethod
			if err := json.Unmarshal(payload, &request); err != nil {
				return nil, err
			}
			return json.RawMessage("{\"objective\":\"review\",\"document\":{\"path\":\"docs/example.md\"}}"), nil
		}),
	}}
	ext := New()
	if err := ext.Register(reg); err != nil {
		t.Fatal(err)
	}

	raw, err := ext.CallTool(
		t.Context(),
		agentsdk.Session{ProjectRoot: "/workspace"},
		"document_context",
		json.RawMessage("{\"objective\":\"review\",\"path\":\"docs/example.md\",\"review\":true}"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if method != documentcontext.MethodBuild {
		t.Fatalf("method = %q, want %q", method, documentcontext.MethodBuild)
	}
	if request.Root != "/workspace" || request.Objective != "review" || request.Path != "docs/example.md" || !request.Review {
		t.Fatalf("request = %#v", request)
	}
	if !json.Valid(raw) {
		t.Fatalf("tool result is not JSON: %s", raw)
	}
}

func TestDocumentContextRequiresObjectiveAndPath(t *testing.T) {
	ext := New()
	ext.services = &fakeRegistrar{services: map[string]service.Invoker{}}
	if _, err := ext.CallTool(t.Context(), agentsdk.Session{}, "document_context", json.RawMessage("{\"path\":\"a.md\"}")); err == nil {
		t.Fatal("expected missing objective to fail")
	}
	if _, err := ext.CallTool(t.Context(), agentsdk.Session{}, "document_context", json.RawMessage("{\"objective\":\"review\"}")); err == nil {
		t.Fatal("expected missing path to fail")
	}
}
