package dagger

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

func TestInvokePassesWorkspaceRelativeModuleAndOutputToDagger(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is unix-only")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "dagger")
	script := `#!/bin/sh
if [ "$1" != "-W" ]; then
  echo "workspace flag must precede api call" >&2
  exit 2
fi
if [ "$3" != "api" ] || [ "$4" != "call" ]; then
  echo "missing api call" >&2
  exit 3
fi
if [ "$5" != "-m" ]; then
  echo "module flag must belong to api call" >&2
  exit 4
fi
if [ "$6" != "$2/.dagger/modules/example" ]; then
  echo "module path was not resolved relative to workspace: $6" >&2
  exit 5
fi
if [ "$7" != "package-artifact" ]; then
  echo "unexpected function: $7" >&2
  exit 6
fi
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
  exit 7
fi
if [ "$found_json" -ne 0 ]; then
  echo "output invocation must not request JSON" >&2
  exit 8
fi
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	workspace := filepath.Join(dir, "workspace")
	e := &Extension{executable: path}
	payload, err := json.Marshal(portable.Invocation{
		Workspace: workspace,
		Module:    ".dagger/modules/example",
		Function:  "package-artifact",
		Output:    "./dist",
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

func TestInvokePropagatesDaggerFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture is unix-only")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "dagger")
	script := "#!/bin/sh\necho 'dagger exploded' >&2\nexit 7\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	e := &Extension{executable: path}
	payload, err := json.Marshal(portable.Invocation{Function: "verify"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.Invoke(context.Background(), portable.MethodInvoke, payload)
	if err == nil {
		t.Fatal("Invoke() expected Dagger failure")
	}
	if !strings.Contains(err.Error(), "dagger exploded") {
		t.Fatalf("Invoke() error = %q, want Dagger stderr", err)
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