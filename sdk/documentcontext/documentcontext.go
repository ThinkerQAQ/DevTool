package documentcontext

const (
	ServiceName = "document-context"
	MethodBuild = "build"
)

type BuildRequest struct {
	Root           string `json:"root"`
	Objective      string `json:"objective"`
	Path           string `json:"path"`
	Section        string `json:"section,omitempty"`
	IncludeContent bool   `json:"include_content,omitempty"`
	IncludeTables  bool   `json:"include_tables,omitempty"`
	References     bool   `json:"references,omitempty"`
	Review         bool   `json:"review,omitempty"`
	Cursor         string `json:"cursor,omitempty"`
	ReviewMaxLines int    `json:"review_max_lines,omitempty"`
	Related        bool   `json:"related,omitempty"`
	RelationDepth  int    `json:"relation_depth,omitempty"`
	RelationLimit  int    `json:"relation_limit,omitempty"`
}
