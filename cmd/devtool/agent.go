package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	coreagent "github.com/thinkerqaq/devtool/core/agent"
	"github.com/thinkerqaq/devtool/core/host"
	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
)

func runAgent(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 0 || args[0] != "mcp" {
		return errors.New("agent requires subcommand: mcp")
	}
	fs := flag.NewFlagSet("agent mcp", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	contextName := fs.String("context", "agent", "agent operation context")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected agent mcp arguments: %v", fs.Args())
	}

	h, err := host.OpenProject(ctx, "")
	if err != nil {
		return err
	}
	defer h.Close()
	session := projectAgentSession(h, strings.TrimSpace(*contextName))
	return coreagent.NewGateway(h.Registry, session).Serve(ctx, os.Stdin, out)
}

func projectAgentSession(h *host.ProjectHost, contextName string) agentsdk.Session {
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
	return agentsdk.Session{
		ProjectRoot:      root,
		Workspaces:       workspaces,
		EnvironmentImage: h.Project.Config.Dev.Environment.Image,
		Context:          contextName,
	}
}
