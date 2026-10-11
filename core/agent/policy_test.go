package agent

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thinkerqaq/devtool/adapters/agentpolicy/gitpatch"
	"github.com/thinkerqaq/devtool/core/config"
	"github.com/thinkerqaq/devtool/core/registry"
	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
)

type contextProvider struct {
	degraded bool
}

func (p contextProvider) ListTools(context.Context, agentsdk.Session) ([]agentsdk.Tool, error) {
	return []agentsdk.Tool{{Name: "code_context", Definition: json.RawMessage(`{"name":"code_context","inputSchema":{"type":"object"}}`)}}, nil
}

func (p contextProvider) CallTool(context.Context, agentsdk.Session, string, json.RawMessage) (json.RawMessage, error) {
	body := `{"indexed":{"items":[]},"realtime":{"diagnostics":[]}}`
	if p.degraded {
		body = `{"indexed":{"items":[]},"realtime":{"diagnostics":[]},"degraded":{"realtime":"offline"}}`
	}
	return json.Marshal(map[string]any{"content": []map[string]string{{"type": "text", "text": body}}})
}

func repoCommand(t *testing.T, root string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=DevTool Test", "GIT_AUTHOR_EMAIL=test@example.org", "GIT_COMMITTER_NAME=DevTool Test", "GIT_COMMITTER_EMAIL=test@example.org")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return out
}

