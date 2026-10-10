# Quick Start

[中文](cn/quick-start.md) · [Documentation](index.md)

This tutorial starts with a clean checkout. It uses DevTool's own repository to demonstrate the same contracts used by other projects.

## 1. Prepare the environment

Install Git and the Go version declared by `go.mod`. Obtain the tools required by the selected provider profile. Running `init` tells you which dependencies or credentials are missing; DevTool does not install them automatically.

```bash
git clone https://github.com/ThinkerQAQ/DevTool.git
cd DevTool
```

If `devtool` is installed, use it. Otherwise bootstrap DevTool from this checkout by replacing `devtool` with `go run ./cmd/devtool` in the examples below.

## 2. Choose a profile and validate configuration

The repository selects Docker for its default environment and has a `local` profile for host execution. In this tutorial:

```bash
export DEVTOOL_PROFILES=local
devtool config path
devtool config validate
```

Expected: `VALID <path>/.devtool.toml`. If validation fails, inspect the reported table, extension loader, or service binding.

## 3. Inspect the project

```bash
devtool project inspect --json
```

Review the configured Project Extension, services, and declared commands. Do not assume that commands defined by one project's extension exist in every project.

## 4. Check providers

```bash
devtool code doctor
devtool init --json
```

`code doctor` reports the selected indexed and realtime code providers. `init` checks active readiness and may initialize a repository index; the first run can take longer. Address any structured missing-dependency or authorization issues before continuing.

## 5. Work through DevTool

The DevTool Project Extension declares the build/verify/package flow:

```bash
devtool build
devtool verify
devtool package
```

Build output is staged under `.devtool/out/`. Verification checks the candidate before it is activated. Portable runtime dependencies may require Docker/Dagger or another selected runtime.

For an installed binary, see [Build, Verify, and Activate](how-to/self-host.md) before using `devtool activate` or `devtool rollback`.

## 6. Connect an AI agent

Start the local MCP gateway, then point your MCP client to this command:

```bash
devtool agent mcp --context codex
```

Other contexts include `agent` and `claude-code`. The gateway exposes stable tools such as `code_context`, `document_context`, `diagram_context`, `scm_publish`, and configured project commands.

## Next

- [Architecture Overview](architecture/overview.md) explains why callers use stable contracts.
- [Replace a Provider](how-to/replace-or-add-provider.md) walks through a TOML-only provider switch.
- [CLI Reference](reference/cli.md) lists commands and their scopes.
