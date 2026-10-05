package contract

type SideEffect string

const (
	SideEffectRead        SideEffect = "read"
	SideEffectWrite       SideEffect = "write"
	SideEffectDeploy      SideEffect = "deploy"
	SideEffectDestructive SideEffect = "destructive"
)

type FieldType string

const (
	FieldString      FieldType = "string"
	FieldInteger     FieldType = "integer"
	FieldNumber      FieldType = "number"
	FieldBoolean     FieldType = "boolean"
	FieldSelect      FieldType = "select"
	FieldMultiSelect FieldType = "multi-select"
	FieldSecret      FieldType = "secret"
	FieldFile        FieldType = "file"
	FieldDirectory   FieldType = "directory"
	FieldDate        FieldType = "date"
	FieldTime        FieldType = "time"
	FieldDateTime    FieldType = "datetime"
	FieldDevice      FieldType = "device"
)

type FieldDescriptor struct {
	Key         string    `json:"key"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Type        FieldType `json:"type"`
	Required    bool      `json:"required,omitempty"`
	Options     []string  `json:"options,omitempty"`
}

type CommandDescriptor struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Description string            `json:"description,omitempty"`
	Parameters  []FieldDescriptor `json:"parameters,omitempty"`
	SideEffect  SideEffect        `json:"side_effect"`
}

type ResourceDescriptor struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Description string            `json:"description,omitempty"`
	Fields      []FieldDescriptor `json:"fields,omitempty"`
}

type ActionDescriptor struct {
	CommandID string `json:"command_id"`
	Label     string `json:"label,omitempty"`
}

type ViewDescriptor struct {
	ID        string             `json:"id"`
	Title     string             `json:"title"`
	Resources []string           `json:"resources,omitempty"`
	Actions   []ActionDescriptor `json:"actions,omitempty"`
}

type FeatureBinding struct {
	ID        string         `json:"id"`
	FeatureID string         `json:"feature_id"`
	Resources []string       `json:"resources,omitempty"`
	Options   map[string]any `json:"options,omitempty"`
}

type NavigationItem struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	ViewID    string `json:"view_id,omitempty"`
	FeatureID string `json:"feature_id,omitempty"`
}

type ProjectIdentity struct {
	Name string `json:"name"`
}

type ProjectDescriptor struct {
	Identity   ProjectIdentity      `json:"identity"`
	Commands   []CommandDescriptor  `json:"commands,omitempty"`
	Resources  []ResourceDescriptor `json:"resources,omitempty"`
	Views      []ViewDescriptor     `json:"views,omitempty"`
	Features   []FeatureBinding     `json:"features,omitempty"`
	Navigation []NavigationItem     `json:"navigation,omitempty"`
}
