package code

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"testing"
	"time"

	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
	contract "github.com/thinkerqaq/devtool/sdk/contract"
	"github.com/thinkerqaq/devtool/sdk/service"
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

func TestAgentSurfaceExposesOneIntentLevelCodeCapability(t *testing.T) {
	e := New()
	tools, err := e.ListTools(context.Background(), agentsdk.Session{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 || tools[0].Name != "code_context" {
		t.Fatalf("tools = %+v; want only code_context", tools)
	}
}

func TestCodeContextComposesIndexedAndRealtimeServices(t *testing.T) {
	var calls []string
	var callsMu sync.Mutex
	record := func(call string) {
		callsMu.Lock()
		defer callsMu.Unlock()
		calls = append(calls, call)
	}
	reg := &fakeRegistrar{services: map[string]service.Invoker{
		"code-indexed": service.Func(func(_ context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
			record("indexed." + method)
			var request map[string]any
			if err := json.Unmarshal(payload, &request); err != nil {
				return nil, err
			}
			if request["query"] != "Registry" {
				t.Fatalf("indexed query = %#v", request["query"])
			}
			return json.RawMessage(`{"matches":[{"path":"core/registry/registry.go"}]}`), nil
		}),
		"code-realtime": service.Func(func(_ context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
			record("realtime." + method)
			switch method {
			case "symbols":
				return json.RawMessage(`{"symbols":[{"name":"Registry"}]}`), nil
			case "references":
				return json.RawMessage(`{"references":[{"path":"core/host/project.go"}]}`), nil
			case "diagnostics":
				return json.RawMessage(`{"diagnostics":[]}`), nil
			default:
				t.Fatalf("unexpected realtime method %q", method)
				return nil, nil
			}
		}),
	}}
	e := New()
	if err := e.Register(reg); err != nil {
		t.Fatal(err)
	}

	raw, err := e.CallTool(
		context.Background(),
		agentsdk.Session{ProjectRoot: "/workspace"},
		"code_context",
		json.RawMessage(`{"objective":"understand registry routing","symbol":"Registry","path":"core/registry/registry.go","limit":5}`),
	)
	if err != nil {
		t.Fatal(err)
	}

	wantCalls := []string{"indexed.search", "realtime.symbols", "realtime.references", "realtime.diagnostics"}
	sort.Strings(calls)
	sort.Strings(wantCalls)
	if len(calls) != len(wantCalls) {
		t.Fatalf("calls = %v; want %v", calls, wantCalls)
	}
	for i := range wantCalls {
		if calls[i] != wantCalls[i] {
			t.Fatalf("calls = %v; want %v", calls, wantCalls)
		}
	}
	if !json.Valid(raw) {
		t.Fatalf("tool result is not JSON: %s", raw)
	}
}

func TestCodeContextRunsIndexedAndRealtimeBranchesInParallel(t *testing.T) {
	reg := &fakeRegistrar{services: map[string]service.Invoker{
		"code-indexed": service.Func(func(ctx context.Context, _ string, _ json.RawMessage) (json.RawMessage, error) {
			select {
			case <-time.After(150 * time.Millisecond):
				return json.RawMessage(`{"matches":[]}`), nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}),
		"code-realtime": service.Func(func(ctx context.Context, _ string, _ json.RawMessage) (json.RawMessage, error) {
			select {
			case <-time.After(50 * time.Millisecond):
				return json.RawMessage(`{}`), nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}),
	}}
	e := New()
	if err := e.Register(reg); err != nil {
		t.Fatal(err)
	}

	started := time.Now()
	_, err := e.CallTool(
		context.Background(),
		agentsdk.Session{ProjectRoot: "/workspace"},
		"code_context",
		json.RawMessage(`{"objective":"registry","symbol":"Registry","path":"core/registry/registry.go"}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(started)
	if elapsed >= 250*time.Millisecond {
		t.Fatalf("parallel code_context took %s; want materially below ~300ms sequential latency", elapsed)
	}
}

func TestCodeContextParallelBranchesRespectCancellation(t *testing.T) {
	blocked := service.Func(func(ctx context.Context, _ string, _ json.RawMessage) (json.RawMessage, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	reg := &fakeRegistrar{services: map[string]service.Invoker{
		"code-indexed":  blocked,
		"code-realtime": blocked,
	}}
	e := New()
	if err := e.Register(reg); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	_, err := e.CallTool(
		ctx,
		agentsdk.Session{ProjectRoot: "/workspace"},
		"code_context",
		json.RawMessage(`{"objective":"registry","symbol":"Registry","path":"core/registry/registry.go"}`),
	)
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("canceled code_context took %s", elapsed)
	}
}
