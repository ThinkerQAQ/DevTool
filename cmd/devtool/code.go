package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/thinkerqaq/devtool/core/host"
	"github.com/thinkerqaq/devtool/sdk/codeintelligence"
)

func runCode(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("code requires subcommand: doctor, verify, graph, or lsp")
	}
	h, err := host.OpenProject(ctx, "")
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
		if _, err := invokeCodeService(ctx, h, codeintelligence.GraphServiceName, codeintelligence.MethodSync, workspace); err != nil {
			return fmt.Errorf("CodeGraph verify: %w", err)
		}
		if _, err := invokeCodeService(ctx, h, codeintelligence.LSPServiceName, codeintelligence.MethodVerify, workspace); err != nil {
			return fmt.Errorf("LSP verify: %w", err)
		}
		fmt.Fprintln(out, "Code intelligence VERIFIED")
		fmt.Fprintln(out, "- CodeGraph: PASS")
		fmt.Fprintln(out, "- LSP: PASS")
		return nil
	case "graph":
		return runCodeGraph(ctx, h, workspace, args[1:], out)
	case "lsp":
		return runCodeLSP(ctx, h, workspace, args[1:])
	default:
		return fmt.Errorf("unknown code subcommand %q; use doctor, verify, graph, or lsp", args[0])
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
		Root:             root,
		Workspaces:       workspaces,
		EnvironmentImage: h.Project.Config.Dev.Environment.Image,
	}
}

func runCodeDoctor(ctx context.Context, h *host.ProjectHost, workspace codeintelligence.Workspace, out io.Writer) error {
	for _, serviceName := range []string{codeintelligence.GraphServiceName, codeintelligence.LSPServiceName} {
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

func runCodeGraph(ctx context.Context, h *host.ProjectHost, workspace codeintelligence.Workspace, args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("code graph requires subcommand: mcp, sync, or query")
	}
	switch args[0] {
	case "mcp":
		if len(args) != 1 {
			return errors.New("code graph mcp does not accept arguments")
		}
		_, err := invokeCodeService(ctx, h, codeintelligence.GraphServiceName, codeintelligence.MethodMCP, codeintelligence.MCPRequest{Workspace: workspace})
		return err
	case "sync":
		if len(args) != 1 {
			return errors.New("code graph sync does not accept arguments")
		}
		raw, err := invokeCodeService(ctx, h, codeintelligence.GraphServiceName, codeintelligence.MethodSync, workspace)
		if err != nil {
			return err
		}
		return writeJSONResult(out, raw)
	case "query":
		if len(args) < 2 || len(args) > 3 {
			return errors.New("code graph query requires <tool> [json-args]")
		}
		queryArgs := json.RawMessage(`{}`)
		if len(args) == 3 {
			queryArgs = json.RawMessage(strings.TrimSpace(args[2]))
			if !json.Valid(queryArgs) {
				return errors.New("code graph query json-args must be valid JSON")
			}
		}
		raw, err := invokeCodeService(ctx, h, codeintelligence.GraphServiceName, codeintelligence.MethodQuery, codeintelligence.GraphQuery{
			Workspace: workspace,
			Tool:      args[1],
			Args:      queryArgs,
		})
		if err != nil {
			return err
		}
		return writeJSONResult(out, raw)
	default:
		return fmt.Errorf("unknown code graph subcommand %q; use mcp, sync, or query", args[0])
	}
}

func runCodeLSP(ctx context.Context, h *host.ProjectHost, workspace codeintelligence.Workspace, args []string) error {
	if len(args) == 0 || args[0] != "mcp" {
		return errors.New("code lsp requires subcommand: mcp")
	}
	fs := flag.NewFlagSet("code lsp mcp", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	contextName := fs.String("context", "agent", "Serena operation context")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected code lsp mcp arguments: %v", fs.Args())
	}
	_, err := invokeCodeService(ctx, h, codeintelligence.LSPServiceName, codeintelligence.MethodMCP, codeintelligence.MCPRequest{
		Workspace: workspace,
		Context:   strings.TrimSpace(*contextName),
	})
	return err
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

func writeJSONResult(out io.Writer, raw json.RawMessage) error {
	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	if !json.Valid(raw) {
		_, err := fmt.Fprintln(out, string(raw))
		return err
	}
	var formatted any
	if err := json.Unmarshal(raw, &formatted); err != nil {
		return err
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(formatted)
}
