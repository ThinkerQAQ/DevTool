package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".devtool.toml")
	raw := []byte(`version = 1
[project]
name = "Example"
[extension.project]
type = "go"
module = "./devcontrol"
package = "./cmd/provider"
[extension.runtime]
type = "builtin"
source = "runtime.dagger"
[service.portable-runtime]
provider = "runtime.dagger"
[ui]
features = ["jobs", "logs"]
`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Project.Name != "Example" {
		t.Fatalf("project name = %q", cfg.Project.Name)
	}
	if cfg.Extension["project"].Module != "./devcontrol" {
		t.Fatalf("unexpected project extension: %+v", cfg.Extension["project"])
	}
	if cfg.Service["portable-runtime"].Provider != "runtime.dagger" {
		t.Fatalf("unexpected service provider: %+v", cfg.Service)
	}
}

func TestValidateRejectsGoExtensionWithoutPackage(t *testing.T) {
	cfg := Config{
		Version: CurrentVersion,
		Project: Project{Name: "Example"},
		Extension: map[string]Extension{
			"project": {Type: "go", Module: "./devcontrol"},
		},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("Validate() expected missing package error")
	}
}

func TestValidateRequiresConfiguredEnvironmentForCodeIntelligence(t *testing.T) {
	cfg := Config{
		Version: CurrentVersion,
		Project: Project{Name: "Example"},
		Service: map[string]Service{
			"code-graph": {Provider: "intelligence.codegraph"},
		},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("Validate() expected missing dev.environment.image error")
	}
}
