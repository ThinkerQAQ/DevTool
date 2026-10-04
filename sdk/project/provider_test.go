package project

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"strings"
	"testing"

	"github.com/thinkerqaq/devtool/core/contract"
	"github.com/thinkerqaq/devtool/core/extension"
	"github.com/thinkerqaq/devtool/protocol"
)

type fakeProvider struct{}

func (fakeProvider) ExtensionDescriptor() extension.Descriptor {
	return extension.Descriptor{ID: "project.fake", Kind: extension.KindProject}
}

func (fakeProvider) ProjectDescriptor() contract.ProjectDescriptor {
	return contract.ProjectDescriptor{
		Identity: contract.ProjectIdentity{Name: "Fake"},
		Commands: []contract.CommandDescriptor{
			{ID: "ping", Title: "Ping", SideEffect: contract.SideEffectRead},
		},
	}
}

func (fakeProvider) Execute(ctx Context, command string, _ map[string]any) error {
	return ctx.Emit("log", command)
}

func TestServeDescribe(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()

	go func() {
		_ = serve(fakeProvider{}, right, right)
	}()

	client := protocol.NewSession(left, left, nil)
	var response describePayload
	if err := client.Call(context.Background(), protocol.MethodDescribe, nil, nil, &response); err != nil {
		t.Fatal(err)
	}
	if response.Extension.ID != "project.fake" {
		t.Fatalf("extension id = %q", response.Extension.ID)
	}
	_ = left.Close()
}

func TestServeExecuteEmitsEvent(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()

	go func() {
		_ = serve(fakeProvider{}, right, right)
	}()

	client := protocol.NewSession(left, left, nil)
	var message string
	var result map[string]bool
	if err := client.Call(context.Background(), protocol.MethodExecute, protocol.ExecuteRequest{Command: "ping"}, func(event protocol.Event) {
		message = event.Message
	}, &result); err != nil {
		t.Fatal(err)
	}
	if message != "ping" {
		t.Fatalf("event = %q, want ping", message)
	}
	if !result["ok"] {
		t.Fatalf("result = %#v", result)
	}
}

func TestContextInvokeService(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()

	_ = protocol.NewSession(left, left, func(_ context.Context, envelope protocol.Envelope) (any, error) {
		if envelope.Method != protocol.MethodServiceInvoke {
			return nil, io.EOF
		}
		var req protocol.ServiceInvokeRequest
		if err := json.Unmarshal(envelope.Payload, &req); err != nil {
			return nil, err
		}
		return map[string]string{"value": req.Service + ":" + req.Method}, nil
	})
	server := protocol.NewSession(right, right, nil)

	ctx := Context{Context: context.Background(), session: server}
	var response map[string]string
	if err := ctx.InvokeService("portable-runtime", "doctor", struct{}{}, &response); err != nil {
		t.Fatal(err)
	}
	if response["value"] != "portable-runtime:doctor" {
		t.Fatalf("response = %#v", response)
	}
}

func TestProviderProtocolIsLineDelimitedJSON(t *testing.T) {
	request, _ := json.Marshal(protocol.Envelope{ID: "1", Type: protocol.MessageRequest, Method: protocol.MethodDescribe})
	var out bytes.Buffer
	reader, writer := io.Pipe()
	done := make(chan error, 1)
	go func() {
		done <- serve(fakeProvider{}, reader, &out)
	}()
	_, _ = writer.Write(append(request, '\n'))
	_ = writer.Close()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(out.String(), "\n") {
		t.Fatalf("protocol output is not line-delimited: %q", out.String())
	}
}
