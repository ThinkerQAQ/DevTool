package main

import (
	"fmt"
	"os"

	githubcredential "github.com/thinkerqaq/devtool/extensions/credential/github"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

func main() {
	if err := extensioncontract.ServeExtension(githubcredential.New()); err != nil {
		fmt.Fprintln(os.Stderr, "credential.github:", err)
		os.Exit(1)
	}
}
