package agent

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thinkerqaq/devtool/adapters/agentpolicy/gitpatch"
	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
)

type routingContextProvider struct {
	name     string
	response string
}

func (p routingContextProvider) ListTools(context.Context, agentsdk.Session) ([]agentsdk.Tool, error) {
	raw, _ := json.Marshal(map[string]any{"name": p.name, "inputSchema": map[string]any{"type": "object"}})
	return []agentsdk.Tool{{Name: p.name, Definition: raw}}, nil
}
func (p routingContextProvider) CallTool(_ context.Context, _ agentsdk.Session, _ string, args json.RawMessage) (json.RawMessage, error) {
	var request struct {
		Path string `json:"path"`
	}
	_ = json.Unmarshal(args, &request)
	response := p.response
	switch p.name {
	case "document_context":
		data, _ := json.Marshal(map[string]any{"objective": "inspect", "document": map[string]any{"path": request.Path, "format": "markdown", "line_count": 6}})
		response = string(data)
	case "diagram_context":
		results := []map[string]any{}
		if request.Path == "docs/graph.md" {
			results = append(results, map[string]any{"index": 1, "language": "mermaid", "status": "found"})
		}
		data, _ := json.Marshal(map[string]any{"objective": "inspect", "path": request.Path, "total_diagrams": len(results), "results": results})
		response = string(data)
	}
	return json.Marshal(map[string]any{"content": []map[string]string{{"type": "text", "text": response}}})
}

