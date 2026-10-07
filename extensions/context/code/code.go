package code

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/thinkerqaq/devtool/sdk/codecontext"
	"github.com/thinkerqaq/devtool/sdk/codeintelligence"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
	"github.com/thinkerqaq/devtool/sdk/service"
	devtooltrace "github.com/thinkerqaq/devtool/sdk/trace"
)

const ExtensionID = "context.code.composite"

type Extension struct {
	services extensioncontract.Registrar
}

func New() *Extension { return &Extension{} }

func (e *Extension) Descriptor() extensioncontract.Descriptor {
	return extensioncontract.Descriptor{
		ID:       ExtensionID,
		Kind:     extensioncontract.KindCodeIntelligence,
		Provides: []string{codecontext.ServiceName},
		Requires: []string{
			codeintelligence.IndexedServiceName,
			codeintelligence.RealtimeServiceName,
		},
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
		Name:         "code-context",
		Layer:        "service",
		Service:      codecontext.ServiceName,
		Provider:     ExtensionID,
		Method:       method,
		RequestBytes: len(payload),
	})
	defer func() { span.End(len(result), err) }()

	response, err := e.build(ctx, request)
	if err != nil {
		return nil, err
	}
	return json.Marshal(response)
}

type branchResult struct {
	name     string
	indexed  json.RawMessage
	realtime map[string]json.RawMessage
	degraded map[string]string
	skipped  []string
	err      error
}

func (e *Extension) build(ctx context.Context, request codecontext.BuildRequest) (codecontext.BuildResponse, error) {
	query := request.Objective
	if request.Symbol != "" {
		query = request.Symbol
	}
	indexedRequest := codeintelligence.SearchRequest{
		Workspace: request.Workspace,
		Query:     query,
		Limit:     request.Limit,
		Discovery: request.Symbol == "",
	}

	response := codecontext.BuildResponse{Objective: request.Objective}
	if request.Symbol == "" && request.Path == "" {
		indexed, err := e.invoke(ctx, codeintelligence.IndexedServiceName, codeintelligence.MethodSearch, indexedRequest)
		if err != nil {
			return codecontext.BuildResponse{}, fmt.Errorf("build indexed code context: %w", err)
		}
		response.Indexed = indexed
		return response, nil
	}

	results := make(chan branchResult, 2)
	go func() {
		raw, err := e.invoke(ctx, codeintelligence.IndexedServiceName, codeintelligence.MethodSearch, indexedRequest)
		results <- branchResult{name: "indexed", indexed: raw, err: err}
	}()
	go func() {
		realtime, degraded, skipped := e.fetchRealtime(ctx, request)
		results <- branchResult{name: "realtime", realtime: realtime, degraded: degraded, skipped: skipped}
	}()

	var indexedErr error
	for range 2 {
		branch := <-results
		switch branch.name {
		case "indexed":
			if branch.err != nil {
				indexedErr = branch.err
				response.Degraded = addDegraded(response.Degraded, "indexed", branch.err)
			} else {
				response.Indexed = branch.indexed
			}
		case "realtime":
			response.Realtime = branch.realtime
			for key, message := range branch.degraded {
				if response.Degraded == nil {
					response.Degraded = map[string]string{}
				}
				response.Degraded[key] = message
			}
			response.NotApplied = append(response.NotApplied, branch.skipped...)
		}
	}

	if len(response.Indexed) == 0 && len(response.Realtime) == 0 {
		if indexedErr != nil {
			return codecontext.BuildResponse{}, fmt.Errorf("code context providers produced no usable result: %w", indexedErr)
		}
		return codecontext.BuildResponse{}, fmt.Errorf("code context providers produced no usable result")
	}
	return response, nil
}

func (e *Extension) fetchRealtime(ctx context.Context, request codecontext.BuildRequest) (map[string]json.RawMessage, map[string]string, []string) {
	realtime := map[string]json.RawMessage{}
	degraded := map[string]string{}
	var skipped []string

	if request.Symbol != "" {
		raw, err := e.invoke(ctx, codeintelligence.RealtimeServiceName, codeintelligence.MethodSymbols, codeintelligence.SymbolRequest{
			Workspace:   request.Workspace,
			Symbol:      request.Symbol,
			Path:        request.Path,
			IncludeBody: request.IncludeBody,
		})
		if err != nil {
			degraded["realtime.symbols"] = err.Error()
		} else {
			realtime["symbols"] = raw
		}
	}

	if request.Symbol != "" && request.Path != "" {
		raw, err := e.invoke(ctx, codeintelligence.RealtimeServiceName, codeintelligence.MethodReferences, codeintelligence.ReferencesRequest{
			Workspace: request.Workspace,
			Symbol:    request.Symbol,
			Path:      request.Path,
		})
		if err != nil {
			degraded["realtime.references"] = err.Error()
		} else {
			realtime["references"] = raw
		}
	}

	if request.Path != "" {
		if isDirectory(request.Root, request.Path) {
			skipped = append(skipped, "realtime.diagnostics: directory scope")
		} else {
			raw, err := e.invoke(ctx, codeintelligence.RealtimeServiceName, codeintelligence.MethodDiagnostics, codeintelligence.DiagnosticsRequest{
				Workspace: request.Workspace,
				Path:      request.Path,
			})
			if err != nil {
				degraded["realtime.diagnostics"] = err.Error()
			} else {
				realtime["diagnostics"] = raw
			}
		}
	}

	if len(realtime) == 0 {
		realtime = nil
	}
	if len(degraded) == 0 {
		degraded = nil
	}
	return realtime, degraded, skipped
}

func (e *Extension) invoke(ctx context.Context, serviceName, method string, request any) (json.RawMessage, error) {
	if e.services == nil {
		return nil, fmt.Errorf("code context service registry is unavailable")
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

func addDegraded(current map[string]string, key string, err error) map[string]string {
	if current == nil {
		current = map[string]string{}
	}
	current[key] = err.Error()
	return current
}

func isDirectory(root, path string) bool {
	candidate := path
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(root, candidate)
	}
	info, err := os.Stat(candidate)
	return err == nil && info.IsDir()
}
