# IDFlow × DevTool V2 Efficiency Benchmark Plan

## Status

This document defines the benchmark baseline and the minimum DevTool V2 changes required before the benchmark starts.

The benchmark must measure the current architecture before performance optimization.

Pinned baselines:

- DevTool: `96f78f7b937d17f52a5ecdccc9164c8611dfd922`
- IDFlow clean baseline: `3a652d74f7c4e34ab29343c33042e548cee248a8`
- Primary Codespaces machine: **4 vCPU / 16 GB RAM**
- Target machine simulation: **4 vCPU / 4 GB RAM**

The existing IDFlow DevTool migration PR must not be used as the benchmark starting tree. Each benchmark run starts from the clean IDFlow baseline above so the agent has to discover and perform the migration itself.

---

## 1. What we are trying to prove

The hypothesis is not merely that a remote server is fast.

The hypothesis is:

> A persistent remote development node running DevTool V2 beside the source tree improves end-to-end agent engineering throughput because DevTool turns repository-scale indexed intelligence and realtime language intelligence into a small, stable set of coarse-grained remote capabilities. The latency added by the remote connector is smaller than the navigation, context-gathering, failed-tool-call and rework cost that DevTool removes.

The target path is:

```text
ChatGPT / remote agent
        |
        v
remote connector
        |
        v
remote development node
        |
        v
DevTool Agent Gateway
        |
        v
stable agent capability
        |
        +----------------------+
        |                      |
        v                      v
code-indexed             code-realtime
CodeGraph                Serena / LSP
```

The primary product claim must be evaluated at the **engineering-task level**, not by individual command latency.

Primary KPI:

> **Time to merge-ready**, under the same correctness gates.

Secondary metrics explain why the total time changes.

---

## 2. Architecture review relevant to the benchmark

At the pinned DevTool baseline, the important path is already present:

- `core/agent/gateway.go` exposes the stable MCP agent boundary.
- `extensions/capability/code/code.go` exposes `code_context`.
- `service.code-indexed` is implemented by CodeGraph.
- `service.code-realtime` is implemented by Serena/LSP.
- provider details remain behind Service/Capability contracts.
- `environment.local` can execute directly inside an already-running remote machine/container.
- `environment.docker` remains available for normal local portable development.
- project-specific semantics stay in the Project Extension.

Current `code_context` composition is intentionally coarse-grained:

```text
code_context
  -> indexed search
  -> realtime symbols       (when symbol is supplied)
  -> realtime references    (when symbol + path are supplied)
  -> realtime diagnostics   (when path is supplied)
```

This matters because one remote tool call may perform multiple code-intelligence operations beside the source tree.

### Known baseline limitations

These are **benchmark inputs**, not things to optimize before measuring:

1. `code_context` currently invokes its indexed/realtime sub-operations sequentially.
2. objective-only indexed lookup currently maps to the CodeGraph symbol-search path.
3. the remote MCP client serializes its own request path under its current lifecycle/locking model.
4. provider startup/index state may differ between cold and warm runs.

Do not optimize these before the first benchmark. The first benchmark must expose whether they are real bottlenecks.

---

## 3. Required DevTool work before testing

Only add **generic observability and benchmark support**.

Do not change the semantic execution path.

### 3.1 Required behavior

DevTool must be able to emit structured timing spans for one agent request across these boundaries:

```text
remote HTTP / MCP request
  -> Agent Gateway dispatch
  -> Agent Capability
  -> Service invocation
  -> Provider operation
```

For `code_context`, a trace should make it possible to see approximately:

```text
tools/call: code_context             1320 ms
  code_context                      1284 ms
    code-indexed.search              410 ms
    code-realtime.symbols            255 ms
    code-realtime.references         351 ms
    code-realtime.diagnostics        249 ms
```

The exact package layout is implementation-dependent, but the mechanism must remain cross-project and provider-neutral.

### 3.2 Minimal trace schema

Use JSON Lines so traces can be inspected with ordinary shell tools and processed later without introducing a telemetry backend.

Recommended event shape:

```json
{
  "timestamp": "2026-10-05T13:00:00.000Z",
  "trace_id": "…",
  "span_id": "…",
  "parent_span_id": "…",
  "layer": "gateway|capability|service|provider|environment",
  "name": "tools.call|code_context|code-indexed.search|code-realtime.references",
  "tool": "code_context",
  "provider": "intelligence.codegraph",
  "method": "search",
  "duration_ms": 123.45,
  "status": "ok|error",
  "request_bytes": 123,
  "response_bytes": 456,
  "error_class": "",
  "run_id": "…"
}
```

Required properties:

