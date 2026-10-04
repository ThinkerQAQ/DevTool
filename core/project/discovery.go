package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/thinkerqaq/devtool/core/config"
)

const ConfigFileName = ".devtool.toml"

var ErrNotFound = errors.New("devtool project not found")

type Project struct {
	Root       string
	ConfigPath string
	Config     config.Config
}

func Discover(start string) (Project, error) {
	if start == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return Project{}, err
		}
		start = cwd
	}

	absolute, err := filepath.Abs(start)
	if err != nil {
		return Project{}, err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return Project{}, err
	}
	if !info.IsDir() {
		absolute = filepath.Dir(absolute)
	}

	current := filepath.Clean(absolute)
	for {
		path := filepath.Join(current, ConfigFileName)
		if info, statErr := os.Stat(path); statErr == nil && !info.IsDir() {
			cfg, loadErr := config.Load(path)
			if loadErr != nil {
				return Project{}, loadErr
			}
			return Project{Root: current, ConfigPath: path, Config: cfg}, nil
		} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return Project{}, statErr
		}

		parent := filepath.Dir(current)
		if parent == current {
			return Project{}, fmt.Errorf("%w from %s", ErrNotFound, absolute)
		}
		current = parent
	}
}
