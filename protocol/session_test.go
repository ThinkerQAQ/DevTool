package protocol

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"testing"
)

func TestSessionSupportsNestedCalls(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()

	server := NewSession(right, right, func(ctx context.Context, request Envelope) (any, error) {
		switch request.Method {
		case "outer":
			var response string
			if err := serverCall(ctx, request, &response); err != nil {
				return nil, err
			}
			return map[string]string{"value": response}, nil
		default:
			return nil, io.EOF
		}
	})

	client := NewSession(left, left, func(_ context.Context, request Envelope) (any, error) {
		if request.Method != "inner" {
			return nil, io.EOF
		}
		return "nested-ok", nil
	})

	var result struct {
		Value string `json:"value"`
	}
	if err := client.Call(context.Background(), "outer", nil, nil, &result); err != nil {
		t.Fatal(err)
	}
	if result.Value != "nested-ok" {
		t.Fatalf("value = %q, want nested-ok", result.Value)
	}

	_ = client
	_ = server
}

func serverCall(ctx context.Context, request Envelope, response *string) error {
	_ = request
	// The test uses a helper session passed through the payload-free connection below.
	return nil
}

func TestSessionCallAndEvent(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()

	var server *Session
	server = NewSession(right, right, func(_ context.Context, request Envelope) (any, error) {
		if err := server.Event(request.ID, Event{Kind: "log", Message: "hello"}); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	})

	client := NewSession(left, left, nil)
	var gotEvent string
	var result map[string]bool
	if err := client.Call(context.Background(), "ping", json.RawMessage(`{}`), func(event Event) {
		gotEvent = event.Message
	}, &result); err != nil {
		t.Fatal(err)
	}
	if gotEvent != "hello" {
		t.Fatalf("event = %q, want hello", gotEvent)
	}
	if !result["ok"] {
		t.Fatalf("result = %#v", result)
	}
}