- one `trace_id` connects one top-level tool call to internal spans;
- parent/child relationships are reconstructable;
- duration uses a monotonic clock;
- success/error is explicit;
- provider/service identity is recorded when known;
- request/response **sizes** may be recorded;
- raw source code, prompts, credentials, authorization headers and tool argument bodies must **not** be written to traces.

### 3.3 Enablement

Tracing must be:

- disabled by default;
- explicitly enabled for benchmark runs;
- independent of IDFlow;
- usable by any DevTool-managed project.

A simple generic runtime switch is sufficient for the benchmark baseline, for example:

```text
DEVTOOL_TRACE_FILE=.devtool/traces/benchmark.jsonl
DEVTOOL_RUN_ID=<run-id>
```

Do not create an IDFlow-specific configuration key.

Do not turn TOML into a workflow language just to support this benchmark.

If the implementation chooses a configuration surface instead of environment variables, it must still follow the same constraints: optional, generic, provider-neutral and disabled by default.

### 3.4 Instrumentation points

At minimum instrument:

1. **Agent HTTP/MCP boundary**
   - `core/agent/gateway.go`
   - total request handling and `tools/call`.

2. **Remote MCP client boundary when used**
   - `core/agent/mcpbridge/http.go`
   - request duration and response size.
   - do not log bearer tokens, headers or payload contents.

3. **Stable code capability**
   - `extensions/capability/code/code.go`
   - total `code_context` duration.
   - each indexed/realtime service call as a child span.

4. **Code intelligence providers**
   - CodeGraph operation duration.
   - Serena/LSP operation duration.
   - provider error classification.

5. **Environment execution**
   - local/docker command execution duration at the generic environment boundary.
   - do not log secret-bearing environment values.

Project commands may also be traced through the generic command/service boundary if this falls out naturally from the implementation.

### 3.5 External metrics, not DevTool responsibilities

Do **not** put host resource monitoring into Core.

Collect these externally during benchmark runs:

- peak RSS;
- CPU utilization / CPU time;
- swap activity;
- OOM events;
- container/cgroup memory;
- wall-clock build/verify/package duration.

Use OS/cgroup/Docker/Codespaces facilities for these metrics.

### 3.6 No benchmark-driven product special cases

The implementation must not introduce:

- `if IDFlow`;
- benchmark-only provider behavior;
- alternate Agent Gateway;
- provider-native Agent tools;
- direct CodeGraph/Serena startup in IDFlow;
- a second SCM path;
- a second environment/runtime control plane;
- changed `code_context` semantics.

---

## 4. Explicitly forbidden optimizations before the first run

Do not implement any of the following until the baseline data exists:

- parallelize CodeGraph and Serena operations;
- add speculative prefetch;
- change CodeGraph query semantics;
- add new caching policy;
- change provider lifecycle solely for speed;
- batch or compress MCP requests;
- bypass the stable `code_context` capability;
- bypass the configured environment service;
- change the remote connector.

The sequence is:

```text
instrument current architecture
        ->
measure
        ->
identify bottleneck
        ->
optimize one layer
        ->
repeat the same benchmark
```

This preserves causal evidence.

---

## 5. Benchmark experiment groups

All primary comparison groups use the same **4 vCPU / 16 GB Codespace**.

The purpose is to keep machine saturation from contaminating the architecture comparison.

### Group A — Remote baseline without DevTool code intelligence

```text
remote agent
  -> remote connector
  -> same Codespace
  -> ordinary repository navigation / shell / file reads
```

Rules:

- no `code_context`;
- no direct CodeGraph/Serena use;
- the agent may use ordinary shell/file/Git operations that are available in the baseline environment;
- use the same model and task prompt as Group B.

This measures the current remote-development baseline.

### Group B — Target architecture

```text
remote agent
  -> remote connector
  -> same Codespace
  -> DevTool local environment profile
  -> code_context
     -> CodeGraph
     -> Serena/LSP
```

This is the architecture that a future persistent cloud development node is intended to run.

### Group C — Transport isolation

```text
agent running directly in Codespace
  -> DevTool
  -> code_context
     -> CodeGraph
     -> Serena/LSP
```

No remote connector in the critical path.

Comparison:

```text
B vs A = value of DevTool code intelligence in the remote workflow
B vs C = cost of the remote connector / transport layer
```

### Group D — 4 GB target-machine simulation

After A/B/C finish on 4C16G, run the target path again with the DevTool workload constrained to approximately:

```text
4 vCPU
4 GB RAM
```

Keep every other variable fixed.

This answers whether the low-cost 4C4G cloud machine is sufficient.

