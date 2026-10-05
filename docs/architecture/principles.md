# Architecture Principles

## One-sentence definition

> DevTool is a minimal, extensible, configuration-driven, self-hosting engineering control plane.

Every architectural decision should preserve that sentence.

## 1. Minimal Core

Core owns mechanisms that are stable across projects and providers:

- project/configuration discovery;
- extension loading protocol;
- registries;
- service routing;
- lifecycle ownership;
- policy/side-effect boundaries;
- agent/control-surface plumbing.

Core does not own provider or project behavior.

Examples that do **not** belong in Core:

- Docker container semantics;
- Dagger invocation rules;
- GitHub API behavior;
- CodeGraph arguments;
- Serena/LSP implementation details;
- Railway deployment rules;
- PhotoWaypoint, IDFlow, BlogCTL, GoTiny or DevTool-specific workflows.

### Design test

Ask:

> Would a second provider for the same capability need this Core change?

If no, the behavior probably belongs outside Core.

---

## 2. Configuration wires; code implements

`.devtool.toml` is declarative wiring.

Configuration may:

- discover extensions;
- select providers;
- compose profiles;
- pass provider settings;
- select workspaces;
- enable UI features.

Configuration should not become another workflow language.

Good:

```toml
[service.code-indexed]
provider = "intelligence.codegraph"
```

Good replacement:

```toml
[profile.sourcegraph.service.code-indexed]
provider = "intelligence.sourcegraph"
```

Avoid encoding project behavior as shell fragments or mini pipelines in TOML.

Behavior belongs in Project Extensions, Capability/Service Contracts and providers.

---

## 3. Stable capability, replaceable provider

A capability expresses stable engineering intent.

A provider implements that intent.

```text
Agent / Project Extension
        |
        v
Stable Capability / Service
        |
        +-------------------+
        |                   |
        v                   v
 Provider A             Provider B
```

Replacing a provider should normally require:

```text
provider implementation
+ configuration
```

It should not require editing a central switch in Core.

---

## 4. Project semantics belong in Project Extension

The Project Extension answers:

- what commands does this project expose?
- what resources/views/actions exist?
- how are project-specific operations composed?

Examples:

```text
build
verify
package
device.full
publish
```

The Project Extension may call stable services. It should not hard-code the concrete provider behind those services.

For DevTool itself:

```text
project.devtool
  -> environment service
  -> selected environment provider
```

This is how DevTool remains self-hosting without a Core special case.

---

## 5. Provider details stay behind contracts

Provider implementation details may be complex internally.

That complexity must not leak upward.

Example:

```text
code_context
    |
    +-- indexed service
    |     -> CodeGraph or Sourcegraph
    |
    +-- realtime service
          -> Serena/LSP
```

The agent asks for code context. It does not orchestrate provider APIs itself.

Likewise:

```text
scm_publish
    |
    v
scm service
    |
    v
scm.github
```

The stable intent remains the same if GitHub is replaced later.

---

## 6. Agent Capability is not an API proxy

Provider methods can be fine-grained.

Agent tools should stay coarse-grained and intent-oriented.

Preferred:

- `code_context`
- `scm_publish`
- `project_verify`

Avoid:

- `codegraph_find_references`
- `serena_get_symbols`
- `sourcegraph_search`
- `github_create_tree`

A new provider API does not automatically justify a new Agent Capability.

Add an Agent Capability only when a new stable engineering intent exists.

---

## 7. Portable and native are different domains

### Portable

Portable work should run behind environment/runtime contracts:

- build;
- test;
- package;
- service lifecycle;
- integration dependencies;
- artifacts;
- caches;
- CI-like execution.

### Native

Host-bound work belongs behind native capabilities:

- ADB / USB / physical devices;
- browser sessions and debugger APIs;
- native process integration;
- OS-specific integration.

Do not force native operations into portable runtimes merely for uniformity.

Do not leak Dagger, Docker, Kubernetes, or another concrete runtime into project contracts.

---

## 8. Environment and Runtime are replaceable layers

The project asks for engineering outcomes.

The configured environment/runtime implements them.

```text
Project Extension
      |
      v
Service Contract
      |
      v
Environment / Runtime Provider
      |
  +---+---------+---------+
  |             |         |
Docker        local    future remote/K8s
```

Kubernetes, Docker, local execution and future remote execution are implementation choices below the project contract.

DevTool should not reimplement cluster scheduling, networking, autoscaling or recovery responsibilities that belong to those systems.

---

## 9. Lifecycle follows ownership

Long-lived reusable objects must be created from the lifecycle context of their owner.

Correct:

```text
ProjectHost lifetime
  -> extension lifetime
    -> provider lifetime
      -> child MCP/LSP/indexer process
```

Incorrect:

```text
one tools/list RPC
  -> start reusable child process
  -> RPC returns
  -> request context canceled
  -> reusable child dies
```

Request context controls one request. Owner context controls reusable resources.

---

## 10. Self-hosting is an architecture test

DevTool must use DevTool to develop DevTool.

```text
DevTool N
  -> Project Extension
  -> stable services
  -> configured providers
  -> build DevTool N+1
  -> verify N+1
  -> package
```

No hidden privileged path should exist for DevTool itself.

A self-host failure is often evidence of a bad boundary, not merely a build inconvenience.

---

## 11. Configure first

When adding behavior, follow this order:

```text
configuration
  -> existing contract
  -> provider
  -> Project Extension
  -> new stable capability
  -> Core mechanism
```

This order prevents project/provider details from drifting into the minimal Core.

---

## 12. Root-cause changes over patch layers

When a real failure exposes a structural problem:

1. identify the ownership/contract boundary;
2. fix the boundary;
3. remove the obsolete path;
4. keep one canonical implementation.

Avoid preserving two control planes, two provider bootstraps, or two publishing paths merely to reduce the immediate diff.

Compatibility layers require an explicit product reason.

---

## 13. One canonical path per capability

A project should have one formal path for each engineering capability.

Examples:

```text
code intelligence -> DevTool capability/services
SCM publishing     -> DevTool SCM capability
build/verify       -> Project Extension
environment        -> configured Environment provider
```

Direct low-level tools may still exist for debugging or bootstrap, but they must not become a competing normal workflow.

---

## 14. Design review checklist

A change is architecturally healthy when most answers are yes:

| Question | Expected |
| --- | --- |
| Can provider selection change in configuration? | Yes |
| Can another provider implement the same contract? | Yes |
| Does Core avoid provider/project knowledge? | Yes |
| Does the Agent Gateway expose stable intent rather than provider methods? | Yes |
| Does the Project Extension own project semantics? | Yes |
| Is lifecycle owned by the correct long-lived component? | Yes |
| Does self-hosting still use the ordinary path? | Yes |
| Is there one canonical capability path? | Yes |
| Can obsolete/legacy paths be removed? | Yes |

## Related

- [Agent Guide](../agent-guide.md)
- [Runtime Lifecycle](../runtime-lifecycle.md)
- [Microkernel Audit](microkernel-audit-20261005.md)
- [Product Design](../product-design.md)
