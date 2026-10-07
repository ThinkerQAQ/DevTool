package workspacecapability

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
	contract "github.com/thinkerqaq/devtool/sdk/contract"
	"github.com/thinkerqaq/devtool/sdk/service"
	workspacecontract "github.com/thinkerqaq/devtool/sdk/workspace"
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

func TestAgentSurfaceUsesWorkspaceIntentNotGitCommands(t *testing.T) {
	e := New()
	tools, err := e.ListTools(context.Background(), agentsdk.Session{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"workspace_create",
		"workspace_list",
		"workspace_inspect",
		"workspace_remove",
	}
	if len(tools) != len(want) {
		t.Fatalf("tools = %+v", tools)
	}
	for index, tool := range tools {
		if tool.Name != want[index] {
			t.Fatalf("tool[%d] = %q; want %q", index, tool.Name, want[index])
		}
		lower := strings.ToLower(string(tool.Definition))
		if strings.Contains(lower, "git worktree") {
			t.Fatalf("provider-native Git API leaked into tool definition: %s", tool.Definition)
		}
	}
}

func TestWorkspaceCreateDelegatesToWorkspaceService(t *testing.T) {
	var method string
	var request workspacecontract.CreateRequest
	reg := &fakeRegistrar{services: map[string]service.Invoker{
		workspacecontract.ServiceName: service.Func(func(
			_ context.Context,
			gotMethod string,
			payload json.RawMessage,
		) (json.RawMessage, error) {
			method = gotMethod
			if err := json.Unmarshal(payload, &request); err != nil {
				return nil, err
			}
			return json.RawMessage(`{
				"workspace":{
					"provider":"workspace.git",
					"name":"feature/ui",
					"identity":{
						"repository_id":"repo-1",
						"workspace_id":"workspace-1",
						"root":"/workspaces/feature-ui"
					},
					"revision":"feature/ui"
				}
			}`), nil
		}),
	}}
	e := New()
	if err := e.Register(reg); err != nil {
		t.Fatal(err)
	}

	raw, err := e.CallTool(
		context.Background(),
		agentsdk.Session{ProjectRoot: "/workspaces/DevTool"},
		"workspace_create",
		json.RawMessage(`{"name":"feature/ui","revision":"main"}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	if method != workspacecontract.MethodCreate {
		t.Fatalf("method = %q", method)
	}
	if request.Root != "/workspaces/DevTool" ||
		request.Name != "feature/ui" ||
		request.Revision != "main" {
		t.Fatalf("request = %#v", request)
	}
	if !strings.Contains(string(raw), "workspace-1") {
		t.Fatalf("result = %s", raw)
	}
}

func TestWorkspaceListDelegatesCurrentProjectRoot(t *testing.T) {
	var request workspacecontract.ListRequest
	reg := &fakeRegistrar{services: map[string]service.Invoker{
		workspacecontract.ServiceName: service.Func(func(
			_ context.Context,
			method string,
			payload json.RawMessage,
		) (json.RawMessage, error) {
			if method != workspacecontract.MethodList {
				t.Fatalf("method = %q", method)
			}
			if err := json.Unmarshal(payload, &request); err != nil {
				return nil, err
			}
			return json.RawMessage(`{"workspaces":[]}`), nil
		}),
	}}
	e := New()
	if err := e.Register(reg); err != nil {
		t.Fatal(err)
	}
	if _, err := e.CallTool(
		context.Background(),
		agentsdk.Session{ProjectRoot: "/workspaces/DevTool"},
		"workspace_list",
		nil,
	); err != nil {
		t.Fatal(err)
	}
	if request.Root != "/workspaces/DevTool" {
		t.Fatalf("root = %q", request.Root)
	}
}
