package diagram

const (
	RenderServiceName  = "diagram-render"
	RenderMethod       = "render"
	ContextServiceName = "diagram-context"
	ContextMethod      = "build"
)

type ContextRequest struct {
	Root          string `json:"root"`
	Path          string `json:"path"`
	Objective     string `json:"objective"`
	Render        bool   `json:"render,omitempty"`
	IncludeSource bool   `json:"include_source,omitempty"`
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
