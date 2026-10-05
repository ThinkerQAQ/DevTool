package local

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	service "github.com/thinkerqaq/devtool/sdk/service"
	environmentcontract "github.com/thinkerqaq/devtool/sdk/environment"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
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

func (e *Extension) Invoke(_ context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
	if method != environmentcontract.MethodCommand {
		return nil, fmt.Errorf("%s does not support method %q", ExtensionID, method)
	}
	var request environmentcontract.CommandRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		return nil, fmt.Errorf("decode environment command request: %w", err)
	}
	spec, err := commandSpec(request)
	if err != nil {
		return nil, err
	}
	return json.Marshal(spec)
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
