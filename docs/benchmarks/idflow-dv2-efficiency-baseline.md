# IDFlow × DevTool V2 Efficiency Benchmark Plan

## Status

This document defines the benchmark baseline and the minimum DevTool V2 changes required before the benchmark starts.

The benchmark must measure the current architecture before performance optimization.

Pinned baselines:

- DevTool: `96f78f7b937d17f52a5ecdccc9164c8611dfd922`
- IDFlow clean baseline: `3a652d74f7c4e34ab29343c33042e548cee248a8`
- Groups B/C remote machine: **GitHub Codespaces, 4 vCPU / 16 GB RAM**
- Target cloud-machine simulation: **4 vCPU / 4 GB RAM**

The existing IDFlow DevTool migration PR must not be used as the benchmark starting tree. Each benchmark run starts from the clean IDFlow baseline above so the agent has to discover and perform the migration itself.

There are exactly **three end-to-end comparison groups**:

- **A — GitHub Connector baseline**
- **B — Remote Connector baseline**
- **C — Remote Connector + DevTool V2 target**

Direct execution inside Codespaces is not a fourth end-to-end group. It is used only for transport microbenchmarks and host-local reference measurements.

---

## 1. What we are trying to prove

The hypothesis is not merely that a remote server is fast.

The hypothesis has three separable parts:

> Moving from GitHub-API-only editing to a real remote workspace improves engineering throughput; adding DevTool V2 on that same remote workspace improves it further by turning repository-scale indexed intelligence and realtime language intelligence into a small, stable set of coarse-grained capabilities; the remote connector overhead remains small enough that the combined architecture is still faster end to end.

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

Only add the **minimum generic observability needed to start the benchmark**.

Do not build a benchmark framework and do not change the semantic execution path.

### 3.1 Required trace path

The mandatory path is:

```text
Agent Gateway tools/call
  -> code_context
  -> code-indexed.search
     -> CodeGraph
  -> code-realtime.symbols/references/diagnostics
     -> Serena/LSP
```

A trace should make it possible to reconstruct approximately:

```text
tools/call: code_context             1320 ms
  code_context                      1284 ms
    code-indexed.search              410 ms
      CodeGraph provider             ...
    code-realtime.symbols            255 ms
      Serena/LSP provider            ...
    code-realtime.references         351 ms
    code-realtime.diagnostics        249 ms
```

Remote MCP client timing in `core/agent/mcpbridge/http.go` is useful but **optional for the minimum baseline**. Add it only if it remains a small, isolated change.

Environment-wide tracing, benchmark result generation and host resource collectors are explicitly deferred.

### 3.2 Minimal trace schema

Use JSON Lines.

Minimum useful fields:

```json
{
  "timestamp": "2026-10-05T13:00:00.000Z",
  "trace_id": "…",
  "span_id": "…",
  "parent_span_id": "…",
  "layer": "gateway|capability|service|provider",
  "name": "tools.call|code_context|code-indexed.search|code-realtime.references",
  "provider": "intelligence.codegraph",
  "method": "search",
  "duration_ms": 123.45,
  "status": "ok|error",
  "request_bytes": 123,
  "response_bytes": 456,
  "run_id": "…"
}
```

Requirements:

- one `trace_id` connects a top-level tool call to internal spans;
- parent/child relationships are reconstructable;
- duration uses a monotonic clock;
- success/error is explicit;
- provider/service identity is recorded when known;
- request/response sizes may be recorded;
- raw source code, prompts, tool argument bodies, credentials, authorization headers and secret-bearing environment values must never be written to traces.

### 3.3 Enablement

Tracing must be:

- disabled by default;
- explicitly enabled for benchmark runs;
- independent of IDFlow;
- usable by any DevTool-managed project.

The smallest acceptable runtime surface is:

```text
DEVTOOL_TRACE_FILE=.devtool/traces/benchmark.jsonl
DEVTOOL_RUN_ID=<run-id>
```

Do not add an IDFlow-specific configuration key and do not turn TOML into a benchmark workflow language.

### 3.4 Mandatory instrumentation points

Instrument only what is required for causal attribution:

1. **Agent Gateway**
   - `core/agent/gateway.go`
   - `tools/call` total duration.

2. **Stable code capability**
   - `extensions/capability/code/code.go`
   - total `code_context`;
   - indexed and realtime child service calls.

3. **Code intelligence providers**
   - CodeGraph operation duration;
   - Serena/LSP operation duration;
   - error status/classification.

4. **Optional remote MCP client**
   - `core/agent/mcpbridge/http.go`;
   - request duration/response size only if cheap to add.

### 3.5 Metrics deliberately kept outside DevTool

Collect these externally during benchmark runs:

- total wall-clock task time;
- peak RSS;
- CPU utilization / CPU time;
- swap activity;
- OOM events;
- build/verify/package wall time;
- connector microbenchmark latency.

DevTool Core must not become a host monitoring system.

