# DevTool Runtime Lifecycle and Reuse

Status: Phase A + Phase B + extension-loader closure implemented and verified

## 1. Why this change exists

DevTool started as a thin wrapper that gave agents one stable entry point for project tooling. As CodeGraph, Serena/LSP, SCM and portable runtimes were added, the main performance cost moved away from the tools themselves and into repeated lifecycle setup.

The original hot path contained three avoidable cold starts:

```text
Agent
  -> DevTool Agent Gateway
  -> extension process
  -> development environment
  -> docker run --rm
  -> CodeGraph / Serena / SCM operation
```

Observed behavior:

- file edits themselves complete in milliseconds;
- CodeGraph incremental indexing is already incremental;
- a Go-backed process extension was started for individual calls;
- the project extension could be rebuilt and its provider process restarted across Describe/Execute;
- CodeGraph and Serena originally reached the development environment through `docker run --rm`, creating a new container for each fresh command.

The result is that cheap operations pay process/container startup costs repeatedly.

## 2. Design principles

This work keeps the original DevTool direction:

1. **Minimal Core**: Core owns configuration, registry/routing, contracts and only the lifecycle primitives required to host extensions.
2. **Extension first**: GitHub, CodeGraph, Serena, Docker and Dagger are capabilities, not Core concepts.
3. **Configuration driven dependencies**: project/environment images and extension/provider selection remain project configuration, not agent-side handwritten setup.
4. **Reuse before orchestration**: reuse an existing process/container before introducing a daemon, scheduler or cluster manager.
5. **No historical fallback path**: one configured path should be authoritative.
6. **Do not build a platform without a real bottleneck**: daemonization, plugin marketplaces, hot reload and distributed scheduling are explicitly out of scope for this phase.

## 3. Problems discovered

### 3.1 Development environment is ephemeral per command

The original development-environment adapter translated each tool launch into:

```text
docker run --rm -i ...
```

This is the wrong lifecycle for stateful development services such as:

- CodeGraph index + file watcher;
- Serena/LSP workspace state;
- language-server caches;
- compiler/package caches.

Desired lifecycle:

```text
project + configured image
  -> ensure one reusable workspace container
  -> docker exec ...
  -> docker exec ...
  -> docker exec ...
```

If the configured image changes, the project container is recreated.

### 3.2 Go process extensions are one-call processes

Originally, a configured process extension executed Describe/Invoke/ListTools/CallTool by creating a fresh process for calls.

Desired lifecycle:

```text
load extension
  -> start provider process once
  -> describe once
  -> RPC
  -> RPC
  -> RPC
  -> host/session exit
```

The protocol already supports multiple requests on one Session, so reuse belongs in the generic process-extension host rather than in SCM-specific code.

### 3.3 Project extension rebuilds on repeated host operations

The original Project Extension path invoked `go build` before Describe/Execute and started a new provider process for each invocation.

The implemented path resolves/caches the Project Extension executable through `adapters/extensionloader`, starts the provider once when the ProjectHost opens, and reuses the same bidirectional RPC Session for Describe and Execute. Language-specific build knowledge is outside Core.

## 4. Minimal implementation for this phase

### Phase A — implemented

- cache Project Extension build output through the external loader;
- retain normal Process Extensions for the lifetime of the DevTool host;
- reuse one development-environment container per project;
- recreate that container only when the configured image changes;
- continue reusing CodeGraph/Serena MCP clients through the existing MCP bridge;
- bind Project Extension, capability extension and MCP processes to the ProjectHost lifecycle and close them explicitly, with the Project Extension closed before its dependency providers.

### Phase B — implemented

- extract development-environment execution behind the generic `environment` Service;
- provide Docker as `environment.docker`;
- make CodeGraph and Serena depend on the Environment service instead of Docker code;
- validate declared Extension service requirements when opening a project;
- run Environment, Dagger, CodeGraph, Serena and SCM as independently configured process extensions;
- allow process extensions to call required Host services over the same bidirectional RPC session;
- remove the central built-in provider-ID catalog;
- move Go build/cache behavior into `adapters/extensionloader`;
- make Core consume only resolved extension executables.

### Phase C — not required now

A global `devtool daemon` is not part of this phase. Add it only if repeated CLI process startup remains a material cost after Phase A/B.

## 5. Target runtime shape

```text
                         DevTool Host
                    config / registry / routing
                              |
          +-------------------+-------------------+
          |                   |                   |
      CodeGraph            Serena/LSP             SCM
      Extension            Extension            Extension
          |                   |                   |
          +--------- reusable processes ----------+
                              |
                   reusable project environment
                              |
                     configured container image
```

