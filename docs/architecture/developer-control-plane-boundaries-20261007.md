# Developer Control Plane Boundaries

Status: accepted direction for the next DevTool refactor.

Date: 2026-10-07

Implementation status: Phase 1 is implemented on `refactor/capability-boundaries-20261007`.

- parameterized Project Commands are projected to typed Agent tools;
- `scm_checkpoint` is one stable SCM service operation;
- `code_context` delegates to `service.code-context` / `context.code.composite`;
- `document_context` delegates to `service.document-context` / `context.document.composite`;
- Phase 2 environment decoupling is implemented with separate `environment` and `tooling-environment` service selections;
- provider processes are already reused inside one DevTool host lifecycle, so no second provider-cache layer is being added;
- repeated cold starts across Remote calls remain a later persistent-DevTool-endpoint problem;
- Phase 3 file+journald log sources and envelope severity normalization are implemented; a dedicated process/session source is deferred until a transport-neutral session identifier exists.

## 1. One-sentence decision

> ChatGPT owns reasoning and planning, Remote Gateway owns remote transport, DevTool owns developer capabilities, and GoTiny is an optional durable execution runtime that is inserted only when work must outlive a single request or conversation.

The default path stays short:

```text
Remote:
ChatGPT
  -> Remote Gateway
  -> DevTool

Local:
Local Agent / CLI
  -> DevTool
```

GoTiny is not part of the default path.

```text
Durable path only when needed:
ChatGPT
  -> Remote Gateway
  -> GoTiny
  -> DevTool
```

## 2. Why this decision is needed

DevTool started as a developer tool and has naturally accumulated several responsibilities:

- CLI and project commands;
- code intelligence;
- document intelligence;
- log intelligence;
- SCM workflow;
- workspace management;
- runtime/build execution;
- Agent/MCP exposure.

At the same time, Remote Desktop Commander is used as the remote entry point from ChatGPT Web, and GoTiny already provides durable run primitives such as state, retry, wait/resume, approval, budget, trace and replay.

Without an explicit boundary, the system can drift into three overlapping products:

1. DevTool becoming an Agent runtime;
2. Remote transport becoming a second developer platform;
3. GoTiny duplicating both ChatGPT reasoning and DevTool capability semantics.

The goal of this refactor is therefore not to add another layer. It is to make the existing layers orthogonal.

## 3. Evidence from the current Remote Commander workflow

A live Remote Commander session was measured before this decision.

Representative sample:

- about 1,128 remote tool calls;
- about 99.5% successful;
- `start_process` plus `read_process_output` accounted for about 74% of calls;
- most `start_process` calls were launching DevTool, Git/GitHub operations, builds/tests or text inspection;
- a large fraction of shell commands repeated workspace `cd`, profile selection, PATH and shell boilerplate;
- a roughly three-hour journald window produced multiple megabytes of logs;
- the local Node processes were not the dominant resource problem; process waits, provider startup and round trips were more important.

The main conclusion is:

> The largest cost is not Node versus Go. It is the granularity and layering of the execution path.

The current anti-pattern is effectively:

```text
ChatGPT
  -> Remote MCP
  -> start_process
  -> shell
  -> devtool agent mcp
  -> DevTool capability
  -> provider
  -> read_process_output
  -> Remote MCP
  -> ChatGPT
```

This is MCP wrapping shell wrapping another MCP lifecycle.


### 3.1 Alternatives considered during this discussion

Several alternatives were considered and deliberately not chosen as the immediate architecture:

