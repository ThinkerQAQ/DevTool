package extensionloader

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/thinkerqaq/devtool/core/config"
	"github.com/thinkerqaq/devtool/core/project"
)

func TestBatchedDependenciesKeepProviderSpecificInvalidation(t *testing.T) {
	root := t.TempDir()
	write := func(path, text string) {
		t.Helper()
		target := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(text), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/batched\n\ngo 1.24\n")
	write("common/common.go", "package common\nfunc Name() string { return \"a\" }\n")
	write("other/other.go", "package other\nfunc Name() string { return \"b\" }\n")
	write("cmd/a/main.go", "package main\nimport \"example.com/batched/common\"\nfunc main() { _ = common.Name() }\n")
	write("cmd/b/main.go", "package main\nimport \"example.com/batched/other\"\nfunc main() { _ = other.Name() }\n")

	p := project.Project{Root: root, Config: config.Config{
		Extension: map[string]config.Extension{
			"project": {Loader: "go", LoaderConfig: map[string]any{"module": ".", "package": "./cmd/a"}},
			"b":       {Loader: "go", LoaderConfig: map[string]any{"module": ".", "package": "./cmd/b"}},
		},
	}}
	snapshots := batchLocalDependencies(t.Context(), p)
	if len(snapshots) != 2 {
		t.Fatalf("snapshots = %d, want 2", len(snapshots))
	}
	for _, pkg := range []string{"./cmd/a", "./cmd/b"} {
		snapshot, ok := snapshots[dependencyKey(root, pkg)]
		if !ok {
			t.Fatalf("missing snapshot %s", pkg)
		}
		dirs, roots, err := goDependencyDirs(t.Context(), root, root, pkg)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(snapshot.dirs, dirs) || !slices.Equal(snapshot.modules, roots) {
			t.Fatalf("batch differs from individual %s:\n%s\n%s\n%s\n%s", pkg, snapshot.dirs, dirs, snapshot.modules, roots)
		}
	}

	output := filepath.Join(root, "a-bin")
	cmd := exec.CommandContext(t.Context(), "go", "build", "-o", output, "./cmd/a")
	cmd.Dir = root
	if outputBytes, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fixture: %v: %s", err, outputBytes)
	}
	check := func(want bool) {
		t.Helper()
		snapshot := snapshots[dependencyKey(root, "./cmd/a")]
		actual, err := goBuildNeedsBuildWithSnapshot(t.Context(), root, root, "./cmd/a", output, &snapshot)
		if err != nil || actual != want {
			t.Fatalf("needs rebuild: got %v err %v, want %v", actual, err, want)
		}
	}
	check(false)

	future := time.Now().Add(3 * time.Second)
	if err := os.Chtimes(filepath.Join(root, "other/other.go"), future, future); err != nil {
		t.Fatal(err)
	}
	check(false) // cmd/a must not be invalidated by unrelated Provider code

	if err := os.Chtimes(filepath.Join(root, "common/common.go"), future, future); err != nil {
		t.Fatal(err)
	}
	check(true) // transitive Go dependency invalidates Provider a
}
