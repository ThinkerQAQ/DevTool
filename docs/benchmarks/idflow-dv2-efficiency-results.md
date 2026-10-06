# IDFlow × DevTool V2 Efficiency Benchmark — A/B/C Results

## Status

This document records the first completed IDFlow × DevTool V2 efficiency benchmark.

The experiment compares three development paths from the same fixed IDFlow baseline.

A — GitHub Connector baseline

~~~text
ChatGPT
  -> GitHub Connector
  -> GitHub repository APIs
~~~

B — Remote Connector baseline

~~~text
ChatGPT
  -> Remote Connector
  -> GitHub Codespaces 4C16G
  -> shell / filesystem / git / build tools
~~~

C — Remote Connector + DevTool V2

~~~text
ChatGPT
  -> Remote Connector
  -> GitHub Codespaces 4C16G
  -> DevTool V2
     -> code_context
        -> CodeGraph
        -> Serena / LSP
     -> project_*
     -> scm_*
~~~

The experiment separates three questions:

~~~text
B vs A
= value of a real remote development workspace

C vs B
= incremental value of DevTool V2 + semantic code intelligence

C vs A
= value of the final target architecture
~~~

---

## 1. Fixed baselines

IDFlow:

~~~text
ThinkerQAQ/IDFlow
3a652d74f7c4e34ab29343c33042e548cee248a8
~~~

DevTool V2:

~~~text
ThinkerQAQ/DevTool
96f78f7b937d17f52a5ecdccc9164c8611dfd922
~~~

Remote machine for B/C:

~~~text
GitHub Codespaces
4 vCPU
16 GB RAM
0 swap
~~~

A/B/C used isolated benchmark branches and were instructed not to reuse the existing IDFlow DevTool migration implementation.

---

## 2. Executive result

### Remote workspace value: validated

Moving from GitHub-Connector-only development to a real remote workspace materially improved repository exploration, executable feedback, local validation, baseline-vs-regression diagnosis, and the ability to reach a genuinely validated merge-ready state.

Group A reached code-complete but could not reach FIRST_GREEN or MERGE_READY.

Group B reached MERGE_READY.

The largest proven gain from A -> B is not raw typing speed. It is the transition from:

~~~text
looks correct
~~~

to:

~~~text
runs, validates, packages, and is proven correct
~~~

### DevTool control-plane value: validated

Group C successfully used stable DevTool capabilities as the Agent control plane:

~~~text
code_context
project_build
project_verify
project_package
scm_checkpoint
~~~

The Agent did not depend on provider-native CodeGraph, Serena, gopls or Sourcegraph APIs.

This validates the architectural abstraction:

~~~text
Agent
  -> stable capability
  -> replaceable provider
~~~

### DevTool code-understanding latency advantage: not validated

Group C did not beat Group B on Repository Understanding time.

Observed Stage 1:

~~~text
Group B: 3m53s
Group C: 7m50s
~~~

Group C was approximately 2.0x slower on this specific Stage 1 measurement.

At the same time, Group C reduced low-level navigation substantially:

~~~text
effective code_context calls: 7
raw IDFlow file reads:       0
grep/find calls:             0
shell understanding fallback:0
~~~

Therefore the abstraction is working, but the current code_context path has not yet demonstrated lower time-to-useful-context.

---

## 3. Group A — GitHub Connector baseline

Branch:

~~~text
benchmark/group-a-github-connector-20261005
~~~

Fixed-base validation branch:

~~~text
benchmark/group-a-base-3a652d74
~~~

Result:

~~~text
code-complete          YES
structurally audited   YES
CI-verified            NO
MERGE_READY            NO
~~~

Timing:

~~~text
T0                  21:52 +08
Architecture map    exact timestamp unavailable
                    upper bound ~11m
First valid patch   ~11m
Code-complete       ~27m
FIRST_GREEN         unavailable
MERGE_READY         not reached
~~~

Group A could modify the repository through GitHub APIs, but had no real workspace.

The experiment could not actually prove:

- DevTool config validation;
- Project Extension compilation;
- Go tests/build;
- extension build;
- appointment acceptance;
- DevTool build/verify/package;
- runtime SCM/Credential wiring.

GitHub Actions accepted the workflow runs, but the hosted runner never executed the first step.

This is a legitimate Group A result, not a code failure.

Understanding a fixed historical SHA required repository API operations such as:

~~~text
commit
  -> recursive Git tree
  -> exact-ref file reads
~~~

Stage 1 alone recorded 44 fetch attempts, 39 successful reads and 5 incorrect-path reads.

---

