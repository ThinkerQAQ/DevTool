# Agent Guide

This guide describes the intended development workflow for an AI agent working in a repository managed by DevTool.

The core rule is simple:

> Discover project intent through DevTool, use stable capabilities, keep provider details behind configuration and contracts.

## 1. Establish project context

Run:

```bash
go run ./cmd/devtool config validate
go run ./cmd/devtool project inspect --json
```

The project inspection output tells you which Project Extension is active and which project commands/resources are available.

Do not infer project semantics from filenames when DevTool already declares them.

## 2. Check capability health

For code work:

```bash
go run ./cmd/devtool code doctor
go run ./cmd/devtool code verify
```

If a configured provider is unhealthy, fix the provider/environment/configuration path instead of silently bypassing DevTool with a parallel toolchain.

## 3. Use the Agent Gateway

Start:

```bash
go run ./cmd/devtool agent mcp --context codex
```

The gateway is the stable agent-facing boundary.

Use intent-level capabilities:

### Understand code

Use `code_context`.

It may compose indexed and realtime providers, but the agent should not care whether the configured implementation is CodeGraph, Sourcegraph, Serena, gopls, or another provider.

Typical objective:

```text
Understand how ProjectHost resolves configured services and routes them to extensions.
```

Ask for the engineering context you need, not a provider method.

### Understand documents

Use `document_context`.

The normal long-document workflow is outline-first:

```text
document_context(path=document)
  -> frontmatter + complete outline + exact section ranges

document_context(path=document, section="...", include_content=true)
  -> one bounded source section
```

Review the relevant sections before making whole-document conclusions. The agent should not depend on Goldmark, Marksman, Serena, or another concrete document provider.

### Execute project operations

Use `project_<command>` tools discovered from the Project Extension.

Examples for DevTool:

- `project_build`
- `project_verify`
- `project_package`

These preserve project semantics and route execution through the configured service graph.

### Publish source changes

Use `scm_publish` when available.

SCM publishing belongs to DevTool because authentication, provider choice and fallback policy must be deterministic and configuration-driven.

Do not make the agent invent a GitHub-specific publication workflow when the capability is configured.

## 4. Decide where a change belongs

Use this decision tree.

```text
Need new behavior
      |
      v
Can configuration select/wire it?
      |
      +-- yes --> change .devtool.toml/profile
      |
      v
Does an existing contract express the intent?
      |
      +-- yes --> implement/replace provider
      |
      v
Is it project-specific orchestration?
      |
      +-- yes --> Project Extension
      |
      v
Is it a new stable cross-project capability?
      |
      +-- yes --> Capability/Service Contract + provider
      |
      v
Only then consider Core mechanism
```

Core should not know project names, provider IDs, container commands, GitHub API details, CodeGraph arguments, Serena behavior, or Dagger semantics.

## 5. Provider replacement test

Before accepting a design, ask:

> If I replace this provider tomorrow, what has to change?

Good result:

```text
provider implementation
+ .devtool.toml/profile
```

Suspicious result:

```text
Core switch
+ Agent Gateway edits
+ Project Extension edits
+ provider implementation
+ documentation rewrite
```

For an existing capability, provider replacement should remain local.

## 6. Keep Agent tools coarse-grained

Provider contracts can be fine-grained internally.

Agent tools should model stable engineering intent.

Good:

```text
code_context
document_context
scm_publish
project_verify
```

Avoid mechanically mirroring provider APIs:

```text
codegraph_find_references
serena_symbols
sourcegraph_search
github_create_tree
```

A new provider method does not justify a new agent tool by itself.

## 7. Respect lifecycle ownership

Long-lived provider clients/processes belong to the lifecycle of their owning ProjectHost/extension/provider, not to an individual RPC request.

Do not create a reusable provider process from a request-scoped context that is canceled as soon as one `tools/list` or `tools/call` completes.

See [Runtime Lifecycle](runtime-lifecycle.md) for the detailed ownership model.

## 8. Change in small steps

Preferred development rhythm:

```text
inspect
  -> change one coherent thing
  -> commit
  -> push
  -> change the next coherent thing
  -> commit
  -> push
  -> concentrated verification
```

This makes progress visible and keeps rollback/review boundaries clean.

Do not require a full regression suite after every tiny edit. Run focused verification after a coherent batch, then the full delivery path before merge.

## 9. Fix structure, not symptoms

When a failure reveals a contract or architecture problem:

- fix the ownership/boundary/root cause;
- remove obsolete paths;
- avoid compatibility shims unless they are an explicit product requirement;
- avoid parallel implementations for the same engineering capability.

A short-term patch that leaves two control planes is normally worse than a focused refactor.

## 10. Tests

Add tests when they protect meaningful behavior:

- contract compatibility;
- provider replacement;
- lifecycle ownership;
- routing;
- self-hosting;
- side-effect boundaries;
- a delivery regression that is likely to recur.

Do not add tests mechanically when they do not improve confidence in the changed behavior.

## 11. Self-hosting acceptance

DevTool itself is the strongest architecture test.

A meaningful DevTool change should preserve:

```text
DevTool N
  -> configured Project Extension
  -> configured services/providers
  -> build DevTool N+1
  -> N+1 inspect/verify
  -> package
```

The repository CI also validates N+1 to N+2 self-hosting.

If DevTool requires a special hidden path to develop itself, the architecture is incomplete.

## 12. Remote environments

A remote runtime such as Railway should change provider selection through configuration/profile, not create a second architecture.

For the current repository:

```bash
DEVTOOL_PROFILES=railway ...
```

selects the local environment provider appropriate inside the already-running remote container.

The same project commands and contracts remain valid.

## 13. Final pre-merge checklist

Before merge:

- configuration validates;
- project inspection resolves;
- code intelligence verifies when relevant;
- document intelligence is exercised on a representative document when relevant;
- project verification passes;
- self-host path passes for Core/runtime changes;
- no provider-specific logic leaked into Core or Agent Gateway;
- no project-local bootstrap was added for dependencies already owned by DevTool;
- documentation reflects any new stable capability or configuration surface.

## Related documentation

- [Quick Start](quick-start.md)
- [Architecture Principles](architecture/principles.md)
- [CLI Reference](reference/cli.md)
- [Configuration Reference](reference/configuration.md)
- [Product Design](product-design.md)
