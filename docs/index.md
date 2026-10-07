# DevTool Documentation

DevTool documentation is organized by reader intent.

If you are an agent entering the repository, start with [AGENTS.md](../AGENTS.md).

## Get started

- [Quick Start](quick-start.md) — validate configuration, inspect the project, verify code intelligence, self-host and connect an agent.
- [Agent Guide](agent-guide.md) — the end-to-end development workflow for AI agents.

## Understand the model

- [Architecture Principles](architecture/principles.md) — Minimal Core, replaceable providers, configuration wiring, self-hosting and lifecycle ownership.
- [Product Design](product-design.md) — broader product goals, scope and extension model.
- [Implementation Design](implementation-design.md) — implementation-level design.
- [Runtime Lifecycle](runtime-lifecycle.md) — process/container/MCP lifecycle ownership.
- [Microkernel Audit](architecture/microkernel-audit-20261005.md) — architecture audit and closure criteria.
- [Log Intelligence and Remote Transport](architecture/log-intelligence-and-remote-transport.md) — arbitrary-log context, local-first providers, and the boundary to a future self-hosted ChatGPT remote transport.

## How-to

- [Replace or Add a Provider](how-to/replace-or-add-provider.md) — add an implementation behind an existing capability without introducing a Core switch.
- [Migration Plan](migration-plan.md) — migrate projects onto the current architecture.

## Reference

- [CLI Reference](reference/cli.md) — stable Core CLI and DevTool's current Project Commands.
- [Configuration Reference](reference/configuration.md) — `.devtool.toml`, extension/service wiring and profiles.

## Reading path by task

### "I just need to use DevTool"

```text
Quick Start
  -> CLI Reference
  -> Configuration Reference
```

### "I am an AI agent changing this repository"

```text
AGENTS.md
  -> Agent Guide
  -> Architecture Principles
  -> relevant Reference/How-to
```

### "I want to add or replace an implementation"

```text
Architecture Principles
  -> Replace or Add a Provider
  -> Configuration Reference
```

### "I want to change Core"

Read these first:

```text
Architecture Principles
  -> Product Design
  -> Runtime Lifecycle
  -> Microkernel Audit
```

Then verify that the change is genuinely cross-project mechanism rather than provider/project behavior.

## Documentation rule

Keep these document types separate:

- **Quick Start** gets the reader to a working result quickly.
- **Guide** teaches a workflow.
- **Architecture/Concepts** explains why the system is shaped this way.
- **How-to** solves a specific engineering task.
- **Reference** describes exact commands/configuration.
- **Design documents** preserve deeper rationale and decisions.

Do not turn one page into all six at once.
