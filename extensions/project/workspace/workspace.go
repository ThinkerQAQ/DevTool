package workspace

import (
	"fmt"
	"strings"

	contract "github.com/thinkerqaq/devtool/sdk/contract"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
	projectsdk "github.com/thinkerqaq/devtool/sdk/project"
)

const ExtensionID = "project.workspace"

type Provider struct {
	name string
}

func (p *Provider) Configure(settings map[string]any) error {
	raw, ok := settings["name"]
	if !ok {
		return fmt.Errorf("project.workspace setting name is required")
	}
	name, ok := raw.(string)
	if !ok || strings.TrimSpace(name) == "" {
		return fmt.Errorf("project.workspace setting name must be a non-empty string")
	}
	p.name = strings.TrimSpace(name)
	return nil
}

func (p *Provider) ExtensionDescriptor() extensioncontract.Descriptor {
	return extensioncontract.Descriptor{
		ID:   ExtensionID,
		Kind: extensioncontract.KindProject,
	}
}

func (p *Provider) ProjectDescriptor() contract.ProjectDescriptor {
	return contract.ProjectDescriptor{
		Identity: contract.ProjectIdentity{Name: p.name},
	}
}

func (p *Provider) Execute(_ projectsdk.Context, command string, _ map[string]any) error {
	return fmt.Errorf("project.workspace has no command %q", command)
}
