# DevTool Architecture Smell Review

Date: 2026-10-07  
Baseline: `main` at `1af157491864ca63c965c9afcd2d0dd141222ac1`

## Conclusion

DevTool still follows the intended direction:

> minimal core + extensible modules + configuration-driven wiring + replaceable providers + self-hosting

This review does **not** recommend a DV3 rewrite. The core registry, extension protocol, service selection, provider boundaries, self-hosting path, and Agent Gateway remain reusable.

The main architectural pressure is now concentrated above the Service/Provider layer: intent-specific Capability handlers are starting to accumulate orchestration logic. If this continues, DevTool can become a higher-level proxy where every new developer intent requires another hard-coded Agent Capability.

The next refactor should therefore tighten boundaries rather than replace the platform.

## Architectural invariants

The review uses these existing rules as constraints:

1. Core stays provider- and project-agnostic.
2. Configuration wires providers and settings; configuration must not become a workflow scripting language.
3. Provider selection is replaceable through configuration.
4. Provider implementation details do not leak into Agent tools.
5. Project-specific semantics stay in Project Extensions.
6. New provider APIs do not automatically become new Agent tools.
7. DevTool self-hosting must keep using the ordinary project/service/provider path.
8. There should be one canonical path per capability.

## Findings

### P1 — Capability handlers are accumulating hard-coded orchestration

Current examples:

- `extensions/capability/scm/scm.go`
  - `scm_checkpoint` performs status -> optional commit -> push.
  - `scm_publish` embeds publishing semantics.
- `extensions/capability/code/code.go`
  - `code_context` decides how indexed search, symbols, references, and diagnostics are composed.
- `extensions/capability/document/document.go`
  - `document_context` owns inspection, whole-document traversal, cursors, relation expansion, and review coverage state.

This is currently manageable, but the growth pattern is risky:

```text
new developer intent
    ->
new Capability Go handler
    ->
new orchestration code
```

That would preserve an intent-level Agent API while still requiring code changes for every new intent.

#### Desired direction

Capability should remain a stable semantic surface, but orchestration should not be duplicated per natural-language intent.

Configuration should continue to select/wire behavior, not encode arbitrary step pipelines.

A future design should prefer:

```text
Agent Tool Descriptor
        ->
stable semantic contract
        ->
service/provider
```

over either of these extremes:

```text
Agent -> provider-native APIs
```

or:

```text
TOML -> generic workflow DSL
```

### P1 — `scm.github` combines generic Git semantics with GitHub forge semantics

The current GitHub SCM provider owns both local Git behavior and hosted-GitHub behavior:

```text
status
add
commit
push
git credential
GitHub REST API
PR create
PR merge
```

This is functional, but it creates duplication pressure for a future GitLab or other forge provider because generic Git behavior would likely be reimplemented.

The boundary should eventually distinguish:

```text
repository / Git semantics
        ->
generic Git implementation

forge semantics
        ->
GitHub / GitLab / other provider
```

This should be done without exposing low-level Git commands to the Agent Gateway.

### P1 — CodeGraph provider contains DevTool-wide repository-discovery policy

`extensions/intelligence/codegraph/codegraph.go` is currently one of the largest production files and contains more than provider adaptation.

It includes:

- keyword extraction and stop words;
- heuristic entry-point discovery;
- candidate scoring;
- path/name/snippet weighting;
- test-file penalties;
- candidate merging/ranking.

Those rules describe how DevTool wants to understand repositories, not only how to call CodeGraph.

Because Sourcegraph does not share the same policy, replacing:

```text
code-indexed -> intelligence.codegraph
```

with:

```text
code-indexed -> intelligence.sourcegraph
```

can change higher-level semantics in addition to replacing the provider.

A cleaner boundary would have indexed providers return normalized semantic candidates while shared ranking/discovery policy lives above provider-specific adapters.

### P1 — Environment provider selection is less orthogonal than code-intelligence selection

Code intelligence follows a clean model:

```toml
[extension.codegraph]
...

[extension.sourcegraph]
...

[service.code-indexed]
provider = "intelligence.codegraph"
```

The environment path currently replaces the logical extension definition in the Railway profile rather than only selecting another already-declared provider.

That means:

```text
CodeGraph <-> Sourcegraph
    = service provider selection

Docker <-> local
    = extension definition replacement + service provider selection
```

The desired shape is:

```text
extension.environment-docker
extension.environment-local

service.environment
    -> selected provider
```

Then a profile only changes the selected provider.

### P1 — Typed Project Command arguments are not implemented

`ProjectHost.Execute` currently rejects commands with typed parameters, and `projectToolProvider` skips parameterized commands entirely.

This works for:

- `build`
- `verify`
- `package`

