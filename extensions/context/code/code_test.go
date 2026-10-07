package code

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

func TestDirectoryScopeSkipsFileDiagnostics(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	realtimeCalls := 0
	reg := &fakeRegistrar{services: map[string]service.Invoker{
		codeintelligence.IndexedServiceName: service.Func(func(_ context.Context, method string, _ json.RawMessage) (json.RawMessage, error) {
			if method != codeintelligence.MethodSearch {
				t.Fatalf("indexed method = %q", method)
			}
			return json.RawMessage("{\"results\":[]}"), nil
		}),
		codeintelligence.RealtimeServiceName: service.Func(func(_ context.Context, method string, _ json.RawMessage) (json.RawMessage, error) {
			realtimeCalls++
			return nil, fmt.Errorf("unexpected realtime method %s", method)
		}),
	}}
	ext := New()
	ext.services = reg

	response, err := ext.build(t.Context(), codecontext.BuildRequest{
		Workspace: codeintelligence.Workspace{Root: root},
		Objective: "review capability package",
		Path:      "pkg",
	})
	if err != nil {
		t.Fatal(err)
	}
	if realtimeCalls != 0 {
		t.Fatalf("realtime calls = %d, want 0", realtimeCalls)
	}
	if len(response.Indexed) == 0 {
		t.Fatal("expected indexed result")
	}
	if len(response.NotApplied) != 1 || response.NotApplied[0] != "realtime.diagnostics: directory scope" {
		t.Fatalf("not applied = %#v", response.NotApplied)
	}
}

func TestIndexedFailureReturnsRealtimePartialResult(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := &fakeRegistrar{services: map[string]service.Invoker{
		codeintelligence.IndexedServiceName: service.Func(func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
			return nil, fmt.Errorf("index unavailable")
		}),
		codeintelligence.RealtimeServiceName: service.Func(func(_ context.Context, method string, _ json.RawMessage) (json.RawMessage, error) {
			if method != codeintelligence.MethodDiagnostics {
				t.Fatalf("realtime method = %q", method)
			}
			return json.RawMessage("{\"diagnostics\":[]}"), nil
		}),
	}}
	ext := New()
	ext.services = reg

	response, err := ext.build(t.Context(), codecontext.BuildRequest{
		Workspace: codeintelligence.Workspace{Root: root},
		Objective: "inspect file",
		Path:      "main.go",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Realtime["diagnostics"]) == 0 {
		t.Fatal("expected realtime diagnostics")
	}
	if response.Degraded["indexed"] == "" {
		t.Fatalf("degraded = %#v", response.Degraded)
	}
}

func TestNoUsableBranchFails(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	failed := service.Func(func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
		return nil, fmt.Errorf("unavailable")
	})
	ext := New()
	ext.services = &fakeRegistrar{services: map[string]service.Invoker{
		codeintelligence.IndexedServiceName:  failed,
		codeintelligence.RealtimeServiceName: failed,
	}}

	if _, err := ext.build(t.Context(), codecontext.BuildRequest{
		Workspace: codeintelligence.Workspace{Root: root},
		Objective: "inspect file",
		Path:      "main.go",
	}); err == nil {
		t.Fatal("expected no usable branch to fail")
	}
}
