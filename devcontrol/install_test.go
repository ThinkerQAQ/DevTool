package devcontrol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestActivateAndRollbackVersionedCandidate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("activation is WSL/Unix-only for now")
	}

	workspace := t.TempDir()
	runTestCommand(t, workspace, "git", "init", "-q")
	runTestCommand(t, workspace, "git", "config", "user.name", "DevTool Test")
	runTestCommand(t, workspace, "git", "config", "user.email", "devtool@example.test")
	writeTestFile(t, filepath.Join(workspace, "VERSION"), "1.2.3-test\n", 0o644)
	writeTestFile(t, filepath.Join(workspace, ".gitignore"), ".devtool/\n", 0o644)
	runTestCommand(t, workspace, "git", "add", "VERSION", ".gitignore")
	runTestCommand(t, workspace, "git", "commit", "-q", "-m", "fixture")
	commit := runTestCommand(t, workspace, "git", "rev-parse", "HEAD")

	outDir := filepath.Join(workspace, ".devtool", "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	candidate := filepath.Join(outDir, nextBinaryName())
	writeVersionScript(t, candidate, "1.2.3-test", commit)
	digest, err := sha256File(candidate)
	if err != nil {
		t.Fatal(err)
	}
	manifest := verificationManifest{
		SchemaVersion: 1,
		Version:       "1.2.3-test",
		Commit:        commit,
		SHA256:        digest,
		Binary:        nextBinaryName(),
	}
	raw, _ := json.Marshal(manifest)
	writeTestFile(t, verificationManifestPath(workspace), string(raw), 0o600)

	root := t.TempDir()
	layout := installLayout{
		BinPath:     filepath.Join(root, "bin", activeBinaryName()),
		DataRoot:    filepath.Join(root, "share", "devtool"),
		VersionsDir: filepath.Join(root, "share", "devtool", "versions"),
		StatePath:   filepath.Join(root, "share", "devtool", "install-state.json"),
	}
	if err := os.MkdirAll(filepath.Dir(layout.BinPath), 0o755); err != nil {
		t.Fatal(err)
	}
	writeVersionScript(t, layout.BinPath, "0.9.0", "legacy123")

	active, previous, err := activateVerifiedCandidate(workspace, layout)
	if err != nil {
		t.Fatalf("activateVerifiedCandidate() error = %v", err)
	}
	if active.Version != "1.2.3-test" || active.Commit != commit {
		t.Fatalf("active = %+v", active)
	}
	if previous == nil || previous.Version != "0.9.0" {
		t.Fatalf("previous = %+v", previous)
	}
	assertVersionScript(t, layout.BinPath, "1.2.3-test")

	rolledBack, err := rollbackInstalled(layout)
	if err != nil {
		t.Fatalf("rollbackInstalled() error = %v", err)
	}
	if rolledBack.Version != "0.9.0" {
		t.Fatalf("rolledBack = %+v", rolledBack)
	}
	assertVersionScript(t, layout.BinPath, "0.9.0")
}

func writeVersionScript(t *testing.T, path, version, commit string) {
	t.Helper()
	script := fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = \"version\" ] && [ \"$2\" = \"--json\" ]; then\n  printf '%%s\\n' '%s'\n  exit 0\nfi\nexit 2\n",
		fmt.Sprintf("{\"version\":%q,\"commit\":%q,\"dirty\":false}", version, commit),
	)
	writeTestFile(t, path, script, 0o755)
}

func assertVersionScript(t *testing.T, path, want string) {
	t.Helper()
	raw, err := exec.Command(path, "version", "--json").Output()
	if err != nil {
		t.Fatalf("%s version: %v", path, err)
	}
	var got struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode %s version: %v", path, err)
	}
	if got.Version != want {
		t.Fatalf("%s version = %q, want %q", path, got.Version, want)
	}
}

func runTestCommand(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v: %s", name, args, err, out)
	}
	return string(bytes.TrimSpace(out))
}

func writeTestFile(t *testing.T, path, value string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), mode); err != nil {
		t.Fatal(err)
	}
}
