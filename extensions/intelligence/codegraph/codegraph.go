package codegraph

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/thinkerqaq/devtool/core/agent/mcpbridge"
	"github.com/thinkerqaq/devtool/extensions/intelligence/internal/envexec"
	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
	"github.com/thinkerqaq/devtool/sdk/codeintelligence"
	environmentcontract "github.com/thinkerqaq/devtool/sdk/environment"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
	"github.com/thinkerqaq/devtool/sdk/readiness"
	service "github.com/thinkerqaq/devtool/sdk/service"
	devtooltrace "github.com/thinkerqaq/devtool/sdk/trace"
)

const ExtensionID = "intelligence.codegraph"

type Extension struct {
	// executable exists only for isolated adapter tests. Production execution
	// is resolved through the configured Environment service.
	executable string
	services   extensioncontract.Registrar

	mu               sync.Mutex
	bridge           *mcpbridge.Provider
	workspaceVersion uint64
	persistent       bool
}

func New() *Extension { return &Extension{persistent: true} }

func (e *Extension) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.bridge == nil {
		return nil
	}
	err := e.bridge.Close()
	e.bridge = nil
	e.workspaceVersion = 0
	return err
}

func (e *Extension) Descriptor() extensioncontract.Descriptor {
	return extensioncontract.Descriptor{
		ID:        ExtensionID,
		Kind:      extensioncontract.KindCodeIntelligence,
		Provides:  []string{codeintelligence.IndexedServiceName},
		Requires:  []string{environmentcontract.ServiceName},
		Readiness: true,
	}
}

func (e *Extension) Register(reg extensioncontract.Registrar) error {
	e.services = reg
	return reg.ProvideService(codeintelligence.IndexedServiceName, ExtensionID, service.Func(e.Invoke))
}

func (e *Extension) CheckReadiness(ctx context.Context, request readiness.Request) (readiness.Report, error) {
	workspace := codeintelligence.Workspace{Root: request.Root, Workspaces: request.Workspaces}
	report := readiness.Report{Provider: ExtensionID}

	raw, err := e.doctor(ctx, workspace)
	if err != nil {
		kind := readiness.KindProviderUnavailable
		resource := ExtensionID
		if strings.Contains(err.Error(), "executable file not found") || strings.Contains(err.Error(), "not found in $PATH") {
			kind = readiness.KindMissingDependency
			resource = "codegraph-server"
		}
		report.Issues = append(report.Issues, readiness.Issue{
			Kind:        kind,
			Resource:    resource,
			Message:     err.Error(),
			Remediation: "Make codegraph-server available in the configured environment, then rerun devtool init.",
		})
		return report, nil
	}

	var doctor codeintelligence.DoctorResponse
	if err := json.Unmarshal(raw, &doctor); err == nil {
		report.Details = map[string]string{
			"executable": doctor.Executable,
			"version":    doctor.Version,
		}
	}

	if _, err := e.runTool(ctx, workspace, "codegraph_reindex_workspace", json.RawMessage(`{"force":false}`)); err != nil {
		report.Issues = append(report.Issues, readiness.Issue{
			Kind:        readiness.KindVerificationFailed,
			Resource:    "codegraph-index",
			Message:     err.Error(),
			Remediation: "Repair the CodeGraph workspace/index and rerun devtool init.",
		})
		return report, nil
	}
	report.Ready = true
	return report, nil
}

func (e *Extension) mcpCommand(ctx context.Context, session agentsdk.Session) (*exec.Cmd, error) {
	workspace := codeintelligence.Workspace{
		Root:       session.ProjectRoot,
		Workspaces: session.Workspaces,
	}
	base, err := graphBaseArgs(workspace, e.executable == "")
	if err != nil {
		return nil, err
	}
	args := append([]string{"--mcp"}, base...)
	return e.command(ctx, workspace, args...)
}