A 2C8G Codespace may be used later only as a secondary CPU-vs-memory stress experiment. It is not the primary benchmark machine.

---

## 6. IDFlow benchmark task

Use a real migration, not a synthetic micro-project.

Every run begins from:

```text
ThinkerQAQ/IDFlow
3a652d74f7c4e34ab29343c33042e548cee248a8
```

The agent must not inspect or reuse the existing DevTool migration PR as an answer key.

### Task objective

Migrate IDFlow to the latest pinned DevTool baseline using the normal architecture:

```text
Configure first

IDFlow
  -> .devtool.toml
  -> IDFlow Project Extension
  -> configured environment
  -> CodeGraph + Serena/LSP
  -> SCM/Credential capability
```

Required architectural outcomes:

- IDFlow-specific DevTool Core changes: **0**;
- project-specific build/verify/package semantics live in the IDFlow Project Extension;
- provider implementation details do not leak to the Agent boundary;
- no project-local CodeGraph/Serena bootstrap;
- no duplicated SCM publication implementation;
- legacy development-control paths are removed when replaced;
- final build / verify / package path succeeds through DevTool.

### Fairness rules

For every A/B/C run:

- identical IDFlow commit;
- identical DevTool commit;
- identical model/configuration;
- identical task prompt;
- separate worktree/branch;
- fresh agent conversation/session;
- no access to another group's patch;
- no access to PR #10 as an implementation guide;
- same correctness gates.

Record deviations explicitly.

---

## 7. Two-stage benchmark

### Stage 1 — Repository understanding

Before modifying code, give each group the same questions:

1. Where is IDFlow's current development control-plane entry?
2. Which code is legacy/bootstrap code that should disappear after DV2 migration?
3. How do build, verify and package currently flow through the repository?
4. Which semantics belong in the IDFlow Project Extension?
5. Which concerns should be supplied by DevTool providers instead of IDFlow code?
6. What is the smallest coherent set of files that must change?

Measure:

- time to a correct architecture map;
- correct file/symbol hit rate;
- incorrect assumptions;
- raw file reads;
- shell/grep/search calls;
- DevTool tool calls;
- human corrections.

The answer should be judged against the final accepted implementation, not against prose style.

### Stage 2 — Full migration

Measure from task start to:

1. first valid patch;
2. first successful build;
3. first successful verify/package path;
4. merge-ready state.

Primary output:

```text
Time to merge-ready
```

---

## 8. Connector microbenchmark

Do this separately from the LLM task.

The connector tax must be measured independently so it is not confused with CodeGraph/LSP work.

Run a trivial remote operation repeatedly, for example:

```text
ping
pwd
git rev-parse HEAD
```

Use enough repetitions to report at least:

- p50;
- p95;
- maximum;
- failure rate.

Then run the same operation directly on the Codespace.

Approximation:

```text
connector tax
  = remote end-to-end latency
  - server-local execution latency
```

Also report:

```text
connector overhead ratio
  = connector tax
  / total code_context latency
```

The connector is acceptable for this architecture when its median overhead is small relative to the coarse-grained work done behind a single tool call.

Initial interpretation bands:

- **< 15%**: acceptable;
- **15–25%**: measurable but not automatically blocking;
- **> 25%**: investigate the transport layer before buying dedicated hardware.

---

## 9. Cold and warm measurements

Measure code intelligence under both states.

### Cold

- fresh worktree;
- no CodeGraph index;
- provider processes not started;
- relevant caches cleared when practical.

Record:

- provider startup time;
- index/verify time;
- first `code_context` latency.

### Warm

- index already present;
- provider lifecycle established;
- normal development caches available.

Warm-state behavior is the main development-machine KPI because a persistent worker spends most of its useful lifetime warm.

For each state measure:

- objective-only `code_context`;
- objective + symbol;
- objective + symbol + path;
- build;
- verify;
- package.

Do not use cold and warm results interchangeably.

---

## 10. Metrics

### Primary

- **time_to_merge_ready_ms**

### Productivity

- time_to_architecture_map;
- time_to_first_valid_patch;
- time_to_first_green;
- time_to_merge_ready;
- human correction count;
- failed-command count;
- rollback/rework count.

### Navigation / context

- raw file reads;
- grep/search invocations;
- Agent tool calls;
- `code_context` calls;
- indexed provider calls;
- realtime provider calls;
- files touched;
- symbols/references requested.

### Transport

- connector p50/p95/max;
- request/response bytes;
- remote MCP p50/p95;
- failure/retry count.

### Runtime

- CodeGraph duration;
- Serena/LSP duration;
- `code_context` total duration;
- environment-command duration;
- build/verify/package duration.

### Resources

