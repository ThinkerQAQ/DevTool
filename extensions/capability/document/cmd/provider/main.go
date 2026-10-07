package main

import (
	"fmt"
	"os"

	documentcapability "github.com/thinkerqaq/devtool/extensions/capability/document"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

func main() {
	if err := extensioncontract.ServeExtension(documentcapability.New()); err != nil {
		fmt.Fprintln(os.Stderr, "capability.document:", err)
		os.Exit(1)
	}
}