The host manages reuse. Capability implementations remain replaceable.

## 6. DevTool versus Kubernetes

Kubernetes and DevTool solve different layers.

Kubernetes can replace parts of the **environment execution backend**:

```text
DevTool Environment capability
  -> Docker
  -> Podman
  -> Kubernetes
  -> remote machine
```

Kubernetes is good at:

- scheduling containers;
- restarting workloads;
- networking and service discovery;
- resource limits;
- multi-machine orchestration.

Kubernetes does not provide DevTool's project/agent semantics:

- discovering a project's configured capabilities;
- exposing CodeGraph/LSP/SCM as one agent-facing tool surface;
- routing an operation to the configured provider;
- defining project commands and portable runtime contracts;
- sharing the same capability model between local agents and remote agents;
- SCM publish semantics and approval boundaries;
- DevTool self-hosting through the same project-extension/runtime model.

Therefore Kubernetes is a possible provider below DevTool, not a replacement for DevTool.

A useful boundary is:

```text
Agent intent / project capability / provider routing     <- DevTool
Process/container/pod scheduling                         <- Docker/Kubernetes/etc.
```

If DevTool ever starts reimplementing pod scheduling, cluster networking or resource orchestration, the boundary has been crossed and that work should be delegated to Kubernetes or another runtime provider.

## 7. Product value

DevTool's value is not "another container runtime". Its value is a **project-scoped capability runtime for agents**.

The practical problem it solves is:

```text
different agents
+ different machines
+ different developer tools
+ different SCM/runtime providers
= one configured project capability surface
```

A project declares the environment and extensions once. Local agents and remote agents consume the same contracts instead of inventing ad-hoc shell commands and fallbacks.

The architecture is valuable only while Core remains small and execution backends remain replaceable.


## 8. Why this is no longer only a thin wrapper

The first useful version of DevTool really was a thin shell:

```text
Agent
  -> DevTool
  -> CodeGraph / LSP / Dagger / Git
```

That model remains the right mental starting point. The extra architecture appeared only after real constraints were observed:

1. local and remote agents must consume the same project capabilities;
2. CodeGraph and language servers have useful long-lived state;
3. project tooling must not recreate containers for every small operation;
4. SCM publishing must have one configured path rather than agent-defined fallbacks;
5. a provider must be replaceable without modifying Core;
6. DevTool itself must use the same contracts it exposes to other projects.

Once these constraints exist, a wrapper that directly shells out to hard-coded tools is no longer sufficient. The smallest useful design becomes a capability router with lifecycle reuse.

The important constraint is that DevTool must stop at that boundary. It should not grow into a general infrastructure orchestrator.

## 9. Minimality rule

The target is **minimal Core + replaceable extensions**, not maximum abstraction.

Core should contain only the mechanics that every provider needs:

- project/config discovery;
- capability/service registry;
- extension protocol and routing;
- lifecycle ownership and cleanup;
- policy/side-effect boundary required by the control plane.

Core should not contain product/provider knowledge such as:

- Docker container semantics;
- Kubernetes scheduling;
- GitHub API behavior;
- Dagger invocation rules;
- CodeGraph arguments;
- Serena/LSP implementation details;
- Go-specific extension build rules.

A useful test is:

> Adding a new provider for an existing capability should normally require only the provider implementation and configuration. It should not require editing a central switch in Core.

A second test is:

> Replacing Docker with Podman/Kubernetes/remote execution should not require changes to CodeGraph, Serena or Agent Gateway.

## 10. What we deliberately do not build

This phase does not add:

- a global always-on DevTool daemon;
- plugin marketplace/distribution infrastructure;
- hot reload;
- cluster scheduling;
- autoscaling;
- service discovery;
- overlay networking;
- pod restart policy;
- multi-node orchestration.

Those are separate problems. If one becomes a measured bottleneck, it can be added through a provider or a later lifecycle layer.

This keeps the implementation proportional to the real problem that triggered the work: repeated process/container cold starts and ad-hoc agent execution paths.

## 11. DevTool and Kubernetes are complementary

The boundary is intentionally explicit:

```text
Agent / project semantics
        |
        v
+----------------------------+
| DevTool                    |
| capability + provider      |
| routing + policy + wiring  |
+----------------------------+
        |
        v
Environment capability
        |
   +----+----------+----------------+
   |               |                |
 Docker          Podman        Kubernetes/remote
```

Kubernetes can be an excellent implementation of the environment/execution layer. It does not replace the project capability model above it.

If DevTool begins implementing Kubernetes responsibilities such as scheduling, networking or cluster recovery, that is a design error: those responsibilities should remain below the Environment/Runtime provider boundary.

