package markdown

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	doc "github.com/thinkerqaq/devtool/sdk/document"
)

func TestTablesRespectSelectedSection(t *testing.T) {
	root := t.TempDir()
	contents := "## 1. First\n\n| Key | Value |\n|---|---|\n| A | One |\n\n## 2. Second\n\n| Key | Value |\n|---|---|\n| B | Two |\n"
	if err := os.WriteFile(filepath.Join(root, "doc.md"), []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		Section string
		Want    string
	}{
		{"1", "One"}, {"2", "Two"},
	} {
		args, _ := json.Marshal(doc.InspectRequest{Root: root, Path: "doc.md", Section: tc.Section, IncludeTables: true})
		payload, err := New().Invoke(t.Context(), doc.MethodInspect, args)
		if err != nil {
			t.Fatal(err)
		}
		var response doc.InspectResponse
		if err := json.Unmarshal(payload, &response); err != nil {
			t.Fatal(err)
		}
		if len(response.Tables) != 1 || response.Tables[0].Rows[0][1] != tc.Want {
			t.Fatalf("section=%s tables=%+v", tc.Section, response.Tables)
		}
	}
}
