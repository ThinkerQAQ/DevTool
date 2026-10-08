package merman

import (
	"context"
	"encoding/json"
	"fmt"
	diagram "github.com/thinkerqaq/devtool/sdk/diagram"
	extension "github.com/thinkerqaq/devtool/sdk/extension"
	service "github.com/thinkerqaq/devtool/sdk/service"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const ExtensionID = "diagram.intelligence.merman"

type Extension struct {
	cli, lsp string
	timeout  time.Duration
}

func New() *Extension {
	return &Extension{cli: "merman-cli", lsp: "merman-lsp", timeout: 40 * time.Second}
}
func (e *Extension) Descriptor() extension.Descriptor {
	return extension.Descriptor{ID: ExtensionID, Kind: extension.KindDocumentIntelligence, Provides: []string{diagram.IntelligenceServiceName}}
}
func (e *Extension) Configure(c map[string]any) error {
	for key, dst := range map[string]*string{"cli_bin": &e.cli, "lsp_bin": &e.lsp} {
		if v, ok := c[key]; ok {
			s, valid := v.(string)
			if !valid || strings.TrimSpace(s) == "" {
				return fmt.Errorf("%s requires nonempty string", key)
			}
			*dst = s
		}
	}
	if v, ok := c["timeout_seconds"]; ok {
		var n float64
		switch x := v.(type) {
		case int:
			n = float64(x)
		case int64:
			n = float64(x)
		case float64:
			n = x
		default:
			return fmt.Errorf("timeout must be numeric")
		}
		if n < 1 || n > 120 || n != float64(int(n)) {
			return fmt.Errorf("timeout_seconds must be integer 1..120")
		}
		e.timeout = time.Duration(n) * time.Second
	}
	return nil
}
func (e *Extension) Register(r extension.Registrar) error {
	return r.ProvideService(diagram.IntelligenceServiceName, ExtensionID, service.Func(e.Invoke))
}
func (e *Extension) Invoke(ctx context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
	if method != diagram.IntelligenceMethod {
		return nil, fmt.Errorf("unsupported diagram method %q", method)
	}
	var req diagram.IntelligenceRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	out, err := e.analyze(ctx, req)
	if err != nil {
		return nil, err
	}
	return json.Marshal(out)
}
func (e *Extension) analyze(ctx context.Context, req diagram.IntelligenceRequest) (diagram.IntelligenceResponse, error) {
	if req.Root == "" || req.Path == "" || len(req.Diagrams) > 200 {
		return diagram.IntelligenceResponse{}, fmt.Errorf("invalid project, document, or diagram count")
	}
	root, err := filepath.EvalSymlinks(req.Root)
	if err != nil {
		return diagram.IntelligenceResponse{}, err
	}
	path := req.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return diagram.IntelligenceResponse{}, err
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return diagram.IntelligenceResponse{}, fmt.Errorf("diagram source escapes project root")
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".md" && ext != ".markdown" && ext != ".mdx" {
		return diagram.IntelligenceResponse{}, fmt.Errorf("diagram intelligence requires Markdown")
	}
	info, err := os.Stat(path)
	if err != nil {
		return diagram.IntelligenceResponse{}, err
	}
	if info.Size() > 4<<20 {
		return diagram.IntelligenceResponse{}, fmt.Errorf("Markdown exceeds 4 MiB")
	}
	for _, it := range req.Diagrams {
		if it.Index <= 0 || it.StartLine < 1 || it.EndLine < it.StartLine || len(it.Source) > 256<<10 {
			return diagram.IntelligenceResponse{}, fmt.Errorf("invalid diagram bounds")
		}
	}
	out := diagram.IntelligenceResponse{Provider: ExtensionID, Results: []diagram.IntelligenceResult{}, DiagnosticsStatus: "not_reported"}
	source, err := os.ReadFile(path)
	if err != nil {
		return out, err
	}
	lsp, err := e.language(ctx, root, path, source)
	if err == nil {
		out.DiagnosticsStatus = "reported"
	} else {
		out.Detail = err.Error()
	}
	bin, binaryErr := exec.LookPath(e.cli)
	for _, it := range req.Diagrams {
		r := diagram.IntelligenceResult{Index: it.Index, Status: "unsupported", Symbols: []diagram.DiagramSymbol{}, Diagnostics: []diagram.DiagramDiagnostic{}}
		if it.Language != "mermaid" {
			r.Detail = "only Mermaid has semantic Merman support"
			out.Results = append(out.Results, r)
			continue
		}
		if binaryErr != nil {
			r.Status = "unavailable"
			r.Detail = "merman-cli missing; install executable explicitly"
			out.Results = append(out.Results, r)
			continue
		}
		graph, err := e.semantic(ctx, bin, it.Source)
		if err != nil {
			r.Status = "failed"
			r.Detail = err.Error()
			out.Results = append(out.Results, r)
			continue
		}
		r.Graph = graph
		if lsp != nil {
			if lsp.Truncated {
				r.Graph.Truncated = true
			}
			for _, sym := range lsp.Symbols {
				if sym.Line >= it.StartLine && sym.Line <= it.EndLine {
					if len(r.Symbols) >= 160 {
						r.Graph.Truncated = true
						break
					}
					r.Symbols = append(r.Symbols, sym)
				}
			}
			for _, diag := range lsp.Diagnostics {
				if diag.Line >= it.StartLine && diag.Line <= it.EndLine {
					if len(r.Diagnostics) >= 80 {
						r.Graph.Truncated = true
						break
					}
					r.Diagnostics = append(r.Diagnostics, diag)
				}
			}
			locations := map[string]int{}
			for _, sym := range r.Symbols {
				if locations[sym.Name] == 0 {
					locations[sym.Name] = sym.Line
				}
			}
			for i := range r.Graph.Nodes {
				r.Graph.Nodes[i].Line = locations[r.Graph.Nodes[i].ID]
			}
		}
		r.Complete = lsp != nil && !r.Graph.Truncated
		r.Status = "ok"
		if !r.Complete {
			r.Status = "partial"
			r.Detail = "semantic graph available, but LSP evidence is missing or truncated"
		}
		out.Results = append(out.Results, r)
	}
	return out, nil
}
