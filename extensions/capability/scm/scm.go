package scm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
	scmcontract "github.com/thinkerqaq/devtool/sdk/scm"
)

const ExtensionID = "capability.scm"

type Extension struct {
	services extensioncontract.Registrar
}

func New() *Extension { return &Extension{} }

func (e *Extension) Descriptor() extensioncontract.Descriptor {
	return extensioncontract.Descriptor{
		ID:         ExtensionID,
		Kind:       extensioncontract.KindCapability,
		Requires:   []string{scmcontract.ServiceName},
		AgentTools: true,
	}
}

func (e *Extension) Register(reg extensioncontract.Registrar) error {
	e.services = reg
	return reg.ProvideAgentTools(ExtensionID, e)
}

func (e *Extension) ListTools(context.Context, agentsdk.Session) ([]agentsdk.Tool, error) {
	return []agentsdk.Tool{
		tool("scm_checkpoint", "Save the current coherent source change and push the branch through the configured SCM provider.", map[string]any{
			"message": map[string]any{"type": "string", "description": "Commit message used when the worktree has changes."},
		}, []string{"message"}),
		tool("scm_publish", "Publish the current clean branch through the configured SCM provider and optionally merge it.", map[string]any{
			"base":  map[string]any{"type": "string", "description": "Optional target branch. When omitted, the SCM provider resolves the repository default branch."},
			"title": map[string]any{"type": "string"},
			"body":  map[string]any{"type": "string"},
			"merge": map[string]any{"type": "boolean", "default": false},
		}, nil),
	}, nil
}

func (e *Extension) CallTool(ctx context.Context, session agentsdk.Session, name string, args json.RawMessage) (json.RawMessage, error) {
	switch strings.TrimSpace(name) {
	case "scm_checkpoint":
		var input struct {
			Message string `json:"message"`
		}
		if err := decodeArgs(args, &input); err != nil {
			return nil, err
		}
		message := strings.TrimSpace(input.Message)
		if message == "" {
			return nil, fmt.Errorf("scm_checkpoint message is required")
		}

		statusRaw, err := e.invoke(ctx, scmcontract.MethodStatus, scmcontract.Request{Root: session.ProjectRoot})
		if err != nil {
			return nil, err
		}
		var status scmcontract.StatusResponse
		if err := json.Unmarshal(statusRaw, &status); err != nil {
			return nil, err
		}

		var commit any
		if !status.Clean {
			commitRaw, err := e.invoke(ctx, scmcontract.MethodCommit, scmcontract.CommitRequest{Root: session.ProjectRoot, Message: message})
			if err != nil {
				return nil, fmt.Errorf("create SCM checkpoint commit: %w", err)
			}
			commit = decodeResult(commitRaw)
		}

		pushRaw, err := e.invoke(ctx, scmcontract.MethodPush, scmcontract.PushRequest{Root: session.ProjectRoot})
		if err != nil {
			return nil, fmt.Errorf("push SCM checkpoint: %w", err)
		}
		var push scmcontract.PushResponse
		if err := json.Unmarshal(pushRaw, &push); err != nil {
			return nil, err
		}
		result := map[string]any{"status": "pushed", "push": decodeResult(pushRaw)}
		if commit != nil {
			result["commit"] = commit
		}
		if push.Authorization != nil {
			result["status"] = "authorization_required"
			result["authorization"] = push.Authorization
		}
		return toolResult(result)

	case "scm_publish":
		var input struct {
			Base  string `json:"base,omitempty"`
			Title string `json:"title,omitempty"`
			Body  string `json:"body,omitempty"`
			Merge bool   `json:"merge,omitempty"`
		}
		if err := decodeArgs(args, &input); err != nil {
			return nil, err
		}
		raw, err := e.invoke(ctx, scmcontract.MethodPublish, scmcontract.PublishRequest{
			Root: session.ProjectRoot, Base: input.Base, Title: input.Title, Body: input.Body, Merge: input.Merge,
		})
		if err != nil {
			return nil, fmt.Errorf("publish SCM branch: %w", err)
		}
		var response scmcontract.PublishResponse
		if err := json.Unmarshal(raw, &response); err != nil {
			return nil, err
		}
		if response.Authorization != nil {
			return toolResult(map[string]any{"status": "authorization_required", "authorization": response.Authorization})
		}
		return toolResult(map[string]any{"status": "published", "result": decodeResult(raw)})
	default:
		return nil, fmt.Errorf("unknown SCM capability tool %q", name)
	}
}

func (e *Extension) invoke(ctx context.Context, method string, request any) (json.RawMessage, error) {
	invoker, ok := e.services.Service(scmcontract.ServiceName)
	if !ok {
		return nil, fmt.Errorf("service %q is not configured", scmcontract.ServiceName)
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	return invoker.Invoke(ctx, method, payload)
}

func tool(name, description string, properties map[string]any, required []string) agentsdk.Tool {
	raw, _ := json.Marshal(map[string]any{
		"name": name, "description": description,
		"inputSchema": map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false},
	})
	return agentsdk.Tool{Name: name, Definition: raw}
}

func decodeArgs(raw json.RawMessage, out any) error {
	if len(raw) == 0 {
		raw = json.RawMessage("{}")
	}
	return json.Unmarshal(raw, out)
}

func decodeResult(raw json.RawMessage) any {
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
	return json.Marshal(map[string]any{"content": []map[string]string{{"type": "text", "text": string(raw)}}})
}