## 4. Group B — Remote Connector baseline

Branch:

~~~text
benchmark/group-b-remote-connector-20261005
~~~

Final HEAD:

~~~text
c085f14cdeff8f0d6e30b39292845ccc2a956c78
~~~

Result:

~~~text
MERGE_READY = YES
~~~

The branch remained unmerged after the experiment.

Timing:

~~~text
T0                   14:32:35.957Z
Architecture map     3m53s
First valid patch    12m10s
First 5 commits      ~35m54s
FIRST_GREEN          ~02:20:44.192Z
MERGE_READY          ~02:30:52.728Z
~~~

Calendar wall-clock was roughly 11h58m because the run included an overnight pause.

Two visible active tool windows totaled approximately 1h47m, but exact active engineering time is not reliably observable. The 1h47m estimate must not be treated as a precise KPI.

The final committed HEAD passed:

~~~text
devtool config validate
devtool project inspect
devtool build
devtool verify
devtool package

go test ./...
go build ./...
extension TypeScript build
DevControl module test
appointment acceptance demo
~~~

Representative DevTool command durations:

~~~text
config validate   5 ms
project inspect   120 ms
build             2.777 s
verify            3.017 s
package           4.262 s
~~~

The remote workspace exposed failures that Group A could not execute far enough to observe, including missing Go module metadata, a migration-caused CLI name collision, private dependency authentication, Docker/host credential isolation, and baseline Go/TypeScript defects.

The main tail-latency sources were:

~~~text
Remote Connector stability
GitHub credential propagation
private dependency authorization
Docker credential isolation
Codespaces suspend/resume
full real validation
~~~

The dominant late-stage cost was not source-code navigation.

---

## 5. Group C — Remote Connector + DevTool V2

Branch:

~~~text
benchmark/group-c-devtool-v2-20261006
~~~

Final HEAD:

~~~text
69714eeb27e150188fd8db2a57e2d5d8880b6a1b
~~~

Result:

~~~text
MERGE_READY = YES
~~~

The migration was completed, validated, committed and pushed.

Fresh independent Codespace:

~~~text
hostname: codespaces-892996
4 vCPU
15 GiB RAM
0 swap
~~~

The experiment intentionally did not reuse the Group B workspace or CodeGraph index.

Timing:

~~~text
T0                      03:10:52Z
ARCHITECTURE_MAP_READY  03:18:42Z
Stage 1                 ~7m50s
Final completion        ~04:21:31Z
Total wall-clock        ~1h10m39s
~~~

The wall-clock includes Remote Connector interruption/reconnection, GitHub Device Authorization, OAuth workflow-scope authorization, dependency downloads, provider startup, validation, and package generation.

It is therefore not equivalent to pure Agent execution time.

Stage 1 code-intelligence use:

~~~text
effective code_context calls          7
initial failed setup/config attempts  3
objective-only calls                  2
path/symbol-enriched calls            5
raw IDFlow file reads                 0
grep/find calls                       0
shell understanding fallbacks         0
~~~

Indexed and realtime providers were both used.

Exact internal CodeGraph/Serena/gopls call counts were not reliably observable.

Whole-run stable capability usage:

~~~text
code_context       17
project_build       7
project_verify      2
project_package     1
scm_checkpoint      5
~~~

This is important architecture evidence: the Agent used the stable capability surface rather than provider-native APIs.

---

## 6. A/B/C comparison

| Metric | A — GitHub Connector | B — Remote Workspace | C — Remote + DV2 |
| --- | ---: | ---: | ---: |
| Architecture map | <=11m | **3m53s** | 7m50s |
| First valid patch | ~11m | 12m10s | not reliably available in current report |
| Low-level file navigation | high | medium | **very low in Stage 1** |
| Semantic code intelligence | no | no | **yes** |
| Real workspace | no | **yes** | **yes** |
| Local execution | no | **yes** | **yes** |
| Stable Agent capability layer | no | no | **yes** |
| FIRST_GREEN | no | **yes** | **yes** |
| MERGE_READY | no | **yes** | **yes** |
| Provider-native API dependency | n/a | n/a | **no** |

The table intentionally does not claim a precise B-vs-C total speedup because the active-time measurement methodology was not equivalent.

---

## 7. Interpretation

### 7.1 A -> B: the strongest proven improvement

The strongest result in the benchmark is the move from GitHub Connector to a real remote workspace.

Group A could perform a real migration and static audit, but could not close the validation loop.

Group B could:

~~~text
edit
  -> compile
  -> test
  -> diagnose
  -> repair
  -> verify
  -> package
  -> prove merge-ready
