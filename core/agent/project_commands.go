package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/thinkerqaq/devtool/core/contract"
	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
)

type ProjectCommandRunner func(context.Context, string, map[string]any, io.Writer) error

type ProjectCommandProvider struct {
	commands map[string]contract.CommandDescriptor
	run      ProjectCommandRunner
}

func NewProjectCommandProvider(descriptor contract.ProjectDescriptor, run ProjectCommandRunner) (*ProjectCommandProvider, error) {
	if run == nil {
		return nil, fmt.Errorf("project command runner is required")
	}
	provider := &ProjectCommandProvider{
		commands: make(map[string]contract.CommandDescriptor, len(descriptor.Commands)),
		run:      run,
	}
	for _, command := range descriptor.Commands {
		name := projectToolName(command.ID)
		if name == "project" {
			return nil, fmt.Errorf("project command %q cannot be mapped to an agent tool name", command.ID)
		}
		if existing, ok := provider.commands[name]; ok {
			return nil, fmt.Errorf("project commands %q and %q map to the same agent tool %q", existing.ID, command.ID, name)
		}
		provider.commands[name] = command
	}
	return provider, nil
}

func (p *ProjectCommandProvider) ListTools(context.Context, agentsdk.Session) ([]agentsdk.Tool, error) {
	tools := make([]agentsdk.Tool, 0, len(p.commands))
	for name, command := range p.commands {
		definition, err := projectToolDefinition(name, command)
		if err != nil {
			return nil, err
		}
		tools = append(tools, agentsdk.Tool{Name: name, Definition: definition})
	}
	return tools, nil
}

func (p *ProjectCommandProvider) CallTool(ctx context.Context, _ agentsdk.Session, name string, args json.RawMessage) (json.RawMessage, error) {
	command, ok := p.commands[name]
	if !ok {
		return nil, fmt.Errorf("unknown project command agent tool %q", name)
	}
	arguments := map[string]any{}
	if len(args) != 0 && string(args) != "null" {
		if err := json.Unmarshal(args, &arguments); err != nil {
			return nil, fmt.Errorf("decode project command %q arguments: %w", command.ID, err)
		}
	}

	var out bytes.Buffer
	if err := p.run(ctx, command.ID, arguments, &out); err != nil {
		return nil, err
	}
	message := strings.TrimSpace(out.String())
	if message == "" {
		message = command.Title + " completed"
	}
	return json.Marshal(map[string]any{
		"content": []map[string]string{{"type": "text", "text": message}},
	})
}

func projectToolDefinition(name string, command contract.CommandDescriptor) (json.RawMessage, error) {
	properties := map[string]any{}
	required := make([]string, 0, len(command.Parameters))
	for _, parameter := range command.Parameters {
		schema, err := projectFieldSchema(parameter)
		if err != nil {
			return nil, fmt.Errorf("project command %q parameter %q: %w", command.ID, parameter.Key, err)
		}
		properties[parameter.Key] = schema
		if parameter.Required {
			required = append(required, parameter.Key)
		}
	}
	inputSchema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) != 0 {
		inputSchema["required"] = required
	}

	definition := map[string]any{
		"name":        name,
		"title":       command.Title,
		"description": command.Description,
		"inputSchema": inputSchema,
		"annotations": map[string]any{
			"title":           command.Title,
			"readOnlyHint":    command.SideEffect == contract.SideEffectRead,
			"destructiveHint": command.SideEffect == contract.SideEffectDestructive,
		},
	}
	return json.Marshal(definition)
}

func projectFieldSchema(field contract.FieldDescriptor) (map[string]any, error) {
	schema := map[string]any{}
	switch field.Type {
	case contract.FieldInteger:
		schema["type"] = "integer"
	case contract.FieldNumber:
		schema["type"] = "number"
	case contract.FieldBoolean:
		schema["type"] = "boolean"
	case contract.FieldMultiSelect:
		items := map[string]any{"type": "string"}
		if len(field.Options) != 0 {
			items["enum"] = field.Options
		}
		schema["type"] = "array"
		schema["items"] = items
	case contract.FieldSelect:
		schema["type"] = "string"
		if len(field.Options) != 0 {
			schema["enum"] = field.Options
		}
	case contract.FieldString, contract.FieldSecret, contract.FieldFile, contract.FieldDirectory,
		contract.FieldDate, contract.FieldTime, contract.FieldDateTime, contract.FieldDevice:
		schema["type"] = "string"
	default:
		return nil, fmt.Errorf("unsupported field type %q", field.Type)
	}
	if field.Title != "" {
		schema["title"] = field.Title
	}
	if field.Description != "" {
		schema["description"] = field.Description
	}
	return schema, nil
}

func projectToolName(commandID string) string {
	var builder strings.Builder
	builder.WriteString("project_")
	underscore := false
	for _, r := range strings.TrimSpace(commandID) {
		switch {
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(unicode.ToLower(r))
			underscore = false
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			builder.WriteRune(r)
			underscore = r == '_'
		default:
			if !underscore {
				builder.WriteByte('_')
				underscore = true
			}
		}
	}
	return strings.TrimRight(builder.String(), "_")
}
