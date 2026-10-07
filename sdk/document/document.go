package document

const (
	ServiceName   = "document-structure"
	MethodInspect = "inspect"
)

type InspectRequest struct {
	Root             string `json:"root"`
	Path             string `json:"path"`
	Section          string `json:"section,omitempty"`
	SectionStartLine int    `json:"section_start_line,omitempty"`
	IncludeContent   bool   `json:"include_content,omitempty"`
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

type InspectResponse struct {
	Path            string           `json:"path"`
	Format          string           `json:"format"`
	LineCount       int              `json:"line_count"`
	Frontmatter     map[string]any   `json:"frontmatter,omitempty"`
	Outline         []Section        `json:"outline,omitempty"`
	SelectedSection *SelectedSection `json:"selected_section,omitempty"`
}