1. **Rewrite Remote Commander from Node.js to Go.** Rejected as a first move because measured latency is dominated by process/provider work and remote round trips, while the local Node footprint is acceptable. Go may still be useful later for a thin self-hosted gateway because of single-binary deployment and ownership, not because Node is currently the main performance bottleneck.
2. **Immediately replace the hosted relay.** Deferred. Hosted-relay pricing and tool-call quotas are valid motivations for eventual self-hosting, but transport replacement should not block the DevTool boundary cleanup.
3. **Treat account/quota switching as architecture.** Rejected. Using another account can be a temporary operational workaround, but it must not become a system dependency.
4. **Put log understanding into Remote Gateway.** Rejected. Gateway should emit transport telemetry; DevTool should analyze logs.
5. **Use Loki/OpenTelemetry as the default arbitrary-log solution.** Rejected for third-party logs because those logs may not contain structured fields, trace IDs or a controlled collection pipeline. Local parsing plus optional tools such as lnav is the general first path.

The design therefore optimizes the boundaries first and keeps infrastructure replacement benchmark-driven.

## 4. Final responsibility model

### 4.1 ChatGPT: reasoning and planning

ChatGPT answers:

> What should be done, in what order, and why?

Responsibilities:

- understand the user's goal;
- reason about architecture and trade-offs;
- choose which capability to call;
- decide the next development step;
- review results and revise the plan.

ChatGPT should not own transport implementation, provider lifecycle or durable execution state.

### 4.2 Remote Gateway: remote access and transport

Remote Gateway answers:

> How does a request reach the target machine reliably?

Current implementation:

```text
Desktop Commander Remote
```

For now it remains the production Remote Gateway. It does not need a source rewrite.

Responsibilities:

- authentication;
- device registration and presence;
- remote session establishment;
- request/response transport;
- stream delivery;
- reconnect;
- timeout/cancellation transport;
- transport telemetry.

It must not own:

- code understanding;
- log root-cause analysis;
- Git workflow semantics;
- build semantics;
- provider selection;
- task planning;
- durable workflow state.

Local DevTool usage bypasses the Remote Gateway entirely.

### 4.3 DevTool: developer capability control plane

DevTool answers:

> How is a developer intent mapped to a stable capability and implemented by replaceable providers?

Examples:

- `code_context`;
- `document_context`;
- `log_context`;
- workspace operations;
- build / verify / package;
- SCM checkpoint / publish;
- runtime and project commands.

DevTool owns:

- stable developer contracts;
- capability schemas;
- service semantics;
- provider selection;
- provider readiness/lifecycle;
- environment/runtime binding;
- bounded developer context.

It must not own:

- open-ended planning loops;
- autonomous goal decomposition;
- conversation memory;
- long-lived workflow policy;
- cross-step retry budgets and approvals.

### 4.4 GoTiny: optional durable execution runtime

GoTiny answers:

> How does a run survive time, failure, waiting, approval and disconnection?

GoTiny is inserted only when the work needs durable lifecycle semantics.

It owns:

- Run and RunState;
- step state;
- wait/resume;
- retry policy at workflow/step level;
- approval/guardrail;
- budget;
- event/trace;
- replay;
- durable continuation across process or conversation boundaries.

It does not know about:

- CodeGraph;
- Serena;
- lnav;
- GitHub;
- Docker;
- Dagger;
- Gradle.

Those remain DevTool details.

## 5. Two execution paths

### 5.1 Fast path

Use for one-shot or short developer operations:

```text
ChatGPT
  -> Remote Gateway
  -> DevTool
```

Examples:

- inspect code;
- inspect logs;
- read workspace status;
- run one build;
- run one verify;
- publish one SCM operation.

GoTiny would only add another hop here.

### 5.2 Durable path

Use only when the operation needs durable state:

```text
ChatGPT
  -> Remote Gateway
  -> GoTiny
  -> DevTool
```

Examples:

- multi-hour migration;
- work that must continue after ChatGPT disconnects;
- explicit wait/resume;
- human approval;
- durable retry/budget;
- replayable benchmark or workflow;
- continuation after process/machine restart.

This keeps GoTiny valuable without making it mandatory.

## 6. Remote Gateway decision

### 6.1 Use Desktop Commander Remote now

The current Remote Commander implementation already provides the required transport baseline.

Do not rewrite it in Go now.

