# Agent development guide

[中文](cn/agent-guide.md) · [Agent instructions](../AGENTS.md)

Use DevTool as the project-aware gateway, not as another name for a collection of direct provider tools.

## Discover the project

```bash
DEVTOOL_PROFILES=local devtool config validate
DEVTOOL_PROFILES=local devtool project inspect --json
DEVTOOL_PROFILES=local devtool init --json
```

Inspect the resolved Project Extension and available commands. If `init` reports unavailable providers, fix the environment or select an appropriate configured profile.

## Connect the agent

```bash
devtool agent mcp --context codex
```

The MCP server gives callers engineering intents; configured extensions implement them. Relevant tools include:

| Tool | Purpose |
| --- | --- |
| `code_context` | Indexed repository context plus realtime code semantics |
| `document_context` | Markdown structure, bounded reviews, and related context |
| `diagram_context` | Mermaid/PlantUML diagram discovery and configured validation |
| `scm_publish` | Publish changes through the selected SCM provider |
| `project_<command>` | Build, verify, package, or other Project Extension commands |

Use the tool schemas returned by the MCP gateway. Do not assume all tools have the same arguments in every release.

## Work on a change

1. Determine the requested engineering intent and inspect the related contract.
2. Use `code_context` or `document_context` instead of launching provider-specific code/Markdown utilities independently.
3. Select existing components and configuration wiring before implementing infrastructure.
4. Make one coherent change; examine status/diff; commit and push.
5. Repeat as needed and run the project's configured verification command.
6. Report what was verified, what was skipped, and any external blockers.

## Keep providers behind the contract

CodeGraph and Sourcegraph implement indexed intelligence; Serena/LSP provides realtime semantics. Agents should continue to call `code_context` after a provider switch. For documents, Markdown parsing and Marksman provide different structure/realtime roles; diagram capabilities follow the same pattern.

Do not create a new agent tool for every provider method. Introduce a new capability only for a distinct, cross-provider intent.

## Failure handling

- **Missing executable:** install/configure the selected provider dependency, or switch a supported profile.
- **Credential issue:** use the designated credential provider. Never embed tokens in docs, TOML, or commits.
- **Stale indexed context:** re-run readiness/verification for the active workspace and branch; use realtime diagnostics for current edits.
- **Project command unavailable:** inspect project commands instead of bypassing DevTool.
- **Remote gateway:** use configured authentication. Do not expose an unauthenticated gateway on a public interface.

See [Quick Start](quick-start.md), [Architecture Principles](architecture/principles.md), and the [CLI Reference](reference/cli.md).