func (e *Extension) Invoke(ctx context.Context, method string, payload json.RawMessage) (result json.RawMessage, err error) {
	ctx, span := devtooltrace.Start(ctx, devtooltrace.Attributes{
		Name:         "CodeGraph",
		Layer:        "provider",
		Service:      codeintelligence.IndexedServiceName,
		Provider:     ExtensionID,
		Method:       method,
		RequestBytes: len(payload),
	})
	defer func() { span.End(len(result), err) }()

	switch method {
	case codeintelligence.MethodDoctor:
		var workspace codeintelligence.Workspace
		if err := json.Unmarshal(payload, &workspace); err != nil {
			return nil, fmt.Errorf("decode CodeGraph doctor request: %w", err)
		}
		return e.doctor(ctx, workspace)
	case codeintelligence.MethodVerify:
		var request codeintelligence.Workspace
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, fmt.Errorf("decode CodeGraph verify request: %w", err)
		}
		raw, err := e.runTool(ctx, request, "codegraph_reindex_workspace", json.RawMessage(`{"force":false}`))
		if err != nil {
			return nil, err
		}
		return json.Marshal(codeintelligence.VerifyResponse{Provider: ExtensionID, Output: strings.TrimSpace(string(raw))})
	case codeintelligence.MethodSearch:
		var request codeintelligence.SearchRequest
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, fmt.Errorf("decode CodeGraph search request: %w", err)
		}
		query := strings.TrimSpace(request.Query)
		if query == "" {
			return nil, fmt.Errorf("CodeGraph search query is required")
		}
		limit := request.Limit
		if limit <= 0 {
			limit = 20
		}
		if request.Discovery {
			return e.discover(ctx, request.Workspace, query, limit)
		}
		args, err := json.Marshal(map[string]any{
			"query":   query,
			"limit":   limit,
			"compact": true,
		})
		if err != nil {
			return nil, err
		}
		return e.runTool(ctx, request.Workspace, "codegraph_symbol_search", args)
	default:
		return nil, fmt.Errorf("%s does not support method %q", ExtensionID, method)
	}
}

type discoveryCandidate struct {
	Path        string  `json:"path"`
	Name        string  `json:"name"`
	Kind        string  `json:"kind,omitempty"`
	Line        int     `json:"line,omitempty"`
	Score       float64 `json:"score"`
	MatchReason string  `json:"match_reason"`
	Snippet     string  `json:"snippet,omitempty"`
}

func (e *Extension) discover(ctx context.Context, workspace codeintelligence.Workspace, query string, limit int) (json.RawMessage, error) {
	keywords := discoveryKeywords(query)
	candidates := map[string]discoveryCandidate{}

	if len(keywords) != 0 {
		// Symbol search has the cheaper cold-start path in CodeGraph and warms
		// the persistent graph session. Run one focused symbol query first,
		// then use repository-wide pattern discovery against the warm provider.
		if err := e.mergeSymbolSearch(ctx, workspace, candidates, keywords[0], limit); err != nil {
			return nil, err
		}

		patternParts := make([]string, 0, len(keywords))
		for _, keyword := range keywords {
			patternParts = append(patternParts, regexp.QuoteMeta(keyword))
		}
		args, err := json.Marshal(map[string]any{
			"pattern": strings.Join(patternParts, "|"),
			"scope":   "any",
			"limit":   max(limit*3, 20),
		})
		if err != nil {
			return nil, err
		}
		raw, err := e.runTool(ctx, workspace, "codegraph_search_by_pattern", args)
		if err != nil {
			return nil, err
		}
		mergePatternCandidates(candidates, raw, keywords)

		for _, keyword := range keywords[1:min(len(keywords), 6)] {
			if err := e.mergeSymbolSearch(ctx, workspace, candidates, keyword, limit); err != nil {
				return nil, err
			}
		}
	}

	if needsEntryPointDiscovery(keywords) || len(candidates) < min(limit, 5) {
		args, _ := json.Marshal(map[string]any{
			"entryType": "main",
			"limit":     max(limit, 10),
			"compact":   true,
		})
		raw, err := e.runTool(ctx, workspace, "codegraph_find_entry_points", args)
		if err == nil {
			mergeEntryCandidates(candidates, raw, keywords)
		}
	}

	pathText := map[string]string{}
	for _, candidate := range candidates {
		pathText[candidate.Path] += " " + candidate.Name + " " + candidate.Snippet
	}
	for key, candidate := range candidates {
		hits := keywordHits(strings.ToLower(candidate.Path+" "+pathText[candidate.Path]), keywords)
		if hits >= 2 {
			candidate.Score += float64(hits) * 15
		}
		lowerPath := strings.ToLower(filepath.ToSlash(candidate.Path))
		if strings.Contains(lowerPath, "_test.") || strings.Contains(lowerPath, "/test/") || strings.Contains(lowerPath, "/tests/") {
			candidate.Score -= 80
		}
		candidates[key] = candidate
	}

	ordered := make([]discoveryCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		ordered = append(ordered, candidate)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Score != ordered[j].Score {
			return ordered[i].Score > ordered[j].Score
		}
		if ordered[i].Path != ordered[j].Path {
			return ordered[i].Path < ordered[j].Path
		}
		return ordered[i].Name < ordered[j].Name
	})
	if len(ordered) > limit {
		ordered = ordered[:limit]
	}
	return json.Marshal(map[string]any{
		"query":   query,
		"results": ordered,
	})
}

