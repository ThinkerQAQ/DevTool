package codegraph

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/thinkerqaq/devtool/core/service"
	"github.com/thinkerqaq/devtool/extensions/intelligence/internal/devenv"
	"github.com/thinkerqaq/devtool/sdk/codeintelligence"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

const ExtensionID = "intelligence.codegraph"

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
		Provides: []string{codeintelligence.GraphServiceName},
	}
}

func (e *Extension) Register(reg extensioncontract.Registrar) error {
	return reg.ProvideService(codeintelligence.GraphServiceName, ExtensionID, service.Func(e.Invoke))
}

func (e *Extension) Invoke(ctx context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
	switch method {
	case codeintelligence.MethodDoctor:
		var workspace codeintelligence.Workspace
		if err := json.Unmarshal(payload, &workspace); err != nil {
			return nil, fmt.Errorf("decode CodeGraph doctor request: %w", err)
		}
		return e.doctor(ctx, workspace)
	case codeintelligence.MethodMCP:
		var request codeintelligence.MCPRequest
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, fmt.Errorf("decode CodeGraph MCP request: %w", err)
		}
		if err := e.mcp(ctx, request.Workspace); err != nil {
			return nil, err
		}
		return json.RawMessage(`null`), nil
	case codeintelligence.MethodSync:
		var request codeintelligence.Workspace
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, fmt.Errorf("decode CodeGraph sync request: %w", err)
		}
		return e.runTool(ctx, request, "codegraph_reindex_workspace", json.RawMessage(`{"force":false}`))
	case codeintelligence.MethodQuery:
		var request codeintelligence.GraphQuery
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, fmt.Errorf("decode CodeGraph query request: %w", err)
		}
		tool := strings.TrimSpace(request.Tool)
		if tool == "" {
			return nil, fmt.Errorf("CodeGraph query tool is required")
		}
		if !strings.HasPrefix(tool, "codegraph_") {
			tool = "codegraph_" + tool
		}
		args := request.Args
		if len(args) == 0 {
			args = json.RawMessage(`{}`)
		}
		if !json.Valid(args) {
			return nil, fmt.Errorf("CodeGraph query args must be valid JSON")
		}
		return e.runTool(ctx, request.Workspace, tool, args)
	default:
		return nil, fmt.Errorf("%s does not support method %q", ExtensionID, method)
	}
}

func (e *Extension) doctor(ctx context.Context, workspace codeintelligence.Workspace) (json.RawMessage, error) {
	out, err := e.combinedOutput(ctx, workspace, "--version")
	if err != nil {
		return nil, fmt.Errorf("CodeGraph version: %w", err)
	}
	response := codeintelligence.DoctorResponse{
		Provider:   ExtensionID,
		Executable: "codegraph-server",
		Version:    strings.TrimSpace(string(out)),
	}
	return json.Marshal(response)
}

func (e *Extension) mcp(ctx context.Context, workspace codeintelligence.Workspace) error {
	args, err := graphBaseArgs(workspace, true)
	if err != nil {
		return err
	}
	args = append(args, "--profile", "graph", "--mcp")
	cmd, err := e.command(ctx, workspace, args...)
	if err != nil {
		return err
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("CodeGraph MCP: %w", err)
	}
	return nil
}

func (e *Extension) runTool(ctx context.Context, workspace codeintelligence.Workspace, tool string, toolArgs json.RawMessage) (json.RawMessage, error) {
	args, err := graphBaseArgs(workspace, strings.TrimSpace(workspace.EnvironmentImage) != "")
	if err != nil {
		return nil, err
	}
	args = append(args,
		"--run-tool", tool,
		"--tool-args", string(toolArgs),
	)
	out, err := e.combinedOutput(ctx, workspace, args...)
	if err != nil {
		return nil, fmt.Errorf("CodeGraph %s: %w", tool, err)
	}
	raw := bytes.TrimSpace(out)
	if len(raw) == 0 {
		return json.RawMessage(`null`), nil
	}
	if json.Valid(raw) {
		return append(json.RawMessage(nil), raw...), nil
	}
	return json.Marshal(string(raw))
}

func graphBaseArgs(workspace codeintelligence.Workspace, containerPaths bool) ([]string, error) {
	root := strings.TrimSpace(workspace.Root)
	if root == "" {
		return nil, fmt.Errorf("CodeGraph workspace root is required")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve CodeGraph root: %w", err)
	}
	workspaces := workspace.Workspaces
	if len(workspaces) == 0 {
		workspaces = []string{root}
	}
	args := []string{"--graph-only"}
	for _, item := range workspaces {
		item = strings.TrimSpace(item)
		if item == "" {
			return nil, fmt.Errorf("CodeGraph workspace cannot be empty")
		}
		if !filepath.IsAbs(item) {
			item = filepath.Join(root, item)
		}
		item = filepath.Clean(item)
		if containerPaths && strings.TrimSpace(workspace.EnvironmentImage) != "" {
			rel, err := filepath.Rel(root, item)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return nil, fmt.Errorf("CodeGraph workspace %s is outside project root %s", item, root)
			}
			if rel == "." {
				item = "/workspace"
			} else {
				item = filepath.ToSlash(filepath.Join("/workspace", rel))
			}
		}
		args = append(args, "--workspace", item)
	}
	return args, nil
}

func (e *Extension) command(ctx context.Context, workspace codeintelligence.Workspace, args ...string) (*exec.Cmd, error) {
	if strings.TrimSpace(e.executable) != "" {
		cmd := exec.CommandContext(ctx, e.executable, args...)
		cmd.Dir = workspace.Root
		return cmd, nil
	}
	return devenv.Command(ctx, workspace, "codegraph-server", args...)
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
	return devenv.CombinedOutput(ctx, workspace, "codegraph-server", args...)
}
