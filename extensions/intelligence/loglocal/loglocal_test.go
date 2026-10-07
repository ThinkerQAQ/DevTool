package loglocal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	logintelligence "github.com/thinkerqaq/devtool/sdk/logintelligence"
)

func TestAnalyzeArbitraryLogBuildsBoundedEvidence(t *testing.T) {
	root := t.TempDir()
	source := strings.Join([]string{
		"2026-10-07T10:00:00Z INFO server started",
		"2026-10-07T10:00:01Z WARN request 42 slow",
		"2026-10-07T10:00:02Z ERROR request 123 failed token=abc123",
		"    at handler.go:44",
		"2026-10-07T10:00:03Z ERROR request 456 failed token=def456",
		"    at handler.go:44",
		"2026-10-07T10:00:04Z INFO server stopped",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(root, "service.log"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	response, err := analyze(logintelligence.AnalyzeRequest{
		Root:  root,
		Path:  "service.log",
		Query: "request",
		Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	if response.Provider != ExtensionID || response.Format != "plain" {
		t.Fatalf("identity = provider %q format %q", response.Provider, response.Format)
	}
	if response.Summary.Lines != 7 || response.Summary.Errors != 2 || response.Summary.Warnings != 1 {
		t.Fatalf("summary = %#v", response.Summary)
	}
	if response.Summary.Matched != 3 {
		t.Fatalf("matched = %d, want 3", response.Summary.Matched)
	}
	if response.Levels["error"] != 2 || response.Levels["warn"] != 1 || response.Levels["info"] != 2 {
		t.Fatalf("levels = %#v", response.Levels)
	}
	if len(response.Patterns) == 0 || response.Patterns[0].Count != 2 {
		t.Fatalf("patterns = %#v", response.Patterns)
	}
	if len(response.Evidence) != 3 {
		t.Fatalf("evidence count = %d, want 3", len(response.Evidence))
	}
	if !strings.Contains(response.Evidence[1].Text, "at handler.go:44") {
		t.Fatalf("error evidence lost stack continuation: %#v", response.Evidence[1])
	}
	for _, evidence := range response.Evidence {
		if strings.Contains(evidence.Text, "abc123") || strings.Contains(evidence.Text, "def456") {
			t.Fatalf("secret was not redacted: %#v", evidence)
		}
	}
}

func TestAnalyzeDetectsJSONLog(t *testing.T) {
	root := t.TempDir()
	source := "{\"time\":\"2026-10-07T10:00:00Z\",\"level\":\"error\",\"message\":\"boom\"}\n"
	if err := os.WriteFile(filepath.Join(root, "events.jsonl"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	response, err := analyze(logintelligence.AnalyzeRequest{Root: root, Path: "events.jsonl"})
	if err != nil {
		t.Fatal(err)
	}
	if response.Format != "json" {
		t.Fatalf("format = %q, want json", response.Format)
	}
	if response.Summary.Errors != 1 {
		t.Fatalf("summary = %#v", response.Summary)
	}
}

func TestAnalyzeRejectsPathOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	path := filepath.Join(outside, "outside.log")
	if err := os.WriteFile(path, []byte("ERROR outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := analyze(logintelligence.AnalyzeRequest{Root: root, Path: path}); err == nil {
		t.Fatal("expected path outside root to fail")
	}
}

func TestAnalyzeRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "outside.log")
	if err := os.WriteFile(target, []byte("ERROR outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "escape.log")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	if _, err := analyze(logintelligence.AnalyzeRequest{Root: root, Path: "escape.log"}); err == nil {
		t.Fatal("expected symlink escape to fail")
	}
}

func TestDetectLevelIgnoresSeverityWordsDeepInPayload(t *testing.T) {
	line := strings.Repeat("x", levelProbeBytes+20) + " panic should not be treated as a level"
	if got := detectLevel(line); got != "" {
		t.Fatalf("level = %q, want empty", got)
	}
	if got := detectLevel("2026-10-07T12:00:00Z service ERROR request failed"); got != "error" {
		t.Fatalf("level = %q, want error", got)
	}
}

func TestBoundedRedactCapsEvidence(t *testing.T) {
	input := "token=secret " + strings.Repeat("x", maxEvidenceBytes*2)
	got := boundedRedact(input)
	if len(got) > maxEvidenceBytes {
		t.Fatalf("evidence length = %d, limit = %d", len(got), maxEvidenceBytes)
	}
	if strings.Contains(got, "secret") {
		t.Fatalf("secret was not redacted: %q", got)
	}
	if !strings.HasSuffix(got, "...[truncated]") {
		t.Fatalf("bounded evidence did not mark truncation: %q", got[len(got)-32:])
	}
}

func TestParseLnavFormat(t *testing.T) {
	format, err := parseLnavFormat([]byte("log_format,count\ngeneric_log,42\n"))
	if err != nil {
		t.Fatal(err)
	}
	if format != "generic_log" {
		t.Fatalf("format = %q, want generic_log", format)
	}
}

func TestInvokeRejectsUnknownMethod(t *testing.T) {
	ext := New()
	payload, _ := json.Marshal(logintelligence.AnalyzeRequest{Root: t.TempDir(), Path: "missing.log"})
	if _, err := ext.Invoke(t.Context(), "unknown", payload); err == nil {
		t.Fatal("expected unknown method to fail")
	}
}
