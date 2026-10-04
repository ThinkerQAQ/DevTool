package protocol

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func TestSessionSupportsNestedCalls(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()

	var server *Session
	server = NewSession(right, right, func(ctx context.Context, request Envelope) (any, error) {
		if request.Method != "outer" {
			return nil, io.EOF
		}
		var inner string
		if err := server.Call(ctx, "inner", map[string]string{"value": "request"}, nil, &inner); err != nil {
			return nil, err
		}
		return map[string]string{"value": inner}, nil
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
}

func TestSessionPropagatesCancellationThroughNestedCalls(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()

	innerStarted := make(chan struct{})
	innerCanceled := make(chan struct{})

	var server *Session
	server = NewSession(right, right, func(ctx context.Context, request Envelope) (any, error) {
		if request.Method != "outer" {
			return nil, io.EOF
		}
		if err := server.Call(ctx, "inner", nil, nil, nil); err != nil {
			return nil, err
		}
		return nil, nil
	})

	client := NewSession(left, left, func(ctx context.Context, request Envelope) (any, error) {
		if request.Method != "inner" {
			return nil, io.EOF
		}
		close(innerStarted)
		<-ctx.Done()
		close(innerCanceled)
		return nil, ctx.Err()
	})

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- client.Call(ctx, "outer", nil, nil, nil)
	}()

	select {
	case <-innerStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("nested request did not start")
	}

	cancel()

	select {
	case <-innerCanceled:
	case <-time.After(2 * time.Second):
		t.Fatal("cancellation did not reach nested request")
	}

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Call() error = %v, want context canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("outer call did not return after cancellation")
	}
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
