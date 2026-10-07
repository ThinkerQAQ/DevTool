package main

import (
	"fmt"
	"os"

	contentrelations "github.com/thinkerqaq/devtool/extensions/document/relations/content"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

func main() {
	if err := extensioncontract.ServeExtension(contentrelations.New()); err != nil {
		fmt.Fprintln(os.Stderr, "document.relations.content:", err)
		os.Exit(1)
	}
}
