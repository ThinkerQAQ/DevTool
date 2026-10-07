package document

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
	contract "github.com/thinkerqaq/devtool/sdk/contract"
	documentcontract "github.com/thinkerqaq/devtool/sdk/document"
	documentcontext "github.com/thinkerqaq/devtool/sdk/documentcontext"
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

func callContext(t *testing.T, e *Extension, ctx context.Context, root string, args json.RawMessage) (json.RawMessage, error) {
	t.Helper()
	var request map[string]any
	if len(args) != 0 {
		if err := json.Unmarshal(args, &request); err != nil {
			return nil, err
		}
	} else {
		request = map[string]any{}
	}
	request["root"] = root
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	return e.Invoke(ctx, documentcontext.MethodBuild, payload)
}

func TestDocumentCapabilityConfiguresReviewBudget(t *testing.T) {
	e := New()
	if e.defaultReviewMaxLines != defaultReviewMaxLines {
		t.Fatalf("default review max lines = %d, want %d", e.defaultReviewMaxLines, defaultReviewMaxLines)
	}
	if err := e.Configure(map[string]any{"review_max_lines": int64(180)}); err != nil {
		t.Fatal(err)
	}
	if e.defaultReviewMaxLines != 180 {
		t.Fatalf("review max lines = %d, want 180", e.defaultReviewMaxLines)
	}
	if err := e.Configure(map[string]any{"review_max_lines": 0}); err == nil {
		t.Fatal("expected zero review budget to fail")
	}
	if err := e.Configure(map[string]any{"review_max_lines": 1.5}); err == nil {
		t.Fatal("expected fractional review budget to fail")
	}
}

