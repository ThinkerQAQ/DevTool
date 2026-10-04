package extension

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/thinkerqaq/devtool/core/project"
	"github.com/thinkerqaq/devtool/core/registry"
	"github.com/thinkerqaq/devtool/core/service"
	"github.com/thinkerqaq/devtool/protocol"
	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

type ProcessExtension struct {
	descriptor extensioncontract.Descriptor
	client     *processExtensionClient
}

type processExtensionClient struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	session *protocol.Session
}

func LoadProcessExtension(ctx context.Context, p project.Project, name, executable string, services *registry.Registry) (*ProcessExtension, error) {
	executable = strings.TrimSpace(executable)
	if executable == "" {
		return nil, fmt.Errorf("extension %q executable is required", name)
	}
	client, err := startProcessExtensionClient(ctx, p.Root, executable, services)
	if err != nil {
		return nil, fmt.Errorf("start extension %q: %w", name, err)
	}

	var response protocol.ExtensionDescribeResponse
	if err := client.call(ctx, protocol.MethodExtensionDescribe, nil, &response); err != nil {
		client.close()
		return nil, fmt.Errorf("describe extension %q: %w", name, err)
	}
	var descriptor extensioncontract.Descriptor
	if err := json.Unmarshal(response.Extension, &descriptor); err != nil {
		client.close()
		return nil, fmt.Errorf("decode extension %q descriptor: %w", name, err)
	}
	if strings.TrimSpace(descriptor.ID) == "" {
		client.close()
		return nil, fmt.Errorf("extension %q returned empty id", name)
	}
	if descriptor.Kind == extensioncontract.KindProject {
		client.close()
		return nil, fmt.Errorf("extension %q returned project kind; project extension is loaded separately", descriptor.ID)
	}
	return &ProcessExtension{
		descriptor: descriptor,
		client:     client,
	}, nil
}

func (p *ProcessExtension) Descriptor() extensioncontract.Descriptor {
	return p.descriptor
}

func (p *ProcessExtension) Close() error {
	if p.client == nil {
		return nil
	}
	p.client.close()
	p.client = nil
	return nil
}

func (p *ProcessExtension) Register(reg extensioncontract.Registrar) error {
	for _, configuredService := range p.descriptor.Provides {
		serviceName := strings.TrimSpace(configuredService)
		if serviceName == "" {
			return fmt.Errorf("extension %q declares an empty service", p.descriptor.ID)
		}
		name := serviceName
		if err := reg.ProvideService(name, p.descriptor.ID, service.Func(func(ctx context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
			return p.invokeService(ctx, name, method, payload)
		})); err != nil {
			return err
		}
	}
	if p.descriptor.AgentTools {
		if err := reg.ProvideAgentTools(p.descriptor.ID, p); err != nil {
			return err
		}
	}
	return nil
}

func (p *ProcessExtension) invokeService(ctx context.Context, serviceName, method string, payload json.RawMessage) (json.RawMessage, error) {
	request := protocol.ExtensionInvokeRequest{Service: serviceName, Method: method, Payload: payload}
	var result json.RawMessage
	if err := p.client.call(ctx, protocol.MethodExtensionInvoke, request, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (p *ProcessExtension) ListTools(ctx context.Context, session agentsdk.Session) ([]agentsdk.Tool, error) {
	rawSession, err := json.Marshal(session)
	if err != nil {
		return nil, err
	}
	var response protocol.AgentToolsListResponse
	if err := p.client.call(ctx, protocol.MethodAgentToolsList, protocol.AgentToolsListRequest{Session: rawSession}, &response); err != nil {
		return nil, err
	}
	tools := make([]agentsdk.Tool, 0, len(response.Tools))
	for _, definition := range response.Tools {
		var metadata struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(definition, &metadata); err != nil {
			return nil, fmt.Errorf("decode agent tool from extension %q: %w", p.descriptor.ID, err)
		}
		if strings.TrimSpace(metadata.Name) == "" {
			return nil, fmt.Errorf("extension %q exposed an unnamed agent tool", p.descriptor.ID)
		}
		tools = append(tools, agentsdk.Tool{Name: metadata.Name, Definition: append(json.RawMessage(nil), definition...)})
	}
	return tools, nil
}

func (p *ProcessExtension) CallTool(ctx context.Context, session agentsdk.Session, name string, args json.RawMessage) (json.RawMessage, error) {
	rawSession, err := json.Marshal(session)
	if err != nil {
		return nil, err
	}
	request := protocol.AgentToolCallRequest{
		Session:   rawSession,
		Name:      name,
		Arguments: args,
	}
	var result json.RawMessage
	if err := p.client.call(ctx, protocol.MethodAgentToolCall, request, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func startProcessExtensionClient(ctx context.Context, root, executable string, services *registry.Registry) (*processExtensionClient, error) {
	cmd := exec.CommandContext(ctx, executable)
	cmd.Dir = root
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil, err
	}
	return &processExtensionClient{
		cmd:   cmd,
		stdin: stdin,
		session: protocol.NewSession(stdout, stdin, func(callCtx context.Context, envelope protocol.Envelope) (any, error) {
			if envelope.Method != protocol.MethodServiceInvoke {
				return nil, fmt.Errorf("unsupported extension callback method %q", envelope.Method)
			}
			if services == nil {
				return nil, fmt.Errorf("host services are unavailable")
			}
			var request protocol.ServiceInvokeRequest
			if err := json.Unmarshal(envelope.Payload, &request); err != nil {
				return nil, err
			}
			invoker, ok := services.Service(request.Service)
			if !ok {
				return nil, fmt.Errorf("service %q is not registered", request.Service)
			}
			return invoker.Invoke(callCtx, request.Method, request.Payload)
		}),
	}, nil
}

func (c *processExtensionClient) call(ctx context.Context, method string, payload any, result any) error {
	if c == nil || c.session == nil {
		return fmt.Errorf("extension process is unavailable")
	}
	if err := c.session.Call(ctx, method, payload, nil, result); err != nil {
		return fmt.Errorf("extension process: %w", err)
	}
	return nil
}

func (c *processExtensionClient) close() {
	if c == nil {
		return
	}
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
		_ = c.cmd.Wait()
	}
}
