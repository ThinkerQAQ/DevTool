package devcontrol

import (
	"context"
	"os/exec"

	"github.com/thinkerqaq/devtool/protocol"
)

func Verify(ctx context.Context, emit func(protocol.Event)) error {
	emit(protocol.Event{Kind: "progress", Message: "Running DevTool tests"})
	if err := run(ctx, emit, "go", "test", "./..."); err != nil {
		return err
	}
	if err := Build(ctx, emit); err != nil {
		return err
	}

	emit(protocol.Event{Kind: "progress", Message: "Verifying DevTool N+1 through its own project extension"})
	cmd := exec.CommandContext(ctx, nextBinaryPath(), "project", "inspect", "--json")
	if output, err := cmd.CombinedOutput(); err != nil {
		return &commandOutputError{command: nextBinaryPath(), output: output, err: err}
	}
	emit(protocol.Event{Kind: "result", Message: "DevTool N+1 self-host verification passed"})
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
