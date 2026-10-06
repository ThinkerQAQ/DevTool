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


## Phase 3 — Objective-only repository discovery

### Before

Objective-only `code_context` forwarded the full natural-language objective directly to
`codegraph_symbol_search`. On the six fixed IDFlow architecture questions this produced
useful top-8 candidates for 4/6 questions.

The weak cases were:

- control-plane discovery: the top results were dominated by unrelated symbols containing
  "control";
- minimal migration surface: the top results did not include the development-control-plane
  files that actually define the migration boundary.

Measured on a fresh fixed-SHA IDFlow harness after Phase 2:

| Question | Latency | Useful top-8 candidate |
| --- | ---: | --- |
| control-plane entry | 7.495 s cold | no |
| legacy/bootstrap | 0.004 s warm | yes |
| build/verify/package | 0.005 s warm | yes |
| Project Extension boundary | 0.003 s warm | yes |
| provider responsibility | 0.003 s warm | yes |
| minimal migration surface | 0.004 s warm | no |

Useful candidate hit rate: **4/6 (67%)**.

### Change

Objective-only requests now mark indexed search as repository discovery while preserving
the same `code_context` Agent tool.

The CodeGraph provider adapts that internal service intent using existing CodeGraph
primitives:

- focused symbol search;
- repository pattern search;
- entry-point discovery when the objective indicates entry/start/control/bootstrap;
- deterministic candidate scoring, de-duplication, and test-file penalty.

No provider-native tool is exposed to the Agent.

Query normalization is generic token/stop-word normalization; there is no IDFlow-specific
path, symbol, or benchmark-answer table.

The first focused symbol query intentionally precedes the broader pattern query. Measurement
showed pattern search is a more expensive CodeGraph cold path; symbol search warms the
persistent provider first and avoids making repository-pattern indexing the first operation.

### After

On the same six fixed IDFlow questions, the top-8 candidates now include relevant
development-boundary files for all six questions.

Representative hits:

- control-plane: `go/cmd/devtool/main.go:printUsage` ranked #1;
- legacy/bootstrap: `code_tools.go:runCodeBootstrap` and
  `extension.go:extensionBootstrap`;
- build/verify/package: `validation.go:runBuild`;
- Project Extension boundary: `extension.go` and project-root helpers;
- provider responsibility: `main.go:runCapabilities` plus local code-intelligence
  bootstrap symbols;
- migration surface: `code.go:runCodeStatus` appears in the top-8 candidate set,
  providing a development-control-plane anchor for follow-up refinement.

Useful candidate hit rate: **6/6 (100%)**.

Warm provider timings after discovery is initialized:

| Question | Latency |
| --- | ---: |
| control-plane entry | 0.684 s first process call on an existing index |
| legacy/bootstrap | 0.016 s |
| build/verify/package | 0.017 s |
| Project Extension boundary | 0.014 s |
| provider responsibility | 0.015 s |
| minimal migration surface | 0.017 s |

A separate fresh-root measurement recorded the first discovery call at **8.461 s** versus
the Phase-2 fresh-root symbol-search baseline of **7.495 s**. The cold path therefore
regresses by ~0.97 s. This is accepted for this phase because the optimization target is the
persistent warm development node, and candidate quality rises from 67% to 100% while warm
discovery remains tens of milliseconds.

### Phase 3 conclusion

Objective-only discovery now returns repository-level candidates rather than treating a
natural-language architecture question as one symbol name.

For the fixed six-question Repository Understanding probe:

- Agent-visible calls needed: 6 objective-only calls;
- useful top-8 hit rate: 4/6 -> 6/6;
- provider-native Agent tools: 0;
- raw file/grep fallback required for candidate discovery: 0;
- wrong-turn questions at candidate-discovery level: 2 -> 0.

The next phase should focus on result/context budget rather than adding more search
primitives. The warm discovery provider time is already small compared with human/Agent
reasoning and follow-up context consumption.


## Phase 4 — Context/result budget

### Before

Serena returned the raw child-MCP tool envelope through the realtime service. The same
semantic payload was commonly present twice:

- once in `content[0].text`;
- again in `structuredContent.result`.

Representative pre-Phase-4 realtime service response sizes from the enriched
`ProjectHost` probe:

| Realtime operation | Before |
| --- | ---: |
| symbols | 957 bytes |
| references | 5,051 bytes |
| diagnostics | 703 bytes |
| total realtime payload | 6,711 bytes |

The duplicated MCP transport shape had no additional value to `code_context`.

### Change

The Serena provider now unwraps its child-MCP result before returning across the stable
realtime service boundary.

Preference order is deterministic:

1. `structuredContent.result`;
2. first textual content item;
3. original payload only when no compact semantic result is available.

