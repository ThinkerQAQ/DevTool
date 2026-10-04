# DevTool Runtime Lifecycle and Reuse

Status: Phase A implemented; Phase B environment boundary implemented

## 1. Why this change exists

DevTool started as a thin wrapper that gave agents one stable entry point for project tooling. As CodeGraph, Serena/LSP, SCM and portable runtimes were added, the main performance cost moved away from the tools themselves and into repeated lifecycle setup.

The current hot path contains three avoidable cold starts:

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
- a Go-backed process extension is started for individual calls;
- the project extension can be rebuilt again for Describe/Execute;
- CodeGraph and Serena currently reach the development environment through `docker run --rm`, creating a new container for a fresh command.

The result is that cheap operations pay process/container startup costs repeatedly.

## 2. Design principles

This work keeps the original DevTool direction:

1. **Minimal Core**: Core owns configuration, registry/routing, contracts and only the lifecycle primitives required to host extensions.
2. **Extension first**: GitHub, CodeGraph, Serena, Docker and Dagger are capabilities, not Core concepts.
3. **Configuration driven dependencies**: project/environment images and extension/provider selection remain project configuration, not agent-side handwritten setup.
4. **Reuse before orchestration**: reuse an existing process/container before introducing a daemon, scheduler or cluster manager.
5. **No historical fallback path**: one configured path should be authoritative.
6. **Do not build a platform without a real bottleneck**: daemonization, plugin marketplaces, hot reload and distributed scheduling are explicitly out of scope for this phase.

## 3. Current problems

### 3.1 Development environment is ephemeral per command

The current development-environment adapter translates each tool launch into:

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

A configured process extension is currently executed for Describe/Invoke/ListTools/CallTool by creating a fresh process for calls.

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

The Project Extension build path always invokes `go build` before Describe/Execute.

The first implementation step is to reuse the existing artifact when the Go source/module files are not newer than the cached provider binary.

Longer term, language-specific build knowledge should leave Core entirely and become an extension/transport concern. That larger extraction is deliberately separated from the current performance fix.

## 4. Minimal implementation for this phase

### Phase A — implemented

- cache Project Extension build output;
- retain a Go Process Extension process for the lifetime of the DevTool host;
- reuse one development-environment container per project;
- recreate that container only when the configured image changes;
- continue reusing CodeGraph/Serena MCP clients through the existing MCP bridge;
- bind reusable extension/MCP processes to the Project Host lifecycle and close them explicitly.

### Phase B — partially implemented

Implemented:

- extract development-environment execution behind the generic `environment` Service;
- provide Docker as `environment.docker`;
- make CodeGraph and Serena depend on the Environment service instead of Docker code;
- validate declared Extension service requirements when opening a project.

Still intentionally deferred:

- remove remaining language-specific Project Extension build logic from Core;
- remove the central built-in extension catalog in favor of generic configured loading.

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
