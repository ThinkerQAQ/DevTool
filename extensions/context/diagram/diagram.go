package diagram

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	diagram "github.com/thinkerqaq/devtool/sdk/diagram"
	document "github.com/thinkerqaq/devtool/sdk/document"
	extension "github.com/thinkerqaq/devtool/sdk/extension"
	service "github.com/thinkerqaq/devtool/sdk/service"
)

const ExtensionID = "context.diagram.composite"

type Extension struct{ registry extension.Registrar }

func New() *Extension { return &Extension{} }
func (e *Extension) Descriptor() extension.Descriptor {
	return extension.Descriptor{ID: ExtensionID, Kind: extension.KindDocumentIntelligence, Provides: []string{diagram.ContextServiceName}, Requires: []string{document.ServiceName, diagram.RenderServiceName}}
}
func (e *Extension) Register(r extension.Registrar) error {
	e.registry = r
	return r.ProvideService(diagram.ContextServiceName, ExtensionID, service.Func(e.Invoke))
}

type entry struct {
	Index        int    `json:"index"`
	Language     string `json:"language"`
	StartLine    int    `json:"start_line"`
	EndLine      int    `json:"end_line"`
	SHA256       string `json:"sha256"`
	Status       string `json:"status"`
	Source       string `json:"source,omitempty"`
	Renderer     string `json:"renderer,omitempty"`
	Diagnostic   string `json:"diagnostic,omitempty"`
	ArtifactPath string `json:"artifact_path,omitempty"`
}

func (e *Extension) Invoke(ctx context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
	if method != diagram.ContextMethod {
		return nil, fmt.Errorf("%s does not support %q", ExtensionID, method)
	}
	var req diagram.ContextRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Root) == "" || strings.TrimSpace(req.Path) == "" || strings.TrimSpace(req.Objective) == "" {
		return nil, fmt.Errorf("diagram context requires root, path and objective")
	}
	if req.Index < 0 {
		return nil, fmt.Errorf("index must be positive")
	}
	svc, ok := e.registry.Service(document.ServiceName)
	if !ok {
		return nil, fmt.Errorf("document-structure service is not configured")
	}
	data, _ := json.Marshal(document.InspectRequest{Root: req.Root, Path: req.Path, IncludeDiagrams: true})
	data, err := svc.Invoke(ctx, document.MethodInspect, data)
	if err != nil {
		return nil, fmt.Errorf("inspect document diagrams: %w", err)
	}
	var inspected document.InspectResponse
	if err := json.Unmarshal(data, &inspected); err != nil {
		return nil, err
	}
	if len(inspected.Diagrams) > 200 {
		return nil, fmt.Errorf("document contains %d diagrams; maximum is 200", len(inspected.Diagrams))
	}
	var renderer service.Invoker
	if req.Render {
		renderer, ok = e.registry.Service(diagram.RenderServiceName)
		if !ok {
			return nil, fmt.Errorf("diagram-render service is not configured")
		}
	}
	results := make([]entry, 0, len(inspected.Diagrams))
	counts := map[string]int{"found": 0, "rendered": 0, "failed": 0, "unavailable": 0, "unsupported": 0}
	for _, item := range inspected.Diagrams {
		if req.Index > 0 && req.Index != item.Index {
			continue
		}
		sum := sha256.Sum256([]byte(item.Source))
		r := entry{Index: item.Index, Language: item.Language, StartLine: item.StartLine, EndLine: item.EndLine, SHA256: hex.EncodeToString(sum[:]), Status: "found"}
		if req.IncludeSource {
			r.Source = item.Source
		}
		if req.Render {
			in, _ := json.Marshal(diagram.RenderRequest{Language: item.Language, Source: item.Source})
			out, err := renderer.Invoke(ctx, diagram.RenderMethod, in)
			if err != nil {
				r.Status = "failed"
				r.Diagnostic = err.Error()
			} else {
				var rendered diagram.RenderResult
				if err := json.Unmarshal(out, &rendered); err != nil {
					return nil, err
				}
				r.Status = rendered.Status
				r.Renderer = rendered.Renderer
				r.ArtifactPath = rendered.ArtifactPath
				r.Diagnostic = rendered.Diagnostic
			}
		}
		counts[r.Status]++
		results = append(results, r)
	}
	if req.Index > 0 && len(results) == 0 {
		return nil, fmt.Errorf("diagram #%d does not exist", req.Index)
	}
	return json.Marshal(map[string]any{"objective": req.Objective, "path": inspected.Path, "total_diagrams": len(inspected.Diagrams), "reviewed_diagrams": len(results), "render_requested": req.Render, "results": results, "summary": counts})
}
