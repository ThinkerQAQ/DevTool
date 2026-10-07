package code

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
	"github.com/thinkerqaq/devtool/sdk/codecontext"
	"github.com/thinkerqaq/devtool/sdk/codeintelligence"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
	devtooltrace "github.com/thinkerqaq/devtool/sdk/trace"
)

const ExtensionID = "capability.code"

type Extension struct {
	services extensioncontract.Registrar
}

func New() *Extension { return &Extension{} }

func (e *Extension) Descriptor() extensioncontract.Descriptor {
	return extensioncontract.Descriptor{
		ID:         ExtensionID,
		Kind:       extensioncontract.KindCapability,
		Requires:   []string{codecontext.ServiceName},
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
			"code_context",
			"Build project code context for an engineering objective through the configured code-context service.",
			map[string]any{
				"objective": map[string]any{
					"type":        "string",
					"description": "What the agent needs to understand or change.",
				},
				"symbol": map[string]any{
					"type":        "string",
					"description": "Optional symbol hint used to enrich context.",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Optional project-relative file or directory scope.",
				},
				"include_body": map[string]any{
					"type":        "boolean",
					"description": "Include symbol bodies when realtime intelligence supports it.",
				},
				"limit": map[string]any{
					"type":    "integer",
					"minimum": 1,
					"maximum": 100,
				},
			},
			[]string{"objective"},
		),
	}, nil
}

func (e *Extension) CallTool(ctx context.Context, session agentsdk.Session, name string, args json.RawMessage) (result json.RawMessage, err error) {
	if strings.TrimSpace(name) != "code_context" {
		return nil, fmt.Errorf("unknown code capability tool %q", name)
	}

	var input struct {
		Objective   string `json:"objective"`
		Symbol      string `json:"symbol,omitempty"`
		Path        string `json:"path,omitempty"`
		IncludeBody bool   `json:"include_body,omitempty"`
		Limit       int    `json:"limit,omitempty"`
	}
	if err := decodeArgs(args, &input); err != nil {
		return nil, err
	}
	input.Objective = strings.TrimSpace(input.Objective)
	input.Symbol = strings.TrimSpace(input.Symbol)
	input.Path = strings.TrimSpace(input.Path)
	if input.Objective == "" {
		return nil, fmt.Errorf("code_context objective is required")
	}
	if input.Limit < 0 || input.Limit > 100 {
		return nil, fmt.Errorf("code_context limit must be between 1 and 100")
	}
	if e.services == nil {
		return nil, fmt.Errorf("code capability service registry is unavailable")
	}
	invoker, ok := e.services.Service(codecontext.ServiceName)
	if !ok {
		return nil, fmt.Errorf("service %q is not configured", codecontext.ServiceName)
	}

	request := codecontext.BuildRequest{
		Workspace: codeintelligence.Workspace{
			Root:       session.ProjectRoot,
			Workspaces: append([]string(nil), session.Workspaces...),
		},
		Objective:   input.Objective,
		Symbol:      input.Symbol,
		Path:        input.Path,
		IncludeBody: input.IncludeBody,
		Limit:       input.Limit,
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	ctx, span := devtooltrace.Start(ctx, devtooltrace.Attributes{
		Name:         "code_context",
		Layer:        "capability",
		Tool:         "code_context",
		Service:      codecontext.ServiceName,
		RequestBytes: len(payload),
	})
	defer func() { span.End(len(result), err) }()

	raw, err := invoker.Invoke(ctx, codecontext.MethodBuild, payload)
	if err != nil {
		return nil, fmt.Errorf("build code context: %w", err)
	}
	return toolResult(decodeResult(raw))
}

func tool(name, description string, properties map[string]any, required []string) agentsdk.Tool {
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
		raw = json.RawMessage("{}")
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode code capability arguments: %w", err)
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
