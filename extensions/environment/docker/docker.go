package docker

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/thinkerqaq/devtool/extensions/environment/internal/commandexec"
	service "github.com/thinkerqaq/devtool/sdk/service"
	environmentcontract "github.com/thinkerqaq/devtool/sdk/environment"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

const ExtensionID = "environment.docker"

type Extension struct {
	mu    sync.Mutex
	image string
}

func New() *Extension {
	return &Extension{}
}

func (e *Extension) Descriptor() extensioncontract.Descriptor {
	return extensioncontract.Descriptor{
		ID:       ExtensionID,
		Kind:     extensioncontract.KindInfrastructure,
		Provides: []string{environmentcontract.ServiceName},
	}
}

func (e *Extension) Configure(settings map[string]any) error {
	raw, ok := settings["image"]
	if !ok {
		e.image = ""
		return nil
	}
	image, ok := raw.(string)
	if !ok {
		return fmt.Errorf("environment.docker settings.image must be a string")
	}
	e.image = strings.TrimSpace(image)
	return nil
}

func (e *Extension) Register(reg extensioncontract.Registrar) error {
	return reg.ProvideService(environmentcontract.ServiceName, ExtensionID, service.Func(e.Invoke))
}

func (e *Extension) Invoke(ctx context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
	var request environmentcontract.CommandRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		return nil, fmt.Errorf("decode environment command request: %w", err)
	}
	spec, err := e.commandSpec(ctx, request)
	if err != nil {
		return nil, err
	}

	switch method {
	case environmentcontract.MethodCommand:
		return json.Marshal(spec)
	case environmentcontract.MethodRun:
		result, err := commandexec.Run(ctx, spec)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)
	default:
		return nil, fmt.Errorf("%s does not support method %q", ExtensionID, method)
	}
}

func (e *Extension) commandSpec(ctx context.Context, request environmentcontract.CommandRequest) (environmentcontract.CommandSpec, error) {
	image := strings.TrimSpace(e.image)
	if image == "" {
		return environmentcontract.CommandSpec{}, fmt.Errorf("environment.docker settings.image is required")
	}
	if strings.TrimSpace(request.Root) == "" {
		return environmentcontract.CommandSpec{}, fmt.Errorf("environment project root is required")
	}
	if strings.TrimSpace(request.Executable) == "" {
		return environmentcontract.CommandSpec{}, fmt.Errorf("environment executable is required")
	}

	root, err := filepath.Abs(request.Root)
	if err != nil {
		return environmentcontract.CommandSpec{}, fmt.Errorf("resolve environment project root: %w", err)
	}
	devenvHome := filepath.Join(root, ".devtool", "cache", "devenv-home")
	if err := os.MkdirAll(devenvHome, 0o755); err != nil {
		return environmentcontract.CommandSpec{}, fmt.Errorf("create development environment home: %w", err)
	}

	uid, gid := currentUserIDs()
	e.mu.Lock()
	name, err := ensureWorkspaceContainer(ctx, root, devenvHome, image, uid, gid)
	e.mu.Unlock()
	if err != nil {
		return environmentcontract.CommandSpec{}, err
	}

	workdir := workspaceContainerDir(request.WorkingDir)
	return environmentcontract.CommandSpec{
		Program: "docker",
		Args:    workspaceExecArgs(name, uid, gid, workdir, request.Env, request.Executable, request.Args...),
		Dir:     root,
	}, nil
}

func workspaceContainerDir(value string) string {
	workdir := strings.TrimSpace(filepath.ToSlash(value))
	if workdir == "" || workdir == "." {
		return "/workspace"
	}
	if path.IsAbs(workdir) {
		return path.Clean(workdir)
	}
	return path.Join("/workspace", workdir)
}

