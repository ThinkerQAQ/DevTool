package envexec

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/thinkerqaq/devtool/sdk/codeintelligence"
	environmentcontract "github.com/thinkerqaq/devtool/sdk/environment"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

func Command(ctx context.Context, services extensioncontract.Registrar, workspace codeintelligence.Workspace, executable string, args ...string) (*exec.Cmd, error) {
	if services == nil {
		return nil, fmt.Errorf("environment service registry is unavailable")
	}
	invoker, ok := services.Service(environmentcontract.ToolingServiceName)
	if !ok {
		return nil, fmt.Errorf("service %q is not configured", environmentcontract.ToolingServiceName)
	}
	payload, err := json.Marshal(environmentcontract.CommandRequest{
		Root:       workspace.Root,
		WorkingDir: environmentcontract.WorkspaceRoot,
		Executable: executable,
		Args:       args,
	})
	if err != nil {
		return nil, err
	}
	raw, err := invoker.Invoke(ctx, environmentcontract.MethodCommand, payload)
	if err != nil {
		return nil, err
	}
	var spec environmentcontract.CommandSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return nil, fmt.Errorf("decode environment command spec: %w", err)
	}
	if spec.Program == "" {
		return nil, fmt.Errorf("environment command spec has empty program")
	}
	cmd := exec.CommandContext(ctx, spec.Program, spec.Args...)
	cmd.Dir = spec.Dir
	if len(spec.Env) != 0 {
		cmd.Env = append(os.Environ(), spec.Env...)
	}
	return cmd, nil
}
