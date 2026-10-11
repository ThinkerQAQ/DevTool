# Controlled edits through DevTool

[简体中文](../cn/how-to/controlled-edits.md) · [Agent guide](../agent-guide.md)

DevTool can apply a **single-file, text-only Git patch** only after the caller obtains the appropriate scoped context: `code_context` for source files, `document_context` for Markdown, and additionally `diagram_context` for embedded Mermaid/PlantUML diagrams. The reasoning agent stays outside DevTool: a browser ChatGPT session can decide what to change, while a persistent DevTool MCP process enforces local constraints. No Pi session or local model is involved.

## Prerequisites

- A Git development **branch or worktree**, not `main` or `master`.
- Git on PATH. For source-code edits, the configured indexed CodeGraph and realtime Serena/LSP providers must both respond. Markdown uses the existing document provider; embedded diagrams use the existing diagram provider.
- A **new DevTool binary built from this checkout**. Do not assume an older system-wide `devtool` binary includes this tool.

```bash
cd /path/to/DevTool-worktree
go build -o /tmp/devtool-policy ./cmd/devtool
DEVTOOL_PROFILES=local /tmp/devtool-policy config validate
DEVTOOL_PROFILES=local /tmp/devtool-policy code doctor
```

## 1. Enable the policy in the project

```toml
[agent.policy]
enabled = true
protected_branches = ["main", "master"]
max_patch_bytes = 262144
context_ttl_seconds = 600
```

The feature defaults to **disabled** for projects without this setting. The policy runs inside the existing MCP Gateway; no second service or model API is required. Git parses, checks, and applies the patch; DevTool does not implement a unified-diff parser.

## 2. Start a persistent DevTool MCP connection

From the target worktree, launch:

```bash
DEVTOOL_PROFILES=local /tmp/devtool-policy agent mcp --context agent
```

Connect your MCP client using stdio and keep the process alive for the entire read/edit sequence. Alternatively use `agent serve --listen 127.0.0.1:8080` with `DEVTOOL_AGENT_TOKEN` set for a long-lived HTTP endpoint. Do not expose an unauthenticated port publicly.

`tools/list` will include `code_context` and the opt-in `workspace_apply_patch`. The latter takes `context_token` and `patch`. For log investigation, use `log_context` rather than treating log files as source-code edits.

## 3. Choose the correct context for each action

| Intent | Required DevTool context |
| --- | --- |
| Change a Go or other source file | `code_context` (indexed and realtime) |
| Change an ordinary `.md` / `.markdown` file | `document_context` |
| Change Markdown containing Mermaid/PlantUML diagrams | Both `document_context` and `diagram_context` on the same path |
| Introduce a new Mermaid/PlantUML block | Both document and diagram context, even if the old document had no diagrams |
| Change an existing standalone `.mmd` / `.mermaid` / `.puml` / `.plantuml` source | `diagram_context` only; successful render validation of the candidate is mandatory |
| Investigate file logs or journald | `log_context` (read-only; does not issue a patch token) |

The appropriate DevTool capability is selected **by the Gateway**, not by asking a local model to reclassify the file. Markdown diagram detection reuses the repository's Goldmark parser. The `diagram_context` provider also inspects standalone Mermaid/PlantUML **source** files. It reuses the same rendering Provider and Git preview; independently rendered SVG/PNG/JPEG files are still unsupported.

For example, a normal Markdown edit begins with:

```json
{"name":"document_context","arguments":{"objective":"Review the section before revising it","path":"docs/agent-guide.md","include_content":true}}
```

A Markdown file with diagrams also needs:

```json
{"name":"diagram_context","arguments":{"objective":"Inspect Mermaid diagram before modifying it","path":"docs/architecture/example.md","include_source":true}}
```

To inspect a standalone Mermaid source before editing it:

```json
{"name":"diagram_context","arguments":{"objective":"Review standalone diagram before editing","path":"docs/architecture/flow.mmd","analyze":true,"include_source":true}}
```

Standalone Mermaid analysis uses the existing Merman CLI for structural graph information; no standalone Merman LSP diagnostics are claimed. PlantUML can be inspected and rendered, but Merman's Mermaid semantics do not apply. A missing configured renderer blocks changed source files.

A log investigation uses the existing read-only capability, without any patch token:

```json
{"name":"log_context","arguments":{"objective":"Investigate recent errors","source":{"kind":"journald","unit":"devtool.service","scope":"user"},"limit":10}}
```

For source-code edits, call `code_context` on the **file** you intend to change:

```json
{"name":"code_context","arguments":{"objective":"Review Gateway code before a small refactor","path":"core/agent/gateway.go"}}
```

When the required providers respond successfully, the existing JSON inside the MCP text content is augmented with `context_token`, `context_path`, and `context_expires_at`. If a Markdown diagram needs a second context call, the first response instead contains `required_context_tools`, and only the second successful call returns the token. The caller must retain the token. A normal, degraded or unscoped read can still provide useful context but **cannot authorize a write**.

After ChatGPT has evaluated that response, generate a Git-format unified diff and call:

```json
{"name":"workspace_apply_patch","arguments":{"context_token":"<token from code_context>","patch":"diff --git a/core/agent/gateway.go b/core/agent/gateway.go\n--- a/core/agent/gateway.go\n+++ b/core/agent/gateway.go\n@@ -1 +1 @@\n-old line\n+new line\n"}}
```

The example patch is illustrative; provide a real diff with correct file content and hunk positions. A successful call returns `structuredContent.status = "applied"`, the changed path, and its SHA-256.

One token authorizes **one attempt on one existing file**. Re-run the appropriate source/document/diagram context for each subsequent edit. The gateway checks the source content digest, Git HEAD, actual worktree root, branch name, patch scope, and token lifetime. Before a Markdown or standalone diagram-source patch is committed, Git applies it inside a temporary preview tree. Changed or newly introduced Mermaid/PlantUML blocks are validated by the **already configured** `diagram-render` Provider. Only a successful renderer result allows the real `git apply`; missing renderers and syntax errors fail closed. Pure prose changes do not rerender unchanged diagrams.

## 4. Verify and publish

Run `project_verify` and `project_build` through the configured DevTool capabilities. The normal `scm_checkpoint` and `scm_publish` tools remain governed by existing SCM contracts; this first feature only constrains the **new** patch tool. For strict protection of every file write, place the outer caller in a filesystem sandbox or remove its unrestricted shell/file tools.

## Tested on DevTool itself

On 2026-10-11 the new binary returned real CodeGraph and Serena context for `core/agent/gateway.go`, issued a scoped token, and applied a one-line comment change through `workspace_apply_patch`. Repeating the same token returned an error. Protected branches, cross-file/multi-file changes, symlinks, stale hashes/HEAD, degraded context, expired tokens, mode changes and oversize patches were checked in local Git-repository tests.

## Known limits

- Tokens are held **in memory** by one Gateway instance. Restarting it invalidates them. They do not transfer between independent CLI invocations.
- Existing regular text files only: no file creation, deletion, rename, permission changes, log edits, rendered SVG/PNG/image edits, or binary patches. A missing Mermaid/PlantUML renderer blocks a changed diagram, but not ordinary Markdown prose edits.
- Policy applies to this tool boundary; Desktop Commander or another agent with unrestricted local filesystem rights can bypass it. A true isolation boundary requires operating-system permissions or a sandbox.
- This is a conservative minimal adapter. The MCP Server, stdio/Streamable HTTP and internal MCP Client bridges now use the [official Go MCP SDK](https://github.com/modelcontextprotocol/go-sdk); project-aware authorization remains in DevTool. A CEL or OPA engine is unnecessary for the current static constraints.
