package project

import (
	"bytes"
	"context"
	"encoding/json"
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

func (fakeProvider) Execute(_ context.Context, command string, _ map[string]any, emit func(protocol.Event)) error {
	emit(protocol.Event{Kind: "log", Message: command})
	return nil
}

func TestServeDescribe(t *testing.T) {
	request, _ := json.Marshal(protocol.Envelope{ID: "1", Type: protocol.MessageRequest, Method: protocol.MethodDescribe})
	var out bytes.Buffer
	if err := serve(fakeProvider{}, strings.NewReader(string(request)+"\n"), &out); err != nil {
		t.Fatal(err)
	}
	var response protocol.Envelope
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &response); err != nil {
		t.Fatal(err)
	}
	if response.Type != protocol.MessageResponse || response.ReplyTo != "1" {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestServeExecuteEmitsEventAndResponse(t *testing.T) {
	payload, _ := json.Marshal(protocol.ExecuteRequest{Command: "ping"})
	request, _ := json.Marshal(protocol.Envelope{ID: "1", Type: protocol.MessageRequest, Method: protocol.MethodExecute, Payload: payload})
	var out bytes.Buffer
	if err := serve(fakeProvider{}, strings.NewReader(string(request)+"\n"), &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d protocol frames, want 2", len(lines))
	}
}
