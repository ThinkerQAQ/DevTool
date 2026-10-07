package codegraph

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
	"github.com/thinkerqaq/devtool/sdk/codeintelligence"
)

func TestDoctorUsesConfiguredExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is unix-only")
	}
	path := writeFixture(t, `#!/bin/sh
if [ "$1" = "--version" ]; then
  echo "codegraph 0.test"
  exit 0
fi
exit 2
`)
	e := &Extension{executable: path}
	payload, _ := json.Marshal(codeintelligence.Workspace{Root: t.TempDir()})
	raw, err := e.Invoke(context.Background(), codeintelligence.MethodDoctor, payload)
	if err != nil {
		t.Fatal(err)
	}
	var response codeintelligence.DoctorResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatal(err)
	}
	if response.Provider != ExtensionID || response.Version != "codegraph 0.test" {
		t.Fatalf("response = %#v", response)
	}
}

func TestVerifyReindexesWorkspace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is unix-only")
	}
	path := writeFixture(t, `#!/bin/sh
found_graph=0
found_root=0
found_child=0
found_tool=0
found_args=0
while [ "$#" -gt 0 ]; do
  case "$1" in
    --graph-only) found_graph=1 ;;
    --workspace)
      shift
      case "$1" in
        */root) found_root=1 ;;
        */root/child) found_child=1 ;;
      esac
      ;;
    --run-tool)
      shift
      [ "$1" = "codegraph_reindex_workspace" ] && found_tool=1
      ;;
    --tool-args)
      shift
      [ "$1" = '{"force":true}' ] && found_args=1
      ;;
  esac
  shift
done
[ "$found_graph" -eq 1 ] || exit 3
[ "$found_root" -eq 1 ] || exit 4
[ "$found_child" -eq 1 ] || exit 5
[ "$found_tool" -eq 1 ] || exit 6
[ "$found_args" -eq 1 ] || exit 7
printf '{"ok":true}'
`)
	root := filepath.Join(t.TempDir(), "root")
	if err := os.MkdirAll(filepath.Join(root, "child"), 0o755); err != nil {
		t.Fatal(err)
	}
	e := &Extension{executable: path}
	payload, err := json.Marshal(codeintelligence.Workspace{Root: root, Workspaces: []string{root, "child"}})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := e.Invoke(context.Background(), codeintelligence.MethodVerify, payload)
	if err != nil {
		t.Fatal(err)
	}
	var response codeintelligence.VerifyResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatal(err)
	}
	if response.Provider != ExtensionID || !strings.Contains(response.Output, `"ok":true`) {
		t.Fatalf("response = %#v", response)
	}
}

func TestMCPCommandUsesPersistentGraphMode(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	if err := os.MkdirAll(filepath.Join(root, "child"), 0o755); err != nil {
		t.Fatal(err)
	}
	e := New()
	e.executable = "codegraph-fixture"
	t.Cleanup(func() { _ = e.Close() })

	cmd, err := e.mcpCommand(context.Background(), agentsdk.Session{
		ProjectRoot: root,
		Workspaces:  []string{root, "child"},
	})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(cmd.Args[1:], " ")
	for _, want := range []string{"--mcp", "--graph-only", "--workspace " + root, "--workspace " + filepath.Join(root, "child")} {
		if !strings.Contains(joined, want) {
			t.Fatalf("args = %q; missing %q", joined, want)
		}
	}
	if strings.Contains(joined, "--run-tool") {
		t.Fatalf("persistent MCP command unexpectedly uses one-shot mode: %q", joined)
	}
}

