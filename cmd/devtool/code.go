package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/thinkerqaq/devtool/adapters/extensionloader"
	"github.com/thinkerqaq/devtool/core/host"
	"github.com/thinkerqaq/devtool/sdk/codeintelligence"
)

func runCode(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("code requires subcommand: doctor or verify")
	}
	h, err := host.OpenProject(ctx, "", extensionloader.Resolve)
	if err != nil {
		return err
	}
	defer h.Close()
	workspace := projectCodeWorkspace(h)

	switch args[0] {
	case "doctor":
		if len(args) != 1 {
			return errors.New("code doctor does not accept arguments")
		}
		return runCodeDoctor(ctx, h, workspace, out)
	case "verify":
		if len(args) != 1 {
			return errors.New("code verify does not accept arguments")
		}
		if _, err := invokeCodeService(ctx, h, codeintelligence.IndexedServiceName, codeintelligence.MethodVerify, workspace); err != nil {
			return fmt.Errorf("indexed intelligence verify: %w", err)
		}
		if _, err := invokeCodeService(ctx, h, codeintelligence.RealtimeServiceName, codeintelligence.MethodVerify, workspace); err != nil {
			return fmt.Errorf("realtime intelligence verify: %w", err)
		}
		fmt.Fprintln(out, "Code intelligence VERIFIED")
		fmt.Fprintln(out, "- Indexed: PASS")
		fmt.Fprintln(out, "- Realtime: PASS")
		return nil
	default:
		return fmt.Errorf("unknown code subcommand %q; use doctor or verify", args[0])
	}
}

func projectCodeWorkspace(h *host.ProjectHost) codeintelligence.Workspace {
	root := h.Project.Root
	configured := h.Project.Config.Code.Workspaces
	workspaces := make([]string, 0, len(configured))
	for _, item := range configured {
		if filepath.IsAbs(item) {
			workspaces = append(workspaces, filepath.Clean(item))
			continue
		}
		workspaces = append(workspaces, filepath.Join(root, item))
	}
	if len(workspaces) == 0 {
		workspaces = []string{root}
	}
	return codeintelligence.Workspace{
		Root:       root,
		Workspaces: workspaces,
	}
}

func runCodeDoctor(ctx context.Context, h *host.ProjectHost, workspace codeintelligence.Workspace, out io.Writer) error {
	for _, serviceName := range []string{codeintelligence.IndexedServiceName, codeintelligence.RealtimeServiceName} {
		raw, err := invokeCodeService(ctx, h, serviceName, codeintelligence.MethodDoctor, workspace)
		if err != nil {
			return fmt.Errorf("%s doctor: %w", serviceName, err)
		}
		var response codeintelligence.DoctorResponse
		if err := json.Unmarshal(raw, &response); err != nil {
			return fmt.Errorf("decode %s doctor response: %w", serviceName, err)
		}
		fmt.Fprintf(out, "%s READY provider=%s executable=%s", serviceName, response.Provider, response.Executable)
		if response.Version != "" {
			fmt.Fprintf(out, " version=%s", response.Version)
		}
		fmt.Fprintln(out)
	}
	return nil
}

func invokeCodeService(ctx context.Context, h *host.ProjectHost, name, method string, request any) (json.RawMessage, error) {
	invoker, ok := h.Registry.Service(name)
	if !ok {
		return nil, fmt.Errorf("service %q is not configured", name)
	}
	var payload json.RawMessage
	if request != nil {
		raw, err := json.Marshal(request)
		if err != nil {
			return nil, err
		}
		payload = raw
	}
	return invoker.Invoke(ctx, method, payload)
}
