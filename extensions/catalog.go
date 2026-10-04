package extensions

import (
	"fmt"

	dockerenv "github.com/thinkerqaq/devtool/extensions/environment/docker"
	codegraphext "github.com/thinkerqaq/devtool/extensions/intelligence/codegraph"
	serenaext "github.com/thinkerqaq/devtool/extensions/intelligence/serena"
	daggerruntime "github.com/thinkerqaq/devtool/extensions/runtime/dagger"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

func Resolve(source string) (extensioncontract.Extension, error) {
	switch source {
	case "environment.docker":
		return dockerenv.New(), nil
	case "runtime.dagger":
		return daggerruntime.New(), nil
	case "intelligence.codegraph":
		return codegraphext.New(), nil
	case "intelligence.lsp.serena":
		return serenaext.New(), nil
	default:
		return nil, fmt.Errorf("unknown built-in extension source %q", source)
	}
}