func testGateway(t *testing.T, branch string, degraded bool) (*Gateway, string) {
	t.Helper()
	root := t.TempDir()
	repoCommand(t, root, "init", "-q", "-b", branch)
	if err := os.MkdirAll(filepath.Join(root, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "app.go"), []byte("package src\n\nconst Value = 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "other.go"), []byte("package src\n\nconst Other = 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	repoCommand(t, root, "add", ".")
	repoCommand(t, root, "commit", "-q", "-m", "baseline")
	reg := registry.New()
	if err := reg.ProvideAgentTools("context.mock", contextProvider{degraded: degraded}); err != nil {
		t.Fatal(err)
	}
	g := NewGateway(reg, agentsdk.Session{ProjectRoot: root}).WithPolicy(gitpatch.New(root, config.AgentPolicy{Enabled: true}))
	return g, root
}

func makePatch(t *testing.T, root, path, newText string) string {
	t.Helper()
	filename := filepath.Join(root, path)
	if err := os.WriteFile(filename, []byte(newText), 0644); err != nil {
		t.Fatal(err)
	}
	diff := string(repoCommand(t, root, "diff", "--", path))
	repoCommand(t, root, "checkout", "--", path)
	if diff == "" {
		t.Fatal("patch was empty")
	}
	return diff
}

func issue(t *testing.T, g *Gateway, path string) string {
	t.Helper()
	raw, err := json.Marshal(map[string]string{"objective": "Understand source before editing", "path": path})
	if err != nil {
		t.Fatal(err)
	}
	response, err := g.callTool(context.Background(), "code_context", raw)
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(response, &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Content) == 0 {
		t.Fatal("code_context returned no content")
	}
	var body struct {
		Token string `json:"context_token"`
	}
	if err := json.Unmarshal([]byte(envelope.Content[0].Text), &body); err != nil {
		t.Fatal(err)
	}
	return body.Token
}

func apply(t *testing.T, g *Gateway, token, patch string) (json.RawMessage, error) {
	t.Helper()
	args, _ := json.Marshal(map[string]string{"context_token": token, "patch": patch})
	return g.callTool(context.Background(), "workspace_apply_patch", args)
}

func TestPolicyGitPatchRealWorktree(t *testing.T) {
	g, root := testGateway(t, "feature/safe", false)
	defs, err := g.listTools(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != 2 || !strings.Contains(string(defs[1]), "workspace_apply_patch") {
		t.Fatalf("unexpected tool list: %s", defs)
	}
	patch := makePatch(t, root, "src/app.go", "package src\n\nconst Value = 2\n")
	token := issue(t, g, "src/app.go")
	if token == "" {
		t.Fatal("successfully scoped code context did not grant a token")
	}
	result, err := apply(t, g, token, patch)
	if err != nil || !strings.Contains(string(result), "applied") {
		t.Fatalf("apply failed: %v %s", err, result)
	}
	after, err := os.ReadFile(filepath.Join(root, "src/app.go"))
	if err != nil || !strings.Contains(string(after), "Value = 2") {
		t.Fatalf("patch didn't change source: %s %v", after, err)
	}
	if _, err := apply(t, g, token, patch); err == nil {
		t.Fatal("replaying a consumed token succeeded")
	}
}

func TestPolicyDeniesProtectedBranchAndStaleState(t *testing.T) {
	for _, branch := range []string{"main", "master"} {
		t.Run(branch, func(t *testing.T) {
			g, root := testGateway(t, branch, false)
			token := issue(t, g, "src/app.go")
			patch := makePatch(t, root, "src/app.go", "package src\n\nconst Value = 2\n")
			if _, err := apply(t, g, token, patch); err == nil || !strings.Contains(err.Error(), "protected") {
				t.Fatalf("did not block protected branch: %v", err)
			}
		})
	}
	t.Run("file changed", func(t *testing.T) {
		g, root := testGateway(t, "feature/stale", false)
		token := issue(t, g, "src/app.go")
		patch := makePatch(t, root, "src/app.go", "package src\n\nconst Value = 2\n")
		if err := os.WriteFile(filepath.Join(root, "src/app.go"), []byte("package src\n\nconst Value = 8\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := apply(t, g, token, patch); err == nil || !strings.Contains(err.Error(), "changed") {
			t.Fatalf("did not reject stale context: %v", err)
		}
	})
	t.Run("head changed", func(t *testing.T) {
		g, root := testGateway(t, "feature/head", false)
		token := issue(t, g, "src/app.go")
		patch := makePatch(t, root, "src/app.go", "package src\n\nconst Value = 2\n")
		repoCommand(t, root, "commit", "--allow-empty", "-q", "-m", "updated")
		if _, err := apply(t, g, token, patch); err == nil || !strings.Contains(err.Error(), "HEAD changed") {
			t.Fatalf("did not reject new HEAD: %v", err)
		}
	})
}

func TestPolicyRejectsUnsafeAndDegradedContext(t *testing.T) {
	g, root := testGateway(t, "feature/reject", false)
	patch := makePatch(t, root, "src/app.go", "package src\n\nconst Value = 2\n")
	t.Run("missing token", func(t *testing.T) {
		if _, err := apply(t, g, "made-up", patch); err == nil {
			t.Fatal("accepted unknown token")
		}
	})
	t.Run("cross file", func(t *testing.T) {
		token := issue(t, g, "src/app.go")
		other := makePatch(t, root, "src/other.go", "package src\n\nconst Other = 2\n")
		if _, err := apply(t, g, token, other); err == nil {
			t.Fatal("accepted patch for another file")
		}
	})
	t.Run("multi file", func(t *testing.T) {
		token := issue(t, g, "src/app.go")
		other := makePatch(t, root, "src/other.go", "package src\n\nconst Other = 2\n")
		if _, err := apply(t, g, token, patch+other); err == nil {
			t.Fatal("accepted multi-file diff")
		}
	})
	t.Run("expired token", func(t *testing.T) {
		token := issue(t, g, "src/app.go")
		g.policy = gitpatch.New(root, config.AgentPolicy{Enabled: true, ContextTTLSeconds: 1})
		token = issue(t, g, "src/app.go")
		time.Sleep(1100 * time.Millisecond)
		if _, err := apply(t, g, token, patch); err == nil {
			t.Fatal("accepted expired token")
		}
	})
	t.Run("symlink", func(t *testing.T) {
		if err := os.Symlink("app.go", filepath.Join(root, "src", "link.go")); err != nil {
			t.Fatal(err)
		}
		if token := issue(t, g, "src/link.go"); token != "" {
			t.Fatalf("accepted symlink scope: %s", token)
		}
	})
	t.Run("degraded", func(t *testing.T) {
		degraded, _ := testGateway(t, "feature/degraded", true)
		if token := issue(t, degraded, "src/app.go"); token != "" {
			t.Fatal("granted patch token after degraded provider result")
		}
	})
	t.Run("wrong patch size", func(t *testing.T) {
		token := issue(t, g, "src/app.go")
		g.policy = gitpatch.New(root, config.AgentPolicy{Enabled: true, MaxPatchBytes: 1})
		if _, err := apply(t, g, token, patch); err == nil {
			t.Fatal("accepted oversized patch")
		}
	})
	t.Run("no mode change", func(t *testing.T) {
		token := issue(t, g, "src/app.go")
		modePatch := strings.Replace(patch, "\n--- a/", "\nold mode 100644\nnew mode 100755\n--- a/", 1)
		if _, err := apply(t, g, token, modePatch); err == nil {
			t.Fatal("accepted mode change")
		}
	})
}

func TestPolicyDisabledDoesNotExposePatch(t *testing.T) {
	reg := registry.New()
	reg.ProvideAgentTools("context.mock", contextProvider{})
	g := NewGateway(reg, agentsdk.Session{ProjectRoot: t.TempDir()})
	tools, err := g.listTools(context.Background())
	if err != nil || len(tools) != 1 {
		t.Fatalf("opt-in policy changed default tools: %v %s", err, tools)
	}
}
