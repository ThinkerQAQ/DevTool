package gitpatch

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/thinkerqaq/devtool/core/config"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// Gate is a deterministic MCP-boundary policy, not a second agent.
// Git owns unified-patch parsing and application. Only a scoped successful
// matching code/document/diagram contexts authorize a one-file mutation.
type Gate struct {
	root            string
	maxBytes        int
	ttl             time.Duration
	protected       map[string]bool
	validateDiagram func(context.Context, string, string) error

	mu       sync.Mutex
	proofs   map[string]patchProof
	evidence map[string]contextEvidence
}

type patchProof struct {
	path     string
	digest   [sha256.Size]byte
	head     string
	expires  time.Time
	contexts map[string]bool
}

type contextEvidence struct {
	digest   [sha256.Size]byte
	head     string
	expires  time.Time
	contexts map[string]bool
}

func New(root string, settings config.AgentPolicy) *Gate {
	maxBytes := settings.MaxPatchBytes
	if maxBytes == 0 {
		maxBytes = 256 * 1024
	}
	ttl := settings.ContextTTLSeconds
	if ttl == 0 {
		ttl = 600
	}
	// Protect the conventional primary branches even when custom names are added.
	branches := append([]string{"main", "master"}, settings.ProtectedBranches...)
	gate := &Gate{
		root: root, maxBytes: maxBytes, ttl: time.Duration(ttl) * time.Second,
		protected: make(map[string]bool), proofs: make(map[string]patchProof), evidence: make(map[string]contextEvidence),
	}
	for _, branch := range branches {
		gate.protected[branch] = true
	}
	return gate
}

// WithDiagramValidator binds the selected existing diagram renderer service.
// A changed diagram fails closed if no validator has been configured.
func (p *Gate) WithDiagramValidator(validator func(context.Context, string, string) error) *Gate {
	p.validateDiagram = validator
	return p
}

func (p *Gate) Tool() json.RawMessage {
	raw, _ := json.Marshal(map[string]any{
		"name":        "workspace_apply_patch",
		"description": "Apply a Git text patch to one existing file after scoped context: code_context for code, document_context for Markdown, and diagram_context for embedded Mermaid/PlantUML diagrams; log files are read-only and use log_context for inspection.",
		"annotations": map[string]any{"readOnlyHint": false, "destructiveHint": true, "openWorldHint": false},
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"context_token": map[string]any{"type": "string", "description": "One-time token issued by the required source-specific context tool(s)."},
				"patch":         map[string]any{"type": "string", "description": "Single-file text patch in 'git diff' format (no create/delete/rename/mode change)."},
			},
			"required":             []string{"context_token", "patch"},
			"additionalProperties": false,
		},
	})
	return raw
}

