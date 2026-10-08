package marksman

import (
	"context"
	doc "github.com/thinkerqaq/devtool/sdk/documentrealtime"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func documentRequest(root, path string, line int) doc.AnalyzeRequest {
	return doc.AnalyzeRequest{Root: root, Path: path, HeadingLine: line}
}

func TestSessionIsReusedAndSynchronizesArticleEdits(t *testing.T) {
	if _, err := exec.LookPath("marksman"); err != nil {
		t.Skip("Marksman CLI is not installed")
	}
	root := t.TempDir()
	article := filepath.Join(root, "doc.md")
	before := "# Title\n\nSee [Title](#title).\n"
	if err := os.WriteFile(article, []byte(before), 0600); err != nil {
		t.Fatal(err)
	}
	e := New()
	defer e.Close()
	first, err := e.analyze(t.Context(), documentRequest(root, "doc.md", 1))
	if err != nil || first.Status != "ok" || len(first.References) < 2 {
		t.Fatalf("first %+v error=%v", first, err)
	}
	session := e.sessions[root]
	if session == nil || session.cmd == nil {
		t.Fatal("expected cached provider process")
	}
	pid := session.cmd.Process.Pid
	after := "# Renamed\n\nSee [Renamed](#renamed).\n"
	if err := os.WriteFile(article, []byte(after), 0600); err != nil {
		t.Fatal(err)
	}
	second, err := e.analyze(context.Background(), documentRequest(root, "doc.md", 1))
	if err != nil || second.Status != "ok" || len(second.References) < 2 {
		t.Fatalf("updated %+v error=%v", second, err)
	}
	if got := e.sessions[root].cmd.Process.Pid; got != pid {
		t.Fatalf("Marksman was restarted: %d vs %d", pid, got)
	}
	if len(e.sessions[root].opened) != 1 || e.sessions[root].opened["file://"+filepath.ToSlash(article)].version != 2 {
		t.Fatalf("didChange not sent to existing session")
	}
	e.Close()
	if e.sessions != nil {
		t.Fatalf("Close did not clear sessions")
	}
	if strings.TrimSpace(second.WorkspaceScope) != "." {
		t.Fatalf("scope=%s", second.WorkspaceScope)
	}
}
