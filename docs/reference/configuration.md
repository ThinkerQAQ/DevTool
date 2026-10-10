# Configuration reference

[中文](../cn/reference/configuration.md) · [Documentation](../index.md)

DevTool resolves `.devtool.toml` at the project root. It is a **declarative binding file**: extension code supplies behavior, configuration selects and connects it.

## Project metadata

```toml
version = 1

[project]
name = "DevTool"
```

## Loadable extensions

```toml
[extension.project]
loader = "go"

[extension.project.loader_config]
module = "./devcontrol"
package = "./cmd/provider"
```

Extension settings belong in `[extension.<name>.settings]`. A provider is not activated merely by registering its extension; a service binding must select it.

## Service binding

```toml
[service.code-indexed]
provider = "intelligence.codegraph"

[service.code-realtime]
provider = "intelligence.lsp.serena"
```

The actual DevTool repository also selects `workspace`, `environment`, `tooling-environment`, `portable-runtime`, `document-context`, `diagram-context`, `credential`, and `scm` services. Consult [the live config](../../.devtool.toml) for every binding.

## Profiles

Profiles override selected sections without editing the base configuration.

```toml
[profile.local.service.environment]
provider = "environment.local"

[profile.sourcegraph.service.code-indexed]
provider = "intelligence.sourcegraph"
```

```bash
DEVTOOL_PROFILES=local devtool project inspect --json
DEVTOOL_PROFILES=local,sourcegraph devtool config validate
```

Profile combinations must be validated. Provider settings and credentials remain provider-owned; do not hard-code them in Core.

## Code workspaces

```toml
[code]
workspaces = ["."]
```

CodeGraph works on indexed/structural repository context. Serena/LSP supplies realtime semantics. A provider change should preserve the stable agent capability contract.

## Validate

```bash
devtool config path
devtool config validate
devtool project inspect --json
devtool init --json
```

The first three commands detect parsing, bindings, and project wiring issues. `init` additionally checks active provider readiness. Missing executables and credentials are environment issues, not signals to build a second dependency manager into DevTool.

## Boundaries

Do not put shell pipelines, application logic, access tokens, or provider-specific branches in configuration. Use a Project Extension for project behavior and a provider for integration behavior.
