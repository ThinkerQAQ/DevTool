# Configuration Reference

DevTool discovers project configuration from `.devtool.toml`.

Configuration has one responsibility:

> discover, select and wire extensions/services without embedding project workflow logic.

The current configuration version is `1`.

## Minimal shape

```toml
version = 1

[project]
name = "Example"

[extension.project]
loader = "go"

[extension.project.loader_config]
module = "./devcontrol"
package = "./cmd/provider"
```

A useful project normally also wires environment/runtime/capability providers through `[extension.*]` and `[service.*]`.

---

## `version`

```toml
version = 1
```

Required.

DevTool rejects unsupported configuration versions.

---

## `[project]`

```toml
[project]
name = "DevTool"
```

Fields:

| Field | Required | Meaning |
| --- | --- | --- |
| `name` | yes | Human/project identity used by the control plane. |

---

## `[extension.<name>]`

Declares an extension instance that DevTool can load.

Example:

```toml
[extension.codegraph]
loader = "go"

[extension.codegraph.loader_config]
module = "."
package = "./extensions/intelligence/codegraph/cmd/provider"

[extension.codegraph.settings]
some_setting = "value"
```

Fields:

| Field | Required | Meaning |
| --- | --- | --- |
| `loader` | yes | Loader mechanism used to resolve/start the extension. |
| `loader_config` | no | Loader-specific configuration. |
| `settings` | no | Provider/extension configuration passed to configurable extensions. |

The extension table key is a local configuration name. Stable provider identity is declared by the loaded extension itself.

### Current Go loader shape

The DevTool repository currently uses:

```toml
[extension.<name>.loader_config]
module = "."
package = "./path/to/cmd/provider"
```

The loader/build adapter owns Go build mechanics. Core does not contain provider-specific build switches.

---

## `[service.<name>]`

Binds a stable service name to a provider ID.

Example:

```toml
[service.code-indexed]
provider = "intelligence.codegraph"

[service.code-realtime]
provider = "intelligence.lsp.serena"

[service.scm]
provider = "scm.github"

[service.document-context]
provider = "context.document.composite"

[service.document-structure]
provider = "document.markdown.goldmark"
```

Fields:

| Field | Required | Meaning |
| --- | --- | --- |
| `provider` | yes | Extension/provider ID selected to implement the service. |

This is the main replacement boundary.

For an existing capability, switching providers should normally happen here or through a profile.

For document intelligence, a content repository can wire the stable service independently of the concrete parser:

```toml
[extension.document-capability]
loader = "go-module"

[extension.document-capability.loader_config]
module = "github.com/thinkerqaq/devtool"
version = "<pinned-commit>"
package = "./extensions/capability/document/cmd/provider"

[extension.document-context]
loader = "go-module"

[extension.document-context.loader_config]
module = "github.com/thinkerqaq/devtool"
version = "<pinned-commit>"
package = "./extensions/context/document/cmd/provider"

[extension.document-context.settings]
review_max_lines = 300

[extension.document-markdown]
loader = "go-module"

[extension.document-markdown.loader_config]
module = "github.com/thinkerqaq/devtool"
version = "<pinned-commit>"
package = "./extensions/document/markdown/cmd/provider"

[extension.document-markdown.settings]
roots = ["src/content"]

[extension.document-relations]
loader = "go-module"

[extension.document-relations.loader_config]
module = "github.com/thinkerqaq/devtool"
version = "<pinned-commit>"
package = "./extensions/document/relations/content/cmd/provider"

[extension.document-relations.settings]
articles = "src/content/articles"
series = "src/content/series"
notes = "src/content/notes"
projects = "src/content/projects"

[service.document-context]
provider = "context.document.composite"

[service.document-structure]
provider = "document.markdown.goldmark"

[service.document-relations]
provider = "document.relations.content"
```

The Agent surface remains `document_context`. The thin Capability delegates to `document-context`; bounded review/cursor/relation composition lives in the selected document-context provider, while parser-specific structure and content-relation behavior remain behind their own services. `review_max_lines` configures the document-context provider and defaults to 300 when omitted. Repositories that do not need cross-document relations can omit `document-relations` entirely.

A content-only repository can use the reusable `project.workspace` Project Extension instead of creating a fake project-local build/runtime implementation:

```toml
[project]
name = "Docs"

[extension.project]
loader = "go-module"

[extension.project.loader_config]
module = "github.com/thinkerqaq/devtool"
version = "<pinned-commit>"
package = "./extensions/project/workspace/cmd/provider"

[extension.project.settings]
name = "Docs"
```