func discoveryKeywords(query string) []string {
	stop := map[string]struct{}{
		"a": {}, "an": {}, "and": {}, "are": {}, "as": {}, "at": {}, "be": {}, "by": {},
		"after": {}, "before": {}, "belongs": {}, "code": {}, "current": {}, "currently": {},
		"delete": {}, "deleted": {}, "does": {}, "do": {}, "for": {}, "from": {}, "how": {}, "in": {},
		"is": {}, "minimal": {}, "of": {}, "on": {}, "or": {}, "responsibility": {}, "should": {},
		"that": {}, "the": {}, "this": {}, "through": {}, "to": {}, "what": {}, "where": {}, "which": {}, "with": {},
	}
	seen := map[string]struct{}{}
	var out []string
	lowerQuery := strings.ToLower(query)
	for _, token := range strings.FieldsFunc(lowerQuery, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
	}) {
		if len([]rune(token)) < 3 {
			continue
		}
		if _, ok := stop[token]; ok {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		out = append(out, token)
	}
	return out
}

func needsEntryPointDiscovery(keywords []string) bool {
	for _, keyword := range keywords {
		switch keyword {
		case "entry", "entrypoint", "main", "start", "startup", "command", "control", "bootstrap":
			return true
		}
	}
	return false
}

func (e *Extension) mergeSymbolSearch(ctx context.Context, workspace codeintelligence.Workspace, dst map[string]discoveryCandidate, keyword string, limit int) error {
	args, err := json.Marshal(map[string]any{
		"query":   keyword,
		"limit":   max(limit, 10),
		"compact": true,
	})
	if err != nil {
		return err
	}
	raw, err := e.runTool(ctx, workspace, "codegraph_symbol_search", args)
	if err != nil {
		return err
	}
	mergeSymbolCandidates(dst, raw, keyword)
	return nil
}

func mergePatternCandidates(dst map[string]discoveryCandidate, raw json.RawMessage, keywords []string) {
	var response struct {
		Matches []struct {
			Name        string `json:"name"`
			Kind        string `json:"kind"`
			Path        string `json:"path"`
			LineStart   int    `json:"line_start"`
			MatchedIn   string `json:"matched_in"`
			MatchedText string `json:"matched_text"`
		} `json:"matches"`
	}
	if json.Unmarshal(raw, &response) != nil {
		return
	}
	for _, match := range response.Matches {
		nameHits := keywordHits(strings.ToLower(match.Name), keywords)
		pathHits := keywordHits(strings.ToLower(match.Path), keywords)
		snippetHits := keywordHits(strings.ToLower(match.MatchedText), keywords)
		score := 40.0 + float64(nameHits)*40 + float64(pathHits)*20 + float64(snippetHits)*5
		if snippetHits >= 2 {
			score += 80 + float64(snippetHits)*20
		}
		if match.MatchedIn == "name" {
			score += 20
		}
		addCandidate(dst, discoveryCandidate{
			Path: match.Path, Name: match.Name, Kind: match.Kind, Line: match.LineStart,
			Score: score, MatchReason: "pattern:" + match.MatchedIn, Snippet: compactSnippet(match.MatchedText),
		})
	}
}

func mergeSymbolCandidates(dst map[string]discoveryCandidate, raw json.RawMessage, keyword string) {
	var response struct {
		Results []struct {
			Score       float64 `json:"score"`
			MatchReason string  `json:"match_reason"`
			Symbol      struct {
				Name     string `json:"name"`
				Kind     string `json:"kind"`
				Location struct {
					File string `json:"file"`
					Line int    `json:"line"`
				} `json:"location"`
			} `json:"symbol"`
		} `json:"results"`
	}
	if json.Unmarshal(raw, &response) != nil {
		return
	}
	for _, result := range response.Results {
		score := 100.0 + result.Score*20
		name := strings.ToLower(result.Symbol.Name)
		if strings.EqualFold(result.Symbol.Name, keyword) {
			score += 80
		} else if strings.Contains(name, strings.ToLower(keyword)) {
			score += 50
		}
		addCandidate(dst, discoveryCandidate{
			Path: result.Symbol.Location.File, Name: result.Symbol.Name, Kind: result.Symbol.Kind,
			Line: result.Symbol.Location.Line, Score: score, MatchReason: "symbol:" + result.MatchReason,
		})
	}
}

