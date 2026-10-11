# DevTool agent instructions

These instructions apply to agents contributing to this repository.

## Entry point

From the repository root, use the installed `devtool` CLI when available:

```bash
DEVTOOL_PROFILES=local devtool config validate
DEVTOOL_PROFILES=local devtool config check
DEVTOOL_PROFILES=local devtool project inspect --json
DEVTOOL_PROFILES=local devtool init --json
DEVTOOL_PROFILES=local devtool code doctor
```

If DevTool is not yet installed, `go run ./cmd/devtool <command>` is the bootstrap path **for this repository only**. Do not bypass an existing DevTool project command with custom shell orchestration.

## Agent interface

Start the configured MCP gateway with `devtool agent mcp --context codex` (or `agent` / `claude-code`). Prefer stable intent-level tools:

- `code_context`: indexed and realtime code context;
- `document_context`: inspect Markdown before editing it;
- `diagram_context`: inspect standalone Mermaid/PlantUML sources, or Markdown-embedded diagrams (in addition to `document_context` for Markdown);
- `log_context`: query project logs or journald for investigations (read-only);
- `project_<command>`: commands provided by the Project Extension;
- `scm_publish`: source publishing through the configured SCM provider;
- `workspace_apply_patch`: when enabled, apply a single-file Git patch using a scoped, one-time context token.

Before editing Go or other code, call `code_context`; before editing Markdown, call `document_context`; for Markdown-embedded Mermaid/PlantUML blocks, call **both** `document_context` and `diagram_context`; for standalone `.mmd`, `.mermaid`, `.puml`, and `.plantuml` source files, call `diagram_context` alone. Changed diagrams must pass the configured renderer before patch application. Investigate log files or journald with `log_context`, not raw Shell log readers. If using `workspace_apply_patch`, the Gateway rejects mutations without the required context evidence.

Do not directly integrate CodeGraph, Serena, Sourcegraph, Dagger, or GitHub into agent workflows when DevTool already provides the intent.

Remote Shell process hygiene: in non-interactive Desktop Commander calls, pass explicit search roots (e.g., `rg -l 'pattern' .` instead of `rg -l 'pattern'`), because the tool's stdin can remain open on a socket. Bound potentially long scans with `timeout`; do not blindly append `head` to an unbounded stdin-reading command. Prefer one existing project-level DevTool command to an improvised chain of redundant Shell calls. Terminate only verified, task-owned child processes; never kill Desktop Commander's shared process group or globally `pkill chrome`.

## Where changes belong

1. Reuse a mature existing component, SDK, library, service, or protocol.
2. If a supported provider already exists, select it through `.devtool.toml` / profiles.
3. If behavior fits an existing contract, extend or replace a provider.
4. Put project-specific commands in the Project Extension.
5. Change Core only for genuinely shared discovery, routing, protocol, or lifecycle mechanisms.

Keep configuration declarative. Do not invent dynamic workflow scripts in TOML, provider-ID switches in Core, or special self-host shortcuts.

## Working loop

1. Read [Quick Start](docs/quick-start.md), [Architecture Overview](docs/architecture/overview.md), and the relevant reference page.
2. Inspect current configuration and related contract using DevTool.
3. Make one focused change; review the diff; commit and push.
4. Repeat for the next coherent change.
5. Run configured DevTool validation and integration checks. Report verified results separately from skipped checks.

Use `devtool build`, `devtool verify`, and `devtool package` when declared by this project's extension. Do not claim that a command succeeded unless it ran successfully.

## Documentation

English is the default; matching Chinese pages live in `docs/cn/`. Update both languages when behavior changes. The guide is a description of actual functionality, not an implementation proposal.

[English documentation](docs/index.md) · [中文文档](docs/cn/index.md) · [Architecture Principles](docs/architecture/principles.md)
