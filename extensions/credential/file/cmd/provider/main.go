package main

import (
	"fmt"
	"os"

	filecredential "github.com/thinkerqaq/devtool/extensions/credential/file"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

func main() {
	if err := extensioncontract.ServeExtension(filecredential.New()); err != nil {
		fmt.Fprintln(os.Stderr, "credential.store.file:", err)
		os.Exit(1)
	}
}
