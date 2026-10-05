package main

import (
	"fmt"
	"os"

	sourcegraphext "github.com/thinkerqaq/devtool/extensions/intelligence/sourcegraph"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

func main() {
	if err := extensioncontract.ServeExtension(sourcegraphext.New()); err != nil {
		fmt.Fprintln(os.Stderr, "intelligence.sourcegraph:", err)
		os.Exit(1)
	}
}
