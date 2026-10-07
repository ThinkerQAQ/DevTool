package main

import (
	"fmt"
	"os"

	documentcontext "github.com/thinkerqaq/devtool/extensions/context/document"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

func main() {
	if err := extensioncontract.ServeExtension(documentcontext.New()); err != nil {
		fmt.Fprintln(os.Stderr, "context.document.composite:", err)
		os.Exit(1)
	}
}
