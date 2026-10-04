package host

import (
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/thinkerqaq/devtool/core/contract"
	coreextension "github.com/thinkerqaq/devtool/core/extension"
	"github.com/thinkerqaq/devtool/core/project"
	"github.com/thinkerqaq/devtool/core/registry"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

type ExtensionResolver func(source string) (extensioncontract.Extension, error)

type ProjectHost struct {
	Project    project.Project
	Extension  extensioncontract.Descriptor
	Descriptor contract.ProjectDescriptor
	Registry   *registry.Registry
	process    coreextension.ProjectProcess
}

func OpenProject(ctx context.Context, start string, resolve ExtensionResolver) (*ProjectHost, error) {
	p, err := project.Discover(start)
	if err != nil {
		return nil, err
	}
	projectExtension, ok := p.Config.Extension["project"]
	if !ok {
		return nil, fmt.Errorf("project extension is not configured")
	}
	if projectExtension.Type != "go" {
		return nil, fmt.Errorf("project extension type %q is unsupported", projectExtension.Type)
	}

	reg := registry.New()
	names := make([]string, 0, len(p.Config.Extension))
	for name := range p.Config.Extension {
		if name != "project" {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	for _, name := range names {
		configured := p.Config.Extension[name]
		if configured.Type != "builtin" {
			return nil, fmt.Errorf("extension %q type %q is unsupported by the host", name, configured.Type)
		}
		if resolve == nil {
			return nil, fmt.Errorf("extension %q requires a resolver", name)
		}
		ext, err := resolve(configured.Source)
		if err != nil {
			return nil, fmt.Errorf("resolve extension %q: %w", name, err)
		}
		if err := ext.Register(reg); err != nil {
			return nil, fmt.Errorf("register extension %q: %w", ext.Descriptor().ID, err)
		}
	}

	for serviceName, configured := range p.Config.Service {
		provider, ok := reg.ServiceProvider(serviceName)
		if !ok {
			return nil, fmt.Errorf("configured service %q has no registered provider", serviceName)
		}
		if provider != configured.Provider {
			return nil, fmt.Errorf("service %q expected provider %q, got %q", serviceName, configured.Provider, provider)
		}
	}

	process := coreextension.ProjectProcess{
		Project:  p,
		Module:   projectExtension.Module,
		Package:  projectExtension.Package,
		Services: reg,
	}
	extensionDescriptor, projectDescriptor, err := process.Describe(ctx)
	if err != nil {
		return nil, err
	}
	if projectDescriptor.Identity.Name != p.Config.Project.Name {
		return nil, fmt.Errorf("project extension identity %q does not match config project name %q", projectDescriptor.Identity.Name, p.Config.Project.Name)
	}
	return &ProjectHost{
		Project:    p,
		Extension:  extensionDescriptor,
		Descriptor: projectDescriptor,
		Registry:   reg,
		process:    process,
	}, nil
}

func (h *ProjectHost) Execute(ctx context.Context, command string, args []string, out io.Writer) error {
	var descriptor *contract.CommandDescriptor
	for i := range h.Descriptor.Commands {
		if h.Descriptor.Commands[i].ID == command {
			descriptor = &h.Descriptor.Commands[i]
			break
		}
	}
	if descriptor == nil {
		return fmt.Errorf("unknown project command %q", command)
	}
	if len(descriptor.Parameters) == 0 && len(args) != 0 {
		return fmt.Errorf("command %q does not accept arguments", command)
	}
	if len(descriptor.Parameters) != 0 {
		return fmt.Errorf("typed command arguments are not implemented yet for command %q", command)
	}
	return h.process.Execute(ctx, command, nil, out)
}
