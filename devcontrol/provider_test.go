package devcontrol

import (
	"runtime"
	"slices"
	"strings"
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

func TestDevtoolBuildArgsIncludesBuildMetadata(t *testing.T) {
	args := devtoolBuildArgs(".devtool/out/devtool-next", buildMetadata{
		Version: "1.2.3-test",
		Commit:  "abc123",
		Dirty:   true,
	}, false)
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"github.com/thinkerqaq/devtool/internal/buildinfo.Version=1.2.3-test",
		"github.com/thinkerqaq/devtool/internal/buildinfo.Commit=abc123",
		"github.com/thinkerqaq/devtool/internal/buildinfo.Dirty=true",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("devtoolBuildArgs() = %q, missing %q", joined, want)
		}
	}
}

func TestValidVersion(t *testing.T) {
	for _, value := range []string{"0.1.0-dev", "1.2.3+build.4", "v2.0.0"} {
		if !validVersion(value) {
			t.Fatalf("validVersion(%q) = false", value)
		}
	}
	for _, value := range []string{"", "../bad", "one two", "1/2"} {
		if validVersion(value) {
			t.Fatalf("validVersion(%q) = true", value)
		}
	}
}
