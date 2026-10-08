package cli

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	diagram "github.com/thinkerqaq/devtool/sdk/diagram"
)

func TestRenderUnavailableIsNotValid(t *testing.T) {
	e := New()
	e.mermaid = filepath.Join(t.TempDir(), "missing-mmdc")
	r := e.render(t.Context(), diagram.RenderRequest{Language: "mermaid", Source: "flowchart TB\n A-->B\n"})
	if r.Status != "unavailable" || r.ArtifactPath != "" {
		t.Fatalf("unexpected result: %+v", r)
	}
}

func TestRenderByInstalledExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix fixture executable")
	}
	dir := t.TempDir()
	executable := filepath.Join(dir, "mmdc")
	script := "#!/bin/sh\nwhile [ \"$#\" -gt 0 ]; do\n  if [ \"$1\" = \"-o\" ]; then\n    shift\n    printf '<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>' > \"$1\"\n  fi\n  shift\ndone\n"
	if err := os.WriteFile(executable, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	e := New()
	e.mermaid = executable
	r := e.render(context.Background(), diagram.RenderRequest{Language: "mermaid", Source: "flowchart TB\n A-->B\n"})
	if r.Status != "rendered" || r.ArtifactPath == "" {
		t.Fatalf("unexpected result: %+v", r)
	}
	svg, err := os.ReadFile(r.ArtifactPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(svg), "<svg") {
		t.Fatalf("bad output: %q", svg)
	}
	t.Cleanup(func() { os.Remove(r.ArtifactPath) })
}

func TestRejectBadRendererOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix fixture executable")
	}
	path := filepath.Join(t.TempDir(), "bad-mmdc")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	e := New()
	e.mermaid = path
	r := e.render(t.Context(), diagram.RenderRequest{Language: "mermaid", Source: "broken"})
	if r.Status != "failed" {
		t.Fatalf("unexpected result: %+v", r)
	}
}