// Attest grants one-use tokens only after the correct capability has inspected
// the exact file. Markdown with embedded diagrams needs both context tools.
// Log querying is handled by log_context and never grants write permission.
func (p *Gate) Attest(ctx context.Context, tool string, args, result json.RawMessage) json.RawMessage {
	if tool != "code_context" && tool != "document_context" && tool != "diagram_context" {
		return result
	}
	var request struct {
		Path string `json:"path"`
	}
	if json.Unmarshal(args, &request) != nil {
		return result
	}
	path, filename, err := p.existingFile(request.Path)
	if err != nil {
		return result
	}
	kind := contextKind(path)
	if kind == "log" || kind == "diagram-standalone" {
		return result
	}
	if (kind == "markdown") != (tool != "code_context") {
		return result
	}

	var envelope map[string]json.RawMessage
	if json.Unmarshal(result, &envelope) != nil {
		return result
	}
	var isError bool
	if json.Unmarshal(envelope["isError"], &isError) == nil && isError {
		return result
	}
	var contents []map[string]json.RawMessage
	if json.Unmarshal(envelope["content"], &contents) != nil {
		return result
	}
	for i, content := range contents {
		var contentType, message string
		if json.Unmarshal(content["type"], &contentType) != nil ||
			json.Unmarshal(content["text"], &message) != nil || contentType != "text" {
			continue
		}
		var data map[string]json.RawMessage
		if json.Unmarshal([]byte(message), &data) != nil || !validContext(tool, path, data) {
			continue
		}
		file, err := os.ReadFile(filename)
		if err != nil {
			return result
		}
		head, err := p.git(ctx, nil, "rev-parse", "HEAD")
		if err != nil {
			return result
		}
		currentHead := strings.TrimSpace(string(head))
		digest := sha256.Sum256(file)
		needsDiagram := kind == "markdown" && hasDiagram(file)
		expires := time.Now().Add(p.ttl)
		p.mu.Lock()
		for key, proof := range p.proofs {
			if time.Now().After(proof.expires) {
				delete(p.proofs, key)
			}
		}
		prior := p.evidence[path]
		if prior.head != currentHead || prior.digest != digest || time.Now().After(prior.expires) {
			prior = contextEvidence{contexts: make(map[string]bool)}
		}
		prior.contexts[tool] = true
		prior.head, prior.digest, prior.expires = currentHead, digest, expires
		p.evidence[path] = prior
		required := []string{"code_context"}
		if kind == "markdown" {
			required = []string{"document_context"}
			if needsDiagram {
				required = append(required, "diagram_context")
			}
		}
		missing := make([]string, 0)
		for _, name := range required {
			if !prior.contexts[name] {
				missing = append(missing, name)
			}
		}
		var token string
		if len(missing) == 0 {
			tokenBytes := make([]byte, 32)
			if _, err := rand.Read(tokenBytes); err == nil {
				token = hex.EncodeToString(tokenBytes)
				copyContexts := make(map[string]bool, len(prior.contexts))
				for name, success := range prior.contexts {
					copyContexts[name] = success
				}
				p.proofs[token] = patchProof{path: path, digest: digest, head: currentHead, expires: expires, contexts: copyContexts}
			}
		}
		p.mu.Unlock()
		data["context_path"], _ = json.Marshal(path)
		if token != "" {
			data["context_token"], _ = json.Marshal(token)
			data["context_expires_at"], _ = json.Marshal(expires.UTC().Format(time.RFC3339))
		} else if len(missing) != 0 {
			data["required_context_tools"], _ = json.Marshal(missing)
		}
		updatedText, err := json.Marshal(data)
		if err != nil {
			return result
		}
		contents[i]["text"], _ = json.Marshal(string(updatedText))
		envelope["content"], _ = json.Marshal(contents)
		updated, err := json.Marshal(envelope)
		if err == nil {
			return updated
		}
		return result
	}
	return result
}

func validContext(tool, path string, value map[string]json.RawMessage) bool {
	switch tool {
	case "code_context":
		for _, name := range []string{"indexed", "realtime"} {
			raw := bytes.TrimSpace(value[name])
			if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
				return false
			}
		}
		degraded := bytes.TrimSpace(value["degraded"])
		return len(degraded) == 0 || bytes.Equal(degraded, []byte("null")) || bytes.Equal(degraded, []byte("{}"))
	case "document_context":
		var doc struct {
			Path   string `json:"path"`
			Format string `json:"format"`
		}
		return json.Unmarshal(value["document"], &doc) == nil && doc.Path == path && doc.Format == "markdown"
	case "diagram_context":
		var actualPath string
		var total int
		var results []json.RawMessage
		return json.Unmarshal(value["path"], &actualPath) == nil && actualPath == path &&
			json.Unmarshal(value["total_diagrams"], &total) == nil && total >= 0 &&
			json.Unmarshal(value["results"], &results) == nil && len(results) == total
	default:
		return false
	}
}

func contextKind(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".md", ".markdown":
		return "markdown"
	case ".log":
		return "log"
	case ".mmd", ".mermaid", ".puml", ".plantuml", ".svg", ".png", ".jpg", ".jpeg", ".webp":
		return "diagram-standalone"
	default:
		return "code"
	}
}

// diagramSources reuses Goldmark AST to extract the actual fenced diagram
// sources from the candidate document, rather than parsing Markdown twice.
type diagramSource struct{ language, source string }

func diagramSources(source []byte) []diagramSource {
	doc := goldmark.DefaultParser().Parse(text.NewReader(source))
	var diagrams []diagramSource
	ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		fenced, ok := node.(*ast.FencedCodeBlock)
		if !ok {
			return ast.WalkContinue, nil
		}
		lang := strings.ToLower(strings.TrimSpace(string(fenced.Language(source))))
		if lang != "mermaid" && lang != "plantuml" {
			return ast.WalkContinue, nil
		}
		var body strings.Builder
		for i := 0; i < fenced.Lines().Len(); i++ {
			segment := fenced.Lines().At(i)
			body.Write(segment.Value(source))
		}
		diagrams = append(diagrams, diagramSource{language: lang, source: body.String()})
		return ast.WalkContinue, nil
	})
	return diagrams
}

func hasDiagram(source []byte) bool { return len(diagramSources(source)) > 0 }

