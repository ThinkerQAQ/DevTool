# CLI reference

[中文](../cn/reference/cli.md) · [Documentation](../index.md)

The commands below are implemented by the current CLI or, where stated, provided by this repository's Project Extension.

| Command | Description |
| --- | --- |
| `devtool version [--json]` | Embedded release/source identity |
| `devtool config path` | Path of resolved `.devtool.toml` |
| `devtool config validate` | Validate configuration and active profiles |
| `devtool project inspect [--json]` | Inspect project descriptor, bindings, and available commands |
| `devtool init [--json]` | Check readiness of selected providers |
| `devtool code doctor` | Check indexed and realtime code provider availability |
| `devtool code verify` | Exercise configured code intelligence |
| `devtool agent mcp [--context agent\|codex\|claude-code]` | Serve a local MCP gateway over stdio |
| `devtool agent serve [--listen :8080]` | Serve an HTTP MCP gateway (token-authenticated by default) |

## Project Extension commands

The following commands are declared by **DevTool's own** Project Extension. Other projects may expose different commands.

| Command | Purpose |
| --- | --- |
| `devtool build` | Produce a candidate binary |
| `devtool verify` | Verify the candidate and record its identity |
| `devtool package` | Produce a deliverable artifact |
| `devtool activate` | Activate the verified candidate locally |
| `devtool rollback` | Restore the previous local installation |

Consult `devtool project inspect --json` for command discovery rather than hard-coding these commands for every project.

## Source bootstrap

Inside this repository, substitute `go run ./cmd/devtool` for the executable name when no installed binary exists:

```bash
go run ./cmd/devtool config validate
go run ./cmd/devtool project inspect --json
```

## Profiles

Use the comma-separated `DEVTOOL_PROFILES` environment variable to compose configurations:

```bash
DEVTOOL_PROFILES=local devtool code doctor
DEVTOOL_PROFILES=sourcegraph devtool code verify
DEVTOOL_PROFILES=local,sourcegraph devtool project inspect --json
```

The commands select providers; they do not silently install unavailable external tools.

## Errors and safety

Read `init --json` for explicit readiness failures. `code doctor` checks availability, while `code verify` exercises providers. Remote `agent serve` requires appropriate authentication; do not publish `--allow-unauthenticated` outside a trusted local environment.

Run `devtool --help` and inspect the project for exact options supported by the installed version.
