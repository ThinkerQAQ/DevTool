package devcontrol

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

type emitter func(kind, message string)

func run(ctx context.Context, emit emitter, name string, args ...string) error {
	emit("log", "$ "+name+" "+joinArgs(args))
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w", name, err)
	}
	return nil
}

func joinArgs(args []string) string {
	var result string
	for i, arg := range args {
		if i > 0 {
			result += " "
		}
		result += arg
	}
	return result
}
