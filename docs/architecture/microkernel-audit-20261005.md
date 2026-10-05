# DevTool Microkernel Architecture Audit

Date: 2026-10-05

## Executive conclusion

DevTool is now structurally aligned with the intended direction:

> micro-kernel + extensibility + configuration-driven provider selection

The architecture is **not yet complete**. Provider registration and selection are now replaceable, but the public Agent capability surface still exposes provider-native tools, and environment-specific settings still leak into higher-level code-intelligence/session contracts.

The remaining work is refinement rather than a rewrite.

## Verified architecture

### Core is provider-agnostic

Production Core/SDK code does not contain hard-coded provider IDs such as:

- `intelligence.codegraph`
- `intelligence.sourcegraph`
- `environment.docker`
- `environment.local`
- `runtime.dagger`
- `scm.github`

Provider IDs live in configuration and extension implementations.

### Multiple providers per capability

The Registry supports multiple implementations for one service:

```text
code-indexed
  -> intelligence.codegraph
  -> intelligence.sourcegraph

code-realtime
  -> intelligence.lsp.serena
```

Project configuration selects the active provider.

### Agent tools follow selected providers

A provider that implements a service only exposes Agent tools when it is selected for at least one of its services.

This fixes the previous behavior where Sourcegraph could be selected while CodeGraph tools were still exposed and CodeGraph was still started.

### Profiles are composable

Deployment and capability choices are independent:

```text
DEVTOOL_PROFILES=railway
DEVTOOL_PROFILES=railway,sourcegraph
```

This allows:

- `railway` to select `environment.local`
- `sourcegraph` to select `intelligence.sourcegraph`

without coupling those dimensions.

### SDK dependency direction

Stable project contracts and service invocation contracts now live under:

```text
sdk/contract
sdk/service
```

Core aliases/validates them, rather than forcing SDK packages and extensions to import `core/*`.

Target dependency direction:

```text
extensions
   |
   v
  SDK
   ^
   |
  Core
```

not:

```text
SDK -> Core
```

### Runtime image layering

```text
dev-base
  -> shared language/toolchain dependencies

devtool-runtime:<commit>
  -> FROM dev-base
  -> versioned DevTool binary
```

Railway runs a matching runtime-image SHA and workspace SHA.

The runtime no longer builds DevTool at startup and no longer mutates `.devtool.toml`.

## Railway/provider verification

### CodeGraph configuration

```text
DEVTOOL_PROFILES=railway
```

Verified:

- Railway online
- Serena/gopls active
- CodeGraph active
- Agent Gateway smoke passes

### Sourcegraph configuration

```text
DEVTOOL_PROFILES=railway,sourcegraph
```

With a Sourcegraph endpoint configured but no valid authentication:

- CodeGraph no longer starts
- Serena/gopls remains active
- the Sourcegraph path fails at remote MCP authentication as expected

This proves provider selection reaches both service routing and Agent tool routing.

A successful Sourcegraph query still requires a valid Sourcegraph instance and OAuth/access token.

---

# Remaining architecture gaps

## P0 — Stable Agent Capability API is still missing

Current MCP tools are still provider-native.

Examples:

```text
codegraph_*
sourcegraph_*
find_symbol
find_referencing_symbols
...
```

Therefore switching the provider changes the tool names/schema visible to ChatGPT.

This violates the intended invariant:

> provider replacement must not change the consumer-facing Capability API.

Desired structure:

```text
ChatGPT / Agent
      |
      v
Stable Capability Tools
      |
      +-- code.search
      +-- code.definition
      +-- code.references
      +-- code.diagnostics
      +-- code.impact
      |
      v
Capability composition / adapters
      |
      +-- realtime -> LSP
      +-- indexed  -> CodeGraph / Sourcegraph
```

Provider-native MCP should become an internal implementation detail or an explicitly diagnostic/raw surface.

This is the most important remaining architecture change.

## P1 — Environment configuration leaks into higher layers

