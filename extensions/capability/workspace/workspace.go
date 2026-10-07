package workspacecapability

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
	workspacecontract "github.com/thinkerqaq/devtool/sdk/workspace"
)

const ExtensionID = "capability.workspace"

type Extension struct {
	services extensioncontract.Registrar
}

func New() *Extension { return &Extension{} }

func (e *Extension) Descriptor() extensioncontract.Descriptor {
	return extensioncontract.Descriptor{
		ID:         ExtensionID,
		Kind:       extensioncontract.KindCapability,
		Requires:   []string{workspacecontract.ServiceName},
		AgentTools: true,
	}
}

func (e *Extension) Register(reg extensioncontract.Registrar) error {
	e.services = reg
	return reg.ProvideAgentTools(ExtensionID, e)
}

func (e *Extension) ListTools(context.Context, agentsdk.Session) ([]agentsdk.Tool, error) {
	return []agentsdk.Tool{
		tool(
			"workspace_create",
			"Create an isolated development workspace for a branch through the configured workspace provider.",
			map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Development branch/workspace name.",
				},
				"revision": map[string]any{
					"type":        "string",
					"description": "Optional base revision used when the branch does not exist.",
				},
			},
			[]string{"name"},
		),
		tool(
			"workspace_list",
			"List development workspaces for the current repository.",
			map[string]any{},
			nil,
		),
		tool(
			"workspace_inspect",
			"Inspect one development workspace by its stable workspace id.",
			map[string]any{
				"workspace_id": map[string]any{
					"type": "string",
				},
			},
			[]string{"workspace_id"},
		),
		tool(
			"workspace_remove",
			"Remove a clean non-primary development workspace through the configured workspace provider.",
			map[string]any{
				"workspace_id": map[string]any{
					"type": "string",
				},
			},
			[]string{"workspace_id"},
		),
	}, nil
}

func (e *Extension) CallTool(
	ctx context.Context,
	session agentsdk.Session,
	name string,
	args json.RawMessage,
) (json.RawMessage, error) {
	switch strings.TrimSpace(name) {
	case "workspace_create":
		var input struct {
			Name     string `json:"name"`
			Revision string `json:"revision,omitempty"`
		}
		if err := decodeArgs(args, &input); err != nil {
			return nil, err
		}
		input.Name = strings.TrimSpace(input.Name)
		if input.Name == "" {
			return nil, fmt.Errorf("workspace_create name is required")
		}
		raw, err := e.invoke(ctx, workspacecontract.MethodCreate, workspacecontract.CreateRequest{
			Root:     session.ProjectRoot,
			Name:     input.Name,
			Revision: strings.TrimSpace(input.Revision),
		})
		if err != nil {
			return nil, err
		}
		return toolResult(decodeResult(raw))

	case "workspace_list":
		raw, err := e.invoke(ctx, workspacecontract.MethodList, workspacecontract.ListRequest{
			Root: session.ProjectRoot,
		})
		if err != nil {
			return nil, err
		}
		return toolResult(decodeResult(raw))

	case "workspace_inspect":
		var input struct {
			WorkspaceID string `json:"workspace_id"`
		}
		if err := decodeArgs(args, &input); err != nil {
			return nil, err
		}
		input.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
		if input.WorkspaceID == "" {
			return nil, fmt.Errorf("workspace_inspect workspace_id is required")
		}
		raw, err := e.invoke(ctx, workspacecontract.MethodInspect, workspacecontract.InspectRequest{
			Root:        session.ProjectRoot,
			WorkspaceID: input.WorkspaceID,
		})
		if err != nil {
			return nil, err
		}
		return toolResult(decodeResult(raw))

	case "workspace_remove":
		var input struct {
			WorkspaceID string `json:"workspace_id"`
		}
		if err := decodeArgs(args, &input); err != nil {
			return nil, err
		}
		input.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
		if input.WorkspaceID == "" {
			return nil, fmt.Errorf("workspace_remove workspace_id is required")
		}
		raw, err := e.invoke(ctx, workspacecontract.MethodRemove, workspacecontract.RemoveRequest{
			Root:        session.ProjectRoot,
			WorkspaceID: input.WorkspaceID,
		})
		if err != nil {
			return nil, err
		}
		return toolResult(decodeResult(raw))

	default:
		return nil, fmt.Errorf("unknown workspace capability tool %q", name)
	}
}

func (e *Extension) invoke(
	ctx context.Context,
	method string,
	request any,
) (json.RawMessage, error) {
	if e.services == nil {
		return nil, fmt.Errorf("workspace capability service registry is unavailable")
	}
	invoker, ok := e.services.Service(workspacecontract.ServiceName)
	if !ok {
		return nil, fmt.Errorf("service %q is not configured", workspacecontract.ServiceName)
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	return invoker.Invoke(ctx, method, payload)
}

func tool(
	name string,
	description string,
	properties map[string]any,
	required []string,
) agentsdk.Tool {
	definition, _ := json.Marshal(map[string]any{
		"name":        name,
		"description": description,
		"inputSchema": map[string]any{
			"type":                 "object",
			"properties":           properties,
			"required":             required,
			"additionalProperties": false,
		},
	})
	return agentsdk.Tool{Name: name, Definition: definition}
}

func decodeArgs(raw json.RawMessage, out any) error {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode workspace capability arguments: %w", err)
	}
	return nil
}

func decodeResult(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return string(raw)
	}
	return value
}

func toolResult(value any) (json.RawMessage, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"content": []map[string]string{{
			"type": "text",
			"text": string(raw),
		}},
	})
}
