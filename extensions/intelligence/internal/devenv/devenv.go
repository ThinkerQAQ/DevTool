package devenv

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"sync"

	"github.com/thinkerqaq/devtool/sdk/codeintelligence"
)

var ensureContainerMu sync.Mutex

func Command(ctx context.Context, workspace codeintelligence.Workspace, executable string, args ...string) (*exec.Cmd, error) {
	image := strings.TrimSpace(workspace.EnvironmentImage)
	if image == "" {
		return nil, fmt.Errorf("development environment image is required")
	}
	if strings.TrimSpace(workspace.Root) == "" {
		return nil, fmt.Errorf("workspace root is required")
	}

	root, err := filepath.Abs(workspace.Root)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace root: %w", err)
	}
	devenvHome := filepath.Join(root, ".devtool", "cache", "devenv-home")
	if err := os.MkdirAll(devenvHome, 0o755); err != nil {
		return nil, fmt.Errorf("create development environment home: %w", err)
	}

	uid, gid := currentUserIDs()
	name, err := ensureWorkspaceContainer(ctx, root, devenvHome, image, uid, gid)
	if err != nil {
		return nil, err
	}

	dockerArgs := workspaceExecArgs(name, uid, gid, executable, args...)
	return exec.CommandContext(ctx, "docker", dockerArgs...), nil
}

func CombinedOutput(ctx context.Context, workspace codeintelligence.Workspace, executable string, args ...string) ([]byte, error) {
	cmd, err := Command(ctx, workspace, executable, args...)
	if err != nil {
		return nil, err
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s in development environment: %w: %s", executable, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

func ensureWorkspaceContainer(ctx context.Context, root, devenvHome, image, uid, gid string) (string, error) {
	ensureContainerMu.Lock()
	defer ensureContainerMu.Unlock()

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
		if strings.Contains(message, "No such object") || strings.Contains(message, "No such container") {
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

func workspaceExecArgs(name, uid, gid, executable string, args ...string) []string {
	dockerArgs := []string{
		"exec", "-i",
		"--workdir", "/workspace",
		"-e", "HOME=/tmp/devenv-home",
	}
	if uid != "" && gid != "" {
		dockerArgs = append(dockerArgs, "--user", uid+":"+gid)
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
