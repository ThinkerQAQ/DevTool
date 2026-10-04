package main

import (
	"fmt"
	"os"

	serenaext "github.com/thinkerqaq/devtool/extensions/intelligence/serena"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

func main() {
	if err := extensioncontract.ServeExtension(serenaext.New()); err != nil {
		fmt.Fprintln(os.Stderr, "intelligence.lsp.serena:", err)
		os.Exit(1)
	}
}
