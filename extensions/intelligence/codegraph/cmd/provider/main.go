package main

import (
	"fmt"
	"os"

	codegraphext "github.com/thinkerqaq/devtool/extensions/intelligence/codegraph"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

func main() {
	if err := extensioncontract.ServeExtension(codegraphext.New()); err != nil {
		fmt.Fprintln(os.Stderr, "intelligence.codegraph:", err)
		os.Exit(1)
	}
}
