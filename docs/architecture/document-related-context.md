# Document Related Context

## One-sentence goal

> Let `document_context` understand the document graph around the current file — parent series, ordered sibling articles, projects, explicit related notes and note scopes — without putting content-schema logic in Core or in the Agent capability.

## Real Blog Content relationship model

The current thread-scheduling draft already has deterministic relationship metadata:

```text
article
  concurrency-series-01-5-thread-scheduling
      |
      | frontmatter.series
      v
series
  concurrency-programming
      |
      +-- relatedArticles[]  -> ordered article sequence
      |
      +-- relatedNoteScopes[]
      |      category = java
      |      topicPath = [JUC]
      |      -> matching notes
      |
      +-- relatedNotes[]     -> explicit notes
      |
      +-- project?           -> project
```

The current series contains ten ordered articles. The draft is the third item.

Its `Java · JUC` note scope currently matches 45 source Markdown notes by frontmatter:

```text
category = java
topic    = JUC
```

The generated content manifest only contains 43 of those notes, so Document Intelligence must resolve relations from source-of-truth Markdown rather than trusting the stale generated manifest.

## Problem

V1/V2 `document_context` can understand one document and can traverse every section without omissions, but cross-document reasoning is still manual:

```text
article
  -> read series id
  -> guess series path
  -> read series
  -> inspect relatedArticles
  -> discover note scope
  -> search all notes
  -> decide which related document to read
```

That means the Agent can understand the relationship if it performs all of those steps itself, but DevTool does not yet guarantee the graph.

## Architecture

Keep the existing Agent boundary:

```text
Agent
  |
  v
document_context
  |
  +--------------------+
  |                    |
  v                    v
document-structure   document-relations
  |                    |
  v                    v
configured          configured
structure provider  relation provider
```

No Core change is required.

### Stable service

Add to `sdk/document`:

```text
service: document-relations
method:  resolve
```

Request:

```text
root
path
depth?
max_nodes?
```

Response:

```text
root_node
nodes[]
edges[]
warnings[]
```

Node:

```text
key       # globally unique inside one response
kind      # article / series / note / project
id        # content identity / slug
path
title
status?
language?
```

Edge:

```text
type
from
to
order?
label?
source?
```

The contract contains graph semantics only. It does not expose YAML parser details or Blog Content filesystem assumptions.

## Initial relation provider

Add a replaceable provider:

```text
extensions/document/relations/content
provider id: document.relations.content
```

It implements common content-repository frontmatter conventions.

Configured roots:

```toml
[extension.document-relations.settings]
articles = "src/content/articles"
series = "src/content/series"
notes = "src/content/notes"
projects = "src/content/projects"
```

The provider, not Core/capability, owns these conventions:

### Article

```text
series       -> member_of_series
project      -> member_of_project
relatedNotes -> related_note
```

### Series

```text
relatedArticles       -> ordered series_article
relatedNotes          -> related_note
relatedNoteCategories -> category-scoped related_note
relatedNoteScopes     -> metadata-scoped related_note
project               -> member_of_project
```

### Project

```text
tutorials      -> project_tutorial
documentation  -> project_documentation
```

## Note scope matching

A note is indexed from Markdown frontmatter.

Canonical fields:

```text
category
topic
topicPath[]
sourcePath
title
status
language
indexable
```

For `relatedNoteScopes`:

1. category must match when specified;
2. scope `topicPath` matches the normalized note topic path prefix;
3. if a historical note has no `topicPath`, `topic` becomes its one-element topic path;
4. the edge records the scope label/source so the Agent knows why the note is related.

This supports current historical JUC notes without rewriting them.

## Source of truth

The provider scans Markdown source, not `src/data/content-manifest.json`.

Reason:

```text
JUC Markdown notes = 45
manifest JUC notes = 43
```

The graph used for editing/review must reflect source files immediately.

A provider-local cache can avoid reparsing unchanged frontmatter, but cache state is implementation detail.

## Agent capability

Extend the existing `document_context` input:

```text
related? : boolean
relation_depth? : integer
relation_limit? : integer
```

Example:

```text
document_context(
  path = "src/content/articles/concurrency-series-01-5-thread-scheduling.md",
  objective = "Review this draft in its series and note context",
  related = true
)
```

Response adds:

```text
relations
  root_node
  nodes
  edges
  warnings
```

The Agent still has one document tool.

`related=true` may be combined with the initial `review=true` call so a whole-document review can begin with both:

- complete section coverage;
- cross-document series/note context.

Continuation cursor calls do not need to repeat the graph.

## Expected current article graph

For the thread-scheduling draft, the provider should resolve at least:

```text
draft article
  -> concurrency-programming series

series
  -> 10 ordered articles
  -> Java · JUC note scope
       -> 45 source notes
```

From the ordered series list, the Agent can also derive:

```text
previous = concurrency-series-01-hardware
current  = concurrency-series-01-5-thread-scheduling
next     = concurrency-series-02-language-memory-model
```

That enables consistency review across the article boundary without relying on model memory.

## Non-goals for this phase

- semantic/vector retrieval;
- reading all related note bodies automatically;
- ranking note relevance by embeddings;
- Markdown LSP;
- modifying content metadata;
- Blog Content logic in Core;
- a second Agent tool.

The graph only discovers deterministic relationships. The Agent requests bodies for the few related documents it actually needs.

## Acceptance

Use the real concurrency draft.

Acceptance requires:

- Core diff remains zero;
- one Agent-facing tool remains `document_context`;
- current article resolves to `concurrency-programming`;
- series resolves ten ordered articles;
- current draft order is correct;
- JUC note scope resolves 45 source notes;
- each note edge explains the matching scope;
- direct focused document calls still work;
- review traversal remains green;
- provider is selected by `.devtool.toml`;
- Blog Content and its template remain configuration-only.
