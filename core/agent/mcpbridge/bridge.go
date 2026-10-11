package mcpbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
	devtooltrace "github.com/thinkerqaq/devtool/sdk/trace"
)

type CommandFactory func(context.Context, agentsdk.Session) (*exec.Cmd, error)

// Provider reuses the official MCP SDK for child-process transports, request
// correlation, protocol negotiation and cancellation.
type Provider struct {
	factory CommandFactory

	lifecycleCtx context.Context
	cancel       context.CancelFunc

	mu        sync.Mutex
	client    *mcp.ClientSession
	sessionID string
	closed    bool
}

func New(factory CommandFactory) *Provider {
	ctx, cancel := context.WithCancel(context.Background())
	return &Provider{factory: factory, lifecycleCtx: ctx, cancel: cancel}
}

func (p *Provider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true
	p.cancel()
	if p.client != nil {
		err := p.client.Close()
		p.client = nil
		p.sessionID = ""
		return err
	}
	return nil
}

func (p *Provider) ListTools(ctx context.Context, session agentsdk.Session) ([]agentsdk.Tool, error) {
	client, err := p.ensureClient(ctx, session)
	if err != nil {
		return nil, err
	}
	var tools []agentsdk.Tool
	for {
		result, err := client.ListTools(ctx, nil)
		if err != nil {
			p.discard(client)
			return nil, err
		}
		for _, tool := range result.Tools {
			raw, err := json.Marshal(tool)
			if err != nil {
				return nil, err
			}
			tools = append(tools, agentsdk.Tool{Name: tool.Name, Definition: raw})
		}
		if result.NextCursor == "" {
			break
		}
		// For paged tool listings, use the explicit cursor rather than first-page caching.
		return p.listPaginated(ctx, client, tools, result.NextCursor)
	}
	return tools, nil
}

func (p *Provider) listPaginated(ctx context.Context, client *mcp.ClientSession, tools []agentsdk.Tool, cursor string) ([]agentsdk.Tool, error) {
	for cursor != "" {
		response, err := client.ListTools(ctx, &mcp.ListToolsParams{Cursor: cursor})
		if err != nil {
			p.discard(client)
			return nil, err
		}
		for _, tool := range response.Tools {
			raw, err := json.Marshal(tool)
			if err != nil {
				return nil, err
			}
			tools = append(tools, agentsdk.Tool{Name: tool.Name, Definition: raw})
		}
		cursor = response.NextCursor
	}
	return tools, nil
}

func (p *Provider) CallTool(ctx context.Context, session agentsdk.Session, name string, args json.RawMessage) (json.RawMessage, error) {
	client, err := p.ensureClient(ctx, session)
	if err != nil {
		return nil, err
	}
	var arguments any = map[string]any{}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &arguments); err != nil {
			return nil, fmt.Errorf("decode tool arguments: %w", err)
		}
	}
	result, err := client.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: arguments})
	if err != nil {
		p.discard(client)
		return nil, err
	}
	return json.Marshal(result)
}

func (p *Provider) discard(client *mcp.ClientSession) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client == client {
		p.client = nil
		p.sessionID = ""
		_ = client.Close()
	}
}

func (p *Provider) ensureClient(ctx context.Context, session agentsdk.Session) (*mcp.ClientSession, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	keyBytes, _ := json.Marshal(session)
	key := string(keyBytes)
	_, span := devtooltrace.Start(ctx, devtooltrace.Attributes{Name: "MCP lock_wait", Layer: "bridge"})
	p.mu.Lock()
	span.End(0, nil)
	defer p.mu.Unlock()
	if p.closed {
		return nil, fmt.Errorf("MCP bridge is closed")
	}
	if p.client != nil && p.sessionID == key {
		return p.client, nil
	}
	if p.client != nil {
		_ = p.client.Close()
		p.client = nil
	}
	_, startup := devtooltrace.Start(ctx, devtooltrace.Attributes{Name: "MCP provider lifecycle", Layer: "bridge", Method: "cold_start"})
	command, err := p.factory(p.lifecycleCtx, session)
	if err != nil {
		startup.End(0, err)
		return nil, err
	}
	command.Stderr = os.Stderr
	client := mcp.NewClient(&mcp.Implementation{Name: "devtool-mcp-bridge", Version: "1"}, nil)
	mcpSession, err := client.Connect(ctx, &mcp.CommandTransport{Command: command, TerminateDuration: 100 * time.Millisecond}, nil)
	if err != nil {
		startup.End(0, err)
		return nil, err
	}
	p.client = mcpSession
	p.sessionID = key
	startup.End(0, nil)
	return mcpSession, nil
}
