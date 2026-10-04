package protocol

import "encoding/json"

const (
	MethodExtensionDescribe = "extension.describe"
	MethodExtensionInvoke   = "extension.invoke"
	MethodAgentToolsList    = "agent.tools.list"
	MethodAgentToolCall     = "agent.tools.call"
)

type ExtensionDescribeResponse struct {
	Extension json.RawMessage `json:"extension"`
}

type ExtensionInvokeRequest struct {
	Service string          `json:"service"`
	Method  string          `json:"method"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type AgentToolsListRequest struct {
	Session json.RawMessage `json:"session"`
}

type AgentToolsListResponse struct {
	Tools []json.RawMessage `json:"tools"`
}

type AgentToolCallRequest struct {
	Session   json.RawMessage `json:"session"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}
