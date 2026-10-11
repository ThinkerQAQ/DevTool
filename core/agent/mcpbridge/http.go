package mcpbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
)

// HTTPProvider delegates Streamable HTTP framing and session negotiation to
// the official MCP SDK. The provided authorization value is forwarded unchanged.
type HTTPProvider struct {
	endpoint      string
	authorization string
	mu            sync.Mutex
	session       *mcp.ClientSession
	closed        bool
}

func NewHTTP(endpoint, authorization string) *HTTPProvider {
	return &HTTPProvider{endpoint: strings.TrimSpace(endpoint), authorization: strings.TrimSpace(authorization)}
}

type authorizationTransport struct {
	value string
	base  http.RoundTripper
}

func (t authorizationTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	next := request.Clone(request.Context())
	if t.value != "" {
		next.Header.Set("Authorization", t.value)
	}
	return t.base.RoundTrip(next)
}

func (p *HTTPProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	if p.session != nil {
		err := p.session.Close()
		p.session = nil
		return err
	}
	return nil
}

func (p *HTTPProvider) connect(ctx context.Context) (*mcp.ClientSession, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil, fmt.Errorf("remote MCP provider is closed")
	}
	if p.session != nil {
		return p.session, nil
	}
	if p.endpoint == "" {
		return nil, fmt.Errorf("remote MCP endpoint is required")
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "devtool-remote-mcp", Version: "1"}, nil)
	transport := &mcp.StreamableClientTransport{
		Endpoint:             p.endpoint,
		HTTPClient:           &http.Client{Timeout: 30 * time.Second, Transport: authorizationTransport{value: p.authorization, base: http.DefaultTransport}},
		DisableStandaloneSSE: true,
		MaxRetries:           -1,
	}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("connect remote MCP: %w", err)
	}
	p.session = session
	return session, nil
}

func (p *HTTPProvider) reset(session *mcp.ClientSession) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.session == session {
		p.session = nil
		_ = session.Close()
	}
}

func (p *HTTPProvider) ListTools(ctx context.Context, _ agentsdk.Session) ([]agentsdk.Tool, error) {
	session, err := p.connect(ctx)
	if err != nil {
		return nil, err
	}
	var tools []agentsdk.Tool
	cursor := ""
	for {
		result, err := session.ListTools(ctx, &mcp.ListToolsParams{Cursor: cursor})
		if err != nil {
			p.reset(session)
			return nil, err
		}
		for _, tool := range result.Tools {
			raw, err := json.Marshal(tool)
			if err != nil {
				return nil, err
			}
			tools = append(tools, agentsdk.Tool{Name: tool.Name, Definition: raw})
		}
		cursor = result.NextCursor
		if cursor == "" {
			break
		}
	}
	return tools, nil
}

func (p *HTTPProvider) CallTool(ctx context.Context, _ agentsdk.Session, name string, args json.RawMessage) (json.RawMessage, error) {
	session, err := p.connect(ctx)
	if err != nil {
		return nil, err
	}
	var arguments any = map[string]any{}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &arguments); err != nil {
			return nil, fmt.Errorf("decode remote MCP args: %w", err)
		}
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: arguments})
	if err != nil {
		p.reset(session)
		return nil, err
	}
	return json.Marshal(result)
}