## 12. Project value and interview-level technical story

The project value is not the number of wrapped CLIs. The value is making heterogeneous developer capabilities deterministic and reusable across agents.

The evolution is:

```text
thin CLI wrapper
    -> project contract
    -> extension/provider boundary
    -> configuration wiring
    -> Agent Gateway
    -> CodeGraph + LSP
    -> SCM publishing
    -> self-hosting
    -> process/MCP/container lifecycle reuse
    -> environment backend abstraction
```

Each step came from a concrete failure mode rather than speculative platform design.

A representative example is the slow remote edit/publish path:

```text
small source edit
  -> repeated remote calls
  -> repeated provider startup
  -> repeated Go build
  -> docker run --rm
  -> tool startup
```

Profiling showed that the source edit itself was negligible. The architecture was paying setup cost around the operation. The resulting change was therefore to fix lifecycle ownership and provider boundaries instead of optimizing the file edit.

That is the intended engineering pattern for future work: **measure a real bottleneck, add the smallest reusable abstraction that removes its root cause, keep provider details outside Core.**

## 13. Closure work completed

The two structural debts identified during the lifecycle review are now closed:

### 13.1 Central built-in extension catalog removed

`extensions/catalog.go` has been removed. Capability providers are now self-contained process extensions selected by `.devtool.toml`; adding a provider no longer requires adding its ID to a central switch.

Current path:

```text
.devtool.toml
   -> generic extension loader
   -> extension process/protocol
```

The loader understands build/transport mechanisms, not provider IDs such as `environment.docker`, `runtime.dagger` or `intelligence.codegraph`.

### 13.2 Go build knowledge removed from Core

Core no longer compiles Project/Process extensions. `adapters/extensionloader` resolves configured Go extensions into cached executables before Core starts them.

Implemented boundary:

```text
Core
  -> asks for an extension executable/artifact

configured loader/build adapter
  -> resolves/builds the executable
```

Core owns protocol/lifecycle. A configured loader owns language/build mechanics.

## 14. Acceptance criteria for the closure

The current refactor has met the closure criteria:

1. ✅ no central provider-ID switch is needed to add a provider;
2. ✅ Core contains no `go build` invocation;
3. ✅ project and normal extensions use the same configured executable-resolution boundary;
4. ✅ process extensions can call required Host services over the generic protocol;
5. ✅ CodeGraph/Serena remain independent of the concrete Environment backend;
6. ✅ N -> N+1 -> N+2 self-hosting passes;
7. ✅ the full CodeGraph/LSP configured-environment path passes;
8. ✅ no global daemon is required to obtain the lifecycle reuse achieved in this phase;
9. ✅ Agent Gateway can list CodeGraph/Serena/SCM tools and then call a CodeGraph tool in the same MCP session.

CI verification also covers Go tests, DevControl tests, real Dagger integration, project inspection, runtime doctor, self-host build, verify and package.


## 15. Lifecycle scope rule: cached handle != persistent process

The Agent Gateway end-to-end test exposed a subtle lifecycle bug after the first reuse implementation.

The MCP bridge cached its child client correctly, but the child process had been created with the context of the current `tools/list` RPC:

```text
Agent tools/list request
        |
        v
request-scoped context
        |
        v
start CodeGraph MCP with exec.CommandContext
        |
        v
tools/list returns
        |
        v
request context canceled
        |
        v
CodeGraph MCP killed
        |
        v
next tools/call -> broken pipe
```

This demonstrates an important rule:

> **A reusable object must be created from the lifecycle context of its owner, not from the context of the request that happened to initialize it.**

The corrected ownership model is:

```text
ProjectHost
  |
  +-- Process Extension lifetime
        |
        +-- MCP bridge lifetime
              |
              +-- CodeGraph / Serena child MCP process

individual RPC context
  |
  +-- only controls that request/wait path
```

Implementation consequences:

- `mcpbridge.Provider` owns a lifecycle context and cancellation function;
- child MCP processes are created from that lifecycle context;
- request cancellation no longer destroys a cached long-lived child;
- `Provider.Close()` cancels the lifecycle and closes the child;
- Process Extensions close gracefully by closing protocol stdin and waiting for the provider to exit, so provider-side deferred `Close()` hooks actually run;
- Project Extension is also retained for the full `ProjectHost` lifetime and is closed before dependency providers.

The CI acceptance path now performs, in one real `devtool agent mcp` session:

```text
initialize
  -> tools/list
  -> verify CodeGraph + Serena + SCM tools
  -> tools/call(codegraph_get_module_summary)
```

This specifically protects against the request-context/long-lived-process regression that the CLI-only tests could not detect.
