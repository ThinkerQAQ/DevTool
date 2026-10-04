package devenv

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestWorkspaceContainerUsesPersistentProjectMounts(t *testing.T) {
	root := t.TempDir()
	devenvHome := filepath.Join(root, ".devtool", "cache", "devenv-home")
	image := "example.invalid/dev-base@sha256:deadbeef"
	name := workspaceContainerName(root)

	args := workspaceCreateArgs(name, root, devenvHome, image, "1000", "1000")

	if slices.Contains(args, "--rm") {
		t.Fatalf("workspace container must be reusable, got --rm in args: %v", args)
	}
	if !slices.Contains(args, "-d") {
		t.Fatalf("workspace container must start detached: %v", args)
	}
	if !slices.Contains(args, "--label") || !slices.Contains(args, "devtool.managed=true") {
		t.Fatalf("managed label missing from docker args: %v", args)
	}

	wantWorkspaceMount := "type=bind,source=" + root + ",target=/workspace"
	if !slices.Contains(args, wantWorkspaceMount) {
		t.Fatalf("workspace mount missing from docker args: %v", args)
	}

	wantHomeMount := "type=bind,source=" + devenvHome + ",target=/tmp/devenv-home"
	if !slices.Contains(args, wantHomeMount) {
		t.Fatalf("persistent DevEnvironment home mount missing from docker args: %v", args)
	}
	if !slices.Contains(args, image) {
		t.Fatalf("configured image missing from docker args: %v", args)
	}
}

func TestWorkspaceExecReusesNamedContainer(t *testing.T) {
	root := t.TempDir()
	name := workspaceContainerName(root)

	args := workspaceExecArgs(name, "1000", "1000", "codegraph-server", "--version")
	if len(args) == 0 || args[0] != "exec" {
		t.Fatalf("expected docker exec, got: %v", args)
	}
	if !slices.Contains(args, name) {
		t.Fatalf("container name missing from docker exec args: %v", args)
	}
	if !slices.Contains(args, "--user") || !slices.Contains(args, "1000:1000") {
		t.Fatalf("host user mapping missing from docker exec args: %v", args)
	}
	if !slices.Contains(args, "HOME=/tmp/devenv-home") {
		t.Fatalf("persistent HOME missing from docker exec args: %v", args)
	}
	if got := strings.Join(args, " "); !strings.Contains(got, "codegraph-server --version") {
		t.Fatalf("tool command missing from docker exec args: %v", args)
	}
}

func TestWorkspaceContainerNameIsProjectScoped(t *testing.T) {
	first := workspaceContainerName(filepath.Join(t.TempDir(), "a"))
	second := workspaceContainerName(filepath.Join(t.TempDir(), "b"))
	if first == second {
		t.Fatalf("different project roots must not share a container name: %q", first)
	}
	if !strings.HasPrefix(first, "devtool-devenv-") {
		t.Fatalf("unexpected container name: %q", first)
	}
}
