package extensionloader

import (
	"context"
	"os"
	"testing"

	"github.com/thinkerqaq/devtool/core/project"
)

func TestCacheOutputNameChangesWithImplementation(t *testing.T) {
	a := cacheOutputName("environment", ".", "./extensions/environment/docker/cmd/provider")
	b := cacheOutputName("environment", ".", "./extensions/environment/local/cmd/provider")
	if a == b {
		t.Fatalf("cache keys should differ: %q", a)
	}
	if got := cacheOutputName("environment", ".", "./extensions/environment/local/cmd/provider"); got != b {
		t.Fatalf("cache key is not deterministic: %q != %q", got, b)
	}
}

func TestGoModuleTarget(t *testing.T) {
	target, binary, err := goModuleTarget(
		"github.com/thinkerqaq/devtool",
		"a8ec384948328f5ca9f4585970b19ab64fd65d08",
		"./extensions/intelligence/codegraph/cmd/provider",
	)
	if err != nil {
		t.Fatal(err)
	}
	if want := "github.com/thinkerqaq/devtool/extensions/intelligence/codegraph/cmd/provider@a8ec384948328f5ca9f4585970b19ab64fd65d08"; target != want {
		t.Fatalf("target = %q, want %q", target, want)
	}
	if binary != "provider" {
		t.Fatalf("binary = %q, want provider", binary)
	}
}

func TestGoModuleTargetRejectsParentTraversal(t *testing.T) {
	if _, _, err := goModuleTarget("example.com/mod", "v1.0.0", "../provider"); err == nil {
		t.Fatal("expected parent traversal to be rejected")
	}
}

func TestGoModuleCacheIncludesVersion(t *testing.T) {
	a := goModuleCacheOutputName("codegraph", "example.com/mod", "v1.0.0", "./cmd/provider")
	b := goModuleCacheOutputName("codegraph", "example.com/mod", "v1.0.1", "./cmd/provider")
	if a == b {
		t.Fatalf("version must participate in cache identity: %q", a)
	}
}

func TestResolveGoModuleProviderIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	if os.Getenv("DEVTOOL_EXTENSION_LOADER_INTEGRATION") == "" {
		t.Skip("set DEVTOOL_EXTENSION_LOADER_INTEGRATION=1 to run")
	}
	root := t.TempDir()
	p := project.Project{Root: root}
	got, err := resolveGoModule(context.Background(), p, "environment", map[string]any{
		"module":  "github.com/thinkerqaq/devtool",
		"version": "a8ec384948328f5ca9f4585970b19ab64fd65d08",
		"package": "./extensions/environment/local/cmd/provider",
	})
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(got)
	if err != nil {
		t.Fatal(err)
	}
	if info.IsDir() {
		t.Fatalf("resolved provider is a directory: %s", got)
	}
}
