package main

import (
	"fmt"
	"os"

	scmcapability "github.com/thinkerqaq/devtool/extensions/capability/scm"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

func main() {
	if err := extensioncontract.ServeExtension(scmcapability.New()); err != nil {
		fmt.Fprintln(os.Stderr, "capability.scm:", err)
		os.Exit(1)
	}
}
