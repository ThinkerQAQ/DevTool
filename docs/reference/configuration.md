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

## Document and diagram scopes

The repository also configures its existing Markdown and Marksman providers at project scope so root-level `README.md` and `AGENTS.md` can be inspected before edits:

```toml
[extension.document-markdown.settings]
roots = ["."]

[extension.document-realtime.settings]
workspace_roots = ["."]
```

A Markdown edit requires `document_context`; embedded Mermaid/PlantUML edits additionally require `diagram_context`. Project logs and journald are queried through the read-only `log_context`.

## Code workspaces

```toml
[code]
workspaces = ["."]
```

CodeGraph works on indexed/structural repository context. Serena/LSP supplies realtime semantics. A provider change should preserve the stable agent capability contract.

## Controlled agent edits

```toml
[agent.policy]
enabled = true
protected_branches = ["main", "master"]
max_patch_bytes = 262144
context_ttl_seconds = 600
```

Disabled by default. The policy exposes `workspace_apply_patch` on the existing MCP Gateway and requires a same-process, one-time `code_context` token. `main` and `master` are always protected even with additional custom branch names. For the full workflow see [Controlled Code Edits](../how-to/controlled-edits.md).

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
