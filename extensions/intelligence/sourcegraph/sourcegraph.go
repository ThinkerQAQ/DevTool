package sourcegraph

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/thinkerqaq/devtool/core/agent/mcpbridge"
	service "github.com/thinkerqaq/devtool/sdk/service"
	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
	"github.com/thinkerqaq/devtool/sdk/codeintelligence"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

const ExtensionID = "intelligence.sourcegraph"

type Extension struct {
	endpoint string
	token    string
	remote   *mcpbridge.HTTPProvider
}

func New() *Extension {
	endpoint := strings.TrimSpace(os.Getenv("SOURCEGRAPH_MCP_URL"))
	token := strings.TrimSpace(os.Getenv("SOURCEGRAPH_ACCESS_TOKEN"))
	e := &Extension{endpoint: endpoint, token: token}
	if endpoint != "" {
		e.remote = mcpbridge.NewHTTP(endpoint, sourcegraphAuthorization(token))
	}
	return e
}

func (e *Extension) Descriptor() extensioncontract.Descriptor {
	return extensioncontract.Descriptor{
		ID:       ExtensionID,
		Kind:     extensioncontract.KindCodeIntelligence,
		Provides: []string{codeintelligence.IndexedServiceName},
	}
}

func (e *Extension) Register(reg extensioncontract.Registrar) error {
	return reg.ProvideService(codeintelligence.IndexedServiceName, ExtensionID, service.Func(e.Invoke))
}

func (e *Extension) Invoke(ctx context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
	switch method {
	case codeintelligence.MethodDoctor:
		tools, err := e.listTools(ctx)
		if err != nil {
			return nil, fmt.Errorf("Sourcegraph MCP doctor: %w", err)
		}
		return json.Marshal(codeintelligence.DoctorResponse{
			Provider:   ExtensionID,
			Executable: e.endpoint,
			Version:    fmt.Sprintf("%d tools", len(tools)),
		})
	case codeintelligence.MethodVerify:
		tools, err := e.listTools(ctx)
		if err != nil {
			return nil, fmt.Errorf("Sourcegraph MCP verify: %w", err)
		}
		for _, tool := range tools {
			if tool.Name == "keyword_search" {
				return json.Marshal(codeintelligence.VerifyResponse{
					Provider: ExtensionID,
					Output:   "keyword_search available; index lifecycle is managed by Sourcegraph",
				})
			}
		}
		return nil, fmt.Errorf("Sourcegraph MCP verify: required tool %q is unavailable", "keyword_search")
	case codeintelligence.MethodSearch:
		if e.remote == nil {
			return nil, fmt.Errorf("Sourcegraph MCP is not configured; set SOURCEGRAPH_MCP_URL")
		}
		var request codeintelligence.SearchRequest
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, fmt.Errorf("decode Sourcegraph search request: %w", err)
		}
		query := strings.TrimSpace(request.Query)
		if query == "" {
			return nil, fmt.Errorf("Sourcegraph search query is required")
		}
		if repository := strings.TrimSpace(request.Repository); repository != "" {
			query += " repo:" + repository
		}
		if revision := strings.TrimSpace(request.Revision); revision != "" {
			query += " rev:" + revision
		}
		limit := request.Limit
		if limit <= 0 {
			limit = 20
		}
		query += fmt.Sprintf(" count:%d", limit)
		args, err := json.Marshal(map[string]any{"query": query})
		if err != nil {
			return nil, err
		}
		return e.remote.CallTool(ctx, agentsdk.Session{}, "keyword_search", args)
	default:
		return nil, fmt.Errorf("%s does not support method %q", ExtensionID, method)
	}
}

func (e *Extension) listTools(ctx context.Context) ([]agentsdk.Tool, error) {
	if e.remote == nil {
		return nil, fmt.Errorf("Sourcegraph MCP is not configured; set SOURCEGRAPH_MCP_URL")
	}
	return e.remote.ListTools(ctx, agentsdk.Session{})
}

func sourcegraphAuthorization(token string) string {
	if token == "" {
		return ""
	}
	return "token " + token
}
