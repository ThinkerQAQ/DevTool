# Document Context V2

## One-sentence goal

> Make whole-document review deterministic: DevTool should expose a stateless review traversal with explicit coverage, while keeping `document_context` as the single Agent-facing intent and leaving document parsing inside replaceable providers.

## Why V1 is not enough

V1 solved structural understanding:

```text
document_context(path)
  -> frontmatter
  -> outline
  -> exact ranges

document_context(path, section, include_content=true)
  -> exact section body
```

This already avoids one huge raw read, but the Agent still has to remember which top-level sections have been reviewed. On a long article, that manual coverage step is the remaining source of "read Java, forget Go / comparison / closing section" errors.

## Design principles

The change must preserve:

1. **Minimal Core** — no Core change.
2. **One Agent intent** — keep `document_context`; do not add provider-native tools.
3. **Capability owns orchestration** — review traversal belongs in `capability.document`, not in Goldmark.
4. **Provider remains structural** — the provider keeps exposing the stable `document-structure` contract.
5. **Stateless continuation** — no hidden in-memory review session; continuation is represented by an opaque cursor.
6. **Configuration-driven providers** — the selected document provider remains controlled by `.devtool.toml`.
7. **No proxy** — review mode combines multiple service calls into a higher-level Agent workflow.

## Target interaction

### Step 1 — Build a review plan

```text
document_context(
  objective = "Review the whole article",
  path = "...",
  review = true
)
```

Response:

```text
document
  -> frontmatter
  -> complete outline
  -> exact ranges

review
  total_sections
  covered_sections = []
  remaining_sections
  next_cursor
  next_section
  complete = false
```

### Step 2 — Read the next top-level section

```text
document_context(
  objective = "Review the whole article",
  path = "...",
  review = true,
  cursor = "<opaque cursor>"
)
```

Response:

```text
review
  current_section
  covered_sections
  remaining_sections
  next_cursor
  complete

section
  -> exact selected section body
```

Repeat until `complete=true`.

## Cursor model

The cursor is capability-owned and opaque to the Agent.

It contains only:

```text
document path
next top-level section index
```

Encoding can be URL-safe base64 over compact JSON.

The cursor is validated against the requested path. No hidden server-side state is required.

## Review units

V2 Phase 1 uses **top-level outline sections** as review units.

This directly solves whole-article coverage and keeps the implementation small.

A later phase can recursively split very large top-level sections by child headings and a context budget. That should not be mixed into the first coverage change.

## Capability behavior

`capability.document`:

1. invokes `document-structure.inspect` without content;
2. derives top-level review units from the provider-neutral outline;
3. on initial review call, returns the plan and first cursor;
4. on continuation, resolves exactly one review unit;
5. invokes `document-structure.inspect` again for that section with content;
6. returns explicit covered/remaining sections plus the next cursor.

Goldmark does not know about coverage or traversal.

## Tool schema

Extend the existing `document_context` input with:

```text
review? : boolean
cursor? : string
```

Rules:

- `cursor` requires `review=true`;
- `section` and `cursor` are mutually exclusive;
- existing V1 calls remain unchanged.

## Output shape

Initial review call:

```json
{
  "objective": "...",
  "document": { "...": "existing inspect response" },
  "review": {
    "total_sections": 7,
    "covered_sections": [],
    "remaining_sections": ["0", "1", "2", "3", "4", "5", "6"],
    "next_section": {"key": "0", "title": "..."},
    "next_cursor": "...",
    "complete": false
  }
}
```

Continuation call:

```json
{
  "objective": "...",
  "review": {
    "current_section": {"key": "0", "title": "..."},
    "covered_sections": ["0"],
    "remaining_sections": ["1", "2", "3", "4", "5", "6"],
    "next_cursor": "...",
    "complete": false
  },
  "section": {
    "start_line": 87,
    "end_line": 96,
    "content": "..."
  }
}
```

## Acceptance

Use the real long thread-scheduling article.

Acceptance requires:

- initial call reports every top-level section in order;
- repeated cursor calls eventually cover all sections;
- no top-level section can be skipped by the traversal;
- each continuation returns only the current section body, not the full document body;
- existing `section + include_content` behavior remains compatible;
- invalid / cross-document cursors fail clearly;
- Core diff remains zero;
- existing code intelligence, SCM, self-hosting and document provider tests remain green.

## Phase 1 acceptance result

The first implementation was exercised against the real 2,271-line thread-scheduling article.

The initial plan found all eight top-level review units:

```text
目录
0. 这一篇继续回答什么？
1. Java
2. Go
3. CPython
4. 三种实现的共同套路
5. 四种线程模型放在一起比较
6. 下一篇：Language Memory Model
```

Repeated cursor calls covered all eight units in order and ended with:

```text
remaining_sections = 0
complete = true
```

Each continuation returned only the current section body. Duplicate heading titles are safe because capability traversal uses the provider-neutral exact start-line anchor instead of title matching.

The acceptance also exposed the next efficiency limit:

```text
Java section    ~15K characters
Go section      ~13K characters
CPython section ~10K characters
```

Coverage is deterministic, but some top-level units are still larger than the desired context budget.

## Next phases

After review traversal is proven:

1. **Nested bounded traversal** — split oversized top-level sections by child headings and context budget.
2. **Cross-document context** — discover related documents and definitions across a series/project.
3. **Markdown realtime intelligence** — Marksman/Serena for links, references and diagnostics where it adds measurable value.
4. **Semantic retrieval** — optional provider for cross-document semantic search; not required for deterministic document coverage.