### 3.6 No benchmark-driven special cases

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

There are exactly three end-to-end groups.

Groups B and C use the same **4 vCPU / 16 GB Codespace**. Group A intentionally has no Codespace because the absence of a real remote workspace is part of the baseline being measured.

### Group A — GitHub Connector baseline

```text
ChatGPT
  -> GitHub Connector
  -> GitHub repository APIs
  -> file/search/commit/branch/PR operations
```

Rules:

- no remote development machine;
- no Remote Connector;
- no DevTool;
- no CodeGraph/Serena/LSP;
- code understanding comes from the GitHub Connector surface available to ChatGPT;
- validation may use repository CI or other capabilities naturally available through the GitHub workflow, but must not secretly introduce a remote shell workspace.

This is the baseline that represents direct ChatGPT-to-GitHub development.

### Group B — Remote Connector baseline

```text
ChatGPT
  -> Remote Connector
  -> Codespace 4C16G
  -> ordinary shell / filesystem / git / build tools
```

Rules:

- real checked-out workspace is available;
- ordinary shell, file reads, `rg`/search, Git, Go build/test and similar development operations are allowed;
- no `code_context`;
- no direct CodeGraph/Serena use;
- no DevTool code-intelligence capability.

This isolates the value of having a real remote development workspace and Remote Connector.

### Group C — Remote Connector + DevTool V2 target

```text
ChatGPT
  -> Remote Connector
  -> same Codespace 4C16G
  -> DevTool V2 using environment.local
  -> stable capabilities
     -> code_context
        -> CodeGraph
        -> Serena/LSP
     -> project_*
     -> scm_*
```

Rules:

- use the same Codespace class as Group B;
- code understanding should prefer `code_context`;
- project operations should prefer `project_*` where available;
- SCM operations should prefer the configured DevTool SCM capability;
- do not bypass DV2 merely to make the benchmark look faster unless the capability is genuinely missing or broken; record every bypass.

This is the target architecture intended for a persistent cloud development node.

### Causal comparisons

```text
B vs A
= value of Remote Connector + real remote workspace

C vs B
= incremental value of DevTool V2 + CodeGraph/LSP

C vs A
= total value of the final architecture
```

### Separate 4C4G hardware stress run

The 4C4G test is **not a fourth agent group**.

After Group C has been measured on 4C16G, repeat representative Group C workloads while constraining the DevTool workload to approximately:

```text
4 vCPU
4 GB RAM
```

Keep the code, task and provider configuration fixed.

This answers whether the low-cost 4C4G cloud machine is sufficient.

A 2C8G Codespace may be used later only as a secondary CPU-vs-memory stress experiment.

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

- identical IDFlow starting commit;
- identical DevTool target commit;
- identical ChatGPT model/configuration where the product surface permits;
- identical task prompt and acceptance criteria;
- separate benchmark branch/worktree or equivalent isolated Git history;
- fresh agent conversation/session;
- no access to another group's patch;
- no access to PR #10 as an implementation guide;
- same final correctness gates.

Environment equality applies where it is part of the controlled comparison:

- B and C must use the same Codespace class and base development environment;
- A intentionally does not use Codespaces because "GitHub Connector only" is the independent variable;
- differences caused by the tool surface itself are part of the measurement and must not be normalized away.

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

This is a **separate transport control**, not an end-to-end experiment group.

The purpose is to estimate Remote Connector overhead without changing the agent used for the full migration.

On the same Codespace used by B/C, measure a trivial operation through the Remote Connector repeatedly, for example:

```text
ping
pwd
git rev-parse HEAD
```

Record at least:

- p50;
- p95;
- maximum;
- failure rate.

Then measure the same underlying command directly on the Codespace host as a local execution reference.

Approximation:

```text
connector tax
  = remote end-to-end operation latency
  - host-local operation latency
```

For DV2 calls also report:

```text
connector overhead ratio
  = connector tax
  / warm code_context latency
```

Initial interpretation bands:

- **< 15%**: acceptable;
- **15–25%**: measurable but not automatically blocking;
- **> 25%**: investigate the transport layer.

Do not run a fourth "agent inside Codespace" migration. Host-local execution exists only to isolate transport cost.

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

### 11.1 Correctness gate

All three groups must satisfy the same final migration acceptance criteria. A faster incorrect implementation does not count.

### 11.2 Overall architecture

The final architecture is validated when Group C demonstrates a meaningful end-to-end advantage over Group A.

Target:

- **C reduces time-to-merge-ready by at least 20% versus A**, or
- C achieves comparable wall time with substantially fewer human corrections, failed edits and rework.

### 11.3 Incremental DV2 value

DV2 code intelligence is considered valuable when C improves on B in at least one strong dimension without regressing correctness:

- lower time-to-architecture-map or time-to-merge-ready;
- roughly **30% lower raw navigation/file/tool churn**;
- materially fewer incorrect assumptions/rework/human corrections.

