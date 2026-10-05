# Replace or Add a Provider

This guide shows the preferred way to change an implementation behind an existing DevTool capability.

The rule is:

> Existing capability + new implementation = provider + configuration, not a Core switch.

## Replace a configured provider

DevTool already demonstrates this with indexed code intelligence.

Base configuration:

```toml
[service.code-indexed]
provider = "intelligence.codegraph"
```

Alternative profile:

```toml
[profile.sourcegraph.service.code-indexed]
provider = "intelligence.sourcegraph"
```

Activate it:

```bash
DEVTOOL_PROFILES=sourcegraph go run ./cmd/devtool code verify
```

The Agent Gateway still exposes the same stable `code_context` intent.

No agent workflow changes are required.

---

## Add a new provider for an existing service

### 1. Identify the stable service contract

Find the SDK/service contract that expresses the engineering intent.

Examples in the current repository include:

- environment;
- portable runtime;
- indexed code intelligence;
- realtime code intelligence;
- SCM.

If the existing contract already represents the behavior, do not create a new Core abstraction.

### 2. Implement an Extension

The extension should declare:

- a stable extension/provider ID;
- extension kind;
- provided service(s);
- required service(s), if any;
- configurable settings when needed.

Provider implementation details stay inside this extension.

### 3. Add loader configuration

Example shape:

```toml
[extension.my-provider]
loader = "go"

[extension.my-provider.loader_config]
module = "."
package = "./extensions/example/my-provider/cmd/provider"
```

The loader configuration tells DevTool how to resolve the extension. Core should not gain a `switch` for the provider ID.

### 4. Wire the service

```toml
[service.some-capability]
provider = "example.my-provider"
```

Or make it optional through a profile:

```toml
[profile.my-provider.service.some-capability]
provider = "example.my-provider"
```

### 5. Verify the resolved project

```bash
DEVTOOL_PROFILES=my-provider \
go run ./cmd/devtool project inspect --json
```

Then run the capability-specific verification path.

For code intelligence:

```bash
DEVTOOL_PROFILES=my-provider \
go run ./cmd/devtool code doctor

DEVTOOL_PROFILES=my-provider \
go run ./cmd/devtool code verify
```

### 6. Verify the stable agent surface

If the provider implements an existing capability, the normal Agent Gateway should not need new provider-specific tools.

Run:

```bash
DEVTOOL_PROFILES=my-provider \
go run ./cmd/devtool agent mcp --context codex
```

The expected result is stable intent-level tools such as `code_context`, not `my_provider_search` or other implementation-specific names.

---

## When a new Capability is justified

Create a new stable capability only when the project needs a new cross-provider engineering intent.

Good reason:

```text
multiple projects need "artifact_publish"
and more than one backend could implement it
```

Weak reason:

```text
one provider added a new method
```

Provider API surface and Agent Capability surface have different stability requirements.

---

## When to change Core

A Core change is justified when the missing behavior is mechanism shared by every implementation.

Examples:

- extension lifecycle semantics;
- registry/routing mechanics;
- configuration discovery;
- generic protocol behavior;
- ownership/cleanup rules;
- generic policy/side-effect enforcement.

A Core change is suspicious when it mentions:

- one provider ID;
- one project name;
- one infrastructure vendor;
- one tool's command-line flags.

---

## Acceptance checklist

A provider addition/replacement is complete when:

- the provider is loadable through generic extension loading;
- service selection is configuration-driven;
- no provider-ID switch was added to Core;
- the Project Extension still depends on the stable service;
- the Agent Gateway still exposes stable intent;
- relevant doctor/verify paths pass;
- self-hosting still works when the change affects DevTool's own dependency graph.

## Related

- [Architecture Principles](../architecture/principles.md)
- [Configuration Reference](../reference/configuration.md)
- [Agent Guide](../agent-guide.md)
