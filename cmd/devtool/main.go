package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/thinkerqaq/devtool/core/config"
	"github.com/thinkerqaq/devtool/core/project"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "devtool:", err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printUsage(out)
		return nil
	}

	switch args[0] {
	case "project":
		return runProject(args[1:], out)
	case "config":
		return runConfig(args[1:], out)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runProject(args []string, out io.Writer) error {
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

	p, err := project.Discover("")
	if err != nil {
		return err
	}
	if *jsonOutput {
		return json.NewEncoder(out).Encode(map[string]any{
			"name":       p.Config.Project.Name,
			"root":       p.Root,
			"config":     p.ConfigPath,
			"extensions": p.Config.Extension,
			"services":   p.Config.Service,
			"ui":         p.Config.UI,
		})
	}
	fmt.Fprintf(out, "Project: %s\n", p.Config.Project.Name)
	fmt.Fprintf(out, "Root: %s\n", p.Root)
	fmt.Fprintf(out, "Config: %s\n", p.ConfigPath)
	fmt.Fprintf(out, "Extensions: %d\n", len(p.Config.Extension))
	fmt.Fprintf(out, "Services: %d\n", len(p.Config.Service))
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
	fmt.Fprintln(out, "Commands:")
	fmt.Fprintln(out, "  devtool project inspect [--json]")
	fmt.Fprintln(out, "  devtool config path")
	fmt.Fprintln(out, "  devtool config validate")
}
