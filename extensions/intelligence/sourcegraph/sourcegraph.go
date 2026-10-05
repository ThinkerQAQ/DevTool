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

const (
	ExtensionID = "intelligence.sourcegraph"
	toolPrefix   = "sourcegraph_"
)

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
		ID:         ExtensionID,
		Kind:       extensioncontract.KindCodeIntelligence,
		Provides:   []string{codeintelligence.IndexedServiceName},
		AgentTools: e.remote != nil,
	}
}

func (e *Extension) Register(reg extensioncontract.Registrar) error {
	if err := reg.ProvideService(codeintelligence.IndexedServiceName, ExtensionID, service.Func(e.Invoke)); err != nil {
		return err
	}
	if e.remote == nil {
		return nil
	}
	return reg.ProvideAgentTools(ExtensionID, prefixedProvider{remote: e.remote})
}

func (e *Extension) Invoke(ctx context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
	switch method {
	case codeintelligence.MethodDoctor:
		if e.remote == nil {
			return nil, fmt.Errorf("Sourcegraph MCP is not configured; set SOURCEGRAPH_MCP_URL")
		}
		tools, err := e.remote.ListTools(ctx, agentsdk.Session{})
		if err != nil {
			return nil, fmt.Errorf("Sourcegraph MCP doctor: %w", err)
		}
		return json.Marshal(codeintelligence.DoctorResponse{
			Provider:   ExtensionID,
			Executable: e.endpoint,
			Version:    fmt.Sprintf("%d tools", len(tools)),
		})
	case codeintelligence.MethodSync:
		if e.remote == nil {
			return nil, fmt.Errorf("Sourcegraph MCP is not configured; set SOURCEGRAPH_MCP_URL")
		}
		return json.Marshal(codeintelligence.VerifyResponse{
			Provider: ExtensionID,
			Output:   "index lifecycle is managed by Sourcegraph",
		})
	case codeintelligence.MethodQuery:
		if e.remote == nil {
			return nil, fmt.Errorf("Sourcegraph MCP is not configured; set SOURCEGRAPH_MCP_URL")
		}
		var request codeintelligence.IndexedQuery
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, fmt.Errorf("decode Sourcegraph query request: %w", err)
		}
		tool := strings.TrimSpace(request.Tool)
		tool = strings.TrimPrefix(tool, toolPrefix)
		if tool == "" {
			return nil, fmt.Errorf("Sourcegraph query tool is required")
		}
		args := request.Args
		if len(args) == 0 {
			args = json.RawMessage(`{}`)
		}
		if !json.Valid(args) {
			return nil, fmt.Errorf("Sourcegraph query args must be valid JSON")
		}
		return e.remote.CallTool(ctx, agentsdk.Session{}, tool, args)
	default:
		return nil, fmt.Errorf("%s does not support method %q", ExtensionID, method)
	}
}

func sourcegraphAuthorization(token string) string {
	if token == "" {
		return ""
	}
	return "token " + token
}

type prefixedProvider struct {
	remote *mcpbridge.HTTPProvider
}

func (p prefixedProvider) ListTools(ctx context.Context, session agentsdk.Session) ([]agentsdk.Tool, error) {
	tools, err := p.remote.ListTools(ctx, session)
	if err != nil {
		return nil, err
	}
	out := make([]agentsdk.Tool, 0, len(tools))
	for _, tool := range tools {
		var definition map[string]any
		if err := json.Unmarshal(tool.Definition, &definition); err != nil {
			return nil, err
		}
		name := toolPrefix + tool.Name
		definition["name"] = name
		raw, err := json.Marshal(definition)
		if err != nil {
			return nil, err
		}
		out = append(out, agentsdk.Tool{Name: name, Definition: raw})
	}
	return out, nil
}

func (p prefixedProvider) CallTool(ctx context.Context, session agentsdk.Session, name string, args json.RawMessage) (json.RawMessage, error) {
	name = strings.TrimPrefix(strings.TrimSpace(name), toolPrefix)
	if name == "" {
		return nil, fmt.Errorf("Sourcegraph tool name is required")
	}
	return p.remote.CallTool(ctx, session, name, args)
}
