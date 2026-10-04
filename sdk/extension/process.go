package extension

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/thinkerqaq/devtool/protocol"
	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
)

type ProcessProvider interface {
	Descriptor() Descriptor
	InvokeService(context.Context, string, string, json.RawMessage) (json.RawMessage, error)
}

func ServeProcess(provider ProcessProvider) error {
	return serveProcess(provider, os.Stdin, os.Stdout)
}

func serveProcess(provider ProcessProvider, in io.Reader, out io.Writer) error {
	session := protocol.NewSession(in, out, func(ctx context.Context, envelope protocol.Envelope) (any, error) {
		switch envelope.Method {
		case protocol.MethodExtensionDescribe:
			raw, err := json.Marshal(provider.Descriptor())
			if err != nil {
				return nil, err
			}
			return protocol.ExtensionDescribeResponse{Extension: raw}, nil
		case protocol.MethodExtensionInvoke:
			var request protocol.ExtensionInvokeRequest
			if err := json.Unmarshal(envelope.Payload, &request); err != nil {
				return nil, err
			}
			return provider.InvokeService(ctx, request.Service, request.Method, request.Payload)
		case protocol.MethodAgentToolsList:
			toolProvider, ok := provider.(agentsdk.ToolProvider)
			if !ok {
				return nil, fmt.Errorf("extension %q does not expose agent tools", provider.Descriptor().ID)
			}
			var request protocol.AgentToolsListRequest
			if err := json.Unmarshal(envelope.Payload, &request); err != nil {
				return nil, err
			}
			var agentSession agentsdk.Session
			if err := json.Unmarshal(request.Session, &agentSession); err != nil {
				return nil, err
			}
			tools, err := toolProvider.ListTools(ctx, agentSession)
			if err != nil {
				return nil, err
			}
			definitions := make([]json.RawMessage, 0, len(tools))
			for _, tool := range tools {
				definitions = append(definitions, append(json.RawMessage(nil), tool.Definition...))
			}
			return protocol.AgentToolsListResponse{Tools: definitions}, nil
		case protocol.MethodAgentToolCall:
			toolProvider, ok := provider.(agentsdk.ToolProvider)
			if !ok {
				return nil, fmt.Errorf("extension %q does not expose agent tools", provider.Descriptor().ID)
			}
			var request protocol.AgentToolCallRequest
			if err := json.Unmarshal(envelope.Payload, &request); err != nil {
				return nil, err
			}
			var agentSession agentsdk.Session
			if err := json.Unmarshal(request.Session, &agentSession); err != nil {
				return nil, err
			}
			return toolProvider.CallTool(ctx, agentSession, request.Name, request.Arguments)
		default:
			return nil, fmt.Errorf("unknown extension provider method %q", envelope.Method)
		}
	})
	return session.Wait()
}
