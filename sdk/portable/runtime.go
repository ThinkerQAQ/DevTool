package portable

import "encoding/json"

const ServiceName = "portable-runtime"

const (
	MethodDoctor = "doctor"
	MethodInvoke = "invoke"
)

type DoctorResponse struct {
	Provider string `json:"provider"`
	Version  string `json:"version"`
	Smoke    string `json:"smoke"`
}

type Invocation struct {
	Workspace string            `json:"workspace,omitempty"`
	Module    string            `json:"module,omitempty"`
	Function  string            `json:"function"`
	Args      map[string]string `json:"args,omitempty"`
}

type Result struct {
	Value json.RawMessage `json:"value,omitempty"`
}
