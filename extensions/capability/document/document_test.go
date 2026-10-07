package document

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
	contract "github.com/thinkerqaq/devtool/sdk/contract"
	documentcontract "github.com/thinkerqaq/devtool/sdk/document"
	service "github.com/thinkerqaq/devtool/sdk/service"
)

type fakeRegistrar struct {
	services map[string]service.Invoker
	tools    agentsdk.ToolProvider
}

func (r *fakeRegistrar) RegisterCommand(contract.CommandDescriptor) error     { return nil }
func (r *fakeRegistrar) RegisterResource(contract.ResourceDescriptor) error   { return nil }
func (r *fakeRegistrar) RegisterView(contract.ViewDescriptor) error           { return nil }
func (r *fakeRegistrar) RegisterFeature(contract.FeatureBinding) error        { return nil }
func (r *fakeRegistrar) RegisterNavigation(contract.NavigationItem) error     { return nil }
func (r *fakeRegistrar) ProvideService(string, string, service.Invoker) error { return nil }
func (r *fakeRegistrar) Service(name string) (service.Invoker, bool) {
	value, ok := r.services[name]
	return value, ok
}
func (r *fakeRegistrar) ProvideAgentTools(_ string, provider agentsdk.ToolProvider) error {
	r.tools = provider
	return nil
}

