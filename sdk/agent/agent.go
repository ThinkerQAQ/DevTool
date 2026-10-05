package agent

import (
	"context"
	"encoding/json"
)

type Session struct {
	ProjectRoot      string   `json:"project_root"`
	Workspaces       []string `json:"workspaces,omitempty"`
	Context          string   `json:"context,omitempty"`
}

type Tool struct {
	Name       string
	Definition json.RawMessage
}

type ToolProvider interface {
	ListTools(context.Context, Session) ([]Tool, error)
	CallTool(context.Context, Session, string, json.RawMessage) (json.RawMessage, error)
}
