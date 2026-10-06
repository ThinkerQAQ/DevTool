# CLI Reference

This reference documents the current stable DevTool CLI implemented by `cmd/devtool`.

Project-specific commands are discovered dynamically from the configured Project Extension and therefore are not hard-coded into Core.

## Core commands

### `devtool project inspect`

Inspect the resolved project and Project Extension.

```bash
devtool project inspect
devtool project inspect --json
```

JSON output includes the project name/root/config path, resolved Project Extension, descriptor, service bindings and UI configuration.

Use this before assuming what a project can do.

---

### `devtool config path`

Print the discovered `.devtool.toml`.

```bash
devtool config path
```

---

### `devtool config validate`

Validate the discovered configuration after applying active profiles.

```bash
devtool config validate
```

Validation currently checks:

- config version;
- `project.name`;
- extension names and loaders;
- service names and provider IDs;
- code workspaces;
- UI feature names.

---

### `devtool code doctor`

Check whether the configured indexed and realtime code-intelligence providers are ready.

```bash
devtool code doctor
```

The output reports the resolved provider/executable and version when available.

---

### `devtool code verify`

Run verification through both configured code-intelligence services.

```bash
devtool code verify
```

A successful result reports:

```text
Code intelligence VERIFIED
- Indexed: PASS
- Realtime: PASS
```

---

### `devtool agent mcp`

Run the Agent Gateway over stdio MCP.

```bash
devtool agent mcp
devtool agent mcp --context codex
```

Options:

- `--context <name>`: operation context exposed to Agent Capability providers. Default: `agent`.

The gateway dynamically exposes tools contributed by configured extensions. Provider-native tool names are not the stable public contract.

---

### `devtool agent serve`

Run the Agent Gateway over HTTP.

```bash
DEVTOOL_AGENT_TOKEN=<secret> \
devtool agent serve --listen :8080 --context agent
```

Options:

- `--listen <addr>`: listen address. Default is `:$PORT` when `PORT` is set, otherwise `:8080`.
- `--context <name>`: agent operation context.
- `--token-env <name>`: environment variable containing the bearer token. Default: `DEVTOOL_AGENT_TOKEN`.
- `--allow-unauthenticated`: disable bearer-token requirement. Use only in trusted local environments.

Without `--allow-unauthenticated`, the configured token environment variable must contain a non-empty token.

---

## Structured tracing

DevTool can emit generic timing spans for Agent Gateway operations and the
`code_context` capability path. Tracing is disabled unless a trace file is
explicitly set for the process:

```bash
DEVTOOL_TRACE_FILE=.devtool/traces/benchmark.jsonl \
DEVTOOL_RUN_ID=<run-id> \
devtool agent mcp --context codex
```

`DEVTOOL_TRACE_FILE` selects an append-only JSON Lines file. Relative paths are
resolved from the DevTool process working directory. `DEVTOOL_RUN_ID` is an
optional correlation label written to every span; it does not enable tracing by
itself.

Each line records span identifiers, parent linkage, layer, operation,
provider/service identity when available, duration, status, request/response
byte counts, run ID and timestamp. Trace context follows the existing extension
process protocol, so one `tools/call` can be reconstructed across the capability,
service and provider processes.

Trace output is metadata-only. It does not include source content, prompts,
tool argument bodies, authorization headers, tokens, credentials, secrets or
secret-bearing environment values. Protect the trace file as operational data
and do not commit benchmark traces before reviewing them.

---

## Project commands

Any first argument not recognized as a Core command is resolved through the configured Project Extension.

For DevTool itself, the current Project Extension declares:

### `devtool build`

Build DevTool N+1 in the configured environment.

Output directory:

```text
.devtool/out/
```

---

### `devtool verify`

Run DevTool self-host verification in the configured environment.

The current DevTool Project Extension verifies:

- root Go tests;
- `devcontrol` Go tests;
- N+1 build;
- N+1 project inspection.

Repository CI adds broader integration/self-host checks around this command.

---

### `devtool package`

Verify first, then build a packaged platform binary.

Output directory:

```text
.devtool/artifacts/
```

---

## Stage 0 from source

Inside the DevTool repository, commands can be bootstrapped with:

```bash
go run ./cmd/devtool <command>
```

Example:

```bash
go run ./cmd/devtool project inspect --json
```

This is DevTool bootstrapping itself, not an alternative control plane.

---

## Profiles

Active profiles are selected with the comma-separated `DEVTOOL_PROFILES` environment variable.

Examples:

```bash
DEVTOOL_PROFILES=railway devtool project inspect --json
DEVTOOL_PROFILES=sourcegraph devtool code verify
DEVTOOL_PROFILES=railway,sourcegraph devtool project inspect --json
```

Profiles are applied left to right. A later profile replaces the same extension/service key set by an earlier profile.

See [Configuration Reference](configuration.md).

---

## Discover rather than memorize

For an arbitrary DevTool-managed project, use:

```bash
devtool project inspect --json
```

to discover project commands/resources, rather than assuming the DevTool repository's `build / verify / package` commands exist everywhere.
