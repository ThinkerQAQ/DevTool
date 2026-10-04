package agent

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/thinkerqaq/devtool/core/contract"
	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
)

func TestProjectCommandProviderMapsCommandsToAgentTools(t *testing.T) {
	descriptor := contract.ProjectDescriptor{
		Commands: []contract.CommandDescriptor{
			{
				ID:          "runtime.doctor",
				Title:       "Runtime Doctor",
				Description: "Verify the configured runtime.",
				SideEffect:  contract.SideEffectRead,
			},
			{
				ID:          "deploy",
				Title:       "Deploy",
				Description: "Deploy the project.",
				SideEffect:  contract.SideEffectDestructive,
				Parameters: []contract.FieldDescriptor{
					{Key: "target", Title: "Target", Type: contract.FieldSelect, Required: true, Options: []string{"staging", "production"}},
					{Key: "force", Title: "Force", Type: contract.FieldBoolean},
				},
			},
		},
	}

	var calledCommand string
	var calledArgs map[string]any
	provider, err := NewProjectCommandProvider(descriptor, func(_ context.Context, command string, args map[string]any, out io.Writer) error {
		calledCommand = command
		calledArgs = args
		_, _ = io.WriteString(out, "command completed")
		return nil
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

	definitions := map[string]map[string]any{}
	for _, tool := range tools {
		var definition map[string]any
		if err := json.Unmarshal(tool.Definition, &definition); err != nil {
			t.Fatal(err)
		}
		definitions[tool.Name] = definition
	}

	doctor := definitions["project_runtime_doctor"]
	annotations := doctor["annotations"].(map[string]any)
	if annotations["readOnlyHint"] != true || annotations["destructiveHint"] != false {
		t.Fatalf("runtime doctor annotations = %#v", annotations)
	}

	deploy := definitions["project_deploy"]
	deployAnnotations := deploy["annotations"].(map[string]any)
	if deployAnnotations["readOnlyHint"] != false || deployAnnotations["destructiveHint"] != true {
		t.Fatalf("deploy annotations = %#v", deployAnnotations)
	}
	inputSchema := deploy["inputSchema"].(map[string]any)
	required := inputSchema["required"].([]any)
	if len(required) != 1 || required[0] != "target" {
		t.Fatalf("deploy required = %#v", required)
	}

	result, err := provider.CallTool(
		context.Background(),
		agentsdk.Session{},
		"project_deploy",
		json.RawMessage(`{"target":"staging","force":true}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	if calledCommand != "deploy" || calledArgs["target"] != "staging" || calledArgs["force"] != true {
		t.Fatalf("call = %q %#v", calledCommand, calledArgs)
	}
	if !strings.Contains(string(result), "command completed") {
		t.Fatalf("result = %s", result)
	}
}

func TestProjectCommandProviderRejectsToolNameCollision(t *testing.T) {
	_, err := NewProjectCommandProvider(contract.ProjectDescriptor{
		Commands: []contract.CommandDescriptor{
			{ID: "runtime.doctor", Title: "A", SideEffect: contract.SideEffectRead},
			{ID: "runtime-doctor", Title: "B", SideEffect: contract.SideEffectRead},
		},
	}, func(context.Context, string, map[string]any, io.Writer) error { return nil })
	if err == nil {
		t.Fatal("expected normalized project tool name collision")
	}
}
