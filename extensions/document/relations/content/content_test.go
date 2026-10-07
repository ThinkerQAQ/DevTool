package content

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	documentcontract "github.com/thinkerqaq/devtool/sdk/document"
)

func TestResolveArticleSeriesAndScopedNotes(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "src/content/articles/a.md"), `---
title: A
status: draft
language: zh
series: s
---
# A
`)
	mustWrite(t, filepath.Join(root, "src/content/articles/b.md"), `---
title: B
status: published
language: zh
series: s
---
# B
`)
	mustWrite(t, filepath.Join(root, "src/content/series/s.md"), `---
title: Series
status: active
relatedArticles:
  - a
  - b
relatedNoteScopes:
  - category: java
    topicPath:
      - JUC
    label: "Java · JUC"
---
`)
	mustWrite(t, filepath.Join(root, "src/content/notes/java/JUC/n1.md"), `---
title: N1
sourcePath: Java/JUC/n1.md
category: java
topic: JUC
status: historical
language: zh
---
`)
	mustWrite(t, filepath.Join(root, "src/content/notes/java/JUC/n2.md"), `---
title: N2
sourcePath: Java/JUC/n2.md
category: java
topic: JUC
topicPath:
  - id: JUC
    label: JUC
status: historical
language: zh
---
`)
	mustWrite(t, filepath.Join(root, "src/content/notes/java/JVM/no.md"), `---
title: JVM
sourcePath: Java/JVM/no.md
category: java
topic: JVM
---
`)

	ext := New()
	raw, err := json.Marshal(documentcontract.RelationsRequest{
		Root: root,
		Path: "src/content/articles/a.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := ext.Invoke(t.Context(), documentcontract.MethodResolveRelations, raw)
	if err != nil {
		t.Fatal(err)
	}
	var response documentcontract.RelationsResponse
	if err := json.Unmarshal(result, &response); err != nil {
		t.Fatal(err)
	}

	if response.RootNode.ID != "a" || response.RootNode.Kind != "article" {
		t.Fatalf("root = %#v", response.RootNode)
	}
	if got := countKind(response.Nodes, "article"); got != 2 {
		t.Fatalf("articles = %d, want 2", got)
	}
	if got := countKind(response.Nodes, "series"); got != 1 {
		t.Fatalf("series = %d, want 1", got)
	}
	if got := countKind(response.Nodes, "note"); got != 2 {
		t.Fatalf("notes = %d, want 2", got)
	}

	seriesKey := nodeKey(response.Nodes, "series", "s")
	if seriesKey == "" {
		t.Fatal("series node missing")
	}
	articleEdges := edgesOfType(response.Edges, "series_article")
	if len(articleEdges) != 2 {
		t.Fatalf("series_article edges = %d, want 2", len(articleEdges))
	}
	if articleEdges[0].Order == nil || *articleEdges[0].Order != 0 {
		t.Fatalf("first article order = %#v", articleEdges[0].Order)
	}
	if articleEdges[1].Order == nil || *articleEdges[1].Order != 1 {
		t.Fatalf("second article order = %#v", articleEdges[1].Order)
	}

	noteEdges := edgesOfType(response.Edges, "related_note")
	if len(noteEdges) != 2 {
		t.Fatalf("related_note edges = %d, want 2: %#v", len(noteEdges), noteEdges)
	}
	for _, edge := range noteEdges {
		if edge.From != seriesKey {
			t.Fatalf("note edge from = %q, want series", edge.From)
		}
		if edge.Label != "Java · JUC" || edge.Source != "relatedNoteScopes" {
			t.Fatalf("note edge metadata = %#v", edge)
		}
	}
}

func TestResolveSkipsEnglishArticleCopies(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "src/content/articles/a.md"), `---
title: A
series: s
language: zh
---
`)
	mustWrite(t, filepath.Join(root, "src/content/articles/en/a.md"), `---
title: A EN
series: s
language: en
---
`)
	mustWrite(t, filepath.Join(root, "src/content/series/s.md"), `---
title: S
relatedArticles:
  - a
---
`)

	ext := New()
	raw, _ := json.Marshal(documentcontract.RelationsRequest{Root: root, Path: "src/content/articles/a.md"})
	result, err := ext.Invoke(t.Context(), documentcontract.MethodResolveRelations, raw)
	if err != nil {
		t.Fatal(err)
	}
	var response documentcontract.RelationsResponse
	if err := json.Unmarshal(result, &response); err != nil {
		t.Fatal(err)
	}
	if got := countKind(response.Nodes, "article"); got != 1 {
		t.Fatalf("articles = %d, want canonical article only", got)
	}
}

func TestResolveHonorsMaxNodes(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "src/content/articles/a.md"), `---
title: A
series: s
---
`)
	mustWrite(t, filepath.Join(root, "src/content/articles/b.md"), `---
title: B
series: s
---
`)
	mustWrite(t, filepath.Join(root, "src/content/series/s.md"), `---
title: S
relatedArticles: [a, b]
---
`)

	ext := New()
	raw, _ := json.Marshal(documentcontract.RelationsRequest{
		Root: root, Path: "src/content/articles/a.md", MaxNodes: 2,
	})
	result, err := ext.Invoke(t.Context(), documentcontract.MethodResolveRelations, raw)
	if err != nil {
		t.Fatal(err)
	}
	var response documentcontract.RelationsResponse
	if err := json.Unmarshal(result, &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Nodes) != 2 {
		t.Fatalf("nodes = %d, want 2", len(response.Nodes))
	}
	if len(response.Warnings) == 0 {
		t.Fatal("expected truncation warning")
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func countKind(nodes []documentcontract.RelationNode, kind string) int {
	count := 0
	for _, node := range nodes {
		if node.Kind == kind {
			count++
		}
	}
	return count
}

func nodeKey(nodes []documentcontract.RelationNode, kind, id string) string {
	for _, node := range nodes {
		if node.Kind == kind && node.ID == id {
			return node.Key
		}
	}
	return ""
}

func edgesOfType(edges []documentcontract.RelationEdge, typ string) []documentcontract.RelationEdge {
	var result []documentcontract.RelationEdge
	for _, edge := range edges {
		if edge.Type == typ {
			result = append(result, edge)
		}
	}
	return result
}
