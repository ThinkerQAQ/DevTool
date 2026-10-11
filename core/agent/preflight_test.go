package agent

import (
	"strings"
	"testing"

	"github.com/thinkerqaq/devtool/core/registry"
	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
)

func TestGatewayPreflightUsesActualMCPToolRegistration(t *testing.T) {
	reg := registry.New()
	if err := reg.ProvideAgentTools("code.one", fakeProvider{name: "code_context"}); err != nil {
		t.Fatal(err)
	}
	gateway := NewGateway(reg, agentsdk.Session{ProjectRoot: t.TempDir()})
	count, err := gateway.Preflight(t.Context())
	if err != nil || count != 1 {
		t.Fatalf("valid preflight: count=%d err=%v", count, err)
	}
}

func TestGatewayPreflightRejectsAgentToolCollision(t *testing.T) {
	reg := registry.New()
	for _, id := range []string{"scm.legacy", "scm.capability"} {
		if err := reg.ProvideAgentTools(id, fakeProvider{name: "scm_publish"}); err != nil {
			t.Fatal(err)
		}
	}
	count, err := NewGateway(reg, agentsdk.Session{ProjectRoot: t.TempDir()}).Preflight(t.Context())
	if count != 0 || err == nil || !strings.Contains(err.Error(), "agent tool \"scm_publish\" is exposed by multiple extensions") {
		t.Fatalf("collision should fail before MCP starts: count=%d err=%v", count, err)
	}
}
