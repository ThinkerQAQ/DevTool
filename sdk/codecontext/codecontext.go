package codecontext

import (
	"encoding/json"

	"github.com/thinkerqaq/devtool/sdk/codeintelligence"
)

const (
	ServiceName = "code-context"
	MethodBuild = "build"
)

type BuildRequest struct {
	codeintelligence.Workspace
	Objective   string `json:"objective"`
	Symbol      string `json:"symbol,omitempty"`
	Path        string `json:"path,omitempty"`
	IncludeBody bool   `json:"include_body,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}

type BuildResponse struct {
	Objective  string                     `json:"objective"`
	Indexed    json.RawMessage            `json:"indexed,omitempty"`
	Realtime   map[string]json.RawMessage `json:"realtime,omitempty"`
	Degraded   map[string]string          `json:"degraded,omitempty"`
	NotApplied []string                   `json:"not_applied,omitempty"`
}
