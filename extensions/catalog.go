package extensions

import (
	"fmt"

	"github.com/thinkerqaq/devtool/core/extension"
	daggerruntime "github.com/thinkerqaq/devtool/extensions/runtime/dagger"
)

func Resolve(source string) (extension.Extension, error) {
	switch source {
	case "builtin:runtime.dagger":
		return daggerruntime.New(), nil
	default:
		return nil, fmt.Errorf("unknown built-in extension source %q", source)
	}
}
