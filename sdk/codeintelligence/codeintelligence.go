package codeintelligence

const (
	IndexedServiceName  = "code-indexed"
	RealtimeServiceName = "code-realtime"

	MethodDoctor      = "doctor"
	MethodVerify      = "verify"
	MethodSearch      = "search"
	MethodSymbols     = "symbols"
	MethodReferences  = "references"
	MethodDiagnostics = "diagnostics"
)

type Workspace struct {
	Root             string   `json:"root"`
	Workspaces       []string `json:"workspaces,omitempty"`
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


type SearchRequest struct {
	Workspace
	Query      string `json:"query"`
	Repository string `json:"repository,omitempty"`
	Revision   string `json:"revision,omitempty"`
	Limit      int    `json:"limit,omitempty"`
}

type SymbolRequest struct {
	Workspace
	Symbol      string `json:"symbol"`
	Path        string `json:"path,omitempty"`
	IncludeBody bool   `json:"include_body,omitempty"`
}

type ReferencesRequest struct {
	Workspace
	Symbol string `json:"symbol"`
	Path   string `json:"path"`
}

type DiagnosticsRequest struct {
	Workspace
	Path string `json:"path"`
}
