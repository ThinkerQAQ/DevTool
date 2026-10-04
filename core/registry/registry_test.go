package registry

import (
	"testing"

	"github.com/thinkerqaq/devtool/core/contract"
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
	if err := r.ProvideService("portable-runtime", "runtime.one", struct{}{}); err != nil {
		t.Fatal(err)
	}
	if err := r.ProvideService("portable-runtime", "runtime.two", struct{}{}); err == nil {
		t.Fatal("ProvideService() expected duplicate provider error")
	}
}
