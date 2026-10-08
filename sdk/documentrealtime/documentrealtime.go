package documentrealtime

const (
	ServiceName   = "document-realtime"
	MethodAnalyze = "analyze"
)

// Provider input is structural; AI clients never calculate UTF-16 LSP positions.
type AnalyzeRequest struct {
	Root        string `json:"root"`
	Path        string `json:"path"`
	HeadingLine int    `json:"heading_line"`
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
type AnalyzeResponse struct {
	Status         string     `json:"status"`
	Provider       string     `json:"provider"`
	WorkspaceScope string     `json:"workspace_scope,omitempty"`
	References     []Location `json:"references"`
	Truncated      bool       `json:"truncated,omitempty"`
	Detail         string     `json:"detail,omitempty"`
}
