package main

import (
	"fmt"
	"os"

	workspacecapability "github.com/thinkerqaq/devtool/extensions/capability/workspace"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

func main() {
	if err := extensioncontract.ServeExtension(workspacecapability.New()); err != nil {
		fmt.Fprintln(os.Stderr, "capability.workspace:", err)
		os.Exit(1)
	}
}
