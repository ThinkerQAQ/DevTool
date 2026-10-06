package codegraph

import (
	"context"
	"encoding/json"
	"os"
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
      [ "$1" = '{"force":false}' ] && found_args=1
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

func TestWorkspaceFingerprintChangesWithSourceState(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "probe.go")
	if err := os.WriteFile(path, []byte("package probe\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	workspace := codeintelligence.Workspace{Root: root}
	first, err := workspaceFingerprint(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package probe\nfunc Changed() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := workspaceFingerprint(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("fingerprint did not change after source edit")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	third, err := workspaceFingerprint(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if second == third {
		t.Fatal("fingerprint did not change after source deletion")
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
found_tool=0
found_args=0
while [ "$#" -gt 0 ]; do
  case "$1" in
    --run-tool)
      shift
      [ "$1" = "codegraph_symbol_search" ] && found_tool=1
      ;;
    --tool-args)
      shift
      [ "$1" = '{"compact":true,"limit":7,"query":"Registry"}' ] && found_args=1
      ;;
  esac
  shift
done
[ "$found_tool" -eq 1 ] || exit 6
[ "$found_args" -eq 1 ] || exit 7
printf '{"results":[]}'
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
