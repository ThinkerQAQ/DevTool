package project

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/thinkerqaq/devtool/core/contract"
	"github.com/thinkerqaq/devtool/core/extension"
	"github.com/thinkerqaq/devtool/protocol"
)

type Provider interface {
	ExtensionDescriptor() extension.Descriptor
	ProjectDescriptor() contract.ProjectDescriptor
	Execute(context.Context, string, map[string]any, func(protocol.Event)) error
}

type describePayload struct {
	Extension extension.Descriptor       `json:"extension"`
	Project   contract.ProjectDescriptor `json:"project"`
}

func Serve(provider Provider) error {
	return serve(provider, os.Stdin, os.Stdout)
}

func serve(provider Provider, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	encoder := json.NewEncoder(out)

	for scanner.Scan() {
		var envelope protocol.Envelope
		if err := json.Unmarshal(scanner.Bytes(), &envelope); err != nil {
			return fmt.Errorf("decode provider request: %w", err)
		}
		if envelope.Type != protocol.MessageRequest {
			return fmt.Errorf("provider accepts request messages only, got %q", envelope.Type)
		}

		switch envelope.Method {
		case protocol.MethodDescribe:
			payload := describePayload{
				Extension: provider.ExtensionDescriptor(),
				Project:   provider.ProjectDescriptor(),
			}
			if err := reply(encoder, envelope.ID, protocol.MessageResponse, payload); err != nil {
				return err
			}
		case protocol.MethodExecute:
			var request protocol.ExecuteRequest
			if err := json.Unmarshal(envelope.Payload, &request); err != nil {
				if replyErr := reply(encoder, envelope.ID, protocol.MessageError, map[string]string{"message": err.Error()}); replyErr != nil {
					return replyErr
				}
				continue
			}
			emit := func(event protocol.Event) {
				_ = reply(encoder, envelope.ID, protocol.MessageEvent, event)
			}
			err := provider.Execute(context.Background(), request.Command, request.Args, emit)
			if err != nil {
				if replyErr := reply(encoder, envelope.ID, protocol.MessageError, map[string]string{"message": err.Error()}); replyErr != nil {
					return replyErr
				}
				continue
			}
			if err := reply(encoder, envelope.ID, protocol.MessageResponse, map[string]bool{"ok": true}); err != nil {
				return err
			}
		default:
			if err := reply(encoder, envelope.ID, protocol.MessageError, map[string]string{"message": "unknown provider method"}); err != nil {
				return err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

func reply(encoder *json.Encoder, requestID string, kind protocol.MessageType, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if requestID == "" {
		return errors.New("request id is required")
	}
	return encoder.Encode(protocol.Envelope{
		ReplyTo: requestID,
		Type:    kind,
		Payload: raw,
	})
}
