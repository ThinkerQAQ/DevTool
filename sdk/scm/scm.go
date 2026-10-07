package scm

import (
	"encoding/json"

	credentialcontract "github.com/thinkerqaq/devtool/sdk/credential"
)

const ServiceName = "scm"

const (
	MethodDoctor     = "doctor"
	MethodStatus     = "status"
	MethodCommit     = "commit"
	MethodPush       = "push"
	MethodCheckpoint = "checkpoint"
	MethodPublish    = "publish"
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

type CheckpointRequest struct {
	Root    string `json:"root"`
	Message string `json:"message"`
}

type PublishRequest struct {
	Root  string `json:"root"`
	Base  string `json:"base,omitempty"`
	Title string `json:"title,omitempty"`
	Body  string `json:"body,omitempty"`
	Merge bool   `json:"merge,omitempty"`
}

type DoctorResponse struct {
	Provider      string                            `json:"provider"`
	Ready         bool                              `json:"ready"`
	Remote        string                            `json:"remote,omitempty"`
	Reason        string                            `json:"reason,omitempty"`
	Authorization *credentialcontract.Authorization `json:"authorization,omitempty"`
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
	Provider      string                            `json:"provider"`
	Branch        string                            `json:"branch"`
	Commit        string                            `json:"commit"`
	Authorization *credentialcontract.Authorization `json:"authorization,omitempty"`
}

type CheckpointResponse struct {
	Provider      string                            `json:"provider"`
	Branch        string                            `json:"branch"`
	Commit        string                            `json:"commit"`
	Committed     bool                              `json:"committed"`
	Authorization *credentialcontract.Authorization `json:"authorization,omitempty"`
}

type PublishResponse struct {
	Provider      string                            `json:"provider"`
	Branch        string                            `json:"branch"`
	Commit        string                            `json:"commit"`
	PRNumber      int                               `json:"pr_number,omitempty"`
	PRURL         string                            `json:"pr_url,omitempty"`
	Merged        bool                              `json:"merged"`
	DurationM     int64                             `json:"duration_ms"`
	Authorization *credentialcontract.Authorization `json:"authorization,omitempty"`
}

type ToolArguments struct {
	json.RawMessage
}
