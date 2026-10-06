# DevTool V2 — C-After Optimization Repository Understanding Benchmark

## 1. Fixed Baseline

| Item | Value |
|---|---|
| Repository | `ThinkerQAQ/IDFlow` |
| IDFlow SHA | `3a652d74f7c4e34ab29343c33042e548cee248a8` |
| DevTool V2 SHA | `f6e2759c9ed4ade275f00842fbc5b41d2f921f5e` |
| Codespace | `codespaces-a35b78` |
| CPU | 4 vCPU, AMD EPYC 7763 |
| RAM | 15 GiB reported |
| Swap | 0 |
| Kernel | Linux 6.8.0-1064-azure x86_64 |

Freshness checks:

- IDFlow worktree started clean.
- No B/C-before benchmark worktree was reused.
- No benchmark migration branch was checked out.
- No Group C CodeGraph index was copied in.
- CodeGraph reported `No persisted graph found — starting fresh`.
- IDFlow was pinned and verified by SHA.

## 2. Protocol Status

### IDFlow worktree

**PASS**

Before and after Repository Understanding:

```text
IDFlow HEAD:
3a652d74f7c4e34ab29343c33042e548cee248a8

IDFLOW_DIRTY_COUNT=0
```

No `.devtool.toml`, source, benchmark report, temporary file, Serena config, or CodeGraph data was written into the IDFlow worktree.

DevTool bootstrap used an external wrapper under:

```text
/tmp/devtool-benchmark/project
```

with IDFlow exposed as the configured code workspace.

### Existing migration exposure

- PR #10: **not viewed**
- A benchmark branch: **not viewed**
- B benchmark branch: **not viewed**
- C-before benchmark branch: **not viewed**
- Prior Architecture Map: **not viewed**
- Prior migration implementation/diff: **not viewed**

`ACCIDENTAL_EXPOSURE = NO`

### Protocol deviation

**YES — environment readiness was incomplete at official T0.**

The external DevTool configuration was validated before T0, but `codegraph-server` and `serena` executables were not installed in the fresh Codespace.

The first real cold `code_context` call therefore failed with:

```text
codegraph-server: executable file not found in $PATH
```

The missing provider runtimes were installed **after T0**. T0 was not reset, so the installation/recovery cost remains part of the measured Architecture Map duration.

No DevTool code was modified for the benchmark.

## 3. Architecture Map

### 3.1 Development control-plane entry

The current IDFlow development control-plane is the project-local Go devtool:

```text
go/cmd/devtool/main.go
```

Entry chain:

```text
main()
  ↓
run(os.Args[1:])
  ↓
central command dispatch
```

`run()` dispatches project development operations including:

```text
deps
code
build
test
bridge
browser
extension
capture
compile
replay
capability
demo
music
appointment
```

Pre-migration topology:

```text
developer / shell
    ↓
IDFlow go/cmd/devtool
    ↓
project paths + command dispatch
    ↓
IDFlow-specific development operations
    ↓
go / npm / Serena / CodeGraph / other tools
```

This custom IDFlow devtool is the development control-plane that DevTool V2 should replace.

### 3.2 Legacy / bootstrap boundary

The clearest legacy boundary is IDFlow's locally implemented generic development infrastructure.

#### Code intelligence

`go/cmd/devtool/code.go` currently implements:

```text
runCodeCommand
runCodeDoctor
runCodeStatus
runCodeVerify
runCodeLSP
runCodeGraph
runCodeGraphOneShot
```

`go/cmd/devtool/code_tools.go` additionally implements managed-tool bootstrap including:

```text
runCodeBootstrap
ensureManagedLSPTools
ensureManagedCodeGraph
ensureManagedTypeScriptTools
runCodeToolCommand
```

`runCodeVerify()` currently performs:

```text
ensureManagedLSPTools
    ↓
ensureManagedCodeGraph
    ↓
runCodeDoctor
    ↓
prepareCodeWorkspace
    ↓
Serena LSP health-check
    ↓
CodeGraph reindex_workspace
```

Serena/LSP/CodeGraph installation, startup, health-check, indexing and provider invocation are generic DevTool concerns and should disappear from IDFlow after migration.

#### Extension bootstrap

`go/cmd/devtool/extension.go` contains:

```text
extensionDoctor
extensionBootstrap
runExtensionCommand
```

Project semantics such as:

```text
IDFlow has a browser extension
extension source is under extension/
its build output is extension/dist
its project build is npm run build
```

belong to the IDFlow Project Extension.

Generic environment/process management should come from DevTool.

### 3.3 Current build / verify / package flow

#### Build

```text
go/cmd/devtool/main.go
    ↓
run()
    ↓
runBuild()
    ↓
runCommand(paths.GoDir, ...)
    ↓
go build ./...
```

Concrete behavior:

```go
runCommand(paths.GoDir, nil, "go", "build", "./...")
```

The meaning of the build belongs to the project. Process/environment execution is generic DevTool infrastructure.

#### Verify

