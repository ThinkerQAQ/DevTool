# Log Intelligence and Remote Transport

Status: accepted direction, implementation starts with local arbitrary-log intelligence.

## 1. Problem definition

DevTool has two related but separate infrastructure problems:

1. **Arbitrary log understanding** — an agent may receive logs produced by a system it does not control. The input may be plain text, JSON, logfmt, Android logcat, Java stack traces, nginx logs, Docker output, or a mixed file.
2. **ChatGPT Web to local development transport** — ChatGPT Web currently reaches a local machine through Remote Desktop Commander. The local component is lightweight enough, but the hosted relay is a metered external dependency.

These problems must remain separate in the architecture. Log understanding is an intelligence capability. Remote access is transport.

## 2. Measured Remote Commander baseline

A live zskwin session running Desktop Commander 0.2.52 was measured before this design was written.

Observed process cost:

- remote process RSS: about 136 MB
- local MCP server RSS: about 163 MB
- combined idle CPU: about 0.2%

Observed tool traffic over roughly 700 calls:

- start_process: about 360 calls
- read_process_output: about 225 calls
- those two operations account for about 85% of calls
- local file/edit operations usually complete in single-digit to tens of milliseconds
- process-start/output waits dominate latency, with a P95 close to the remote wait cap

Conclusion:

> Rewriting the local Node implementation in Go is not justified by runtime performance. The higher-value optimization is fewer remote round trips and a thinner, self-hostable transport.

The hosted relay being metered is a separate architectural motivation. DevTool should not depend permanently on a per-tool-call third-party relay for its primary local-development path.

## 3. Remote transport direction

Do **not** clone Desktop Commander's complete feature set.

Target architecture:

~~~
ChatGPT Web
    |
    v
DevTool Plugin / App
    |
    v
DevTool Remote Gateway
    |
    v
outbound persistent connection
    |
    v
Local DevTool Agent
    |
    +-- code_context
    +-- log_context
    +-- build / verify
    +-- scm_publish
    +-- runtime capabilities
    +-- minimal exec fallback
~~~

The remote layer owns only authentication, device registration, presence, request correlation, dispatch, response delivery, reconnect and timeout/cancellation.

It must not become a second development platform or a proxy that duplicates DevTool's capability layer.

Short-term operation may continue to use Remote Desktop Commander. Account/quota workarounds are operational concerns only; they are not part of the architecture and must not become a dependency.

## 4. Arbitrary logs are not an LSP problem

LSP works because source code has stable semantic concepts such as symbols, definitions, references, types, diagnostics and call hierarchy.

An arbitrary log stream instead has timestamps, levels, messages, sources, requests/operations, durations, errors, repeated patterns and optional correlation identifiers.

There is no universal "Log LSP" with equivalent semantics.

The useful abstraction is:

~~~
raw logs
   |
format recognition
   |
normalization
   |
filter / aggregate / pattern detection
   |
bounded evidence
   |
log_context
~~~

The agent should receive a compact evidence bundle, not an entire multi-megabyte file.

## 5. Two different log scenarios

### 5.1 Third-party / arbitrary logs

This is the first-class MVP.

Inputs may be files produced by systems DevTool does not control. Therefore the implementation must **not** assume OpenTelemetry, trace IDs, Loki labels, structured JSON, a predefined schema, or a running Docker stack.

Preferred local strategy:

1. inspect the file directly;
2. use lnav when available for recognized formats and structured querying;
3. use deterministic local parsing/fallback when lnav is unavailable or the format is unknown;
4. return bounded summaries, error clusters/patterns and representative lines.

### 5.2 Systems we control

For DevTool and applications that we control, richer observability can be added later:

~~~
structured logs
    + metrics
    + traces
       |
OpenTelemetry
       |
Loki / trace backend
~~~

This is a later provider, not a prerequisite for log_context.

OpenTelemetry cannot reconstruct trace relationships that were never present in a third-party log, so it must not be the default arbitrary-log path.

## 6. Capability model

The stable agent-facing intent is log_context.

Input:

~~~
objective

# legacy file shorthand
path?

# structured source
source?:
  kind = file | journald
  path?        # file
  unit?        # journald
  scope?       # user | system
  since?       # RFC3339
  until?       # RFC3339
  max_entries?

