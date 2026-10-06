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
