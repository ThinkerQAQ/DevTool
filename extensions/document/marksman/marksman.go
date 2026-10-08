package marksman

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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
	mu             sync.Mutex
	sessions       map[string]*lspSession
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
	return doc.AnalyzeResponse{Provider: ExtensionID, Status: "unavailable", Detail: detail, References: []doc.Location{}}
}
func (e *Extension) analyze(ctx context.Context, in doc.AnalyzeRequest) (doc.AnalyzeResponse, error) {
	if in.Root == "" || in.Path == "" {
		return doc.AnalyzeResponse{}, fmt.Errorf("document realtime requires root and path")
	}
	if in.HeadingLine <= 0 {
		return doc.AnalyzeResponse{}, fmt.Errorf("heading line must be positive")
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
	lines := strings.Split(string(source), "\n")
	if in.HeadingLine > len(lines) {
		return doc.AnalyzeResponse{}, fmt.Errorf("heading line out of range")
	}
	heading := lines[in.HeadingLine-1]
	trimmed := strings.TrimLeft(heading, " \t")
	hashes := 0
	for hashes < len(trimmed) && trimmed[hashes] == '#' {
		hashes++
	}
	if hashes < 1 || hashes > 6 || hashes >= len(trimmed) || (trimmed[hashes] != ' ' && trimmed[hashes] != '\t') {
		return doc.AnalyzeResponse{}, fmt.Errorf("line %d is not a Markdown heading", in.HeadingLine)
	}
	pos := len(heading) - len(trimmed) + hashes + 1
	for pos < len(heading) && (heading[pos] == ' ' || heading[pos] == '\t') {
		pos++
	}
	bin, err := exec.LookPath(e.binary)
	if err != nil {
		return unavailable("Marksman executable unavailable; configure document-realtime binary; no automatic installation"), nil
	}
	session, ephemeral := e.getSession(workspaceRoot)
	session.mu.Lock()
	defer session.mu.Unlock()
	run, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()
	stopDeadline := context.AfterFunc(run, session.stop)
	defer stopDeadline()
	if ephemeral {
		defer session.stop()
	}
	failed := func(detail string) (doc.AnalyzeResponse, error) {
		if !ephemeral {
			e.dropSession(workspaceRoot, session)
		} else {
			session.stop()
		}
		if run.Err() != nil {
			detail = "Marksman request timed out or canceled: " + run.Err().Error()
		}
		return unavailable(detail), nil
	}
	rootURI := (&url.URL{Scheme: "file", Path: filepath.ToSlash(workspaceRoot)}).String()
	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
	if session.client == nil {
		if err := session.start(bin, workspaceRoot); err != nil {
			return failed("Marksman start failed: " + err.Error())
		}
		init := map[string]any{"processId": nil, "rootUri": rootURI,
			"capabilities": map[string]any{"textDocument": map[string]any{
				"references": map[string]any{}},
				"workspace": map[string]any{"workspaceFolders": true}},
			"workspaceFolders": []map[string]any{{"uri": rootURI, "name": filepath.Base(workspaceRoot)}}}
		if _, err := session.client.request(1, "initialize", init); err != nil {
			return failed("Marksman initialize failed: " + err.Error())
		}
		if err := session.client.notify("initialized", map[string]any{}); err != nil {
			return failed(err.Error())
		}
	}
	if err := session.syncDocument(uri, source); err != nil {
		return failed("Marksman sync failed: " + err.Error())
	}
	params := map[string]any{
		"textDocument": map[string]string{"uri": uri},
		"position":     map[string]int{"line": in.HeadingLine - 1, "character": pos},
		"context":      map[string]bool{"includeDeclaration": true},
	}
	refs, err := session.client.request(session.requestID(), "textDocument/references", params)
	if err != nil {
		return failed("Marksman references failed: " + err.Error())
	}
	locations, truncated := parseLocations(refs, root)
	result := doc.AnalyzeResponse{Status: "ok", Provider: ExtensionID, WorkspaceScope: filepath.ToSlash(scope),
		References: locations, Truncated: truncated}
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

func parseLocations(raw json.RawMessage, root string) ([]doc.Location, bool) {
	var items []struct {
		URI   string   `json:"uri"`
		Range lspRange `json:"range"`
	}
	if len(raw) == 0 || string(raw) == "null" {
		return []doc.Location{}, false
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
	return out, len(items) > 300
}
