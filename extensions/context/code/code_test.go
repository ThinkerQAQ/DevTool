package code

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
	"github.com/thinkerqaq/devtool/sdk/codecontext"
	"github.com/thinkerqaq/devtool/sdk/codeintelligence"
	contract "github.com/thinkerqaq/devtool/sdk/contract"
	"github.com/thinkerqaq/devtool/sdk/service"
)

type fakeRegistrar struct {
	services map[string]service.Invoker
	provided map[string]service.Invoker
}

func (r *fakeRegistrar) RegisterCommand(contract.CommandDescriptor) error      { return nil }
func (r *fakeRegistrar) RegisterResource(contract.ResourceDescriptor) error    { return nil }
func (r *fakeRegistrar) RegisterView(contract.ViewDescriptor) error            { return nil }
func (r *fakeRegistrar) RegisterFeature(contract.FeatureBinding) error         { return nil }
func (r *fakeRegistrar) RegisterNavigation(contract.NavigationItem) error      { return nil }
func (r *fakeRegistrar) ProvideAgentTools(string, agentsdk.ToolProvider) error { return nil }
func (r *fakeRegistrar) ProvideService(name, _ string, invoker service.Invoker) error {
	if r.provided == nil {
		r.provided = map[string]service.Invoker{}
	}
	r.provided[name] = invoker
	return nil
}
func (r *fakeRegistrar) Service(name string) (service.Invoker, bool) {
	value, ok := r.services[name]
	return value, ok
}

func TestIndexedOnlyUsesNativeSymbolSearch(t *testing.T) {
	var searches []codeintelligence.SearchRequest
	reg := &fakeRegistrar{services: map[string]service.Invoker{
		codeintelligence.IndexedServiceName: service.Func(func(_ context.Context, method string, raw json.RawMessage) (json.RawMessage, error) {
			if method != codeintelligence.MethodSearch {
				t.Fatalf("method = %q", method)
			}
			var request codeintelligence.SearchRequest
			if err := json.Unmarshal(raw, &request); err != nil {
				t.Fatal(err)
			}
			searches = append(searches, request)
			return json.RawMessage(`{"results":[{"name":"AreaAnchor","path":"Models.kt"}]}`), nil
		}),
		codeintelligence.RealtimeServiceName: service.Func(func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
			t.Fatal("realtime provider must not be invoked by code context")
			return nil, nil
		}),
	}}
	ext := New()
	ext.services = reg
	for _, path := range []string{"", "Models.kt"} {
		response, err := ext.build(t.Context(), codecontext.BuildRequest{
			Workspace: codeintelligence.Workspace{Root: t.TempDir()},
			Objective: "Find AreaAnchor definition",
			Symbol:    "AreaAnchor",
			Path:      path,
			Limit:     5,
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(response.Indexed) == 0 || len(response.Realtime) != 0 || len(response.Degraded) != 0 {
			t.Fatalf("unexpected context: %+v", response)
		}
		if len(response.NotApplied) == 0 || response.NotApplied[0] != "realtime code intelligence is not configured" {
			t.Fatalf("missing realtime marker: %+v", response.NotApplied)
		}
		if path != "" && len(response.NotApplied) != 2 {
			t.Fatalf("scope note missing: %+v", response.NotApplied)
		}
	}
	if len(searches) != 2 {
		t.Fatalf("search calls = %d", len(searches))
	}
	for _, request := range searches {
		if request.Discovery || request.Query != "AreaAnchor" || request.Limit != 5 {
			t.Fatalf("expected direct native indexed search, got %+v", request)
		}
	}
}

func TestIndexedProviderFailurePropagates(t *testing.T) {
	reg := &fakeRegistrar{services: map[string]service.Invoker{
		codeintelligence.IndexedServiceName: service.Func(func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
			return nil, fmt.Errorf("index unavailable")
		}),
	}}
	ext := New()
	ext.services = reg
	_, err := ext.build(t.Context(), codecontext.BuildRequest{
		Workspace: codeintelligence.Workspace{Root: t.TempDir()},
		Objective: "inspect source",
	})
	if err == nil {
		t.Fatal("expected indexed provider failure")
	}
}
