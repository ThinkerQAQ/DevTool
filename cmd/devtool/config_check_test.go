package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Static validity must not be confused with runtime Provider wiring.
func TestConfigCheckRequiresLoadableProject(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	err := os.WriteFile(filepath.Join(root, ".devtool.toml"), []byte("version = 1\n[project]\nname = \"Fixture\"\n"), 0600)
	if err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := run(context.Background(), []string{"config", "validate"}, &output); err != nil {
		t.Fatalf("quick syntax check must pass: %v", err)
	}
	if !strings.HasPrefix(output.String(), "VALID ") {
		t.Fatalf("quick syntax output: %s", output.String())
	}

	output.Reset()
	err = run(context.Background(), []string{"config", "check"}, &output)
	if err == nil || !strings.Contains(err.Error(), "project extension is not configured") {
		t.Fatalf("preflight must reject missing project provider: %v", err)
	}
	if output.Len() != 0 {
		t.Fatalf("preflight must not claim success: %s", output.String())
	}
}

func TestConfigCheckRejectsUnexpectedArguments(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if err := os.WriteFile(filepath.Join(root, ".devtool.toml"), []byte("version = 1\n[project]\nname = \"Fixture\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run(context.Background(), []string{"config", "check", "--unknown"}, &bytes.Buffer{}); err == nil {
		t.Fatal("expected invalid flag rejection")
	}
}
