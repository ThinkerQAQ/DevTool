package devcontrol

import (
	"fmt"

	"github.com/thinkerqaq/devtool/core/contract"
	"github.com/thinkerqaq/devtool/core/extension"
	"github.com/thinkerqaq/devtool/sdk/project"
)

type Provider struct{}

func (Provider) ExtensionDescriptor() extension.Descriptor {
	return extension.Descriptor{
		ID:   "project.devtool",
		Kind: extension.KindProject,
	}
}

func (Provider) ProjectDescriptor() contract.ProjectDescriptor {
	return contract.ProjectDescriptor{
		Identity: contract.ProjectIdentity{Name: "DevTool"},
		Commands: []contract.CommandDescriptor{
			{ID: "build", Title: "Build", Description: "Build the next DevTool binary.", SideEffect: contract.SideEffectWrite},
			{ID: "package", Title: "Package", Description: "Package the verified DevTool binary.", SideEffect: contract.SideEffectWrite},
			{ID: "verify", Title: "Verify", Description: "Run tests and verify a newly built DevTool against itself.", SideEffect: contract.SideEffectWrite},
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
	emit := func(kind, message string) {
		_ = ctx.Emit(kind, message)
	}
	switch command {
	case "build":
		return Build(ctx.Context, emit)
	case "verify":
		return Verify(ctx.Context, emit)
	case "package":
		return Package(ctx.Context, emit)
	default:
		return fmt.Errorf("unknown DevTool project command %q", command)
	}
}