but creates pressure to add custom Capability handlers for future commands such as:

- deploy with an environment;
- release with a version;
- device operations with a target;
- resource state changes.

Typed Project Command schemas should eventually map generically to CLI/Agent inputs so project semantics do not require new Gateway plumbing.

### P1 — Sourcegraph bypasses generic configuration/credential boundaries

`extensions/intelligence/sourcegraph/sourcegraph.go` reads:

- `SOURCEGRAPH_MCP_URL`
- `SOURCEGRAPH_ACCESS_TOKEN`

directly from process environment.

The endpoint is provider configuration and should come through generic Extension settings.

The secret should flow through a credential boundary rather than each provider inventing its own environment-variable path.

### P1/P2 — Dagger provider bootstraps itself with a remote installer script

`runtime.dagger` currently falls back to:

```sh
curl -fsSL https://dl.dagger.io/dagger/install.sh | sh
```

This conflicts with the platform's stronger dependency model:

```text
DevEnvironment
    ->
versioned immutable toolchain
```

A runtime/provider should ideally consume a declared dependency supplied by the environment or a generic tool-resolution mechanism rather than acting as its own package manager.

This is also a supply-chain concern because execution depends on a remote installer script at runtime.

### P2 — Extension lifecycle is still eager

The repository already documents this issue in the previous microkernel audit.

Configured extensions are generally resolved/loaded before final service use. In addition, extensions with no explicit descriptor ID in configuration are treated as selected during host loading.

The result is:

```text
routing selection is configuration-driven
but lifecycle startup is not fully lazy
```

The eventual optimization should use static extension metadata or another generic discovery mechanism so unselected providers do not need to start just to discover their descriptors.

This is an optimization/ownership problem, not a reason to redesign the registry.

### P2 — Profile application performs shallow replacement

`ApplyProfiles` replaces complete Extension and Service entries.

This can force profiles to repeat loader metadata when they only intend to change one dimension.

Provider declarations plus service-only selection should reduce the need for deep profile merging. A generic deep-merge mechanism should only be added if concrete configuration cases still require it afterward.

### P2 — Readiness maps failures through error-string inspection

Some providers classify missing dependencies using string matching such as:

- executable not found;
- gopls-related text;
- provider-specific error strings.

This is brittle.

The SDK already has structured readiness issue kinds, so lower layers should gradually return typed errors that map directly to:

- missing dependency;
- authorization required;
- configuration required;
- provider unavailable;
- verification failed.

### P2 — CodeGraph workspace fingerprinting can become a hidden O(N) cost

`workspaceFingerprint` walks workspace trees and includes file metadata to detect changes.

With larger repositories, multiple worktrees, or frequent context requests, this can become a significant fixed cost after provider startup itself has been optimized.

The provider needs a cheaper invalidation strategy before this becomes a new benchmark bottleneck.

### P3 — Capability helper code is duplicated

The code/document/scm capabilities each contain similar helpers for:

- tool-definition construction;
- argument decoding;
- result decoding;
- MCP text-result wrapping.

This is not currently an architecture blocker, but it is a sign that a small shared Agent-tool utility may eventually be justified.

Do not extract it until the Capability boundary itself is settled.

## What should remain unchanged

The following parts remain structurally sound and should not be rewritten merely because of the findings above:

- Core project/config discovery;
- Registry and service selection;
- generic extension process protocol;
- Project Extension model;
- Provider replacement through `.devtool.toml`;
- Agent Gateway transport/routing;
- readiness aggregation;
- self-hosting N -> N+1 -> N+2;
- environment contract;
- code indexed/realtime service split;
- credential service/store concept.

## Recommended refactor order

Do not start implementation until the target capability model is agreed.

When implementation begins, prefer this order:

1. Settle the Capability semantic boundary so new developer intents do not imply new hard-coded Agent handlers.
2. Add typed Project Command argument support and generic Agent schema generation.
3. Normalize environment providers so Docker/local selection uses the same provider-selection model as other services.
4. Separate generic Git semantics from hosted forge semantics.
5. Move repository-discovery/ranking policy out of the CodeGraph-specific adapter.
6. Move Sourcegraph configuration/secrets behind generic settings/credential boundaries.
7. Remove provider-owned package installation where the environment/toolchain should own dependencies.
8. Optimize lazy extension lifecycle and provider state/invalidation only after semantic boundaries stabilize.

## Non-goals

This review does **not** recommend:

- a new DevTool major-version rewrite;
- replacing the registry;
- replacing the extension protocol;
- introducing a generic workflow language in TOML;
- exposing provider-native methods to agents;
- adding one Agent tool per Git/GitHub operation;
- preserving parallel legacy control paths.

The intended result is a smaller and more orthogonal control plane, not another abstraction layer.
