package devcontrol

import (
	"context"
	"os/exec"
)

func Verify(ctx context.Context, emit emitter) error {
	emit("progress", "Running DevTool tests")
	if err := run(ctx, emit, "go", "test", "./..."); err != nil {
		return err
	}
	if err := Build(ctx, emit); err != nil {
		return err
	}

	emit("progress", "Verifying DevTool N+1 through its own project extension")
	cmd := exec.CommandContext(ctx, nextBinaryPath(), "project", "inspect", "--json")
	if output, err := cmd.CombinedOutput(); err != nil {
		return &commandOutputError{command: nextBinaryPath(), output: output, err: err}
	}
	emit("result", "DevTool N+1 self-host verification passed")
	return nil
}

type commandOutputError struct {
	command string
	output  []byte
	err     error
}

func (e *commandOutputError) Error() string {
	return e.command + " failed: " + e.err.Error() + ": " + string(e.output)
}

func (e *commandOutputError) Unwrap() error { return e.err }
