package extension

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/thinkerqaq/devtool/core/contract"
	"github.com/thinkerqaq/devtool/core/service"
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

// ServeExtension exposes a normal DevTool Extension through the process
// protocol. The process-side registrar captures services/tools provided by the
// extension and proxies required services back to the host over the same
// bidirectional protocol Session.
func ServeExtension(ext Extension) error {
	return serveExtension(ext, os.Stdin, os.Stdout)
}

func serveExtension(ext Extension, in io.Reader, out io.Writer) error {
	if ext == nil {
		return fmt.Errorf("extension is required")
	}

	var session *protocol.Session
	reg := newProcessRegistrar(func(ctx context.Context, name, method string, payload json.RawMessage) (json.RawMessage, error) {
		if session == nil {
			return nil, fmt.Errorf("extension host session is unavailable")
		}
		request := protocol.ServiceInvokeRequest{Service: name, Method: method, Payload: payload}
		var result json.RawMessage
		if err := session.Call(ctx, protocol.MethodServiceInvoke, request, nil, &result); err != nil {
			return nil, err
		}
		return result, nil
	})
	if err := ext.Register(reg); err != nil {
		return fmt.Errorf("register extension %q: %w", ext.Descriptor().ID, err)
	}
	if closer, ok := ext.(io.Closer); ok {
		defer closer.Close()
	}

	session = protocol.NewSession(in, out, func(ctx context.Context, envelope protocol.Envelope) (any, error) {
		switch envelope.Method {
		case protocol.MethodExtensionDescribe:
			raw, err := json.Marshal(ext.Descriptor())
			if err != nil {
				return nil, err
			}
			return protocol.ExtensionDescribeResponse{Extension: raw}, nil
		case protocol.MethodExtensionInvoke:
			var request protocol.ExtensionInvokeRequest
			if err := json.Unmarshal(envelope.Payload, &request); err != nil {
				return nil, err
			}
			invoker, ok := reg.provided[request.Service]
			if !ok {
				return nil, fmt.Errorf("extension %q does not provide service %q", ext.Descriptor().ID, request.Service)
			}
			return invoker.Invoke(ctx, request.Method, request.Payload)
		case protocol.MethodAgentToolsList:
			if reg.tools == nil {
				return nil, fmt.Errorf("extension %q does not expose agent tools", ext.Descriptor().ID)
			}
			var request protocol.AgentToolsListRequest
			if err := json.Unmarshal(envelope.Payload, &request); err != nil {
				return nil, err
			}
			var agentSession agentsdk.Session
			if err := json.Unmarshal(request.Session, &agentSession); err != nil {
				return nil, err
			}
			tools, err := reg.tools.ListTools(ctx, agentSession)
			if err != nil {
				return nil, err
			}
			definitions := make([]json.RawMessage, 0, len(tools))
			for _, tool := range tools {
				definitions = append(definitions, append(json.RawMessage(nil), tool.Definition...))
			}
			return protocol.AgentToolsListResponse{Tools: definitions}, nil
		case protocol.MethodAgentToolCall:
			if reg.tools == nil {
				return nil, fmt.Errorf("extension %q does not expose agent tools", ext.Descriptor().ID)
			}
			var request protocol.AgentToolCallRequest
			if err := json.Unmarshal(envelope.Payload, &request); err != nil {
				return nil, err
			}
			var agentSession agentsdk.Session
			if err := json.Unmarshal(request.Session, &agentSession); err != nil {
				return nil, err
			}
			return reg.tools.CallTool(ctx, agentSession, request.Name, request.Arguments)
		default:
			return nil, fmt.Errorf("unknown extension provider method %q", envelope.Method)
		}
	})
	return session.Wait()
}

type processRegistrar struct {
	provided map[string]service.Invoker
	tools    agentsdk.ToolProvider
	invoke   func(context.Context, string, string, json.RawMessage) (json.RawMessage, error)
}

func newProcessRegistrar(invoke func(context.Context, string, string, json.RawMessage) (json.RawMessage, error)) *processRegistrar {
	return &processRegistrar{
		provided: map[string]service.Invoker{},
		invoke:   invoke,
	}
}

func (r *processRegistrar) RegisterCommand(contract.CommandDescriptor) error {
	return fmt.Errorf("process capability extensions cannot register project commands")
}

func (r *processRegistrar) RegisterResource(contract.ResourceDescriptor) error {
	return fmt.Errorf("process capability extensions cannot register project resources")
}

func (r *processRegistrar) RegisterView(contract.ViewDescriptor) error {
	return fmt.Errorf("process capability extensions cannot register project views")
}

func (r *processRegistrar) RegisterFeature(contract.FeatureBinding) error {
	return fmt.Errorf("process capability extensions cannot register project feature bindings")
}

func (r *processRegistrar) RegisterNavigation(contract.NavigationItem) error {
	return fmt.Errorf("process capability extensions cannot register project navigation")
}

func (r *processRegistrar) ProvideService(name, extensionID string, value service.Invoker) error {
	if name == "" {
		return fmt.Errorf("service name is required")
	}
	if value == nil {
		return fmt.Errorf("service %q cannot register a nil implementation", name)
	}
	if _, exists := r.provided[name]; exists {
		return fmt.Errorf("service %q is already provided in extension process", name)
	}
	r.provided[name] = value
	return nil
}

func (r *processRegistrar) Service(name string) (service.Invoker, bool) {
	if value, ok := r.provided[name]; ok {
		return value, true
	}
	if r.invoke == nil {
		return nil, false
	}
	return service.Func(func(ctx context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
		return r.invoke(ctx, name, method, payload)
	}), true
}

func (r *processRegistrar) ProvideAgentTools(extensionID string, provider agentsdk.ToolProvider) error {
	if provider == nil {
		return fmt.Errorf("agent tool provider %q cannot be nil", extensionID)
	}
	if r.tools != nil {
		return fmt.Errorf("agent tools are already provided in extension process")
	}
	r.tools = provider
	return nil
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
