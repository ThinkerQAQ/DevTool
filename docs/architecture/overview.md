# Architecture overview

[中文](../cn/architecture/overview.md) · [Documentation](../index.md)

## One-line model

DevTool is a thin engineering control plane that selects concrete implementations through declarative configuration.

```text
              Human / Agent / CI
                      |
                CLI / MCP API
                      |
           Project Extension / Capability
                      |
                Service contract
                      |
             configured provider
                      |
          Git / LSP / Docker / Dagger / ...
```

## Ownership

| Layer | Responsibility | Does not own |
| --- | --- | --- |
| Core | Project discovery, extension registration/loading, routing, policy and lifecycle | MCP wire protocol, container, GitHub, or project-specific behavior |
| Project Extension | Project commands (`build`, `verify`, `package`) and project resources | Selection of a concrete infrastructure provider |
| Capability | Stable user/agent intent such as `code_context` or `scm_publish` | Provider-specific tool names |
| Service | Contract for one replaceable class of implementation | User interface or application workflows |
| Provider | Integration with CodeGraph, Serena, Sourcegraph, Dagger, GitHub, etc. | Cross-provider Core policy |

## MCP transport

The official [Go MCP SDK](https://github.com/modelcontextprotocol/go-sdk) owns the Server, stdio/Streamable HTTP transports, Client bridges, protocol version negotiation, request correlation, and cancellation. DevTool owns **engineering capabilities**, project-aware routing, selected Providers, and optional context-bound patch policies. Do not add a second JSON-RPC protocol implementation to Core.

Both older MCP clients that negotiate `2025-06-18` and modern SDK clients are supported. SDK sessions are transport-level; the active DevTool Gateway retains one-use patch authorization in memory. Bearer token authentication remains required for remote HTTP access.

## Code intelligence

Two independently configured services answer different questions:

- `code-indexed`: indexed/structural repository search. The default example uses CodeGraph, with Sourcegraph available as an alternative.
- `code-realtime`: live language-server semantics. The repository uses Serena/LSP.

An agent requests `code_context`; the capability coordinates the appropriate services. Switching providers should not require changing the caller.

Document and diagram capabilities use the same pattern. Merman diagram intelligence uses the mature [jrpc2](https://github.com/creachadair/jrpc2) LSP transport and JSON-RPC client for framing, request/response correlation and server callbacks; DevTool retains diagram-specific symbol and diagnostic mapping. The configured `merman-lsp` executable remains replaceable through `.devtool.toml`. No custom LSP framing implementation is maintained.

## Environment and lifecycle

The Docker environment provider can consume the [DevEnvironment](https://github.com/ThinkerQAQ/DevEnvironment) toolchain images. The selected `dev-base` image is pinned by digest in the repository’s [`.devtool.toml`](../../.devtool.toml). DevEnvironment owns toolchain builds and image publication; DevTool owns provider contracts and lifecycle. Neither repository vendors the other.

Portable operations can use configured environment/runtime providers. Host-specific operations (for example a physical Android device) remain native. Core coordinates lifetime and cleanup; it does not implement an external tool's runtime.

## Self-hosting

DevTool defines itself as an ordinary DevTool project. Its Project Extension provides `build`, `verify`, and `package`. A generated binary includes version and commit identity and can be verified before activation. See [self-hosting](../how-to/self-host.md).

## Extension decision

Before modifying Core:

1. Try a mature existing library, SDK, protocol, or provider.
2. Check whether TOML can select an existing implementation.
3. Check whether the current service/capability contract already models the need.
4. Implement provider-specific behavior in the provider, project-specific behavior in the Project Extension.
5. Change Core only for a mechanism that every provider or project genuinely shares.

Read [Architecture Principles](principles.md) for these rules in detail.
