package main

import (
	"fmt"
	"os"

	marksman "github.com/thinkerqaq/devtool/extensions/document/marksman"
	extension "github.com/thinkerqaq/devtool/sdk/extension"
)

func main() {
	if err := extension.ServeExtension(marksman.New()); err != nil {
		fmt.Fprintln(os.Stderr, "document.realtime.marksman:", err)
		os.Exit(1)
	}
}
