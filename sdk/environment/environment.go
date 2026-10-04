package environment

import "encoding/json"

const (
	ServiceName   = "environment"
	MethodCommand = "command"
)

type CommandRequest struct {
	Root       string   `json:"root"`
	Image      string   `json:"image"`
	WorkingDir string   `json:"working_dir,omitempty"`
	Executable string   `json:"executable"`
	Args       []string `json:"args,omitempty"`
}

type CommandSpec struct {
	Program string   `json:"program"`
	Args    []string `json:"args,omitempty"`
	Dir     string   `json:"dir,omitempty"`
	Env     []string `json:"env,omitempty"`
}

func Encode(value any) (json.RawMessage, error) {
	return json.Marshal(value)
}
