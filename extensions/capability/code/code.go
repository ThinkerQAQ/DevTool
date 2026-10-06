package code

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
	"github.com/thinkerqaq/devtool/sdk/codeintelligence"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
	devtooltrace "github.com/thinkerqaq/devtool/sdk/trace"
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

func (e *Extension) CallTool(ctx context.Context, session agentsdk.Session, name string, args json.RawMessage) (result json.RawMessage, err error) {
	if strings.TrimSpace(name) != "code_context" {
		return nil, fmt.Errorf("unknown code capability tool %q", name)
	}
	ctx, span := devtooltrace.Start(ctx, devtooltrace.Attributes{
		Name:         "code_context",
		Layer:        "capability",
		Tool:         "code_context",
		RequestBytes: len(args),
	})
	defer func() { span.End(len(result), err) }()

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

	symbol := strings.TrimSpace(input.Symbol)
	path := strings.TrimSpace(input.Path)

	indexedRequest := codeintelligence.SearchRequest{
		Workspace: workspace,
		Query:     query,
		Limit:     input.Limit,
		Discovery: symbol == "",
	}

	var indexed json.RawMessage
	var realtime map[string]any
	if symbol == "" && path == "" {
		indexed, err = e.fetchIndexed(ctx, indexedRequest)
		if err != nil {
			return nil, err
		}
	} else {
		indexed, realtime, err = e.fetchContextParallel(ctx, indexedRequest, workspace, symbol, path, input.IncludeBody)
		if err != nil {
			return nil, err
		}
	}

	bundle := map[string]any{
		"objective": objective,
		"indexed":   decodeResult(indexed),
	}
	if len(realtime) != 0 {
		bundle["realtime"] = realtime
	}

	return toolResult(bundle)
}

func (e *Extension) fetchIndexed(ctx context.Context, request codeintelligence.SearchRequest) (json.RawMessage, error) {
	raw, err := e.invoke(ctx, codeintelligence.IndexedServiceName, codeintelligence.MethodSearch, request)
	if err != nil {
		return nil, fmt.Errorf("build indexed code context: %w", err)
	}
	return raw, nil
}

type contextBranchResult struct {
	name     string
	indexed  json.RawMessage
	realtime map[string]any
	err      error
}

func (e *Extension) fetchContextParallel(
	ctx context.Context,
	indexedRequest codeintelligence.SearchRequest,
	workspace codeintelligence.Workspace,
	symbol string,
	path string,
	includeBody bool,
) (json.RawMessage, map[string]any, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan contextBranchResult, 2)

	go func() {
		raw, err := e.fetchIndexed(ctx, indexedRequest)
		results <- contextBranchResult{name: "indexed", indexed: raw, err: err}
	}()
	go func() {
		realtime, err := e.fetchRealtime(ctx, workspace, symbol, path, includeBody)
		results <- contextBranchResult{name: "realtime", realtime: realtime, err: err}
	}()

	var indexed json.RawMessage
	var realtime map[string]any
	var indexedErr, realtimeErr error
	for range 2 {
		result := <-results
		switch result.name {
		case "indexed":
			indexed, indexedErr = result.indexed, result.err
		case "realtime":
			realtime, realtimeErr = result.realtime, result.err
		}
		if result.err != nil {
			cancel()
		}
	}
	if indexedErr != nil {
		return nil, nil, indexedErr
	}
	if realtimeErr != nil {
		return nil, nil, realtimeErr
	}
	return indexed, realtime, nil
}

func (e *Extension) fetchRealtime(ctx context.Context, workspace codeintelligence.Workspace, symbol, path string, includeBody bool) (map[string]any, error) {
	realtime := map[string]any{}
	if symbol != "" {
		raw, err := e.invoke(ctx, codeintelligence.RealtimeServiceName, codeintelligence.MethodSymbols, codeintelligence.SymbolRequest{
			Workspace:   workspace,
			Symbol:      symbol,
			Path:        path,
			IncludeBody: includeBody,
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
	return realtime, nil
}

func (e *Extension) invoke(ctx context.Context, serviceName, method string, request any) (result json.RawMessage, err error) {
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
	ctx, span := devtooltrace.Start(ctx, devtooltrace.Attributes{
		Name:         serviceName + "." + method,
		Layer:        "service",
		Service:      serviceName,
		Method:       method,
		RequestBytes: len(payload),
	})
	defer func() { span.End(len(result), err) }()
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
