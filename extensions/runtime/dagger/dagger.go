package dagger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	service "github.com/thinkerqaq/devtool/sdk/service"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
	"github.com/thinkerqaq/devtool/sdk/portable"
)

const ExtensionID = "runtime.dagger"

type Extension struct {
	executable string
}

func New() *Extension {
	return &Extension{executable: "dagger"}
}

func (e *Extension) Descriptor() extensioncontract.Descriptor {
	return extensioncontract.Descriptor{
		ID:       ExtensionID,
		Kind:     extensioncontract.KindRuntime,
		Provides: []string{portable.ServiceName},
	}
}

func (e *Extension) Register(reg extensioncontract.Registrar) error {
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

	args := make([]string, 0, 9+len(request.Args))
	if request.Workspace != "" {
		args = append(args, "-W", request.Workspace)
	}
	args = append(args, "api", "call")
	if request.Module != "" {
		module := request.Module
		if request.Workspace != "" && !filepath.IsAbs(module) {
			module = filepath.Join(request.Workspace, module)
		}
		args = append(args, "-m", module)
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
	if request.Output != "" {
		args = append(args, "--output="+request.Output)
	} else {
		args = append(args, "--json")
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