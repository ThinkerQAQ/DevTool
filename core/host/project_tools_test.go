package host

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/thinkerqaq/devtool/core/contract"
	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
)

type fakeProjectCommandExecutor struct {
	command string
	args    map[string]any
}

func (f *fakeProjectCommandExecutor) Execute(_ context.Context, command string, args map[string]any, out io.Writer) error {
	f.command = command
	f.args = args
	_, _ = io.WriteString(out, "ran "+command)
	return nil
}

func TestProjectToolProviderExposesExecutableProjectCommands(t *testing.T) {
	executor := &fakeProjectCommandExecutor{}
	provider, err := newProjectToolProvider(executor, contract.ProjectDescriptor{
		Identity: contract.ProjectIdentity{Name: "example"},
		Commands: []contract.CommandDescriptor{
			{ID: "build", Title: "Build", SideEffect: contract.SideEffectWrite},
			{ID: "device.full", Title: "Device Full", SideEffect: contract.SideEffectWrite},
			{
				ID:         "parameterized",
				Title:      "Parameterized",
				SideEffect: contract.SideEffectRead,
				Parameters: []contract.FieldDescriptor{{
					Key:      "value",
					Type:     contract.FieldString,
					Required: true,
				}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	tools, err := provider.ListTools(context.Background(), agentsdk.Session{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 3 {
		t.Fatalf("tools = %d, want 3", len(tools))
	}
	if tools[0].Name != "project_build" || tools[1].Name != "project_device_full" || tools[2].Name != "project_parameterized" {
		t.Fatalf("tools = %q, %q, %q", tools[0].Name, tools[1].Name, tools[2].Name)
	}

	if _, err := provider.CallTool(
		context.Background(),
		agentsdk.Session{},
		"project_parameterized",
		json.RawMessage(`{"value":"hello"}`),
	); err != nil {
		t.Fatal(err)
	}
	if executor.command != "parameterized" || executor.args["value"] != "hello" {
		t.Fatalf("parameterized execution = command %q args %#v", executor.command, executor.args)
	}

	result, err := provider.CallTool(context.Background(), agentsdk.Session{}, "project_device_full", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if executor.command != "device.full" {
		t.Fatalf("executed %q, want device.full", executor.command)
	}
	if !strings.Contains(string(result), "ran device.full") {
		t.Fatalf("result = %s", result)
	}
}

func TestProjectToolProviderValidatesParameterizedArguments(t *testing.T) {
	provider, err := newProjectToolProvider(&fakeProjectCommandExecutor{}, contract.ProjectDescriptor{
		Identity: contract.ProjectIdentity{Name: "example"},
		Commands: []contract.CommandDescriptor{{
			ID:         "deploy",
			Title:      "Deploy",
			SideEffect: contract.SideEffectDeploy,
			Parameters: []contract.FieldDescriptor{
				{Key: "environment", Type: contract.FieldSelect, Required: true, Options: []string{"staging", "production"}},
				{Key: "replicas", Type: contract.FieldInteger},
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		args json.RawMessage
		want string
	}{
		{name: "missing required", args: json.RawMessage("{}"), want: "requires argument"},
		{name: "unknown argument", args: json.RawMessage("{\"environment\":\"staging\",\"extra\":true}"), want: "does not accept argument"},
		{name: "invalid select", args: json.RawMessage("{\"environment\":\"dev\"}"), want: "must be one of"},
		{name: "invalid integer", args: json.RawMessage("{\"environment\":\"staging\",\"replicas\":1.5}"), want: "must be an integer"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := provider.CallTool(context.Background(), agentsdk.Session{}, "project_deploy", tc.args)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want substring %q", err, tc.want)
			}
		})
	}
}
