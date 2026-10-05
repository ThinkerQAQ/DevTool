package main

import (
	"fmt"
	"os"

	localenv "github.com/thinkerqaq/devtool/extensions/environment/local"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

func main() {
	if err := extensioncontract.ServeExtension(localenv.New()); err != nil {
		fmt.Fprintln(os.Stderr, "environment.local:", err)
		os.Exit(1)
	}
}
