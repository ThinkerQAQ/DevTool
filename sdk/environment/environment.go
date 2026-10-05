package environment

import "encoding/json"

const (
	ServiceName   = "environment"
	MethodCommand = "command"
	MethodRun     = "run"
	WorkspaceRoot = "/workspace"
)

type CommandRequest struct {
	Root       string   `json:"root"`
	WorkingDir string   `json:"working_dir,omitempty"`
	Executable string   `json:"executable"`
	Args       []string `json:"args,omitempty"`
	Env        []string `json:"env,omitempty"`
}

type CommandSpec struct {
	Program string   `json:"program"`
	Args    []string `json:"args,omitempty"`
	Dir     string   `json:"dir,omitempty"`
	Env     []string `json:"env,omitempty"`
}

type RunResult struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
}

func Encode(value any) (json.RawMessage, error) {
	return json.Marshal(value)
}
