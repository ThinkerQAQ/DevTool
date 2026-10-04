package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const CurrentVersion = 1

type Project struct {
	Name string `toml:"name" json:"name"`
}

type Extension struct {
	Type    string `toml:"type" json:"type"`
	Source  string `toml:"source" json:"source,omitempty"`
	Module  string `toml:"module" json:"module,omitempty"`
	Package string `toml:"package" json:"package,omitempty"`
}

type Service struct {
	Provider string `toml:"provider" json:"provider"`
}

type Environment struct {
	Profile string `toml:"profile" json:"profile,omitempty"`
	Image   string `toml:"image" json:"image,omitempty"`
}

type Dev struct {
	Environment Environment `toml:"environment" json:"environment"`
}

type Code struct {
	Workspaces []string `toml:"workspaces" json:"workspaces,omitempty"`
}

type UI struct {
	Features []string `toml:"features" json:"features"`
}

type Config struct {
	Version   int                  `toml:"version" json:"version"`
	Project   Project              `toml:"project" json:"project"`
	Extension map[string]Extension `toml:"extension" json:"extension,omitempty"`
	Service   map[string]Service   `toml:"service" json:"service,omitempty"`
	Dev       Dev                  `toml:"dev" json:"dev,omitempty"`
	Code      Code                 `toml:"code" json:"code,omitempty"`
	UI        UI                   `toml:"ui" json:"ui"`
}

func Load(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := toml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := Validate(cfg); err != nil {
		return Config{}, fmt.Errorf("validate %s: %w", path, err)
	}
	return cfg, nil
}

func Validate(cfg Config) error {
	if cfg.Version != CurrentVersion {
		return fmt.Errorf("unsupported config version %d; expected %d", cfg.Version, CurrentVersion)
	}
	if strings.TrimSpace(cfg.Project.Name) == "" {
		return fmt.Errorf("project.name is required")
	}
	for name, ext := range cfg.Extension {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("extension name is required")
		}
		if ext.Type != "go" {
			return fmt.Errorf("extension.%s.type %q is unsupported; configured extensions must use a process-backed loader", name, ext.Type)
		}
		if strings.TrimSpace(ext.Module) == "" {
			return fmt.Errorf("extension.%s.module is required for go extension", name)
		}
		if strings.TrimSpace(ext.Package) == "" {
			return fmt.Errorf("extension.%s.package is required for go extension", name)
		}
	}
	for name, service := range cfg.Service {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("service name is required")
		}
		if strings.TrimSpace(service.Provider) == "" {
			return fmt.Errorf("service.%s.provider is required", name)
		}
	}
	if _, ok := cfg.Service["code-graph"]; ok && strings.TrimSpace(cfg.Dev.Environment.Image) == "" {
		return fmt.Errorf("dev.environment.image is required when code-graph service is configured")
	}
	if _, ok := cfg.Service["code-lsp"]; ok && strings.TrimSpace(cfg.Dev.Environment.Image) == "" {
		return fmt.Errorf("dev.environment.image is required when code-lsp service is configured")
	}
	for index, workspace := range cfg.Code.Workspaces {
		if strings.TrimSpace(workspace) == "" {
			return fmt.Errorf("code.workspaces[%d] cannot be empty", index)
		}
	}
	for index, feature := range cfg.UI.Features {
		if strings.TrimSpace(feature) == "" {
			return fmt.Errorf("ui.features[%d] cannot be empty", index)
		}
	}
	return nil
}
