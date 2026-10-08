package document

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
	documentcontext "github.com/thinkerqaq/devtool/sdk/documentcontext"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
	devtooltrace "github.com/thinkerqaq/devtool/sdk/trace"
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
		Requires:   []string{documentcontext.ServiceName},
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
			"Build structured document context or traverse a whole document with explicit stateless review coverage.",
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
				"references": map[string]any{
					"type":        "boolean",
					"description": "Include inbound links to the selected Markdown section with coverage and completeness metadata; requires section.",
				},
				"include_tables": map[string]any{
					"type":        "boolean",
					"description": "Include GFM tables as header/cell matrices with source lines (initial review call or focused inspection).",
				},
				"include_content": map[string]any{
					"type":        "boolean",
					"description": "Include the exact selected section source in the response.",
				},
				"review": map[string]any{
					"type":        "boolean",
					"description": "Build or continue a deterministic whole-document review traversal over top-level sections.",
				},
				"cursor": map[string]any{
					"type":        "string",
					"description": "Opaque continuation cursor returned by a previous review-mode call.",
				},
				"review_max_lines": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"description": "Optional maximum lines per structurally splittable review unit.",
				},
				"related": map[string]any{
					"type":        "boolean",
					"description": "Include deterministic cross-document relationship context from the configured relation provider.",
				},
				"relation_depth": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"description": "Optional relationship traversal depth.",
				},
				"relation_limit": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"description": "Optional maximum number of relation nodes returned.",
				},
			},
			[]string{"objective", "path"},
		),
	}, nil
}

func (e *Extension) CallTool(ctx context.Context, session agentsdk.Session, name string, args json.RawMessage) (result json.RawMessage, err error) {
	if strings.TrimSpace(name) != "document_context" {
		return nil, fmt.Errorf("unknown document capability tool %q", name)
	}
	if e.services == nil {
		return nil, fmt.Errorf("document capability service registry is unavailable")
	}

	var request documentcontext.BuildRequest
	if len(args) == 0 {
		args = json.RawMessage("{}")
	}
	if err := json.Unmarshal(args, &request); err != nil {
		return nil, fmt.Errorf("decode document capability arguments: %w", err)
	}
	request.Root = session.ProjectRoot
	request.Objective = strings.TrimSpace(request.Objective)
	request.Path = strings.TrimSpace(request.Path)
	if request.Objective == "" {
		return nil, fmt.Errorf("document_context objective is required")
	}
	if request.Path == "" {
		return nil, fmt.Errorf("document_context path is required")
	}

	invoker, ok := e.services.Service(documentcontext.ServiceName)
	if !ok {
		return nil, fmt.Errorf("service %q is not configured", documentcontext.ServiceName)
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	ctx, span := devtooltrace.Start(ctx, devtooltrace.Attributes{
		Name:         "document_context",
		Layer:        "capability",
		Tool:         "document_context",
		Service:      documentcontext.ServiceName,
		RequestBytes: len(payload),
	})
	defer func() { span.End(len(result), err) }()

	raw, err := invoker.Invoke(ctx, documentcontext.MethodBuild, payload)
	if err != nil {
		return nil, fmt.Errorf("build document context: %w", err)
	}
	return toolResult(decodeResult(raw))
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
