# DevTool Agent Contract

DevTool is the canonical control plane for project-aware development operations.

## Agent entry point

Agents connect to one configured entry point: `devtool agent mcp`.

The Agent Gateway must not manually register provider-specific tools. Configured extensions contribute tools through the Extension Registry, and the gateway dynamically discovers and routes them. Adding or replacing a provider must not require changing the gateway.

## Code intelligence

Use the tools exposed by the DevTool Agent Gateway rather than starting CodeGraph, Serena, gopls, or other language servers directly.

- Semantic/compiler-grade navigation, edits, and diagnostics come from the configured LSP extension.
- Structural graph, callers/callees, dependency topology, and impact analysis come from the configured graph extension.
- Environment and workspace health remain available through `devtool code doctor` and `devtool code verify`.

For DevTool's own Stage 0 bootstrap, `go run ./cmd/devtool ...` is the minimal allowed entry point. It launches DevTool itself; it is not a parallel project control plane.

Generic repository primitives such as reading files and Git status/diff/log may remain direct until a configured DevTool capability exists for the operation.

Runtime executables are supplied by configured environment/runtime providers. Repositories do not install or pin their own CodeGraph, Serena, or language-server binaries.
