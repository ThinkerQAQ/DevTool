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

## Resolved — Agent Capability is intent-level, not a Provider proxy

The previous provider-native tool surface has been removed.

Agent-facing code intelligence now exposes a single engineering-intent capability:

```text
code_context
```

Internally it can compose:

```text
selected code-indexed provider
  -> search / indexed context

selected code-realtime provider
  -> symbols
  -> references
  -> diagnostics
```

This establishes the anti-proxy invariant:

> adding a Provider method does not automatically add an Agent Tool.

Provider Contracts may grow with fine-grained semantic operations, while the public Agent surface only grows when a new stable engineering intent exists.

Provider-native tools such as `codegraph_*`, `sourcegraph_*`, `find_symbol`, and individual `code_search/code_references/code_diagnostics` wrappers are not exposed by default.

## Resolved — Environment configuration is provider-owned

Docker image selection no longer appears in:

```text
Agent Session
CodeIntelligence Workspace
Environment CommandRequest
```

Generic Extension configuration now supports opaque Provider settings:

```toml
[extension.environment.settings]
image = "..."
```

The Host transports these settings through the generic `extension.configure` protocol without interpreting Provider-specific fields.

`environment.docker` owns and validates `settings.image`; `environment.local` does not need to know the concept exists.

The same configuration channel is available to normal Process Extensions and Project Extensions.

## Resolved — Extension loader metadata is opaque to Core

Core configuration no longer contains Go-specific `module/package` fields.

Current schema:

```toml
[extension.example]
loader = "go"

[extension.example.loader_config]
module = "."
package = "./..."
```

Core only understands:

```text
loader
loader_config (opaque)
settings      (opaque)
```

The selected loader adapter owns interpretation of `loader_config`.

Today the implemented loader is `go`, and only `adapters/extensionloader` understands `module/package`. Future prebuilt/remote loaders can use different loader config without widening Core's schema.

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
| Intent-level Agent Capability API | Good | `code_context` composes internal semantic services; provider-native tools stay private |
| Provider-owned environment config | Good | generic extension settings; Docker owns image configuration |
| Loader extensibility | Good | Core owns opaque loader config; Go metadata is adapter-private |
| Lazy extension lifecycle | Optional optimization | not required for correctness |

## Recommended order

1. Keep the Agent-facing capability surface intent-level; do not reintroduce one-tool-per-provider-operation wrappers.
2. Move legacy code-intelligence lifecycle operations such as raw MCP/query/sync further behind provider/runtime boundaries.
3. Reduce built-in domain handling in the main CLI through capability/descriptor-driven dispatch where it adds real value.
4. Then optimize lazy extension startup and provider caches/state persistence.

The architecture should continue to evolve by tightening boundaries, not by adding another orchestration/platform layer.