func setupContextFixtures(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "logs"), 0755); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		"docs/guide.md": "# Guide\n\nBefore change.\n",
		"docs/graph.md": "# Graph\n\n```mermaid\ngraph TD\n    A-->B\n```\n",
		"logs/app.log":  "INFO started\nERROR something failed\n",
	} {
		if err := os.WriteFile(filepath.Join(root, path), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	repoCommand(t, root, "add", ".")
	repoCommand(t, root, "commit", "-q", "-m", "context fixture")
}
func setupContextGateway(t *testing.T) (*Gateway, string) {
	t.Helper()
	g, root := testGateway(t, "feature/context-routing", false)
	setupContextFixtures(t, root)
	g.policy.(*gitpatch.Gate).WithDiagramValidator(func(context.Context, string, string) error { return nil })
	for _, x := range []routingContextProvider{
		{"document_context", `{"objective":"inspect","document":{"path":"docs/guide.md","format":"markdown","line_count":3}}`},
		{"diagram_context", `{"objective":"inspect","path":"docs/graph.md","total_diagrams":1,"results":[{"index":1,"language":"mermaid","status":"found"}]}`},
		{"log_context", `{"objective":"investigate","log":{"source":"logs/app.log","summary":{"lines":2,"errors":1}}}`},
	} {
		if err := g.registry.ProvideAgentTools("mock."+x.name, x); err != nil {
			t.Fatal(err)
		}
	}
	return g, root
}
func contextData(t *testing.T, g *Gateway, name, path string) map[string]any {
	t.Helper()
	args, _ := json.Marshal(map[string]string{"objective": "Review before changing or investigating", "path": path})
	res, err := g.callTool(context.Background(), name, args)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(res, &out); err != nil || len(out.Content) == 0 {
		t.Fatalf("bad MCP response %v: %s", err, res)
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(out.Content[0].Text), &data); err != nil {
		t.Fatal(err)
	}
	return data
}
func tokenFrom(data map[string]any) string { v, _ := data["context_token"].(string); return v }

func TestPolicyRoutesMarkdownToDocumentContext(t *testing.T) {
	g, root := setupContextGateway(t)
	if token := issue(t, g, "docs/guide.md"); token != "" {
		t.Fatal("code_context issued a Markdown write token")
	}
	ticket := contextData(t, g, "document_context", "docs/guide.md")
	if tokenFrom(ticket) == "" {
		t.Fatalf("document_context did not authorize Markdown: %+v", ticket)
	}
	patch := makePatch(t, root, "docs/guide.md", "# Guide\n\nAfter change.\n")
	if result, err := apply(t, g, tokenFrom(ticket), patch); err != nil || !strings.Contains(string(result), "applied") {
		t.Fatalf("document patch failed: %v %s", err, result)
	}
}

func TestPolicyRequiresDocumentAndDiagramContexts(t *testing.T) {
	g, root := setupContextGateway(t)
	if token := issue(t, g, "docs/graph.md"); token != "" {
		t.Fatal("code_context issued a diagram edit token")
	}
	doc := contextData(t, g, "document_context", "docs/graph.md")
	if tokenFrom(doc) != "" {
		t.Fatal("document_context alone allowed modifying a Markdown diagram")
	}
	required, ok := doc["required_context_tools"].([]any)
	if !ok || len(required) != 1 || required[0] != "diagram_context" {
		t.Fatalf("missing diagram requirement: %+v", doc)
	}
	dgm := contextData(t, g, "diagram_context", "docs/graph.md")
	if tokenFrom(dgm) == "" {
		t.Fatalf("diagram_context failed to complete authorization: %+v", dgm)
	}
	patch := makePatch(t, root, "docs/graph.md", "# Graph\n\n```mermaid\ngraph TD\n    B-->C\n```\n")
	result, err := apply(t, g, tokenFrom(dgm), patch)
	if err != nil || !strings.Contains(string(result), "applied") {
		t.Fatalf("diagram patch failed: %v %s", err, result)
	}
}

func TestPolicyRequiresDiagramForNewFence(t *testing.T) {
	g, root := setupContextGateway(t)
	doc := contextData(t, g, "document_context", "docs/guide.md")
	token := tokenFrom(doc)
	if token == "" {
		t.Fatal("missing document-only ticket")
	}
	patch := makePatch(t, root, "docs/guide.md", "# Guide\n\nBefore change.\n\n```mermaid\ngraph TD\n    A-->B\n```\n")
	if _, err := apply(t, g, token, patch); err == nil || !strings.Contains(err.Error(), "diagram_context") {
		t.Fatalf("new diagram fence bypassed diagram_context: %v", err)
	}
	// The diagram provider can inspect the source with zero diagrams before adding one.
	diagram := contextData(t, g, "diagram_context", "docs/guide.md")
	if tokenFrom(diagram) == "" {
		t.Fatalf("new diagram is still blocked: %+v", diagram)
	}
	if result, err := apply(t, g, tokenFrom(diagram), patch); err != nil || !strings.Contains(string(result), "applied") {
		t.Fatalf("new diagram edit failed: %v %s", err, result)
	}
}

func TestPolicyLogContextIsReadOnly(t *testing.T) {
	g, _ := setupContextGateway(t)
	if token := issue(t, g, "logs/app.log"); token != "" {
		t.Fatal("code_context allowed log editing")
	}
	logs := contextData(t, g, "log_context", "logs/app.log")
	if _, ok := logs["log"]; !ok {
		t.Fatal("log_context returned no log analysis")
	}
	if tokenFrom(logs) != "" {
		t.Fatal("log_context issued a source edit token")
	}
	doc := contextData(t, g, "document_context", "logs/app.log")
	if tokenFrom(doc) != "" {
		t.Fatal("document_context issued a log edit token")
	}
}

func TestPolicyRejectsSpoofedDocumentAndDiagramContext(t *testing.T) {
	g, _ := setupContextGateway(t)
	malformed := []struct{ tool, path, payload string }{
		{"document_context", "docs/guide.md", `{"document":{"path":"other.md","format":"markdown"}}`},
		{"document_context", "docs/guide.md", `{"document":{"path":"docs/guide.md","format":"plaintext"}}`},
		{"diagram_context", "docs/graph.md", `{"path":"docs/graph.md","total_diagrams":2,"results":[{}]}`},
		{"diagram_context", "docs/graph.md", `{"path":"different.md","total_diagrams":1,"results":[{}]}`},
	}
	for _, tc := range malformed {
		t.Run(tc.tool+" "+tc.payload, func(t *testing.T) {
			args, _ := json.Marshal(map[string]string{"path": tc.path})
			response, _ := json.Marshal(map[string]any{"content": []map[string]string{{"type": "text", "text": tc.payload}}})
			before := g.policy.Attest(context.Background(), tc.tool, args, response)
			if string(before) != string(response) {
				t.Fatalf("malformed context was trusted: %s", before)
			}
		})
	}
}

func TestPolicyMismatchedCapabilityNotAttested(t *testing.T) {
	g, _ := setupContextGateway(t)
	if token := tokenFrom(contextData(t, g, "document_context", "src/app.go")); token != "" {
		t.Fatal("document context allowed code file")
	}
	if token := tokenFrom(contextData(t, g, "diagram_context", "src/app.go")); token != "" {
		t.Fatal("diagram context allowed code file")
	}
}

func TestPolicyRejectsBrokenDiagramBeforeTouchingWorktree(t *testing.T) {
	g, root := setupContextGateway(t)
	g.policy.(*gitpatch.Gate).WithDiagramValidator(func(_ context.Context, language, source string) error {
		if language != "mermaid" || !strings.Contains(source, "invalid") {
			t.Fatalf("wrong diagram candidate: %q %q", language, source)
		}
		return errors.New("invalid diagram syntax")
	})
	contextData(t, g, "document_context", "docs/graph.md")
	token := tokenFrom(contextData(t, g, "diagram_context", "docs/graph.md"))
	before, err := os.ReadFile(filepath.Join(root, "docs/graph.md"))
	if err != nil {
		t.Fatal(err)
	}
	patch := makePatch(t, root, "docs/graph.md", "# Graph\n\n```mermaid\ninvalid syntax\n```\n")
	if _, err := apply(t, g, token, patch); err == nil || !strings.Contains(err.Error(), "invalid diagram syntax") {
		t.Fatalf("bad candidate accepted: %v", err)
	}
	after, err := os.ReadFile(filepath.Join(root, "docs/graph.md"))
	if err != nil || string(after) != string(before) {
		t.Fatalf("rejected candidate changed source: %v", err)
	}
}

func TestPolicyRequiresConfiguredRendererForChangedDiagram(t *testing.T) {
	g, root := setupContextGateway(t)
	g.policy.(*gitpatch.Gate).WithDiagramValidator(nil)
	contextData(t, g, "document_context", "docs/graph.md")
	token := tokenFrom(contextData(t, g, "diagram_context", "docs/graph.md"))
	patch := makePatch(t, root, "docs/graph.md", "# Graph\n\n```mermaid\ngraph TD\n    X-->Y\n```\n")
	if _, err := apply(t, g, token, patch); err == nil || !strings.Contains(err.Error(), "diagram-render provider required") {
		t.Fatalf("missing renderer accepted: %v", err)
	}
}

func TestPolicyUnchangedDiagramDoesNotCallRenderer(t *testing.T) {
	g, root := setupContextGateway(t)
	g.policy.(*gitpatch.Gate).WithDiagramValidator(func(context.Context, string, string) error { t.Fatal("unchanged diagram rendered"); return nil })
	contextData(t, g, "document_context", "docs/graph.md")
	token := tokenFrom(contextData(t, g, "diagram_context", "docs/graph.md"))
	patch := makePatch(t, root, "docs/graph.md", "# Graph updated\n\n```mermaid\ngraph TD\n    A-->B\n```\n")
	if _, err := apply(t, g, token, patch); err != nil {
		t.Fatalf("ordinary Markdown text edit failed: %v", err)
	}
}

// Extended fenced-code info strings are parsed by Goldmark as Mermaid even
// though the patch-line heuristic does not recognize their full label.
func TestPolicyCandidateASTRequiresDiagramContext(t *testing.T) {
	g, root := setupContextGateway(t)
	orig, err := os.ReadFile(filepath.Join(root, "docs/guide.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := contextData(t, g, "document_context", "docs/guide.md")
	token := tokenFrom(doc)
	if token == "" {
		t.Fatal("missing document context token")
	}
	patch := makePatch(t, root, "docs/guide.md", "# Guide\n\nBefore change.\n\n```mermaid {#graph}\ngraph TD\n  A --> B\n```\n")
	if _, err := apply(t, g, token, patch); err == nil || !strings.Contains(err.Error(), "diagram_context") {
		t.Fatalf("candidate bypassed diagram context: %v", err)
	}
	after, err := os.ReadFile(filepath.Join(root, "docs/guide.md"))
	if err != nil || string(after) != string(orig) {
		t.Fatalf("unapproved graph changed source: %v", err)
	}
}
