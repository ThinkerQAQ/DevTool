package main

import (
	"fmt"
	cli "github.com/thinkerqaq/devtool/extensions/diagram/cli"
	extension "github.com/thinkerqaq/devtool/sdk/extension"
	"os"
)

func main() {
	if err := extension.ServeExtension(cli.New()); err != nil {
		fmt.Fprintln(os.Stderr, "diagram.render.cli:", err)
		os.Exit(1)
	}
}