query?         # optional literal hint
limit?         # bounded evidence count
~~~

Exactly one of `path` or `source` is supplied. The legacy `path` form remains equivalent to `source={kind:"file", path:"..."}`.

Expected output shape:

~~~
objective
source
format
summary
levels
patterns
matches
evidence
provider
~~~

The exact provider-specific mechanics remain hidden.

Service boundary:

~~~
capability.log
    |
    v
service.log-analysis
    |
    +-- intelligence.log.local      # MVP
    +-- intelligence.log.lnav       # optional specialization
    +-- intelligence.log.loki       # future
~~~

The MVP may combine direct parsing with opportunistic lnav use inside a single local provider, but the public contract must not expose lnav commands.

## 7. Local provider requirements

intelligence.log.local must:

- work on an ordinary local file without Docker;
- read local journald units without moving log analysis into Remote Gateway;
- keep journald outer severity (`PRIORITY`) separate from severity detected inside `MESSAGE`;
- constrain paths to the project/workspace boundary unless explicitly allowed by a future policy;
- bound bytes/lines returned to the agent;
- recognize common levels (trace, debug, info, warn, error, fatal, panic);
- preserve multiline exception/stack-trace context when selecting evidence;
- report total lines/bytes and truncation;
- support a query hint;
- surface representative repeated error patterns;
- identify whether lnav was used;
- avoid logging full input contents into DevTool's own normal logs.

It should degrade gracefully:

~~~
lnav available + recognizes input
    -> use it where it improves parsing/querying

otherwise
    -> deterministic Go fallback
~~~

## 8. Security and privacy

Remote Commander journald currently records complete tool arguments and outputs, including commands and file contents. The same mistake must not be repeated in DevTool log intelligence.

Rules:

- traces record metadata and sizes, not complete source logs;
- redact obvious credentials/tokens from returned evidence when feasible;
- do not persist imported logs unless explicitly requested;
- do not upload logs to an external service in the local provider;
- future cloud providers require explicit configuration.

## 9. Delivery stages

### Stage L0 — local arbitrary-log context

- add sdk/logintelligence
- add service.log-analysis
- add intelligence.log.local
- add capability.log
- expose log_context
- wire through .devtool.toml
- add deterministic unit tests and one fixture-based acceptance test

### Stage L1 — lnav enrichment

Implemented in the local provider:

- detect lnav from PATH and use it opportunistically in headless mode;
- use lnav's format recognition to enrich the response (`lnav_used=true`);
- keep deterministic Go parsing as the fallback and as the bounded evidence path;
- cap each returned evidence/sample item so a single huge journal message cannot flood agent context;
- only treat severity words near the log prefix as levels, avoiding false positives from embedded source code and command payloads.

Remaining work is benchmark-driven: add deeper lnav query enrichment only when it materially improves arbitrary-log analysis.

### Stage L2 — journald source and envelope semantics

Implemented in the local provider:

- structured `source.kind=journald` with user/system scope and bounded entry count;
- `PRIORITY` is normalized as the outer event severity;
- severity words inside `MESSAGE` are reported separately as embedded severity;
- evidence contains normalized event time/source/outer level/embedded level;
- the existing file source remains backward compatible through `path`.

A dedicated process/session source remains deferred until DevTool has a stable transport-neutral session identifier to consume. It should not depend on Remote Commander's private PID/session representation.

### Stage L3 — controlled-system observability

Only when there is a real multi-service need: structured DevTool logs, correlation/trace IDs, OpenTelemetry exporter/provider and a Loki/LogQL provider.

### Stage R0 — remote transport spike

Separately from log intelligence:

- verify ChatGPT Web Plugin/App write path
- prototype a minimal self-hosted relay
- local outbound device connection
- expose DevTool capabilities, not Desktop Commander clones

No R0 work is required for L0.

## 10. Architecture tests

The feature is complete only if:

- Core contains no log-provider special case;
- agent tool registration comes from capability.log;
- provider selection is configuration-driven;
- replacing intelligence.log.local does not change the agent contract;
- log_context works when Docker is unavailable;
- an arbitrary fixture log can be summarized without Loki or OpenTelemetry.
