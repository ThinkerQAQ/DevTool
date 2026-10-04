package extension

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/thinkerqaq/devtool/core/contract"
	"github.com/thinkerqaq/devtool/core/project"
	"github.com/thinkerqaq/devtool/core/registry"
	"github.com/thinkerqaq/devtool/protocol"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

type ProjectProcess struct {
	project project.Project
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	session *protocol.Session

	closeOnce sync.Once
	closeErr  error
}

type describePayload struct {
	Extension extensioncontract.Descriptor `json:"extension"`
	Project   contract.ProjectDescriptor   `json:"project"`
}

func StartProjectProcess(ctx context.Context, p project.Project, executable string, services *registry.Registry) (*ProjectProcess, error) {
	executable = strings.TrimSpace(executable)
	if executable == "" {
		return nil, fmt.Errorf("project extension executable is required")
	}

	cmd := exec.CommandContext(ctx, executable)
	cmd.Dir = p.Root
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

	process := &ProjectProcess{
		project: p,
		cmd:     cmd,
		stdin:   stdin,
	}
	process.session = protocol.NewSession(stdout, stdin, func(callCtx context.Context, envelope protocol.Envelope) (any, error) {
		if envelope.Method != protocol.MethodServiceInvoke {
			return nil, fmt.Errorf("unsupported host rpc method %q", envelope.Method)
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
	})
	return process, nil
}

func (p *ProjectProcess) Describe(ctx context.Context) (extensioncontract.Descriptor, contract.ProjectDescriptor, error) {
	var payload describePayload
	if err := p.call(ctx, protocol.MethodDescribe, nil, io.Discard, &payload); err != nil {
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

func (p *ProjectProcess) Execute(ctx context.Context, command string, args map[string]any, out io.Writer) error {
	request := protocol.ExecuteRequest{Command: command, Args: args}
	return p.call(ctx, protocol.MethodExecute, request, out, nil)
}

func (p *ProjectProcess) call(ctx context.Context, method string, payload any, out io.Writer, result any) error {
	if p == nil || p.session == nil {
		return fmt.Errorf("project extension process is unavailable")
	}
	if err := p.session.Call(ctx, method, payload, func(event protocol.Event) {
		if out != nil {
			fmt.Fprintln(out, event.Message)
		}
	}, result); err != nil {
		return fmt.Errorf("project extension: %w", err)
	}
	return nil
}

func (p *ProjectProcess) Close() error {
	if p == nil {
		return nil
	}
	p.closeOnce.Do(func() {
		if p.stdin != nil {
			_ = p.stdin.Close()
		}
		var sessionErr error
		if p.session != nil {
			sessionErr = p.session.Wait()
		}
		var processErr error
		if p.cmd != nil {
			processErr = p.cmd.Wait()
		}
		switch {
		case sessionErr != nil && sessionErr != io.EOF:
			p.closeErr = fmt.Errorf("project extension protocol: %w", sessionErr)
		case processErr != nil:
			p.closeErr = fmt.Errorf("project extension process: %w", processErr)
		}
	})
	return p.closeErr
}
