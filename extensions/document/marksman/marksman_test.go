package marksman

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	doc "github.com/thinkerqaq/devtool/sdk/documentrealtime"
)

func TestNormalizeSymbolsAndReferences(t *testing.T) {
	raw := json.RawMessage(`[{"name":"章节","range":{"start":{"line":2,"character":0},"end":{"line":3,"character":4}},"children":[{"name":"子节","range":{"start":{"line":4,"character":2},"end":{"line":5,"character":3}}}]}]`)
	symbols := parseSymbols(raw)
	if len(symbols) != 2 || symbols[0].Name != "章节" || symbols[0].Range.Start.Line != 3 || symbols[1].Depth != 2 {
		t.Fatalf("symbols=%+v", symbols)
	}
	root := t.TempDir()
	allowed := "file://" + filepath.ToSlash(filepath.Join(root, "inside.md"))
	outside := "file:///tmp/not-in-workspace.md"
	locs := parseLocations(json.RawMessage(`[{"uri":"`+allowed+`","range":{"start":{"line":1,"character":7},"end":{"line":1,"character":11}}},{"uri":"`+outside+`","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}}}]`), root)
	if len(locs) != 1 || locs[0].Path != "inside.md" || locs[0].Range.Start.Column != 8 {
		t.Fatalf("locations=%+v", locs)
	}
}

func TestMissingMarksmanIsExplicitlyUnavailable(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "doc.md"), []byte("# Heading\n"), 0600); err != nil {
		t.Fatal(err)
	}
	e := New()
	e.binary = filepath.Join(root, "does-not-exist")
	got, err := e.analyze(context.Background(), doc.AnalyzeRequest{Root: root, Path: "doc.md"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "unavailable" || got.DiagnosticsStatus != "not_reported" {
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
