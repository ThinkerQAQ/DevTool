package protocol

import (
	"encoding/json"

	devtooltrace "github.com/thinkerqaq/devtool/sdk/trace"
)

type MessageType string

const (
	MessageRequest  MessageType = "request"
	MessageResponse MessageType = "response"
	MessageEvent    MessageType = "event"
	MessageError    MessageType = "error"
	MessageCancel   MessageType = "cancel"
)

type Envelope struct {
	ID      string                `json:"id,omitempty"`
	ReplyTo string                `json:"reply_to,omitempty"`
	Type    MessageType           `json:"type"`
	Method  string                `json:"method,omitempty"`
	Payload json.RawMessage       `json:"payload,omitempty"`
	Trace   *devtooltrace.Carrier `json:"trace,omitempty"`
}

const (
	MethodDescribe      = "provider.describe"
	MethodExecute       = "provider.execute"
	MethodServiceInvoke = "service.invoke"
)

type ExecuteRequest struct {
	Command string         `json:"command"`
	Args    map[string]any `json:"args,omitempty"`
}

type ServiceInvokeRequest struct {
	Service string          `json:"service"`
	Method  string          `json:"method"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type Event struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}
