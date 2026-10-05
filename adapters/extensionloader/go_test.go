package extensionloader

import "testing"

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
