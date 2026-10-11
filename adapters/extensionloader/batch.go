package extensionloader

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/thinkerqaq/devtool/core/config"
	"github.com/thinkerqaq/devtool/core/project"
)

// dependencySnapshot is scoped to one OpenProject call. It preserves the
// package-specific transitive dependency set used by goBuildNeedsBuild.
type dependencySnapshot struct {
	dirs    []string
	modules []string
}

// NewResolver batches dependency discovery for local Go extensions inside one
// project opening. No metadata is persisted across OpenProject calls, so new
// sessions re-check edits and module changes as before.
func NewResolver() func(context.Context, project.Project, string, config.Extension) (string, error) {
	var once sync.Once
	var snapshots map[string]dependencySnapshot
	return func(ctx context.Context, p project.Project, name string, configured config.Extension) (string, error) {
		if strings.TrimSpace(configured.Loader) != "go" {
			return Resolve(ctx, p, name, configured)
		}
		once.Do(func() { snapshots = batchLocalDependencies(ctx, p) })
		module, _ := configured.LoaderConfig["module"].(string)
		pkg, _ := configured.LoaderConfig["package"].(string)
		key := dependencyKey(filepath.Join(p.Root, filepath.FromSlash(strings.TrimSpace(module))), strings.TrimSpace(pkg))
		snapshot, ok := snapshots[key]
		if !ok {
			return Resolve(ctx, p, name, configured)
		}
		return resolveGoWithSnapshot(ctx, p, name, configured.LoaderConfig, &snapshot)
	}
}

func dependencyKey(moduleDir, pkg string) string {
	return filepath.Clean(moduleDir) + "\x00" + strings.TrimSpace(pkg)
}

func batchLocalDependencies(ctx context.Context, p project.Project) map[string]dependencySnapshot {
	groups := map[string][]string{}
	for name, ext := range p.Config.Extension {
		if strings.TrimSpace(ext.Loader) != "go" {
			continue
		}
		// Match OpenProject selection: an explicit ID is active only if a
		// service binding selects it. Project Extension is always loaded.
		if name != "project" && strings.TrimSpace(ext.ID) != "" {
			selected := false
			for _, s := range p.Config.Service {
				if strings.TrimSpace(s.Provider) == strings.TrimSpace(ext.ID) {
					selected = true
					break
				}
			}
			if !selected {
				continue
			}
		}
		module, moduleOK := ext.LoaderConfig["module"].(string)
		pkg, packageOK := ext.LoaderConfig["package"].(string)
		if !moduleOK || !packageOK || strings.TrimSpace(module) == "" || strings.TrimSpace(pkg) == "" {
			continue // original per-extension resolver will report validation errors
		}
		dir := filepath.Join(p.Root, filepath.FromSlash(strings.TrimSpace(module)))
		groups[dir] = append(groups[dir], strings.TrimSpace(pkg))
	}
	results := map[string]dependencySnapshot{}
	for moduleDir, pkgs := range groups {
		if len(pkgs) < 2 {
			continue // original single-package query is already optimal
		}
		slices.Sort(pkgs)
		pkgs = slices.Compact(pkgs)
		snapshots, err := goDependencyDirsBatch(ctx, p.Root, moduleDir, pkgs)
		if err != nil {
			continue // don't mask authoritative individual go list/build failures
		}
		for pkg, snapshot := range snapshots {
			results[dependencyKey(moduleDir, pkg)] = snapshot
		}
	}
	return results
}

func goDependencyDirsBatch(ctx context.Context, projectRoot, moduleDir string, packages []string) (map[string]dependencySnapshot, error) {
	args := append([]string{"list", "-deps", "-json"}, packages...)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = moduleDir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, err
	}
	type entry struct {
		ImportPath string    `json:"ImportPath"`
		Dir        string    `json:"Dir"`
		Match      []string  `json:"Match"`
		Deps       []string  `json:"Deps"`
		Error      *struct{} `json:"Error"`
		Module     *struct {
			Dir string `json:"Dir"`
		} `json:"Module"`
	}
	entries := map[string]entry{}
	roots := map[string]string{} // exact requested package argument -> import path
	decoder := json.NewDecoder(&stdout)
	for {
		var pkg entry
		if err := decoder.Decode(&pkg); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if pkg.Error != nil {
			return nil, fmt.Errorf("go list returned an incomplete dependency graph")
		}
		entries[pkg.ImportPath] = pkg
		for _, pattern := range pkg.Match {
			for _, requested := range packages {
				if pattern == requested {
					roots[requested] = pkg.ImportPath
				}
			}
		}
	}
	out := map[string]dependencySnapshot{}
	for _, requested := range packages {
		importPath, ok := roots[requested]
		if !ok {
			continue // fallback to independent go list for unusual patterns
		}
		target := entries[importPath]
		dirs := map[string]struct{}{}
		modules := map[string]struct{}{}
		for _, depName := range append(append([]string(nil), target.Deps...), importPath) {
			pkg, present := entries[depName]
			if !present || pkg.Module == nil || pkg.Dir == "" {
				continue
			}
			if insideProject(root, pkg.Dir) {
				dirs[filepath.Clean(pkg.Dir)] = struct{}{}
			}
			if pkg.Module.Dir != "" && insideProject(root, pkg.Module.Dir) {
				modules[filepath.Clean(pkg.Module.Dir)] = struct{}{}
			}
		}
		out[requested] = dependencySnapshot{dirs: sortedKeys(dirs), modules: sortedKeys(modules)}
	}
	return out, nil
}
