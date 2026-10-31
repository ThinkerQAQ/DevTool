# DevTool: an engineering experiment and its lessons

[中文](cn/retrospective.md) · [Documentation](index.md) · [Repository](https://github.com/ThinkerQAQ/DevTool)

**Status (October 2026): experimental; active development paused; not production-ready.**

This document records what the repository set out to explore and which engineering questions remain open. It is not a release announcement or a claim that the prototype provides a supported production service.

## The original question

Could one project-aware engineering control plane serve humans, AI agents, and CI without replacing Git, LSPs, code search engines, container runtimes, or existing build tools?

The conventional alternative is for every client to directly integrate each tool, handle its configuration, and duplicate cross-tool orchestration. DevTool explored a thin, declaratively configured layer with **stable intent-level capabilities** above replaceable Providers.

## Engineering work preserved in this repository

| Area | Explored implementation | Boundary |
| --- | --- | --- |
| Project lifecycle | Project Extension contracts for inspect/build/verify/package and self-hosting | External runtime dependencies still apply |
| Provider independence | Service contracts and TOML profiles for switching implementations | Providers do not all have identical behavior |
| Code context | Indexed search (CodeGraph/Sourcegraph) alongside realtime semantics (Serena/LSP) | Availability and freshness depend on configured tools |
| Agent integration | MCP gateway for intent-level engineering tools; official Go MCP SDK for transport | An MCP endpoint alone does not ensure safe agent decisions |
| Documentation/diagrams | Markdown and Mermaid/PlantUML context through configured services | Requires compatible external providers |
| Edit controls | Context-bound, one-use authorization for a scoped Git patch | Not a general-purpose security guarantee |
| Packaging and validation | Candidate build, verify, package, and activation/rollback paths | A development workflow is not a supported release process |

For precise commands and contract boundaries, see the [architecture](architecture/overview.md), [CLI reference](reference/cli.md), and [configuration reference](reference/configuration.md). The source and tests are the ultimate record of implemented behavior.

## Why this was technically difficult

1. **Choosing the right abstraction boundary.** A uniform capability must remain useful without flattening provider-specific strengths or embedding vendor-specific switches in Core.
2. **Reconciling different kinds of context.** Indexed search and live LSP information answer different questions and have distinct freshness, latency, and failure modes.
3. **Making agents act safely.** Tool discovery, project context, credential isolation, authorization, and side-effect boundaries all matter beyond exposing an MCP tool.
4. **Working across environments.** Local executables, remote services, containers, and project-specific operations have different prerequisites and lifecycle semantics.
5. **Controlling integration cost.** A control plane can become harder to configure and maintain than using individual tools directly if its contracts are too broad.

These are technical trade-offs demonstrated by the scope of the project; this list is **not** a verified root-cause analysis of why development stopped.

## Outcome and unfinished work

The repository reached a substantial experimental implementation, but **not** the complete, stable, broadly usable platform originally envisioned. Version metadata remains `0.1.0-dev`. Active development was paused, and the repository was archived before being reopened to preserve and document the work.

The following are **unresolved product-readiness questions**, not promises or existing guarantees:

- Consistently reproducible onboarding across machines and providers.
- A small, reliable default experience without extensive per-provider setup.
- A maintained release and compatibility policy.
- Security review, operational hardening, and real-world adoption evidence.
- A clear demonstration that the control plane provides more value than its ongoing integration and maintenance cost.

No general installation, compatibility, maintenance, or production support is implied.

## What remains useful

DevTool is a reference for **minimal core design, replaceable Providers, declarative wiring, and intention-level Agent integration**. Reading the contracts, tests, and historical design decisions can be useful even if running the entire stack is impractical.

A project can fail to reach its product goal while still yielding concrete engineering work and reusable design lessons. This repository is presented in that spirit, without rebranding an unfinished experiment as a successful product.

## Source and licensing

The author's original source and documentation are published under the [MIT License](../LICENSE). Third-party libraries and external tools retain their own licenses and are not relicensed by this repository. Review dependencies and security implications independently before reuse.
