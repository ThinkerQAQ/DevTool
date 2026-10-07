# Worktree-Aware Workspace and CodeGraph Freshness

## One-sentence definition

> DevTool should treat each Git worktree as an isolated development workspace, keep LSP realtime semantics unchanged, and make CodeGraph responsible for the freshness of its own persistent index.

This design preserves DevTool's existing architecture:

```text
Agent / Human / CI
        |
        v
Stable Capability / Service
        |
        v
Configured Provider
```

It does not introduce a proxy layer, a second control plane, or provider-specific branching in Core.

---

## 1. Problem statement

IDFlow, PhotoWaypoint, DevTool and similar projects routinely have multiple branches under active development.

A single checkout directory creates one mutable workspace:

```text
/workspaces/IDFlow
    branch A
      |
   checkout
      v
    branch B
```

All workspace-scoped state is forced to follow that mutation:

- filesystem contents;
- LSP snapshots and diagnostics;
- CodeGraph persistent index;
- Agent session context;
- build/runtime state.

This is especially problematic for persistent indexed intelligence.

A verified CodeGraph 0.21 behavior is:

```text
b.go exists
  -> indexed

delete b.go
  -> reindex(force=false)

symbol from b.go
  -> can remain in the persistent graph
```

A forced reindex removes the stale symbol:

```text
reindex(force=true)
  -> deleted-file node disappears
```

Git worktrees reduce the frequency of bulk workspace replacement by assigning concurrent branches distinct workspace roots.

---

## 2. Fixed architecture

The target architecture is:

```text
                         Workspace Service
                               ^
                               |
                         workspace.git
                               |
                 +-------------+-------------+
                 |                           |
                 v                           v
        Realtime Code Service         Indexed Code Service
                 ^                           ^
                 |                           |
       intelligence.lsp.serena      intelligence.codegraph
```

Responsibilities remain separate:

- Workspace service owns workspace lifecycle and identity.
- Git provider owns `git worktree` behavior.
- LSP provider owns realtime language semantics.
- CodeGraph provider owns persistent graph freshness.
- Core owns only stable service routing and lifecycle mechanisms.

There is no:

```text
CodeGraphProxy
WorkspaceProxy
if provider == codegraph
if workspace == git
```

in Core.

---

## 3. Existing `project.workspace` remains unchanged

The existing `extensions/project/workspace` provider is a project descriptor provider.

It currently owns:

```text
ProjectDescriptor
  -> ProjectIdentity{Name}
```

That is not the same domain as development workspace lifecycle.

It must not be expanded into a Git worktree manager.

A new stable workspace service will represent development workspaces.

---

## 4. Workspace contract

The stable contract expresses engineering semantics, not Git commands.

Conceptually:

```text
Workspace Service
├── create
├── list
├── inspect
└── remove
```

The identity model is:

```text
WorkspaceIdentity
├── RepositoryID
├── WorkspaceID
└── Root
```

Where:

```text
RepositoryID
  -> repository/common storage identity

WorkspaceID
  -> one concrete development workspace identity

Root
  -> canonical workspace filesystem root
```

The contract must not expose provider-native methods such as:

```text
CreateGitWorktree
DeleteGitWorktree
CheckoutGitWorktree
```

The Git provider maps stable operations to Git internally:

```text
Workspace.Create
      |
      v
git worktree add

Workspace.Remove
      |
      v
git worktree remove
```

---

## 5. Worktree operating model

Concurrent work becomes:

```text
Repository
├── main workspace
├── feat/ui workspace
├── feat/appointment workspace
└── benchmark workspace
```

Each workspace owns isolated mutable state:

```text
Workspace A
├── dirty filesystem
├── branch/tree
├── LSP session
├── CodeGraph project
├── Agent session
└── runtime/build state

Workspace B
├── dirty filesystem
├── branch/tree
├── LSP session
├── CodeGraph project
├── Agent session
└── runtime/build state
```

Repository-level resources may remain shared:

- Git object database;
- remote configuration;
- credentials;
- Go module cache;
- Go build cache;
- SDK/toolchain files;
- OS page cache.

---

## 6. LSP remains unchanged

