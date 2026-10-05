package registry

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/thinkerqaq/devtool/core/contract"
	"github.com/thinkerqaq/devtool/core/service"
	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
)

type serviceEntry struct {
	extensionID string
	value       service.Invoker
}

type AgentToolProviderEntry struct {
	ExtensionID string
	Provider    agentsdk.ToolProvider
}

type Registry struct {
	mu             sync.RWMutex
	commands       map[string]contract.CommandDescriptor
	resources      map[string]contract.ResourceDescriptor
	views          map[string]contract.ViewDescriptor
	features       map[string]contract.FeatureBinding
	navigation     map[string]contract.NavigationItem
	services       map[string]map[string]service.Invoker
	selected       map[string]string
	agentProviders map[string]agentsdk.ToolProvider
}

func New() *Registry {
	return &Registry{
		commands:       map[string]contract.CommandDescriptor{},
		resources:      map[string]contract.ResourceDescriptor{},
		views:          map[string]contract.ViewDescriptor{},
		features:       map[string]contract.FeatureBinding{},
		navigation:     map[string]contract.NavigationItem{},
		services:       map[string]map[string]service.Invoker{},
		selected:       map[string]string{},
		agentProviders: map[string]agentsdk.ToolProvider{},
	}
}

func (r *Registry) RegisterCommand(desc contract.CommandDescriptor) error {
	return putUnique(&r.mu, r.commands, desc.ID, desc, "command")
}

func (r *Registry) RegisterResource(desc contract.ResourceDescriptor) error {
	return putUnique(&r.mu, r.resources, desc.ID, desc, "resource")
}

func (r *Registry) RegisterView(desc contract.ViewDescriptor) error {
	return putUnique(&r.mu, r.views, desc.ID, desc, "view")
}

func (r *Registry) RegisterFeature(desc contract.FeatureBinding) error {
	return putUnique(&r.mu, r.features, desc.ID, desc, "feature binding")
}

func (r *Registry) RegisterNavigation(desc contract.NavigationItem) error {
	return putUnique(&r.mu, r.navigation, desc.ID, desc, "navigation item")
}

func putUnique[T any](mu *sync.RWMutex, values map[string]T, id string, value T, kind string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%s id is required", kind)
	}
	mu.Lock()
	defer mu.Unlock()
	if _, exists := values[id]; exists {
		return fmt.Errorf("%s %q is already registered", kind, id)
	}
	values[id] = value
	return nil
}

func (r *Registry) ProvideService(name, extensionID string, value service.Invoker) error {
	name = strings.TrimSpace(name)
	extensionID = strings.TrimSpace(extensionID)
	if name == "" {
		return fmt.Errorf("service name is required")
	}
	if extensionID == "" {
		return fmt.Errorf("service %q requires provider extension id", name)
	}
	if value == nil {
		return fmt.Errorf("service %q cannot register a nil implementation", name)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	providers := r.services[name]
	if providers == nil {
		providers = map[string]service.Invoker{}
		r.services[name] = providers
	}
	if _, exists := providers[extensionID]; exists {
		return fmt.Errorf("service %q is already provided by extension %q", name, extensionID)
	}
	providers[extensionID] = value
	return nil
}

func (r *Registry) SelectService(name, extensionID string) error {
	name = strings.TrimSpace(name)
	extensionID = strings.TrimSpace(extensionID)
	r.mu.Lock()
	defer r.mu.Unlock()
	providers := r.services[name]
	if providers == nil {
		return fmt.Errorf("service %q has no registered providers", name)
	}
	if _, ok := providers[extensionID]; !ok {
		return fmt.Errorf("service %q has no provider %q", name, extensionID)
	}
	r.selected[name] = extensionID
	return nil
}

func (r *Registry) Service(name string) (service.Invoker, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	providers := r.services[name]
	if len(providers) == 0 {
		return nil, false
	}
	if selected := r.selected[name]; selected != "" {
		value, ok := providers[selected]
		return value, ok
	}
	if len(providers) != 1 {
		return nil, false
	}
	for _, value := range providers {
		return value, true
	}
	return nil, false
}

func (r *Registry) ServiceProvider(name string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	providers := r.services[name]
	if len(providers) == 0 {
		return "", false
	}
	if selected := r.selected[name]; selected != "" {
		_, ok := providers[selected]
		return selected, ok
	}
	if len(providers) != 1 {
		return "", false
	}
	for provider := range providers {
		return provider, true
	}
	return "", false
}

func (r *Registry) ProvideAgentTools(extensionID string, provider agentsdk.ToolProvider) error {
	extensionID = strings.TrimSpace(extensionID)
	if extensionID == "" {
		return fmt.Errorf("agent tool provider requires extension id")
	}
	if provider == nil {
		return fmt.Errorf("agent tool provider %q cannot be nil", extensionID)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.agentProviders[extensionID]; exists {
		return fmt.Errorf("agent tool provider %q is already registered", extensionID)
	}
	r.agentProviders[extensionID] = provider
	return nil
}

func (r *Registry) AgentToolProviders() []AgentToolProviderEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.agentProviders))
	for id := range r.agentProviders {
		if r.agentProviderActiveLocked(id) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	entries := make([]AgentToolProviderEntry, 0, len(ids))
	for _, id := range ids {
		entries = append(entries, AgentToolProviderEntry{ExtensionID: id, Provider: r.agentProviders[id]})
	}
	return entries
}

func (r *Registry) agentProviderActiveLocked(extensionID string) bool {
	providesService := false
	for serviceName, providers := range r.services {
		if _, ok := providers[extensionID]; !ok {
			continue
		}
		providesService = true
		selected := r.selected[serviceName]
		if selected == extensionID || (selected == "" && len(providers) == 1) {
			return true
		}
	}
	return !providesService
}

func (r *Registry) ProjectDescriptor(identity contract.ProjectIdentity) contract.ProjectDescriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	d := contract.ProjectDescriptor{Identity: identity}
	for _, value := range r.commands {
		d.Commands = append(d.Commands, value)
	}
	for _, value := range r.resources {
		d.Resources = append(d.Resources, value)
	}
	for _, value := range r.views {
		d.Views = append(d.Views, value)
	}
	for _, value := range r.features {
		d.Features = append(d.Features, value)
	}
	for _, value := range r.navigation {
		d.Navigation = append(d.Navigation, value)
	}

	sort.Slice(d.Commands, func(i, j int) bool { return d.Commands[i].ID < d.Commands[j].ID })
	sort.Slice(d.Resources, func(i, j int) bool { return d.Resources[i].ID < d.Resources[j].ID })
	sort.Slice(d.Views, func(i, j int) bool { return d.Views[i].ID < d.Views[j].ID })
	sort.Slice(d.Features, func(i, j int) bool { return d.Features[i].ID < d.Features[j].ID })
	sort.Slice(d.Navigation, func(i, j int) bool { return d.Navigation[i].ID < d.Navigation[j].ID })
	return d
}
