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
	"github.com/thinkerqaq/devtool/sdk/readiness"
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

func TestReadinessClassifiesMissingGoplsFromSerenaStdout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is unix-only")
	}
	path := writeFixture(t, `#!/bin/sh
if [ "$1" = "--version" ]; then
  echo "Serena 1.test"
  exit 0
fi
if [ "$1" = "project" ] && [ "$2" = "health-check" ]; then
  echo "go: Found a Go version but gopls is not installed."
  echo "Please install gopls and add it to PATH."
  exit 1
fi
exit 2
`)
	root := t.TempDir()
	e := &Extension{executable: path}
	report, err := e.CheckReadiness(context.Background(), readiness.Request{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if report.Ready {
		t.Fatal("report unexpectedly ready")
	}
	if len(report.Issues) != 1 {
		t.Fatalf("issues = %#v", report.Issues)
	}
	issue := report.Issues[0]
	if issue.Kind != readiness.KindMissingDependency || issue.Resource != "gopls" {
		t.Fatalf("issue = %#v", issue)
	}
	if !strings.Contains(strings.ToLower(issue.Message), "gopls") {
		t.Fatalf("message = %q; want gopls detail", issue.Message)
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

func TestUnwrapMCPToolResultPrefersStructuredResult(t *testing.T) {
	raw := json.RawMessage(`{
		"content":[{"type":"text","text":"[{\"name_path\":\"ProjectHost\"}]"}],
		"isError":false,
		"structuredContent":{"result":"[{\"name_path\":\"ProjectHost\"}]"}
	}`)
	got, err := unwrapMCPToolResult(raw)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `[{"name_path":"ProjectHost"}]` {
		t.Fatalf("got %s", got)
	}
}

func TestUnwrapMCPToolResultPropagatesToolError(t *testing.T) {
	raw := json.RawMessage(`{"content":[{"type":"text","text":"boom"}],"isError":true}`)
	if _, err := unwrapMCPToolResult(raw); err == nil {
		t.Fatal("expected Serena tool error")
	}
}
