package agent

import (
	"context"
	"encoding/json"
)

type Session struct {
	ProjectRoot string   `json:"project_root"`
	Workspaces  []string `json:"workspaces,omitempty"`
	Context     string   `json:"context,omitempty"`
}

type Tool struct {
	Name       string
	Definition json.RawMessage
}

type ToolProvider interface {
	ListTools(context.Context, Session) ([]Tool, error)
	CallTool(context.Context, Session, string, json.RawMessage) (json.RawMessage, error)
}

// ExecutionPolicy is an optional deterministic tool boundary; no model or
// workflow engine runs inside it. Adapters may implement it using Git,
// a sandbox, or another mature execution backend.
type ExecutionPolicy interface {
	Tool() json.RawMessage
	Attest(context.Context, string, json.RawMessage, json.RawMessage) json.RawMessage
	Apply(context.Context, json.RawMessage) (json.RawMessage, error)
}
