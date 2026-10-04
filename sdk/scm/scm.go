package scm

import "encoding/json"

const ServiceName = "scm-publish"

const (
	MethodDoctor  = "doctor"
	MethodPublish = "publish"
)

type Request struct {
	Root string `json:"root"`
}

type PublishRequest struct {
	Root          string `json:"root"`
	Base          string `json:"base,omitempty"`
	Title         string `json:"title,omitempty"`
	Body          string `json:"body,omitempty"`
	CommitMessage string `json:"commit_message,omitempty"`
	Merge         bool   `json:"merge,omitempty"`
}

type DoctorResponse struct {
	Provider string `json:"provider"`
	Ready    bool   `json:"ready"`
	Remote   string `json:"remote,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

type PublishResponse struct {
	Provider  string `json:"provider"`
	Branch    string `json:"branch"`
	Commit    string `json:"commit"`
	PRNumber  int    `json:"pr_number,omitempty"`
	PRURL     string `json:"pr_url,omitempty"`
	Merged    bool   `json:"merged"`
	DurationM int64  `json:"duration_ms"`
}

type ToolArguments struct {
	json.RawMessage
}
