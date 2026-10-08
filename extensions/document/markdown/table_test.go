package markdown

import (
	"encoding/json"
	doc "github.com/thinkerqaq/devtool/sdk/document"
	"os"
	"path/filepath"
	"testing"
)

func TestGFMTableStructuredCellsAndLines(t *testing.T) {
	root := t.TempDir()
	article := "---\ntitle: Example\n---\n\n## Compare\n\n| 模型 | Scheduler |\n|---|:---:|\n| Go | G-M-P |\n| Java | JVM |\n\n~~~markdown\n| not | a table |\n~~~\n"
	if err := os.WriteFile(filepath.Join(root, "article.md"), []byte(article), 0600); err != nil {
		t.Fatal(err)
	}
	request, _ := json.Marshal(doc.InspectRequest{Root: root, Path: "article.md", IncludeTables: true})
	raw, err := New().Invoke(t.Context(), doc.MethodInspect, request)
	if err != nil {
		t.Fatal(err)
	}
	var got doc.InspectResponse
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Tables) != 1 {
		t.Fatalf("tables=%+v", got.Tables)
	}
	table := got.Tables[0]
	if table.Columns != 2 || table.StartLine != 7 || table.EndLine != 10 {
		t.Fatalf("table span/columns=%+v", table)
	}
	if len(table.Headers) != 2 || table.Headers[0] != "模型" || table.Headers[1] != "Scheduler" {
		t.Fatalf("headers=%+v", table.Headers)
	}
	if len(table.Rows) != 2 || table.Rows[0][0] != "Go" || table.Rows[1][1] != "JVM" {
		t.Fatalf("rows=%+v", table.Rows)
	}
	if table.Alignments[1] != "center" {
		t.Fatalf("alignments=%+v", table.Alignments)
	}
}
