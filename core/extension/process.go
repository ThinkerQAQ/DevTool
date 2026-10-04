package extension

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/thinkerqaq/devtool/core/contract"
	"github.com/thinkerqaq/devtool/core/project"
	"github.com/thinkerqaq/devtool/core/registry"
	"github.com/thinkerqaq/devtool/protocol"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

type ProjectProcess struct {
	Project    project.Project
	Executable string
	Services   *registry.Registry
}

type describePayload struct {
	Extension extensioncontract.Descriptor `json:"extension"`
	Project   contract.ProjectDescriptor   `json:"project"`
}

func (p ProjectProcess) Describe(ctx context.Context) (extensioncontract.Descriptor, contract.ProjectDescriptor, error) {
	var payload describePayload
	if err := p.invoke(ctx, protocol.MethodDescribe, nil, io.Discard, &payload); err != nil {
		return extensioncontract.Descriptor{}, contract.ProjectDescriptor{}, err
	}
	if payload.Extension.Kind != extensioncontract.KindProject {
		return extensioncontract.Descriptor{}, contract.ProjectDescriptor{}, fmt.Errorf("extension %q has kind %q; expected %q", payload.Extension.ID, payload.Extension.Kind, extensioncontract.KindProject)
	}
	if err := contract.ValidateProjectDescriptor(payload.Project); err != nil {
		return extensioncontract.Descriptor{}, contract.ProjectDescriptor{}, fmt.Errorf("validate project extension descriptor: %w", err)
	}
	return payload.Extension, payload.Project, nil
}

func (p ProjectProcess) Execute(ctx context.Context, command string, args map[string]any, out io.Writer) error {
	request := protocol.ExecuteRequest{Command: command, Args: args}
	return p.invoke(ctx, protocol.MethodExecute, request, out, nil)
}

func (p ProjectProcess) invoke(ctx context.Context, method string, payload any, out io.Writer, result any) error {
	executable := strings.TrimSpace(p.Executable)
	if executable == "" {
		return fmt.Errorf("project extension executable is required")
	}
	cmd := exec.CommandContext(ctx, executable)
	cmd.Dir = p.Project.Root
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}

	session := protocol.NewSession(stdout, stdin, func(callCtx context.Context, envelope protocol.Envelope) (any, error) {
		if envelope.Method != protocol.MethodServiceInvoke {
			return nil, fmt.Errorf("unsupported host rpc method %q", envelope.Method)
		}
		if p.Services == nil {
			return nil, fmt.Errorf("host services are unavailable")
		}
		var request protocol.ServiceInvokeRequest
		if err := json.Unmarshal(envelope.Payload, &request); err != nil {
			return nil, err
		}
		invoker, ok := p.Services.Service(request.Service)
		if !ok {
			return nil, fmt.Errorf("service %q is not registered", request.Service)
		}
		return invoker.Invoke(callCtx, request.Method, request.Payload)
	})

	callErr := session.Call(ctx, method, payload, func(event protocol.Event) {
		if out != nil {
			fmt.Fprintln(out, event.Message)
		}
	}, result)

	_ = stdin.Close()
	sessionErr := session.Wait()
	processErr := cmd.Wait()

	if callErr != nil {
		return fmt.Errorf("project extension: %w", callErr)
	}
	if sessionErr != nil {
		return fmt.Errorf("project extension protocol: %w", sessionErr)
	}
	if processErr != nil {
		return fmt.Errorf("project extension process: %w", processErr)
	}
	return nil
}
