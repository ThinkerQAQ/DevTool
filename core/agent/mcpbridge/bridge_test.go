package mcpbridge

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"sync/atomic"
	"testing"
	"time"

	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
)

func TestProviderCancelsBlockedChildAndRestarts(t *testing.T) {
	var starts atomic.Int32
	provider := New(func(context.Context, agentsdk.Session) (*exec.Cmd, error) {
		mode := "stall"
		if starts.Add(1) > 1 {
			mode = "fast"
		}
		cmd := exec.Command(os.Args[0], "-test.run=TestMCPBridgeHelperProcess")
		cmd.Env = append(os.Environ(),
			"GO_WANT_MCP_BRIDGE_HELPER=1",
			"MCP_BRIDGE_HELPER_MODE="+mode,
		)
		return cmd, nil
	})
	defer provider.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := provider.CallTool(ctx, agentsdk.Session{ProjectRoot: t.TempDir()}, "slow", nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("CallTool error = %v; want deadline exceeded", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("canceled CallTool took %s", elapsed)
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	raw, err := provider.CallTool(ctx2, agentsdk.Session{ProjectRoot: t.TempDir()}, "fast", nil)
	if err != nil {
		t.Fatalf("CallTool after restart: %v", err)
	}
	if !json.Valid(raw) {
		t.Fatalf("result is not JSON: %s", raw)
	}
	if starts.Load() < 2 {
		t.Fatalf("provider starts = %d; want restart after cancellation", starts.Load())
	}
}

func TestMCPBridgeHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_MCP_BRIDGE_HELPER") != "1" {
		return
	}
	mode := os.Getenv("MCP_BRIDGE_HELPER_MODE")
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	for scanner.Scan() {
		var request struct {
			ID     json.RawMessage `json:"id,omitempty"`
			Method string          `json:"method"`
		}
		if json.Unmarshal(scanner.Bytes(), &request) != nil {
			continue
		}
		switch request.Method {
		case "initialize":
			_ = encoder.Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      request.ID,
				"result": map[string]any{
					"protocolVersion": "2025-06-18",
					"capabilities":    map[string]any{},
				},
			})
		case "notifications/initialized":
		case "tools/call":
			if mode == "stall" {
				select {}
			}
			_ = encoder.Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      request.ID,
				"result": map[string]any{
					"content": []map[string]string{{"type": "text", "text": "ok"}},
				},
			})
		}
	}
	os.Exit(0)
}
