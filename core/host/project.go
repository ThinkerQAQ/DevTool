package host

import (
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/thinkerqaq/devtool/core/config"
	"github.com/thinkerqaq/devtool/core/contract"
	coreextension "github.com/thinkerqaq/devtool/core/extension"
	"github.com/thinkerqaq/devtool/core/project"
	"github.com/thinkerqaq/devtool/core/registry"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
	"github.com/thinkerqaq/devtool/sdk/readiness"
)

type ExecutableResolver func(context.Context, project.Project, string, config.Extension) (string, error)

type ReadinessProviderEntry struct {
	ExtensionID string
	Provides    []string
	Checker     readiness.Checker
}

type ProjectHost struct {
	Project    project.Project
	Extension  extensioncontract.Descriptor
	Descriptor contract.ProjectDescriptor
	Registry   *registry.Registry
	process    *coreextension.ProjectProcess
	closers    []io.Closer
	readiness  []ReadinessProviderEntry
}

func OpenProject(ctx context.Context, start string, resolve ExecutableResolver) (*ProjectHost, error) {
	p, err := project.Discover(start)
	if err != nil {
		return nil, err
	}
	projectExtension, ok := p.Config.Extension["project"]
	if !ok {
		return nil, fmt.Errorf("project extension is not configured")
	}
	if resolve == nil {
		return nil, fmt.Errorf("extension executable resolver is required")
	}

	reg := registry.New()
	var descriptors []extensioncontract.Descriptor
	var closers []io.Closer
	var readinessProviders []ReadinessProviderEntry
	keepExtensions := false
	defer func() {
		if keepExtensions {
			return
		}
		closeExtensions(closers)
	}()

	names := make([]string, 0, len(p.Config.Extension))
	for name := range p.Config.Extension {
		if name != "project" {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	for _, name := range names {
		configured := p.Config.Extension[name]
		executable, err := resolve(ctx, p, name, configured)
		if err != nil {
			return nil, fmt.Errorf("resolve extension %q executable: %w", name, err)
		}
		ext, err := coreextension.LoadProcessExtension(ctx, p, name, executable, configured.Settings, reg)
		if err != nil {
			return nil, fmt.Errorf("load extension %q: %w", name, err)
		}
		descriptor := ext.Descriptor()
		descriptors = append(descriptors, descriptor)
		closers = append(closers, ext)
		if descriptor.Readiness {
			readinessProviders = append(readinessProviders, ReadinessProviderEntry{
				ExtensionID: descriptor.ID,
				Provides:    append([]string(nil), descriptor.Provides...),
				Checker:     ext,
			})
		}
		if err := ext.Register(reg); err != nil {
			return nil, fmt.Errorf("register extension %q: %w", ext.Descriptor().ID, err)
		}
	}

	for serviceName, configured := range p.Config.Service {
		if err := reg.SelectService(serviceName, configured.Provider); err != nil {
			return nil, fmt.Errorf("configure service %q: %w", serviceName, err)
		}
	}

	for _, descriptor := range descriptors {
		for _, required := range descriptor.Requires {
			if _, ok := reg.Service(required); !ok {
				return nil, fmt.Errorf("extension %q requires service %q, but no provider is selected", descriptor.ID, required)
			}
		}
	}

	projectExecutable, err := resolve(ctx, p, "project", projectExtension)
	if err != nil {
		return nil, fmt.Errorf("resolve project extension executable: %w", err)
	}
	process, err := coreextension.StartProjectProcess(ctx, p, projectExecutable, projectExtension.Settings, reg)
	if err != nil {
		return nil, fmt.Errorf("start project extension: %w", err)
	}
	keepProjectProcess := false
	defer func() {
		if !keepProjectProcess {
			_ = process.Close()
		}
	}()
	extensionDescriptor, projectDescriptor, err := process.Describe(ctx)
	if err != nil {
		return nil, err
	}
	if projectDescriptor.Identity.Name != p.Config.Project.Name {
		return nil, fmt.Errorf("project extension identity %q does not match config project name %q", projectDescriptor.Identity.Name, p.Config.Project.Name)
	}
	for _, required := range extensionDescriptor.Requires {
		if _, ok := reg.Service(required); !ok {
			return nil, fmt.Errorf("project extension %q requires service %q, but no provider is registered", extensionDescriptor.ID, required)
		}
	}
	projectTools, err := newProjectToolProvider(process, projectDescriptor)
	if err != nil {
		return nil, fmt.Errorf("register project agent tools: %w", err)
	}
	if projectTools.Count() != 0 {
		if err := reg.ProvideAgentTools(extensionDescriptor.ID+".commands", projectTools); err != nil {
			return nil, fmt.Errorf("register project agent tools: %w", err)
		}
	}

	h := &ProjectHost{
		Project:    p,
		Extension:  extensionDescriptor,
		Descriptor: projectDescriptor,
		Registry:   reg,
		process:    process,
		closers:    closers,
		readiness:  readinessProviders,
	}
	keepProjectProcess = true
	keepExtensions = true
	return h, nil
}

func (h *ProjectHost) ReadinessProviders() []ReadinessProviderEntry {
	if h == nil {
		return nil
	}
	out := make([]ReadinessProviderEntry, 0, len(h.readiness))
	for _, entry := range h.readiness {
		active := len(entry.Provides) == 0
		for _, serviceName := range entry.Provides {
			provider, ok := h.Registry.ServiceProvider(serviceName)
			if ok && provider == entry.ExtensionID {
				active = true
				break
			}
		}
		if active {
			out = append(out, entry)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ExtensionID < out[j].ExtensionID })
	return out
}

func (h *ProjectHost) Close() error {
	if h == nil {
		return nil
	}
	var firstErr error
	if h.process != nil {
		if err := h.process.Close(); err != nil {
			firstErr = err
		}
		h.process = nil
	}
	if err := closeExtensions(h.closers); err != nil && firstErr == nil {
		firstErr = err
	}
	h.closers = nil
	return firstErr
}

func closeExtensions(closers []io.Closer) error {
	var firstErr error
	for i := len(closers) - 1; i >= 0; i-- {
		if err := closers[i].Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
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
