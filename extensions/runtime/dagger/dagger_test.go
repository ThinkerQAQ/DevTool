package dagger

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/thinkerqaq/devtool/sdk/portable"
)

func TestInvokeRejectsMissingFunction(t *testing.T) {
	e := New()
	raw, _ := json.Marshal(portable.Invocation{})
	if _, err := e.Invoke(context.Background(), portable.MethodInvoke, raw); err == nil {
		t.Fatal("Invoke() expected missing function error")
	}
}

func TestInvokePassesOutputToDagger(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is unix-only")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "dagger")
	script := `#!/bin/sh
found_output=0
found_json=0
for arg in "$@"; do
  if [ "$arg" = "--output=./dist" ]; then
    found_output=1
  fi
  if [ "$arg" = "--json" ]; then
    found_json=1
  fi
done
if [ "$found_output" -ne 1 ]; then
  echo "missing output flag" >&2
  exit 3
fi
if [ "$found_json" -ne 0 ]; then
  echo "output invocation must not request JSON" >&2
  exit 4
fi
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	e := &Extension{executable: path}
	payload, err := json.Marshal(portable.Invocation{
		Function: "package-artifact",
		Output:   "./dist",
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := e.Invoke(context.Background(), portable.MethodInvoke, payload)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "null" {
		t.Fatalf("result = %s, want null", raw)
	}
}

func TestDoctorUsesConfiguredExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is unix-only")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "dagger")
	script := `#!/bin/sh
if [ "$1" = "version" ]; then
  echo "dagger fake"
  exit 0
fi
if [ "$1" = "api" ] && [ "$2" = "query" ]; then
  cat >/dev/null
  echo '{"data":{"container":{"from":{"withExec":{"stdout":"dagger-smoke"}}}}}'
  exit 0
fi
exit 2
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	e := &Extension{executable: path}
	raw, err := e.Invoke(context.Background(), portable.MethodDoctor, nil)
	if err != nil {
		t.Fatal(err)
	}
	var response portable.DoctorResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatal(err)
	}
	if response.Provider != ExtensionID || response.Smoke != "dagger-smoke" {
		t.Fatalf("response = %#v", response)
	}
}
