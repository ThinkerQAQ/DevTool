package local

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/thinkerqaq/devtool/extensions/environment/internal/commandexec"
	environmentcontract "github.com/thinkerqaq/devtool/sdk/environment"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
	service "github.com/thinkerqaq/devtool/sdk/service"
)

const ExtensionID = "environment.local"

type Extension struct{}

func New() *Extension { return &Extension{} }

func (e *Extension) Descriptor() extensioncontract.Descriptor {
	return extensioncontract.Descriptor{
		ID:       ExtensionID,
		Kind:     extensioncontract.KindInfrastructure,
		Provides: []string{environmentcontract.ServiceName},
	}
}

func (e *Extension) Register(reg extensioncontract.Registrar) error {
	return reg.ProvideService(environmentcontract.ServiceName, ExtensionID, service.Func(e.Invoke))
}

func (e *Extension) Invoke(ctx context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
	var request environmentcontract.CommandRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		return nil, fmt.Errorf("decode environment command request: %w", err)
	}
	spec, err := commandSpec(request)
	if err != nil {
		return nil, err
	}

	switch method {
	case environmentcontract.MethodCommand:
		return json.Marshal(spec)
	case environmentcontract.MethodRun:
		result, err := commandexec.Run(ctx, spec)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)
	default:
		return nil, fmt.Errorf("%s does not support method %q", ExtensionID, method)
	}
}

func commandSpec(request environmentcontract.CommandRequest) (environmentcontract.CommandSpec, error) {
	if strings.TrimSpace(request.Root) == "" {
		return environmentcontract.CommandSpec{}, fmt.Errorf("environment project root is required")
	}
	if strings.TrimSpace(request.Executable) == "" {
		return environmentcontract.CommandSpec{}, fmt.Errorf("environment executable is required")
	}

	root, err := filepath.Abs(request.Root)
	if err != nil {
		return environmentcontract.CommandSpec{}, fmt.Errorf("resolve environment project root: %w", err)
	}
	workdir := strings.TrimSpace(request.WorkingDir)
	if workdir == "" {
		workdir = root
	} else if !filepath.IsAbs(workdir) {
		workdir = filepath.Join(root, workdir)
	}
	workdir = filepath.Clean(workdir)
	rel, err := filepath.Rel(root, workdir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return environmentcontract.CommandSpec{}, fmt.Errorf("environment working directory %s is outside project root %s", workdir, root)
	}

	return environmentcontract.CommandSpec{
		Program: strings.TrimSpace(request.Executable),
		Args:    append([]string(nil), request.Args...),
		Dir:     workdir,
	}, nil
}
