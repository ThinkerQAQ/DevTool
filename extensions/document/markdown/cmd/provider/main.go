package main

import (
	"fmt"
	"os"

	markdownext "github.com/thinkerqaq/devtool/extensions/document/markdown"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

func main() {
	if err := extensioncontract.ServeExtension(markdownext.New()); err != nil {
		fmt.Fprintln(os.Stderr, "document.markdown.goldmark:", err)
		os.Exit(1)
	}
}
