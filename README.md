# DevTool

DevTool is a self-hosting, extensible, configuration-driven engineering control plane for local development, AI agents, CI/CD, native-device workflows, and project control surfaces.

Core principle:

> Minimal core + extensible modules + configuration wiring + self-hosting.

Design documents:

- [Product Design / PRD](docs/product-design.md)
- [Implementation Design](docs/implementation-design.md)
- [Migration Plan](docs/migration-plan.md)

DevTool intentionally keeps project semantics, portable execution, native capabilities, infrastructure integrations, and UI features behind replaceable extension contracts.

The first portable runtime implementation is expected to use Dagger through an adapter. Dagger is not part of DevTool Core and can be replaced without changing project contracts.

DevTool itself is a first-class dogfooding project: it must be developed, verified, built, packaged, installed, and released through DevTool.


## Runtime image layering

DevTool keeps the shared development environment and the DevTool runtime separate:

```text
ghcr.io/thinkerqaq/dev-base
  -> language/toolchain dependencies
  -> CodeGraph / Serena / gopls / Dagger prerequisites

ghcr.io/thinkerqaq/devtool-runtime:<commit>
  -> FROM dev-base
  -> adds the versioned DevTool binary
```

Remote deployments such as Railway should use the DevTool runtime image. This avoids rebuilding DevTool during container startup while keeping the shared DevEnvironment image project-agnostic.

Provider selection remains configuration-driven. For example, Railway sets `DEVTOOL_PROFILE=railway`, which selects `environment.local` without mutating `.devtool.toml` at runtime.