A target of ~15%+ task-time improvement from C vs B is strong evidence, but the benchmark should preserve the raw data even when the improvement is smaller.

### 11.4 Remote workspace value

B vs A determines whether a real remote workspace is itself valuable.

This result must be reported independently from DV2 so that improvements are not incorrectly attributed to CodeGraph/LSP.

### 11.5 Transport

Remote Connector overhead should preferably be:

- **<15%** of warm `code_context` latency;
- non-dominant in end-to-end task time.

### 11.6 4 GB viability

The 4C4G stress run passes when:

- no OOM occurs;
- there is no sustained swap thrashing;
- peak working set leaves operational headroom;
- representative warm DV2 workloads are no more than roughly **20%** slower than the 4C16G reference unless task-level throughput remains clearly acceptable.

If 4 GB fails, recommend a larger persistent node instead of hiding the problem with aggressive swap.

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
- group A/B/C;
- machine CPU/RAM;
- resource profile (`4c16g-reference` or `4c4g-stress`) when applicable;
- agent/model identifier where available;
- run start/end;
- cold/warm state;
- prompt/version identifier.

Do not commit secrets, bearer tokens or raw private prompts/source snapshots into result artifacts.

Benchmark results may be committed later to a dedicated docs/results area after private/sensitive fields are reviewed.

---

## 13. Minimum implementation plan for the local agent

The current local-agent quota is limited. Implement only the smallest benchmark-ready tracing path.

Follow the normal development rhythm:

```text
change one coherent unit
  -> commit
  -> push
  -> next unit
  -> commit
  -> push
  -> concentrated verification
```

Do not run the full regression suite after every small edit.

### Commit 1 — Minimal structured tracing

- add generic trace/span primitives;
- disabled by default;
- JSONL output;
- `DEVTOOL_TRACE_FILE` / `DEVTOOL_RUN_ID` or an equivalently small generic switch;
- trace/span/parent/run identifiers;
- metadata only: no source, prompt, credentials or secret-bearing environment values.

### Commit 2 — Trace the code-intelligence critical path

Instrument:

- Agent Gateway `tools/call`;
- `code_context`;
- indexed service call;
- realtime symbols/references/diagnostics calls;
- CodeGraph provider duration;
- Serena/LSP provider duration.

If remote MCP client timing is trivial to add safely, include it. Otherwise defer it.

Do not add environment-wide tracing.

### Commit 3 — Focused verification and documentation

Run concentrated verification:

```text
go test ./...
trace disabled -> behavior unchanged
trace enabled -> valid JSONL
one code_context -> reconstructable parent/child spans
CodeGraph vs Serena/LSP cost -> distinguishable
trace output -> no secrets/source payloads
```

Run the existing self-host/delivery verification required by the changed boundary if it is practical within the session.

Update only the reference documentation necessary to explain how to enable tracing.

### Explicitly deferred

Do not spend the pre-benchmark implementation budget on:

- benchmark harness;
- automatic `metrics.json`;
- automatic `summary.md`;
- CPU/RSS collector;
- cold/warm state manager;
- environment-wide tracing;
- dashboard;
- OpenTelemetry backend;
- Prometheus.

These can be added after the first benchmark if the data proves they are worth productizing.

---

## 14. Merge gate for benchmark readiness

The DevTool implementation is benchmark-ready when:

- tracing is generic and disabled by default;
- one `code_context` call produces a reconstructable span tree;
- Agent Gateway, `code_context`, CodeGraph and Serena/LSP costs are distinguishable;
- no execution semantics have been optimized or altered;
- no IDFlow special case exists;
- no provider-native tool leaked to the Agent surface;
- focused tests pass;
- trace output contains no credentials, prompts, tool bodies or source payloads.

Environment-wide timing, an automated benchmark harness and automatic resource collection are **not** merge gates for the first benchmark.

Only after this gate should the actual IDFlow A/B/C benchmark begin.

---

## 15. Decision after the benchmark

The benchmark must answer four separate questions.

### Remote-workspace decision

Does having a real remote development workspace improve over GitHub-Connector-only editing?

Derived from:

```text
B vs A
```

### DevTool decision

Does DV2 + CodeGraph/LSP add incremental value once the same remote workspace already exists?

Derived from:

```text
C vs B
```

### Final architecture decision

Does the complete architecture justify using a persistent cloud development node?

Derived from:

```text
C vs A
```

### Connector decision

Is Remote Connector overhead small enough to remain in the architecture?

Derived from:

```text
connector microbenchmark
+ trace-level transport share when available
```

Do not infer connector cost from C vs B because both groups use the same Remote Connector.

### Hardware decision

Is a persistent 4C4G development node sufficient?

Derived from:

```text
Group C on 4C4G stress profile
vs
Group C on 4C16G reference
+ memory/swap/OOM/resource metrics
```

Only after these questions are answered should we optimize DV2 or justify buying the persistent cloud machine from benchmark evidence.
