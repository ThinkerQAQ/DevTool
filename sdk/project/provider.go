package project

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	contract "github.com/thinkerqaq/devtool/sdk/contract"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
	"github.com/thinkerqaq/devtool/protocol"
)

type Context struct {
	context.Context
	session *protocol.Session
	replyTo string
}

func (c Context) Emit(kind, message string) error {
	return c.session.Event(c.replyTo, protocol.Event{Kind: kind, Message: message})
}

func (c Context) InvokeService(service, method string, request, response any) error {
	raw, err := json.Marshal(request)
	if err != nil {
		return err
	}
	return c.session.Call(c.Context, protocol.MethodServiceInvoke, protocol.ServiceInvokeRequest{
		Service: service,
		Method:  method,
		Payload: raw,
	}, nil, response)
}

type Provider interface {
	ExtensionDescriptor() extensioncontract.Descriptor
	ProjectDescriptor() contract.ProjectDescriptor
	Execute(Context, string, map[string]any) error
}

type describePayload struct {
	Extension extensioncontract.Descriptor `json:"extension"`
	Project   contract.ProjectDescriptor   `json:"project"`
}

func Serve(provider Provider) error {
	return serve(provider, os.Stdin, os.Stdout)
}

func serve(provider Provider, in io.Reader, out io.Writer) error {
	var session *protocol.Session
	session = protocol.NewSession(in, out, func(ctx context.Context, envelope protocol.Envelope) (any, error) {
		switch envelope.Method {
		case protocol.MethodDescribe:
			return describePayload{
				Extension: provider.ExtensionDescriptor(),
				Project:   provider.ProjectDescriptor(),
			}, nil
		case protocol.MethodExecute:
			var request protocol.ExecuteRequest
			if err := json.Unmarshal(envelope.Payload, &request); err != nil {
				return nil, err
			}
			providerContext := Context{
				Context: ctx,
				session: session,
				replyTo: envelope.ID,
			}
			if err := provider.Execute(providerContext, request.Command, request.Args); err != nil {
				return nil, err
			}
			return map[string]bool{"ok": true}, nil
		default:
			return nil, fmt.Errorf("unknown provider method %q", envelope.Method)
		}
	})
	return session.Wait()
}
