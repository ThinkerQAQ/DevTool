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

func TestRegistryRejectsDuplicateService(t *testing.T) {
	r := New()
	invoker := service.Func(func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{}`), nil
	})
	if err := r.ProvideService("portable-runtime", "runtime.one", invoker); err != nil {
		t.Fatal(err)
	}
	if err := r.ProvideService("portable-runtime", "runtime.two", invoker); err == nil {
		t.Fatal("ProvideService() expected duplicate provider error")
	}
}
