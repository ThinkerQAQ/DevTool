# Pi → DevTool self-hosting proof of concept

**Status:** Experimental integration, not a released feature. **Target:** use upstream Pi to develop DevTool in its own isolated DV2-created Git worktree. This code is not a general security boundary and does not replace DevTool Core.

## Why this stays outside DevTool Core

Pi owns its model loop and hooks. DevTool still owns project discovery, code intelligence, build/verify/package, and SCM providers. The experiment consists of a **declarative Pi MCP client config** and a **small Pi-specific policy extension**. No DV2 internal service contract or provider is rewritten.

The root `.pi/mcp.json` in this experimental branch enables exactly these DevTool capabilities: `code_context`, `document_context`, `workspace_list`, `workspace_inspect`, `project_build`, `project_verify`, and `project_package`. All other DevTool tools are hidden from the model. `scm_checkpoint` and `scm_publish` are intentionally disabled in the agent PoC; commits of this branch can still be made through an external trusted DV2 operator.

`policy.ts` blocks direct shell, tool search, provider/SCM tools and edits outside the worktree. For each edited file, it requires a successfully completed `mcp__devtool__code_context` result that includes **both** indexed and realtime data without degraded fields. It also refuses to edit the experiment's own policy directory, `.pi`, `.git` and `.devtool.toml`. Context is invalidated at the start of every agent turn.

This is intentionally stricter than generic DevTool semantics: regular DV2 queries may legitimately return indexed-only discovery. The experiment's scoped context gate requires both backends for this particular source-file-editing scenario. New file creation, safe dependency updates, cross-file refactors and permission-isolated outer agents need additional design and tests.

## Ubuntu evidence (2026-10-11)

| Check | Result |
| --- | --- |
| Repo and workspace | DevTool `main` remained clean; DV2 `workspace_create` created `experiment/pi-dv2-selfhost-poc-20261011` |
| Pi install | `@earendil-works/pi-coding-agent@1.1.0` installed under NVM Node 24 |
| DV2 install | `0.1.0-dev`, embedded commit `89a45fa`; current repo `main` was `388790d` (mismatch to resolve) |
| DV2 provider smoke | Real `code_context` in experiment worktree returned indexed + realtime diagnostics, no `degraded` |
| Pi connection | Real Pi RPC session eventually registered 15 DV2 MCP tools |
| Pi exposure | Confirmed direct tools were exactly the configured allowlist; `scm_publish` exposure `hidden` |
| Policy behavior | 5 Node tests passed using synthetic Pi `tool_call`/`tool_result` events |
| Live coding Agent | **NOT TESTED:** `pi auth check --provider openai-codex` returned `not_ready`; no credentials were generated or copied |

**Important discovery:** a Pi RPC command issued immediately after process startup observed zero DV2 tools. Repeating the probe after approximately nine seconds found the tools. A real unattended runner must check MCP readiness from the live Pi tool registry before sending a coding prompt (rather than sleeping a fixed time).

## Run tests without a model

```bash
cd /path/to/DevTool-experiment-worktree
node --test examples/pi-selfhost/policy.test.mjs
DEVTOOL_PROFILES=local devtool config validate
DEVTOOL_PROFILES=local devtool code doctor
```

Project MCP config is only loaded once Pi trusts the project. To run an RPC session with a one-run trust approval:

```bash
pi --mode rpc --no-session --approve \
  --extension ./examples/pi-selfhost/policy.ts \
  --extension ./examples/pi-selfhost/probe.ts \
  --tools 'read,edit,write,mcp__devtool__*' \
  --exclude-tools 'bash,powershell,codemode,tool_search'
```

In the RPC stream send `{"type":"prompt","message":"/dv2probe","id":"probe"}` **after** the MCP connection is ready, then inspect the `DV2_TOOL_STATE=` notification for active names/exposure. This command is implemented by `probe.ts` and does **not** call a model. Never use `--no-extensions` in this test: that also disables Pi's built-in MCP connector.

After Pi has its own authorized model, replace the probe extension with a real, bounded task. Example: identify a small issue in `extensions/capability/code/code.go`, call DV2 context for that path, edit only that path, then call `project_build` and `project_verify`. Inspect Pi's JSONL events and DevTool trace before treating it as a successful agent test. **Do not merge or publish automatically.**

## Remaining limitations and acceptance criteria

1. The test harness simulates hook events; it does **not** prove that a real LLM issued and had an invalid tool call blocked. Live negative tests remain necessary once Pi is authorized.
2. Keep `pi` upstream and DV2 independent. The policy extension is only the agent-specific adapter. If the outer Codex/ChatGPT/Desktop Commander can run direct shell or write files, it can bypass Pi: true enforcement requires an OS/container or controlled runner boundary.
3. A successful `code_context` result is scoped to a file within a turn; source edits themselves may invalidate assumptions. Add a revision/content freshness check before production use.
4. Verify branch isolation, simultaneous Pi sessions, cancellation, provider outage, and symlink/path boundaries in a real sandbox before broad deployment.
5. Reconcile the installed DV2 CLI version with the selected source revision. Do not silently activate a new DevTool binary as part of this PoC.
6. No model-backed code change or full DevTool self-host build/verification was run; package/verify capabilities are **exposed**, not yet exercised by Pi.

See the separate [Pi integration architecture proposal](https://github.com/ThinkerQAQ/DevTool/blob/docs/pi-devtool-integration-20261011/docs/architecture/pi-agent-gateway.md) (documentation branch, not yet merged).