func mergeEntryCandidates(dst map[string]discoveryCandidate, raw json.RawMessage, keywords []string) {
	var entries []struct {
		EntryType string `json:"entry_type"`
		Symbol    struct {
			Name     string `json:"name"`
			Kind     string `json:"kind"`
			Location struct {
				File string `json:"file"`
				Line int    `json:"line"`
			} `json:"location"`
		} `json:"symbol"`
	}
	if json.Unmarshal(raw, &entries) != nil {
		return
	}
	for _, entry := range entries {
		haystack := strings.ToLower(entry.Symbol.Name + " " + entry.Symbol.Location.File)
		score := 50.0 + float64(keywordHits(haystack, keywords))*20
		addCandidate(dst, discoveryCandidate{
			Path: entry.Symbol.Location.File, Name: entry.Symbol.Name, Kind: entry.Symbol.Kind,
			Line: entry.Symbol.Location.Line, Score: score, MatchReason: "entry:" + entry.EntryType,
		})
	}
}

func addCandidate(dst map[string]discoveryCandidate, candidate discoveryCandidate) {
	if candidate.Path == "" || candidate.Name == "" {
		return
	}
	key := candidate.Path + "\x00" + candidate.Name
	if current, ok := dst[key]; !ok || candidate.Score > current.Score {
		dst[key] = candidate
	}
}

func keywordHits(haystack string, keywords []string) int {
	hits := 0
	for _, keyword := range keywords {
		if strings.Contains(haystack, keyword) {
			hits++
		}
	}
	return hits
}

func compactSnippet(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	const maxRunes = 220
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes]) + "…"
}

func (e *Extension) doctor(ctx context.Context, workspace codeintelligence.Workspace) (json.RawMessage, error) {
	out, err := e.combinedOutput(ctx, workspace, "--version")
	if err != nil {
		return nil, fmt.Errorf("CodeGraph version: %w", err)
	}
	response := codeintelligence.DoctorResponse{
		Provider:   ExtensionID,
		Executable: "codegraph-server",
		Version:    strings.TrimSpace(string(out)),
	}
	return json.Marshal(response)
}

func (e *Extension) runTool(ctx context.Context, workspace codeintelligence.Workspace, tool string, toolArgs json.RawMessage) (json.RawMessage, error) {
	if e.persistent {
		return e.runPersistentTool(ctx, workspace, tool, toolArgs)
	}
	return e.runOneShotTool(ctx, workspace, tool, toolArgs)
}

