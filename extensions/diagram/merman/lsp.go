package merman

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	diagram "github.com/thinkerqaq/devtool/sdk/diagram"
)

type evidence struct {
	Symbols     []diagram.DiagramSymbol
	Diagnostics []diagram.DiagramDiagnostic
}
type rpc struct {
	in  io.Writer
	out *bufio.Reader
}

func (c *rpc) send(id int, method string, params any) error {
	msg := map[string]any{"jsonrpc": "2.0", "method": method, "params": params}
	if id > 0 {
		msg["id"] = id
	}
	return c.write(msg)
}
func (c *rpc) write(msg any) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if _, err = c.in.Write([]byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body)))); err != nil {
		return err
	}
	_, err = c.in.Write(body)
	return err
}
func (c *rpc) next() (json.RawMessage, error) {
	size := -1
	for {
		line, err := c.out.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if ok && strings.EqualFold(strings.TrimSpace(key), "Content-Length") {
			n, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, err
			}
			size = n
		}
	}
	if size <= 0 || size > 8<<20 {
		return nil, fmt.Errorf("invalid LSP frame size %d", size)
	}
	body := make([]byte, size)
	if _, err := io.ReadFull(c.out, body); err != nil {
		return nil, err
	}
	if !json.Valid(body) || !bytes.HasPrefix(bytes.TrimSpace(body), []byte("{")) {
		return nil, fmt.Errorf("invalid LSP message")
	}
	return body, nil
}
func (c *rpc) call(id int, method string, params any) (json.RawMessage, error) {
	if err := c.send(id, method, params); err != nil {
		return nil, err
	}
	for {
		payload, err := c.next()
		if err != nil {
			return nil, err
		}
		var msg struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(payload, &msg); err != nil {
			return nil, err
		}
		if msg.Method != "" {
			if len(msg.ID) > 0 {
				if err := c.write(map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(msg.ID), "result": nil}); err != nil {
					return nil, err
				}
			}
			continue
		}
		if string(msg.ID) == strconv.Itoa(id) {
			if msg.Error != nil {
				return nil, fmt.Errorf("%s: %s", method, msg.Error.Message)
			}
			return msg.Result, nil
		}
	}
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

func flatten(items []lspSymbol, depth int, out *[]diagram.DiagramSymbol) {
	for _, s := range items {
		if len(*out) >= 1000 {
			return
		}
		// The level-0 symbol describes a whole diagram, while child symbols
		// represent actual nodes, participants, states, subgraphs.
		if depth > 0 {
			*out = append(*out, diagram.DiagramSymbol{Name: s.Name, Kind: symbolKind(s.Kind), Line: s.Range.Start.Line + 1})
		}
		flatten(s.Children, depth+1, out)
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
	c := &rpc{in: stdin, out: bufio.NewReader(stdout)}
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
	if _, err := c.call(1, "initialize", init); err != nil {
		return nil, fmt.Errorf("Merman initialize: %w", err)
	}
	if err := c.send(0, "initialized", map[string]any{}); err != nil {
		return nil, err
	}
	if err := c.send(0, "textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": uri, "languageId": "markdown", "version": 1, "text": string(source)}}); err != nil {
		return nil, err
	}
	raw, err := c.call(2, "textDocument/documentSymbol", map[string]any{"textDocument": map[string]string{"uri": uri}})
	if err != nil {
		return nil, fmt.Errorf("Merman document symbols: %w", err)
	}
	var symbols []lspSymbol
	if err := json.Unmarshal(raw, &symbols); err != nil {
		return nil, fmt.Errorf("Merman symbols shape: %w", err)
	}
	ev := &evidence{Symbols: []diagram.DiagramSymbol{}, Diagnostics: []diagram.DiagramDiagnostic{}}
	flatten(symbols, 0, &ev.Symbols)
	diagRaw, err := c.call(3, "textDocument/diagnostic", map[string]any{"textDocument": map[string]string{"uri": uri}})
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
