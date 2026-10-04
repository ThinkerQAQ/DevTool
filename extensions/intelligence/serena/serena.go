package serena

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/thinkerqaq/devtool/core/service"
	"github.com/thinkerqaq/devtool/extensions/intelligence/internal/devenv"
	"github.com/thinkerqaq/devtool/sdk/codeintelligence"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

const ExtensionID = "intelligence.lsp.serena"

type Extension struct {
	// executable exists only for isolated adapter tests. Production execution
	// always uses the project's pinned DevEnvironment image.
	executable string
}

func New() *Extension {
	return &Extension{}
}

func (e *Extension) Descriptor() extensioncontract.Descriptor {
	return extensioncontract.Descriptor{
		ID:       ExtensionID,
		Kind:     extensioncontract.KindCodeIntelligence,
		Provides: []string{codeintelligence.LSPServiceName},
	}
}

func (e *Extension) Register(reg extensioncontract.Registrar) error {
	return reg.ProvideService(codeintelligence.LSPServiceName, ExtensionID, service.Func(e.Invoke))
}

func (e *Extension) Invoke(ctx context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
	switch method {
	case codeintelligence.MethodDoctor:
		var workspace codeintelligence.Workspace
		if err := json.Unmarshal(payload, &workspace); err != nil {
			return nil, fmt.Errorf("decode LSP doctor request: %w", err)
		}
		return e.doctor(ctx, workspace)
	case codeintelligence.MethodVerify:
		var request codeintelligence.Workspace
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, fmt.Errorf("decode LSP verify request: %w", err)
		}
		return e.verify(ctx, request)
	case codeintelligence.MethodMCP:
		var request codeintelligence.MCPRequest
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, fmt.Errorf("decode LSP MCP request: %w", err)
		}
		if err := e.mcp(ctx, request); err != nil {
			return nil, err
		}
		return json.RawMessage(`null`), nil
	default:
		return nil, fmt.Errorf("%s does not support method %q", ExtensionID, method)
	}
}

func (e *Extension) doctor(ctx context.Context, workspace codeintelligence.Workspace) (json.RawMessage, error) {
	out, err := e.combinedOutput(ctx, workspace, "--version")
	if err != nil {
		return nil, fmt.Errorf("Serena version: %w", err)
	}
	response := codeintelligence.DoctorResponse{
		Provider:   ExtensionID,
		Executable: "serena",
		Version:    strings.TrimSpace(string(out)),
	}
	return json.Marshal(response)
}

func (e *Extension) verify(ctx context.Context, workspace codeintelligence.Workspace) (json.RawMessage, error) {
	out, err := e.combinedOutput(ctx, workspace, "project", "health-check", projectPath(workspace))
	if err != nil {
		return nil, fmt.Errorf("Serena project health-check: %w", err)
	}
	response := codeintelligence.VerifyResponse{
		Provider: ExtensionID,
		Output:   strings.TrimSpace(string(out)),
	}
	return json.Marshal(response)
}

func (e *Extension) mcp(ctx context.Context, request codeintelligence.MCPRequest) error {
	contextName := strings.TrimSpace(request.Context)
	if contextName == "" {
		contextName = "agent"
	}
	cmd, err := e.command(ctx, request.Workspace,
		"start-mcp-server",
		"--project", projectPath(request.Workspace),
		"--context", contextName,
	)
	if err != nil {
		return err
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Serena LSP MCP: %w", err)
	}
	return nil
}

func projectPath(workspace codeintelligence.Workspace) string {
	if strings.TrimSpace(workspace.EnvironmentImage) != "" {
		return "/workspace"
	}
	return workspace.Root
}

func (e *Extension) command(ctx context.Context, workspace codeintelligence.Workspace, args ...string) (*exec.Cmd, error) {
	if strings.TrimSpace(e.executable) != "" {
		cmd := exec.CommandContext(ctx, e.executable, args...)
		cmd.Dir = workspace.Root
		return cmd, nil
	}
	return devenv.Command(ctx, workspace, "serena", args...)
}

func (e *Extension) combinedOutput(ctx context.Context, workspace codeintelligence.Workspace, args ...string) ([]byte, error) {
	if strings.TrimSpace(e.executable) != "" {
		cmd := exec.CommandContext(ctx, e.executable, args...)
		cmd.Dir = workspace.Root
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("%s: %w: %s", e.executable, err, strings.TrimSpace(stderr.String()))
		}
		return stdout.Bytes(), nil
	}
	return devenv.CombinedOutput(ctx, workspace, "serena", args...)
}
