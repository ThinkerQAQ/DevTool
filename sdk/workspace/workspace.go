package workspace

const ServiceName = "workspace"

const (
	MethodCreate  = "create"
	MethodList    = "list"
	MethodInspect = "inspect"
	MethodRemove  = "remove"
)

type Identity struct {
	RepositoryID string `json:"repository_id"`
	WorkspaceID  string `json:"workspace_id"`
	Root         string `json:"root"`
}

type Descriptor struct {
	Provider string   `json:"provider"`
	Name     string   `json:"name"`
	Identity Identity `json:"identity"`
	Revision string   `json:"revision,omitempty"`
	Primary  bool     `json:"primary,omitempty"`
}

type CreateRequest struct {
	Root     string `json:"root"`
	Name     string `json:"name"`
	Revision string `json:"revision,omitempty"`
}

type CreateResponse struct {
	Workspace Descriptor `json:"workspace"`
}

type ListRequest struct {
	Root string `json:"root"`
}

type ListResponse struct {
	Workspaces []Descriptor `json:"workspaces"`
}

type InspectRequest struct {
	Root        string `json:"root"`
	WorkspaceID string `json:"workspace_id"`
}

type InspectResponse struct {
	Workspace Descriptor `json:"workspace"`
}

type RemoveRequest struct {
	Root        string `json:"root"`
	WorkspaceID string `json:"workspace_id"`
}

type RemoveResponse struct {
	WorkspaceID string `json:"workspace_id"`
	Removed     bool   `json:"removed"`
}
