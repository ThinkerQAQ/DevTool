package devenv

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/thinkerqaq/devtool/sdk/codeintelligence"
)

func Command(ctx context.Context, workspace codeintelligence.Workspace, executable string, args ...string) (*exec.Cmd, error) {
	if strings.TrimSpace(workspace.EnvironmentImage) == "" {
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

	dockerArgs := []string{"run", "--rm", "-i",
		"--mount", "type=bind,source=" + root + ",target=/workspace",
		"--mount", "type=bind,source=" + devenvHome + ",target=/tmp/devenv-home",
		"--workdir", "/workspace",
	}
	if current, err := user.Current(); err == nil && current.Uid != "" && current.Gid != "" {
		dockerArgs = append(dockerArgs,
			"-e", "DEVENV_RUN_UID="+current.Uid,
			"-e", "DEVENV_RUN_GID="+current.Gid,
		)
	}
	dockerArgs = append(dockerArgs, workspace.EnvironmentImage, executable)
	dockerArgs = append(dockerArgs, args...)
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
