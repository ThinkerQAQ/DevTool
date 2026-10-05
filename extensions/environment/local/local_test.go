package local

import (
	"path/filepath"
	"testing"

	environmentcontract "github.com/thinkerqaq/devtool/sdk/environment"
)

func TestCommandSpecRunsInsideProjectRoot(t *testing.T) {
	root := t.TempDir()
	workdir := filepath.Join(root, "sub")
	spec, err := commandSpec(environmentcontract.CommandRequest{
		Root:       root,
		WorkingDir: workdir,
		Executable: "gopls",
		Args:       []string{"version"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if spec.Program != "gopls" {
		t.Fatalf("Program = %q", spec.Program)
	}
	if spec.Dir != workdir {
		t.Fatalf("Dir = %q, want %q", spec.Dir, workdir)
	}
	if len(spec.Args) != 1 || spec.Args[0] != "version" {
		t.Fatalf("Args = %v", spec.Args)
	}
}

func TestCommandSpecRejectsWorkingDirectoryOutsideRoot(t *testing.T) {
	root := t.TempDir()
	_, err := commandSpec(environmentcontract.CommandRequest{
		Root:       root,
		WorkingDir: filepath.Dir(root),
		Executable: "gopls",
	})
	if err == nil {
		t.Fatal("expected working directory outside root to fail")
	}
}