func TestAgentSurfaceExposesOneIntentLevelDocumentCapability(t *testing.T) {
	e := New()
	tools, err := e.ListTools(context.Background(), agentsdk.Session{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 || tools[0].Name != "document_context" {
		t.Fatalf("tools = %+v; want only document_context", tools)
	}
}

func TestDocumentContextInvokesStableDocumentService(t *testing.T) {
	var method string
	var request documentcontract.InspectRequest
	reg := &fakeRegistrar{services: map[string]service.Invoker{
		documentcontract.ServiceName: service.Func(func(_ context.Context, gotMethod string, payload json.RawMessage) (json.RawMessage, error) {
			method = gotMethod
			if err := json.Unmarshal(payload, &request); err != nil {
				return nil, err
			}
			return json.RawMessage("{\"path\":\"src/content/articles/example.md\",\"format\":\"markdown\",\"line_count\":120,\"outline\":[]}"), nil
		}),
	}}
	e := New()
	if err := e.Register(reg); err != nil {
		t.Fatal(err)
	}

	raw, err := e.CallTool(
		context.Background(),
		agentsdk.Session{ProjectRoot: "/workspace"},
		"document_context",
		json.RawMessage("{\"objective\":\"review the lifecycle section\",\"path\":\"src/content/articles/example.md\",\"section\":\"1.3\",\"include_content\":true}"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if method != documentcontract.MethodInspect {
		t.Fatalf("method = %q, want %q", method, documentcontract.MethodInspect)
	}
	if request.Root != "/workspace" {
		t.Fatalf("root = %q", request.Root)
	}
	if request.Path != "src/content/articles/example.md" || request.Section != "1.3" || !request.IncludeContent {
		t.Fatalf("request = %#v", request)
	}
	if !json.Valid(raw) {
		t.Fatalf("tool result is not JSON: %s", raw)
	}
}

func TestDocumentReviewBuildsPlanAndTraversesSections(t *testing.T) {
	var requests []documentcontract.InspectRequest
	reg := &fakeRegistrar{services: map[string]service.Invoker{
		documentcontract.ServiceName: service.Func(func(_ context.Context, _ string, payload json.RawMessage) (json.RawMessage, error) {
			var request documentcontract.InspectRequest
			if err := json.Unmarshal(payload, &request); err != nil {
				return nil, err
			}
			requests = append(requests, request)
			if request.SectionStartLine > 0 {
				return json.Marshal(documentcontract.InspectResponse{
					Path:      request.Path,
					Format:    "markdown",
					LineCount: 40,
					SelectedSection: &documentcontract.SelectedSection{
						Title:     "Repeated",
						Level:     2,
						StartLine: request.SectionStartLine,
						EndLine:   request.SectionStartLine + 9,
						Content:   fmt.Sprintf("section at %d", request.SectionStartLine),
					},
				})
			}
			return json.Marshal(documentcontract.InspectResponse{
				Path:      request.Path,
				Format:    "markdown",
				LineCount: 40,
				Outline: []documentcontract.Section{
					{Title: "Repeated", Level: 2, StartLine: 10, EndLine: 19},
					{Title: "Repeated", Level: 2, StartLine: 20, EndLine: 29},
					{Key: "3", Title: "3. Closing", Level: 2, StartLine: 30, EndLine: 40},
				},
			})
		}),
	}}
	e := New()
	if err := e.Register(reg); err != nil {
		t.Fatal(err)
	}
	session := agentsdk.Session{ProjectRoot: "/workspace"}
	path := "src/content/articles/example.md"

	initialRaw, err := e.CallTool(
		context.Background(),
		session,
		"document_context",
		json.RawMessage("{\"objective\":\"review the whole article\",\"path\":\""+path+"\",\"review\":true}"),
	)
	if err != nil {
		t.Fatal(err)
	}
	initial := decodeToolPayload(t, initialRaw)
	review := mustMap(t, initial["review"])
	if got := int(review["total_sections"].(float64)); got != 3 {
		t.Fatalf("total sections = %d, want 3", got)
	}
	if got := len(mustSlice(t, review["covered_sections"])); got != 0 {
		t.Fatalf("covered sections = %d, want 0", got)
	}
	if got := len(mustSlice(t, review["remaining_sections"])); got != 3 {
		t.Fatalf("remaining sections = %d, want 3", got)
	}
	cursor, ok := review["next_cursor"].(string)
	if !ok || cursor == "" {
		t.Fatalf("next cursor = %#v", review["next_cursor"])
	}
	if len(requests) != 1 || requests[0].IncludeContent || requests[0].SectionStartLine != 0 {
		t.Fatalf("initial requests = %#v", requests)
	}

	continueArgs, _ := json.Marshal(map[string]any{
		"objective": "review the whole article",
		"path":      path,
		"review":    true,
		"cursor":    cursor,
	})
	nextRaw, err := e.CallTool(context.Background(), session, "document_context", continueArgs)
	if err != nil {
		t.Fatal(err)
	}
	next := decodeToolPayload(t, nextRaw)
	review = mustMap(t, next["review"])
	if got := len(mustSlice(t, review["covered_sections"])); got != 1 {
		t.Fatalf("covered sections = %d, want 1", got)
	}
	if got := len(mustSlice(t, review["remaining_sections"])); got != 2 {
		t.Fatalf("remaining sections = %d, want 2", got)
	}
	current := mustMap(t, review["current_section"])
	if got := int(current["start_line"].(float64)); got != 10 {
		t.Fatalf("current start line = %d, want 10", got)
	}
	section := mustMap(t, next["section"])
	if section["content"] != "section at 10" {
		t.Fatalf("section content = %#v", section["content"])
	}
	if len(requests) != 3 {
		t.Fatalf("requests = %d, want 3", len(requests))
	}
	if got := requests[2].SectionStartLine; got != 10 {
		t.Fatalf("review selected start line = %d, want 10", got)
	}
	if !requests[2].IncludeContent {
		t.Fatal("review section request must include content")
	}
}

func TestDocumentReviewRejectsStaleCursor(t *testing.T) {
	version := 1
	reg := &fakeRegistrar{services: map[string]service.Invoker{
		documentcontract.ServiceName: service.Func(func(_ context.Context, _ string, payload json.RawMessage) (json.RawMessage, error) {
			var request documentcontract.InspectRequest
			if err := json.Unmarshal(payload, &request); err != nil {
				return nil, err
			}
			title := "1. First"
			if version == 2 {
				title = "1. Changed"
			}
			return json.Marshal(documentcontract.InspectResponse{
				Path:      request.Path,
				Format:    "markdown",
				LineCount: 10,
				Outline: []documentcontract.Section{
					{Key: "1", Title: title, Level: 2, StartLine: 1, EndLine: 10},
				},
			})
		}),
	}}
	e := New()
	if err := e.Register(reg); err != nil {
		t.Fatal(err)
	}
	session := agentsdk.Session{ProjectRoot: "/workspace"}
	initialRaw, err := e.CallTool(
		context.Background(),
		session,
		"document_context",
		json.RawMessage("{\"objective\":\"review\",\"path\":\"article.md\",\"review\":true}"),
	)
	if err != nil {
		t.Fatal(err)
	}
	initial := decodeToolPayload(t, initialRaw)
	cursor := mustMap(t, initial["review"])["next_cursor"].(string)

	version = 2
	args, _ := json.Marshal(map[string]any{
		"objective": "review",
		"path":      "article.md",
		"review":    true,
		"cursor":    cursor,
	})
	if _, err := e.CallTool(context.Background(), session, "document_context", args); err == nil {
		t.Fatal("expected stale review cursor to fail")
	}
}

func TestDocumentReviewRejectsCrossDocumentCursor(t *testing.T) {
	reg := &fakeRegistrar{services: map[string]service.Invoker{
		documentcontract.ServiceName: service.Func(func(_ context.Context, _ string, payload json.RawMessage) (json.RawMessage, error) {
			var request documentcontract.InspectRequest
			if err := json.Unmarshal(payload, &request); err != nil {
				return nil, err
			}
			return json.Marshal(documentcontract.InspectResponse{
				Path:      request.Path,
				Format:    "markdown",
				LineCount: 10,
				Outline: []documentcontract.Section{
					{Key: "1", Title: "1. First", Level: 2, StartLine: 1, EndLine: 10},
				},
			})
		}),
	}}
	e := New()
	if err := e.Register(reg); err != nil {
		t.Fatal(err)
	}
	session := agentsdk.Session{ProjectRoot: "/workspace"}
	initialRaw, err := e.CallTool(
		context.Background(),
		session,
		"document_context",
		json.RawMessage("{\"objective\":\"review\",\"path\":\"first.md\",\"review\":true}"),
	)
	if err != nil {
		t.Fatal(err)
	}
	cursor := mustMap(t, decodeToolPayload(t, initialRaw)["review"])["next_cursor"].(string)
	args, _ := json.Marshal(map[string]any{
		"objective": "review",
		"path":      "second.md",
		"review":    true,
		"cursor":    cursor,
	})
	if _, err := e.CallTool(context.Background(), session, "document_context", args); err == nil {
		t.Fatal("expected cross-document review cursor to fail")
	}
}

func TestDocumentContextValidatesIntentAndReviewArguments(t *testing.T) {
	e := New()
	reg := &fakeRegistrar{services: map[string]service.Invoker{
		documentcontract.ServiceName: service.Func(func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage("{}"), nil
		}),
	}}
	if err := e.Register(reg); err != nil {
		t.Fatal(err)
	}

	session := agentsdk.Session{ProjectRoot: "/workspace"}
	cases := []string{
		"{\"objective\":\"\",\"path\":\"article.md\"}",
		"{\"objective\":\"review\",\"path\":\"\"}",
		"{\"objective\":\"review\",\"path\":\"article.md\",\"cursor\":\"abc\"}",
		"{\"objective\":\"review\",\"path\":\"article.md\",\"review\":true,\"section\":\"1\"}",
		"{\"objective\":\"review\",\"path\":\"article.md\",\"review\":true,\"include_content\":true}",
	}
	for _, raw := range cases {
		if _, err := e.CallTool(context.Background(), session, "document_context", json.RawMessage(raw)); err == nil {
			t.Fatalf("expected arguments to fail: %s", raw)
		}
	}
}

func decodeToolPayload(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var envelope struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Content) != 1 {
		t.Fatalf("content blocks = %d, want 1", len(envelope.Content))
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(envelope.Content[0].Text), &payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

func mustMap(t *testing.T, value any) map[string]any {
	t.Helper()
	result, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("value = %#v, want map", value)
	}
	return result
}

func mustSlice(t *testing.T, value any) []any {
	t.Helper()
	result, ok := value.([]any)
	if !ok {
		t.Fatalf("value = %#v, want slice", value)
	}
	return result
}
