package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thinkerqaq/devtool/adapters/extensionloader"
	coreagent "github.com/thinkerqaq/devtool/core/agent"
	"github.com/thinkerqaq/devtool/core/host"
	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
)

func runAgent(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("agent requires subcommand: mcp or serve")
	}
	switch args[0] {
	case "mcp":
		return runAgentMCP(ctx, args[1:], out)
	case "serve":
		return runAgentHTTP(ctx, args[1:], out)
	default:
		return fmt.Errorf("unknown agent subcommand %q", args[0])
	}
}

func runAgentMCP(ctx context.Context, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("agent mcp", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	contextName := fs.String("context", "agent", "agent operation context")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected agent mcp arguments: %v", fs.Args())
	}

	h, err := host.OpenProject(ctx, "", extensionloader.Resolve)
	if err != nil {
		return err
	}
	defer h.Close()
	session := projectAgentSession(h, strings.TrimSpace(*contextName))
	return coreagent.NewGateway(h.Registry, session).Serve(ctx, os.Stdin, out)
}

func runAgentHTTP(ctx context.Context, args []string, out io.Writer) error {
	defaultListen := ":8080"
	if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
		defaultListen = ":" + port
	}

	fs := flag.NewFlagSet("agent serve", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	listen := fs.String("listen", defaultListen, "HTTP listen address")
	contextName := fs.String("context", "agent", "agent operation context")
	tokenEnv := fs.String("token-env", "DEVTOOL_AGENT_TOKEN", "environment variable containing the bearer token")
	allowUnauthenticated := fs.Bool("allow-unauthenticated", false, "allow unauthenticated /mcp requests")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected agent serve arguments: %v", fs.Args())
	}

	token := ""
	if name := strings.TrimSpace(*tokenEnv); name != "" {
		token = strings.TrimSpace(os.Getenv(name))
	}
	if token == "" && !*allowUnauthenticated {
		return fmt.Errorf("%s is required; use --allow-unauthenticated only for trusted local environments", strings.TrimSpace(*tokenEnv))
	}

	h, err := host.OpenProject(ctx, "", extensionloader.Resolve)
	if err != nil {
		return err
	}
	defer h.Close()

	session := projectAgentSession(h, strings.TrimSpace(*contextName))
	gateway := coreagent.NewGateway(h.Registry, session)
	server := &http.Server{
		Addr:              strings.TrimSpace(*listen),
		Handler:           gateway.HTTPHandler(token),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()
	fmt.Fprintf(out, "DevTool agent HTTP listening on %s\n", server.Addr)

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		err := <-errCh
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
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
