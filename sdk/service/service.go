package service

import (
	"context"
	"encoding/json"
)

type Invoker interface {
	Invoke(context.Context, string, json.RawMessage) (json.RawMessage, error)
}

type Func func(context.Context, string, json.RawMessage) (json.RawMessage, error)

func (f Func) Invoke(ctx context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
	return f(ctx, method, payload)
}
