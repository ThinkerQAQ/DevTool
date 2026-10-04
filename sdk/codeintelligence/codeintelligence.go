package codeintelligence

import "encoding/json"

const (
	GraphServiceName = "code-graph"
	LSPServiceName   = "code-lsp"

	MethodDoctor = "doctor"
	MethodMCP    = "mcp"
	MethodSync   = "sync"
	MethodQuery  = "query"
	MethodVerify = "verify"
)

type Workspace struct {
	Root             string   `json:"root"`
	Workspaces       []string `json:"workspaces,omitempty"`
	EnvironmentImage string   `json:"environment_image,omitempty"`
}

type GraphQuery struct {
	Workspace
	Tool string          `json:"tool"`
	Args json.RawMessage `json:"args,omitempty"`
}

type MCPRequest struct {
	Workspace
	Context string `json:"context,omitempty"`
}

type DoctorResponse struct {
	Provider   string `json:"provider"`
	Executable string `json:"executable"`
	Version    string `json:"version,omitempty"`
}

type VerifyResponse struct {
	Provider string `json:"provider"`
	Output   string `json:"output,omitempty"`
}