The measured Node process cost does not justify a rewrite for performance.

The current `zskwin` development machine already runs Remote Commander under systemd. The same operational shape should be retained on the future development server.

Immediate operational target:

```text
systemd
  -> Desktop Commander Remote
  -> restart on failure
  -> journald telemetry
```

The future development server should treat Remote Commander as infrastructure, not as a developer capability layer.

### 6.2 What may change later

Only benchmark-driven changes should be considered.

The first useful improvement is not a full rewrite. It is a thinner path between the Remote Gateway and a persistent DevTool endpoint:

```text
ChatGPT
  -> Remote Gateway
  -> persistent DevTool MCP
```

This avoids repeatedly launching:

```text
shell
  -> devtool agent mcp
  -> provider startup
```

If hosted relay pricing or platform constraints become the dominant problem, a self-hosted Remote Gateway can be built later with only:

- auth;
- device routing;
- persistent tunnel;
- request correlation;
- dispatch;
- reconnect;
- timeout/cancellation.

It must not clone Desktop Commander's full filesystem/process/productivity feature set.

## 7. Remote logs and observability

Remote Gateway should produce facts, not analyze them.

Gateway telemetry should look like:

```text
request_id
device
operation
duration_ms
request_bytes
response_bytes
status
connection/reconnect events
```

It should avoid logging full source files, complete commands, credentials or complete tool outputs.

DevTool remains the analysis layer:

```text
Remote Gateway
  -> journald / telemetry source
  -> DevTool log-analysis
  -> log_context
  -> ChatGPT
```

Future `log-analysis` sources may include:

- file;
- journald;
- process/session output;
- later Loki/remote backends.

Therefore "analyze Remote Gateway logs" is a DevTool log-intelligence use case, not a reason to put log intelligence inside the gateway.


### 7.1 Arbitrary logs and controlled observability are different problems

DevTool must support two distinct cases.

**Third-party/arbitrary logs** may be plain text, JSONL, logcat, Java stack traces, nginx output, Docker output or mixed text. They cannot assume OpenTelemetry, trace IDs, Loki labels or a known schema.

The local-first path is:

```text
raw log
  -> format recognition
  -> normalization
  -> filter / aggregate / pattern detection
  -> bounded evidence
  -> log_context
```

`lnav` is an optional enrichment tool, not a required public contract.

**Controlled systems** may later emit structured logs, metrics and traces and can use OpenTelemetry/Loki when that provides measurable value.

The public DevTool intent remains stable in both cases. Backend observability technology stays behind the log-analysis service/provider boundary.

## 8. DevTool Capability layer findings

CodeGraph plus Serena/gopls review of current `origin/main` confirms that the Agent Gateway itself is relatively clean, while some Capability extensions have started absorbing domain orchestration.

### 8.1 Agent Gateway is currently healthy

`core/agent/gateway.go` currently owns:

- MCP initialize/ping;
- tool discovery;
- tool routing;
- `tools/call`;
- transport-level tracing.

It does not contain planning loops, retries, developer workflows or provider-specific semantics.

This boundary should be preserved.

### 8.2 `capability.log` is the preferred Capability shape

`capability.log` is close to the desired pattern:

```text
Agent schema
  -> validate input
  -> one logical service call
  -> wrap stable result
```

This is the model for future Capability cleanup.

### 8.3 `capability.code` contains orchestration

Current `code_context` decides how to combine:

- indexed search;
- realtime symbols;
- references;
- diagnostics;
- parallel execution;
- cancellation;
- result bundling.

A real failure was reproduced during review:

```text
code_context(path="extensions/capability")
  -> CodeGraph accepts directory scope
  -> LSP diagnostics receives the same directory path
  -> IsADirectoryError
  -> whole code_context call fails
```

This demonstrates that provider input assumptions are leaking into Capability orchestration.

### 8.4 `capability.document` has become a stateful document engine

The document Capability contains:

