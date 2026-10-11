package merman

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"path/filepath"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
	diagram "github.com/thinkerqaq/devtool/sdk/diagram"
)

type evidence struct {
	Truncated   bool
	Symbols     []diagram.DiagramSymbol
	Diagnostics []diagram.DiagramDiagnostic
}
type lspRange struct {
	Start struct {
		Line int `json:"line"`
	} `json:"start"`
	End struct {
		Line int `json:"line"`
	} `json:"end"`
}
type lspSymbol struct {
	Name     string      `json:"name"`
	Kind     int         `json:"kind"`
	Range    lspRange    `json:"range"`
	Children []lspSymbol `json:"children"`
}

func flatten(items []lspSymbol, depth int, out *[]diagram.DiagramSymbol, truncated *bool) {
	for _, s := range items {
		if len(*out) >= 1000 {
			*truncated = true
			return
		}
		// The level-0 symbol describes a whole diagram, while child symbols
		// represent actual nodes, participants, states, subgraphs.
		if depth > 0 {
			*out = append(*out, diagram.DiagramSymbol{Name: s.Name, Kind: symbolKind(s.Kind), Line: s.Range.Start.Line + 1})
		}
		flatten(s.Children, depth+1, out, truncated)
	}
}
func symbolKind(kind int) string {
	switch kind {
	case 2:
		return "node"
	case 3:
		return "subgraph"
	case 5:
		return "state"
	case 24:
		return "participant"
	default:
		return fmt.Sprintf("symbol-%d", kind)
	}
}
func (e *Extension) language(ctx context.Context, root, path string, source []byte) (*evidence, error) {
	bin, err := exec.LookPath(e.lsp)
	if err != nil {
		return nil, fmt.Errorf("merman-lsp executable missing; install explicitly")
	}
	run, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()
	cmd := exec.CommandContext(run, bin)
	cmd.Dir = root
	cmd.Stderr = io.Discard
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	// The mature JSON-RPC SDK owns LSP Content-Length framing, request IDs,
	// concurrent notifications and server callbacks. Stop the child before
	// closing the client to release the reader goroutine deterministically.
	client := jrpc2.NewClient(channel.LSP(stdout, stdin), &jrpc2.ClientOptions{
		OnCallback: func(context.Context, *jrpc2.Request) (any, error) {
			// Match the previous protocol bridge's empty callback response.
			return nil, nil
		},
	})
	defer func() { _ = cmd.Process.Kill(); _ = client.Close() }()
	rootURI := (&url.URL{Scheme: "file", Path: filepath.ToSlash(root)}).String()
	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
	init := map[string]any{
		"processId": nil, "rootUri": rootURI,
		"capabilities": map[string]any{
			"textDocument": map[string]any{"documentSymbol": map[string]any{"hierarchicalDocumentSymbolSupport": true}, "diagnostic": map[string]any{}},
			"workspace":    map[string]any{"workspaceFolders": true},
		},
		"workspaceFolders": []map[string]string{{"uri": rootURI, "name": filepath.Base(root)}},
	}
	var initializeResult json.RawMessage
	if err := client.CallResult(run, "initialize", init, &initializeResult); err != nil {
		return nil, fmt.Errorf("Merman initialize: %w", err)
	}
	if err := client.Notify(run, "initialized", map[string]any{}); err != nil {
		return nil, err
	}
	if err := client.Notify(run, "textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": uri, "languageId": "markdown", "version": 1, "text": string(source)}}); err != nil {
		return nil, err
	}
	var raw json.RawMessage
	err = client.CallResult(run, "textDocument/documentSymbol", map[string]any{"textDocument": map[string]string{"uri": uri}}, &raw)
	if err != nil {
		return nil, fmt.Errorf("Merman document symbols: %w", err)
	}
	var symbols []lspSymbol
	if err := json.Unmarshal(raw, &symbols); err != nil {
		return nil, fmt.Errorf("Merman symbols shape: %w", err)
	}
	ev := &evidence{Symbols: []diagram.DiagramSymbol{}, Diagnostics: []diagram.DiagramDiagnostic{}}
	flatten(symbols, 0, &ev.Symbols, &ev.Truncated)
	var diagRaw json.RawMessage
	err = client.CallResult(run, "textDocument/diagnostic", map[string]any{"textDocument": map[string]string{"uri": uri}}, &diagRaw)
	if err != nil {
		return nil, fmt.Errorf("Merman diagnostics: %w", err)
	}
	var report struct {
		Kind  string `json:"kind"`
		Items []struct {
			Message  string   `json:"message"`
			Severity int      `json:"severity"`
			Range    lspRange `json:"range"`
		} `json:"items"`
	}
	if err := json.Unmarshal(diagRaw, &report); err != nil {
		return nil, fmt.Errorf("Merman diagnostics shape: %w", err)
	}
	if report.Kind != "full" && report.Kind != "unchanged" {
		return nil, fmt.Errorf("Merman diagnostics unrecognized status %q", report.Kind)
	}
	for _, v := range report.Items {
		if len(ev.Diagnostics) >= 300 {
			return nil, fmt.Errorf("Merman diagnostics exceeded 300 entries")
		}
		ev.Diagnostics = append(ev.Diagnostics, diagram.DiagramDiagnostic{Message: v.Message, Severity: v.Severity, Line: v.Range.Start.Line + 1})
	}
	return ev, nil
}
