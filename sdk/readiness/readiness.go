package readiness

import "context"

const (
	KindMissingDependency     = "missing_dependency"
	KindAuthorizationRequired = "authorization_required"
	KindConfigurationRequired = "configuration_required"
	KindProviderUnavailable   = "provider_unavailable"
	KindVerificationFailed    = "verification_failed"
)

type Request struct {
	Root       string   `json:"root"`
	Workspaces []string `json:"workspaces,omitempty"`
}

type Issue struct {
	Kind        string            `json:"kind"`
	Resource    string            `json:"resource,omitempty"`
	Message     string            `json:"message"`
	Remediation string            `json:"remediation,omitempty"`
	Details     map[string]string `json:"details,omitempty"`
}

type Report struct {
	Provider string            `json:"provider"`
	Ready    bool              `json:"ready"`
	Issues   []Issue           `json:"issues,omitempty"`
	Details  map[string]string `json:"details,omitempty"`
}

type Checker interface {
	CheckReadiness(context.Context, Request) (Report, error)
}
