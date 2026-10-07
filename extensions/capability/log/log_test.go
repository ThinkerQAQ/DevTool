package log

import (
	"context"
	"encoding/json"
	"testing"

	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
	contract "github.com/thinkerqaq/devtool/sdk/contract"
	logintelligence "github.com/thinkerqaq/devtool/sdk/logintelligence"
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

func TestAgentSurfaceExposesOnlyLogContext(t *testing.T) {
	tools, err := New().ListTools(t.Context(), agentsdk.Session{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 || tools[0].Name != "log_context" {
		t.Fatalf("tools = %+v; want only log_context", tools)
	}
}

func TestLogContextDelegatesToConfiguredService(t *testing.T) {
	var got logintelligence.AnalyzeRequest
	reg := &fakeRegistrar{services: map[string]service.Invoker{
		logintelligence.ServiceName: service.Func(func(_ context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
			if method != logintelligence.MethodAnalyze {
				t.Fatalf("method = %q", method)
			}
			if err := json.Unmarshal(payload, &got); err != nil {
				return nil, err
			}
			return json.Marshal(logintelligence.AnalyzeResponse{
				Provider: "intelligence.log.fake",
				Source:   "logs/app.log",
				Format:   "plain",
				Summary:  logintelligence.Summary{Lines: 3, Errors: 1},
			})
		}),
	}}
	ext := New()
	if err := ext.Register(reg); err != nil {
		t.Fatal(err)
	}

	raw, err := ext.CallTool(
		t.Context(),
		agentsdk.Session{ProjectRoot: "/workspace"},
		"log_context",
		json.RawMessage(`{"objective":"find the failure","path":"logs/app.log","query":"timeout","limit":7}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	if got.Root != "/workspace" || got.Path != "logs/app.log" || got.Query != "timeout" || got.Limit != 7 {
		t.Fatalf("request = %#v", got)
	}
	if !json.Valid(raw) {
		t.Fatalf("tool result is not JSON: %s", raw)
	}
}

func TestLogContextRejectsMissingObjective(t *testing.T) {
	ext := New()
	ext.services = &fakeRegistrar{services: map[string]service.Invoker{}}
	if _, err := ext.CallTool(
		t.Context(),
		agentsdk.Session{ProjectRoot: "/workspace"},
		"log_context",
		json.RawMessage(`{"path":"logs/app.log"}`),
	); err == nil {
		t.Fatal("expected missing objective to fail")
	}
}
