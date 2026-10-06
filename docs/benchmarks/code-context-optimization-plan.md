# DevTool V2 — code_context Time-to-Useful-Context Optimization Plan

## 0. Goal

The A/B/C benchmark established:

~~~text
Group B — Remote Workspace
Architecture Map: 3m53s

Group C — Remote + DV2
Architecture Map: 7m50s
~~~

At the same time, Group C reduced Stage-1 low-level navigation to:

~~~text
raw file reads:         0
grep/find:              0
shell fallback:         0
effective code_context: 7
~~~

Therefore the next optimization target is narrow:

> Reduce code_context Time-to-Useful-Context without breaking the stable capability / replaceable provider architecture.

This phase is not for adding new providers, UI, workflow features, or project-specific logic.

---

## 1. Success criteria

### Primary

Repeat only the Repository Understanding benchmark first.

Target:

~~~text
C optimized Architecture Map <= 3m53s
stretch target              <= 3m00s
~~~

### Secondary

Keep all of these properties:

~~~text
raw file reads               ~= 0
grep/find fallback           ~= 0
provider-native Agent tools   = 0
IDFlow-specific DV2 Core      = 0
correct architecture map      = PASS
~~~

For micro measurements, compare optimized values against the frozen pre-optimization baseline rather than inventing absolute latency targets.

Measure at least:

- cold code_context;
- warm code_context;
- CodeGraph duration;
- Serena/LSP duration;
- capability composition duration;
- Remote Connector / MCP duration where observable.

---

## 2. Constraints

Architecture remains:

~~~text
Agent
  -> stable capability
  -> service contract
  -> replaceable provider
~~~

Do not introduce:

- if IDFlow;
- direct CodeGraph/Serena Agent tools;
- project-managed provider lifecycle;
- a second Agent Gateway;
- hard-coded provider selection;
- benchmark-only product behavior;
- a second code-intelligence control plane.

All provider replacement must remain configuration-driven.

Do not optimize multiple causal layers in one commit. We need to know which change produced the improvement.

---

## 3. Baseline code path to inspect

Primary files:

~~~text
core/agent/gateway.go
core/agent/mcpbridge/http.go

extensions/capability/code/code.go

extensions/intelligence/codegraph/codegraph.go
extensions/intelligence/serena/serena.go

sdk/codeintelligence/
~~~

Current logical path:

~~~text
code_context
  -> code-indexed.search
     -> CodeGraph
  -> code-realtime.symbols / references / diagnostics
     -> Serena/LSP
~~~

Known benchmark-relevant risks:

1. provider startup/index cost;
2. indexed and realtime calls are composed sequentially;
3. objective-only lookup currently depends heavily on indexed symbol search quality;
4. MCP/provider request serialization may add wait time;
5. response payload may contain more context than the next Agent step needs.

---

## 4. Phase 0 — Observability first

Before changing behavior, make one code_context call explain where time went.

Use the smallest generic tracing mechanism already present; if the current branch does not yet contain usable tracing, add only minimal JSONL timing.

Required spans:

~~~text
tools/call code_context
  -> code_context
     -> code-indexed.search
        -> CodeGraph provider
     -> code-realtime.*
        -> Serena/LSP provider
~~~

Also capture when practical:

~~~text
provider cold_start
provider warm_request
index/update
MCP lock_wait
MCP request
response_bytes
~~~

Rules:

- tracing disabled by default;
- no source code/prompt/token/credential bodies;
- generic across projects;
- no benchmark framework.

Deliverable:

> A cold and warm timing breakdown for the current implementation before optimization.

Commit separately.

---

## 5. Phase 1 — Provider lifecycle reuse

First inspect whether CodeGraph and Serena/LSP processes/connections/index state are recreated or revalidated more often than necessary.

Target steady state:

~~~text
ProjectHost lifetime
  ├─ CodeGraph provider instance reused
  ├─ Serena/LSP provider bridge reused
  └─ code_context requests reuse existing provider state
~~~

Implement only if measurement confirms repeated startup/reinitialization.

Requirements:

- lifecycle belongs to provider/host boundary, not IDFlow;
- clean shutdown/cancellation remains correct;
- branch/worktree changes must not silently use stale semantic state;
- provider replacement remains config-only.

Validation:

- first call may remain cold;
- second and later calls must show materially lower provider-startup overhead;
- no stale symbol/reference results after a controlled file change.

Commit separately.

---

## 6. Phase 2 — Parallelize independent indexed/realtime work

Current capability composition should be changed only where operations are independent.

For inputs that already contain enough user-supplied information:

~~~text
symbol
symbol + path
path
~~~

run independent branches concurrently:

~~~text
             -> code-indexed.search
code_context
             -> code-realtime.*
~~~

Then merge results deterministically.

Do not parallelize when one result is semantically required to construct the next request.

Requirements:

- preserve cancellation;
- fail predictably when one branch errors;
- no goroutine leaks;
- stable output schema/order;
- no provider-specific concurrency logic in Core.

Expected benefit:

> Reduce wall-clock latency from approximately sum(indexed, realtime) toward max(indexed, realtime).

Commit separately.

---

## 7. Phase 3 — Improve objective-only candidate discovery

