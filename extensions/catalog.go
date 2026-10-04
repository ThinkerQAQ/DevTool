package extensions

import (
	"fmt"

	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
	daggerruntime "github.com/thinkerqaq/devtool/extensions/runtime/dagger"
)

func Resolve(source string) (extensioncontract.Extension, error) {
	switch source {
	case "runtime.dagger":
		return daggerruntime.New(), nil
	default:
		return nil, fmt.Errorf("unknown built-in extension source %q", source)
	}
}
