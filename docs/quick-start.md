# Quick Start

This guide gets a contributor or agent from a fresh DevTool checkout to a working, verified DevTool project.

## 1. Enter the repository

DevTool bootstraps itself from source. Until a packaged binary is installed, use:

```bash
go run ./cmd/devtool ...
```

That command is Stage 0 bootstrap for DevTool itself. It still loads the same project configuration, Project Extension, services and providers used by a built DevTool binary.

## 2. Validate configuration

```bash
go run ./cmd/devtool config validate
go run ./cmd/devtool config path
```

Expected result:

```text
VALID <repository>/.devtool.toml
```

The repository configuration is the source of truth for extension loading and service/provider selection.

## 3. Inspect the project contract

```bash
go run ./cmd/devtool project inspect --json
```

This reveals the configured Project Extension, project commands, resources, service bindings and UI metadata.

Do this before inventing project-specific shell commands.

## 4. Verify code intelligence

```bash
go run ./cmd/devtool code doctor
go run ./cmd/devtool code verify
```

DevTool currently separates code intelligence into:

- `code-indexed` for indexed/structural repository intelligence.
- `code-realtime` for realtime language-server/compiler intelligence.

The default DevTool configuration selects CodeGraph for indexed intelligence and Serena/LSP for realtime intelligence.

Provider choice is configuration, not agent API.

## 5. Build DevTool through DevTool

```bash
go run ./cmd/devtool build
```

The Project Extension routes the build through the configured `environment` service. The resulting N+1 binary is written under:

```text
.devtool/out/
```

Inspect the next binary with:

```bash
./.devtool/out/devtool-next project inspect --json
```

On Windows use the `.exe` variant.

## 6. Run verification

After a coherent batch of changes:

```bash
go run ./cmd/devtool verify
```

For packaging:

```bash
go run ./cmd/devtool package
```

Project verification is intentionally a Project Extension command rather than an ad-hoc CI-only script.

## 7. Connect an agent

For a local MCP-capable agent:

```bash
go run ./cmd/devtool agent mcp --context codex
```

Other useful context names include `agent` and `claude-code`.

The Agent Gateway exposes stable intent-level tools from configured extensions. An agent should use tools such as:

- `code_context`
- `project_build`
- `project_verify`
- `scm_publish`

The gateway intentionally hides provider-native tools.

## 8. Run the remote Agent Gateway

For a remote runtime:

```bash
DEVTOOL_AGENT_TOKEN=<secret> \
go run ./cmd/devtool agent serve --listen :8080
```

The HTTP MCP endpoint requires bearer-token authentication by default.

`--allow-unauthenticated` is intended only for trusted local environments.

## 9. Switch profiles without changing the base configuration

DevTool profiles override configured extensions/services.

Use the Railway environment profile:

```bash
DEVTOOL_PROFILES=railway go run ./cmd/devtool project inspect --json
```

Use the Sourcegraph indexed-intelligence profile:

```bash
DEVTOOL_PROFILES=sourcegraph go run ./cmd/devtool code verify
```

Profiles compose:

```bash
DEVTOOL_PROFILES=railway,sourcegraph go run ./cmd/devtool project inspect --json
```

Later profiles override earlier profiles for the same extension or service key.

## 10. Before editing Core

Use this order:

1. configuration;
2. existing Service/Capability Contract;
3. provider implementation;
4. Project Extension;
5. Core mechanism.

If a change only exists because one provider or one project needs it, it normally belongs outside Core.

## Next

- [Agent Guide](agent-guide.md) — how an agent should execute real development work.
- [Architecture Principles](architecture/principles.md) — the boundaries every change must preserve.
- [CLI Reference](reference/cli.md) — exact command surface.
- [Configuration Reference](reference/configuration.md) — `.devtool.toml` and profiles.
