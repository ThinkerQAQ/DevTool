package main

import (
	"fmt"
	"os"

	daggerruntime "github.com/thinkerqaq/devtool/extensions/runtime/dagger"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

func main() {
	if err := extensioncontract.ServeExtension(daggerruntime.New()); err != nil {
		fmt.Fprintln(os.Stderr, "runtime.dagger:", err)
		os.Exit(1)
	}
}
