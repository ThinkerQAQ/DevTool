package main

import (
	"fmt"
	"os"

	projectsdk "github.com/thinkerqaq/devtool/sdk/project"
	"github.com/thinkerqaq/devtool/devcontrol"
)

func main() {
	if err := projectsdk.Serve(devcontrol.Provider{}); err != nil {
		fmt.Fprintln(os.Stderr, "devtool project extension:", err)
		os.Exit(1)
	}
}
