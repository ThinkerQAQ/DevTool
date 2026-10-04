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
source = "./devcontrol"
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
	if cfg.Service["portable-runtime"].Provider != "runtime.dagger" {
		t.Fatalf("unexpected service provider: %+v", cfg.Service)
	}
}
