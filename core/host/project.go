package host

import (
	"context"
	"fmt"
	"io"

	"github.com/thinkerqaq/devtool/core/contract"
	"github.com/thinkerqaq/devtool/core/extension"
	"github.com/thinkerqaq/devtool/core/project"
)

type ProjectHost struct {
	Project   project.Project
	Extension extension.Descriptor
	Descriptor contract.ProjectDescriptor
	process   extension.ProjectProcess
}

func OpenProject(ctx context.Context, start string) (*ProjectHost, error) {
	p, err := project.Discover(start)
	if err != nil {
		return nil, err
	}
	projectExtension, ok := p.Config.Extension["project"]
	if !ok {
		return nil, fmt.Errorf("project extension is not configured")
	}
	process := extension.ProjectProcess{
		Project: p,
		Source:  projectExtension.Source,
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
