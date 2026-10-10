# DevTool documentation

**Language:** English (default) · [简体中文](cn/index.md)

The guides below describe the current repository, not a proposed future implementation.

## Tutorial

- [Quick Start](quick-start.md) — start from a checkout, validate the configuration, and inspect the project.
- [Agent Guide](agent-guide.md) — let an MCP-capable agent work through DevTool.

## Concepts

- [Architecture Overview](architecture/overview.md) — Core, extensions, contracts, and providers.
- [Architecture Principles](architecture/principles.md) — decision rules for extending DevTool.

## How-to

- [Replace or Add a Provider](how-to/replace-or-add-provider.md).
- [Build, Verify, and Activate](how-to/self-host.md).

## Reference

- [CLI Reference](reference/cli.md).
- [Configuration Reference](reference/configuration.md).

## Scope and maintenance

- English pages in `docs/` are canonical. The corresponding Chinese pages live in `docs/cn/` with the same relative names.
- Commands must match the current CLI; settings must match `.devtool.toml` and the configuration parser.
- Explain prerequisites, a runnable example, expected result, and common failure modes. Do not present a future feature as implemented.
- Avoid release diaries, approval language, generic claims, and generated summaries in the user guide. Git history and pull requests hold old project decisions.
