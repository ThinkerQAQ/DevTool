package extension

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"

	"github.com/thinkerqaq/devtool/core/project"
	"github.com/thinkerqaq/devtool/core/registry"
	"github.com/thinkerqaq/devtool/core/service"
	"github.com/thinkerqaq/devtool/protocol"
	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

type ProcessExtension struct {
	project    project.Project
	executable string
	descriptor extensioncontract.Descriptor
	client     *processExtensionClient
}

type processExtensionClient struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	session *protocol.Session
}

func LoadProcessExtension(ctx context.Context, p project.Project, name, module, pkg string, services *registry.Registry) (*ProcessExtension, error) {
	executable, err := buildProcessExtension(ctx, p, name, module, pkg)
	if err != nil {
		return nil, err
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
		project:    p,
		executable: executable,
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
		cmd:     cmd,
		stdin:   stdin,
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

func buildProcessExtension(ctx context.Context, p project.Project, name, module, pkg string) (string, error) {
	module = strings.TrimSpace(module)
	pkg = strings.TrimSpace(pkg)
	if module == "" {
		return "", fmt.Errorf("extension.%s.module is required", name)
	}
	if pkg == "" {
		return "", fmt.Errorf("extension.%s.package is required", name)
	}
	moduleDir := filepath.Join(p.Root, filepath.FromSlash(module))
	info, err := os.Stat(moduleDir)
	if err != nil {
		return "", fmt.Errorf("extension %q module %q: %w", name, module, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("extension %q module %q is not a directory", name, module)
	}
	cacheDir := filepath.Join(p.Root, ".devtool", "cache", "extensions")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", err
	}
	outputName := sanitizeExtensionName(name)
	if runtime.GOOS == "windows" {
		outputName += ".exe"
	}
	output := filepath.Join(cacheDir, outputName)
	rebuild, err := processExtensionNeedsBuild(moduleDir, output)
	if err != nil {
		return "", err
	}
	if !rebuild {
		return output, nil
	}
	cmd := exec.CommandContext(ctx, "go", "build", "-o", output, pkg)
	cmd.Dir = moduleDir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("build extension %q: %w", name, err)
	}
	return output, nil
}

func processExtensionNeedsBuild(moduleDir, output string) (bool, error) {
	outputInfo, err := os.Stat(output)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, err
	}
	outputTime := outputInfo.ModTime()
	rebuild := false
	err = filepath.WalkDir(moduleDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", ".devtool", "node_modules":
				if path != moduleDir {
					return fs.SkipDir
				}
			}
			return nil
		}
		name := entry.Name()
		if filepath.Ext(name) != ".go" && name != "go.mod" && name != "go.sum" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.ModTime().After(outputTime.Add(time.Millisecond)) {
			rebuild = true
			return fs.SkipAll
		}
		return nil
	})
	if err != nil && err != fs.SkipAll {
		return false, err
	}
	return rebuild, nil
}

func sanitizeExtensionName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "extension-provider"
	}
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, name)
}
