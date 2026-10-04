package serena

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
  echo "Serena 1.test"
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
	if response.Provider != ExtensionID || response.Version != "Serena 1.test" {
		t.Fatalf("response = %#v", response)
	}
}

func TestVerifyRunsProjectHealthCheck(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is unix-only")
	}
	path := writeFixture(t, `#!/bin/sh
[ "$1" = "project" ] || exit 3
[ "$2" = "health-check" ] || exit 4
[ -d "$3" ] || exit 5
echo "health ok"
`)
	root := t.TempDir()
	e := &Extension{executable: path}
	payload, _ := json.Marshal(codeintelligence.Workspace{Root: root})
	raw, err := e.Invoke(context.Background(), codeintelligence.MethodVerify, payload)
	if err != nil {
		t.Fatal(err)
	}
	var response codeintelligence.VerifyResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatal(err)
	}
	if response.Provider != ExtensionID || response.Output != "health ok" {
		t.Fatalf("response = %#v", response)
	}
}

func TestMCPPassesProjectAndContext(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is unix-only")
	}
	path := writeFixture(t, `#!/bin/sh
[ "$1" = "start-mcp-server" ] || exit 3
[ "$2" = "--project" ] || exit 4
[ -d "$3" ] || exit 5
[ "$4" = "--context" ] || exit 6
[ "$5" = "codex" ] || exit 7
`)
	root := t.TempDir()
	e := &Extension{executable: path}
	payload, _ := json.Marshal(codeintelligence.MCPRequest{
		Workspace: codeintelligence.Workspace{Root: root},
		Context:   "codex",
	})
	raw, err := e.Invoke(context.Background(), codeintelligence.MethodMCP, payload)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(raw)) != "null" {
		t.Fatalf("result = %s, want null", raw)
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
