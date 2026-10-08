package main

import (
	"fmt"
	diagram "github.com/thinkerqaq/devtool/extensions/capability/diagram"
	extension "github.com/thinkerqaq/devtool/sdk/extension"
	"os"
)

func main() {
	if err := extension.ServeExtension(diagram.New()); err != nil {
		fmt.Fprintln(os.Stderr, "capability.diagram:", err)
		os.Exit(1)
	}
}
