package main

import (
	"fmt"
	"os"

	loglocal "github.com/thinkerqaq/devtool/extensions/intelligence/loglocal"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

func main() {
	if err := extensioncontract.ServeExtension(loglocal.New()); err != nil {
		fmt.Fprintln(os.Stderr, "intelligence.log.local:", err)
		os.Exit(1)
	}
}
