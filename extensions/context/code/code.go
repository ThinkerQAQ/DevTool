package code

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/thinkerqaq/devtool/sdk/codecontext"
	"github.com/thinkerqaq/devtool/sdk/codeintelligence"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
	"github.com/thinkerqaq/devtool/sdk/service"
	devtooltrace "github.com/thinkerqaq/devtool/sdk/trace"
)

const ExtensionID = "context.code.composite"

// This is an indexed-only transitional service. It does not attempt
// realtime LSP operations or infer provider availability across RPC.
// Native CodeGraph and LSP MCP tools will replace this composite.
type Extension struct {
	services extensioncontract.Registrar
}

func New() *Extension { return &Extension{} }

func (e *Extension) Descriptor() extensioncontract.Descriptor {
	return extensioncontract.Descriptor{
		ID:       ExtensionID,
		Kind:     extensioncontract.KindCodeIntelligence,
		Provides: []string{codecontext.ServiceName},
		Requires: []string{codeintelligence.IndexedServiceName},
	}
}

func (e *Extension) Register(reg extensioncontract.Registrar) error {
	e.services = reg
	return reg.ProvideService(codecontext.ServiceName, ExtensionID, service.Func(e.Invoke))
}

func (e *Extension) Invoke(ctx context.Context, method string, payload json.RawMessage) (result json.RawMessage, err error) {
	if method != codecontext.MethodBuild {
		return nil, fmt.Errorf("%s does not support method %q", ExtensionID, method)
	}
	var request codecontext.BuildRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		return nil, fmt.Errorf("decode code context request: %w", err)
	}
	request.Objective = strings.TrimSpace(request.Objective)
	request.Symbol = strings.TrimSpace(request.Symbol)
	request.Path = strings.TrimSpace(request.Path)
	if request.Objective == "" {
		return nil, fmt.Errorf("code context objective is required")
	}

	ctx, span := devtooltrace.Start(ctx, devtooltrace.Attributes{
		Name: "code-context", Layer: "service",
		Service: codecontext.ServiceName, Provider: ExtensionID,
		Method: method, RequestBytes: len(payload),
	})
	defer func() { span.End(len(result), err) }()

	response, err := e.build(ctx, request)
	if err != nil {
		return nil, err
	}
	return json.Marshal(response)
}

func (e *Extension) build(ctx context.Context, request codecontext.BuildRequest) (codecontext.BuildResponse, error) {
	if e.services == nil {
		return codecontext.BuildResponse{}, fmt.Errorf("code context service registry is unavailable")
	}
	invoker, ok := e.services.Service(codeintelligence.IndexedServiceName)
	if !ok {
		return codecontext.BuildResponse{}, fmt.Errorf("indexed code intelligence is not configured")
	}
	query := request.Objective
	if request.Symbol != "" {
		query = request.Symbol
	}
	search := codeintelligence.SearchRequest{
		Workspace: request.Workspace,
		Query:     query,
		Limit:     request.Limit,
		// Use CodeGraph's native symbol search rather than the bespoke
		// keyword splitter and ranking-based discovery path.
		Discovery: false,
	}
	raw, err := json.Marshal(search)
	if err != nil {
		return codecontext.BuildResponse{}, err
	}
	indexed, err := invoker.Invoke(ctx, codeintelligence.MethodSearch, raw)
	if err != nil {
		return codecontext.BuildResponse{}, fmt.Errorf("indexed code intelligence: %w", err)
	}

	skipped := []string{"realtime code intelligence is not configured"}
	if request.Path != "" {
		skipped = append(skipped, "indexed search does not guarantee a file or directory scope")
	}
	return codecontext.BuildResponse{
		Objective:  request.Objective,
		Indexed:    indexed,
		NotApplied: skipped,
	}, nil
}
