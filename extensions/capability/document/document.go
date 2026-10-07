package document

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
	documentcontract "github.com/thinkerqaq/devtool/sdk/document"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

const ExtensionID = "capability.document"

type Extension struct {
	services extensioncontract.Registrar
}

func New() *Extension { return &Extension{} }

func (e *Extension) Descriptor() extensioncontract.Descriptor {
	return extensioncontract.Descriptor{
		ID:         ExtensionID,
		Kind:       extensioncontract.KindCapability,
		Requires:   []string{documentcontract.ServiceName},
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
			"document_context",
			"Build structured context for a document review objective without loading the whole document by default.",
			map[string]any{
				"objective": map[string]any{
					"type":        "string",
					"description": "What the agent needs to understand or review.",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Project-relative document path.",
				},
				"section": map[string]any{
					"type":        "string",
					"description": "Optional exact section title or numbered section key such as 1.3.",
				},
				"include_content": map[string]any{
					"type":        "boolean",
					"description": "Include the exact selected section source in the response.",
				},
			},
			[]string{"objective", "path"},
		),
	}, nil
}

func (e *Extension) CallTool(ctx context.Context, session agentsdk.Session, name string, args json.RawMessage) (json.RawMessage, error) {
	if strings.TrimSpace(name) != "document_context" {
		return nil, fmt.Errorf("unknown document capability tool %q", name)
	}
	if e.services == nil {
		return nil, fmt.Errorf("document capability service registry is unavailable")
	}

	var input struct {
		Objective      string `json:"objective"`
		Path           string `json:"path"`
		Section        string `json:"section,omitempty"`
		IncludeContent bool   `json:"include_content,omitempty"`
	}
	if err := decodeArgs(args, &input); err != nil {
		return nil, err
	}
	objective := strings.TrimSpace(input.Objective)
	if objective == "" {
		return nil, fmt.Errorf("document_context objective is required")
	}
	path := strings.TrimSpace(input.Path)
	if path == "" {
		return nil, fmt.Errorf("document_context path is required")
	}

	invoker, ok := e.services.Service(documentcontract.ServiceName)
	if !ok {
		return nil, fmt.Errorf("service %q is not configured", documentcontract.ServiceName)
	}
	payload, err := json.Marshal(documentcontract.InspectRequest{
		Root:           session.ProjectRoot,
		Path:           path,
		Section:        strings.TrimSpace(input.Section),
		IncludeContent: input.IncludeContent,
	})
	if err != nil {
		return nil, err
	}
	raw, err := invoker.Invoke(ctx, documentcontract.MethodInspect, payload)
	if err != nil {
		return nil, fmt.Errorf("build document context: %w", err)
	}

	return toolResult(map[string]any{
		"objective": objective,
		"document":  decodeResult(raw),
	})
}

func tool(name, description string, properties map[string]any, required []string) agentsdk.Tool {
	raw, _ := json.Marshal(map[string]any{
		"name":        name,
		"description": description,
		"inputSchema": map[string]any{
			"type":                 "object",
			"properties":           properties,
			"required":             required,
			"additionalProperties": false,
		},
	})
	return agentsdk.Tool{Name: name, Definition: raw}
}

func decodeArgs(raw json.RawMessage, out any) error {
	if len(raw) == 0 {
		raw = json.RawMessage("{}")
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode document capability arguments: %w", err)
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
