package diagram

const (
	RenderServiceName       = "diagram-render"
	RenderMethod            = "render"
	ContextServiceName      = "diagram-context"
	ContextMethod           = "build"
	IntelligenceServiceName = "diagram-intelligence"
	IntelligenceMethod      = "analyze"
)

type ContextRequest struct {
	Root          string `json:"root"`
	Path          string `json:"path"`
	Objective     string `json:"objective"`
	Render        bool   `json:"render,omitempty"`
	IncludeSource bool   `json:"include_source,omitempty"`
	Analyze       bool   `json:"analyze,omitempty"`
	Index         int    `json:"index,omitempty"`
}

type RenderRequest struct {
	Language string `json:"language"`
	Source   string `json:"source"`
}

type RenderResult struct {
	Status       string `json:"status"`
	Renderer     string `json:"renderer,omitempty"`
	ArtifactPath string `json:"artifact_path,omitempty"`
	Diagnostic   string `json:"diagnostic,omitempty"`
}

// Intelligence contract: structural interpretation, not a proxy for LSP methods.
type IntelligenceRequest struct {
	Root     string          `json:"root"`
	Path     string          `json:"path"`
	Diagrams []DiagramSource `json:"diagrams"`
}
type DiagramSource struct {
	Index     int    `json:"index"`
	Language  string `json:"language"`
	Source    string `json:"source"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}
type GraphNode struct {
	ID    string `json:"id"`
	Label string `json:"label,omitempty"`
}
type GraphEdge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label,omitempty"`
}
type GraphGroup struct {
	ID    string   `json:"id"`
	Label string   `json:"label,omitempty"`
	Nodes []string `json:"nodes,omitempty"`
}
type DiagramSymbol struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	Line int    `json:"line"`
}
type DiagramDiagnostic struct {
	Message  string `json:"message"`
	Severity int    `json:"severity,omitempty"`
	Line     int    `json:"line"`
}
type Graph struct {
	Kind      string       `json:"kind,omitempty"`
	Nodes     []GraphNode  `json:"nodes"`
	Edges     []GraphEdge  `json:"edges"`
	Groups    []GraphGroup `json:"groups"`
	NodeCount int          `json:"node_count"`
	EdgeCount int          `json:"edge_count"`
	Truncated bool         `json:"truncated,omitempty"`
}
type IntelligenceResult struct {
	Index       int                 `json:"index"`
	Status      string              `json:"status"`
	Complete    bool                `json:"complete"`
	Graph       Graph               `json:"graph"`
	Symbols     []DiagramSymbol     `json:"symbols"`
	Diagnostics []DiagramDiagnostic `json:"diagnostics"`
	Detail      string              `json:"detail,omitempty"`
}
type IntelligenceResponse struct {
	Provider          string               `json:"provider"`
	Results           []IntelligenceResult `json:"results"`
	DiagnosticsStatus string               `json:"diagnostics_status"`
	Detail            string               `json:"detail,omitempty"`
}
