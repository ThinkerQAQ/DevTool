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
