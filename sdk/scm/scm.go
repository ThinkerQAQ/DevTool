package scm

import "encoding/json"

const ServiceName = "scm"

const (
	MethodDoctor  = "doctor"
	MethodStatus  = "status"
	MethodCommit  = "commit"
	MethodPush    = "push"
	MethodPublish = "publish"
)

type Request struct {
	Root string `json:"root"`
}

type CommitRequest struct {
	Root    string `json:"root"`
	Message string `json:"message"`
}

type PushRequest struct {
	Root string `json:"root"`
}

type PublishRequest struct {
	Root  string `json:"root"`
	Base  string `json:"base,omitempty"`
	Title string `json:"title,omitempty"`
	Body  string `json:"body,omitempty"`
	Merge bool   `json:"merge,omitempty"`
}

type DoctorResponse struct {
	Provider string `json:"provider"`
	Ready    bool   `json:"ready"`
	Remote   string `json:"remote,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

type StatusResponse struct {
	Provider string `json:"provider"`
	Branch   string `json:"branch"`
	Head     string `json:"head"`
	Clean    bool   `json:"clean"`
	Changes  string `json:"changes,omitempty"`
	Ahead    int    `json:"ahead,omitempty"`
	Behind   int    `json:"behind,omitempty"`
}

type CommitResponse struct {
	Provider string `json:"provider"`
	Branch   string `json:"branch"`
	Commit   string `json:"commit"`
}

type PushResponse struct {
	Provider string `json:"provider"`
	Branch   string `json:"branch"`
	Commit   string `json:"commit"`
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
