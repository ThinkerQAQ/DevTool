package marksman

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	doc "github.com/thinkerqaq/devtool/sdk/documentrealtime"
	extension "github.com/thinkerqaq/devtool/sdk/extension"
	service "github.com/thinkerqaq/devtool/sdk/service"
)

const ExtensionID = "document.realtime.marksman"
const maxDocument = 4 << 20

type Extension struct {
	binary         string
	timeout        time.Duration
	workspaceRoots []string
}

func New() *Extension { return &Extension{binary: "marksman", timeout: 30 * time.Second} }
func (e *Extension) Descriptor() extension.Descriptor {
	return extension.Descriptor{ID: ExtensionID, Kind: extension.KindDocumentIntelligence, Provides: []string{doc.ServiceName}}
}
func (e *Extension) Configure(s map[string]any) error {
	if raw, exists := s["workspace_roots"]; exists {
		entries, ok := raw.([]any)
		if !ok {
			return fmt.Errorf("workspace_roots must be a list of project-relative directories")
		}
		e.workspaceRoots = nil
		for _, entry := range entries {
			dir, ok := entry.(string)
			if !ok || strings.TrimSpace(dir) == "" || filepath.IsAbs(dir) ||
				filepath.Clean(dir) == ".." || strings.HasPrefix(filepath.Clean(dir), ".."+string(filepath.Separator)) {
				return fmt.Errorf("workspace_roots must contain project-relative paths")
			}
			e.workspaceRoots = append(e.workspaceRoots, filepath.Clean(dir))
		}
	}

	if v, ok := s["binary"]; ok {
		name, valid := v.(string)
		if !valid || strings.TrimSpace(name) == "" {
			return fmt.Errorf("marksman binary must be a non-empty string")
		}
		e.binary = name
	}
	if v, ok := s["timeout_seconds"]; ok {
		var n float64
		switch x := v.(type) {
		case int64:
			n = float64(x)
		case int:
			n = float64(x)
		case float64:
			n = x
		default:
			return fmt.Errorf("timeout_seconds must be numeric")
		}
		if n < 1 || n > 120 || n != float64(int(n)) {
			return fmt.Errorf("timeout_seconds must be an integer between 1 and 120")
		}
		e.timeout = time.Duration(int(n)) * time.Second
	}
	return nil
}
func (e *Extension) Register(r extension.Registrar) error {
	return r.ProvideService(doc.ServiceName, ExtensionID, service.Func(e.Invoke))
}
func (e *Extension) Invoke(ctx context.Context, method string, raw json.RawMessage) (json.RawMessage, error) {
	if method != doc.MethodAnalyze {
		return nil, fmt.Errorf("%s does not support %q", ExtensionID, method)
	}
	var request doc.AnalyzeRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		return nil, err
	}
	result, err := e.analyze(ctx, request)
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}
func unavailable(detail string) doc.AnalyzeResponse {
	return doc.AnalyzeResponse{Provider: ExtensionID, Status: "unavailable", Detail: detail,
		Symbols: []doc.Symbol{}, Diagnostics: []doc.Diagnostic{}, DiagnosticsStatus: "not_reported",
		Definitions: []doc.Location{}, References: []doc.Location{}}
}
func (e *Extension) analyze(ctx context.Context, in doc.AnalyzeRequest) (doc.AnalyzeResponse, error) {
	if in.Root == "" || in.Path == "" {
		return doc.AnalyzeResponse{}, fmt.Errorf("document realtime requires root and path")
	}
	if in.Line < 0 || in.Column < 0 || (in.Line > 0) != (in.Column > 0) {
		return doc.AnalyzeResponse{}, fmt.Errorf("line and column must be positive together")
	}
	if in.IncludeReferences && in.Line == 0 {
		return doc.AnalyzeResponse{}, fmt.Errorf("references require line and column")
	}
	root, err := filepath.EvalSymlinks(in.Root)
	if err != nil {
		return doc.AnalyzeResponse{}, fmt.Errorf("resolve project root: %w", err)
	}
	path := in.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return doc.AnalyzeResponse{}, fmt.Errorf("resolve document: %w", err)
	}
	if !within(root, path) {
		return doc.AnalyzeResponse{}, fmt.Errorf("document escapes project root")
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".md" && ext != ".markdown" {
		return doc.AnalyzeResponse{}, fmt.Errorf("unsupported document extension %s", ext)
	}
	stat, err := os.Stat(path)
	if err != nil {
		return doc.AnalyzeResponse{}, err
	}
	if stat.Size() > maxDocument {
		return doc.AnalyzeResponse{}, fmt.Errorf("document exceeds 4 MiB realtime limit")
	}
	workspaceRoot := root
	for _, candidate := range e.workspaceRoots {
		resolved, err := filepath.EvalSymlinks(filepath.Join(root, candidate))
		if err != nil || !within(root, resolved) || !within(resolved, path) {
			continue
		}
		if len(resolved) > len(workspaceRoot) {
			workspaceRoot = resolved
		}
	}
	scope, _ := filepath.Rel(root, workspaceRoot)
	source, err := os.ReadFile(path)
	if err != nil {
		return doc.AnalyzeResponse{}, err
	}
	bin, err := exec.LookPath(e.binary)
	if err != nil {
		return unavailable("Marksman executable unavailable; configure document-realtime binary; no automatic installation"), nil
	}
	run, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()
	cmd := exec.CommandContext(run, bin, "server")
	cmd.Dir = workspaceRoot
	cmd.Stderr = io.Discard
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return doc.AnalyzeResponse{}, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return doc.AnalyzeResponse{}, err
	}
	if err := cmd.Start(); err != nil {
		return unavailable("Marksman could not start: " + err.Error()), nil
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	client := &stdioClient{stdin: stdin, out: bufio.NewReader(stdout)}
	rootURI := (&url.URL{Scheme: "file", Path: filepath.ToSlash(workspaceRoot)}).String()
	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
	init := map[string]any{"processId": nil, "rootUri": rootURI,
		"capabilities": map[string]any{"textDocument": map[string]any{
			"documentSymbol": map[string]any{"hierarchicalDocumentSymbolSupport": true},
			"definition":     map[string]any{}, "references": map[string]any{},
			"publishDiagnostics": map[string]any{}},
			"workspace": map[string]any{"workspaceFolders": true}},
		"workspaceFolders": []map[string]any{{"uri": rootURI, "name": filepath.Base(workspaceRoot)}}}
	if _, err := client.request(1, "initialize", init); err != nil {
		return unavailable("Marksman initialize failed: " + err.Error()), nil
	}
	if err := client.notify("initialized", map[string]any{}); err != nil {
		return unavailable(err.Error()), nil
	}
	if err := client.notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{
		"uri": uri, "languageId": "markdown", "version": 1, "text": string(source)}}); err != nil {
		return unavailable(err.Error()), nil
	}
	rawSymbols, err := client.request(2, "textDocument/documentSymbol", map[string]any{"textDocument": map[string]string{"uri": uri}})
	if err != nil {
		return unavailable("Marksman documentSymbol failed: " + err.Error()), nil
	}
	result := doc.AnalyzeResponse{Status: "ok", Provider: ExtensionID, WorkspaceScope: filepath.ToSlash(scope),
		Symbols: parseSymbols(rawSymbols), Diagnostics: []doc.Diagnostic{}, DiagnosticsStatus: "not_reported",
		Definitions: []doc.Location{}, References: []doc.Location{}}
	if in.Line > 0 {
		params := map[string]any{"textDocument": map[string]string{"uri": uri},
			"position": map[string]int{"line": in.Line - 1, "character": in.Column - 1}}
		def, err := client.request(3, "textDocument/definition", params)
		if err != nil {
			return unavailable("Marksman definition failed: " + err.Error()), nil
		}
		result.Definitions = parseLocations(def, root)
		if in.IncludeReferences {
			params["context"] = map[string]bool{"includeDeclaration": true}
			refs, err := client.request(4, "textDocument/references", params)
			if err != nil {
				return unavailable("Marksman references failed: " + err.Error()), nil
			}
			result.References = parseLocations(refs, root)
		}
	}
	if client.diagnosticsReported {
		result.DiagnosticsStatus = "reported"
		result.Diagnostics = parseDiagnostics(client.diagnostics)
	}
	return result, nil
}
func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