func (e *Extension) runPersistentTool(ctx context.Context, workspace codeintelligence.Workspace, tool string, toolArgs json.RawMessage) (json.RawMessage, error) {
	version, err := workspaceFingerprint(workspace)
	if err != nil {
		return nil, err
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.bridge == nil || e.workspaceVersion != version {
		if e.bridge != nil {
			_ = e.bridge.Close()
		}
		e.bridge = mcpbridge.New(e.mcpCommand)
		e.workspaceVersion = version
	}

	session := agentsdk.Session{
		ProjectRoot: workspace.Root,
		Workspaces:  append([]string(nil), workspace.Workspaces...),
		Context:     "codegraph",
	}
	raw, err := e.bridge.CallTool(ctx, session, tool, toolArgs)
	if err != nil {
		return nil, fmt.Errorf("CodeGraph %s: %w", tool, err)
	}
	return unwrapMCPToolResult(raw)
}

func workspaceFingerprint(workspace codeintelligence.Workspace) (uint64, error) {
	root := strings.TrimSpace(workspace.Root)
	if root == "" {
		return 0, fmt.Errorf("CodeGraph workspace root is required")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return 0, fmt.Errorf("resolve CodeGraph root: %w", err)
	}
	workspaces := workspace.Workspaces
	if len(workspaces) == 0 {
		workspaces = []string{root}
	}

	h := fnv.New64a()
	for _, item := range workspaces {
		if !filepath.IsAbs(item) {
			item = filepath.Join(root, item)
		}
		item = filepath.Clean(item)
		if _, err := os.Stat(item); err != nil {
			return 0, fmt.Errorf("stat CodeGraph workspace %s: %w", item, err)
		}
		if err := filepath.WalkDir(item, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() && path != item && ignoredFingerprintDir(entry.Name()) {
				return filepath.SkipDir
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			_, _ = h.Write([]byte(filepath.ToSlash(rel)))
			_, _ = h.Write([]byte{0})
			_, _ = h.Write([]byte(strconv.FormatInt(info.Size(), 10)))
			_, _ = h.Write([]byte{0})
			_, _ = h.Write([]byte(strconv.FormatInt(info.ModTime().UnixNano(), 10)))
			_, _ = h.Write([]byte{0})
			return nil
		}); err != nil {
			return 0, fmt.Errorf("fingerprint CodeGraph workspace %s: %w", item, err)
		}
	}
	return h.Sum64(), nil
}

func ignoredFingerprintDir(name string) bool {
	switch name {
	case ".git", ".devtool", ".cache", "node_modules", "dist", "coverage":
		return true
	default:
		return false
	}
}

func (e *Extension) runOneShotTool(ctx context.Context, workspace codeintelligence.Workspace, tool string, toolArgs json.RawMessage) (json.RawMessage, error) {
	args, err := graphBaseArgs(workspace, e.executable == "")
	if err != nil {
		return nil, err
	}
	args = append(args,
		"--run-tool", tool,
		"--tool-args", string(toolArgs),
	)
	out, err := e.combinedOutput(ctx, workspace, args...)
	if err != nil {
		return nil, fmt.Errorf("CodeGraph %s: %w", tool, err)
	}
	raw := bytes.TrimSpace(out)
	if len(raw) == 0 {
		return json.RawMessage(`null`), nil
	}
	if json.Valid(raw) {
		return append(json.RawMessage(nil), raw...), nil
	}
	return json.Marshal(string(raw))
}

func unwrapMCPToolResult(raw json.RawMessage) (json.RawMessage, error) {
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError,omitempty"`
	}
	if err := json.Unmarshal(raw, &result); err != nil || len(result.Content) == 0 {
		return append(json.RawMessage(nil), raw...), nil
	}
	text := strings.TrimSpace(result.Content[0].Text)
	if result.IsError {
		if text == "" {
			text = "CodeGraph MCP tool returned an error"
		}
		return nil, fmt.Errorf("%s", text)
	}
	if text != "" && json.Valid([]byte(text)) {
		return json.RawMessage(text), nil
	}
	return append(json.RawMessage(nil), raw...), nil
}

func graphBaseArgs(workspace codeintelligence.Workspace, environmentPaths bool) ([]string, error) {
	root := strings.TrimSpace(workspace.Root)
	if root == "" {
		return nil, fmt.Errorf("CodeGraph workspace root is required")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve CodeGraph root: %w", err)
	}
	workspaces := workspace.Workspaces
	if len(workspaces) == 0 {
		workspaces = []string{root}
	}
	args := []string{"--graph-only"}
	for _, item := range workspaces {
		item = strings.TrimSpace(item)
		if item == "" {
			return nil, fmt.Errorf("CodeGraph workspace cannot be empty")
		}
		if !filepath.IsAbs(item) {
			item = filepath.Join(root, item)
		}
		item = filepath.Clean(item)
		if environmentPaths {
			rel, err := filepath.Rel(root, item)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return nil, fmt.Errorf("CodeGraph workspace %s is outside project root %s", item, root)
			}
			if rel == "." {
				item = environmentcontract.WorkspaceRoot
			} else {
				item = filepath.ToSlash(filepath.Join(environmentcontract.WorkspaceRoot, rel))
			}
		}
		args = append(args, "--workspace", item)
	}
	return args, nil
}

func (e *Extension) command(ctx context.Context, workspace codeintelligence.Workspace, args ...string) (*exec.Cmd, error) {
	if strings.TrimSpace(e.executable) != "" {
		cmd := exec.CommandContext(ctx, e.executable, args...)
		cmd.Dir = workspace.Root
		return cmd, nil
	}
	return envexec.Command(ctx, e.services, workspace, "codegraph-server", args...)
}

func (e *Extension) combinedOutput(ctx context.Context, workspace codeintelligence.Workspace, args ...string) ([]byte, error) {
	cmd, err := e.command(ctx, workspace, args...)
	if err != nil {
		return nil, err
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("codegraph-server: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}
