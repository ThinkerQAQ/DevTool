package main

import (
	"fmt"
	"os"

	adbext "github.com/thinkerqaq/devtool/extensions/native/adb"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

func main() {
	if err := extensioncontract.ServeExtension(adbext.New()); err != nil {
		fmt.Fprintln(os.Stderr, "native.adb:", err)
		os.Exit(1)
	}
}
