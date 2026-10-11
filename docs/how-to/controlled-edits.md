# Controlled edits through DevTool

[简体中文](../cn/how-to/controlled-edits.md) · [Agent guide](../agent-guide.md)

DevTool can apply a **single-file, text-only Git patch** only after the caller requests a successful, scoped `code_context`. The reasoning agent stays outside DevTool: a browser ChatGPT session can decide what to change, while a persistent DevTool MCP process enforces local constraints. No Pi session or local model is involved.

## Prerequisites

- A Git development **branch or worktree**, not `main` or `master`.
- Git on PATH; the configured code-indexed (CodeGraph) and code-realtime (Serena/LSP) providers must both be available.
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

`tools/list` will include `code_context` and the opt-in `workspace_apply_patch`. The latter takes `context_token` and `patch`.

## 3. Read, decide, and apply

Call `code_context` on the **file** you intend to change:

```json
{"name":"code_context","arguments":{"objective":"Review Gateway code before a small refactor","path":"core/agent/gateway.go"}}
```

When both providers respond without degradation, the existing JSON inside the MCP text content is augmented with `context_token`, `context_path`, and `context_expires_at`. The caller must retain the token. A normal, degraded or unscoped read can still provide useful context but **cannot authorize a write**.

After ChatGPT has evaluated that response, generate a Git-format unified diff and call:

```json
{"name":"workspace_apply_patch","arguments":{"context_token":"<token from code_context>","patch":"diff --git a/core/agent/gateway.go b/core/agent/gateway.go\n--- a/core/agent/gateway.go\n+++ b/core/agent/gateway.go\n@@ -1 +1 @@\n-old line\n+new line\n"}}
```

The example patch is illustrative; provide a real diff with correct file content and hunk positions. A successful call returns `structuredContent.status = "applied"`, the changed path, and its SHA-256.

One token authorizes **one attempt on one existing file**. Re-run `code_context` for each subsequent edit. The gateway checks the source content digest, Git HEAD, actual worktree root, branch name, patch scope, and token lifetime; `git apply --check` must succeed before application.

## 4. Verify and publish

Run `project_verify` and `project_build` through the configured DevTool capabilities. The normal `scm_checkpoint` and `scm_publish` tools remain governed by existing SCM contracts; this first feature only constrains the **new** patch tool. For strict protection of every file write, place the outer caller in a filesystem sandbox or remove its unrestricted shell/file tools.

## Tested on DevTool itself

On 2026-10-11 the new binary returned real CodeGraph and Serena context for `core/agent/gateway.go`, issued a scoped token, and applied a one-line comment change through `workspace_apply_patch`. Repeating the same token returned an error. Protected branches, cross-file/multi-file changes, symlinks, stale hashes/HEAD, degraded context, expired tokens, mode changes and oversize patches were checked in local Git-repository tests.

## Known limits

- Tokens are held **in memory** by one Gateway instance. Restarting it invalidates them. They do not transfer between independent CLI invocations.
- Existing regular files only: no file creation, deletion, rename, permission changes or binary patches.
- Policy applies to this tool boundary; Desktop Commander or another agent with unrestricted local filesystem rights can bypass it. A true isolation boundary requires operating-system permissions or a sandbox.
- This is a conservative minimal adapter. The MCP JSON-RPC interface is already implemented in DevTool; migrating the whole gateway to the [official Go MCP SDK](https://github.com/modelcontextprotocol/go-sdk) is a separate interoperability task, not needed to execute Git patches. A CEL or OPA engine is unnecessary for the current four static constraints.
