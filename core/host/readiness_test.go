package host

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/thinkerqaq/devtool/core/registry"
	coreservice "github.com/thinkerqaq/devtool/core/service"
	"github.com/thinkerqaq/devtool/sdk/readiness"
)

type readinessCheckerFunc func(context.Context, readiness.Request) (readiness.Report, error)

func (f readinessCheckerFunc) CheckReadiness(ctx context.Context, request readiness.Request) (readiness.Report, error) {
	return f(ctx, request)
}

func TestReadinessProvidersIncludesOnlySelectedServiceProvider(t *testing.T) {
	reg := registry.New()
	invoker := coreservice.Func(func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
		return nil, nil
	})
	if err := reg.ProvideService("code-indexed", "intelligence.codegraph", invoker); err != nil {
		t.Fatal(err)
	}
	if err := reg.ProvideService("code-indexed", "intelligence.sourcegraph", invoker); err != nil {
		t.Fatal(err)
	}
	if err := reg.SelectService("code-indexed", "intelligence.codegraph"); err != nil {
		t.Fatal(err)
	}

	h := &ProjectHost{
		Registry: reg,
		readiness: []ReadinessProviderEntry{
			{
				ExtensionID: "intelligence.codegraph",
				Provides:    []string{"code-indexed"},
				Checker: readinessCheckerFunc(func(context.Context, readiness.Request) (readiness.Report, error) {
					return readiness.Report{Provider: "intelligence.codegraph", Ready: true}, nil
				}),
			},
			{
				ExtensionID: "intelligence.sourcegraph",
				Provides:    []string{"code-indexed"},
				Checker: readinessCheckerFunc(func(context.Context, readiness.Request) (readiness.Report, error) {
					return readiness.Report{Provider: "intelligence.sourcegraph", Ready: true}, nil
				}),
			},
		},
	}

	got := h.ReadinessProviders()
	if len(got) != 1 {
		t.Fatalf("ReadinessProviders() len = %d, want 1", len(got))
	}
	if got[0].ExtensionID != "intelligence.codegraph" {
		t.Fatalf("ReadinessProviders()[0] = %q, want intelligence.codegraph", got[0].ExtensionID)
	}
}
