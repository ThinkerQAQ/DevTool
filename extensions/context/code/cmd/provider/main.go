package main

import (
	"fmt"
	"os"

	codecontext "github.com/thinkerqaq/devtool/extensions/context/code"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

func main() {
	if err := extensioncontract.ServeExtension(codecontext.New()); err != nil {
		fmt.Fprintln(os.Stderr, "context.code.composite:", err)
		os.Exit(1)
	}
}
