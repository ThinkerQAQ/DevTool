package protocol

import "encoding/json"

type MessageType string

const (
	MessageRequest  MessageType = "request"
	MessageResponse MessageType = "response"
	MessageEvent    MessageType = "event"
	MessageError    MessageType = "error"
)

type Envelope struct {
	ID      string          `json:"id,omitempty"`
	ReplyTo string          `json:"reply_to,omitempty"`
	Type    MessageType     `json:"type"`
	Method  string          `json:"method,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

const (
	MethodDescribe = "provider.describe"
	MethodExecute  = "provider.execute"
)

type ExecuteRequest struct {
	Command string         `json:"command"`
	Args    map[string]any `json:"args,omitempty"`
}

type Event struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}
