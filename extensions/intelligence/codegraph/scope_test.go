package codegraph

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/thinkerqaq/devtool/sdk/codeintelligence"
)

func scopeFixture(t *testing.T) (codeintelligence.Workspace, string) {
	t.Helper()
	root := t.TempDir()
	for _, path := range []string{"go/main.go", "go/lib.go", "android/Main.kt", "go-next/other.go"} {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("fixture\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return codeintelligence.Workspace{Root: root}, root
}

func TestSearchScopeFiltersDirectoryAndFileBeforeLimit(t *testing.T) {
	ws, _ := scopeFixture(t)
	scope, err := resolveSearchScope(ws, "go")
	if err != nil {
		t.Fatal(err)
	}
	raw := json.RawMessage(`{"query":"AreaAnchor","results":[
        {"path":"android/Main.kt","name":"Outside","score":200},
        {"path":"go-next/other.go","name":"PrefixTrick","score":190},
        {"path":"/workspace/go/main.go","name":"First","score":100},
        {"symbol":{"location":{"file":"go/lib.go"}},"name":"Second","score":90}
    ]}`)
	filtered, err := filterSearchResultScope(ws, raw, scope, 1)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Results []struct {
			Name string `json:"name"`
		} `json:"results"`
	}
	if err := json.Unmarshal(filtered, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Results) != 1 || result.Results[0].Name != "First" {
		t.Fatalf("filtered: %s", filtered)
	}
	fileScope, err := resolveSearchScope(ws, "go/lib.go")
	if err != nil {
		t.Fatal(err)
	}
	filtered, err = filterSearchResultScope(ws, raw, fileScope, 4)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(filtered, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Results) != 1 || result.Results[0].Name != "Second" {
		t.Fatalf("file scope: %s", filtered)
	}
}

func TestDiscoveryScopePrunesOutsideCandidatesBeforeRanking(t *testing.T) {
	ws, _ := scopeFixture(t)
	scope, _ := resolveSearchScope(ws, "go")
	candidates := map[string]discoveryCandidate{
		"outside": {Path: "android/Main.kt", Name: "Outside", Score: 1000},
		"inside":  {Path: "go/main.go", Name: "Inside", Score: 20},
		"prefix":  {Path: "go-next/other.go", Name: "Prefix", Score: 900},
	}
	filterDiscoveryCandidates(ws, candidates, scope)
	if len(candidates) != 1 || candidates["inside"].Name != "Inside" {
		t.Fatalf("candidates %#v", candidates)
	}
}

func TestSearchScopeRejectsEscapesAndMissingTargets(t *testing.T) {
	ws, root := scopeFixture(t)
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "outside")); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../outside", "/tmp", "outside", "missing.go"} {
		if _, err := resolveSearchScope(ws, path); err == nil {
			t.Errorf("scope %q unexpectedly accepted", path)
		}
	}
	// Unscoped searches retain existing behavior and do not depend on a path.
	if scope, err := resolveSearchScope(ws, ""); err != nil || scope != "" {
		t.Fatal(err, scope)
	}
	// Lexical matches must never treat a sibling prefix as a child directory.
	if searchPathMatchesScope(ws, filepath.Join(root, "go"), "go-next/other.go") {
		t.Fatal("scope crossed directory boundary")
	}
	if searchPathMatchesScope(ws, filepath.Join(root, "go"), strings.Repeat("../", 4)+"etc/passwd") {
		t.Fatal("scope escaped root")
	}
}

func TestCodeGraphSearchHonorsDirectoryScopeEndToEnd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is Unix-only")
	}
	ws, _ := scopeFixture(t)
	executable := writeFixture(t, `#!/bin/sh
tool=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --run-tool) shift; tool="$1";;
  esac
  shift
done
case "$tool" in
  codegraph_reindex_workspace)
    printf '''{"status":"success"}'''
    ;;
  codegraph_symbol_search)
    printf '''{"results":[
      {"symbol":{"name":"Outside","kind":"type","location":{"file":"android/Main.kt","line":1}},"score":1000},
      {"symbol":{"name":"AreaAnchor","kind":"type","location":{"file":"go/main.go","line":2}},"score":1}
    ]}'''
    ;;
  codegraph_search_by_pattern)
    printf '''{"matches":[
      {"path":"android/Main.kt","name":"Outside","kind":"type","line_start":1,"matched_in":"name","matched_text":"AreaAnchor"},
      {"path":"go/main.go","name":"AreaAnchor","kind":"type","line_start":2,"matched_in":"name","matched_text":"AreaAnchor"}
    ]}'''
    ;;
  codegraph_find_entry_points) printf '''[]''';;
  *) exit 7;;
esac
`)
	ext := &Extension{executable: executable}
	for _, discovery := range []bool{true, false} {
		payload, err := json.Marshal(codeintelligence.SearchRequest{
			Workspace: ws, Query: "AreaAnchor", Path: "go", Limit: 1, Discovery: discovery,
		})
		if err != nil {
			t.Fatal(err)
		}
		raw, err := ext.Invoke(t.Context(), codeintelligence.MethodSearch, payload)
		if err != nil {
			t.Fatalf("discovery=%v: %v", discovery, err)
		}
		var response struct {
			Results []struct {
				Path   string `json:"path"`
				Symbol struct {
					Location struct {
						File string `json:"file"`
					} `json:"location"`
				} `json:"symbol"`
			} `json:"results"`
		}
		if err := json.Unmarshal(raw, &response); err != nil {
			t.Fatal(err)
		}
		if len(response.Results) != 1 {
			t.Fatalf("discovery=%v: %s", discovery, raw)
		}
		got := response.Results[0].Path
		if got == "" {
			got = response.Results[0].Symbol.Location.File
		}
		if got != "go/main.go" {
			t.Fatalf("discovery=%v out-of-scope result=%q", discovery, got)
		}
	}
}
