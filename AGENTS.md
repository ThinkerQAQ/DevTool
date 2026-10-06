# DevTool Agent Contract

DevTool is the canonical control plane for project-aware development operations.

The goal of this file is operational: an agent should be able to enter the repository, discover the configured capabilities, make a change through the intended boundaries, and verify the result without inventing a second toolchain.

## First minute

From the repository root:

```bash
go run ./cmd/devtool init --json
go run ./cmd/devtool project inspect --json
```

Treat `devtool init --json` as the readiness feedback loop. If it returns structured issues such as a missing executable, missing configuration, or authorization requirement, remediate them with the available environment/account tools and rerun `init` until the required providers are ready. DevTool reports readiness; the Agent performs remediation rather than turning DevTool Core into a package manager.

For DevTool's own Stage 0 bootstrap, `go run ./cmd/devtool ...` is the minimal allowed entry point. It launches DevTool itself; it is not a parallel project control plane.

To expose the project to an MCP-capable agent:

```bash
go run ./cmd/devtool agent mcp --context codex
```

Use another context name when appropriate, for example `agent` or `claude-code`.

## Stable agent surface

Agents consume stable engineering intent, not provider-native APIs.

Current stable agent capabilities include:

- `code_context`: gather code context by composing configured indexed and realtime intelligence.
- `scm_publish`: publish source-control changes through the configured SCM provider.
- `project_<command>`: invoke commands declared by the Project Extension, such as `project_build` and `project_verify`.

Do not expose or depend on provider-native names such as CodeGraph, Serena, Sourcegraph, Dagger, GitHub, Docker, or Railway at the agent boundary unless the task is explicitly about configuring or implementing that provider.

## Code intelligence

Use the DevTool Agent Gateway rather than starting CodeGraph, Serena, gopls, Sourcegraph, or another language server/indexer directly.

The configured services currently separate two concerns:

- `code-indexed`: repository-scale indexed/structural intelligence.
- `code-realtime`: compiler/language-server-grade realtime intelligence.

The Code Capability composes those services into stable agent intent. Replacing CodeGraph with Sourcegraph must be a configuration change, not an Agent Gateway change.

For environment and provider health use:

```bash
go run ./cmd/devtool code doctor
go run ./cmd/devtool code verify
```

## Project commands

Project semantics come from the configured Project Extension. Discover them instead of hard-coding project-specific shell commands:

```bash
go run ./cmd/devtool project inspect --json
```

DevTool currently declares:

```bash
go run ./cmd/devtool build
go run ./cmd/devtool verify
go run ./cmd/devtool package
```

Those commands execute through configured services/providers. Do not replace them with ad-hoc container, Dagger, or build scripts when the Project Extension already defines the operation.

## Configuration before code

Prefer configuration and existing contracts before changing Core.

The expected order is:

1. Select or replace an existing provider in `.devtool.toml`.
2. Reuse an existing Service/Capability Contract.
3. Extend or add a provider behind that contract.
4. Extend the Project Extension for project semantics.
5. Change Core only when the missing behavior is truly cross-project mechanism.

Adding a provider for an existing capability should normally require provider implementation plus configuration, with no central switch in Core.

## Architecture invariants

DevTool follows four permanent principles:

> Minimal Core + extensible modules + configuration wiring + self-hosting.

Concretely:

- **Minimal Core** owns discovery, configuration, registries, routing, protocol and lifecycle mechanics.
- **Extensible** capabilities live behind explicit Extension and Service/Capability Contracts.
- **Configuration-driven** means TOML selects and wires implementations; configuration does not become a workflow scripting language.
- **Self-hosting** means DevTool must be able to build, verify and package itself through the same contracts used by other projects.

Keep project semantics, provider implementation details and infrastructure-specific behavior outside Core.

## Portable vs native boundary

Portable engineering work belongs behind Environment/Runtime providers where possible: build, test, package, services, artifacts, caches and CI-like execution.

Host-bound work belongs behind native capabilities: physical devices, ADB/USB, browser sessions, native processes and OS integration.

Do not force native work into containers, and do not leak a particular portable runtime into project contracts.

## Change workflow

Use small, reviewable steps:

1. inspect the configured capability and relevant contract;
2. make one coherent change;
3. commit and push that change;
4. continue with the next coherent change;
5. after the related batch is complete, run concentrated verification and regression checks.

Prefer root-cause fixes and clean contract changes over compatibility shims or patch layers. Add tests when they materially protect the changed contract or delivery path.

## SCM

Publishing belongs to the configured SCM capability. If `scm_publish` is available, use it rather than inventing a provider-specific fallback path.

Direct repository primitives such as reading files and inspecting status/diff/log may remain direct until DevTool exposes an equivalent stable capability.

## Dependency rule

Runtime executables are supplied by configured environment/runtime providers. Repositories do not install or pin their own CodeGraph, Serena, gopls, Sourcegraph, Dagger, or similar provider binaries just to make DevTool work.

## Before changing Core

Check these questions in order:

1. Can this be expressed by configuration?
2. Does an existing Service/Capability Contract already model the intent?
3. Is this provider-specific implementation?
4. Is this project-specific behavior that belongs in the Project Extension?
5. Would a new provider for the same capability need this Core change too?

If the answer to the last question is no, the change probably does not belong in Core.

## Documentation map

Start here, then use:

- [README](README.md) — product entry point and quick start.
- [Agent Guide](docs/agent-guide.md) — end-to-end agent workflow.
- [Architecture Principles](docs/architecture/principles.md) — boundaries and design tests.
- [CLI Reference](docs/reference/cli.md) — current stable CLI surface.
- [Configuration Reference](docs/reference/configuration.md) — `.devtool.toml`, profiles, services and extensions.
- [Product Design](docs/product-design.md) — broader product model and rationale.
- [Runtime Lifecycle](docs/runtime-lifecycle.md) — provider/process/container lifecycle decisions.
