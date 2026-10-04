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
