# Build, verify, and activate DevTool

[中文](../cn/how-to/self-host.md) · [Quick Start](../quick-start.md)

DevTool can develop itself using the same Project Extension interface available to other projects.

## Check the selected project

```bash
devtool project inspect --json
devtool config validate
```

The DevTool repository's Project Extension defines `build`, `verify`, and `package`. It uses configured environment/runtime providers, which may require external dependencies.

## Build and verify

```bash
devtool build
devtool verify
```

The candidate is written to `.devtool/out/devtool-next` (or `.exe` on Windows). Verification creates `.devtool/out/devtool-next.verified.json` with release version, source identity, and the candidate digest.

Inspect the candidate before activation:

```bash
./.devtool/out/devtool-next version --json
./.devtool/out/devtool-next project inspect --json
```

## Activate or roll back

On a host with an existing DevTool installation, after successful verification:

```bash
devtool activate
```

The local installation uses `~/.local/bin/devtool`; versioned artifacts and previous binaries are maintained under `~/.local/share/devtool`. To restore the previous activated version:

```bash
devtool rollback
```

Do not activate an unverified candidate. Run `devtool package` when a package is needed, not merely to use the installed CLI.

## Bootstrap a fresh checkout

Without an installed `devtool`, use `go run ./cmd/devtool <command>` to enter DevTool's own configured extension graph. This bootstrap exception is not permission to replace Project Extension commands with arbitrary build scripts.

See [CLI Reference](../reference/cli.md) and [Configuration Reference](../reference/configuration.md).
