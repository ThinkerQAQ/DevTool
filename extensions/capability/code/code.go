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
		ID:         ExtensionID,
		Kind:       extensioncontract.KindCapability,
		Requires:   []string{codeintelligence.IndexedServiceName, codeintelligence.RealtimeServiceName},
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
			"code_context",
			"Build project code context for an engineering objective by composing indexed and realtime code intelligence.",
			map[string]any{
				"objective": map[string]any{
					"type":        "string",
					"description": "What the agent needs to understand or change.",
				},
				"symbol": map[string]any{
					"type":        "string",
					"description": "Optional symbol hint used to enrich realtime context.",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Optional project-relative path used to enrich realtime context.",
				},
				"include_body": map[string]any{
					"type":        "boolean",
					"description": "Include symbol bodies when realtime intelligence supports it.",
				},
				"limit": map[string]any{
					"type":    "integer",
					"minimum": 1,
					"maximum": 100,
				},
			},
			[]string{"objective"},
		),
	}, nil
}

func (e *Extension) CallTool(ctx context.Context, session agentsdk.Session, name string, args json.RawMessage) (json.RawMessage, error) {
	if strings.TrimSpace(name) != "code_context" {
		return nil, fmt.Errorf("unknown code capability tool %q", name)
	}

	var input struct {
		Objective   string `json:"objective"`
		Symbol      string `json:"symbol,omitempty"`
		Path        string `json:"path,omitempty"`
		IncludeBody bool   `json:"include_body,omitempty"`
		Limit       int    `json:"limit,omitempty"`
	}
	if err := decodeArgs(args, &input); err != nil {
		return nil, err
	}
	objective := strings.TrimSpace(input.Objective)
	if objective == "" {
		return nil, fmt.Errorf("code_context objective is required")
	}

	workspace := workspaceFromSession(session)
	query := objective
	if symbol := strings.TrimSpace(input.Symbol); symbol != "" {
		query = symbol
	}

	indexed, err := e.invoke(ctx, codeintelligence.IndexedServiceName, codeintelligence.MethodSearch, codeintelligence.SearchRequest{
		Workspace: workspace,
		Query:     query,
		Limit:     input.Limit,
	})
	if err != nil {
		return nil, fmt.Errorf("build indexed code context: %w", err)
	}

	bundle := map[string]any{
		"objective": objective,
		"indexed":   decodeResult(indexed),
	}

	symbol := strings.TrimSpace(input.Symbol)
	path := strings.TrimSpace(input.Path)
	realtime := map[string]any{}

	if symbol != "" {
		raw, err := e.invoke(ctx, codeintelligence.RealtimeServiceName, codeintelligence.MethodSymbols, codeintelligence.SymbolRequest{
			Workspace:   workspace,
			Symbol:      symbol,
			Path:        path,
			IncludeBody: input.IncludeBody,
		})
		if err != nil {
			return nil, fmt.Errorf("build realtime symbol context: %w", err)
		}
		realtime["symbols"] = decodeResult(raw)
	}
	if symbol != "" && path != "" {
		raw, err := e.invoke(ctx, codeintelligence.RealtimeServiceName, codeintelligence.MethodReferences, codeintelligence.ReferencesRequest{
			Workspace: workspace,
			Symbol:    symbol,
			Path:      path,
		})
		if err != nil {
			return nil, fmt.Errorf("build realtime reference context: %w", err)
		}
		realtime["references"] = decodeResult(raw)
	}
	if path != "" {
		raw, err := e.invoke(ctx, codeintelligence.RealtimeServiceName, codeintelligence.MethodDiagnostics, codeintelligence.DiagnosticsRequest{
			Workspace: workspace,
			Path:      path,
		})
		if err != nil {
			return nil, fmt.Errorf("build realtime diagnostics context: %w", err)
		}
		realtime["diagnostics"] = decodeResult(raw)
	}
	if len(realtime) != 0 {
		bundle["realtime"] = realtime
	}

	return toolResult(bundle)
}

func (e *Extension) invoke(ctx context.Context, serviceName, method string, request any) (json.RawMessage, error) {
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
	return invoker.Invoke(ctx, method, payload)
}

func tool(name, description string, properties map[string]any, required []string) agentsdk.Tool {
	definition, _ := json.Marshal(map[string]any{
		"name":        name,
		"description": description,
		"inputSchema": map[string]any{
			"type":                 "object",
			"properties":           properties,
			"required":             required,
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
		Root:       session.ProjectRoot,
		Workspaces: append([]string(nil), session.Workspaces...),
	}
}

func decodeResult(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return string(raw)
	}
	return value
}

func toolResult(value any) (json.RawMessage, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"content": []map[string]string{{
			"type": "text",
			"text": string(raw),
		}},
	})
}
