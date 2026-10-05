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
