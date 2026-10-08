package document

const (
	ServiceName   = "document-structure"
	MethodInspect = "inspect"

	RelationsServiceName   = "document-relations"
	MethodResolveRelations = "resolve"
)

type InspectRequest struct {
	Root             string `json:"root"`
	Path             string `json:"path"`
	Section          string `json:"section,omitempty"`
	SectionStartLine int    `json:"section_start_line,omitempty"`
	RangeStartLine   int    `json:"range_start_line,omitempty"`
	RangeEndLine     int    `json:"range_end_line,omitempty"`
	IncludeContent   bool   `json:"include_content,omitempty"`
	IncludeDiagrams  bool   `json:"include_diagrams,omitempty"`
	IncludeTables    bool   `json:"include_tables,omitempty"`
}

type Section struct {
	Key       string    `json:"key,omitempty"`
	Title     string    `json:"title"`
	Level     int       `json:"level"`
	StartLine int       `json:"start_line"`
	EndLine   int       `json:"end_line"`
	Children  []Section `json:"children,omitempty"`
}

type SelectedSection struct {
	Key       string `json:"key,omitempty"`
	Title     string `json:"title"`
	Level     int    `json:"level"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Content   string `json:"content,omitempty"`
}

type SelectedRange struct {
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Content   string `json:"content,omitempty"`
}

type Diagram struct {
	Index     int    `json:"index"`
	Language  string `json:"language"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Source    string `json:"source"`
}

// Table represents a GFM table with bounded, source-anchored structured cells.
type Table struct {
	Index      int        `json:"index"`
	StartLine  int        `json:"start_line"`
	EndLine    int        `json:"end_line"`
	Columns    int        `json:"columns"`
	Headers    []string   `json:"headers"`
	Rows       [][]string `json:"rows"`
	Alignments []string   `json:"alignments,omitempty"`
}

type InspectResponse struct {
	Path            string           `json:"path"`
	Format          string           `json:"format"`
	LineCount       int              `json:"line_count"`
	Frontmatter     map[string]any   `json:"frontmatter,omitempty"`
	Outline         []Section        `json:"outline,omitempty"`
	Diagrams        []Diagram        `json:"diagrams,omitempty"`
	Tables          []Table          `json:"tables,omitempty"`
	SelectedSection *SelectedSection `json:"selected_section,omitempty"`
	SelectedRange   *SelectedRange   `json:"selected_range,omitempty"`
}

type RelationsRequest struct {
	Root     string `json:"root"`
	Path     string `json:"path"`
	Depth    int    `json:"depth,omitempty"`
	MaxNodes int    `json:"max_nodes,omitempty"`
}

type RelationNode struct {
	Key      string `json:"key"`
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Path     string `json:"path"`
	Title    string `json:"title,omitempty"`
	Status   string `json:"status,omitempty"`
	Language string `json:"language,omitempty"`
}

type RelationEdge struct {
	Type   string `json:"type"`
	From   string `json:"from"`
	To     string `json:"to"`
	Order  *int   `json:"order,omitempty"`
	Label  string `json:"label,omitempty"`
	Source string `json:"source,omitempty"`
}

type RelationsResponse struct {
	RootNode RelationNode   `json:"root_node"`
	Nodes    []RelationNode `json:"nodes,omitempty"`
	Edges    []RelationEdge `json:"edges,omitempty"`
	Warnings []string       `json:"warnings,omitempty"`
}