~~~

Therefore a persistent remote development node has clear value even before adding DV2 semantic intelligence.

### 7.2 B -> C: abstraction improved more than latency

Group C proved that the DV2 abstraction works:

~~~text
code understanding -> code_context
project actions    -> project_*
SCM                -> scm_*
~~~

The Agent performed Repository Understanding without raw file reads or grep/find fallbacks during Stage 1.

However:

~~~text
B Stage 1 = 3m53s
C Stage 1 = 7m50s
~~~

So the first benchmark does not support the claim that current DV2 semantic intelligence is faster than ordinary workspace search for this task.

The result is:

~~~text
navigation/tool churn     improved
abstraction quality       improved
time-to-useful-context    regressed
~~~

### 7.3 Fewer calls are not enough

C demonstrates an important distinction:

~~~text
fewer Agent-visible operations
!=
lower wall-clock time
~~~

The remaining latency may come from some combination of provider cold startup, first CodeGraph index/search, Serena/LSP startup, sequential indexed/realtime composition, objective-to-symbol-search effectiveness, Remote Connector call overhead, and setup/configuration failures.

This must be measured rather than assumed.

---

## 8. Benchmark validity and caveats

### Group A

Valid.

The inability to reach green is part of the GitHub-Connector-only capability boundary.

### Group B

Valid with minor protocol deviations.

Recorded deviations:

- python3 used a few times as a text-editing helper;
- gh used for authentication/Git credential integration.

Neither was used as semantic code intelligence or as an alternate primary development path.

### Group C

The report states that Repository Understanding was performed primarily through code_context, with zero initial raw file reads/grep/find fallbacks.

One detail should remain explicitly checked in the archived Group C report:

> whether the three initial setup/configuration attempts modified the IDFlow worktree before ARCHITECTURE_MAP_READY.

If those attempts only touched external runtime/temporary configuration, there is no protocol issue.

If they changed IDFlow files before Stage 1 completed, record that as a protocol deviation. It does not automatically invalidate the full C run, but it affects Stage 1 purity.

---

## 9. Hardware conclusion so far

The benchmark establishes a strong case for using a persistent remote development machine.

It does not yet prove that 4C4G is sufficient.

B/C were run on 4C16G.

The next hardware-specific experiment should run representative Group C workloads under approximately:

~~~text
4 vCPU
4 GB RAM
~~~

and record peak RSS, swap, OOM, CodeGraph/LSP startup, warm code_context latency, and build/verify/package latency.

The server purchase decision should use that result rather than extrapolating from 4C16G.

---

## 10. Product decision

Current evidence supports:

> Use a persistent remote workspace as part of the development architecture.

Current evidence also supports:

> Keep DevTool V2 as the stable Agent control plane.

Current evidence does not yet support:

> CodeGraph/LSP through the current code_context implementation is already faster than ordinary repository search.

This distinction is important.

The architectural direction is useful; the current semantic-context execution path still has a measurable performance problem.

---

## 11. Next optimization target

Do not add more capabilities before addressing the measured bottleneck.

The next optimization target is:

> **Time-to-Useful-Context for warm code_context.**

Investigate in this order:

~~~text
1. provider cold/warm startup cost
2. CodeGraph index/search latency
3. Serena/LSP startup and request latency
4. sequential indexed + realtime execution
5. objective-only query quality
6. Remote Connector transport overhead
~~~

Do not optimize all layers simultaneously.

Change one causal layer at a time and repeat the same Repository Understanding sub-benchmark.

---

## 12. Next benchmark

Freeze the current A/B/C branches.

Do not rewrite their history and do not use them as implementation input for the next run.

After DV2 performance work, rerun only the smallest useful controlled comparison first:

~~~text
B-understanding
vs
C-understanding-before-optimization
vs
C-understanding-after-optimization
~~~

Primary metric:

~~~text
Time-to-Useful-Context
~~~

Secondary metrics:

~~~text
architecture-map time
code_context calls
raw reads
grep/find fallbacks
provider latency
connector latency
wrong turns
~~~

Only after semantic-context latency is improved should the full migration benchmark be repeated.

---

## 13. Current decision summary

~~~text
GitHub Connector only
        |
        |  insufficient execution/validation closure
        v
Remote Workspace
        |
        |  clearly validated improvement
        v
DevTool V2 stable control plane
        |
        |  architecture validated
        |  semantic latency not yet validated
        v
Optimize code_context Time-to-Useful-Context
        |
        v
4C4G hardware stress test
        |
        v
cloud machine purchase decision
~~~