Current generic structures still contain:

```text
Agent Session.EnvironmentImage
CodeIntelligence Workspace.EnvironmentImage
Environment CommandRequest.Image
```

An image is a Docker-style implementation concern.

It does not naturally apply to:

- environment.local
- SSH
- Coder
- Kubernetes with a separately defined runtime
- other future providers

Desired direction:

```text
Code Intelligence / Agent
        |
        | project/workspace only
        v
Environment Service
        |
        v
selected Environment Provider
        |
        +-- provider-owned settings
```

Environment settings should belong to the selected environment provider, not to code intelligence or generic Agent session contracts.

## P1 — Extension configuration is still Go-loader shaped

Current extension configuration uses:

```toml
type = "go"
module = "."
package = "./..."
```

Core correctly injects the executable resolver, so the micro-kernel itself does not compile Go directly.

However the project configuration schema still assumes build metadata shaped around the current Go loader.

Longer-term desired model:

```text
Extension Contract
  -> executable/process protocol

Loader adapters
  -> Go package builder
  -> prebuilt executable
  -> remote extension
  -> future loaders
```

Adding a new extension implementation mechanism should not require widening the core config schema every time.

## P1 — Code Intelligence service methods are provider-shaped

The internal service contract still exposes:

```text
mcp
sync
query
verify
```

These operations reflect current provider mechanics rather than stable engineering semantics.

Target:

```text
definition
references
symbols
diagnostics
search
callers
callees
impact
```

Lifecycle operations such as MCP startup or index synchronization should stay private to providers/runtime management.

## P1 — Code CLI is still a built-in domain surface

`cmd/devtool/code.go` directly knows the realtime/indexed code-intelligence domain.

This is acceptable as a transitional first-class DevTool capability, but a fully extensible control layer should eventually support capability-contributed CLI/API surfaces without editing the main CLI switch.

The core commands should remain small:

```text
project
config
agent
capability/extension dispatch
```

## P2 — Unselected extensions are eagerly loaded

All configured extension processes are currently loaded before service selection.

This is functionally correct and simplifies discovery/registration, but it adds startup/build overhead.

Future optimization:

- static extension metadata/manifest
- select provider first
- lazily start only selected/runtime-required extensions

Do this only after stable Capability contracts are finished.

## P2 — Indexed-state persistence remains provider-specific work

CodeGraph currently reports memory-only storage on Railway.

This affects performance, not the micro-kernel boundary.

Provider state should use a generic DevTool state directory contract, for example:

```text
DEVTOOL_STATE_DIR
  / codegraph
  / serena
  / ...
```

without Core knowing CodeGraph-specific persistence semantics.

---

# Architecture scorecard

| Principle | Status | Notes |
|---|---|---|
| Micro-kernel Core | Good | Core no longer contains concrete provider selection logic |
| Replaceable service providers | Good | Multi-provider Registry + config selection works |
| Configuration-driven deployment | Good | composable profiles verified on Railway |
| Runtime/provider separation | Good | local/docker selection proven without changing intelligence implementations |
| SDK/Core dependency direction | Good after current refactor | contracts promoted to SDK |
| Agent tools respect provider selection | Good after current fix | verified with Sourcegraph switch |
| Stable consumer-facing Capability API | **Not complete** | provider-native MCP tools still leak |
| Provider-owned environment config | **Not complete** | EnvironmentImage leaks upward |
| Loader extensibility | Partial | current config is Go-loader shaped |
| Lazy extension lifecycle | Optional optimization | not required for correctness |

## Recommended order

1. Build stable Agent-facing Code Capability tools.
2. Make CodeGraph / Sourcegraph / LSP adapters implement those semantic operations.
3. Stop exposing provider-native MCP tools by default.
4. Move environment image/settings into environment-provider configuration.
5. Generalize extension loader configuration.
6. Only then optimize lazy loading and provider caches.

The architecture should continue to evolve by tightening boundaries, not by adding another orchestration/platform layer.