- bounded review planning;
- cursor encode/decode;
- cursor signature/staleness checks;
- section traversal;
- coverage state;
- line budgets;
- relation expansion.

These semantics are useful, but they no longer belong in an Agent API adapter.

### 8.5 `capability.scm` contains a domain workflow

`scm_checkpoint` currently performs:

```text
status
  -> if dirty: commit
  -> push
  -> authorization handling
```

Checkpoint is a legitimate stable SCM intent, but the sequence should be owned by an SCM service operation rather than by the Capability adapter.

### 8.6 Project command exposure is incomplete

Current project-tool generation skips commands with parameters.

Conceptually:

```go
if len(command.Parameters) != 0 {
    continue
}
```

This creates pressure to add bespoke Agent Capabilities whenever a project operation needs arguments.

That is a structural source of future Capability growth.

## 9. Target DevTool architecture

The desired shape is:

```text
MCP / CLI / API
      |
      v
Capability
thin public adapter
      |
      v
Service
developer-domain semantics
      |
      v
Provider
replaceable implementation
```


`Service` here does **not** mean adding a new Core workflow framework. DevTool already has a service registry and provider-selection contract. The refactor should use that existing mechanism.

When a logical operation must compose lower-level services, the composition should be implemented by a normal replaceable extension/provider that exposes the higher-level service. For example:

```text
capability.code
  -> service.code-context
       provider = context.code.composite
         -> service.code-indexed
         -> service.code-realtime
```

Core still only knows service names, configured providers and extension registration. It does not gain code-context, document-context or SCM workflow special cases.

### Capability owns

- public tool name;
- JSON schema;
- basic request validation;
- Session -> request mapping;
- trace boundary;
- stable response envelope.

### Service owns

- domain semantics;
- operation composition;
- applicability rules;
- partial-failure policy;
- normalized result model.

### Provider owns

- integration with concrete technology;
- provider-specific process/API calls;
- provider lifecycle/readiness;
- provider-specific retry for transient implementation failures.

The distinction is:

> Capability defines the public intent; Service defines what that intent means; Provider defines how one implementation performs it.

## 10. Concrete refactors

### 10.1 Code Context Service

Current:

```text
capability.code
  -> code-indexed
  -> code-realtime
  -> manually compose results
```

Target:

```text
capability.code
  -> service.code-context
       -> code-indexed
       -> code-realtime
```

`service.code-context` owns:

- scope normalization;
- file versus directory applicability;
- indexed/realtime composition;
- parallelism;
- partial-failure policy;
- normalized context bundle.

Acceptance behavior:

- directory scope does not invoke file-only diagnostics;
- indexed success survives realtime failure;
- realtime success survives indexed failure when useful;
- result explicitly reports unavailable/degraded branches;
- Capability contains no CodeGraph/Serena-specific branching.

### 10.2 Document Context Service

Move document review semantics out of `capability.document` into a logical document-context service.

Target:

```text
capability.document
  -> service.document-context
       -> document-structure
       -> document-relations
```

The service owns:

- bounded review plan;
- section coverage;
- cursor semantics;
- stale-cursor detection;
- relation composition.

The Goldmark provider remains structural and provider-neutral.

The Capability becomes a thin request/response adapter.

### 10.3 SCM checkpoint service operation

Target:

```text
capability.scm
  -> service.scm.checkpoint(message)
```

The service/provider boundary owns the semantics:

```text
status
  -> commit when needed
  -> push
  -> authorization result
```

The Capability should not manually orchestrate these primitives.

### 10.4 Typed Project Commands

Complete the existing Project Command -> Agent Tool projection.

Target:

```text
CommandDescriptor.Parameters
  -> MCP JSON Schema
  -> typed argument decoding
  -> Execute(command, args)
```

This allows project operations such as:

```text
deploy(environment)
release(version)
device(target)
benchmark(group)
```

without creating another bespoke `capability.*`.