// validateCandidate applies the patch only inside a temporary isolated file
// tree. The configured renderer validates changed diagram blocks before the
// actual worktree is modified.
func (p *Gate) validateCandidate(ctx context.Context, path string, before, patch []byte) error {
	root, err := os.MkdirTemp("", "devtool-diagram-preview-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	filename := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
		return err
	}
	if err := os.WriteFile(filename, before, 0600); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "git", "apply", "-")
	cmd.Dir = root
	cmd.Stdin = bytes.NewReader(patch)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("preview git apply: %w: %s", err, strings.TrimSpace(string(out)))
	}
	after, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	old := diagramSources(before)
	fresh := diagramSources(after)
	seen := map[diagramSource]int{}
	for _, d := range old {
		seen[d]++
	}
	for _, d := range fresh {
		if seen[d] > 0 {
			seen[d]--
			continue
		}
		if p.validateDiagram == nil {
			return errors.New("diagram-render provider required to validate changed diagrams")
		}
		if err := p.validateDiagram(ctx, d.language, d.source); err != nil {
			return fmt.Errorf("reject changed %s diagram: %w", d.language, err)
		}
	}
	return nil
}

func addsDiagramFence(patch string) bool {
	for _, line := range strings.Split(patch, "\n") {
		if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, "+"))
		if strings.HasPrefix(value, "\x60\x60\x60") || strings.HasPrefix(value, "~~~") {
			label := strings.ToLower(strings.TrimSpace(strings.TrimLeft(value, "~\x60")))
			if label == "mermaid" || label == "plantuml" {
				return true
			}
		}
	}
	return false
}

func (p *Gate) Apply(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var request struct {
		Token string `json:"context_token"`
		Patch string `json:"patch"`
	}
	if err := json.Unmarshal(args, &request); err != nil {
		return nil, fmt.Errorf("decode workspace_apply_patch: %w", err)
	}
	if request.Token == "" || request.Patch == "" {
		return nil, errors.New("context_token and patch are required")
	}
	if len(request.Patch) > p.maxBytes {
		return nil, fmt.Errorf("patch exceeds configured %d bytes", p.maxBytes)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	proof, ok := p.proofs[request.Token]
	// One-time even for failed validation: refresh the required context tools to retry.
	delete(p.proofs, request.Token)
	if !ok || time.Now().After(proof.expires) {
		return nil, errors.New("missing or expired context_token; run the required scoped context tools")
	}
	path, filename, err := p.existingFile(proof.path)
	if err != nil {
		return nil, err
	}
	if path != proof.path {
		return nil, errors.New("file path changed since context inspection")
	}
	if err := p.checkWorktree(ctx, proof.head); err != nil {
		return nil, err
	}
	file, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	if sha256.Sum256(file) != proof.digest {
		return nil, errors.New("file changed since context inspection; refresh context")
	}

	// A context issued for one file type cannot authorize another capability.
	kind := contextKind(path)
	if kind == "log" || kind == "diagram-standalone" {
		return nil, fmt.Errorf("%s is not supported by workspace_apply_patch; use log_context for log queries", path)
	}
	required := []string{"code_context"}
	if kind == "markdown" {
		required = []string{"document_context"}
		if hasDiagram(file) || addsDiagramFence(request.Patch) {
			required = append(required, "diagram_context")
		}
	}
	for _, name := range required {
		if !proof.contexts[name] {
			return nil, fmt.Errorf("patch requires %s for %s", name, path)
		}
	}
	// Restrict the accepted Git syntax; git apply itself remains the parser.
	// This first version intentionally excludes new/deleted/renamed/binary files.
	header := "diff --git a/" + path + " b/" + path + "\n"
	if !strings.HasPrefix(request.Patch, header) ||
		!strings.Contains(request.Patch, "\n--- a/"+path+"\n") ||
		!strings.Contains(request.Patch, "\n+++ b/"+path+"\n") {
		return nil, errors.New("patch must be a single Git text diff for the authorized path")
	}
	for _, line := range strings.Split(request.Patch, "\n") {
		if strings.HasPrefix(line, "diff --git ") && line != strings.TrimSuffix(header, "\n") {
			return nil, errors.New("patch contains another file")
		}
		if strings.HasPrefix(line, "new file mode ") || strings.HasPrefix(line, "deleted file mode ") ||
			strings.HasPrefix(line, "old mode ") || strings.HasPrefix(line, "new mode ") ||
			strings.HasPrefix(line, "rename ") || strings.HasPrefix(line, "copy ") ||
			strings.HasPrefix(line, "GIT binary patch") || strings.HasPrefix(line, "Binary files ") ||
			line == "--- /dev/null" || line == "+++ /dev/null" {
			return nil, errors.New("patch creation, deletion, renaming, mode changes and binary patches are forbidden")
		}
	}

	commandContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	patch := []byte(request.Patch)
	numstat, err := p.git(commandContext, patch, "apply", "--numstat", "-z", "-")
	if err != nil {
		return nil, fmt.Errorf("inspect patch: %w", err)
	}
	expected := []byte("\x00")
	// Git --numstat -z emits added TAB deleted TAB pathname NUL for a normal single-file diff.
	entries := bytes.Split(bytes.TrimSuffix(numstat, expected), expected)
	if len(entries) != 1 {
		return nil, errors.New("patch must touch exactly one ordinary file")
	}
	fields := bytes.SplitN(entries[0], []byte("\t"), 3)
	if len(fields) != 3 || string(fields[2]) != path {
		return nil, errors.New("patch scope does not match context inspection")
	}
	for _, field := range fields[:2] {
		if _, err := strconv.ParseUint(string(field), 10, 64); err != nil {
			return nil, errors.New("non-text or malformed patch")
		}
	}
	if _, err := p.git(commandContext, patch, "apply", "--check", "-"); err != nil {
		return nil, fmt.Errorf("patch check failed: %w", err)
	}
	if kind == "markdown" {
		if err := p.validateCandidate(commandContext, path, file, patch); err != nil {
			return nil, err
		}
	}
	if _, err := p.git(commandContext, patch, "apply", "-"); err != nil {
		return nil, fmt.Errorf("patch apply failed: %w", err)
	}
	delete(p.evidence, path)
	changed, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("verify patched file: %w", err)
	}
	return json.Marshal(map[string]any{
		"content": []map[string]string{{"type": "text", "text": fmt.Sprintf("Applied patch to %s", path)}},
		"structuredContent": map[string]any{
			"status": "applied", "path": path, "sha256": fmt.Sprintf("%x", sha256.Sum256(changed)),
		},
	})
}

