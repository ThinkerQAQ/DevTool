package codegraph

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

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

func TestQueryPassesWorkspacesAndJSON(t *testing.T) {
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
      [ "$1" = "codegraph_analyze_impact" ] && found_tool=1
      ;;
    --tool-args)
      shift
      [ "$1" = '{"symbol":"OpenProject"}' ] && found_args=1
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
	payload, err := json.Marshal(codeintelligence.GraphQuery{
		Workspace: codeintelligence.Workspace{Root: root, Workspaces: []string{root, "child"}},
		Tool:      "analyze_impact",
		Args:      json.RawMessage(`{"symbol":"OpenProject"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := e.Invoke(context.Background(), codeintelligence.MethodQuery, payload)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(raw)) != `{"ok":true}` {
		t.Fatalf("result = %s", raw)
	}
}

func TestQueryRejectsInvalidArgs(t *testing.T) {
	e := &Extension{executable: "unused"}
	payload, err := json.Marshal(codeintelligence.GraphQuery{
		Workspace: codeintelligence.Workspace{Root: t.TempDir()},
		Tool:      "analyze_impact",
	})
	if err != nil {
		t.Fatal(err)
	}
	var request codeintelligence.GraphQuery
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatal(err)
	}
	request.Args = json.RawMessage(`{`)
	payload, _ = json.Marshal(request)
	if _, err := e.Invoke(context.Background(), codeintelligence.MethodQuery, payload); err == nil {
		t.Fatal("Invoke() expected invalid JSON error")
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
