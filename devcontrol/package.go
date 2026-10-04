package devcontrol

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/thinkerqaq/devtool/protocol"
)

func Package(ctx context.Context, emit func(protocol.Event)) error {
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
	emit(protocol.Event{Kind: "artifact", Message: "Packaged " + target})
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
