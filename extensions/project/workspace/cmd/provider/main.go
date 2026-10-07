package main

import (
	"fmt"
	"os"

	"github.com/thinkerqaq/devtool/extensions/project/workspace"
	projectsdk "github.com/thinkerqaq/devtool/sdk/project"
)

func main() {
	provider := &workspace.Provider{}
	if err := projectsdk.Serve(provider); err != nil {
		fmt.Fprintln(os.Stderr, "workspace project extension:", err)
		os.Exit(1)
	}
}