Code-intelligence verification currently runs:

```text
runCodeVerify
    ↓
ensureManagedLSPTools
ensureManagedCodeGraph
    ↓
runCodeDoctor
    ↓
prepareCodeWorkspace
    ↓
Serena LSP health check
    ↓
CodeGraph reindex verification
```

This should become DevTool V2 provider/capability behavior rather than IDFlow-owned implementation.

#### Package / extension artifact

No independent top-level `package` command was found in the fixed baseline.

For the browser extension:

```text
runExtensionCommand("build")
    ↓
extensionBootstrap(...)
    ↓
npm run build
    ↓
extension/dist
```

`extension clean` deletes `extension/dist`.

### 3.4 IDFlow Project Extension ownership

The IDFlow Project Extension should own IDFlow semantics:

```text
IDFlow build
IDFlow project verification / source-boundary verification

browser-extension:
  doctor
  bootstrap prerequisites specific to this project
  build
  clean / artifact semantics

IDFlow-specific bridge/browser/capture development operations
compile/replay operations
IDFlow capability/domain development commands
appointment/music/demo development entry points
IDFlow repository/path knowledge
```

Boundary:

```text
Project Extension
    = what IDFlow means by build / verify / extension / replay / etc.

DevTool Provider
    = how commands execute, how code intelligence runs,
      how environments/runtimes are supplied
```

Product/domain implementation remains in IDFlow packages such as:

```text
go/bridge
go/capture
go/compiler
go/replay
go/workflow
go/capability
go/verticals/appointment
go/verticals/music
extension/src/...
```

### 3.5 Generic DevTool V2 Provider / Capability ownership

The following are generic:

```text
CodeGraph lifecycle
CodeGraph search/index/reindex
Serena lifecycle
LSP/gopls lifecycle
symbols/references/diagnostics
code_context composition
code doctor / generic code verification
managed code-tool installation
provider process/MCP lifecycle
environment command execution
runtime abstraction
extension loading
service/provider selection
agent-facing stable capability surface
```

Desired dependency direction:

```text
Agent
  ↓
DevTool V2 stable capabilities
  ↓
code_context
  ├─ code-indexed
  │    └─ CodeGraph
  └─ code-realtime
       └─ Serena/LSP

IDFlow Project Extension
  ↓
project-specific build / verify / extension / domain development semantics

IDFlow product packages
  ↓
actual browser/workflow/appointment/music/etc. implementation
```

### 3.6 Minimum coherent migration surface

```text
1. Make DevTool V2 the development control-plane.

2. Add/configure the IDFlow DevTool project definition.

3. Provide an IDFlow Project Extension containing only
   project-specific development operations.

4. Route build/test/extension/project operations through that extension.

5. Remove the old IDFlow-owned code-intelligence/bootstrap layer:
   - code.go generic provider orchestration
   - code_tools.go managed Serena/LSP/CodeGraph machinery
   - equivalent generic bootstrap/doctor/status/provider wrappers

6. Retire the custom go/cmd/devtool top-level control-plane once
   all required project operations are reachable through DV2.

7. Keep domain/product packages unchanged unless an actual
   project-command adapter needs a small wiring change.
```

Migration boundary:

```text
development infrastructure
          ↓ migrate to DV2

project command semantics
          ↓ expose through IDFlow Project Extension

product/domain semantics
          ↓ keep in IDFlow
```

## 4. Timing

```text
T0 = 2026-10-06T08:26:55.648112853Z
T1 = 2026-10-06T08:34:16.220255148Z
```

Architecture Map duration:

```text
440.572143 seconds
= 7m20.572s
```

This includes post-T0 provider-runtime recovery because T0 was deliberately not reset.

## 5. code_context Metrics

| Metric | Result |
|---|---:|
| Total `code_context` calls | **11** |
| Objective-only | **7** |
| Symbol/path enriched | **4** |
| Failed calls | **1** |
| Successful calls | **10** |

Call latencies:

| Call | Mode | Latency | Result |
|---|---|---:|---|
| 1 | objective-only, first cold attempt | **2.598 ms** | FAIL — CodeGraph executable absent |
| 2 | objective-only, first successful indexed cold | **318.131 ms** | OK |
| 3 | objective-only warm | 17.513 ms | OK |
| 4 | objective-only warm | 10.448 ms | OK |
| 5 | enriched, first Serena/LSP cold | **8194.100 ms** | OK |
| 6 | enriched warm | 3359.651 ms | OK |
| 7 | objective-only warm | 7.303 ms | OK |
| 8 | objective-only warm | 6.146 ms | OK |
| 9 | enriched warm | 3433.238 ms | OK |
| 10 | objective-only warm | 12.743 ms | OK |
| 11 | enriched warm | 3266.189 ms | OK |

For the ten calls after the failed first attempt:

```text
mean   = 1862.546 ms
median = 167.822 ms
min    = 6.146 ms
max    = 8194.100 ms
```

Total traced `code_context` response payload:

```text
76,636 bytes
```

