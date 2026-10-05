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
}

func (f *fakeProjectCommandExecutor) Execute(_ context.Context, command string, _ map[string]any, out io.Writer) error {
	f.command = command
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
				Parameters: []contract.FieldDescriptor{{Key: "value", Type: contract.FieldString}},
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
	if len(tools) != 2 {
		t.Fatalf("tools = %d, want 2", len(tools))
	}
	if tools[0].Name != "project_build" || tools[1].Name != "project_device_full" {
		t.Fatalf("tools = %q, %q", tools[0].Name, tools[1].Name)
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
