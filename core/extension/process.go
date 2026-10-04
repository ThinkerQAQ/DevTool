package extension

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/thinkerqaq/devtool/core/contract"
	"github.com/thinkerqaq/devtool/core/project"
	"github.com/thinkerqaq/devtool/core/registry"
	"github.com/thinkerqaq/devtool/protocol"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

type ProjectProcess struct {
	Project  project.Project
	Module   string
	Package  string
	Services *registry.Registry
}

type describePayload struct {
	Extension extensioncontract.Descriptor `json:"extension"`
	Project   contract.ProjectDescriptor   `json:"project"`
}

func (p ProjectProcess) Describe(ctx context.Context) (extensioncontract.Descriptor, contract.ProjectDescriptor, error) {
	executable, err := p.build(ctx)
	if err != nil {
		return extensioncontract.Descriptor{}, contract.ProjectDescriptor{}, err
	}

	var payload describePayload
	if err := p.invoke(ctx, executable, protocol.MethodDescribe, nil, io.Discard, &payload); err != nil {
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
	executable, err := p.build(ctx)
	if err != nil {
		return err
	}
	request := protocol.ExecuteRequest{Command: command, Args: args}
	return p.invoke(ctx, executable, protocol.MethodExecute, request, out, nil)
}

func (p ProjectProcess) build(ctx context.Context) (string, error) {
	module := strings.TrimSpace(p.Module)
	pkg := strings.TrimSpace(p.Package)
	if module == "" {
		return "", fmt.Errorf("project extension module is required")
	}
	if pkg == "" {
		return "", fmt.Errorf("project extension package is required")
	}

	moduleDir := filepath.Join(p.Project.Root, filepath.FromSlash(module))
	info, err := os.Stat(moduleDir)
	if err != nil {
		return "", fmt.Errorf("project extension module %q: %w", module, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("project extension module %q is not a directory", module)
	}

	cacheDir := filepath.Join(p.Project.Root, ".devtool", "cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", err
	}
	name := "project-provider"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	output := filepath.Join(cacheDir, name)

	cmd := exec.CommandContext(ctx, "go", "build", "-o", output, pkg)
	cmd.Dir = moduleDir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("build project extension: %w", err)
	}
	return output, nil
}

func (p ProjectProcess) invoke(ctx context.Context, executable, method string, payload any, out io.Writer, result any) error {
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

	// Closing stdin tells the provider there will be no more requests. Drain the
	// protocol stream before cmd.Wait closes the stdout pipe owned by os/exec.
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
