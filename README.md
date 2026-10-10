# DevTool

**A configuration-driven development control plane for humans, AI agents, and CI.**

DevTool exposes project operations and engineering capabilities through stable contracts. A project declares its extensions and provider bindings in `.devtool.toml`; callers use the same commands regardless of the selected implementation.

[Documentation](docs/index.md) · [中文文档](docs/cn/index.md) · [Agent instructions](AGENTS.md)

## What it does

- **Project operations:** discover, build, verify, and package projects through a Project Extension.
- **Code intelligence:** combine indexed repository context (CodeGraph or Sourcegraph) with live language-server context (Serena/LSP).
- **Document and diagram context:** inspect Markdown documents and Mermaid/PlantUML diagrams using configured providers.
- **SCM and environments:** use configured source-control, environment, and runtime providers.
- **Agent access:** expose intention-level tools over MCP rather than requiring agents to call every provider separately.

DevTool is a control plane, **not** a replacement for Git, language servers, Docker, Dagger, or build systems.

## Start from a checkout

Prerequisites: Git, Go (version specified in `go.mod`), and dependencies required by the selected providers. Use the `local` profile for a local-first environment.

```bash
git clone https://github.com/ThinkerQAQ/DevTool.git
cd DevTool
DEVTOOL_PROFILES=local go run ./cmd/devtool config validate
DEVTOOL_PROFILES=local go run ./cmd/devtool project inspect --json
DEVTOOL_PROFILES=local go run ./cmd/devtool init --json
```

If `devtool` is already installed, replace `go run ./cmd/devtool` with `devtool`. The source command is only for bootstrapping DevTool itself.

Read the [Quick Start](docs/quick-start.md) for readiness failures, verification, agent access, and self-hosting.

## How it fits together

```text
Human / Agent / CI
        |
   CLI / MCP gateway
        |
  Project Extension + stable capabilities
        |
      Services
        |
  configured providers
```

Core owns discovery, routing, extension loading, and lifecycle mechanics. Project behavior and integrations belong in extensions. New provider implementations are connected by configuration, not by Core switches.

The guiding principles are **minimal Core, composable extensions, declarative configuration, self-hosting, and reuse of established components before writing new ones**.

## Documentation

| Goal | English | 中文 |
| --- | --- | --- |
| Learn the basics | [Quick Start](docs/quick-start.md) | [快速开始](docs/cn/quick-start.md) |
| Understand the design | [Architecture](docs/architecture/overview.md) | [架构概览](docs/cn/architecture/overview.md) |
| Develop with an agent | [Agent Guide](docs/agent-guide.md) | [Agent 开发指南](docs/cn/agent-guide.md) |
| Replace a provider | [Provider Guide](docs/how-to/replace-or-add-provider.md) | [Provider 指南](docs/cn/how-to/replace-or-add-provider.md) |
| Look up commands and configuration | [CLI](docs/reference/cli.md) / [TOML](docs/reference/configuration.md) | [CLI](docs/cn/reference/cli.md) / [TOML](docs/cn/reference/configuration.md) |

For the current project's concrete provider selection, consult [`.devtool.toml`](.devtool.toml). Names and examples in documentation do not supersede that file.

## Contributing

Follow [AGENTS.md](AGENTS.md) and [Architecture Principles](docs/architecture/principles.md). Prefer an existing SDK or provider, make small commits, and verify changes through configured DevTool commands.
