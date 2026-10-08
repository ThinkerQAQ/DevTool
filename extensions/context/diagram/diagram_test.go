package diagram

import (
	"context"
	"encoding/json"
	agent "github.com/thinkerqaq/devtool/sdk/agent"
	contract "github.com/thinkerqaq/devtool/sdk/contract"
	diagram "github.com/thinkerqaq/devtool/sdk/diagram"
	document "github.com/thinkerqaq/devtool/sdk/document"
	service "github.com/thinkerqaq/devtool/sdk/service"
	"testing"
)

type registrar struct{ values map[string]service.Invoker }

func (r *registrar) RegisterCommand(contract.CommandDescriptor) error     { return nil }
func (r *registrar) RegisterResource(contract.ResourceDescriptor) error   { return nil }
func (r *registrar) RegisterView(contract.ViewDescriptor) error           { return nil }
func (r *registrar) RegisterFeature(contract.FeatureBinding) error        { return nil }
func (r *registrar) RegisterNavigation(contract.NavigationItem) error     { return nil }
func (r *registrar) ProvideService(string, string, service.Invoker) error { return nil }
func (r *registrar) Service(name string) (service.Invoker, bool) {
	v, ok := r.values[name]
	return v, ok
}
func (r *registrar) ProvideAgentTools(string, agent.ToolProvider) error { return nil }

func TestCombinedMarkdownOutlineAndMermaidEvidence(t *testing.T) {
	invocations := 0
	e := New()
	reg := &registrar{values: map[string]service.Invoker{
		document.ServiceName: service.Func(func(_ context.Context, method string, raw json.RawMessage) (json.RawMessage, error) {
			if method != document.MethodInspect {
				t.Fatalf("unexpected method %s", method)
			}
			return json.Marshal(document.InspectResponse{Path: "article.md",
				Outline: []document.Section{{Key: "2", Title: "Go", StartLine: 10, EndLine: 100, Children: []document.Section{
					{Key: "2.3", Title: "Go blocking", StartLine: 30, EndLine: 60},
				}}},
				Diagrams: []document.Diagram{{Index: 1, Language: "mermaid", StartLine: 36, EndLine: 42, Source: "flowchart LR\nA-->B"}},
			})
		}),
		diagram.IntelligenceServiceName: service.Func(func(_ context.Context, method string, raw json.RawMessage) (json.RawMessage, error) {
			if method != diagram.IntelligenceMethod {
				t.Fatalf("unexpected method %s", method)
			}
			var req diagram.IntelligenceRequest
			if err := json.Unmarshal(raw, &req); err != nil {
				t.Fatal(err)
			}
			if req.Path != "article.md" || len(req.Diagrams) != 1 || req.Diagrams[0].StartLine != 36 {
				t.Fatalf("request=%+v", req)
			}
			invocations++
			return json.Marshal(diagram.IntelligenceResponse{Provider: "test", DiagnosticsStatus: "reported",
				Results: []diagram.IntelligenceResult{{Index: 1, Status: "ok", Complete: true,
					Graph: diagram.Graph{Kind: "flowchart-v2", NodeCount: 2, EdgeCount: 1, Nodes: []diagram.GraphNode{{ID: "A", Line: 37}, {ID: "B", Line: 38}}}}}})
		}),
	}}
	if err := e.Register(reg); err != nil {
		t.Fatal(err)
	}
	input, _ := json.Marshal(diagram.ContextRequest{Root: "/workspace", Path: "article.md", Objective: "review Go diagram", Analyze: true})
	raw, err := e.Invoke(t.Context(), diagram.ContextMethod, input)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Results           []entry `json:"results"`
		DiagnosticsStatus string  `json:"diagnostics_status"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if invocations != 1 || result.DiagnosticsStatus != "reported" || len(result.Results) != 1 {
		t.Fatalf("result %s", raw)
	}
	item := result.Results[0]
	if item.Section == nil || item.Section.Key != "2.3" || item.Intelligence == nil || !item.Intelligence.Complete || item.Intelligence.Graph.Nodes[0].Line != 37 {
		t.Fatalf("joined result %+v", item)
	}
}
func TestAbsentIntelligenceProviderIsNotReportedAsClean(t *testing.T) {
	e := New()
	_ = e.Register(&registrar{values: map[string]service.Invoker{
		document.ServiceName: service.Func(func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
			return json.Marshal(document.InspectResponse{Path: "a.md", Diagrams: []document.Diagram{{Index: 1, Language: "mermaid", StartLine: 1, EndLine: 4, Source: "flowchart LR\nA-->B"}}})
		}),
	}})
	input, _ := json.Marshal(diagram.ContextRequest{Root: "/workspace", Path: "a.md", Objective: "review", Analyze: true})
	raw, err := e.Invoke(t.Context(), diagram.ContextMethod, input)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Results           []entry `json:"results"`
		DiagnosticsStatus string  `json:"diagnostics_status"`
	}
	_ = json.Unmarshal(raw, &result)
	if result.DiagnosticsStatus != "unconfigured" || result.Results[0].Intelligence.Complete || result.Results[0].Intelligence.Status != "unconfigured" {
		t.Fatalf("got %s", raw)
	}
}