func ensureWorkspaceContainer(ctx context.Context, root, devenvHome, image, uid, gid string) (string, error) {
	name := workspaceContainerName(root)
	existingImage, running, exists, err := inspectWorkspaceContainer(ctx, name)
	if err != nil {
		return "", err
	}
	if exists && existingImage != image {
		if err := removeWorkspaceContainer(ctx, name); err != nil {
			return "", err
		}
		exists = false
		running = false
	}
	if !exists {
		if err := createWorkspaceContainer(ctx, name, root, devenvHome, image, uid, gid); err != nil {
			return "", err
		}
		return name, nil
	}
	if !running {
		if err := startWorkspaceContainer(ctx, name); err != nil {
			return "", err
		}
	}
	return name, nil
}

func inspectWorkspaceContainer(ctx context.Context, name string) (image string, running bool, exists bool, err error) {
	cmd := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.Config.Image}}\t{{.State.Running}}", name)
	out, commandErr := cmd.CombinedOutput()
	if commandErr != nil {
		message := strings.TrimSpace(string(out))
		if isContainerNotFoundMessage(message) {
			return "", false, false, nil
		}
		return "", false, false, fmt.Errorf("inspect development environment container %s: %w: %s", name, commandErr, message)
	}
	parts := strings.SplitN(strings.TrimSpace(string(out)), "\t", 2)
	if len(parts) != 2 {
		return "", false, false, fmt.Errorf("inspect development environment container %s returned unexpected output %q", name, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]) == "true", true, nil
}

func isContainerNotFoundMessage(message string) bool {
	normalized := strings.ToLower(strings.TrimSpace(message))
	return strings.Contains(normalized, "no such object") || strings.Contains(normalized, "no such container")
}

func createWorkspaceContainer(ctx context.Context, name, root, devenvHome, image, uid, gid string) error {
	args := workspaceCreateArgs(name, root, devenvHome, image, uid, gid)
	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("create development environment container %s: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func startWorkspaceContainer(ctx context.Context, name string) error {
	out, err := exec.CommandContext(ctx, "docker", "start", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("start development environment container %s: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func removeWorkspaceContainer(ctx context.Context, name string) error {
	out, err := exec.CommandContext(ctx, "docker", "rm", "-f", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("replace development environment container %s: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func workspaceContainerName(root string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(root)))
	return fmt.Sprintf("devtool-devenv-%x", sum[:6])
}

func workspaceCreateArgs(name, root, devenvHome, image, uid, gid string) []string {
	args := []string{
		"run", "-d",
		"--name", name,
		"--label", "devtool.managed=true",
		"--mount", "type=bind,source=" + root + ",target=/workspace",
		"--mount", "type=bind,source=" + devenvHome + ",target=/tmp/devenv-home",
		"--workdir", "/workspace",
	}
	if uid != "" && gid != "" {
		args = append(args,
			"-e", "DEVENV_RUN_UID="+uid,
			"-e", "DEVENV_RUN_GID="+gid,
		)
	}
	args = append(args,
		image,
		"sh", "-lc",
		"trap 'exit 0' TERM INT; while :; do sleep 3600; done",
	)
	return args
}

func workspaceExecArgs(name, uid, gid, workdir string, env []string, executable string, args ...string) []string {
	dockerArgs := []string{
		"exec", "-i",
		"--workdir", workdir,
		"-e", "HOME=/tmp/devenv-home",
	}
	if uid != "" && gid != "" {
		dockerArgs = append(dockerArgs, "--user", uid+":"+gid)
	}
	for _, item := range env {
		if value := strings.TrimSpace(item); value != "" {
			dockerArgs = append(dockerArgs, "-e", value)
		}
	}
	dockerArgs = append(dockerArgs, name, executable)
	dockerArgs = append(dockerArgs, args...)
	return dockerArgs
}

func currentUserIDs() (string, string) {
	current, err := user.Current()
	if err != nil {
		return "", ""
	}
	return strings.TrimSpace(current.Uid), strings.TrimSpace(current.Gid)
}
