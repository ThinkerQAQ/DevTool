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
	ID           string         `toml:"id" json:"id,omitempty"`
	Loader       string         `toml:"loader" json:"loader"`
	LoaderConfig map[string]any `toml:"loader_config" json:"loader_config,omitempty"`
	Settings     map[string]any `toml:"settings" json:"settings,omitempty"`
}

type Service struct {
	Provider string `toml:"provider" json:"provider"`
}

type Code struct {
	Workspaces []string `toml:"workspaces" json:"workspaces,omitempty"`
}

type UI struct {
	Features []string `toml:"features" json:"features"`
}

// AgentPolicy governs optional, model-free workspace mutations at the MCP boundary.
type AgentPolicy struct {
	Enabled           bool     `toml:"enabled" json:"enabled"`
	ProtectedBranches []string `toml:"protected_branches" json:"protected_branches,omitempty"`
	MaxPatchBytes     int      `toml:"max_patch_bytes" json:"max_patch_bytes,omitempty"`
	ContextTTLSeconds int      `toml:"context_ttl_seconds" json:"context_ttl_seconds,omitempty"`
}

type Agent struct {
	Policy AgentPolicy `toml:"policy" json:"policy"`
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
	Code      Code                 `toml:"code" json:"code,omitempty"`
	Agent     Agent                `toml:"agent" json:"agent,omitempty"`
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
		if strings.TrimSpace(ext.Loader) == "" {
			return fmt.Errorf("extension.%s.loader is required", name)
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
	for index, workspace := range cfg.Code.Workspaces {
		if strings.TrimSpace(workspace) == "" {
			return fmt.Errorf("code.workspaces[%d] cannot be empty", index)
		}
	}
	if cfg.Agent.Policy.MaxPatchBytes < 0 || cfg.Agent.Policy.MaxPatchBytes > 1024*1024 {
		return fmt.Errorf("agent.policy.max_patch_bytes must be between 0 and 1048576")
	}
	if cfg.Agent.Policy.ContextTTLSeconds < 0 || cfg.Agent.Policy.ContextTTLSeconds > 3600 {
		return fmt.Errorf("agent.policy.context_ttl_seconds must be between 0 and 3600")
	}
	for _, branch := range cfg.Agent.Policy.ProtectedBranches {
		if strings.TrimSpace(branch) == "" {
			return fmt.Errorf("agent.policy.protected_branches cannot contain an empty branch")
		}
	}
	for index, feature := range cfg.UI.Features {
		if strings.TrimSpace(feature) == "" {
			return fmt.Errorf("ui.features[%d] cannot be empty", index)
		}
	}
	return nil
}
