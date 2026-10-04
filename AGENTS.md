# DevTool Agent Contract

DevTool is the canonical control plane for project-aware development operations.

## Code intelligence

Use DevTool rather than starting CodeGraph, Serena, gopls, or other language servers directly when DevTool exposes the operation.

- Semantic/compiler-grade navigation and diagnostics: `devtool code lsp mcp`.
- Structural graph, callers/callees, dependency topology, and impact analysis: `devtool code graph mcp`.
- Strong-consistency graph checks after edits: `devtool code graph query <tool> [json-args]`.
- Environment and workspace health: `devtool code doctor` and `devtool code verify`.

For DevTool's own Stage 0 bootstrap, `go run ./cmd/devtool ...` is the minimal allowed entry point. It launches DevTool itself; it is not a parallel project control plane.

Generic repository primitives such as reading/editing files and Git status/diff/log remain direct.

Code-intelligence executables are supplied by DevEnvironment. DevTool discovers and routes them; repositories do not install or pin their own CodeGraph/Serena binaries.
