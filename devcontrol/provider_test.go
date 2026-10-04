package devcontrol

import (
	"runtime"
	"testing"

	"github.com/thinkerqaq/devtool/core/contract"
	"github.com/thinkerqaq/devtool/sdk/portable"
)

func TestProjectDescriptorIsValid(t *testing.T) {
	if err := contract.ValidateProjectDescriptor(Provider{}.ProjectDescriptor()); err != nil {
		t.Fatalf("ProjectDescriptor() is invalid: %v", err)
	}
}

func TestProjectExtensionRequiresPortableRuntime(t *testing.T) {
	descriptor := Provider{}.ExtensionDescriptor()
	if len(descriptor.Requires) != 1 || descriptor.Requires[0] != portable.ServiceName {
		t.Fatalf("Requires = %#v, want [%q]", descriptor.Requires, portable.ServiceName)
	}
}

func TestTargetArgsUseHostTarget(t *testing.T) {
	args := targetArgs()
	if args["target-os"] != runtime.GOOS || args["target-arch"] != runtime.GOARCH {
		t.Fatalf("targetArgs() = %#v, want %s/%s", args, runtime.GOOS, runtime.GOARCH)
	}
}