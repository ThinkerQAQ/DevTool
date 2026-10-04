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
found=0
for arg in "$@"; do
  if [ "$arg" = "--output=./dist" ]; then
    found=1
  fi
done
if [ "$found" -ne 1 ]; then
  echo "missing output flag" >&2
  exit 3
fi
echo '"ok"'
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	e := &Extension{executable: path}
	payload, err := json.Marshal(portable.Invocation{
		Function: "package",
		Output:   "./dist",
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := e.Invoke(context.Background(), portable.MethodInvoke, payload)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `"ok"` {
		t.Fatalf("result = %s", raw)
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