External collection:

- peak RSS;
- CPU time/utilization;
- swap;
- OOM;
- disk/cache growth.

---

## 11. Success criteria

Do not redefine success after seeing the data.

The target architecture is considered validated for the IDFlow experiment when all of these hold:

1. **Correctness**
   - all compared implementations pass the same final acceptance gates.

2. **Productivity**
   - Group B reduces time-to-merge-ready by at least **20%** versus Group A, or produces equivalent time with a substantial reduction in human correction/rework.

3. **Navigation**
   - Group B reduces raw navigation/file/tool churn by roughly **30%** or more versus Group A.

4. **Human intervention**
   - Group B does not require more human corrections than Group A.

5. **Transport**
   - connector overhead is preferably **<15%** of warm `code_context` latency and is not a dominant component of end-to-end task time.

6. **4 GB viability**
   - no OOM;
   - no sustained swap thrashing;
   - peak working set leaves operational headroom;
   - core warm workloads are no more than roughly **20%** slower than the unconstrained 4C16G reference unless the total task-level result is still clearly acceptable.

If 4 GB fails this gate, the benchmark should recommend a larger development node rather than hiding the problem with aggressive swap.

---

## 12. Result artifact

Each run should produce a small machine-readable and human-readable result set, for example:

```text
.devtool/benchmark/<run-id>/
  metadata.json
  trace.jsonl
  metrics.json
  summary.md
```

`metadata.json` should pin at least:

- DevTool SHA;
- IDFlow SHA;
- group A/B/C/D;
- machine CPU/RAM;
- agent/model identifier where available;
- run start/end;
- cold/warm state;
- prompt/version identifier.

Do not commit secrets, bearer tokens or raw private prompts/source snapshots into result artifacts.

Benchmark results may be committed later to a dedicated docs/results area after private/sensitive fields are reviewed.

---

## 13. Implementation plan for the local agent

Implement the benchmark baseline in small coherent commits.

### Commit 1 — Generic trace primitives

- add minimal generic trace/span primitives;
- disabled by default;
- JSONL output;
- run/trace/span identifiers;
- security rule: metadata only, no source/prompt/credentials.

### Commit 2 — Agent/Gateway instrumentation

- instrument HTTP/MCP top-level request;
- instrument `tools/call`;
- preserve existing behavior and status codes.

### Commit 3 — Capability and provider instrumentation

- instrument `code_context`;
- child spans for indexed and realtime service calls;
- instrument CodeGraph and Serena provider operations.

### Commit 4 — Environment/project execution timing

- instrument generic environment execution boundary;
- ensure local/docker behavior is unchanged.

### Commit 5 — Benchmark harness/support

Add only lightweight, reusable benchmark support needed to:

- create/run tagged benchmark sessions;
- write metadata/result files;
- distinguish cold/warm runs;
- avoid hand-calculated timestamps.

Do not add an IDFlow-specific benchmark implementation to DevTool Core.

### Commit 6 — Verification and docs

Verify:

```text
go test ./...
DevTool self-host verify/package path
trace disabled -> behavior unchanged
trace enabled -> valid JSONL
no secrets in trace output
provider replacement remains configuration-only
```

Then update the relevant reference documentation for the new generic trace switch.

Follow the repository development rhythm:

```text
change a coherent unit
  -> commit
  -> push
  -> next unit
  -> commit
  -> push
  -> concentrated verification
```

Do not wait until the whole implementation is complete before pushing.

---

## 14. Merge gate for benchmark readiness

The DevTool implementation is benchmark-ready when:

- trace is generic and disabled by default;
- one remote `code_context` call produces a reconstructable span tree;
- CodeGraph and Serena/LSP child costs are visible;
- environment/project-command timing is visible where used;
- no execution semantics have been optimized or altered;
- no IDFlow special case exists;
- no provider-native tool leaked to the Agent surface;
- tests/self-hosting pass;
- the trace output contains no credentials/source payloads.

Only after this gate should the actual IDFlow A/B/C/D benchmark begin.

---

## 15. Decision after the benchmark

The benchmark should answer three separate decisions:

### Architecture decision

Does DV2 code intelligence improve remote agent engineering throughput?

Derived mainly from:

```text
B vs A
```

### Connector decision

Is the remote connector cheap enough to keep in the architecture?

Derived mainly from:

```text
B vs C
+ connector microbenchmark
```

### Hardware decision

Is a persistent 4C4G development node enough, or is more memory required?

Derived mainly from:

```text
D vs B
+ memory/swap/OOM/resource metrics
```

Only after these are answered should performance optimizations or a cloud-machine purchase be justified from benchmark evidence.
