package main

import (
	"fmt"
	"os"

	dockerenv "github.com/thinkerqaq/devtool/extensions/environment/docker"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

func main() {
	if err := extensioncontract.ServeExtension(dockerenv.New()); err != nil {
		fmt.Fprintln(os.Stderr, "environment.docker:", err)
		os.Exit(1)
	}
}
