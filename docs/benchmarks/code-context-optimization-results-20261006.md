# code_context Time-to-Useful-Context Optimization Results — 2026-10-06

## Baseline

Branch:

`perf/code-context-time-to-useful-context-20261006`

Starting branch HEAD:

`62369a4e61422fbeace26cc986b54caa12040a34`

Reference benchmark:

- Group B Architecture Map: 3m53s
- Group C Architecture Map: 7m50s
- Group C Stage 1 raw reads / grep / shell fallback: 0 / 0 / 0

## Phase 0 — Observability

### Before

Existing tracing already covered:

- Agent `tools/call`
- `code_context`
- service calls
- CodeGraph provider duration
- Serena/LSP provider duration
- request/response byte counts

It did not expose:

- child MCP provider cold start vs warm reuse
- MCP provider lock wait
- child MCP request lock wait
- child MCP request duration

### Change

Added generic MCP bridge spans using the existing opt-in `sdk/trace` JSONL mechanism.

No new telemetry subsystem was added.

Tracing remains disabled unless `DEVTOOL_TRACE_FILE` is set.

No source, prompt, tool argument body, token, or credential is recorded.

### Measurement

Persistent Agent Gateway process, DevTool repository as workspace.

External timings:

| Call | Before detailed MCP spans | After detailed MCP spans |
| --- | ---: | ---: |
| objective-only cold | 0.642 s | 0.690 s |
| objective-only warm | 0.635 s | 0.651 s |
| enriched cold | 7.869 s | 7.322 s |
| enriched warm | 0.964 s | 1.066 s |
| enriched repeated warm | n/a | 0.865 s |

The variance is larger than the trace overhead; no material regression is visible.

Cold enriched breakdown after instrumentation:

| Layer | Duration |
| --- | ---: |
| CodeGraph search | 0.647 s |
| MCP provider cold start | 4.374 s |
| child MCP initialize request | 4.347 s |
| Serena symbols total | 4.385 s |
| Serena references | 2.152 s |
| Serena diagnostics | 0.132 s |
| code_context total | 7.321 s |

Warm enriched breakdown:

| Layer | Duration |
| --- | ---: |
| CodeGraph search | 0.740 s |
| MCP provider reuse | ~0 ms |
| Serena symbols | 0.062 s |
| Serena references | 0.135 s |
| Serena diagnostics | 0.125 s |
| code_context total | 1.066 s |

Repeated warm:

| Layer | Duration |
| --- | ---: |
| CodeGraph search | 0.610 s |
| Serena symbols | 0.094 s |
| Serena references | 0.032 s |
| Serena diagnostics | 0.125 s |
| code_context total | 0.864 s |

MCP lock wait was effectively zero in all observed single-client requests.

### Phase 0 conclusion

1. The Serena MCP bridge already reuses its child process inside one ProjectHost lifetime.
2. Cold realtime cost is dominated by child MCP initialization (~4.35 s).
3. One first-use references call adds another ~2.15 s.
4. Warm realtime work is relatively small.
5. Warm `code_context` is dominated by the one-shot CodeGraph search (~0.6–0.75 s).
6. Current capability composition is sequential, so warm total is approximately indexed + realtime.
7. MCP mutex contention is not material in this workload.

These measurements justify inspecting CodeGraph lifecycle/process reuse and then parallel composition. They do not justify transport/mutex optimization.

## Phase 1 — Provider lifecycle reuse

### Before

CodeGraph executed every indexed search as a new one-shot process:

`codegraph-server --graph-only --run-tool ...`

Measured warm CodeGraph search was typically 0.61–0.74 s.

Serena/LSP already reused its child MCP process correctly inside the ProjectHost
lifetime, so no Serena lifecycle rewrite was needed.

### Change

CodeGraph now uses one persistent internal MCP bridge for the provider lifetime.

The Agent surface is unchanged:

`code_context -> code-indexed service -> intelligence.codegraph`

The CodeGraph MCP tool surface remains provider-private.

To prevent stale state, the provider computes a lightweight workspace fingerprint
(path, size, and mtime metadata while excluding generated/cache directories). A changed
worktree causes the provider-owned CodeGraph bridge to restart and rebuild from current
workspace state. A clean unchanged worktree reuses the resident process.

### After

Objective-only repeated calls:

| Call | Before | After |
| --- | ---: | ---: |
| cold objective | ~0.64–0.69 s | 1.607 s |
| first warm objective | ~0.64–0.65 s | 0.004 s |
| repeated warm objective | ~0.64–0.65 s | 0.004 s |

The cold objective call regressed because persistent MCP startup/index ownership moved
into the first request. The target workload is warm; subsequent indexed searches fell
from ~0.65 s to ~3–4 ms.

Enriched calls:

| Call | Before Phase 1 | After Phase 1 |
| --- | ---: | ---: |
| cold enriched | 7.869 s | 7.100 s |
| warm enriched | 0.964 s | 0.264 s |
| repeated warm enriched | ~0.865–1.066 s | 0.364 s |

Provider detail on the measured warm enriched call:

- CodeGraph search: 3.396 ms
- Serena symbols: 97.748 ms
- Serena references: 34.975 ms
- Serena diagnostics: 124.013 ms
- code_context total: 263.495 ms

### Freshness validation

A unique source symbol was added after the provider was warm:

`PhaseOneFreshnessProbeUnique`

The next `code_context` call discovered it after the workspace fingerprint forced a
provider restart.

The file was then deleted. A subsequent call restarted the provider again and the
indexed result no longer contained the deleted symbol.

Therefore provider reuse does not silently retain stale source state across controlled
worktree changes.

### Phase 1 conclusion

Provider lifecycle reuse is a material warm-path optimization.

The next measured bottleneck is capability composition: indexed and realtime work still
runs sequentially even though user-supplied symbol/path hints make the two branches
independent.

## Phase 2 — Parallel independent indexed/realtime branches

### Before

With provider lifecycle reuse already applied, enriched `code_context` still executed:

`indexed search -> symbols -> references -> diagnostics`

sequentially.

Measured Phase 1:

- cold enriched: 7.100 s
- warm enriched: 0.264 s
- repeated warm enriched: 0.364 s

### Change

When the caller already supplies a symbol and/or path, the indexed search and realtime
branch are semantically independent. They now start concurrently and merge only after
both branches complete.

Realtime operations remain sequential internally; this phase changes only the
independent indexed-vs-realtime composition layer.

The output schema and provider selection are unchanged.

Cancellation is propagated through a derived context. Both branch result channels are
buffered, so completion cannot leak a goroutine while the caller is collecting results.

### After

| Call | Phase 1 | Phase 2 |
| --- | ---: | ---: |
| cold enriched | 7.100 s | 6.818 s |
| warm enriched | 0.264 s | 0.263 s |
| repeated warm enriched | 0.364 s | 0.360 s |

Cold-call savings are visible because CodeGraph startup/search (~0.636 s) now overlaps
Serena initialization and first-use realtime work.

Warm-call improvement is intentionally small: Phase 1 had already reduced warm CodeGraph
to ~4 ms, while realtime work dominates the remaining ~0.26–0.36 s.

This phase therefore improves the causal path without claiming a large warm-path gain
that the measurements do not support.
