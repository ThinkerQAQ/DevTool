package workspace

import "testing"

func TestProviderDescribeConfiguredWorkspace(t *testing.T) {
	provider := &Provider{}
	if err := provider.Configure(map[string]any{"name": "Docs"}); err != nil {
		t.Fatal(err)
	}

	if got := provider.ExtensionDescriptor().ID; got != ExtensionID {
		t.Fatalf("extension id = %q, want %q", got, ExtensionID)
	}
	project := provider.ProjectDescriptor()
	if got := project.Identity.Name; got != "Docs" {
		t.Fatalf("project name = %q, want Docs", got)
	}
	if len(project.Commands) != 0 {
		t.Fatalf("workspace project should expose no commands, got %d", len(project.Commands))
	}
}

func TestProviderRequiresName(t *testing.T) {
	provider := &Provider{}
	if err := provider.Configure(nil); err == nil {
		t.Fatal("expected missing name to fail")
	}
	if err := provider.Configure(map[string]any{"name": "   "}); err == nil {
		t.Fatal("expected blank name to fail")
	}
}
