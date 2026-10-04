package devenv

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/thinkerqaq/devtool/sdk/codeintelligence"
)

func TestCommandMountsProjectScopedDevEnvironmentHome(t *testing.T) {
	root := t.TempDir()
	cmd, err := Command(context.Background(), codeintelligence.Workspace{
		Root:             root,
		EnvironmentImage: "example.invalid/dev-base@sha256:deadbeef",
	}, "codegraph-server", "--version")
	if err != nil {
		t.Fatalf("Command() error = %v", err)
	}

	devenvHome := filepath.Join(root, ".devtool", "cache", "devenv-home")
	info, err := os.Stat(devenvHome)
	if err != nil {
		t.Fatalf("stat development environment home: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("development environment home is not a directory: %s", devenvHome)
	}

	wantWorkspaceMount := "type=bind,source=" + root + ",target=/workspace"
	if !slices.Contains(cmd.Args, wantWorkspaceMount) {
		t.Fatalf("workspace mount missing from docker args: %v", cmd.Args)
	}

	wantHomeMount := "type=bind,source=" + devenvHome + ",target=/tmp/devenv-home"
	if !slices.Contains(cmd.Args, wantHomeMount) {
		t.Fatalf("persistent DevEnvironment home mount missing from docker args: %v", cmd.Args)
	}
}