This is one of the highest-leverage ways to prevent Capability inflation.

## 11. Provider lifecycle improvements

The remote log audit exposed two different lifecycle concerns that must not be conflated.

Within one DevTool `ProjectHost` / MCP process, loaded extension processes are already retained and reused. CodeGraph and Serena therefore do not need another DevTool-side provider cache.

The repeated cold starts observed in Remote Commander logs mainly come from repeatedly launching an entirely new `devtool agent mcp` process. That belongs to the later Remote Gateway / persistent DevTool endpoint benchmark.

The DevTool-side problem that **does** require a change is environment coupling. Code intelligence should not require the project Docker environment merely because build/runtime uses Docker.

Implemented wiring:

```text
project build / verify / package
  -> service.environment
  -> configured environment provider

CodeGraph / Serena / LSP
  -> service.tooling-environment
  -> configured tooling environment provider
```

Both services use the same Environment Contract and remain provider-replaceable. A deployment may select a different provider for either service through configuration.

This separates "understand the code" from "build in an isolated reproducible environment" without duplicating lifecycle management.

## 12. Log Intelligence follow-up

`log_context` is now a real DevTool capability and should remain in DevTool.

Nested/envelope logs are now normalized in the local journald source.

A Remote Commander journal event may contain another tool's output, which may itself contain an error log.

Implemented normalized model:

```text
NormalizedEvent
  outer:
    timestamp
    source
    level
  embedded:
    source?
    level?
    message
```

This allows separate questions:

- Did Remote Gateway itself fail?
- Did a command executed through Remote Gateway fail?
- Did a nested provider such as Serena or Docker fail?

The fix is a structured source/parser model, not more global severity regexes.

## 13. Markdown and document intelligence

The currently merged Markdown path is:

```text
document_context
  -> document-structure
  -> document.markdown.goldmark
```

It already provides:

- frontmatter parsing;
- heading tree;
- exact source ranges;
- bounded section retrieval;
- bounded whole-document review;
- related-context support through a separate relation service.

This is structural document intelligence, not an LSP.

Marksman/Markdown LSP remains a possible realtime enrichment provider for:

- links/references;
- diagnostics;
- workspace-aware Markdown relationships.

Current validation on 2026-10-07 confirms that this is **not yet a merged provider**. The configured Serena realtime path starts `gopls` for the DevTool repository; running `code_context` against this Markdown ADR returned empty realtime diagnostics and did not start a Markdown language server. That result must not be represented as Markdown-LSP validation.

The merged writing/document path that was actually used to review this ADR is:

```text
document_context(review=true)
  -> document-structure
  -> document.markdown.goldmark
```

It produced a deterministic 25-unit bounded traversal with complete coverage and no oversized unit at a 120-line budget.

A future Marksman/Markdown-LSP provider should be added only behind a stable document-realtime service if measurement shows that Goldmark plus relation providers leave a real gap.

Do not expose Marksman methods directly to the Agent.

## 14. Retry ownership

Retry must remain layered.

### Provider retry

Provider-local transient reliability:

```text
HTTP 503
  -> retry same provider operation

temporary language-server startup failure
  -> limited provider retry
```

This belongs to DevTool Provider implementation.

### Durable workflow retry

Workflow-level behavior:

```text
verify failed
  -> later retry step

wait for approval
  -> resume

retry until budget exhausted
```

This belongs to GoTiny when the durable path is used.

DevTool must not recreate GoTiny's Run/Retry state machine.

## 15. Run identity and process identity

Do not create a second durable Run model inside DevTool.

Fast-path DevTool requests need only transport/request correlation.

For durable work:

```text
GoTiny run_id
  -> step
  -> DevTool capability call
```

Process PIDs are implementation details and may disappear.

Remote transport may keep a request/session ID, but durable workflow identity belongs to GoTiny.

## 16. When GoTiny should be introduced

GoTiny should remain outside the default path until at least one real requirement exists:

- execution must continue after the controlling ChatGPT session disconnects;
- execution spans hours/days;
- explicit wait/resume is required;
- an approval checkpoint must survive process restart;
- retry/budget policy must survive restarts;
- replay is required for a formal benchmark or production workflow.

If none of these apply, use the fast path.

This rule prevents GoTiny from becoming an unnecessary Agent-between-Agents layer.

## 17. Problem closure matrix

This refactor must distinguish between problems it **directly closes**, problems it **enables a clean fix for**, and problems it **deliberately defers**.

| Observed problem | Owner | This refactor does | Closure |
| --- | --- | --- | --- |
| Directory path passed into file-only LSP diagnostics breaks `code_context` | DevTool | Move applicability and composition into `service.code-context`; file-only diagnostics are skipped for directory scope | Directly close |
| One code-intelligence branch failure destroys all useful context | DevTool | Add explicit partial/degraded result semantics in the code-context service | Directly close |
| `capability.code` contains indexed/realtime orchestration | DevTool | Move composition behind one logical service contract | Directly close |
| `capability.document` contains review cursor/state/planning | DevTool | Move bounded-review semantics behind `service.document-context` | Directly close |
| `scm_checkpoint` manually performs status/commit/push in Capability | DevTool | Add checkpoint as a stable SCM service operation | Directly close |
| Parameterized Project Commands disappear from Agent surface | DevTool | Generate MCP JSON Schema and dispatch typed arguments | Directly close |
| Code intelligence depends on Docker even when isolation is unnecessary | DevTool | Bind intelligence providers to local environment by default; retain Docker/Dagger for reproducible execution | Directly close |
| Arbitrary logs require ad-hoc reads and nested severity causes false attribution | DevTool | Add source adapters and envelope/nested event normalization behind log-analysis | Directly close in log phases |
| CodeGraph / Serena appear to cold-start across Remote calls | Remote Gateway + persistent DevTool endpoint | DevTool already reuses providers within one host; benchmark reusing the whole DevTool endpoint across Remote calls | Enabled, not a Provider-cache change |
| Remote transport logs full source/commands/tool output | Remote Gateway operation/config | Move toward metadata-first telemetry; do not move analysis into gateway | Requires gateway logging/config support |
| `start_process -> read_process_output` creates many paid remote calls | Remote Gateway + persistent DevTool endpoint | Benchmark a persistent DevTool MCP path and remove shell/MCP nesting if it materially helps | Enabled, not automatically closed by Capability refactor |
| Hosted Remote Commander relay is metered/paid | Remote Gateway | Keep current gateway now; self-host only if pricing/platform constraints justify it | Explicitly deferred |
| Switching account avoids quota temporarily | Operations | Treat only as a temporary workaround | Not architecture |
| Long tasks must survive ChatGPT disconnect/restart | GoTiny | Insert GoTiny only on the durable path | Deferred until a real durable workflow requires it |
| PID/session disappears during a long durable run | GoTiny / transport | Durable run identity belongs to GoTiny; transport PID remains implementation detail | Closed only when durable path is adopted |
| Markdown document needs structural bounded review | DevTool Document Intelligence | Already provided by `document_context + Goldmark` | Already closed |
| Markdown links/references need true realtime LSP diagnostics | Future document-realtime provider | Add Marksman/Markdown LSP only if measured need remains | Explicitly deferred |

The important consequence is:

> The Capability refactor does not claim to solve hosted-relay pricing or every remote round trip. It removes the architectural coupling that currently makes those problems harder to optimize independently.

## 18. Refactor order

### Phase 0 — freeze boundaries

- keep `core/agent/gateway.go` transport/routing-only;
- keep Remote Commander unchanged except operational deployment/telemetry;
- do not integrate GoTiny into the default path.

### Phase 1 — stop Capability inflation

1. complete typed Project Command argument exposure;
2. move `scm_checkpoint` semantics behind the SCM service;
3. introduce `service.code-context`;
4. introduce `service.document-context`;
5. use `capability.log` as the thin-adapter reference shape.

