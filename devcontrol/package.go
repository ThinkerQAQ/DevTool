package devcontrol

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

func Package(ctx context.Context, emit emitter) error {
	if err := Verify(ctx, emit); err != nil {
		return err
	}
	source := nextBinaryPath()
	name := fmt.Sprintf("devtool-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	target := filepath.Join(".devtool", "artifacts", name)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if err := copyFile(source, target); err != nil {
		return err
	}
	emit("artifact", "Packaged "+target)
	return nil
}

func copyFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