---

## `[code]`

Configures code-intelligence workspace roots.

```toml
[code]
workspaces = [".", "./devcontrol"]
```

`workspaces` contains project-relative or absolute paths.

Relative paths are resolved against the discovered project root.

When no workspace is configured, DevTool uses the project root.

---

## `[ui]`

Declares generic control-surface features.

```toml
[ui]
features = ["environment", "jobs", "logs"]
```

Current configuration model:

| Field | Required | Meaning |
| --- | --- | --- |
| `features` | no | Feature IDs consumed by the control-surface layer. |

UI configuration enables generic features; it should not contain project-specific pages or a general-purpose UI scripting language.

---

## Profiles

Profiles override extension and service wiring without mutating the base configuration.

Example:

```toml
[profile.railway.service.environment]
provider = "environment.local"
```

The base DevTool configuration already loads `environment.local` for `tooling-environment`, so the Railway profile only changes the selected project `environment` provider. It does not load a duplicate local extension.

Activate:

```bash
DEVTOOL_PROFILES=railway devtool project inspect --json
```

### Composition

Profiles are comma-separated and applied left to right:

```bash
DEVTOOL_PROFILES=railway,sourcegraph ...
```

For the same extension/service key, a later profile replaces an earlier value.

Current profile configuration can override:

- `extension`
- `service`

It does not independently overlay `project`, `code`, or `ui`.

Unknown profile names are rejected.

---

## Current DevTool repository wiring

The repository currently uses this service graph conceptually:

```text
environment
  -> environment.docker

tooling-environment
  -> environment.local

portable-runtime
  -> runtime.dagger

code-indexed
  -> intelligence.codegraph

code-realtime
  -> intelligence.lsp.serena

scm
  -> scm.github
```

The `sourcegraph` profile replaces only `code-indexed`:

```text
code-indexed
  -> intelligence.sourcegraph
```

The `railway` profile changes only the project `environment` service selection to `environment.local` because the Railway deployment is already inside the remote runtime container. `tooling-environment` remains local in both profiles, so CodeGraph/Serena do not depend on Docker availability.

These are provider choices, not Project Extension changes.

---

## Full repository example

The current shape is equivalent to:

```toml
version = 1

[project]
name = "DevTool"

[extension.project]
loader = "go"

[extension.project.loader_config]
module = "./devcontrol"
package = "./cmd/provider"

[extension.environment]
loader = "go"

[extension.environment.loader_config]
module = "."
package = "./extensions/environment/docker/cmd/provider"

[extension.runtime]
loader = "go"

[extension.runtime.loader_config]
module = "."
package = "./extensions/runtime/dagger/cmd/provider"

[extension.code-capability]
loader = "go"

[extension.code-capability.loader_config]
module = "."
package = "./extensions/capability/code/cmd/provider"

[extension.codegraph]
loader = "go"

[extension.codegraph.loader_config]
module = "."
package = "./extensions/intelligence/codegraph/cmd/provider"

[extension.lsp]
loader = "go"

[extension.lsp.loader_config]
module = "."
package = "./extensions/intelligence/serena/cmd/provider"

[extension.scm]
loader = "go"

[extension.scm.loader_config]
module = "."
package = "./extensions/scm/github/cmd/provider"

[service.environment]
provider = "environment.docker"

[service.tooling-environment]
provider = "environment.local"

[service.portable-runtime]
provider = "runtime.dagger"

[service.code-indexed]
provider = "intelligence.codegraph"

[service.code-realtime]
provider = "intelligence.lsp.serena"

[service.scm]
provider = "scm.github"

[code]
workspaces = ["."]
```

Refer to the repository's actual `.devtool.toml` for the complete current provider/profile list.

---

## What should not go in configuration

Avoid:

```toml
build = "go build ./..."
verify = "go test ./..."
deploy = "some shell pipeline"
```

That turns configuration into a second workflow language.

Instead:

```text
Project Extension
  -> stable service
  -> selected provider
```

Configuration chooses the implementation; code owns behavior.

---

## Validation

Run:

```bash
devtool config validate
```

or, while bootstrapping DevTool itself:

```bash
go run ./cmd/devtool config validate
```

Then inspect the resolved result:

```bash
devtool project inspect --json
```

When profiles are active, validation and inspection use the profile-applied configuration.

## Related

- [Quick Start](../quick-start.md)
- [Agent Guide](../agent-guide.md)
- [Architecture Principles](../architecture/principles.md)
- [CLI Reference](cli.md)