### Phase 2 — decouple tooling from project environment

- keep the existing extension-process reuse inside one DevTool host lifecycle;
- add `tooling-environment` as a separate stable Environment service selection;
- run CodeGraph/Serena through `tooling-environment`;
- select the tooling provider by profile rather than hard-coding local execution;
- keep the default/CI profile on the shared DevEnvironment container and provide an explicit `local` profile for host-installed tooling;
- retain Docker/Dagger for reproducible project execution where selected;
- treat repeated whole-DV2 cold starts as a later Remote Gateway optimization, not a Provider cache problem.

### Phase 3 — broaden log sources

- add file + journald source adapters;
- add envelope/nested event normalization;
- preserve bounded/redacted evidence;
- defer a dedicated process/session source until DevTool has a stable transport-neutral session identifier rather than depending on Remote Commander's private PID/session model.

### Phase 4 — optimize remote path only after measurement

- benchmark persistent DevTool MCP behind Remote Gateway;
- only then consider a thin self-hosted gateway if pricing/platform constraints justify it.

### Phase 5 — add GoTiny only for durable workflows

- expose DevTool capability calls as GoTiny tools;
- keep DV2 contracts unchanged;
- keep ChatGPT as the default reasoner/planner unless a separate autonomous-agent goal is explicitly chosen.

## 19. Acceptance criteria

The refactor is complete only when all of the following hold:

1. Capability handlers do not implement multi-provider workflow orchestration.
2. `code_context(path=<directory>)` does not fail because diagnostics require a file.
3. Code context can return useful partial results when one intelligence provider fails.
4. Within one DevTool host lifecycle, CodeGraph and Serena/LSP processes are reused rather than restarted for every capability call.
5. Document review state/plan logic is outside the Capability adapter.
6. SCM checkpoint composition is outside the Capability adapter.
7. Parameterized Project Commands can become Agent tools without bespoke Capabilities.
8. `log_context` can consume at least file and journald sources without moving analysis into Remote Gateway.
9. Gateway telemetry is metadata-first and does not routinely log full source/tool payloads.
10. Local code intelligence does not require Docker unless explicitly configured.
11. Remote Gateway remains transport-only.
12. GoTiny remains absent from the fast path.
13. No Core special case is added for a specific project, provider, log source or remote device.

## 20. Non-goals

This refactor does not attempt to:

- build a new general-purpose Agent platform inside DevTool;
- rewrite Desktop Commander;
- replace ChatGPT reasoning with GoTiny;
- put log intelligence inside Remote Gateway;
- expose provider-native CodeGraph/LSP/Goldmark/Marksman APIs to Agents;
- make every operation durable by default;
- add another workflow engine.

## 21. Final architecture

```text
                         ChatGPT
                  reasoning / planning
                          |
                 Remote Gateway
          auth / device / session / transport
                          |
              +-----------+-----------+
              |                       |
              | fast path             | durable path
              |                       v
              |                    GoTiny
              |             run / wait / resume
              |             retry / approval / replay
              |                       |
              +-----------+-----------+
                          |
                          v
                        DevTool
              Developer Capability Plane
                          |
          +---------------+---------------+
          |               |               |
          v               v               v
       Context          Runtime           SCM
   code/document/log   build/verify    workspace/publish
          |               |               |
          v               v               v
     Providers        Providers        Providers
 CodeGraph/Serena    Dagger/Docker      Git/GitHub
 Goldmark/lnav
```

The guiding test for future features is:

> Is this about how one developer capability works, how a run survives over time, or how a request reaches a machine?

- one developer capability -> DevTool;
- run lifecycle -> GoTiny;
- remote connectivity -> Remote Gateway;
- reasoning/planning -> ChatGPT.

If a feature does not fit one of these boundaries cleanly, stop and review the architecture before adding it.