func TestWorkspaceIdentityStableAcrossSourceState(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "probe.go")
	if err := os.WriteFile(path, []byte("package probe\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	workspace := codeintelligence.Workspace{Root: root}
	first, err := codeGraphWorkspaceIdentity(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package probe\nfunc Changed() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := codeGraphWorkspaceIdentity(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("workspace identity changed after source edit")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	third, err := codeGraphWorkspaceIdentity(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if second != third {
		t.Fatal("workspace identity changed after source deletion")
	}
}

func TestWorkspaceIdentityIncludesGitWorktreeIdentity(t *testing.T) {
	root := t.TempDir()
	gitFile := filepath.Join(root, ".git")
	if err := os.WriteFile(gitFile, []byte("gitdir: ../repo/.git/worktrees/one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	workspace := codeintelligence.Workspace{Root: root}
	first, err := codeGraphWorkspaceIdentity(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gitFile, []byte("gitdir: ../repo/.git/worktrees/two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := codeGraphWorkspaceIdentity(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("workspace identity did not change with git worktree identity")
	}
}

func TestUnwrapMCPToolResult(t *testing.T) {
	raw := json.RawMessage(`{"content":[{"type":"text","text":"{\"results\":[]}"}]}`)
	got, err := unwrapMCPToolResult(raw)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"results":[]}` {
		t.Fatalf("got = %s", got)
	}
}

func TestSearchMapsToSymbolSearch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is unix-only")
	}
	path := writeFixture(t, `#!/bin/sh
tool=""
tool_args=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --run-tool)
      shift
      tool="$1"
      ;;
    --tool-args)
      shift
      tool_args="$1"
      ;;
  esac
  shift
done
case "$tool" in
  codegraph_reindex_workspace)
    [ "$tool_args" = '{"force":false}' ] || exit 6
    printf '{"status":"success"}'
    ;;
  codegraph_symbol_search)
    [ "$tool_args" = '{"compact":true,"limit":7,"query":"Registry"}' ] || exit 7
    printf '{"results":[]}'
    ;;
  *)
    exit 8
    ;;
esac
`)
	e := &Extension{executable: path}
	payload, err := json.Marshal(codeintelligence.SearchRequest{
		Workspace: codeintelligence.Workspace{Root: t.TempDir()},
		Query:     "Registry",
		Limit:     7,
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := e.Invoke(context.Background(), codeintelligence.MethodSearch, payload)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(raw)) != `{"results":[]}` {
		t.Fatalf("result = %s", raw)
	}
}

func writeFixture(t *testing.T, script string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDiscoveryKeywordsRemoveGenericInstructionWords(t *testing.T) {
	got := discoveryKeywords("Which code is legacy/bootstrap that should be deleted after migration?")
	joined := strings.Join(got, ",")
	for _, want := range []string{"legacy", "bootstrap", "migration"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("keywords = %v; missing %q", got, want)
		}
	}
	for _, unwanted := range []string{"code", "deleted", "should", "after", "which"} {
		for _, keyword := range got {
			if keyword == unwanted {
				t.Fatalf("keywords = %v; contains generic word %q", got, unwanted)
			}
		}
	}
}

func TestPatternDiscoveryRewardsMultipleObjectiveTerms(t *testing.T) {
	raw := json.RawMessage(`{"matches":[
		{"name":"printUsage","kind":"function","path":"go/cmd/devtool/main.go","line_start":10,"matched_in":"body","matched_text":"development operations use this control plane entry"},
		{"name":"handleControl","kind":"function","path":"go/browser/control.go","line_start":20,"matched_in":"name","matched_text":"control request"}
	]}`)
	candidates := map[string]discoveryCandidate{}
	keywords := []string{"development", "control", "plane", "entry"}
	mergePatternCandidates(candidates, raw, keywords)

	main := candidates["go/cmd/devtool/main.go\x00printUsage"]
	other := candidates["go/browser/control.go\x00handleControl"]
	if main.Score <= other.Score {
		t.Fatalf("multi-term score %.1f <= single-term score %.1f", main.Score, other.Score)
	}
}

func TestFilterSearchResultPathsMapsEnvironmentWorkspaceRoot(t *testing.T) {
	root := t.TempDir()
	fresh := filepath.Join(root, "go", "fresh.go")
	if err := os.MkdirAll(filepath.Dir(fresh), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fresh, []byte("package fresh\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	raw := json.RawMessage(`{
		"results": [
			{"path": "/workspace/go/fresh.go", "name": "Fresh"},
			{"path": "/workspace/go/deleted.go", "name": "Deleted"}
		]
	}`)
	filtered, stale, err := filterSearchResultPaths(
		codeintelligence.Workspace{Root: root},
		raw,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !stale {
		t.Fatal("expected deleted workspace path to mark the index stale")
	}

	var response struct {
		Results []struct {
			Name string `json:"name"`
		} `json:"results"`
	}
	if err := json.Unmarshal(filtered, &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Results) != 1 || response.Results[0].Name != "Fresh" {
		t.Fatalf("filtered results = %#v", response.Results)
	}
}

func TestSearchRepairsDeletedFileStaleIndex(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("CodeGraph integration fixture uses unix workspace paths")
	}
	executable, err := exec.LookPath("codegraph-server")
	if err != nil {
		t.Skip("codegraph-server is not available")
	}

	root := t.TempDir()
	keep := filepath.Join(root, "keep.go")
	deleted := filepath.Join(root, "deleted.go")
	if err := os.WriteFile(keep, []byte("package demo\nfunc KeepMe() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(deleted, []byte("package demo\nfunc DeleteMeUniqueXYZ() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	workspace := codeintelligence.Workspace{Root: root}
	e := &Extension{executable: executable}
	workspacePayload, _ := json.Marshal(workspace)
	if _, err := e.Invoke(context.Background(), codeintelligence.MethodVerify, workspacePayload); err != nil {
		t.Fatal(err)
	}

	search := codeintelligence.SearchRequest{
		Workspace: workspace,
		Query:     "DeleteMeUniqueXYZ",
		Limit:     5,
	}
	searchPayload, _ := json.Marshal(search)
	before, err := e.Invoke(context.Background(), codeintelligence.MethodSearch, searchPayload)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(before), "DeleteMeUniqueXYZ") {
		t.Fatalf("expected symbol before deletion, got %s", before)
	}

	if err := os.Remove(deleted); err != nil {
		t.Fatal(err)
	}
	if _, err := e.runTool(
		context.Background(),
		workspace,
		"codegraph_reindex_workspace",
		json.RawMessage(`{"force":false}`),
	); err != nil {
		t.Fatal(err)
	}

	args, _ := json.Marshal(map[string]any{
		"query":   "DeleteMeUniqueXYZ",
		"limit":   5,
		"compact": true,
	})
	upstream, err := e.runTool(
		context.Background(),
		workspace,
		"codegraph_symbol_search",
		args,
	)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(upstream), "DeleteMeUniqueXYZ") {
		t.Log("reproduced CodeGraph deleted-file stale symbol before provider guard")
	} else {
		t.Log("CodeGraph upstream already reconciled the deleted file")
	}

	after, err := e.Invoke(context.Background(), codeintelligence.MethodSearch, searchPayload)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(after), "DeleteMeUniqueXYZ") {
		t.Fatalf("deleted symbol leaked through CodeGraph provider: %s", after)
	}
}

func TestRevisionRefreshPolicy(t *testing.T) {
	current := revisionState{Head: "head-a", Tree: "tree-a"}
	if revisionRefreshRequired(revisionState{}, false, current, false) {
		t.Fatal("non-git workspace should stay incremental")
	}
	if !revisionRefreshRequired(revisionState{}, false, current, true) {
		t.Fatal("first git revision should force refresh")
	}
	if revisionRefreshRequired(current, true, current, true) {
		t.Fatal("unchanged git revision should stay incremental")
	}
	if !revisionRefreshRequired(current, true, revisionState{Head: "head-b", Tree: "tree-a"}, true) {
		t.Fatal("HEAD transition should force refresh")
	}
	if !revisionRefreshRequired(current, true, revisionState{Head: "head-a", Tree: "tree-b"}, true) {
		t.Fatal("tree transition should force refresh")
	}
}

func TestGitRevisionStateChangesAfterCommit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	runGit(t, root, "config", "user.name", "DevTool Test")
	runGit(t, root, "config", "user.email", "devtool@example.test")

	path := filepath.Join(root, "probe.go")
	if err := os.WriteFile(path, []byte("package probe\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "probe.go")
	runGit(t, root, "commit", "-q", "-m", "initial")

	first, ok := gitRevisionState(context.Background(), root)
	if !ok {
		t.Fatal("expected git revision state")
	}

	if err := os.WriteFile(path, []byte("package probe\nfunc Changed() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "probe.go")
	runGit(t, root, "commit", "-q", "-m", "change")

	second, ok := gitRevisionState(context.Background(), root)
	if !ok {
		t.Fatal("expected changed git revision state")
	}
	if first == second {
		t.Fatal("git revision state did not change after commit")
	}
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	command := append([]string{"-C", root}, args...)
	out, err := exec.Command("git", command...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, strings.TrimSpace(string(out)))
	}
}
