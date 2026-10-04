package devcontrol

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/thinkerqaq/devtool/core/contract"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
	"github.com/thinkerqaq/devtool/sdk/portable"
	"github.com/thinkerqaq/devtool/sdk/project"
)

const portableModule = "./.dagger/modules/devtool"

type Provider struct{}

func (Provider) ExtensionDescriptor() extensioncontract.Descriptor {
	return extensioncontract.Descriptor{
		ID:       "project.devtool",
		Kind:     extensioncontract.KindProject,
		Requires: []string{portable.ServiceName},
	}
}

func (Provider) ProjectDescriptor() contract.ProjectDescriptor {
	return contract.ProjectDescriptor{
		Identity: contract.ProjectIdentity{Name: "DevTool"},
		Commands: []contract.CommandDescriptor{
			{ID: "build", Title: "Build", Description: "Build the next DevTool binary through the portable runtime.", SideEffect: contract.SideEffectWrite},
			{ID: "package", Title: "Package", Description: "Package a verified DevTool binary through the portable runtime.", SideEffect: contract.SideEffectWrite},
			{ID: "runtime.doctor", Title: "Runtime Doctor", Description: "Verify the configured portable runtime.", SideEffect: contract.SideEffectRead},
			{ID: "verify", Title: "Verify", Description: "Run DevTool self-host verification through the portable runtime.", SideEffect: contract.SideEffectWrite},
		},
		Resources: []contract.ResourceDescriptor{
			{ID: "environment", Title: "Environment", Description: "DevTool development environment."},
		},
		Views: []contract.ViewDescriptor{
			{
				ID:        "overview",
				Title:     "Overview",
				Resources: []string{"environment"},
				Actions: []contract.ActionDescriptor{
					{CommandID: "build", Label: "Build"},
					{CommandID: "verify", Label: "Verify"},
					{CommandID: "package", Label: "Package"},
					{CommandID: "runtime.doctor", Label: "Runtime Doctor"},
				},
			},
		},
		Navigation: []contract.NavigationItem{
			{ID: "overview", Title: "Overview", ViewID: "overview"},
		},
	}
}

func (Provider) Execute(ctx project.Context, command string, _ map[string]any) error {
	workspace, err := os.Getwd()
	if err != nil {
		return err
	}

	switch command {
	case "build":
		return invokePortable(
			ctx,
			workspace,
			"build-artifact",
			filepath.Join(workspace, ".devtool", "out", nextBinaryName()),
			targetArgs(),
			"Building DevTool N+1 through portable-runtime",
			"DevTool N+1 built through portable-runtime",
		)
	case "verify":
		return invokePortable(
			ctx,
			workspace,
			"verify",
			"",
			nil,
			"Verifying DevTool self-hosting through portable-runtime",
			"DevTool portable self-host verification passed",
		)
	case "package":
		return invokePortable(
			ctx,
			workspace,
			"package-artifact",
			filepath.Join(workspace, ".devtool", "artifacts", packageBinaryName()),
			targetArgs(),
			"Packaging verified DevTool through portable-runtime",
			"DevTool package exported through portable-runtime",
		)
	case "runtime.doctor":
		var response portable.DoctorResponse
		if err := ctx.InvokeService(portable.ServiceName, portable.MethodDoctor, struct{}{}, &response); err != nil {
			return err
		}
		return ctx.Emit("result", fmt.Sprintf("%s READY: %s (%s)", response.Provider, response.Version, response.Smoke))
	default:
		return fmt.Errorf("unknown DevTool project command %q", command)
	}
}

func invokePortable(
	ctx project.Context,
	workspace string,
	function string,
	output string,
	args map[string]string,
	progress string,
	success string,
) error {
	if err := ctx.Emit("progress", progress); err != nil {
		return err
	}
	var result json.RawMessage
	if err := ctx.InvokeService(
		portable.ServiceName,
		portable.MethodInvoke,
		portable.Invocation{
			Workspace: workspace,
			Module:    portableModule,
			Function:  function,
			Args:      args,
			Output:    output,
		},
		&result,
	); err != nil {
		return err
	}
	return ctx.Emit("result", success)
}

func targetArgs() map[string]string {
	return map[string]string{
		"target-os":   runtime.GOOS,
		"target-arch": runtime.GOARCH,
	}
}

func nextBinaryName() string {
	if runtime.GOOS == "windows" {
		return "devtool-next.exe"
	}
	return "devtool-next"
}

func packageBinaryName() string {
	name := fmt.Sprintf("devtool-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}