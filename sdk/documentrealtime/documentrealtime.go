package documentrealtime

const (
	ServiceName   = "document-realtime"
	MethodAnalyze = "analyze"
)

// AnalyzeRequest asks a realtime provider to inspect a workspace document.
// Line and Column are 1-based; Column is an LSP UTF-16 code-unit offset.
type AnalyzeRequest struct {
	Root              string `json:"root"`
	Path              string `json:"path"`
	Line              int    `json:"line,omitempty"`
	Column            int    `json:"column,omitempty"`
	IncludeReferences bool   `json:"include_references,omitempty"`
}

type Position struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}
type Location struct {
	Path  string `json:"path"`
	Range Range  `json:"range"`
}
type Symbol struct {
	Name  string `json:"name"`
	Level int    `json:"level,omitempty"`
	Range Range  `json:"range"`
}
type Diagnostic struct {
	Message  string `json:"message"`
	Severity int    `json:"severity,omitempty"`
	Range    Range  `json:"range"`
}
type AnalyzeResponse struct {
	Status            string       `json:"status"`
	Provider          string       `json:"provider"`
	WorkspaceScope    string       `json:"workspace_scope,omitempty"`
	Symbols           []Symbol     `json:"symbols"`
	Diagnostics       []Diagnostic `json:"diagnostics"`
	DiagnosticsStatus string       `json:"diagnostics_status"`
	Definitions       []Location   `json:"definitions"`
	References        []Location   `json:"references"`
	Detail            string       `json:"detail,omitempty"`
}
