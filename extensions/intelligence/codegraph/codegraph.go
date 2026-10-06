package codegraph

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/thinkerqaq/devtool/core/agent/mcpbridge"
	"github.com/thinkerqaq/devtool/extensions/intelligence/internal/envexec"
	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
	"github.com/thinkerqaq/devtool/sdk/codeintelligence"
	environmentcontract "github.com/thinkerqaq/devtool/sdk/environment"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
	service "github.com/thinkerqaq/devtool/sdk/service"
	devtooltrace "github.com/thinkerqaq/devtool/sdk/trace"
)

const ExtensionID = "intelligence.codegraph"

type Extension struct {
	// executable exists only for isolated adapter tests. Production execution
	// is resolved through the configured Environment service.
	executable string
	services   extensioncontract.Registrar

	mu               sync.Mutex
	bridge           *mcpbridge.Provider
	workspaceVersion uint64
	persistent       bool
}

func New() *Extension { return &Extension{persistent: true} }

func (e *Extension) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.bridge == nil {
		return nil
	}
	err := e.bridge.Close()
	e.bridge = nil
	e.workspaceVersion = 0
	return err
}

func (e *Extension) Descriptor() extensioncontract.Descriptor {
	return extensioncontract.Descriptor{
		ID:       ExtensionID,
		Kind:     extensioncontract.KindCodeIntelligence,
		Provides: []string{codeintelligence.IndexedServiceName},
		Requires: []string{environmentcontract.ServiceName},
	}
}

func (e *Extension) Register(reg extensioncontract.Registrar) error {
	e.services = reg
	return reg.ProvideService(codeintelligence.IndexedServiceName, ExtensionID, service.Func(e.Invoke))
}

func (e *Extension) mcpCommand(ctx context.Context, session agentsdk.Session) (*exec.Cmd, error) {
	workspace := codeintelligence.Workspace{
		Root:       session.ProjectRoot,
		Workspaces: session.Workspaces,
	}
	base, err := graphBaseArgs(workspace, e.executable == "")
	if err != nil {
		return nil, err
	}
	args := append([]string{"--mcp"}, base...)
	return e.command(ctx, workspace, args...)
}

func (e *Extension) Invoke(ctx context.Context, method string, payload json.RawMessage) (result json.RawMessage, err error) {
	ctx, span := devtooltrace.Start(ctx, devtooltrace.Attributes{
		Name:         "CodeGraph",
		Layer:        "provider",
		Service:      codeintelligence.IndexedServiceName,
		Provider:     ExtensionID,
		Method:       method,
		RequestBytes: len(payload),
	})
	defer func() { span.End(len(result), err) }()

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
	if e.persistent {
		return e.runPersistentTool(ctx, workspace, tool, toolArgs)
	}
	return e.runOneShotTool(ctx, workspace, tool, toolArgs)
}

func (e *Extension) runPersistentTool(ctx context.Context, workspace codeintelligence.Workspace, tool string, toolArgs json.RawMessage) (json.RawMessage, error) {
	version, err := workspaceFingerprint(workspace)
	if err != nil {
		return nil, err
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.bridge == nil || e.workspaceVersion != version {
		if e.bridge != nil {
			_ = e.bridge.Close()
		}
		e.bridge = mcpbridge.New(e.mcpCommand)
		e.workspaceVersion = version
	}

	session := agentsdk.Session{
		ProjectRoot: workspace.Root,
		Workspaces:  append([]string(nil), workspace.Workspaces...),
		Context:     "codegraph",
	}
	raw, err := e.bridge.CallTool(ctx, session, tool, toolArgs)
	if err != nil {
		return nil, fmt.Errorf("CodeGraph %s: %w", tool, err)
	}
	return unwrapMCPToolResult(raw)
}

func workspaceFingerprint(workspace codeintelligence.Workspace) (uint64, error) {
	root := strings.TrimSpace(workspace.Root)
	if root == "" {
		return 0, fmt.Errorf("CodeGraph workspace root is required")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return 0, fmt.Errorf("resolve CodeGraph root: %w", err)
	}
	workspaces := workspace.Workspaces
	if len(workspaces) == 0 {
		workspaces = []string{root}
	}

	h := fnv.New64a()
	for _, item := range workspaces {
		if !filepath.IsAbs(item) {
			item = filepath.Join(root, item)
		}
		item = filepath.Clean(item)
		if _, err := os.Stat(item); err != nil {
			return 0, fmt.Errorf("stat CodeGraph workspace %s: %w", item, err)
		}
		if err := filepath.WalkDir(item, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() && path != item && ignoredFingerprintDir(entry.Name()) {
				return filepath.SkipDir
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			_, _ = h.Write([]byte(filepath.ToSlash(rel)))
			_, _ = h.Write([]byte{0})
			_, _ = h.Write([]byte(strconv.FormatInt(info.Size(), 10)))
			_, _ = h.Write([]byte{0})
			_, _ = h.Write([]byte(strconv.FormatInt(info.ModTime().UnixNano(), 10)))
			_, _ = h.Write([]byte{0})
			return nil
		}); err != nil {
			return 0, fmt.Errorf("fingerprint CodeGraph workspace %s: %w", item, err)
		}
	}
	return h.Sum64(), nil
}

func ignoredFingerprintDir(name string) bool {
	switch name {
	case ".git", ".devtool", ".cache", "node_modules", "dist", "coverage":
		return true
	default:
		return false
	}
}

func (e *Extension) runOneShotTool(ctx context.Context, workspace codeintelligence.Workspace, tool string, toolArgs json.RawMessage) (json.RawMessage, error) {
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

func unwrapMCPToolResult(raw json.RawMessage) (json.RawMessage, error) {
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError,omitempty"`
	}
	if err := json.Unmarshal(raw, &result); err != nil || len(result.Content) == 0 {
		return append(json.RawMessage(nil), raw...), nil
	}
	text := strings.TrimSpace(result.Content[0].Text)
	if result.IsError {
		if text == "" {
			text = "CodeGraph MCP tool returned an error"
		}
		return nil, fmt.Errorf("%s", text)
	}
	if text != "" && json.Valid([]byte(text)) {
		return json.RawMessage(text), nil
	}
	return append(json.RawMessage(nil), raw...), nil
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
