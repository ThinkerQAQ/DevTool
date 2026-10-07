package host

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/thinkerqaq/devtool/core/contract"
	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
)

type projectCommandExecutor interface {
	Execute(context.Context, string, map[string]any, io.Writer) error
}

type projectToolProvider struct {
	executor projectCommandExecutor
	commands map[string]contract.CommandDescriptor
}

func newProjectToolProvider(executor projectCommandExecutor, descriptor contract.ProjectDescriptor) (*projectToolProvider, error) {
	provider := &projectToolProvider{
		executor: executor,
		commands: map[string]contract.CommandDescriptor{},
	}
	for _, command := range descriptor.Commands {
		name := projectToolName(command.ID)
		if existing, ok := provider.commands[name]; ok {
			return nil, fmt.Errorf("project commands %q and %q map to the same agent tool %q", existing.ID, command.ID, name)
		}
		provider.commands[name] = command
	}
	return provider, nil
}

func (p *projectToolProvider) Count() int {
	if p == nil {
		return 0
	}
	return len(p.commands)
}

func (p *projectToolProvider) ListTools(context.Context, agentsdk.Session) ([]agentsdk.Tool, error) {
	if p == nil {
		return nil, nil
	}
	names := make([]string, 0, len(p.commands))
	for name := range p.commands {
		names = append(names, name)
	}
	sort.Strings(names)

	tools := make([]agentsdk.Tool, 0, len(names))
	for _, name := range names {
		command := p.commands[name]
		description := strings.TrimSpace(command.Description)
		if description == "" {
			description = command.Title
		}
		if description == "" {
			description = "Run project command " + command.ID
		}
		description = fmt.Sprintf("%s Project command: %s. Side effect: %s.", description, command.ID, command.SideEffect)
		definition, _ := json.Marshal(map[string]any{
			"name":        name,
			"description": description,
			"inputSchema": projectCommandInputSchema(command),
		})
		tools = append(tools, agentsdk.Tool{Name: name, Definition: definition})
	}
	return tools, nil
}

func (p *projectToolProvider) CallTool(ctx context.Context, _ agentsdk.Session, name string, args json.RawMessage) (json.RawMessage, error) {
	if p == nil || p.executor == nil {
		return nil, fmt.Errorf("project command executor is unavailable")
	}
	command, ok := p.commands[strings.TrimSpace(name)]
	if !ok {
		return nil, fmt.Errorf("unknown project command tool %q", name)
	}

	values := map[string]any{}
	if len(args) != 0 {
		if err := json.Unmarshal(args, &values); err != nil {
			return nil, fmt.Errorf("decode project command arguments: %w", err)
		}
	}
	if err := validateProjectCommandArguments(command, values); err != nil {
		return nil, err
	}

	var output bytes.Buffer
	if err := p.executor.Execute(ctx, command.ID, values, &output); err != nil {
		return nil, err
	}
	result := map[string]any{"command": command.ID, "status": "ok"}
	if text := strings.TrimSpace(output.String()); text != "" {
		result["output"] = text
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"content": []map[string]string{{"type": "text", "text": string(raw)}},
	})
}

func projectCommandInputSchema(command contract.CommandDescriptor) map[string]any {
	properties := make(map[string]any, len(command.Parameters))
	required := make([]string, 0, len(command.Parameters))
	for _, field := range command.Parameters {
		properties[field.Key] = projectFieldSchema(field)
		if field.Required {
			required = append(required, field.Key)
		}
	}
	schema := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(required) != 0 {
		schema["required"] = required
	}
	return schema
}

func projectFieldSchema(field contract.FieldDescriptor) map[string]any {
	schema := map[string]any{}
	switch field.Type {
	case contract.FieldInteger:
		schema["type"] = "integer"
	case contract.FieldNumber:
		schema["type"] = "number"
	case contract.FieldBoolean:
		schema["type"] = "boolean"
	case contract.FieldMultiSelect:
		schema["type"] = "array"
		item := map[string]any{"type": "string"}
		if len(field.Options) != 0 {
			item["enum"] = field.Options
		}
		schema["items"] = item
	default:
		schema["type"] = "string"
	}

	switch field.Type {
	case contract.FieldSelect:
		if len(field.Options) != 0 {
			schema["enum"] = field.Options
		}
	case contract.FieldDate:
		schema["format"] = "date"
	case contract.FieldTime:
		schema["format"] = "time"
	case contract.FieldDateTime:
		schema["format"] = "date-time"
	}

	if description := strings.TrimSpace(field.Description); description != "" {
		schema["description"] = description
	} else if title := strings.TrimSpace(field.Title); title != "" {
		schema["description"] = title
	}
	return schema
}

func validateProjectCommandArguments(command contract.CommandDescriptor, values map[string]any) error {
	fields := make(map[string]contract.FieldDescriptor, len(command.Parameters))
	for _, field := range command.Parameters {
		fields[field.Key] = field
		if field.Required {
			if _, ok := values[field.Key]; !ok {
				return fmt.Errorf("project command %q requires argument %q", command.ID, field.Key)
			}
		}
	}
	for key, value := range values {
		field, ok := fields[key]
		if !ok {
			return fmt.Errorf("project command %q does not accept argument %q", command.ID, key)
		}
		if err := validateProjectFieldValue(field, value); err != nil {
			return fmt.Errorf("project command %q argument %q: %w", command.ID, key, err)
		}
	}
	return nil
}

func validateProjectFieldValue(field contract.FieldDescriptor, value any) error {
	switch field.Type {
	case contract.FieldInteger:
		number, ok := value.(float64)
		if !ok || math.Trunc(number) != number {
			return fmt.Errorf("must be an integer")
		}
	case contract.FieldNumber:
		if _, ok := value.(float64); !ok {
			return fmt.Errorf("must be a number")
		}
	case contract.FieldBoolean:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("must be a boolean")
		}
	case contract.FieldMultiSelect:
		items, ok := value.([]any)
		if !ok {
			return fmt.Errorf("must be an array of strings")
		}
		for _, item := range items {
			text, ok := item.(string)
			if !ok {
				return fmt.Errorf("must be an array of strings")
			}
			if err := validateFieldOption(field, text); err != nil {
				return err
			}
		}
	default:
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("must be a string")
		}
		if field.Type == contract.FieldSelect {
			if err := validateFieldOption(field, text); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateFieldOption(field contract.FieldDescriptor, value string) error {
	if len(field.Options) == 0 {
		return nil
	}
	if slices.Contains(field.Options, value) {
		return nil
	}
	return fmt.Errorf("must be one of %s", strings.Join(field.Options, ", "))
}

func projectToolName(commandID string) string {
	var b strings.Builder
	b.WriteString("project_")
	for _, r := range strings.TrimSpace(commandID) {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return strings.TrimRight(b.String(), "_")
}