func (p *Gate) checkWorktree(ctx context.Context, head string) error {
	root, err := p.git(ctx, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("not a Git worktree: %w", err)
	}
	configured, err := filepath.EvalSymlinks(p.root)
	if err != nil {
		return err
	}
	actual, err := filepath.EvalSymlinks(strings.TrimSpace(string(root)))
	if err != nil || actual != configured {
		return errors.New("Git worktree does not match configured project root")
	}
	branch, err := p.git(ctx, nil, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return errors.New("detached HEAD cannot be patched")
	}
	if p.protected[strings.TrimSpace(string(branch))] {
		return fmt.Errorf("cannot patch protected branch %q", strings.TrimSpace(string(branch)))
	}
	current, err := p.git(ctx, nil, "rev-parse", "HEAD")
	if err != nil || strings.TrimSpace(string(current)) != head {
		return errors.New("Git HEAD changed since context inspection; refresh context")
	}
	return nil
}

func (p *Gate) existingFile(raw string) (string, string, error) {
	if raw == "" || filepath.IsAbs(raw) || strings.Contains(raw, "\\") {
		return "", "", errors.New("a project-relative existing file path is required")
	}
	clean := filepath.Clean(raw)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", "", errors.New("file must remain inside project root")
	}
	if clean == ".devtool.toml" || clean == ".git" ||
		strings.HasPrefix(clean, ".git/") || strings.HasPrefix(clean, ".devtool/") {
		return "", "", errors.New("DevTool configuration and internal metadata are protected")
	}
	filename := filepath.Join(p.root, clean)
	root, err := filepath.EvalSymlinks(p.root)
	if err != nil {
		return "", "", err
	}
	actual, err := filepath.EvalSymlinks(filename)
	if err != nil {
		return "", "", fmt.Errorf("resolve patch target: %w", err)
	}
	rel, err := filepath.Rel(root, actual)
	if err != nil || rel != clean {
		return "", "", errors.New("target must be a regular file without symlink redirection")
	}
	info, err := os.Stat(filename)
	if err != nil || !info.Mode().IsRegular() {
		return "", "", errors.New("patch target must be an existing regular file")
	}
	return filepath.ToSlash(clean), filename, nil
}

func (p *Gate) git(ctx context.Context, stdin []byte, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, "git", args...)
	command.Dir = p.root
	if stdin != nil {
		command.Stdin = bytes.NewReader(stdin)
	}
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return output, nil
}
