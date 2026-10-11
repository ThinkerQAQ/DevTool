package codegraph

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/thinkerqaq/devtool/sdk/codeintelligence"
)

// resolveSearchScope constrains searches to an existing project-relative file or
// directory. It also resolves symlinks so a path cannot escape the workspace.
func resolveSearchScope(workspace codeintelligence.Workspace, path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("code search scope must be project-relative: %q", path)
	}
	root, err := filepath.EvalSymlinks(workspace.Root)
	if err != nil {
		return "", fmt.Errorf("resolve code search project root: %w", err)
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return "", err
	}
	candidate, inside := pathWithinRoot(root, filepath.Join(root, filepath.FromSlash(path)))
	if !inside {
		return "", fmt.Errorf("code search scope escapes project root: %q", path)
	}
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve code search scope %q: %w", path, err)
	}
	if _, inside := pathWithinRoot(root, resolved); !inside {
		return "", fmt.Errorf("code search scope escapes project root through symlink: %q", path)
	}
	return resolved, nil
}

func searchPathMatchesScope(workspace codeintelligence.Workspace, scope, path string) bool {
	if scope == "" {
		return true
	}
	candidate, ok := resolveWorkspacePath(workspace, path)
	if !ok {
		return false
	}
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return false
	}
	_, inside := pathWithinRoot(scope, resolved)
	return inside
}

// filterDiscoveryCandidates runs before sorting and truncation. Results without
// a trustworthy project file are excluded when a caller requests a scope.
func filterDiscoveryCandidates(workspace codeintelligence.Workspace, candidates map[string]discoveryCandidate, scope string) {
	if scope == "" {
		return
	}
	for key, candidate := range candidates {
		if !searchPathMatchesScope(workspace, scope, candidate.Path) {
			delete(candidates, key)
		}
	}
}

// filterSearchResultScope handles the non-discovery CodeGraph symbol response.
// Its upstream query overfetches before this final limit is enforced.
func filterSearchResultScope(workspace codeintelligence.Workspace, raw json.RawMessage, scope string, limit int) (json.RawMessage, error) {
	if scope == "" {
		return raw, nil
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decode scoped CodeGraph search response: %w", err)
	}
	var results []json.RawMessage
	if err := json.Unmarshal(envelope["results"], &results); err != nil {
		return nil, fmt.Errorf("decode scoped CodeGraph search results: %w", err)
	}
	filtered := make([]json.RawMessage, 0, min(len(results), limit))
	for _, result := range results {
		path, ok, err := codeGraphResultPath(result)
		if err != nil {
			return nil, err
		}
		if ok && searchPathMatchesScope(workspace, scope, path) {
			filtered = append(filtered, result)
			if len(filtered) == limit {
				break
			}
		}
	}
	envelope["results"], _ = json.Marshal(filtered)
	return json.Marshal(envelope)
}
