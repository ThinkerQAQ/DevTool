package commandexec

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"

	environmentcontract "github.com/thinkerqaq/devtool/sdk/environment"
)

func Run(ctx context.Context, spec environmentcontract.CommandSpec) (environmentcontract.RunResult, error) {
	cmd := exec.CommandContext(ctx, spec.Program, spec.Args...)
	cmd.Dir = spec.Dir
	if len(spec.Env) != 0 {
		cmd.Env = append(os.Environ(), spec.Env...)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := environmentcontract.RunResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
	if err == nil {
		return result, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}
	return environmentcontract.RunResult{}, err
}
