package main

import (
	"fmt"
	merman "github.com/thinkerqaq/devtool/extensions/diagram/merman"
	extension "github.com/thinkerqaq/devtool/sdk/extension"
	"os"
)

func main() {
	if err := extension.ServeExtension(merman.New()); err != nil {
		fmt.Fprintln(os.Stderr, "diagram.intelligence.merman:", err)
		os.Exit(1)
	}
}
