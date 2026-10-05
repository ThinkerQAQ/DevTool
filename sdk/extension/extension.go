package extension

import (
	contract "github.com/thinkerqaq/devtool/sdk/contract"
	service "github.com/thinkerqaq/devtool/sdk/service"
	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
)

type Kind string

const (
	KindProject          Kind = "project"
	KindRuntime          Kind = "runtime"
	KindNative           Kind = "native"
	KindInfrastructure   Kind = "infrastructure"
	KindCodeIntelligence Kind = "code-intelligence"
	KindCapability       Kind = "capability"
	KindPolicy           Kind = "policy"
	KindUI               Kind = "ui"
)

type Descriptor struct {
	ID         string   `json:"id"`
	Kind       Kind     `json:"kind"`
	Provides   []string `json:"provides,omitempty"`
	Requires   []string `json:"requires,omitempty"`
	AgentTools bool     `json:"agent_tools,omitempty"`
}

type Registrar interface {
	RegisterCommand(contract.CommandDescriptor) error
	RegisterResource(contract.ResourceDescriptor) error
	RegisterView(contract.ViewDescriptor) error
	RegisterFeature(contract.FeatureBinding) error
	RegisterNavigation(contract.NavigationItem) error
	ProvideService(name, extensionID string, value service.Invoker) error
	Service(name string) (service.Invoker, bool)
	ProvideAgentTools(extensionID string, provider agentsdk.ToolProvider) error
}

type Extension interface {
	Descriptor() Descriptor
	Register(Registrar) error
}
