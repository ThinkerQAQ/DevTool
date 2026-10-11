# Remote execution performance and structured logs

DevTool owns engineering intents: project builds, verification, Code/Document/Diagram/Log Context, SCM and Git Workspaces. Desktop Commander is the authorized **remote operating-system and GUI bridge**. Prefer an existing intent-level DV2 tool to repeated `start_process` and `read_process_output` calls. This recommendation does not mean DevTool has been installed as a direct ChatGPT connector; when used through Desktop Commander there is still an outer remote tool call.

## Inspect Desktop Commander with DevTool

```json
{"name":"log_context","arguments":{"objective":"Compare tool activity, latency and true failures","source":{"kind":"journald","scope":"user","unit":"desktop-commander.service","since":"2026-10-11T09:00:00+08:00","max_entries":14000},"limit":10}}
```

The existing `intelligence.log.local` Provider continues to consume **unchanged detailed journald records**. When it recognizes Desktop Commander Remote event prefixes, it additionally returns `log.tool_calls` with:

- `received`, `completed`, `failed` per tool; these are **transport event counts**, not shell command success/failure or retries.
- `duration_samples`, `p50_ms`, `p90_ms`, and `max_ms` for sampled request-to-response time.
- `correlation=approximate_fifo_by_tool_name`: completion events have a tool name but no call ID; overlapping calls of the same name cannot be paired exactly. A time window can also contain completions started before it.
- `timeout_ms` inside a user-supplied command **is not a timeout event**. Search text and outer log severity are not reliable tools-call outcome metrics.

The full remote log stream is preserved for troubleshooting. No change is made to Desktop Commander logging verbosity, journal retention, authorization or command logging. Keep the structured aggregates separate from any raw command arguments, which can contain private material.

## Reproduce local project command A/B

```bash
python3 scripts/benchmark-project-invocation.py \
  --project /path/to/DownKit-worktree \
  --devtool /path/to/compiled/devtool \
  --repeats 3
```

Both modes execute the **same configured** `verify` command through DevTool's Project Extension: A starts `devtool verify` afresh each time; B sends `project_verify` through one official Go MCP SDK stdio session. The script interleaves runs, fails if verification fails, and reports medians and total duration including MCP initialization.

### Baseline on local WSL, October 11, 2026

| Metric | Restart CLI (A) | Persistent MCP (B) |
| --- | ---: | ---: |
| Repetitions | 3 | 3 |
| DevTool process starts | 3 | 1 |
| Warm median verify duration | 2,145 ms | 2,055 ms |
| Three verifies, including B initialization | 6,627 ms | 6,445 ms |

B initialization was 114 ms. The median savings was about **4.2%**; total saving was about **2.7%**. These are local execution measurements, **not remote ChatGPT/connector latency**. The first cold standalone baseline took 44.61 seconds due to cache and setup effects and was excluded from the warm A/B comparison. Running other projects concurrently can change the results.

### Observed Desktop Commander traffic, same day

A sampled 3-hour journald window contained 1,379 tool-request lines: `start_process` 822 (59.6%), `read_process_output` 375 (27.2%), all others 182 (13.2%). This is evidence of heavy shell/poll traffic **across concurrent projects**, not a causal before/after benchmark.

## Recommended workflow

1. For supported engineering intents, invoke DV2's `project_verify`, `project_build`, `workspace_create`, `scm_publish`, and context capabilities. Keep one MCP session where the caller can do so.
2. Avoid polling a remote PID when the result is already available synchronously; long-running tasks still require appropriate output inspection.
3. Use Desktop Commander for remote machine setup, direct filesystem/GUI requirements and troubleshooting that DevTool does not model.
4. Do not build another generic Agent Runner or duplicate the official MCP SDK solely to reduce a few shell calls. Add a direct DevTool MCP connection to the client only through explicit configuration, not a hidden fallback.

Further improvement requires instrumenting the **end-to-end** remote client/transport separately; this local A/B does not justify claims of major network speedups.
