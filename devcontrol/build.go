package devcontrol

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

func nextBinaryPath() string {
	name := "devtool-next"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(".devtool", "out", name)
}

func Build(ctx context.Context, emit emitter) error {
	output := nextBinaryPath()
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	emit("progress", "Building DevTool N+1")
	if err := run(ctx, emit, "go", "build", "-o", output, "./cmd/devtool"); err != nil {
		return err
	}
	emit("artifact", fmt.Sprintf("Built %s", output))
	return nil
}
