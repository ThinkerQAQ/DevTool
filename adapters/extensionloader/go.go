package extensionloader

import (
	"bytes"
	"crypto/sha256"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"

	"github.com/thinkerqaq/devtool/core/config"
	"github.com/thinkerqaq/devtool/core/project"
)

// Resolve translates configured extension build metadata into an executable.
// This adapter owns Go-specific build/cache behavior so Core only owns the
// extension process protocol and lifecycle.
func Resolve(ctx context.Context, p project.Project, name string, configured config.Extension) (string, error) {
	switch strings.TrimSpace(configured.Loader) {
	case "go":
		return resolveGo(ctx, p, name, configured.LoaderConfig)
	default:
		return "", fmt.Errorf("extension %q loader %q is unsupported", name, configured.Loader)
	}
}

func resolveGo(ctx context.Context, p project.Project, name string, loaderConfig map[string]any) (string, error) {
	var configured struct {
		Module  string `json:"module"`
		Package string `json:"package"`
	}
	raw, err := json.Marshal(loaderConfig)
	if err != nil {
		return "", fmt.Errorf("encode extension.%s.loader_config: %w", name, err)
	}
	if err := json.Unmarshal(raw, &configured); err != nil {
		return "", fmt.Errorf("decode extension.%s.loader_config for go loader: %w", name, err)
	}
	module := strings.TrimSpace(configured.Module)
	pkg := strings.TrimSpace(configured.Package)
	if module == "" {
		return "", fmt.Errorf("extension.%s.loader_config.module is required", name)
	}
	if pkg == "" {
		return "", fmt.Errorf("extension.%s.loader_config.package is required", name)
	}

	moduleDir := filepath.Join(p.Root, filepath.FromSlash(module))
	info, err := os.Stat(moduleDir)
	if err != nil {
		return "", fmt.Errorf("extension %q module %q: %w", name, module, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("extension %q module %q is not a directory", name, module)
	}

	cacheDir := filepath.Join(p.Root, ".devtool", "cache", "extensions")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", err
	}
	outputName := cacheOutputName(name, module, pkg)
	if runtime.GOOS == "windows" {
		outputName += ".exe"
	}
	output := filepath.Join(cacheDir, outputName)

	rebuild, err := goBuildNeedsBuild(ctx, p.Root, moduleDir, pkg, output)
	if err != nil {
		return "", fmt.Errorf("check extension %q build cache: %w", name, err)
	}
	if !rebuild {
		return output, nil
	}

	cmd := exec.CommandContext(ctx, "go", "build", "-o", output, pkg)
	cmd.Dir = moduleDir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("build extension %q: %w", name, err)
	}
	return output, nil
}

func goBuildNeedsBuild(ctx context.Context, projectRoot, moduleDir, pkg, output string) (bool, error) {
	outputInfo, err := os.Stat(output)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, err
	}

	dependencies, moduleRoots, err := goDependencyDirs(ctx, projectRoot, moduleDir, pkg)
	if err != nil {
		// Let go build produce the authoritative compile/dependency error.
		return true, nil
	}

	outputTime := outputInfo.ModTime()
	for _, dir := range dependencies {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return false, err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				return false, err
			}
			if info.ModTime().After(outputTime) {
				return true, nil
			}
		}
	}
	for _, root := range moduleRoots {
		for _, name := range []string{"go.mod", "go.sum"} {
			info, err := os.Stat(filepath.Join(root, name))
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return false, err
			}
			if info.ModTime().After(outputTime) {
				return true, nil
			}
		}
	}
	return false, nil
}

func goDependencyDirs(ctx context.Context, projectRoot, moduleDir, pkg string) ([]string, []string, error) {
	cmd := exec.CommandContext(ctx, "go", "list", "-deps", "-json", pkg)
	cmd.Dir = moduleDir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, nil, fmt.Errorf("go list %s: %w: %s", pkg, err, strings.TrimSpace(stderr.String()))
	}

	projectRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, nil, err
	}
	dependencySet := map[string]struct{}{}
	moduleSet := map[string]struct{}{}
	decoder := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
	for {
		var value struct {
			Dir    string `json:"Dir"`
			Module *struct {
				Dir string `json:"Dir"`
			} `json:"Module"`
		}
		if err := decoder.Decode(&value); err != nil {
			if err == io.EOF {
				break
			}
			return nil, nil, err
		}
		if value.Module == nil || strings.TrimSpace(value.Dir) == "" {
			continue
		}
		if insideProject(projectRoot, value.Dir) {
			dependencySet[filepath.Clean(value.Dir)] = struct{}{}
		}
		if strings.TrimSpace(value.Module.Dir) != "" && insideProject(projectRoot, value.Module.Dir) {
			moduleSet[filepath.Clean(value.Module.Dir)] = struct{}{}
		}
	}
	return sortedKeys(dependencySet), sortedKeys(moduleSet), nil
}

func insideProject(root, path string) bool {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(root, absolute)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func sortedKeys(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	// Determinism is useful for diagnostics and tests; ordering has no semantic
	// effect on cache invalidation.
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func sanitizeName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "extension-provider"
	}
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, name)
}


func cacheOutputName(name, module, pkg string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(module) + "\x00" + strings.TrimSpace(pkg)))
	return fmt.Sprintf("%s-%x", sanitizeName(name), sum[:6])
}
