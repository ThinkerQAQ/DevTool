package devcontrol

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/thinkerqaq/devtool/core/contract"
	environmentcontract "github.com/thinkerqaq/devtool/sdk/environment"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
	"github.com/thinkerqaq/devtool/sdk/project"
)

type Provider struct{}

func (Provider) ExtensionDescriptor() extensioncontract.Descriptor {
	return extensioncontract.Descriptor{
		ID:       "project.devtool",
		Kind:     extensioncontract.KindProject,
		Requires: []string{environmentcontract.ServiceName},
	}
}

func (Provider) ProjectDescriptor() contract.ProjectDescriptor {
	return contract.ProjectDescriptor{
		Identity: contract.ProjectIdentity{Name: "DevTool"},
		Commands: []contract.CommandDescriptor{
			{ID: "build", Title: "Build", Description: "Build the next DevTool binary in the configured development environment.", SideEffect: contract.SideEffectWrite},
			{ID: "package", Title: "Package", Description: "Package a verified DevTool binary in the configured development environment.", SideEffect: contract.SideEffectWrite},
			{ID: "verify", Title: "Verify", Description: "Run DevTool self-host verification in the configured development environment.", SideEffect: contract.SideEffectWrite},
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
		return build(ctx, workspace)
	case "verify":
		return verify(ctx, workspace)
	case "package":
		return packageArtifact(ctx, workspace)
	default:
		return fmt.Errorf("unknown DevTool project command %q", command)
	}
}

func build(ctx project.Context, workspace string) error {
	if err := os.MkdirAll(filepath.Join(workspace, ".devtool", "out"), 0o755); err != nil {
		return fmt.Errorf("create DevTool output directory: %w", err)
	}
	if err := ctx.Emit("progress", "Building DevTool N+1 in configured environment"); err != nil {
		return err
	}
	if err := runEnvironment(ctx, workspace, environmentcontract.CommandRequest{
		Executable: "go",
		Args: []string{
			"build",
			"-trimpath",
			"-o", ".devtool/out/" + nextBinaryName(),
			"./cmd/devtool",
		},
		Env: targetEnv(),
	}); err != nil {
		return err
	}
	return ctx.Emit("result", "DevTool N+1 built in configured environment")
}

func verify(ctx project.Context, workspace string) error {
	if err := os.MkdirAll(filepath.Join(workspace, ".devtool", "out"), 0o755); err != nil {
		return fmt.Errorf("create DevTool output directory: %w", err)
	}
	if err := ctx.Emit("progress", "Verifying DevTool self-hosting in configured environment"); err != nil {
		return err
	}

	steps := []environmentcontract.CommandRequest{
		{Executable: "go", Args: []string{"test", "./..."}},
		{Executable: "go", Args: []string{"-C", "devcontrol", "test", "./..."}},
		{
			Executable: "go",
			Args: []string{
				"build",
				"-trimpath",
				"-o", ".devtool/out/" + nextBinaryName(),
				"./cmd/devtool",
			},
		},
		{
			Executable: ".devtool/out/" + nextBinaryName(),
			Args:       []string{"project", "inspect", "--json"},
		},
	}
	for _, step := range steps {
		if err := runEnvironment(ctx, workspace, step); err != nil {
			return err
		}
	}
	return ctx.Emit("result", "DevTool self-host verification passed in configured environment")
}

func packageArtifact(ctx project.Context, workspace string) error {
	if err := verify(ctx, workspace); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(workspace, ".devtool", "artifacts"), 0o755); err != nil {
		return fmt.Errorf("create DevTool artifact directory: %w", err)
	}
	if err := ctx.Emit("progress", "Packaging DevTool in configured environment"); err != nil {
		return err
	}
	if err := runEnvironment(ctx, workspace, environmentcontract.CommandRequest{
		Executable: "go",
		Args: []string{
			"build",
			"-trimpath",
			"-ldflags=-s -w",
			"-o", ".devtool/artifacts/" + packageBinaryName(),
			"./cmd/devtool",
		},
		Env: targetEnv(),
	}); err != nil {
		return err
	}
	return ctx.Emit("result", "DevTool package exported from configured environment")
}

func runEnvironment(ctx project.Context, workspace string, request environmentcontract.CommandRequest) error {
	request.Root = workspace

	var result environmentcontract.RunResult
	if err := ctx.InvokeService(
		environmentcontract.ServiceName,
		environmentcontract.MethodRun,
		request,
		&result,
	); err != nil {
		return err
	}
	if result.ExitCode != 0 {
		message := strings.TrimSpace(result.Stderr)
		if message == "" {
			message = strings.TrimSpace(result.Stdout)
		}
		if message == "" {
			message = fmt.Sprintf("exit code %d", result.ExitCode)
		}
		return fmt.Errorf("%s failed: %s", request.Executable, message)
	}
	return nil
}

func targetEnv() []string {
	return []string{
		"CGO_ENABLED=0",
		"GOOS=" + runtime.GOOS,
		"GOARCH=" + runtime.GOARCH,
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