The benchmark showed that reducing low-level navigation is not enough; objective-only must produce useful repository context quickly.

First inspect the real CodeGraph provider/tool surface.

Do not assume a new API exists.

Preferred strategy:

~~~text
natural-language objective
  -> repository-scale candidate discovery
  -> small ranked set of files/symbols
  -> optional realtime refinement
~~~

If CodeGraph already exposes a more suitable generic search primitive, adapt the indexed provider behind the existing service contract.

If only symbol search is available, keep the current provider contract and implement the smallest query normalization/ranking improvement supported by evidence.

Requirements:

- do not expose new provider-native Agent tools;
- do not make Core understand CodeGraph;
- preserve code_context as the stable Agent entry;
- return a bounded candidate set.

Validation:

Use the same six IDFlow architecture questions from the benchmark and compare:

- number of code_context calls;
- useful file/symbol hit rate;
- wrong turns;
- architecture-map time.

Commit separately.

---

## 8. Phase 4 — Context/result budget

Reduce transport and Agent parsing cost without reducing correctness.

For code_context, prefer a compact bundle:

~~~text
relevant files
relevant symbols
references when requested
diagnostics when requested
short rationale / match reason
~~~

Avoid returning large bodies unless include_body=true.

Add deterministic deduplication across indexed/realtime results.

Keep limits configurable through the existing request contract; do not add a second context API.

Measure:

- response bytes;
- Agent follow-up calls;
- architecture-map correctness.

Commit separately.

---

## 9. Phase 5 — Remote Connector / MCP only if it is material

Do not optimize transport by intuition.

Measure:

~~~text
host-local trivial request
vs
Remote Connector same request
~~~

and:

~~~text
MCP lock_wait
MCP request time
provider execution time
~~~

Decision rule:

~~~text
connector share < 15%
  -> leave it alone

connector share materially > 15%
  -> investigate session reuse / serialization / payload cost
~~~

In particular, inspect the mutex/serialization path in:

~~~text
core/agent/mcpbridge/http.go
~~~

Only change it if tracing proves lock contention or request serialization is a meaningful part of code_context latency.

Do not batch unrelated operations simply to improve benchmark numbers.

---

## 10. Cold vs warm policy

The persistent remote-development-machine use case is mainly warm.

Optimize in this order:

~~~text
1. warm code_context
2. provider reuse
3. cold startup
~~~

Do not make every DevTool startup eagerly initialize all providers unless data proves that is worthwhile.

If cold startup remains expensive after warm-path optimization, consider a configurable provider warm-up policy as a separate change.

That must remain:

- optional;
- configuration-driven;
- provider-neutral at the Agent surface.

---

## 11. Implementation sequence

Use small commits and push each coherent change.

Recommended order:

~~~text
1. perf: add code-context latency breakdown
2. perf: reuse code intelligence provider lifecycle
3. perf: parallelize independent code-context branches
4. perf: improve indexed objective discovery
5. perf: reduce code-context result payload
6. perf: optimize MCP transport only if measurements justify it
~~~

After each phase:

~~~text
commit
push
run focused tests
record before/after latency
~~~

Do not wait until the entire optimization series is complete before pushing.

---

## 12. Verification

Every optimization must preserve:

~~~text
go test ./...
DevTool self-host verification
provider replacement by configuration
cancellation behavior
no stale semantic results
no provider-native Agent API
~~~

Focused code-intelligence tests should include:

1. objective-only query;
2. symbol query;
3. symbol + path references;
4. path diagnostics;
5. provider error;
6. cancellation;
7. repeated warm calls;
8. source change followed by fresh semantic result.

---

## 13. Benchmark rerun

Do not rerun the full migration first.

Freeze existing benchmark branches.

Run the small controlled Repository Understanding test:

~~~text
B-understanding baseline
  = 3m53s

C-before optimization
  = 7m50s

C-after optimization
  = ?
~~~

Use the same six architecture questions and the same fixed IDFlow SHA.

Record:

- T0 -> ARCHITECTURE_MAP_READY;
- code_context count;
- objective-only vs enriched calls;
- raw reads;
- grep/find fallbacks;
- CodeGraph latency;
- Serena/LSP latency;
- provider cold/warm latency;
- connector latency;
- wrong turns;
- correctness.

Only after C optimized reaches or beats the B baseline should we repeat the full migration benchmark.

---

## 14. Stop conditions

Do not continue adding complexity when:

- C optimized <= B architecture-map time;
- correctness remains equal;
- raw navigation stays near zero;
- provider-native tools remain hidden;
- further optimization produces only marginal gains.

If the largest remaining latency is external Remote Connector overhead, stop changing DV2 and treat that as a separate infrastructure problem.

---

## 15. Final target

The desired steady-state path is:

~~~text
ChatGPT
  -> Remote Connector
  -> persistent remote node
  -> DevTool V2
  -> one/few coarse-grained code_context calls
     -> warm CodeGraph
     -> warm Serena/LSP
     -> parallel where independent
     -> compact merged context
~~~

Target outcome:

> DV2 keeps the low navigation/tool churn observed in Group C while matching or beating the Group B Time-to-Useful-Context.
