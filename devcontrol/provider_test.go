package devcontrol

import (
	"runtime"
	"slices"
	"testing"

	"github.com/thinkerqaq/devtool/core/contract"
	environmentcontract "github.com/thinkerqaq/devtool/sdk/environment"
)

func TestProjectDescriptorIsValid(t *testing.T) {
	if err := contract.ValidateProjectDescriptor(Provider{}.ProjectDescriptor()); err != nil {
		t.Fatalf("ProjectDescriptor() is invalid: %v", err)
	}
}

func TestProjectExtensionRequiresEnvironment(t *testing.T) {
	descriptor := Provider{}.ExtensionDescriptor()
	if len(descriptor.Requires) != 1 || descriptor.Requires[0] != environmentcontract.ServiceName {
		t.Fatalf("Requires = %#v, want [%q]", descriptor.Requires, environmentcontract.ServiceName)
	}
}

func TestTargetEnvUsesHostTarget(t *testing.T) {
	env := targetEnv()
	for _, want := range []string{
		"CGO_ENABLED=0",
		"GOOS=" + runtime.GOOS,
		"GOARCH=" + runtime.GOARCH,
	} {
		if !slices.Contains(env, want) {
			t.Fatalf("targetEnv() = %#v, missing %q", env, want)
		}
	}
}
