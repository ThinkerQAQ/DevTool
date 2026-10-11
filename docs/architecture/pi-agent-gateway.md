# Pi as a controlled agent entry point for DevTool V2

[中文](../cn/architecture/pi-agent-gateway.md) · [Architecture](overview.md) · [Agent guide](../agent-guide.md)

> **Status: proposal (2026-10-11).** This document defines a potential integration, not a shipped DevTool/Pi feature. This change contains documentation only.

## Decision

Use upstream **Pi** as the optional, shared coding-agent execution layer; keep **DevTool V2 (DV2)** as the project-aware engineering control plane. Start with an external Pi extension and native MCP configuration. Do **not** fork Pi, replace DevTool Core, or create another orchestration service before a measured proof of concept.

This addresses two existing paths:

```text
Local developer / Codex ────> Pi CLI or headless ──┐
                                                   ├──> Pi policy adapter
Web ChatGPT ──> Desktop Commander ──> Pi headless ──┘           |
                                                           DevTool MCP
                                                        /      |       \
                                                  CodeGraph  Serena/LSP  SCM/Project
```

- **Local:** use Pi interactively, or have another local agent delegate a narrowly specified execution task to Pi.
- **Remote:** ChatGPT discusses and scopes work; Desktop Commander invokes Pi on the authorized Ubuntu machine, with explicit repository, worktree, task and execution limits.
- **Both:** Pi performs coding-agent tool decisions; DV2 provides stable project capabilities through configured services/providers. Do not embed Pi-specific behavior in DevTool Core.

An outer agent that retains unrestricted shell, file and GitHub access can bypass Pi. Routing through Pi is an architectural convention until execution permissions enforce it.

## Evidence collected on the local Ubuntu machine

| Evidence | Observed result |
| --- | --- |
| DevTool checkout | `/home/zsk/code/DevTool`, clean `main` at `388790d` when inspected |
| Installed DevTool CLI | `0.1.0-dev`, embedded commit `89a45fa`; differs from checkout HEAD and must be reconciled before PoC |
| DevTool readiness | `devtool code doctor`: CodeGraph `0.21.0` READY; Serena `1.7.0` READY |
| DevTool MCP | Real `tools/list` exposed `code_context`, `workspace_create`, `scm_checkpoint`, `scm_publish`, project actions |
| DevTool code inspection | Real MCP `code_context` call returned indexed results and LSP diagnostics; symbol reference lookup degraded when the supplied symbol did not match |
| Pi source | `/home/zsk/code/pi`, upstream `earendil-works/pi`, shallow clone at `ea448f4` (2026-10-10), clean before temporary research setup |
| Pi DevTool inspection | A temporary **local-only** `.devtool.toml` selected DevTool project/code providers. Config validation, project inspection, provider doctor and a real `code_context` call succeeded against Pi source; not an upstream Pi integration |
| Pi runtime | Source examined; **Pi CLI not installed, model authorization not tested, no agent task or policy interception executed** |

Source locations reviewed:

