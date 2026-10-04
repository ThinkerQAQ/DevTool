package dagger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/thinkerqaq/devtool/core/extension"
	"github.com/thinkerqaq/devtool/core/service"
	"github.com/thinkerqaq/devtool/sdk/portable"
)

const ExtensionID = "runtime.dagger"

type Extension struct {
	executable string
}

func New() *Extension {
	return &Extension{executable: "dagger"}
}

func (e *Extension) Descriptor() extension.Descriptor {
	return extension.Descriptor{
		ID:       ExtensionID,
		Kind:     extension.KindRuntime,
		Provides: []string{portable.ServiceName},
	}
}

func (e *Extension) Register(reg extension.Registrar) error {
	return reg.ProvideService(portable.ServiceName, ExtensionID, service.Func(e.Invoke))
}

func (e *Extension) Invoke(ctx context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
	switch method {
	case portable.MethodDoctor:
		response, err := e.doctor(ctx)
		if err != nil {
			return nil, err
		}
		return json.Marshal(response)
	case portable.MethodInvoke:
		var request portable.Invocation
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, fmt.Errorf("decode portable invocation: %w", err)
		}
		return e.invoke(ctx, request)
	default:
		return nil, fmt.Errorf("runtime.dagger does not support method %q", method)
	}
}

func (e *Extension) doctor(ctx context.Context) (portable.DoctorResponse, error) {
	versionOut, err := exec.CommandContext(ctx, e.executable, "version").CombinedOutput()
	if err != nil {
		return portable.DoctorResponse{}, fmt.Errorf("dagger version: %w: %s", err, strings.TrimSpace(string(versionOut)))
	}

	query := `{container{from(address:"alpine:3.20"){withExec(args:["sh","-c","printf dagger-smoke"]){stdout}}}}`
	cmd := exec.CommandContext(ctx, e.executable, "api", "query")
	cmd.Stdin = strings.NewReader(query)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return portable.DoctorResponse{}, fmt.Errorf("dagger engine smoke: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	if !strings.Contains(stdout.String(), "dagger-smoke") {
		return portable.DoctorResponse{}, fmt.Errorf("dagger engine smoke returned unexpected output: %s", strings.TrimSpace(stdout.String()))
	}

	return portable.DoctorResponse{
		Provider: ExtensionID,
		Version:  strings.TrimSpace(string(versionOut)),
		Smoke:    "dagger-smoke",
	}, nil
}

func (e *Extension) invoke(ctx context.Context, request portable.Invocation) (json.RawMessage, error) {
	if strings.TrimSpace(request.Function) == "" {
		return nil, fmt.Errorf("portable invocation function is required")
	}

	args := []string{"api", "call", "--silent", "--progress=plain", "--json"}
	if request.Workspace != "" {
		args = append(args, "-W", request.Workspace)
	}
	if request.Module != "" {
		args = append(args, "-m", request.Module)
	}
	args = append(args, request.Function)

	keys := make([]string, 0, len(request.Args))
	for key := range request.Args {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		args = append(args, "--"+key+"="+request.Args[key])
	}

	cmd := exec.CommandContext(ctx, e.executable, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("dagger api call: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	raw := bytes.TrimSpace(stdout.Bytes())
	if len(raw) == 0 {
		return json.RawMessage(`null`), nil
	}
	if json.Valid(raw) {
		return append(json.RawMessage(nil), raw...), nil
	}
	return json.Marshal(string(raw))
}
