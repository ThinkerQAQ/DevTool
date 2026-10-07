# Document Intelligence

## One-sentence definition

> Document Intelligence gives an agent a stable, structured view of long-form documents without forcing the agent to ingest an entire file or orchestrate parser/LSP provider APIs itself.

The first consumer is `ThinkerQAQ/blog-content`, but the capability must stay project-neutral and provider-neutral.

## Problem

Long Markdown articles can exceed the useful context budget of a single raw read. The current code-oriented path is optimized around symbols and references, while a document review task first needs:

- document metadata;
- the complete heading tree;
- exact section source ranges;
- one selected section body at a time;
- a reliable way to cover every section before making whole-document conclusions.

The current DevTool `code_context` capability is therefore not the right abstraction for article review.

## Architectural constraints

The implementation must preserve the existing DevTool principles:

1. **Minimal Core** — no Blog Content, Markdown, Goldmark, Marksman, Serena, or article-specific behavior in Core.
2. **Pluggable providers** — provider replacement is configuration plus provider implementation.
3. **Configuration-driven assembly** — `.devtool.toml` selects the project, capability, provider, SCM and provider settings.
4. **Stable Agent intent** — expose `document_context`, not parser/LSP methods.
5. **One canonical path** — normal document understanding goes through the capability/service contract.
6. **No proxy layer** — the capability owns document-context semantics; providers implement the document contract. It must not mechanically mirror Goldmark or Serena APIs.
7. **Configure first** — Blog Content contains only wiring and project guidance. Generic implementation stays in DevTool.

## Findings from the current repositories

### DevTool

The current Agent path is:

```text
Agent Gateway
  -> Agent Capability
  -> stable service contract
  -> configured provider
```

`code_context` is the reference shape:

```text
code_context
  -> code-indexed
  -> code-realtime
```

The Registry already supports multiple replaceable providers and configuration-based service selection. No new Core routing mechanism is required.

The current `ProjectHost` requires a Project Extension. A content-only repository should not create a repository-local fake build/devcontrol implementation. Instead DevTool should provide a reusable generic Project Extension for a workspace with identity but no project commands.

### Blog Content

`ThinkerQAQ/blog-content` is a content-source repository:

```text
src/content/
  articles/
  notes/
  note-translations/
  projects/
  series/
```

It currently has no DevTool configuration.

The repository contains more than one thousand Markdown notes, and some articles are large. The current thread-scheduling draft is about 2,290 lines with deep H2-H5 structure. This is exactly the case where outline-first, section-by-section reading is safer than a full raw read.

## Target architecture

```text
Agent
  |
  v
document_context
  |
  v
document-structure service
  |
  +---------------------------+
  |                           |
  v                           v
document.markdown.goldmark    future provider
```

The first implementation uses Goldmark because the immediate need is structural correctness and exact source ranges.

Marksman/Serena remains a possible future realtime provider for link/reference/diagnostic enrichment. It is not required for the first stable capability and should only be added after measuring an unmet need.

## Stable document contract

Add a provider-neutral SDK package:

```text
sdk/document
```

Service:

```text
document-structure
```

Method:

```text
inspect
```

Request:

```text
root
path
section?          # exact title or numbered prefix such as 1.3
include_content?  # default false
```

Response:

```text
path
format
line_count
frontmatter
outline[]
selected_section?
```

Each outline section contains:

```text
key?              # leading numbered key when present, e.g. 1.3
title
level
start_line
end_line
children[]
```

A selected section additionally contains exact source content for its range.

The contract is intentionally structural. It does not expose Goldmark AST node kinds, parser internals, or Marksman/LSP methods.

## Agent capability

Add:

```text
extensions/capability/document
```

Agent tool:

```text
document_context
```

Inputs:

```text
objective
path
section?
include_content?
```

Behavior:

1. validate the stable Agent intent;
2. invoke the selected `document-context` service;
3. let that service compose structure/review/optional relation semantics;
4. return one compact document-context bundle.

The Capability does not implement review traversal or parser/provider orchestration.

Expected usage for a long article:

```text
document_context(path=article)
  -> outline + ranges

document_context(path=article, section="1", include_content=true)
document_context(path=article, section="2", include_content=true)
document_context(path=article, section="3", include_content=true)
...
  -> whole-document review after coverage
```

The Agent performs reasoning and coverage. DevTool provides deterministic structure and bounded source retrieval.

## Markdown provider

Add:

```text
extensions/document/markdown
```

Provider ID:

```text
document.markdown.goldmark
```

