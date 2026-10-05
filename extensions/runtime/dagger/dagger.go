package dagger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	service "github.com/thinkerqaq/devtool/sdk/service"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
	"github.com/thinkerqaq/devtool/sdk/portable"
)

const (
	ExtensionID          = "runtime.dagger"
	defaultDaggerVersion = "1.0.0-beta.15"
)

type Extension struct {
	mu         sync.Mutex
	executable string
	version    string
}

func New() *Extension {
	return &Extension{
		executable: "dagger",
		version:    defaultDaggerVersion,
	}
}

func (e *Extension) Descriptor() extensioncontract.Descriptor {
	return extensioncontract.Descriptor{
		ID:       ExtensionID,
		Kind:     extensioncontract.KindRuntime,
		Provides: []string{portable.ServiceName},
	}
}

func (e *Extension) Register(reg extensioncontract.Registrar) error {
	return reg.ProvideService(portable.ServiceName, ExtensionID, service.Func(e.Invoke))
}

func (e *Extension) Invoke(ctx context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
	switch method {
	case portable.MethodDoctor:
		response, err := e.doctor(ctx)
		if err != nil {
			return nil, err
		}
		return json.Marshal(response)
	case portable.MethodInvoke:
		var request portable.Invocation
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, fmt.Errorf("decode portable invocation: %w", err)
		}
		return e.invoke(ctx, request)
	default:
		return nil, fmt.Errorf("runtime.dagger does not support method %q", method)
	}
}

func (e *Extension) doctor(ctx context.Context) (portable.DoctorResponse, error) {
	executable, err := e.executablePath(ctx)
	if err != nil {
		return portable.DoctorResponse{}, err
	}
	versionOut, err := exec.CommandContext(ctx, executable, "version").CombinedOutput()
	if err != nil {
		return portable.DoctorResponse{}, fmt.Errorf("dagger version: %w: %s", err, strings.TrimSpace(string(versionOut)))
	}

	query := `{container{from(address:"alpine:3.20"){withExec(args:["sh","-c","printf dagger-smoke"]){stdout}}}}`
	cmd := exec.CommandContext(ctx, executable, "api", "query")
	cmd.Stdin = strings.NewReader(query)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return portable.DoctorResponse{}, fmt.Errorf("dagger engine smoke: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	if !strings.Contains(stdout.String(), "dagger-smoke") {
		return portable.DoctorResponse{}, fmt.Errorf("dagger engine smoke returned unexpected output: %s", strings.TrimSpace(stdout.String()))
	}

	return portable.DoctorResponse{
		Provider: ExtensionID,
		Version:  strings.TrimSpace(string(versionOut)),
		Smoke:    "dagger-smoke",
	}, nil
}

func (e *Extension) executablePath(ctx context.Context) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if configured := strings.TrimSpace(e.executable); configured != "" {
		if resolved, err := exec.LookPath(configured); err == nil {
			e.executable = resolved
			return resolved, nil
		}
		if filepath.IsAbs(configured) {
			if info, err := os.Stat(configured); err == nil && !info.IsDir() {
				return configured, nil
			}
		}
	}

	if runtime.GOOS == "windows" {
		return "", fmt.Errorf("%s cannot bootstrap Dagger on native Windows; use WSL or provide dagger on PATH", ExtensionID)
	}

	version := strings.TrimSpace(e.version)
	if version == "" {
		version = defaultDaggerVersion
	}
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve DevTool tool cache: %w", err)
	}
	binDir := filepath.Join(cacheRoot, "devtool", "tools", "dagger", version)
	executable := filepath.Join(binDir, "dagger")
	if info, err := os.Stat(executable); err == nil && !info.IsDir() {
		e.executable = executable
		return executable, nil
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return "", fmt.Errorf("create Dagger tool cache: %w", err)
	}

	installer := exec.CommandContext(ctx, "sh", "-c", "curl -fsSL https://dl.dagger.io/dagger/install.sh | sh")
	installer.Env = append(os.Environ(), "DAGGER_VERSION="+version, "BIN_DIR="+binDir)
	output, err := installer.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("bootstrap Dagger %s: %w: %s", version, err, strings.TrimSpace(string(output)))
	}
	info, err := os.Stat(executable)
	if err != nil {
		return "", fmt.Errorf("Dagger bootstrap completed without executable %s: %w", executable, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("Dagger bootstrap path is a directory: %s", executable)
	}
	if err := os.Chmod(executable, 0o755); err != nil {
		return "", fmt.Errorf("make Dagger executable: %w", err)
	}
	e.executable = executable
	return executable, nil
}

func (e *Extension) invoke(ctx context.Context, request portable.Invocation) (json.RawMessage, error) {
	if strings.TrimSpace(request.Function) == "" {
		return nil, fmt.Errorf("portable invocation function is required")
	}
	executable, err := e.executablePath(ctx)
	if err != nil {
		return nil, err
	}

	args := make([]string, 0, 9+len(request.Args))
	if request.Workspace != "" {
		args = append(args, "-W", request.Workspace)
	}
	args = append(args, "api", "call")
	if request.Module != "" {
		module := request.Module
		if request.Workspace != "" && !filepath.IsAbs(module) {
			module = filepath.Join(request.Workspace, module)
		}
		args = append(args, "-m", module)
	}
	args = append(args, request.Function)

	keys := make([]string, 0, len(request.Args))
	for key := range request.Args {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		args = append(args, "--"+key+"="+request.Args[key])
	}
	if request.Output != "" {
		args = append(args, "--output="+request.Output)
	} else {
		args = append(args, "--json")
	}

	cmd := exec.CommandContext(ctx, executable, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("dagger api call: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	raw := bytes.TrimSpace(stdout.Bytes())
	if len(raw) == 0 {
		return json.RawMessage(`null`), nil
	}
	if json.Valid(raw) {
		return append(json.RawMessage(nil), raw...), nil
	}
	return json.Marshal(string(raw))
}