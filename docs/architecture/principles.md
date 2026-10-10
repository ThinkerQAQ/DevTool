# Architecture principles

[中文](../cn/architecture/principles.md) · [Architecture Overview](overview.md)

## Minimal Core

Core owns project discovery, configuration, extension loading, registries, routing, and lifecycle coordination. It must not know specific vendor IDs, project workflows, or installation commands.

**Test:** Would a second provider for the same service require this change? If not, the change probably belongs outside Core.

## Reuse first

Before implementing functionality, examine maintained open-source code, SDKs, frameworks, services, and standard protocols. Check feature coverage, maintenance, license, portability, and extension points. Write new infrastructure only when no suitable option meets the requirements.

## Configuration selects; code implements

Use `.devtool.toml` to register extensions, bind services to providers, and select environment profiles. TOML is declarative wiring, not a script runner or a second programming language.

```toml
[service.code-indexed]
provider = "intelligence.codegraph"

[profile.sourcegraph.service.code-indexed]
provider = "intelligence.sourcegraph"
```

## Stable contracts; replaceable providers

A capability describes engineering intent, such as `code_context` or `scm_publish`. The service defines a replaceable contract; a provider implements it. Do not expose a separate agent tool solely because a provider adds an API method.

## Project logic lives in Project Extensions

Projects declare `build`, `verify`, `package`, and other domain commands. DevTool itself has no exceptional implementation route. Inspect project commands before running ad-hoc build scripts.

## Portable and native boundaries

Builds, packages, and containers belong behind portable environment/runtime services. USB, ADB, host OS integration, and physical devices remain native; do not force them into a container abstraction.

## Lifecycle and side effects

The owner that starts a process or acquires a resource owns its cancellation and cleanup. Keep credentials and external writes behind explicit providers; avoid hidden fallback paths with different behavior.

## Change discipline

Prefer root-cause fixes over compatibility shims. Keep commits small and push after each coherent change. Verify the related batch through DevTool. Replace obsolete implementations instead of maintaining parallel behaviors without a requirement.

## Documentation discipline

Document current, tested behavior. Separate tutorials, how-to procedures, concepts, and references. Maintain English and Chinese pages together, with English as the default entry.
