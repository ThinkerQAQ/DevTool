# DevTool

[简体中文](README_ZH.md)

- [1. Introduction](#1-introduction)
- [2. Quick Start](#2-quick-start)
- [3. Configuration](#3-configuration)
- [4. Optional Features](#4-optional-features)
- [5. Development](#5-development)
- [6. License](#6-license)

## 1. Introduction

Working on a project often means switching between Git, build tools, code search, language servers, and development environments. Humans, automation, and AI agents have to discover how each tool works and decide which one to use. Connecting all these pieces separately creates repeated integration and configuration work.

DevTool was an experiment in putting a **small, project-aware control layer** above existing tools. Projects declare their capabilities and selected providers in `.devtool.toml`. A developer, CI job, or MCP-compatible agent can then use a consistent interface for inspecting a project, checking its environment, and invoking operations supplied by its Project Extension.

**This repository was previously archived. Active development is paused, and this experimental prototype is not production-ready.** It preserves working source, tests, and design decisions, but the original goal of a broadly reusable, easy-to-deploy engineering platform was not achieved. It has **no supported installer, maintained release, or compatibility guarantee**. The independent [DevEnvironment](https://github.com/ThinkerQAQ/DevEnvironment) project provides optional reusable Docker toolchain definitions; it is not needed for the local demonstration below.

## 2. Quick Start

Use the repository itself to inspect a project through DevTool. The initial check uses the built-in **local profile** so it does not depend on pulling a private Docker image. This is a source-level demonstration, **not a production installation**.

### 2.1. Prerequisites

Install Git and the Go version required by [go.mod](go.mod). These commands are shown for a Bash shell on Linux/macOS or Git Bash/WSL on Windows. The local profile can run on a host, but specific optional providers may need additional programs or credentials. DevTool does not install them automatically.

### 2.2. Clone and validate configuration

```bash
git clone https://github.com/ThinkerQAQ/DevTool.git
cd DevTool
export DEVTOOL_PROFILES=local
go run ./cmd/devtool version --json
go run ./cmd/devtool config validate
```

The version command returns source/build identity (a source checkout may report `dev`); validation should print `VALID .../.devtool.toml`. A successful syntax check does **not** mean all external providers are ready.

### 2.3. Inspect the configured project

```bash
go run ./cmd/devtool config path
go run ./cmd/devtool project inspect --json
```

The JSON describes this repository's Project Extension, service bindings, and declared commands. Confirm the project name is **DevTool** and that the Project Extension advertises `build`, `verify`, and `package`. These commands belong to **this** project; other projects may declare different operations.

### 2.4. Verify the source

```bash
go test ./...
go -C devcontrol test ./...
```

These tests check the implementation without installing a background server or changing your system-wide tools. They do not prove all third-party provider integrations are available. The initial walkthrough is complete after configuration, project inspection, and source tests succeed.

### 2.5. Troubleshooting

- **Unsupported Go version:** use the toolchain required by `go.mod`, then retry from the repository root.
- **No `.devtool.toml` found:** run commands inside the cloned DevTool repository; other projects must supply their own config.
- **`VALID` succeeds but provider checks fail:** `config validate` checks configuration, whereas the optional `config check` and `init` connect to configured extensions and external tools. See [Case 1](#41-case-1-check-external-providers).
- **Docker/GHCR errors:** the local profile is intended to avoid requiring the configured Docker image for this first test. Confirm `DEVTOOL_PROFILES=local` remains set in the same shell.
- **A command is not found:** project commands are discovered from the current Project Extension. Check `project inspect --json` before assuming a command is supported.

## 3. Configuration

[`.devtool.toml`](.devtool.toml) declares project identity, extension bindings, services, and environment profiles. The selected profile controls **which provider implementation is used**, not whether missing third-party tools are silently installed.

Choose profiles in the process environment before invoking the CLI:

```bash
export DEVTOOL_PROFILES=local
go run ./cmd/devtool config validate
```

Other configured profiles may use Docker, Dagger, Sourcegraph, or code-intelligence services and may require credentials or infrastructure that are **not included** in this repository. Do not copy account secrets into `.devtool.toml` or commit private provider configuration. The independent [DevEnvironment](https://github.com/ThinkerQAQ/DevEnvironment) repository contains optional shared toolchain definitions, while this repository only references the selected environment.

## 4. Optional Features

### 4.1. Case 1: Check external providers

For a deeper preflight, keep the local profile active and run:

```bash
go run ./cmd/devtool config check
go run ./cmd/devtool code doctor
go run ./cmd/devtool init --json
```

`config check` loads configured extensions and validates MCP tool registration; `code doctor` reports indexed and real-time code-provider availability; `init` checks runtime readiness and may prepare indexes. These operations can fail when an optional dependency is absent. Inspect the report and address only the providers you intend to use. Do not interpret a missing provider as proof that the source-level Quick Start failed.

### 4.2. Case 2: Build and verify DevTool itself

This repository's Project Extension supports a self-hosting workflow using the local profile:

```bash
export DEVTOOL_PROFILES=local
go run ./cmd/devtool build
go run ./cmd/devtool verify
go run ./cmd/devtool package
```

Inspect the candidate under `.devtool/out/` and its verification metadata. **Do not run `activate` or `rollback` as part of a first-run test**: those commands change the local installed binary. This is an experimental development path, not a maintained binary distribution.

### 4.3. Case 3: Connect a local MCP client

DevTool can serve an experimental MCP gateway over **stdio**:

```bash
export DEVTOOL_PROFILES=local
go run ./cmd/devtool agent mcp --context agent
```

Register an equivalent command in your local MCP client's stdio-server settings, with the **absolute path to the DevTool checkout as its working directory**. Some clients do not support an explicit working directory; for those, build a `devtool` binary with `go build -o devtool ./cmd/devtool` and configure the client to launch it from an appropriate project root. Exposed tools depend on the active provider configuration and permissions. Avoid starting a network-accessible HTTP gateway by default; remote HTTP mode requires explicit authentication and an independent security review.

### 4.4. Case 4: What the experiment established

The prototype explored stable intent-level MCP tools, declarative provider selection, indexed-search and live-LSP context routing, Markdown/diagram analysis, scoped Git-patch authorization, and build/verify/activate contracts. The hard questions were the **abstraction boundary** between the stable API and different providers, the **freshness and failure modes** of different context sources, safe agent side effects, and the cost of integrating tools across environments.

It did **not** demonstrate a consistently reproducible installation across machines, production hardening, a maintained release/compatibility policy, or that the extra control layer was worth its integration cost. These are **unresolved product questions**, not proven reasons for development stopping. This project is preserved as an engineering reference rather than advertised as a finished platform.

## 5. Development

Use the [Go toolchain specified by the source](go.mod):

```bash
go test ./...
go -C devcontrol test ./...
DEVTOOL_PROFILES=local go run ./cmd/devtool project inspect --json
DEVTOOL_PROFILES=local go run ./cmd/devtool build
```

CI performs unit checks and local self-hosting tests. Integration tests involving privately accessible images or other external providers are gated and should not be reported as passing in an unconfigured environment. Follow [AGENTS.md](AGENTS.md) for repository development conventions. Reopening this repository for documentation changes does not imply new feature development or maintenance support.

## 6. License

Original source and documentation are provided **as is** under the [MIT License](LICENSE). External tools and dependencies keep their own licenses. This experimental prototype is not production-certified and receives no guaranteed fixes or security updates. Run shell and patch capabilities only in an environment whose files and credentials you intend to expose; an MCP connection does not by itself provide an authorization or sandbox boundary.