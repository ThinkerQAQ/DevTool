package document

import (
	"context"
	"encoding/json"
	"testing"

	document "github.com/thinkerqaq/devtool/sdk/document"
	realtime "github.com/thinkerqaq/devtool/sdk/documentrealtime"
	"github.com/thinkerqaq/devtool/sdk/service"
)

func TestSectionReferencesComposesProviderResults(t *testing.T) {
	calls := 0
	reg := &fakeRegistrar{services: map[string]service.Invoker{
		document.ServiceName: service.Func(func(_ context.Context, method string, raw json.RawMessage) (json.RawMessage, error) {
			var in document.InspectRequest
			if err := json.Unmarshal(raw, &in); err != nil {
				return nil, err
			}
			if in.Section != "1.3" {
				t.Fatalf("incorrect selection %+v", in)
			}
			return json.Marshal(document.InspectResponse{Path: "doc.md", Format: "markdown",
				SelectedSection: &document.SelectedSection{Key: "1.3", Title: "1.3 Scheduling", StartLine: 15, EndLine: 29}})
		}),
		realtime.ServiceName: service.Func(func(_ context.Context, method string, raw json.RawMessage) (json.RawMessage, error) {
			var req realtime.AnalyzeRequest
			if err := json.Unmarshal(raw, &req); err != nil {
				return nil, err
			}
			if method != realtime.MethodAnalyze || req.Root != "/workspace" || req.HeadingLine != 15 || req.Path != "doc.md" {
				t.Fatalf("bad request %+v", req)
			}
			calls++
			return json.Marshal(realtime.AnalyzeResponse{Status: "ok", Provider: "test", WorkspaceScope: "docs", References: []realtime.Location{
				{Path: "doc.md", Range: realtime.Range{Start: realtime.Position{Line: 15, Column: 1}}},
				{Path: "doc.md", Range: realtime.Range{Start: realtime.Position{Line: 4, Column: 8}}},
				{Path: "another.md", Range: realtime.Range{Start: realtime.Position{Line: 7, Column: 2}}},
			}})
		}),
	}}
	e := New()
	if err := e.Register(reg); err != nil {
		t.Fatal(err)
	}
	raw, err := callContext(t, e, t.Context(), "/workspace", json.RawMessage(`{"objective":"check section links","path":"doc.md","section":"1.3","references":true}`))
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		References struct {
			Status       string              `json:"status"`
			Complete     bool                `json:"complete"`
			ReferencedBy []realtime.Location `json:"referenced_by"`
			Scope        string              `json:"scope"`
		} `json:"references"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || !result.References.Complete || result.References.Scope != "docs" || len(result.References.ReferencedBy) != 2 {
		t.Fatalf("unexpected result %s", raw)
	}
}
func TestSectionReferencesMissingProviderIsExplicit(t *testing.T) {
	e := New()
	if err := e.Register(&fakeRegistrar{services: map[string]service.Invoker{
		document.ServiceName: service.Func(func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(`{"path":"doc.md","selected_section":{"title":"Section","start_line":3,"end_line":8}}`), nil
		}),
	}}); err != nil {
		t.Fatal(err)
	}
	raw, err := callContext(t, e, t.Context(), "/workspace", json.RawMessage(`{"objective":"check references","path":"doc.md","section":"Section","references":true}`))
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		References struct {
			Status   string `json:"status"`
			Complete bool   `json:"complete"`
		} `json:"references"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if result.References.Status != "unconfigured" || result.References.Complete {
		t.Fatalf("unexpected result %s", raw)
	}
}
func TestReferencesRequireSelectedSection(t *testing.T) {
	e := New()
	e.services = &fakeRegistrar{services: map[string]service.Invoker{}}
	_, err := callContext(t, e, t.Context(), "/workspace", json.RawMessage(`{"objective":"check","path":"doc.md","references":true}`))
	if err == nil {
		t.Fatal("references without section should be rejected")
	}
}
func TestTruncatedReferenceSetIsIncomplete(t *testing.T) {
	e := New()
	_ = e.Register(&fakeRegistrar{services: map[string]service.Invoker{
		document.ServiceName: service.Func(func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(`{"path":"doc.md","selected_section":{"title":"Section","start_line":3,"end_line":8}}`), nil
		}),
		realtime.ServiceName: service.Func(func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
			return json.Marshal(realtime.AnalyzeResponse{Status: "ok", Truncated: true, References: []realtime.Location{}})
		}),
	}})
	raw, err := callContext(t, e, t.Context(), "/workspace", json.RawMessage(`{"objective":"check","path":"doc.md","section":"Section","references":true}`))
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		References struct {
			Status   string `json:"status"`
			Complete bool   `json:"complete"`
		} `json:"references"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if result.References.Status != "partial" || result.References.Complete {
		t.Fatalf("got %s", raw)
	}
}
