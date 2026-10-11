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

## Actual process audit, October 11, 2026

A second audit found that Desktop Commander Remote owns two necessary Node processes (remote connector and local MCP server). An unresponsive BlogContent pipeline had three extra processes — `bash`, `rg`, and `head` — all inactive for over 65 minutes. The `rg -l` invocation had **no explicit search path**, and in Desktop Commander's non-interactive Shell its standard input was a still-open socket. It waited for stdin instead of finishing a repository search; `head -40` waited on its pipe. The three exact PIDs were terminated without killing the shared Desktop Commander process group or restarting the service. Cgroup task count dropped from 51 to 48.

For non-interactive commands use an explicit search root, such as:

```bash
timeout 10s rg -l 'some heading' docs
```

If stdin should not be read, use `</dev/null` or pass explicit file paths. Do **not** apply that redirection to commands that genuinely require interactive input. For long-running child processes, record ownership, set bounded deadlines, and use SDK-managed cleanup. Killing process group 405 would also kill Desktop Commander: only terminate a *verified task-owned PID or independent process group*.

In the sampled 3-hour activity log, there were about 817 `start_process` requests and 366 `read_process_output` requests across 249 PIDs; 178 of those PIDs were read just once. Most calls were distinct, and DevTool appeared frequently among the invoked commands. It would therefore be misleading to claim the overhead is entirely duplicate polling, to merge unrelated commands into one daemon, or to present a persistent MCP service as a mandatory second remote connector.

## Recommended workflow

1. For supported engineering intents, invoke DV2's `project_verify`, `project_build`, `workspace_create`, `scm_publish`, and context capabilities. Reuse an existing local DevTool session when available, but do not add another remote MCP connector just for process reuse.
2. Avoid polling a remote PID when the result is already available synchronously; long-running tasks still require appropriate output inspection.
3. Use Desktop Commander for remote machine setup, direct filesystem/GUI requirements and troubleshooting that DevTool does not model.
4. Do not build another generic Agent Runner or duplicate the official MCP SDK solely to reduce a few shell calls. Add a direct DevTool MCP connection to the client only through explicit configuration, not a hidden fallback.

Further improvement requires instrumenting the **end-to-end** remote client/transport separately; this local A/B does not justify claims of major network speedups.

## Remote round-trip versus command execution, October 11, 2026

Use Desktop Commander's supported `verbose_timing` on a trivial `start_process`, and measure the outer tool call independently. In a five-call sample of `printf 'REMOTE_PROBE_OK'`, remote execution time was 8–9 ms, while request-to-response wall time ranged from 2,577 to 3,388 ms (median 2,731 ms). The outer measurement includes client orchestration, transport and result handling; **it must not be labeled pure network latency**.

A second, paired A/B used the same three read-only shell checks: print a marker, verify the DevTool TOML exists, and read the DevTool main commit. Both variants returned equivalent output. A made three sequential Desktop Commander `start_process` calls; B made one call executing those three checks in a bounded, short shell command.

| Round | Three separate remote calls | One combined remote call |
| --- | ---: | ---: |
| AB | 7,781 ms | 2,571 ms |
| BA | 7,518 ms | 2,560 ms |
| Median | 7,650 ms | 2,566 ms |

For this controlled trivial workload, one call was about **2.98× faster** (66.5% less wall time). This measures the full outer request path in the actual connected ChatGPT/Desktop Commander environment; it is not a local microbenchmark, and different network loads may change the result.

**Action:** batch only adjacent deterministic steps that require no intermediate Agent decision, with explicit search roots, bounded output and appropriate `set -e`/`pipefail` error handling. Keep meaningful build, test and SCM operations behind the existing DevTool commands. Do not assemble arbitrary long-running tasks into opaque Shell scripts, merge Desktop Commander's two active Node processes, or add a second remote MCP service. Retain detailed journald logs; use structured call statistics alongside them.