- DV2: `core/project/discovery.go`, `extensions/capability/code/code.go`, `extensions/context/code/code.go`, `extensions/capability/workspace/workspace.go`, `extensions/capability/scm/scm.go`, `adapters/extensionloader/go.go`.
- Pi: `packages/coding-agent/docs/{mcp,extensions,cli,cli-integration,rpc,security}.md`, `packages/agent/src/agent-loop.ts`.
- Upstream reference: [Pi source](https://github.com/earendil-works/pi/tree/ea448f454af4) and [DevTool repository](https://github.com/ThinkerQAQ/DevTool).

## What exists, and what does not

**Existing DV2:** `devtool agent mcp --context agent` exposes intent-level tools. The current `code_context` accepts `objective`, optional `symbol` and project-relative `path`; it constructs the workspace from its **active project session**, not an arbitrary user-supplied root. When neither symbol nor path is specified, the composite implementation returns indexed discovery without invoking realtime methods. With targeted scope it may combine CodeGraph and Serena. Therefore “every call must hit both providers” is not a valid invariant.

**Existing Pi:** the checked-out release has native `.pi/mcp.json` / user-level MCP servers, `exposure` and `toolExposure`, TypeScript extension hooks (`tool_call` can block), `--tools` / `--exclude-tools`, print/JSON/RPC modes and SDK. Pi does **not** provide an OS-level permission boundary: the extension runs with the process user's privileges. MCP tools with indirect exposure can remain callable through `codemode`; omitting a tool from the model's visible declaration alone is insufficient.

**Not implemented:** a DV2-specific Pi policy extension, mandatory context-before-edit rule, cross-agent permission isolation, structured attestation, persistent Pi runner or per-worktree task coordinator.

## Target contracts

1. **Workspace identity:** every request carries project root, Git common directory, worktree path, branch/HEAD, task ID and an allowlisted edit scope. A session may write only in its assigned worktree. Reject symlink/path escapes and stale workspace identity; invalidate context after relevant changes.
2. **Context gate:** before source editing, require a successful DV2 `code_context` for the task/scope; capture provider successes, degradations and skipped methods. For documentation-only work, use DV2 `document_context` instead. Fail closed for mandatory capabilities; optional capabilities may degrade only when policy explicitly allows it.
3. **Tool gate:** hide/deny direct provider tools and forbidden shell/SCM operations. For the strict PoC, disable Pi `bash` and indirect generic `codemode` unless separately audited; allow only the necessary file-edit tools and selected `mcp__devtool__*` tools. A Pi `tool_call` hook blocks prohibited calls, including attempts to change policy/agent configuration.
4. **Publishing:** allow DV2 `scm_checkpoint` for small changes and `scm_publish` for PR publication. Merge remains opt-in, with branch protection and CI as independent external safeguards.
5. **Evidence:** record task/worktree identity, Pi session, tool call ID, DV2 capability, resolved provider(s), invocation result, fallback decision, commit and PR URL when applicable. Never label the task “DV2 verified” solely because the agent says so; the run must contain actual successful events.
6. **Privilege boundary:** local Codex or remote ChatGPT can still bypass Pi if their bridge has unrestricted file/shell access. A hard guarantee requires restricting those outer pathways to a runner command or a sandboxed account/container, not just adding an `AGENTS.md` instruction.

Prefer a reusable policy config and a single thin Pi extension; project/provider selection stays in `.devtool.toml`. Do not put procedural scripts inside TOML.

## Illustrative Pi configuration (not committed or tested)

Place project-specific `.pi/mcp.json` only in a trusted DV2-enabled project; alternatively use the user-level Pi MCP file. Pi enforces project trust before reading project MCP configuration.

```json
{
  "mcpServers": {
    "devtool": {
      "command": "devtool",
      "args": ["agent", "mcp", "--context", "agent"],
      "env": { "DEVTOOL_PROFILES": "local" },
      "exposure": "hidden",
      "toolExposure": {
        "code_context": "direct",
        "document_context": "direct",
        "workspace_create": "direct",
        "scm_checkpoint": "direct",
        "scm_publish": "direct",
        "project_verify": "direct"
      }
    }
  }
}
```

Tool availability is project-dependent; `project_verify` is a Project Extension command, not a universal contract. This snippet assumes the target project declares the `local` profile and the selected tools. A different project must supply its own valid profile and tool selection. **Do not copy it into the upstream Pi repository as a permanent integration.**

Illustrative strict-mode launch, **after** the extension and policy have been implemented:

```bash
cd /path/to/assigned/worktree
pi --mode json --no-session \
  --tools 'read,edit,write,mcp__devtool__*' \
  --exclude-tools 'codemode,tool_search,bash,powershell' \
  --extension /path/to/dv2-policy.ts \
  "Implement the approved, scoped task through DevTool"
```

The extension must verify the tool set actually in effect, reject policy/config edits and observe both direct and nested tool invocations. Flags by themselves do not enforce context-before-edit or protect against launching a different Pi command. Use JSON events for one-off automation; choose RPC and wait for `agent_settled` only when a real multi-turn runner is needed. A successful RPC prompt acknowledgment is not task completion.

## Minimal implementation sequence

| Phase | Scope | Exit criterion |
| --- | --- | --- |
| 0 — Baseline | Align installed DevTool binary with the desired commit; pin Pi release/source; verify project trust, model login and MCP readiness in an isolated worktree | Versions, active providers and actual MCP tool schemas recorded |
| 1 — Extension | Build **one** Pi policy extension; fail-closed edit gate, tool allowlist, worktree validation, safe structured audit | A positive context→edit→DV2 verify path passes; direct shell/provider/SCM bypasses fail |
| 2 — Dual entry | Local Pi/optional Codex delegation; ChatGPT→Desktop Commander→Pi one-shot in the same isolated repo | Same policy and trace evidence from both entry paths, no unmanaged writes |
| 3 — Concurrency | One writer per worktree, explicit task ID, bounded parallel tasks, cancellation and cleanup | Two independent worktrees do not cross-write; duplicate/cancel/retry behavior is deterministic |
| 4 — Optional expansion | Reuse the adapter in PhotoWaypoint and IDFlow; consider RPC service only if sessions demonstrably need reuse | Measured overhead and failure rate justify broader adoption |

Do not introduce a custom daemon, queue, agent scheduler, or Pi fork during phases 0–2. GoTiny already owns separate agent-runner concerns; any durable task mechanism must be justified against that existing project before implementation.

## Required negative tests

- Edit a Go/TypeScript file without a successful `code_context` → block; correct scoped context then edit → allow.
- Ask for `git push`, raw `go test`, direct CodeGraph/Serena, a shell escape or policy-file rewrite → block or route through an explicitly configured DV2 capability.
- Disconnect DV2 MCP or make indexed/realtime providers fail → required step fails visibly, not silently via generic shell tools.
- Switch branch/worktree or alter source after context → invalidate context where applicable; never reuse another worktree's attestation.
- Change `--tools`, load `codemode`, disable the extension or invoke Desktop Commander outside Pi → document/test the **outer access-control** boundary, which Pi cannot enforce.
- Run two tasks against distinct worktrees and one duplicate request → no cross-write or duplicate publication.
- Kill/cancel Pi midway → no falsely successful audit or partial publish.

## Decision gate

Adopt upstream Pi plus an extension **only if** both entry points pass the same negative tests and the additional LLM round trip/cost is acceptable. Prefer direct DV2 calls for deterministic work where another agent loop adds no value. Fork Pi only after identifying a specific missing hook/API that cannot be implemented as an extension and measuring its impact. Until then, this is a proposal, not an instruction to migrate every project.
