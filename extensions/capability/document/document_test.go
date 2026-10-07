package document

import (
	"context"
	"encoding/json"
	"testing"

	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
	contract "github.com/thinkerqaq/devtool/sdk/contract"
	documentcontract "github.com/thinkerqaq/devtool/sdk/document"
	service "github.com/thinkerqaq/devtool/sdk/service"
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
	e := New()
	tools, err := e.ListTools(context.Background(), agentsdk.Session{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 || tools[0].Name != "document_context" {
		t.Fatalf("tools = %+v; want only document_context", tools)
	}
}

func TestDocumentContextInvokesStableDocumentService(t *testing.T) {
	var method string
	var request documentcontract.InspectRequest
	reg := &fakeRegistrar{services: map[string]service.Invoker{
		documentcontract.ServiceName: service.Func(func(_ context.Context, gotMethod string, payload json.RawMessage) (json.RawMessage, error) {
			method = gotMethod
			if err := json.Unmarshal(payload, &request); err != nil {
				return nil, err
			}
			return json.RawMessage("{\"path\":\"src/content/articles/example.md\",\"format\":\"markdown\",\"line_count\":120,\"outline\":[]}"), nil
		}),
	}}
	e := New()
	if err := e.Register(reg); err != nil {
		t.Fatal(err)
	}

	raw, err := e.CallTool(
		context.Background(),
		agentsdk.Session{ProjectRoot: "/workspace"},
		"document_context",
		json.RawMessage("{\"objective\":\"review the lifecycle section\",\"path\":\"src/content/articles/example.md\",\"section\":\"1.3\",\"include_content\":true}"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if method != documentcontract.MethodInspect {
		t.Fatalf("method = %q, want %q", method, documentcontract.MethodInspect)
	}
	if request.Root != "/workspace" {
		t.Fatalf("root = %q", request.Root)
	}
	if request.Path != "src/content/articles/example.md" || request.Section != "1.3" || !request.IncludeContent {
		t.Fatalf("request = %#v", request)
	}
	if !json.Valid(raw) {
		t.Fatalf("tool result is not JSON: %s", raw)
	}
}

func TestDocumentContextValidatesIntentAndPath(t *testing.T) {
	e := New()
	reg := &fakeRegistrar{services: map[string]service.Invoker{
		documentcontract.ServiceName: service.Func(func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage("{}"), nil
		}),
	}}
	if err := e.Register(reg); err != nil {
		t.Fatal(err)
	}

	session := agentsdk.Session{ProjectRoot: "/workspace"}
	if _, err := e.CallTool(context.Background(), session, "document_context", json.RawMessage("{\"objective\":\"\",\"path\":\"article.md\"}")); err == nil {
		t.Fatal("expected empty objective to fail")
	}
	if _, err := e.CallTool(context.Background(), session, "document_context", json.RawMessage("{\"objective\":\"review\",\"path\":\"\"}")); err == nil {
		t.Fatal("expected empty path to fail")
	}
}
