package merman

import (
	"context"
	"encoding/json"
	diagram "github.com/thinkerqaq/devtool/sdk/diagram"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSemanticGraphAndCrossLayerEdges(t *testing.T) {
	cli, err := exec.LookPath("merman-cli")
	if err != nil {
		t.Skip("merman-cli not installed")
	}
	g, err := New().semantic(t.Context(), cli, "flowchart TB\n subgraph A[Language]\n X[A] \n end\n subgraph B[Runtime]\n Y[B] \n end\n X -->|maps to| Y\n")
	if err != nil {
		t.Fatal(err)
	}
	if g.NodeCount != 2 || g.EdgeCount != 1 || len(g.Groups) != 2 || g.CrossLayerEdges != 1 || g.UnlabeledCrossLayerEdges != 0 {
		t.Fatalf("graph=%+v", g)
	}
}
func TestFullMarkdownSymbolsAndSemanticGraph(t *testing.T) {
	if _, err := exec.LookPath("merman-cli"); err != nil {
		t.Skip("merman-cli not installed")
	}
	if _, err := exec.LookPath("merman-lsp"); err != nil {
		t.Skip("merman-lsp not installed")
	}
	root := t.TempDir()
	source := "# Example\n\n## Architecture\n\n```mermaid\nflowchart TB\n subgraph L1[Language]\n A[A node]\n end\n subgraph L2[Runtime]\n B[B node]\n end\n A -->|mapping| B\n```\n"
	if err := os.WriteFile(filepath.Join(root, "article.md"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	m := "flowchart TB\n subgraph L1[Language]\n A[A node]\n end\n subgraph L2[Runtime]\n B[B node]\n end\n A -->|mapping| B\n"
	e := New()
	result, err := e.analyze(context.Background(), diagram.IntelligenceRequest{Root: root, Path: "article.md", Diagrams: []diagram.DiagramSource{
		{Index: 1, Language: "mermaid", Source: m, StartLine: 5, EndLine: 14},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if result.DiagnosticsStatus != "reported" || len(result.Results) != 1 {
		t.Fatalf("result=%+v", result)
	}
	r := result.Results[0]
	if !r.Complete || r.Status != "ok" || r.Graph.CrossLayerEdges != 1 || len(r.Symbols) == 0 {
		t.Fatalf("result=%+v", r)
	}
	if r.Graph.Nodes[0].Line <= 5 {
		t.Fatalf("missing Markdown source line: %+v", r.Graph.Nodes)
	}
}
func TestMissingProviderIsNotClean(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "article.md"), []byte("# Example\n"), 0600)
	e := New()
	e.cli = filepath.Join(root, "absent")
	e.lsp = filepath.Join(root, "absent")
	got, err := e.analyze(context.Background(), diagram.IntelligenceRequest{Root: root, Path: "article.md", Diagrams: []diagram.DiagramSource{
		{Index: 1, Language: "mermaid", StartLine: 2, EndLine: 4, Source: "flowchart LR\nA-->B"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got.DiagnosticsStatus == "reported" || got.Results[0].Complete || got.Results[0].Status != "unavailable" {
		t.Fatalf("false clean result %+v", got)
	}
}
func TestRejectEscape(t *testing.T) {
	root := t.TempDir()
	e := New()
	_, err := e.analyze(context.Background(), diagram.IntelligenceRequest{Root: root, Path: "../outside.md"})
	if err == nil {
		t.Fatal("missing path traversal guard")
	}
}
func TestInvokeContract(t *testing.T) {
	e := New()
	r, _ := json.Marshal(diagram.IntelligenceRequest{})
	_, err := e.Invoke(context.Background(), "not-analyze", r)
	if err == nil {
		t.Fatal("unknown method allowed")
	}
}
