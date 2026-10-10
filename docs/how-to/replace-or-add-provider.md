# Replace or add a provider

[中文](../cn/how-to/replace-or-add-provider.md) · [Documentation](../index.md)

A provider is the implementation behind a stable service contract. Selection belongs in TOML; behavior belongs in code.

## Switch to an existing provider

The repository selects CodeGraph for indexed intelligence:

```toml
[service.code-indexed]
provider = "intelligence.codegraph"
```

Its `sourcegraph` profile replaces that binding:

```toml
[profile.sourcegraph.service.code-indexed]
provider = "intelligence.sourcegraph"
```

Use it without changing any agent tool:

```bash
DEVTOOL_PROFILES=sourcegraph devtool config validate
DEVTOOL_PROFILES=sourcegraph devtool project inspect --json
DEVTOOL_PROFILES=sourcegraph devtool code doctor
```

Sourcegraph itself must be configured and reachable for verification to pass.

## Add an implementation

1. Inspect the service/SDK contract and check whether a maintained provider, SDK, or protocol already covers the requirement.
2. Implement the new provider as a loadable extension; keep external tool arguments and credentials in the provider.
3. Register the extension and its settings in `.devtool.toml`.
4. Bind the service to the provider, either in the base configuration or a named profile.
5. Run `config validate`, `project inspect --json`, and the relevant capability/Project Extension verification.
6. Confirm the MCP capability name and contract stay unchanged for callers.

Example Go extension registration (replace the placeholders with the actual module/package):

```toml
[extension.example]
loader = "go"

[extension.example.loader_config]
module = "."
package = "./extensions/example/cmd/provider"

[profile.example.service.code-indexed]
provider = "intelligence.example"
```

The provider must register the indicated provider ID and satisfy the `code-indexed` contract. The TOML alone does not create an implementation.

## When Core may change

Only change Core for cross-provider mechanisms, such as routing, lifecycle, discovery, or protocol invariants. Adding a provider-specific switch to Core is not a supported extension strategy.

Further reading: [Configuration Reference](../reference/configuration.md) and [Architecture Principles](../architecture/principles.md).
