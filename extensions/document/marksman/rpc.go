package marksman

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// stdioClient is an internal JSON-RPC transport; it is not an Agent capability.
type stdioClient struct {
	stdin               io.Writer
	out                 *bufio.Reader
	diagnostics         json.RawMessage
	diagnosticsReported bool
}

const maxMessage = 8 << 20

func (c *stdioClient) write(v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = c.stdin.Write([]byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))))
	if err != nil {
		return err
	}
	_, err = c.stdin.Write(body)
	return err
}
func (c *stdioClient) notify(method string, params any) error {
	return c.write(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}
func (c *stdioClient) request(id int, method string, params any) (json.RawMessage, error) {
	if err := c.write(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
		return nil, err
	}
	for {
		payload, err := c.read()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", method, err)
		}
		var msg struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
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
			if msg.Method == "textDocument/publishDiagnostics" {
				var n struct {
					Diagnostics json.RawMessage `json:"diagnostics"`
				}
				if json.Unmarshal(msg.Params, &n) == nil {
					c.diagnostics = n.Diagnostics
					c.diagnosticsReported = true
				}
			}
			// Respond to server-initiated requests to avoid blocking the language server.
			if len(msg.ID) > 0 {
				if err := c.write(map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(msg.ID), "result": nil}); err != nil {
					return nil, err
				}
			}
			continue
		}
		if string(msg.ID) == strconv.Itoa(id) {
			if msg.Error != nil {
				return nil, fmt.Errorf("%s LSP %d: %s", method, msg.Error.Code, msg.Error.Message)
			}
			return msg.Result, nil
		}
	}
}
func (c *stdioClient) read() ([]byte, error) {
	length := -1
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
			size, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, err
			}
			length = size
		}
	}
	if length < 0 || length > maxMessage {
		return nil, fmt.Errorf("invalid LSP message size %d", length)
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(c.out, body); err != nil {
		return nil, err
	}
	if !json.Valid(body) || !bytes.HasPrefix(bytes.TrimSpace(body), []byte("{")) {
		return nil, fmt.Errorf("invalid LSP JSON-RPC message")
	}
	return body, nil
}