Responsibilities:

- parse Markdown with Goldmark;
- detect headings structurally, ignoring heading-like text inside code fences;
- build nested heading hierarchy;
- compute exact 1-based section line ranges;
- parse YAML frontmatter;
- resolve one section by exact title or unique numbered prefix;
- optionally return that section's exact source content;
- enforce configured allowed roots.

Provider settings:

```toml
[extension.document-markdown.settings]
roots = ["src/content"]
```

The provider is pure Go and does not require an Environment service.

## Generic workspace Project Extension

Add:

```text
extensions/project/workspace
```

Provider ID:

```text
project.workspace
```

It is a real reusable Project Extension, not a proxy. It owns only generic workspace project semantics:

- project identity from settings;
- no commands;
- no resources/views unless explicitly added in the future;
- no provider-specific logic.

This keeps `ProjectHost` unchanged and avoids adding a content-only special case to Core.

Example:

```toml
[extension.project.settings]
name = "BlogContent"
```

## Blog Content wiring

Add to `ThinkerQAQ/blog-content`:

```text
.devtool.toml
AGENTS.md
```

The config should load DevTool extensions by pinned DevTool commit:

```text
project.workspace
capability.document
context.document.composite
document.markdown.goldmark
capability.scm
credential.store.file
credential.github
scm.github
```

Selected services:

```text
document-context   -> context.document.composite
document-structure -> document.markdown.goldmark
credential-store   -> credential.store.file
credential         -> credential.github
scm                -> scm.github
```

No BlogCTL runtime, Docker runtime, Dagger runtime, CodeGraph, or Go-specific project command is required for normal Blog Content review.

`AGENTS.md` should establish the canonical review workflow:

1. call `document_context` for the outline before reviewing a long document;
2. read/review sections through `document_context` rather than one full raw read;
3. track section coverage before whole-document conclusions;
4. keep Chinese/English counterparts aligned when both exist;
5. use DevTool SCM capability for checkpoints/publishing when available.

## Implementation phases

### Phase 0 — Plan and branch

- review current DevTool architecture and Blog Content layout;
- commit this plan before implementation.

### Phase 1 — Stable contract and generic workspace project

DevTool:

- add `sdk/document`;
- add `project.workspace`;
- add focused contract/provider tests.

No Core change.

### Phase 2 — Markdown structural provider

DevTool:

- add Goldmark + YAML frontmatter dependencies;
- implement `document.markdown.goldmark`;
- test nested headings, code fences, line ranges, frontmatter, section selection and root restrictions.

### Phase 3 — Document context service and Agent capability

DevTool:

- add `context.document.composite` behind the stable `document-context` service;
- keep bounded review/cursor/relation composition in that service provider;
- add thin `capability.document`;
- expose only `document_context`;
- test Capability delegation separately from document-context orchestration.

### Phase 4 — DevTool integration and self-host verification

- wire the new extensions into DevTool's own configuration only when needed for self-test fixtures;
- run Go tests;
- run DevTool self-host verification;
- verify existing `code_context` and SCM paths remain unchanged.

### Phase 5 — Blog Content adoption

Blog Content:

- add minimal `.devtool.toml`;
- add `AGENTS.md`;
- pin to the implemented DevTool commit;
- validate project inspection and readiness;
- verify Agent Gateway exposes `document_context` and SCM tools.

### Phase 6 — Real article acceptance

Use a real long Blog Content article and verify:

1. the first call returns the complete heading hierarchy without full-body ingestion;
2. section selection by numbered key returns the correct exact range;
3. code-fence content cannot create false headings;
4. section-by-section calls can cover the entire document;
5. a raw 2,000+ line read is no longer required for structural review.

## Non-goals

The first implementation does not add:

- embeddings/RAG;
- cross-document semantic search;
- Blog Content-specific rules inside DevTool;
- automatic article rewriting;
- a workflow DSL in TOML;
- a second Agent Gateway;
- provider-native parser/LSP tools;
- a Core special case for Blog Content.

## Acceptance checklist

- [ ] Core contains no Markdown/Blog Content/provider-specific behavior.
- [ ] `document_context` is the only new Agent-facing document tool.
- [ ] Document provider is selected by `.devtool.toml`.
- [ ] A second provider could implement the same `sdk/document` contract without changing Core/capability.
- [ ] Blog Content has no repository-local parser or fake devcontrol implementation.
- [ ] Long Markdown review works outline-first and section-by-section.
- [ ] Existing code intelligence, SCM and self-hosting remain green.
