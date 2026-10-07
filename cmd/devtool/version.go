package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/thinkerqaq/devtool/internal/buildinfo"
)

func runVersion(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOutput := fs.Bool("json", false, "print machine-readable build metadata")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", fs.Args())
	}

	info := buildinfo.Current()
	if *jsonOutput {
		return json.NewEncoder(out).Encode(info)
	}
	fmt.Fprintf(out, "DevTool %s\n", info.Version)
	fmt.Fprintf(out, "Commit: %s\n", info.Commit)
	return nil
}
