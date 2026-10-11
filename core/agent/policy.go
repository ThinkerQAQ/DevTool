package agent

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
)

// patchGate is a deterministic MCP-boundary policy, not a second agent.
// Git owns unified-patch parsing and application. Only a scoped successful
// code_context result can authorize a one-file mutation on a development branch.
type patchGate struct {
	root      string
	maxBytes  int
	ttl       time.Duration
	protected map[string]bool

	mu     sync.Mutex
	proofs map[string]patchProof
}

type patchProof struct {
	path    string
	digest  [sha256.Size]byte
	head    string
	expires time.Time
}

func newPatchGate(root string, settings config.AgentPolicy) *patchGate {
	maxBytes := settings.MaxPatchBytes
	if maxBytes == 0 {
		maxBytes = 256 * 1024
	}
	ttl := settings.ContextTTLSeconds
	if ttl == 0 {
		ttl = 600
	}
	branches := settings.ProtectedBranches
	if len(branches) == 0 {
		branches = []string{"main", "master"}
	}
	gate := &patchGate{
		root: root, maxBytes: maxBytes, ttl: time.Duration(ttl) * time.Second,
		protected: make(map[string]bool), proofs: make(map[string]patchProof),
	}
	for _, branch := range branches {
		gate.protected[branch] = true
	}
	return gate
}

func (p *patchGate) tool() json.RawMessage {
	raw, _ := json.Marshal(map[string]any{
		"name":        "workspace_apply_patch",
		"description": "Apply one Git unified diff to an existing file in the current development worktree. Requires a fresh context_token returned by a scoped code_context tool call; never permitted on protected branches.",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"context_token": map[string]any{"type": "string", "description": "One-time token from the preceding scoped code_context."},
				"patch":         map[string]any{"type": "string", "description": "Single-file text patch in 'git diff' format (no create/delete/rename/mode change)."},
			},
			"required":             []string{"context_token", "patch"},
			"additionalProperties": false,
		},
	})
	return raw
}

// attest wraps the original tool result only after both code backends succeed.
// A degraded or malformed context remains readable but has no write authority.
func (p *patchGate) attest(ctx context.Context, args, result json.RawMessage) json.RawMessage {
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
	var envelope struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if json.Unmarshal(result, &envelope) != nil || envelope.IsError {
		return result
	}
	for i, content := range envelope.Content {
		if content.Type != "text" {
			continue
		}
		var data map[string]json.RawMessage
		if json.Unmarshal([]byte(content.Text), &data) != nil {
			continue
		}
		if !validContext(data) {
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
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			return result
		}
		token := hex.EncodeToString(tokenBytes)
		expires := time.Now().Add(p.ttl)
		p.mu.Lock()
		for key, proof := range p.proofs {
			if time.Now().After(proof.expires) {
				delete(p.proofs, key)
			}
		}
		p.proofs[token] = patchProof{
			path: path, digest: sha256.Sum256(file),
			head: strings.TrimSpace(string(head)), expires: expires,
		}
		p.mu.Unlock()
		data["context_token"], _ = json.Marshal(token)
		data["context_expires_at"], _ = json.Marshal(expires.UTC().Format(time.RFC3339))
		data["context_path"], _ = json.Marshal(path)
		updatedText, err := json.Marshal(data)
		if err != nil {
			return result
		}
		envelope.Content[i].Text = string(updatedText)
		updated, err := json.Marshal(envelope)
		if err == nil {
			return updated
		}
		return result
	}
	return result
}

func validContext(value map[string]json.RawMessage) bool {
	for _, name := range []string{"indexed", "realtime"} {
		raw := bytes.TrimSpace(value[name])
		if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
			return false
		}
	}
	degraded := bytes.TrimSpace(value["degraded"])
	return len(degraded) == 0 || bytes.Equal(degraded, []byte("null")) || bytes.Equal(degraded, []byte("{}"))
}

func (p *patchGate) apply(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
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
	// One-time even for failed validation: call code_context again to retry.
	delete(p.proofs, request.Token)
	if !ok || time.Now().After(proof.expires) {
		return nil, errors.New("missing or expired context_token; run scoped code_context")
	}
	path, filename, err := p.existingFile(proof.path)
	if err != nil {
		return nil, err
	}
	if path != proof.path {
		return nil, errors.New("file path changed since code_context")
	}
	if err := p.checkWorktree(ctx, proof.head); err != nil {
		return nil, err
	}
	file, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	if sha256.Sum256(file) != proof.digest {
		return nil, errors.New("file changed since code_context; refresh context")
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
		return nil, errors.New("patch scope does not match code_context")
	}
	for _, field := range fields[:2] {
		if _, err := strconv.ParseUint(string(field), 10, 64); err != nil {
			return nil, errors.New("non-text or malformed patch")
		}
	}
	if _, err := p.git(commandContext, patch, "apply", "--check", "-"); err != nil {
		return nil, fmt.Errorf("patch check failed: %w", err)
	}
	if _, err := p.git(commandContext, patch, "apply", "-"); err != nil {
		return nil, fmt.Errorf("patch apply failed: %w", err)
	}
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

func (p *patchGate) checkWorktree(ctx context.Context, head string) error {
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
		return errors.New("Git HEAD changed since code_context; refresh context")
	}
	return nil
}

func (p *patchGate) existingFile(raw string) (string, string, error) {
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

func (p *patchGate) git(ctx context.Context, stdin []byte, args ...string) ([]byte, error) {
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
