package main

import (
	"fmt"
	"os"

	scmgithub "github.com/thinkerqaq/devtool/extensions/scm/github"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

func main() {
	if err := extensioncontract.ServeProcess(scmgithub.New()); err != nil {
		fmt.Fprintln(os.Stderr, "scm.github:", err)
		os.Exit(1)
	}
}