JSON semantic results are returned as JSON. Provider-native MCP envelope metadata is not
forwarded to the Agent.

No source body is added by this change, and `include_body=false` remains respected.

### After

Measured realtime service response sizes:

| Realtime operation | Before | After | Reduction |
| --- | ---: | ---: | ---: |
| symbols | 957 B | 132 B | 86% |
| references | 5,051 B | 2,251 B | 55% |
| diagnostics | 703 B | 256 B | 64% |
| total realtime payload | 6,711 B | 2,639 B | 61% |

Warm enriched latency remained provider-bound:

- measured warm call: 0.375 s;
- symbols: 0.103 s;
- references: 0.140 s;
- diagnostics: 0.130 s.

The indexed result was intentionally not truncated beyond the caller's existing `limit`.
The existing request contract therefore remains the context budget control.

Cross-provider semantic de-duplication was not implemented by teaching the capability
CodeGraph- or Serena-specific response fields. Doing that would violate replaceable-provider
boundaries. Discovery candidates are already deterministically de-duplicated inside the
indexed provider, while the realtime provider now removes exact duplicate MCP envelope
representations before they cross the service boundary.

### Phase 4 conclusion

The largest measured result-budget waste was transport duplication inside the realtime
provider. Removing it cuts realtime response bytes by ~61% without changing Agent tools,
provider selection, or requesting additional source bodies.


## Phase 5 — Remote Connector / MCP decision

### Measurement

A local DevTool HTTP Agent Gateway was started on loopback and the same trivial
`tools/list` request was measured in two paths.

Host-local HTTP, 30 samples:

- p50: **0.624 ms**
- p95: **0.883 ms**
- first/cold outlier: 106 ms

Remote Desktop Commander command path, 12 samples, where each connector operation caused
the Codespace to issue the same loopback `tools/list` request:

- p50: **2.494 s**
- p95: **3.339 s**
- mean: **2.636 s**

The large difference is not inside DevTool's MCP HTTP implementation. It includes the
external Remote Connector command round-trip and remote process/shell launch.

For the active `code_context` provider path:

- Phase-0 MCP lock wait was effectively zero;
- warm CodeGraph provider work is milliseconds after lifecycle reuse;
- warm Serena provider work is ~0.1–0.4 s depending on the requested refinement;
- `core/agent/mcpbridge/http.go` is not on the default CodeGraph + Serena path. It is
  currently used by replaceable remote providers such as Sourcegraph.

### Decision

**Do not change `core/agent/mcpbridge/http.go` for this optimization series.**

The >15% overhead observed through Remote Desktop Commander is external to DevTool.
Changing DevTool's HTTP mutex, batching unrelated calls, or adding another proxy would not
remove that connector/shell round-trip and would add a second transport/control-plane
concern without evidence of an internal bottleneck.

The correct architecture remains:

```text
Remote Agent / Connector
  -> persistent DevTool Agent Gateway
  -> code_context
  -> configured service contracts
  -> replaceable providers
```

The remote integration should reuse the persistent Agent Gateway instead of spawning a
new remote shell command per stable-capability call. That is an integration/infrastructure
concern, not a new DevTool proxy layer.

### Phase 5 conclusion

Transport optimization inside DevTool is **not justified** by the measured data.

No Phase-5 transport code change was made.

---

## Controlled Repository Understanding rerun

After Phases 0–4, the six fixed IDFlow architecture questions were rerun against a fresh
copy of fixed baseline SHA `3a652d74f7c4e34ab29343c33042e548cee248a8`.

One persistent DevTool Agent Gateway process was used. No raw file read, grep/find, or
provider-native Agent tool was required for candidate discovery.

| Question | code_context latency | Useful top-8 candidate |
| --- | ---: | --- |
| development control-plane entry | 6.595 s cold | yes |
| legacy/bootstrap | 0.012 s warm | yes |
| build/verify/package | 0.014 s warm | yes |
| Project Extension boundary | 0.014 s warm | yes |
| DevTool provider responsibility | 0.015 s warm | yes |
| minimal migration surface | 0.018 s warm | yes |

Total stable-capability execution time for all six questions:

**6.669 s**

Useful candidate hit rate:

**6/6 (100%)**

Reference benchmark architecture-map wall-clock:

- Group B Remote Workspace: **3m53s**
- Group C before optimization: **7m50s**

The controlled optimized provider/capability execution time is now far below the B
reference budget. A full human/Agent architecture-map wall-clock contains reasoning and
connector overhead and is therefore not directly equivalent to the 6.669 s provider
execution measurement, but the original DevTool warm-path bottleneck is no longer the
dominant factor.

This satisfies the stop condition for further code-path complexity: the remaining large
round-trip cost measured in this environment is external Remote Connector infrastructure,
not the stable capability or provider composition.