No LSP pooling or semantic-state sharing is introduced.

The default remains:

```text
Workspace A -> LSP A
Workspace B -> LSP B
Workspace C -> LSP C
```

The operating system and language toolchain already share lower-level immutable/cache resources.

This change therefore does not add:

- a shared gopls daemon;
- a global language-server pool;
- cross-worktree mutable semantic caches;
- an LSP proxy.

LSP remains the realtime view of the current workspace.

---

## 7. CodeGraph role

CodeGraph is a rebuildable persistent acceleration index.

It is not the source of truth.

```text
Filesystem + LSP
      |
      | source of current truth
      v
 Code Context
      ^
      |
 CodeGraph
 persistent structural index
```

The CodeGraph provider owns its own freshness lifecycle.

It tracks enough state to distinguish:

- a new workspace;
- ordinary source edits;
- a Git tree transition;
- stale indexed paths.

---

## 8. Separate identity from revision state

The current CodeGraph implementation uses a recursive filesystem fingerprint containing path, size and mtime.

That mixes two separate concepts:

```text
Workspace Identity
vs.
Workspace Content Version
```

The replacement model is:

### Workspace identity

Used for provider process/session ownership.

```text
WorkspaceIdentity
  = canonical root
  + worktree identity
```

For Git-backed workspaces, the provider can derive stable information from Git metadata such as common-dir/worktree git-dir without leaking Git fields into the generic workspace contract.

### Revision state

Used for index freshness decisions.

```text
RevisionState
├── HEAD
└── tree
```

A Git tree transition is treated as a bulk workspace replacement signal.

Ordinary dirty edits continue through incremental indexing.

---

## 9. CodeGraph freshness policy

Freshness remains internal to `intelligence.codegraph`.

Conceptually:

```text
EnsureFresh(workspace)
        |
        +-- new workspace
        |      -> force reindex
        |
        +-- git tree changed
        |      -> force reindex
        |
        +-- ordinary edit
               -> incremental reindex
```

No generic freshness manager is added to Core.

No Agent tool directly invokes CodeGraph reindex operations.

---

## 10. Strong verification

`code verify` means proving that the configured indexed provider matches the current workspace.

For CodeGraph this requires a forced reconciliation:

```text
devtool code verify
        |
        v
codegraph_reindex_workspace
        force=true
```

Readiness/init may use a cheaper incremental warm-up when appropriate, but verification must be strong.

---

## 11. Stale result guard

Persistent indexes can still contain invalid references because of provider bugs or interrupted lifecycle transitions.

Before CodeGraph search/discovery results are exposed upward, the provider validates workspace-local file paths.

```text
CodeGraph result
      |
      v
resolve against workspace
      |
      +-- exists
      |     -> keep
      |
      +-- missing
            -> drop
            -> mark index stale
            -> force refresh
```

This guard belongs inside `intelligence.codegraph`.

It is not a proxy because it enforces the correctness of one provider's own output.

---

## 12. Worktree lifecycle

Creating a new worktree produces a new workspace identity and therefore a distinct CodeGraph/LSP lifecycle.

```text
Workspace.Create
      |
      v
workspace.git
      |
      v
new workspace root
      |
      +-- CodeGraph cold/warm index
      +-- LSP session
      +-- Agent session
      +-- runtime state
```

Workspace removal follows lifecycle ownership rather than acting as a global process supervisor.

The Agent/Host that owns a workspace-scoped LSP, CodeGraph MCP process, or runtime process must close those resources when that owner/session ends. The workspace provider removes the clean filesystem worktree; it does not discover and terminate unrelated provider processes owned by another Agent/Host.

```text
Agent / ProjectHost lifetime ends
      |
      +-- close realtime provider session
      +-- close indexed provider session
      +-- release owned runtime/process state
      |
      v
Workspace.Remove
      |
      v
workspace.git remove
```

Persistent CodeGraph storage is a rebuildable cache and may outlive the filesystem worktree. A later workspace with a distinct workspace identity must not reuse that cache incorrectly.

Do not add a global workspace process registry or event bus merely to make `workspace_remove` terminate resources it does not own.

---

## 13. Sourcegraph remains a replaceable provider

