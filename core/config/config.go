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

type Profile struct {
	Extension map[string]Extension `toml:"extension" json:"extension,omitempty"`
	Service   map[string]Service   `toml:"service" json:"service,omitempty"`
}

type Config struct {
	Version   int                  `toml:"version" json:"version"`
	Project   Project              `toml:"project" json:"project"`
	Extension map[string]Extension `toml:"extension" json:"extension,omitempty"`
	Service   map[string]Service   `toml:"service" json:"service,omitempty"`
	Dev       Dev                  `toml:"dev" json:"dev,omitempty"`
	Code      Code                 `toml:"code" json:"code,omitempty"`
	UI        UI                   `toml:"ui" json:"ui"`
	Profile   map[string]Profile   `toml:"profile" json:"profile,omitempty"`
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
	if profiles := strings.TrimSpace(os.Getenv("DEVTOOL_PROFILES")); profiles != "" {
		var err error
		cfg, err = ApplyProfiles(cfg, strings.Split(profiles, ",")...)
		if err != nil {
			return Config{}, err
		}
	}
	if err := Validate(cfg); err != nil {
		return Config{}, fmt.Errorf("validate %s: %w", path, err)
	}
	return cfg, nil
}

func ApplyProfiles(cfg Config, names ...string) (Config, error) {
	for _, rawName := range names {
		name := strings.TrimSpace(rawName)
		if name == "" {
			continue
		}
		profile, ok := cfg.Profile[name]
		if !ok {
			return Config{}, fmt.Errorf("profile %q is not configured", name)
		}
		if cfg.Extension == nil {
			cfg.Extension = map[string]Extension{}
		}
		for key, value := range profile.Extension {
			cfg.Extension[key] = value
		}
		if cfg.Service == nil {
			cfg.Service = map[string]Service{}
		}
		for key, value := range profile.Service {
			cfg.Service[key] = value
		}
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
		if strings.TrimSpace(ext.Type) == "" {
			return fmt.Errorf("extension.%s.type is required", name)
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
	if _, ok := cfg.Service["code-indexed"]; ok && strings.TrimSpace(cfg.Dev.Environment.Image) == "" {
		return fmt.Errorf("dev.environment.image is required when code-indexed service is configured")
	}
	if _, ok := cfg.Service["code-realtime"]; ok && strings.TrimSpace(cfg.Dev.Environment.Image) == "" {
		return fmt.Errorf("dev.environment.image is required when code-realtime service is configured")
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
