package scm

import (
	"context"
	"encoding/json"
	"testing"

	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
	contract "github.com/thinkerqaq/devtool/sdk/contract"
	scmcontract "github.com/thinkerqaq/devtool/sdk/scm"
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

func TestCheckpointDelegatesSingleSCMOperation(t *testing.T) {
	var methods []string
	var got scmcontract.CheckpointRequest
	reg := &fakeRegistrar{services: map[string]service.Invoker{
		scmcontract.ServiceName: service.Func(func(_ context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
			methods = append(methods, method)
			if err := json.Unmarshal(payload, &got); err != nil {
				return nil, err
			}
			return json.Marshal(scmcontract.CheckpointResponse{
				Provider:  "scm.fake",
				Branch:    "feature/test",
				Commit:    "abc123",
				Committed: true,
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
		"scm_checkpoint",
		json.RawMessage("{\"message\":\"checkpoint change\"}"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(methods) != 1 || methods[0] != scmcontract.MethodCheckpoint {
		t.Fatalf("methods = %#v, want only checkpoint", methods)
	}
	if got.Root != "/workspace" || got.Message != "checkpoint change" {
		t.Fatalf("request = %#v", got)
	}
	if !json.Valid(raw) {
		t.Fatalf("tool result is not JSON: %s", raw)
	}
}
