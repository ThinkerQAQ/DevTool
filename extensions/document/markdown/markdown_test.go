package markdown

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	documentcontract "github.com/thinkerqaq/devtool/sdk/document"
)

func TestInspectBuildsOutlineAndExactSection(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "article.md")
	fence := string([]byte{96, 96, 96})
	source := "---\n" +
		"title: \"Example\"\n" +
		"status: draft\n" +
		"---\n\n" +
		"## 1. Java\n\n" +
		"Intro.\n\n" +
		"### 1.1 Model\n\n" +
		fence + "text\n" +
		"## fake heading\n" +
		fence + "\n\n" +
		"#### 1.1.1 Detail\n\n" +
		"Detail body.\n\n" +
		"## 2. Go\n\n" +
		"Go body.\n"
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	ext := New()
	if err := ext.Configure(map[string]any{"roots": []any{"."}}); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(documentcontract.InspectRequest{
		Root: root, Path: "article.md", Section: "1.1", IncludeContent: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := ext.Invoke(t.Context(), documentcontract.MethodInspect, raw)
	if err != nil {
		t.Fatal(err)
	}
	var response documentcontract.InspectResponse
	if err := json.Unmarshal(result, &response); err != nil {
		t.Fatal(err)
	}

	if response.Format != "markdown" {
		t.Fatalf("format = %q", response.Format)
	}
	if response.LineCount != strings.Count(source, "\n") {
		t.Fatalf("line count = %d, want %d", response.LineCount, strings.Count(source, "\n"))
	}
	if got := response.Frontmatter["title"]; got != "Example" {
		t.Fatalf("frontmatter title = %#v", got)
	}
	if len(response.Outline) != 2 {
		t.Fatalf("top-level sections = %d, want 2", len(response.Outline))
	}
	if response.Outline[0].Key != "1" || response.Outline[0].Title != "1. Java" {
		t.Fatalf("first section = %#v", response.Outline[0])
	}
	if len(response.Outline[0].Children) != 1 || response.Outline[0].Children[0].Key != "1.1" {
		t.Fatalf("nested outline = %#v", response.Outline[0].Children)
	}
	if response.SelectedSection == nil {
		t.Fatal("selected section is nil")
	}
	if response.SelectedSection.Key != "1.1" {
		t.Fatalf("selected key = %q", response.SelectedSection.Key)
	}
	if !strings.Contains(response.SelectedSection.Content, "#### 1.1.1 Detail") {
		t.Fatalf("selected section did not include nested child:\n%s", response.SelectedSection.Content)
	}
	if strings.Contains(response.SelectedSection.Content, "## 2. Go") {
		t.Fatalf("selected content crossed section boundary:\n%s", response.SelectedSection.Content)
	}
	for _, child := range response.Outline[0].Children {
		if child.Title == "fake heading" {
			t.Fatal("code fence created a false heading")
		}
	}
}

func TestInspectSelectsSectionByExactStartLine(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "article.md")
	source := "## Repeat\n\nFirst.\n\n## Repeat\n\nSecond.\n"
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	ext := New()
	if err := ext.Configure(map[string]any{"roots": []any{"."}}); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(documentcontract.InspectRequest{
		Root: root, Path: "article.md", SectionStartLine: 5, IncludeContent: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := ext.Invoke(t.Context(), documentcontract.MethodInspect, raw)
	if err != nil {
		t.Fatal(err)
	}
	var response documentcontract.InspectResponse
	if err := json.Unmarshal(result, &response); err != nil {
		t.Fatal(err)
	}
	if response.SelectedSection == nil {
		t.Fatal("selected section is nil")
	}
	if response.SelectedSection.StartLine != 5 {
		t.Fatalf("start line = %d, want 5", response.SelectedSection.StartLine)
	}
	if !strings.Contains(response.SelectedSection.Content, "Second.") {
		t.Fatalf("selected content = %q", response.SelectedSection.Content)
	}
	if strings.Contains(response.SelectedSection.Content, "First.") {
		t.Fatalf("selected content crossed into the first duplicate heading: %q", response.SelectedSection.Content)
	}
}

func TestInspectSelectsExactLineRange(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "article.md")
	source := "line1\nline2\nline3\nline4\nline5\n"
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	ext := New()
	if err := ext.Configure(map[string]any{"roots": []any{"."}}); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(documentcontract.InspectRequest{
		Root: root, Path: "article.md", RangeStartLine: 2, RangeEndLine: 4, IncludeContent: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := ext.Invoke(t.Context(), documentcontract.MethodInspect, raw)
	if err != nil {
		t.Fatal(err)
	}
	var response documentcontract.InspectResponse
	if err := json.Unmarshal(result, &response); err != nil {
		t.Fatal(err)
	}
	if response.SelectedRange == nil {
		t.Fatal("selected range is nil")
	}
	if response.SelectedRange.StartLine != 2 || response.SelectedRange.EndLine != 4 {
		t.Fatalf("range = %d..%d, want 2..4", response.SelectedRange.StartLine, response.SelectedRange.EndLine)
	}
	if response.SelectedRange.Content != "line2\nline3\nline4\n" {
		t.Fatalf("range content = %q", response.SelectedRange.Content)
	}
}

func TestInspectRejectsInvalidLineRange(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "article.md")
	if err := os.WriteFile(path, []byte("## 1. One\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ext := New()
	if err := ext.Configure(map[string]any{"roots": []any{"."}}); err != nil {
		t.Fatal(err)
	}

	cases := []documentcontract.InspectRequest{
		{Root: root, Path: "article.md", RangeStartLine: 1},
		{Root: root, Path: "article.md", RangeEndLine: 1},
		{Root: root, Path: "article.md", RangeStartLine: 2, RangeEndLine: 1},
		{Root: root, Path: "article.md", RangeStartLine: 1, RangeEndLine: 3},
		{Root: root, Path: "article.md", Section: "1", RangeStartLine: 1, RangeEndLine: 1},
	}
	for _, request := range cases {
		raw, _ := json.Marshal(request)
		if _, err := ext.Invoke(t.Context(), documentcontract.MethodInspect, raw); err == nil {
			t.Fatalf("expected invalid request to fail: %#v", request)
		}
	}
}

func TestInspectRejectsPathOutsideConfiguredRoots(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src", "content"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ext := New()
	if err := ext.Configure(map[string]any{"roots": []any{"src/content"}}); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(documentcontract.InspectRequest{Root: root, Path: "README.md"})
	if _, err := ext.Invoke(t.Context(), documentcontract.MethodInspect, raw); err == nil {
		t.Fatal("expected path outside configured roots to fail")
	}
}

func TestInspectRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	contentRoot := filepath.Join(root, "src", "content")
	if err := os.MkdirAll(contentRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	target := filepath.Join(outside, "outside.md")
	if err := os.WriteFile(target, []byte("# outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(contentRoot, "linked.md")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink is unavailable: %v", err)
	}

	ext := New()
	if err := ext.Configure(map[string]any{"roots": []any{"src/content"}}); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(documentcontract.InspectRequest{Root: root, Path: "src/content/linked.md"})
	if _, err := ext.Invoke(t.Context(), documentcontract.MethodInspect, raw); err == nil {
		t.Fatal("expected symlink escape to fail")
	}
}

func TestSelectSectionRejectsAmbiguousPrefix(t *testing.T) {
	sections := []flatSection{
		{Title: "Alpha One", Level: 2},
		{Title: "Alpha Two", Level: 2},
	}
	if _, err := selectSection(sections, "Alpha"); err == nil {
		t.Fatal("expected ambiguous prefix to fail")
	}
}
