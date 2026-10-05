package registry

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/thinkerqaq/devtool/core/contract"
	"github.com/thinkerqaq/devtool/core/service"
)

func TestRegistryBuildsDescriptor(t *testing.T) {
	r := New()
	if err := r.RegisterCommand(contract.CommandDescriptor{ID: "build", SideEffect: contract.SideEffectWrite}); err != nil {
		t.Fatal(err)
	}
	if err := r.RegisterResource(contract.ResourceDescriptor{ID: "environment"}); err != nil {
		t.Fatal(err)
	}
	d := r.ProjectDescriptor(contract.ProjectIdentity{Name: "example"})
	if got := len(d.Commands); got != 1 {
		t.Fatalf("commands = %d, want 1", got)
	}
	if got := len(d.Resources); got != 1 {
		t.Fatalf("resources = %d, want 1", got)
	}
}

func TestRegistryAllowsMultipleServiceProvidersAndSelectsOne(t *testing.T) {
	r := New()
	one := service.Func(func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{"provider":"one"}`), nil
	})
	two := service.Func(func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{"provider":"two"}`), nil
	})
	if err := r.ProvideService("code-indexed", "intelligence.one", one); err != nil {
		t.Fatal(err)
	}
	if err := r.ProvideService("code-indexed", "intelligence.two", two); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Service("code-indexed"); ok {
		t.Fatal("Service() should be ambiguous before provider selection")
	}
	if err := r.SelectService("code-indexed", "intelligence.two"); err != nil {
		t.Fatal(err)
	}
	provider, ok := r.ServiceProvider("code-indexed")
	if !ok || provider != "intelligence.two" {
		t.Fatalf("ServiceProvider() = %q, %v; want intelligence.two, true", provider, ok)
	}
}

func TestRegistryRejectsDuplicateServiceProvider(t *testing.T) {
	r := New()
	invoker := service.Func(func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{}`), nil
	})
	if err := r.ProvideService("portable-runtime", "runtime.one", invoker); err != nil {
		t.Fatal(err)
	}
	if err := r.ProvideService("portable-runtime", "runtime.one", invoker); err == nil {
		t.Fatal("ProvideService() expected duplicate provider error")
	}
}