type lspRange struct {
	Start struct {
		Line      int `json:"line"`
		Character int `json:"character"`
	} `json:"start"`
	End struct {
		Line      int `json:"line"`
		Character int `json:"character"`
	} `json:"end"`
}

func normalizedRange(in lspRange) doc.Range {
	return doc.Range{Start: doc.Position{Line: in.Start.Line + 1, Column: in.Start.Character + 1},
		End: doc.Position{Line: in.End.Line + 1, Column: in.End.Character + 1}}
}

type lspSymbol struct {
	Name     string   `json:"name"`
	Range    lspRange `json:"range"`
	Location *struct {
		Range lspRange `json:"range"`
	} `json:"location"`
	Children []lspSymbol `json:"children"`
}

func parseSymbols(raw json.RawMessage) []doc.Symbol {
	var values []lspSymbol
	_ = json.Unmarshal(raw, &values)
	out := make([]doc.Symbol, 0, len(values))
	var visit func([]lspSymbol, int)
	visit = func(items []lspSymbol, level int) {
		for _, item := range items {
			if len(out) >= 500 {
				return
			}
			r := item.Range
			if item.Location != nil {
				r = item.Location.Range
			}
			out = append(out, doc.Symbol{Name: item.Name, Level: level, Range: normalizedRange(r)})
			visit(item.Children, level+1)
		}
	}
	visit(values, 1)
	return out
}
func parseLocations(raw json.RawMessage, root string) []doc.Location {
	var items []struct {
		URI   string   `json:"uri"`
		Range lspRange `json:"range"`
	}
	if len(raw) == 0 || string(raw) == "null" {
		return []doc.Location{}
	}
	if raw[0] == '[' {
		_ = json.Unmarshal(raw, &items)
	} else {
		var item struct {
			URI   string   `json:"uri"`
			Range lspRange `json:"range"`
		}
		if json.Unmarshal(raw, &item) == nil {
			items = append(items, item)
		}
	}
	out := make([]doc.Location, 0, len(items))
	for _, item := range items {
		if len(out) >= 300 {
			break
		}
		uri, err := url.Parse(item.URI)
		if err != nil || uri.Scheme != "file" {
			continue
		}
		name := filepath.FromSlash(uri.Path)
		if !within(root, name) {
			continue
		}
		rel, err := filepath.Rel(root, name)
		if err != nil {
			continue
		}
		out = append(out, doc.Location{Path: filepath.ToSlash(rel), Range: normalizedRange(item.Range)})
	}
	return out
}
func parseDiagnostics(raw json.RawMessage) []doc.Diagnostic {
	var values []struct {
		Message  string   `json:"message"`
		Severity int      `json:"severity"`
		Range    lspRange `json:"range"`
	}
	_ = json.Unmarshal(raw, &values)
	out := make([]doc.Diagnostic, 0, len(values))
	for _, v := range values {
		if len(out) >= 200 {
			break
		}
		out = append(out, doc.Diagnostic{Message: v.Message, Severity: v.Severity, Range: normalizedRange(v.Range)})
	}
	return out
}
