package main

import (
	"fmt"
	"os"

	gitworkspace "github.com/thinkerqaq/devtool/extensions/workspace/git"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

func main() {
	if err := extensioncontract.ServeExtension(gitworkspace.New()); err != nil {
		fmt.Fprintln(os.Stderr, "workspace.git:", err)
		os.Exit(1)
	}
}
