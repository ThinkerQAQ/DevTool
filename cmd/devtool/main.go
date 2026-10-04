package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/thinkerqaq/devtool/core/config"
	"github.com/thinkerqaq/devtool/core/host"
	"github.com/thinkerqaq/devtool/core/project"
	"github.com/thinkerqaq/devtool/extensions"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "devtool:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printUsage(out)
		return nil
	}

	switch args[0] {
	case "project":
		return runProject(ctx, args[1:], out)
	case "config":
		return runConfig(args[1:], out)
	case "code":
		return runCode(ctx, args[1:], out)
	case "agent":
		return runAgent(ctx, args[1:], out)
	default:
		h, err := host.OpenProject(ctx, "", extensions.Resolve)
		if err != nil {
			return err
		}
		defer h.Close()
		return h.Execute(ctx, args[0], args[1:], out)
	}
}

func runProject(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 0 || args[0] != "inspect" {
		return errors.New("project requires subcommand: inspect")
	}
	fs := flag.NewFlagSet("project inspect", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOutput := fs.Bool("json", false, "print machine-readable project metadata")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", fs.Args())
	}

	h, err := host.OpenProject(ctx, "", extensions.Resolve)
	if err != nil {
		return err
	}
	defer h.Close()
	if *jsonOutput {
		return json.NewEncoder(out).Encode(map[string]any{
			"name":       h.Project.Config.Project.Name,
			"root":       h.Project.Root,
			"config":     h.Project.ConfigPath,
			"extension":  h.Extension,
			"descriptor": h.Descriptor,
			"services":   h.Project.Config.Service,
			"ui":         h.Project.Config.UI,
		})
	}
	fmt.Fprintf(out, "Project: %s\n", h.Project.Config.Project.Name)
	fmt.Fprintf(out, "Root: %s\n", h.Project.Root)
	fmt.Fprintf(out, "Config: %s\n", h.Project.ConfigPath)
	fmt.Fprintf(out, "Project Extension: %s\n", h.Extension.ID)
	fmt.Fprintf(out, "Commands: %d\n", len(h.Descriptor.Commands))
	fmt.Fprintf(out, "Resources: %d\n", len(h.Descriptor.Resources))
	return nil
}

func runConfig(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("config requires subcommand: path or validate")
	}
	p, err := project.Discover("")
	if err != nil {
		return err
	}
	switch args[0] {
	case "path":
		if len(args) != 1 {
			return errors.New("config path does not accept arguments")
		}
		fmt.Fprintln(out, p.ConfigPath)
		return nil
	case "validate":
		if len(args) != 1 {
			return errors.New("config validate does not accept arguments")
		}
		if err := config.Validate(p.Config); err != nil {
			return err
		}
		fmt.Fprintf(out, "VALID %s\n", p.ConfigPath)
		return nil
	default:
		return fmt.Errorf("unknown config subcommand %q", args[0])
	}
}

func printUsage(out io.Writer) {
	fmt.Fprintln(out, "DevTool")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Core Commands:")
	fmt.Fprintln(out, "  devtool project inspect [--json]")
	fmt.Fprintln(out, "  devtool config path")
	fmt.Fprintln(out, "  devtool config validate")
	fmt.Fprintln(out, "  devtool agent mcp [--context agent|codex|claude-code]")
	fmt.Fprintln(out, "  devtool code doctor")
	fmt.Fprintln(out, "  devtool code verify")
	fmt.Fprintln(out, "  devtool code graph mcp")
	fmt.Fprintln(out, "  devtool code graph sync")
	fmt.Fprintln(out, "  devtool code graph query <tool> [json-args]")
	fmt.Fprintln(out, "  devtool code lsp mcp [--context agent|codex|claude-code]")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Project commands are discovered from the configured Project Extension.")
}