Sourcegraph is not used as a replacement for local realtime workspace semantics.

It remains an optional provider selected through configuration:

```toml
[service.code-indexed]
provider = "intelligence.codegraph"

[profile.sourcegraph.service.code-indexed]
provider = "intelligence.sourcegraph"
```

The intended roles are:

```text
LSP
  -> realtime current-workspace semantics

CodeGraph
  -> local persistent structural graph

Sourcegraph
  -> remote/cross-repository/history/revision search
```

There is no CodeGraph-to-Sourcegraph fallback proxy.

---

## 14. Implementation phases

### W0 - CodeGraph correctness

1. Make CodeGraph `verify` use `force=true`.
2. Add stale workspace-path filtering for search/discovery results.
3. Add a deleted-file regression test proving stale symbols are not exposed.

Acceptance:

```text
index file
delete file
verify/search
-> deleted symbol is not returned
```

### W1 - Workspace identity

Introduce the stable workspace service contract and identity model.

Requirements:

- canonical workspace root;
- repository identity;
- workspace identity;
- no Git-specific API in the contract;
- existing `project.workspace` unchanged.

### W2 - CodeGraph lifecycle

Replace recursive content fingerprinting as the MCP session identity mechanism.

Split:

```text
workspace identity -> process/session ownership
revision state     -> index freshness
```

Add Git tree transition detection within the provider integration boundary without adding Core provider special cases.

### W3 - Git worktree provider

Implement:

- create;
- list;
- inspect;
- remove.

The provider owns Git commands and physical worktree layout.

### W4 - Agent workspace intent

Expose stable Agent engineering intent only if a real user/Agent workflow requires it.

Preferred surface:

```text
workspace_create
workspace_list
workspace_remove
```

Avoid provider-native Git worktree operations.

### W5 - Parallel acceptance

Verify with at least:

```text
main workspace
worktree A
worktree B
```

Acceptance conditions:

- distinct workspace identities;
- distinct CodeGraph project indexes;
- isolated LSP sessions;
- deleting a file in A cannot produce a stale file result after provider reconciliation;
- B is unaffected;
- A/B provider sessions remain workspace-scoped and are closed by their owning Agent/Host lifecycle;
- removing clean A removes only A's filesystem worktree and leaves B unaffected;
- self-hosting build/verify still uses the normal DevTool path.

---

## 15. Commit checkpoints

Use the normal small-step development cadence:

```text
01 docs: define worktree and codegraph freshness architecture

02 fix(codegraph): make verify strongly reconcile index
03 fix(codegraph): filter stale indexed paths
04 test(codegraph): cover deleted-file stale index

05 feat(workspace): define workspace service contract
06 feat(workspace): add git workspace provider

07 refactor(codegraph): separate workspace identity from revision state
08 refactor(codegraph): refresh on git tree transition
09 refactor(codegraph): keep provider sessions scoped to workspace identity

10 feat(agent): expose workspace intent if justified
11 test: verify parallel worktree isolation
```

Each coherent change is committed and pushed independently. Full validation runs after each phase and at final acceptance.

---

## 16. Architecture gates

The finished design must preserve:

```text
Core
  X imports intelligence.codegraph
  X imports workspace.git
  X branches on CodeGraph
  X branches on Git worktree

intelligence.codegraph
  ✓ implements Indexed Code Service
  ✓ owns persistent-index freshness
  ✓ owns CodeGraph-specific reconciliation

workspace.git
  ✓ implements Workspace Service
  ✓ owns git worktree commands

intelligence.lsp.serena
  ✓ remains Realtime Code provider
  ✓ remains workspace-scoped
  ✓ requires no change for this project

Agent
  ✓ consumes stable engineering intent
  X orchestrates CodeGraph reindex
  X directly owns git worktree lifecycle
```

---

## 17. Non-goals

This work does not implement:

- a generic index proxy;
- shared mutable LSP semantic state;
- a custom overlay graph;
- a Sourcegraph replacement migration;
- a universal repository abstraction;
- a workflow/event-bus framework;
- automatic background process pooling unrelated to workspace lifecycle.

The smallest correct architecture is preferred.