func TestPlanReviewSectionsRecursivelyBoundsStructuredContent(t *testing.T) {
	outline := []documentcontract.Section{
		{
			Key: "1", Title: "1. Java", Level: 2, StartLine: 1, EndLine: 700,
			Children: []documentcontract.Section{
				{Key: "1.1", Title: "1.1 Model", Level: 3, StartLine: 21, EndLine: 180},
				{
					Key: "1.2", Title: "1.2 Lifecycle", Level: 3, StartLine: 181, EndLine: 620,
					Children: []documentcontract.Section{
						{Key: "1.2.1", Title: "1.2.1 Create", Level: 4, StartLine: 201, EndLine: 320},
						{Key: "1.2.2", Title: "1.2.2 Wait", Level: 4, StartLine: 321, EndLine: 500},
						{Key: "1.2.3", Title: "1.2.3 Finish", Level: 4, StartLine: 501, EndLine: 620},
					},
				},
				{Key: "1.3", Title: "1.3 Huge Leaf", Level: 3, StartLine: 621, EndLine: 700},
			},
		},
	}
	units := planReviewSections(outline, 150)
	want := []struct {
		start, end int
		kind       string
		oversized  bool
	}{
		{1, 20, "preamble", false},
		{21, 180, "section", true},
		{181, 200, "preamble", false},
		{201, 320, "section", false},
		{321, 500, "section", true},
		{501, 620, "section", false},
		{621, 700, "section", false},
	}
	if len(units) != len(want) {
		t.Fatalf("units = %d, want %d: %#v", len(units), len(want), units)
	}
	for i, expected := range want {
		unit := units[i]
		if unit.StartLine != expected.start || unit.EndLine != expected.end || unit.RangeKind != expected.kind || unit.Oversized != expected.oversized {
			t.Fatalf("unit %d = %#v, want %d..%d %s oversized=%v", i, unit, expected.start, expected.end, expected.kind, expected.oversized)
		}
		if i > 0 && units[i-1].EndLine+1 != unit.StartLine {
			t.Fatalf("gap/overlap between units %d and %d: %#v %#v", i-1, i, units[i-1], unit)
		}
	}
	if units[0].StartLine != 1 || units[len(units)-1].EndLine != 700 {
		t.Fatalf("planned coverage = %d..%d, want 1..700", units[0].StartLine, units[len(units)-1].EndLine)
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

	raw, err := callContext(t, e, context.Background(), "/workspace",
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
			if request.RangeStartLine > 0 {
				return json.Marshal(documentcontract.InspectResponse{
					Path:      request.Path,
					Format:    "markdown",
					LineCount: 40,
					SelectedRange: &documentcontract.SelectedRange{
						StartLine: request.RangeStartLine,
						EndLine:   request.RangeEndLine,
						Content:   fmt.Sprintf("section at %d", request.RangeStartLine),
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

	initialRaw, err := callContext(t, e, context.Background(), session.ProjectRoot,
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
	nextRaw, err := callContext(t, e, context.Background(), session.ProjectRoot, continueArgs)
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
	if got := requests[2].RangeStartLine; got != 10 {
		t.Fatalf("review selected range start line = %d, want 10", got)
	}
	if got := requests[2].RangeEndLine; got != 19 {
		t.Fatalf("review selected range end line = %d, want 19", got)
	}
	if !requests[2].IncludeContent {
		t.Fatal("review range request must include content")
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
	initialRaw, err := callContext(t, e, context.Background(), session.ProjectRoot,
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
	if _, err := callContext(t, e, context.Background(), session.ProjectRoot, args); err == nil {
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
	initialRaw, err := callContext(t, e, context.Background(), session.ProjectRoot,
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
	if _, err := callContext(t, e, context.Background(), session.ProjectRoot, args); err == nil {
		t.Fatal("expected cross-document review cursor to fail")
	}
}

func TestDocumentContextIncludesRelatedContext(t *testing.T) {
	var relationsRequest documentcontract.RelationsRequest
	reg := &fakeRegistrar{services: map[string]service.Invoker{
		documentcontract.ServiceName: service.Func(func(_ context.Context, _ string, payload json.RawMessage) (json.RawMessage, error) {
			var request documentcontract.InspectRequest
			if err := json.Unmarshal(payload, &request); err != nil {
				return nil, err
			}
			return json.Marshal(documentcontract.InspectResponse{
				Path:      request.Path,
				Format:    "markdown",
				LineCount: 20,
				Outline:   []documentcontract.Section{{Key: "1", Title: "1. Intro", Level: 2, StartLine: 1, EndLine: 20}},
			})
		}),
		documentcontract.RelationsServiceName: service.Func(func(_ context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
			if method != documentcontract.MethodResolveRelations {
				return nil, fmt.Errorf("unexpected method %q", method)
			}
			if err := json.Unmarshal(payload, &relationsRequest); err != nil {
				return nil, err
			}
			return json.Marshal(documentcontract.RelationsResponse{
				RootNode: documentcontract.RelationNode{
					Key:   "article:src/content/articles/a.md",
					Kind:  "article",
					ID:    "a",
					Path:  "src/content/articles/a.md",
					Title: "A",
				},
				Nodes: []documentcontract.RelationNode{
					{Key: "article:src/content/articles/a.md", Kind: "article", ID: "a", Path: "src/content/articles/a.md", Title: "A"},
					{Key: "series:src/content/series/s.md", Kind: "series", ID: "s", Path: "src/content/series/s.md", Title: "S"},
				},
				Edges: []documentcontract.RelationEdge{
					{Type: "member_of_series", From: "article:src/content/articles/a.md", To: "series:src/content/series/s.md", Source: "series"},
				},
			})
		}),
	}}
	e := New()
	if err := e.Register(reg); err != nil {
		t.Fatal(err)
	}

	raw, err := callContext(t, e, context.Background(), "/workspace",
		json.RawMessage("{\"objective\":\"review article and related context\",\"path\":\"src/content/articles/a.md\",\"related\":true,\"relation_depth\":3,\"relation_limit\":50}"),
	)
	if err != nil {
		t.Fatal(err)
	}
	payload := decodeToolPayload(t, raw)
	relations := mustMap(t, payload["relations"])
	if root := mustMap(t, relations["root_node"]); root["id"] != "a" {
		t.Fatalf("relation root = %#v", root)
	}
	if relationsRequest.Root != "/workspace" || relationsRequest.Path != "src/content/articles/a.md" {
		t.Fatalf("relations request = %#v", relationsRequest)
	}
	if relationsRequest.Depth != 3 || relationsRequest.MaxNodes != 50 {
		t.Fatalf("relations limits = %#v", relationsRequest)
	}
}

func TestDocumentReviewInitialCallCanIncludeRelatedContext(t *testing.T) {
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
					{Key: "1", Title: "1. Intro", Level: 2, StartLine: 1, EndLine: 10},
				},
			})
		}),
		documentcontract.RelationsServiceName: service.Func(func(_ context.Context, _ string, payload json.RawMessage) (json.RawMessage, error) {
			return json.Marshal(documentcontract.RelationsResponse{
				RootNode: documentcontract.RelationNode{Key: "article:a", Kind: "article", ID: "a", Path: "a.md"},
			})
		}),
	}}
	e := New()
	if err := e.Register(reg); err != nil {
		t.Fatal(err)
	}

	raw, err := callContext(t, e, context.Background(), "/workspace",
		json.RawMessage("{\"objective\":\"review whole article and graph\",\"path\":\"a.md\",\"review\":true,\"related\":true}"),
	)
	if err != nil {
		t.Fatal(err)
	}
	payload := decodeToolPayload(t, raw)
	if _, ok := payload["review"]; !ok {
		t.Fatal("review context missing")
	}
	if _, ok := payload["relations"]; !ok {
		t.Fatal("related context missing")
	}
}

func TestDocumentContextRelatedRequiresConfiguredProvider(t *testing.T) {
	reg := &fakeRegistrar{services: map[string]service.Invoker{
		documentcontract.ServiceName: service.Func(func(_ context.Context, _ string, payload json.RawMessage) (json.RawMessage, error) {
			return json.Marshal(documentcontract.InspectResponse{Path: "a.md", Format: "markdown"})
		}),
	}}
	e := New()
	if err := e.Register(reg); err != nil {
		t.Fatal(err)
	}
	if _, err := callContext(t, e, context.Background(), "/workspace",
		json.RawMessage("{\"objective\":\"review relations\",\"path\":\"a.md\",\"related\":true}"),
	); err == nil {
		t.Fatal("expected missing relation provider to fail")
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
		"{\"objective\":\"review\",\"path\":\"article.md\",\"review\":true,\"cursor\":\"abc\",\"related\":true}",
		"{\"objective\":\"review\",\"path\":\"article.md\",\"relation_depth\":2}",
		"{\"objective\":\"review\",\"path\":\"article.md\",\"relation_limit\":10}",
		"{\"objective\":\"review\",\"path\":\"article.md\",\"related\":true,\"relation_depth\":-1}",
		"{\"objective\":\"review\",\"path\":\"article.md\",\"related\":true,\"relation_limit\":-1}",
	}
	for _, raw := range cases {
		if _, err := callContext(t, e, context.Background(), session.ProjectRoot, json.RawMessage(raw)); err == nil {
			t.Fatalf("expected arguments to fail: %s", raw)
		}
	}
}

func decodeToolPayload(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
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
