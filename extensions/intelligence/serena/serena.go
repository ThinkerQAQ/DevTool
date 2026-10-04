package serena

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/thinkerqaq/devtool/core/agent/mcpbridge"
	"github.com/thinkerqaq/devtool/core/service"
	"github.com/thinkerqaq/devtool/extensions/intelligence/internal/envexec"
	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
	"github.com/thinkerqaq/devtool/sdk/codeintelligence"
	environmentcontract "github.com/thinkerqaq/devtool/sdk/environment"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

const ExtensionID = "intelligence.lsp.serena"

type Extension struct {
	// executable exists only for isolated adapter tests. Production execution
	// is resolved through the configured Environment service.
	executable string
	services   extensioncontract.Registrar
	bridge     *mcpbridge.Provider
}

func New() *Extension {
	e := &Extension{}
	e.bridge = mcpbridge.New(e.agentMCPCommand)
	return e
}

func (e *Extension) Close() error {
	if e.bridge == nil {
		return nil
	}
	return e.bridge.Close()
}

func (e *Extension) Descriptor() extensioncontract.Descriptor {
	return extensioncontract.Descriptor{
		ID:         ExtensionID,
		Kind:       extensioncontract.KindCodeIntelligence,
		Provides:   []string{codeintelligence.LSPServiceName},
		Requires:   []string{environmentcontract.ServiceName},
		AgentTools: true,
	}
}

func (e *Extension) Register(reg extensioncontract.Registrar) error {
	e.services = reg
	if err := reg.ProvideService(codeintelligence.LSPServiceName, ExtensionID, service.Func(e.Invoke)); err != nil {
		return err
	}
	if e.bridge == nil {
		e.bridge = mcpbridge.New(e.agentMCPCommand)
	}
	return reg.ProvideAgentTools(ExtensionID, e.bridge)
}

func (e *Extension) agentMCPCommand(ctx context.Context, session agentsdk.Session) (*exec.Cmd, error) {
	workspace := codeintelligence.Workspace{
		Root:             session.ProjectRoot,
		Workspaces:       session.Workspaces,
		EnvironmentImage: session.EnvironmentImage,
	}
	contextName := strings.TrimSpace(session.Context)
	if contextName == "" {
		contextName = "agent"
	}
	return e.command(ctx, workspace,
		"start-mcp-server",
		"--project", e.projectPath(workspace),
		"--context", contextName,
	)
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
	out, err := e.combinedOutput(ctx, workspace, "project", "health-check", e.projectPath(workspace))
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
		"--project", e.projectPath(request.Workspace),
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

func (e *Extension) projectPath(workspace codeintelligence.Workspace) string {
	if strings.TrimSpace(e.executable) == "" {
		return environmentcontract.WorkspaceRoot
	}
	return workspace.Root
}

func (e *Extension) command(ctx context.Context, workspace codeintelligence.Workspace, args ...string) (*exec.Cmd, error) {
	if strings.TrimSpace(e.executable) != "" {
		cmd := exec.CommandContext(ctx, e.executable, args...)
		cmd.Dir = workspace.Root
		return cmd, nil
	}
	return envexec.Command(ctx, e.services, workspace, "serena", args...)
}

func (e *Extension) combinedOutput(ctx context.Context, workspace codeintelligence.Workspace, args ...string) ([]byte, error) {
	cmd, err := e.command(ctx, workspace, args...)
	if err != nil {
		return nil, err
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("serena: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}
