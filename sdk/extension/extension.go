package extension

import (
	"context"

	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
	contract "github.com/thinkerqaq/devtool/sdk/contract"
	"github.com/thinkerqaq/devtool/sdk/readiness"
	service "github.com/thinkerqaq/devtool/sdk/service"
)

type Kind string

const (
	KindProject              Kind = "project"
	KindRuntime              Kind = "runtime"
	KindNative               Kind = "native"
	KindInfrastructure       Kind = "infrastructure"
	KindCodeIntelligence     Kind = "code-intelligence"
	KindDocumentIntelligence Kind = "document-intelligence"
	KindCapability           Kind = "capability"
	KindPolicy               Kind = "policy"
	KindUI                   Kind = "ui"
)

type Descriptor struct {
	ID         string   `json:"id"`
	Kind       Kind     `json:"kind"`
	Provides   []string `json:"provides,omitempty"`
	Requires   []string `json:"requires,omitempty"`
	AgentTools bool     `json:"agent_tools,omitempty"`
	Readiness  bool     `json:"readiness,omitempty"`
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

type ReadinessProvider interface {
	CheckReadiness(context.Context, readiness.Request) (readiness.Report, error)
}

type Configurable interface {
	Configure(map[string]any) error
}
