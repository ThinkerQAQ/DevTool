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

	t.Run("value result", func(t *testing.T) {
		request, err := json.Marshal(portable.Invocation{
			Workspace: workspace,
			Module:    ".dagger/modules/smoke",
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
	})

	t.Run("file artifact export", func(t *testing.T) {
		output := filepath.Join(t.TempDir(), "artifact.txt")
		request, err := json.Marshal(portable.Invocation{
			Workspace: workspace,
			Module:    ".dagger/modules/smoke",
			Function:  "package-file",
			Output:    output,
		})
		if err != nil {
			t.Fatal(err)
		}

		raw, err := New().Invoke(context.Background(), portable.MethodInvoke, request)
		if err != nil {
			t.Fatal(err)
		}
		if string(raw) != "null" {
			t.Fatalf("result = %s, want null", raw)
		}

		got, err := os.ReadFile(output)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "file-output-ok\n" {
			t.Fatalf("artifact = %q, want file-output-ok", string(got))
		}
	})

	t.Run("directory artifact export", func(t *testing.T) {
		output := filepath.Join(t.TempDir(), "dist")
		request, err := json.Marshal(portable.Invocation{
			Workspace: workspace,
			Module:    ".dagger/modules/smoke",
			Function:  "package-directory",
			Output:    output,
		})
		if err != nil {
			t.Fatal(err)
		}

		raw, err := New().Invoke(context.Background(), portable.MethodInvoke, request)
		if err != nil {
			t.Fatal(err)
		}
		if string(raw) != "null" {
			t.Fatalf("result = %s, want null", raw)
		}

		got, err := os.ReadFile(filepath.Join(output, "artifact.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "directory-output-ok\n" {
			t.Fatalf("artifact = %q, want directory-output-ok", string(got))
		}
	})
}