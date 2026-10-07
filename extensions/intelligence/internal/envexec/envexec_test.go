package envexec

import (
	"context"
	"encoding/json"
	"testing"

	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
	"github.com/thinkerqaq/devtool/sdk/codeintelligence"
	contract "github.com/thinkerqaq/devtool/sdk/contract"
	environmentcontract "github.com/thinkerqaq/devtool/sdk/environment"
	"github.com/thinkerqaq/devtool/sdk/service"
)

type fakeRegistrar struct {
	services map[string]service.Invoker
}

func (r *fakeRegistrar) RegisterCommand(contract.CommandDescriptor) error      { return nil }
func (r *fakeRegistrar) RegisterResource(contract.ResourceDescriptor) error    { return nil }
func (r *fakeRegistrar) RegisterView(contract.ViewDescriptor) error            { return nil }
func (r *fakeRegistrar) RegisterFeature(contract.FeatureBinding) error         { return nil }
func (r *fakeRegistrar) RegisterNavigation(contract.NavigationItem) error      { return nil }
func (r *fakeRegistrar) ProvideService(string, string, service.Invoker) error  { return nil }
func (r *fakeRegistrar) ProvideAgentTools(string, agentsdk.ToolProvider) error { return nil }
func (r *fakeRegistrar) Service(name string) (service.Invoker, bool) {
	value, ok := r.services[name]
	return value, ok
}

func TestCommandUsesToolingEnvironment(t *testing.T) {
	var called string
	reg := &fakeRegistrar{services: map[string]service.Invoker{
		environmentcontract.ServiceName: service.Func(func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
			t.Fatal("project environment must not be used for intelligence tooling")
			return nil, nil
		}),
		environmentcontract.ToolingServiceName: service.Func(func(_ context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
			called = method
			var request environmentcontract.CommandRequest
			if err := json.Unmarshal(payload, &request); err != nil {
				return nil, err
			}
			if request.Executable != "gopls" || request.Root != "/workspace" {
				t.Fatalf("request = %#v", request)
			}
			return json.Marshal(environmentcontract.CommandSpec{
				Program: "gopls",
				Args:    []string{"version"},
				Dir:     "/workspace",
			})
		}),
	}}

	cmd, err := Command(
		t.Context(),
		reg,
		codeintelligence.Workspace{Root: "/workspace"},
		"gopls",
		"version",
	)
	if err != nil {
		t.Fatal(err)
	}
	if called != environmentcontract.MethodCommand {
		t.Fatalf("method = %q, want %q", called, environmentcontract.MethodCommand)
	}
	if len(cmd.Args) != 2 || cmd.Args[0] != "gopls" || cmd.Args[1] != "version" {
		t.Fatalf("command = %#v", cmd.Args)
	}
}

func TestCommandRequiresToolingEnvironment(t *testing.T) {
	reg := &fakeRegistrar{services: map[string]service.Invoker{}}
	if _, err := Command(t.Context(), reg, codeintelligence.Workspace{Root: "/workspace"}, "gopls"); err == nil {
		t.Fatal("expected missing tooling environment to fail")
	}
}
