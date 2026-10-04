package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverWalksToProjectRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ConfigFileName), []byte("version = 1\n[project]\nname = \"Example\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	p, err := Discover(nested)
	if err != nil {
		t.Fatal(err)
	}
	if p.Root != root {
		t.Fatalf("root = %q, want %q", p.Root, root)
	}
}
