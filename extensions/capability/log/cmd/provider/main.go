package main

import (
	"fmt"
	"os"

	logcapability "github.com/thinkerqaq/devtool/extensions/capability/log"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

func main() {
	if err := extensioncontract.ServeExtension(logcapability.New()); err != nil {
		fmt.Fprintln(os.Stderr, "capability.log:", err)
		os.Exit(1)
	}
}
