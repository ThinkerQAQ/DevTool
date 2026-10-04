package extension

import "github.com/thinkerqaq/devtool/core/contract"

type Kind string

const (
	KindProject        Kind = "project"
	KindRuntime        Kind = "runtime"
	KindNative         Kind = "native"
	KindInfrastructure Kind = "infrastructure"
	KindPolicy         Kind = "policy"
	KindUI             Kind = "ui"
)

type Descriptor struct {
	ID       string   `json:"id"`
	Kind     Kind     `json:"kind"`
	Provides []string `json:"provides,omitempty"`
	Requires []string `json:"requires,omitempty"`
}

type Registrar interface {
	RegisterCommand(contract.CommandDescriptor) error
	RegisterResource(contract.ResourceDescriptor) error
	RegisterView(contract.ViewDescriptor) error
	RegisterFeature(contract.FeatureBinding) error
	RegisterNavigation(contract.NavigationItem) error
	ProvideService(name, extensionID string, value any) error
}

type Extension interface {
	Descriptor() Descriptor
	Register(Registrar) error
}
