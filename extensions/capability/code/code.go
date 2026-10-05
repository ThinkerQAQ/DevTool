package code

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
	"github.com/thinkerqaq/devtool/sdk/codeintelligence"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

const ExtensionID = "capability.code"

type Extension struct {
	services extensioncontract.Registrar
}

func New() *Extension { return &Extension{} }

func (e *Extension) Descriptor() extensioncontract.Descriptor {
	return extensioncontract.Descriptor{
		ID:       ExtensionID,
		Kind:     extensioncontract.KindCapability,
		Requires: []string{codeintelligence.IndexedServiceName, codeintelligence.RealtimeServiceName},
		AgentTools: true,
	}
}

func (e *Extension) Register(reg extensioncontract.Registrar) error {
	e.services = reg
	return reg.ProvideAgentTools(ExtensionID, e)
}

func (e *Extension) ListTools(context.Context, agentsdk.Session) ([]agentsdk.Tool, error) {
	return []agentsdk.Tool{
		tool(
			"code_search",
			"Search code through the selected indexed-intelligence provider.",
			map[string]any{
				"query": map[string]any{"type": "string", "description": "Code or symbol query."},
				"repository": map[string]any{"type": "string", "description": "Optional repository name for remote indexed providers."},
				"revision": map[string]any{"type": "string", "description": "Optional branch, tag, or revision."},
				"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 100},
			},
			[]string{"query"},
		),
		tool(
			"code_symbols",
			"Find symbols in the current working tree through realtime code intelligence.",
			map[string]any{
				"symbol": map[string]any{"type": "string", "description": "Symbol or name-path pattern."},
				"path": map[string]any{"type": "string", "description": "Optional project-relative file path."},
				"include_body": map[string]any{"type": "boolean", "description": "Include symbol body when supported."},
			},
			[]string{"symbol"},
		),
		tool(
			"code_references",
			"Find references to a symbol in the current working tree through realtime code intelligence.",
			map[string]any{
				"symbol": map[string]any{"type": "string", "description": "Symbol name path."},
				"path": map[string]any{"type": "string", "description": "Project-relative path containing the symbol definition."},
			},
			[]string{"symbol", "path"},
		),
		tool(
			"code_diagnostics",
			"Get realtime language diagnostics for a project file.",
			map[string]any{
				"path": map[string]any{"type": "string", "description": "Project-relative file path."},
			},
			[]string{"path"},
		),
	}, nil
}

func (e *Extension) CallTool(ctx context.Context, session agentsdk.Session, name string, args json.RawMessage) (json.RawMessage, error) {
	workspace := workspaceFromSession(session)
	switch strings.TrimSpace(name) {
	case "code_search":
		var input struct {
			Query      string `json:"query"`
			Repository string `json:"repository,omitempty"`
			Revision   string `json:"revision,omitempty"`
			Limit      int    `json:"limit,omitempty"`
		}
		if err := decodeArgs(args, &input); err != nil {
			return nil, err
		}
		if strings.TrimSpace(input.Query) == "" {
			return nil, fmt.Errorf("code_search query is required")
		}
		return e.invokeTool(ctx, codeintelligence.IndexedServiceName, codeintelligence.MethodSearch, codeintelligence.SearchRequest{
			Workspace: workspace,
			Query: strings.TrimSpace(input.Query),
			Repository: strings.TrimSpace(input.Repository),
			Revision: strings.TrimSpace(input.Revision),
			Limit: input.Limit,
		})
	case "code_symbols":
		var input struct {
			Symbol      string `json:"symbol"`
			Path        string `json:"path,omitempty"`
			IncludeBody bool   `json:"include_body,omitempty"`
		}
		if err := decodeArgs(args, &input); err != nil {
			return nil, err
		}
		if strings.TrimSpace(input.Symbol) == "" {
			return nil, fmt.Errorf("code_symbols symbol is required")
		}
		return e.invokeTool(ctx, codeintelligence.RealtimeServiceName, codeintelligence.MethodSymbols, codeintelligence.SymbolRequest{
			Workspace: workspace,
			Symbol: strings.TrimSpace(input.Symbol),
			Path: strings.TrimSpace(input.Path),
			IncludeBody: input.IncludeBody,
		})
	case "code_references":
		var input struct {
			Symbol string `json:"symbol"`
			Path   string `json:"path"`
		}
		if err := decodeArgs(args, &input); err != nil {
			return nil, err
		}
		if strings.TrimSpace(input.Symbol) == "" || strings.TrimSpace(input.Path) == "" {
			return nil, fmt.Errorf("code_references symbol and path are required")
		}
		return e.invokeTool(ctx, codeintelligence.RealtimeServiceName, codeintelligence.MethodReferences, codeintelligence.ReferencesRequest{
			Workspace: workspace,
			Symbol: strings.TrimSpace(input.Symbol),
			Path: strings.TrimSpace(input.Path),
		})
	case "code_diagnostics":
		var input struct {
			Path string `json:"path"`
		}
		if err := decodeArgs(args, &input); err != nil {
			return nil, err
		}
		if strings.TrimSpace(input.Path) == "" {
			return nil, fmt.Errorf("code_diagnostics path is required")
		}
		return e.invokeTool(ctx, codeintelligence.RealtimeServiceName, codeintelligence.MethodDiagnostics, codeintelligence.DiagnosticsRequest{
			Workspace: workspace,
			Path: strings.TrimSpace(input.Path),
		})
	default:
		return nil, fmt.Errorf("unknown code capability tool %q", name)
	}
}

func (e *Extension) invokeTool(ctx context.Context, serviceName, method string, request any) (json.RawMessage, error) {
	if e.services == nil {
		return nil, fmt.Errorf("code capability service registry is unavailable")
	}
	invoker, ok := e.services.Service(serviceName)
	if !ok {
		return nil, fmt.Errorf("service %q is not configured", serviceName)
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	raw, err := invoker.Invoke(ctx, method, payload)
	if err != nil {
		return nil, err
	}
	return wrapResult(serviceName, raw)
}

func tool(name, description string, properties map[string]any, required []string) agentsdk.Tool {
	definition, _ := json.Marshal(map[string]any{
		"name": name,
		"description": description,
		"inputSchema": map[string]any{
			"type": "object",
			"properties": properties,
			"required": required,
			"additionalProperties": false,
		},
	})
	return agentsdk.Tool{Name: name, Definition: definition}
}

func decodeArgs(raw json.RawMessage, out any) error {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode code capability arguments: %w", err)
	}
	return nil
}

func workspaceFromSession(session agentsdk.Session) codeintelligence.Workspace {
	return codeintelligence.Workspace{
		Root: session.ProjectRoot,
		Workspaces: append([]string(nil), session.Workspaces...),
		EnvironmentImage: session.EnvironmentImage,
	}
}

func wrapResult(serviceName string, raw json.RawMessage) (json.RawMessage, error) {
	var result any
	if len(raw) == 0 {
		result = nil
	} else if err := json.Unmarshal(raw, &result); err != nil {
		result = string(raw)
	}
	envelope, err := json.Marshal(map[string]any{
		"service": serviceName,
		"result": result,
	})
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"content": []map[string]string{{"type": "text", "text": string(envelope)}},
	})
}
