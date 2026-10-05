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
type = "go"
module = "."
package = "./extensions/runtime/dagger/cmd/provider"
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

func TestValidateRequiresExtensionType(t *testing.T) {
	cfg := Config{
		Version: CurrentVersion,
		Project: Project{Name: "Example"},
		Extension: map[string]Extension{
			"project": {},
		},
	}
	if err := Validate(cfg); err == nil {
		t.Fatal("Validate() expected missing extension type error")
	}
}


func TestApplyProfileOverridesExtensionAndService(t *testing.T) {
	cfg := Config{
		Version: CurrentVersion,
		Project: Project{Name: "Example"},
		Extension: map[string]Extension{
			"environment": {Type: "go", Package: "./extensions/environment/docker/cmd/provider"},
		},
		Service: map[string]Service{
			"environment": {Provider: "environment.docker"},
		},
		Profile: map[string]Profile{
			"railway": {
				Extension: map[string]Extension{
					"environment": {Type: "go", Package: "./extensions/environment/local/cmd/provider"},
				},
				Service: map[string]Service{
					"environment": {Provider: "environment.local"},
				},
			},
		},
	}
	got, err := ApplyProfiles(cfg, "railway")
	if err != nil {
		t.Fatal(err)
	}
	if got.Extension["environment"].Package != "./extensions/environment/local/cmd/provider" {
		t.Fatalf("extension override = %+v", got.Extension["environment"])
	}
	if got.Service["environment"].Provider != "environment.local" {
		t.Fatalf("service override = %+v", got.Service["environment"])
	}
}

func TestApplyProfileRejectsUnknownProfile(t *testing.T) {
	cfg := Config{Profile: map[string]Profile{}}
	if _, err := ApplyProfiles(cfg, "missing"); err == nil {
		t.Fatal("ApplyProfiles() expected unknown profile error")
	}
}


func TestApplyProfilesComposesInOrder(t *testing.T) {
	cfg := Config{
		Version: CurrentVersion,
		Project: Project{Name: "Example"},
		Service: map[string]Service{
			"environment": {Provider: "environment.docker"},
			"code-indexed": {Provider: "intelligence.codegraph"},
		},
		Profile: map[string]Profile{
			"railway": {
				Service: map[string]Service{
					"environment": {Provider: "environment.local"},
				},
			},
			"sourcegraph": {
				Service: map[string]Service{
					"code-indexed": {Provider: "intelligence.sourcegraph"},
				},
			},
		},
	}
	got, err := ApplyProfiles(cfg, "railway", "sourcegraph")
	if err != nil {
		t.Fatal(err)
	}
	if got.Service["environment"].Provider != "environment.local" {
		t.Fatalf("environment provider = %q", got.Service["environment"].Provider)
	}
	if got.Service["code-indexed"].Provider != "intelligence.sourcegraph" {
		t.Fatalf("indexed provider = %q", got.Service["code-indexed"].Provider)
	}
}


func TestLoadExtensionSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".devtool.toml")
	raw := []byte(`version = 1
[project]
name = "Example"
[extension.environment]
type = "go"
module = "."
package = "./provider"
[extension.environment.settings]
image = "example.invalid/dev-base:1"
`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Extension["environment"].Settings["image"]; got != "example.invalid/dev-base:1" {
		t.Fatalf("extension image setting = %#v", got)
	}
}
