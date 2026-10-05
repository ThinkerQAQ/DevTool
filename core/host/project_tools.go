package host

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
		if len(command.Parameters) != 0 {
			continue
		}
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
			"inputSchema": map[string]any{
				"type":                 "object",
				"properties":           map[string]any{},
				"additionalProperties": false,
			},
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
	if len(args) != 0 {
		var values map[string]any
		if err := json.Unmarshal(args, &values); err != nil {
			return nil, fmt.Errorf("decode project command arguments: %w", err)
		}
		if len(values) != 0 {
			return nil, fmt.Errorf("project command %q does not accept arguments", command.ID)
		}
	}

	var output bytes.Buffer
	if err := p.executor.Execute(ctx, command.ID, nil, &output); err != nil {
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
