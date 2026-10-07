# DevTool

DevTool is a self-hosting, extensible, configuration-driven engineering control plane for local development, AI agents, CI/CD, native-device workflows, and project control surfaces.

> **Minimal Core + extensible modules + configuration wiring + self-hosting.**

DevTool keeps project semantics and provider implementation details behind stable contracts. Projects ask for engineering intent; configuration selects the implementation.

```text
Human / Agent / CI
        |
        v
     DevTool
        |
   Capability / Service
        |
   configured Provider
```

## Quick Start

From the DevTool repository:

```bash
go run ./cmd/devtool init --json
go run ./cmd/devtool project inspect --json
go run ./cmd/devtool build
```

`devtool init` resolves the active providers and reports project readiness before real work starts. It does not become a package manager: missing dependencies, credentials, or configuration are returned as structured issues for the operator or Agent to remediate, then `init` is rerun.

This is DevTool bootstrapping itself through the same Project Extension and service/provider graph used by a built binary.

Full walkthrough: [Quick Start](docs/quick-start.md).

## For AI Agents

Read [AGENTS.md](AGENTS.md) first.

Then start the Agent Gateway:

```bash
go run ./cmd/devtool agent mcp --context codex
```

Agents should consume stable intent-level capabilities such as:

- `code_context`
- `project_build`
- `project_verify`
- `scm_publish`

Provider-native APIs such as CodeGraph, Serena, Sourcegraph, GitHub or Dagger are implementation details unless the task is explicitly about that provider.

See [Agent Guide](docs/agent-guide.md).

## Architecture

The permanent architecture rules are:

1. **Minimal Core** — Core owns discovery, registries, routing, protocol and lifecycle mechanics.
2. **Extensible** — project, runtime, native, infrastructure, code-intelligence and policy behavior live behind extensions/contracts.
3. **Configuration-driven** — `.devtool.toml` selects and wires providers; configuration does not become a workflow scripting language.
4. **Self-hosting** — DevTool must build, verify and package itself through ordinary DevTool contracts.
5. **Replaceable providers** — an existing capability should gain a new implementation through provider code + configuration, not a central Core switch.
6. **Stable agent intent** — Agent Capability expresses engineering intent rather than mirroring provider APIs.

Read [Architecture Principles](docs/architecture/principles.md).

## Current Repository Wiring

The current configuration selects:

```text
environment
  -> environment.docker

portable-runtime
  -> runtime.dagger

code-indexed
  -> intelligence.codegraph

code-realtime
  -> intelligence.lsp.serena

scm
  -> scm.github
```

Profiles can replace providers without changing project semantics.

Examples:

```bash
DEVTOOL_PROFILES=railway go run ./cmd/devtool project inspect --json
DEVTOOL_PROFILES=sourcegraph go run ./cmd/devtool code verify
DEVTOOL_PROFILES=railway,sourcegraph go run ./cmd/devtool project inspect --json
```

See [Configuration Reference](docs/reference/configuration.md).

## Documentation

### Get Started

- [Quick Start](docs/quick-start.md)
- [Agent Guide](docs/agent-guide.md)

### Understand

- [Architecture Principles](docs/architecture/principles.md)
- [Product Design / PRD](docs/product-design.md)
- [Implementation Design](docs/implementation-design.md)
- [Runtime Lifecycle](docs/runtime-lifecycle.md)
- [Microkernel Audit](docs/architecture/microkernel-audit-20261005.md)

### How-to

- [Replace or Add a Provider](docs/how-to/replace-or-add-provider.md)
- [Migration Plan](docs/migration-plan.md)

### Reference

- [CLI Reference](docs/reference/cli.md)
- [Configuration Reference](docs/reference/configuration.md)

See the complete [Documentation Index](docs/index.md).

## Self-hosting

DevTool is a first-class DevTool project.

```text
DevTool N
  -> Project Extension
  -> configured services/providers
  -> build DevTool N+1
  -> inspect/verify N+1
  -> activate N+1
  -> package
```

The repository `VERSION` file provides the release version. Self-host builds additionally embed the exact source commit and record the verified binary SHA-256 before activation.

Normal local updates use:

```bash
devtool build
devtool verify
devtool activate
```

and `devtool rollback` restores the previously activated version.

Repository CI also validates N+1 to N+2 self-hosting.

If DevTool needs a hidden special path to develop itself, the architecture is incomplete.

## Runtime Image Layering

The shared development environment and DevTool runtime image remain separate:

```text
ghcr.io/thinkerqaq/dev-base
  -> language/toolchain dependencies
  -> CodeGraph / Serena / gopls / Dagger prerequisites

ghcr.io/thinkerqaq/devtool-runtime:<commit>
  -> FROM dev-base
  -> versioned DevTool binary
```

Remote deployments such as Railway use the DevTool runtime image and can select `environment.local` through the `railway` profile. The project contract does not change.