## 6. Provider Metrics

### CodeGraph

First attempted provider call:

```text
1.599 ms
status = ERROR
reason = codegraph-server absent
```

First successful cold/indexed provider call:

```text
CodeGraph search = 316.738 ms
```

Successful MCP cold start:

```text
38.659 ms
```

Cold index:

```text
101 files
1604 nodes
3528 edges
graph-only mode
```

Warm CodeGraph searches:

```text
15.969 ms
8.587 ms
1.013 ms
1.088 ms
5.884 ms
4.374 ms
1.018 ms
11.520 ms
1.152 ms
```

Warm CodeGraph:

```text
mean   = 5.623 ms
median = 4.374 ms
range  = 1.013–15.969 ms
```

### Serena / LSP

MCP cold start:

```text
3178.789 ms
```

First cold operations:

```text
symbols     = 3988.184 ms
references  = 2118.423 ms
diagnostics = 2083.250 ms
```

Warm operations:

```text
symbols mean     = 205.064 ms
references mean  = 280.578 ms
diagnostics mean = 2864.082 ms
```

After startup:

```text
CodeGraph search       ≈ single-digit milliseconds
Serena symbols         ≈ 0.2 s
Serena references      ≈ 0.28 s
Serena diagnostics     ≈ 2.86 s
```

### MCP / composition

Gateway overhead over the `code_context` capability span:

```text
mean   ≈ 0.625 ms
median ≈ 0.498 ms
```

Trace events recorded:

```text
314
```

## 7. Navigation Metrics

```text
raw IDFlow file reads          = 0
grep calls on IDFlow           = 0
find calls on IDFlow           = 0
shell understanding fallback   = 0
provider-native tool calls      = 0
Sourcegraph calls               = 0
GitHub Connector understanding = 0
```

All IDFlow understanding came through:

```text
DevTool V2
  ↓
code_context
```

No `DEVTOOL_FALLBACK` was required.

## 8. Failures / Wrong Turns

### Pre-Stage setup failures

Before official T0, external bootstrap configuration encountered:

1. missing Project Extension configuration;
2. missing generic `environment` service required by CodeGraph;
3. Project Extension identity mismatch.

None caused a code-intelligence invocation or warm index.

### Stage-1 provider failure

Official cold call failed because:

```text
codegraph-server executable file not found in $PATH
```

Runtime recovery after T0 installed:

```text
CodeGraph 0.21.0
uv 0.12.23
Serena 2.0.0.dev0
```

T0 was not reset.

### Context-quality wrong turns

Three meaningful wrong turns occurred:

1. Initial broad six-question objective over-weighted browser workflow/UI matches.
2. A more verbose “developer tooling only” objective still returned too much browser/UI context.
3. One combined `runCodeVerify extensionBuild extensionClean` objective returned almost no useful detail.

These caused **3 follow-up calls**.

A shorter query:

```text
main devtool build verify package
```

was substantially more useful than the verbose objectives.

### Transport

```text
confirmed connector disconnects = 0
operation/tool timeout during setup = 1
retry after timeout = 1
reconnects = 0
```

## 9. Correctness

**Architecture correctness: PASS**

- development control-plane entry: **PASS**
- legacy/bootstrap boundary: **PASS**
- build flow: **PASS**
- verify flow: **PASS**
- package/artifact flow: **PASS**
- Project Extension ownership: **PASS**
- generic DV2 Provider ownership: **PASS**
- minimal migration surface: **PASS**
- development infrastructure vs product/domain semantics: **PASS**

Core conclusion:

```text
DevTool V2
= generic development infrastructure + stable capabilities

IDFlow Project Extension
= IDFlow-specific development semantics

IDFlow product code
= remains IDFlow product code
```

## 10. Final Comparison

```text
B baseline:
3m53s
= 233.000s

C before optimization:
7m50s
= 470.000s

C after optimization:
7m20.572s
= 440.572s
```

### C-after vs B

```text
+207.572s
= +3m27.572s
= 89.09% slower than B
```

### C-after vs C-before

```text
29.428s faster
= 6.26% improvement
```

### Final result

```text
Architecture correctness = PASS
raw IDFlow reads         = 0
grep/find fallback       = 0
shell understanding      = 0

Architecture Map Time    = 7m20.572s

TARGET <= 3m53s          = FAIL
STRETCH <= 3m00s         = FAIL
```

Benchmark validity note:

```text
STRICT BENCHMARK VALIDITY = DEGRADED
```

because the fresh Codespace was not provider-runtime-ready at official T0.

The actual C-After run therefore **did not beat the Remote Workspace baseline**. It improved from 7m50s to 7m20.572s while preserving zero raw-read/grep/find navigation churn.

Provider-level performance did improve strongly after warm-up:

```text
CodeGraph warm mean ≈ 5.6 ms
DevTool gateway overhead ≈ 0.6 ms
```

while enriched Repository Understanding remained dominated by Serena/LSP diagnostics and this run's post-T0 provider-runtime setup failure.
