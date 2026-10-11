# Code intelligence: native tools, not a composite router

[中文](../cn/architecture/code-intelligence.md) · [Architecture Principles](principles.md)

## Current state

The default DevTool profile uses CodeGraph for indexed code intelligence. Serena has been removed. Realtime LSP is not yet configured; `code_context` continues to return indexed context and explicitly reports the missing realtime provider in `not_applied`. The source-code patch gate still requires both indexed and realtime evidence, so it cannot authorize source patches in the default profile.

This is an intermediate state, not a claim that CodeGraph replaces a language server.

## Problem with `code_context`

The current composite mixes distinct operations: repository discovery, symbol lookup, references and diagnostics. It also tokenizes objectives and rescores CodeGraph search results. This changes provider semantics and can rank generic identifiers above the intended symbol. A failed LSP startup can contaminate an otherwise successful indexed result with large error payloads.

More special cases, stop words or fallback rules would preserve the underlying design problem.

## Target design

- **Index and code graph:** expose the selected indexed MCP provider's native tools (e.g., CodeGraph symbol search, curated context, call graph and impact analysis) without rewriting queries or rescoring results.
- **Realtime semantics:** a mature generic MCP–LSP bridge owns language-server sessions, file synchronization, definition, references and diagnostics. Each language server must fail independently. Go uses `gopls`; Kotlin must be verified separately.
- **DevTool boundary:** only configured MCP discovery, namespacing, invocation, lifecycle and transport. Reuse `core/agent/mcpbridge`, which already implements dynamic `ListTools` and `CallTool`. There is no DevTool-side natural-language query planner, no handwritten command-to-command mapping, and no automatic shell fallback.
- **Tool exposure:** choose a provider or its native tool profile in configuration. Avoid listing every upstream method in DevTool source or in per-project Agent instructions. Preserve native tool schemas.
- **Source edit policy:** remove the `code_context` token dependency or replace it with evidence from explicit code operations before retiring the composite. Do not silently weaken edit authorization.

```text
ChatGPT / Local Agent
        |
   DevTool MCP + Tunnel
        |
 Configured MCP providers
     /              \
CodeGraph           LSP–MCP bridge
(native tools)      (native LSP capabilities)
                         |
                  gopls / Kotlin / ...
```

## Migration boundaries

1. Remove Serena code and configuration; keep the indexed provider working and make missing realtime capability explicit. **Completed in this branch.**
2. Evaluate a maintained generic LSP–MCP bridge against real Go and Kotlin workspaces. Do not build another language-server manager inside DevTool.
3. Expose the chosen providers' native MCP tools through the existing bridge, with discovery and filtering driven by provider configuration.
4. Remove `code_context`, its heuristic CodeGraph `discover` implementation and the composite service only after the new path has passed end-to-end checks and the edit gate no longer requires `code_context`.

## Acceptance

Benchmark identical native tool calls through ChatGPT → Tunnel → DevTool and ChatGPT → Desktop Commander → local provider. Measure both end-to-end latency and local execution time, separate cold starts from warm queries, and check correctness for symbol definitions, cross-file references, diagnostics and language-server crash isolation. No GitHub Actions needed for local acceptance.
