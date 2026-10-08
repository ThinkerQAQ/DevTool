package diagram

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	agent "github.com/thinkerqaq/devtool/sdk/agent"
	diagram "github.com/thinkerqaq/devtool/sdk/diagram"
	extension "github.com/thinkerqaq/devtool/sdk/extension"
)

const ExtensionID = "capability.diagram"

type Extension struct{ registry extension.Registrar }

func New() *Extension { return &Extension{} }
func (e *Extension) Descriptor() extension.Descriptor {
	return extension.Descriptor{ID: ExtensionID, Kind: extension.KindCapability, Requires: []string{diagram.ContextServiceName}, AgentTools: true}
}
func (e *Extension) Register(r extension.Registrar) error {
	e.registry = r
	return r.ProvideAgentTools(ExtensionID, e)
}
func (e *Extension) ListTools(context.Context, agent.Session) ([]agent.Tool, error) {
	definition, _ := json.Marshal(map[string]any{
		"name":        "diagram_context",
		"description": "Inspect Markdown Mermaid/PlantUML diagrams and optionally render to SVG through configured tools, distinguishing rendered, failed and unavailable.",
		"inputSchema": map[string]any{
			"type": "object", "additionalProperties": false,
			"properties": map[string]any{
				"objective":      map[string]any{"type": "string"},
				"path":           map[string]any{"type": "string", "description": "Project-relative Markdown file."},
				"analyze":        map[string]any{"type": "boolean", "description": "Enrich each diagram with structural graph, Mermaid language symbols, scoped diagnostics and containing Markdown section; provider controlled."},
				"render":         map[string]any{"type": "boolean", "description": "Validate by renderer and generate SVG artifacts."},
				"include_source": map[string]any{"type": "boolean", "description": "Return exact diagram sources."},
				"index":          map[string]any{"type": "integer", "minimum": 1, "description": "Optional 1-based diagram index."},
			},
			"required": []string{"objective", "path"},
		},
	})
	return []agent.Tool{{Name: "diagram_context", Definition: definition}}, nil
}
func (e *Extension) CallTool(ctx context.Context, session agent.Session, name string, args json.RawMessage) (json.RawMessage, error) {
	if name != "diagram_context" {
		return nil, fmt.Errorf("unknown diagram tool %q", name)
	}
	var req diagram.ContextRequest
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, err
	}
	req.Root = session.ProjectRoot
	req.Path = strings.TrimSpace(req.Path)
	req.Objective = strings.TrimSpace(req.Objective)
	if req.Path == "" || req.Objective == "" {
		return nil, fmt.Errorf("diagram_context requires objective and path")
	}
	if req.Index < 0 {
		return nil, fmt.Errorf("diagram index must be positive")
	}
	svc, ok := e.registry.Service(diagram.ContextServiceName)
	if !ok {
		return nil, fmt.Errorf("diagram-context service is not configured")
	}
	in, _ := json.Marshal(req)
	out, err := svc.Invoke(ctx, diagram.ContextMethod, in)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"content": []map[string]string{{"type": "text", "text": string(out)}}})
}
