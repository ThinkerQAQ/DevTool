package codegraph

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	service "github.com/thinkerqaq/devtool/sdk/service"
	"github.com/thinkerqaq/devtool/extensions/intelligence/internal/envexec"
	"github.com/thinkerqaq/devtool/sdk/codeintelligence"
	environmentcontract "github.com/thinkerqaq/devtool/sdk/environment"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

const ExtensionID = "intelligence.codegraph"

type Extension struct {
	// executable exists only for isolated adapter tests. Production execution
	// is resolved through the configured Environment service.
	executable string
	services   extensioncontract.Registrar
}

func New() *Extension { return &Extension{} }

func (e *Extension) Descriptor() extensioncontract.Descriptor {
	return extensioncontract.Descriptor{
		ID:         ExtensionID,
		Kind:       extensioncontract.KindCodeIntelligence,
		Provides:   []string{codeintelligence.IndexedServiceName},
		Requires:   []string{environmentcontract.ServiceName},
	}
}

func (e *Extension) Register(reg extensioncontract.Registrar) error {
	e.services = reg
	return reg.ProvideService(codeintelligence.IndexedServiceName, ExtensionID, service.Func(e.Invoke))
}

func (e *Extension) Invoke(ctx context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
	switch method {
	case codeintelligence.MethodDoctor:
		var workspace codeintelligence.Workspace
		if err := json.Unmarshal(payload, &workspace); err != nil {
			return nil, fmt.Errorf("decode CodeGraph doctor request: %w", err)
		}
		return e.doctor(ctx, workspace)
	case codeintelligence.MethodVerify:
		var request codeintelligence.Workspace
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, fmt.Errorf("decode CodeGraph verify request: %w", err)
		}
		raw, err := e.runTool(ctx, request, "codegraph_reindex_workspace", json.RawMessage(`{"force":false}`))
		if err != nil {
			return nil, err
		}
		return json.Marshal(codeintelligence.VerifyResponse{Provider: ExtensionID, Output: strings.TrimSpace(string(raw))})
	case codeintelligence.MethodSearch:
		var request codeintelligence.SearchRequest
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, fmt.Errorf("decode CodeGraph search request: %w", err)
		}
		query := strings.TrimSpace(request.Query)
		if query == "" {
			return nil, fmt.Errorf("CodeGraph search query is required")
		}
		limit := request.Limit
		if limit <= 0 {
			limit = 20
		}
		args, err := json.Marshal(map[string]any{
			"query":   query,
			"limit":   limit,
			"compact": true,
		})
		if err != nil {
			return nil, err
		}
		return e.runTool(ctx, request.Workspace, "codegraph_symbol_search", args)
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

func (e *Extension) runTool(ctx context.Context, workspace codeintelligence.Workspace, tool string, toolArgs json.RawMessage) (json.RawMessage, error) {
	args, err := graphBaseArgs(workspace, e.executable == "")
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

func graphBaseArgs(workspace codeintelligence.Workspace, environmentPaths bool) ([]string, error) {
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
		if environmentPaths {
			rel, err := filepath.Rel(root, item)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return nil, fmt.Errorf("CodeGraph workspace %s is outside project root %s", item, root)
			}
			if rel == "." {
				item = environmentcontract.WorkspaceRoot
			} else {
				item = filepath.ToSlash(filepath.Join(environmentcontract.WorkspaceRoot, rel))
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
	return envexec.Command(ctx, e.services, workspace, "codegraph-server", args...)
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
		return nil, fmt.Errorf("codegraph-server: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}
