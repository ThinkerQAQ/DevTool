package dagger

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/thinkerqaq/devtool/sdk/portable"
)

func TestInvokeRealDagger(t *testing.T) {
	if os.Getenv("DEVTOOL_DAGGER_INTEGRATION") != "1" {
		t.Skip("set DEVTOOL_DAGGER_INTEGRATION=1 to run the real Dagger integration test")
	}

	workspace, err := filepath.Abs(filepath.Join("testdata", "workspace"))
	if err != nil {
		t.Fatal(err)
	}

	request, err := json.Marshal(portable.Invocation{
		Workspace: workspace,
		Module:    "./.dagger/modules/smoke",
		Function:  "verify",
	})
	if err != nil {
		t.Fatal(err)
	}

	raw, err := New().Invoke(context.Background(), portable.MethodInvoke, request)
	if err != nil {
		t.Fatal(err)
	}

	var got string
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode Dagger result %q: %v", string(raw), err)
	}
	if got != "portable-invoke-ok" {
		t.Fatalf("result = %q, want portable-invoke-ok", got)
	}
}
