package main

import (
	"fmt"
	"os"

	codecapability "github.com/thinkerqaq/devtool/extensions/capability/code"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

func main() {
	if err := extensioncontract.ServeExtension(codecapability.New()); err != nil {
		fmt.Fprintln(os.Stderr, "capability.code:", err)
		os.Exit(1)
	}
}
