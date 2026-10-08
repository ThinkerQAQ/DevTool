package marksman

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	doc "github.com/thinkerqaq/devtool/sdk/documentrealtime"
)

func TestNormalizeReferencesAndBoundOutput(t *testing.T) {
	root := t.TempDir()
	good := "file://" + filepath.ToSlash(filepath.Join(root, "inside.md"))
	bad := "file:///tmp/not-in-workspace.md"
	locations, truncated := parseLocations(json.RawMessage(`[{"uri":"`+good+`","range":{"start":{"line":1,"character":7},"end":{"line":1,"character":11}}},{"uri":"`+bad+`","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}]`), root)
	if truncated || len(locations) != 1 || locations[0].Path != "inside.md" || locations[0].Range.Start.Column != 8 {
		t.Fatalf("got %+v %v", locations, truncated)
	}
}

func TestMissingMarksmanIsExplicitlyUnavailable(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "doc.md"), []byte("# Heading\n"), 0600); err != nil {
		t.Fatal(err)
	}
	e := New()
	e.binary = filepath.Join(root, "does-not-exist")
	got, err := e.analyze(context.Background(), doc.AnalyzeRequest{Root: root, Path: "doc.md", HeadingLine: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "unavailable" {
		t.Fatalf("response=%+v", got)
	}
}

func TestConfigurationRejectsOutOfWorkspaceScopes(t *testing.T) {
	e := New()
	for _, config := range []map[string]any{
		{"workspace_roots": []any{"../outside"}},
		{"workspace_roots": []any{"/outside"}},
		{"timeout_seconds": 0},
		{"binary": ""},
	} {
		if err := e.Configure(config); err == nil {
			t.Fatalf("accepted invalid config: %+v", config)
		}
	}
}
