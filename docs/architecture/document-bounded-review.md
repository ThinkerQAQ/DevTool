# Document Bounded Review

## One-sentence goal

> Keep deterministic whole-document coverage while ensuring each review continuation returns a bounded, non-overlapping slice of the document whenever heading structure permits.

## Problem

Whole-document review traversal now guarantees coverage, but its units are top-level headings.

The real thread-scheduling article exposes the remaining efficiency problem:

```text
Java     ~830 lines / ~15K characters
Go       ~670 lines / ~13K characters
CPython  ~530 lines / ~10K characters
```

The Agent no longer skips a top-level chapter, but a single continuation can still inject too much source context.

## Architectural rules

1. **Core remains unchanged.**
2. **One Agent tool remains `document_context`.**
3. **Capability owns review planning and cursor semantics.**
4. **Document provider owns exact source extraction.**
5. **Review units must not overlap or leave gaps.**
6. **Heading hierarchy is the first splitting boundary.**
7. **Do not split arbitrary Markdown blocks merely to hit a byte/token target.**
8. **Default budget is configuration-driven and can be overridden per review.**
9. **Provider APIs remain hidden behind `document-structure`.**

## Stable contract extension

The current provider can select a complete section by title/key/start-line.

Bounded traversal also needs exact range retrieval for parent-section preambles.

Extend `sdk/document.InspectRequest`:

```text
range_start_line?
range_end_line?
```

Extend `InspectResponse`:

```text
selected_range?
  start_line
  end_line
  content?
```

Rules:

- section selection and range selection are mutually exclusive;
- range is 1-based and inclusive;
- range must be inside the document;
- `include_content=true` returns the exact range source;
- this is a generic document contract, not a Goldmark-specific AST surface.

## Configuration

`capability.document` gains a configurable default:

```toml
[extension.document-capability.settings]
review_max_lines = 300
```

Default when omitted:

```text
300 lines
```

The Agent may override the budget on the initial review call:

```text
document_context(
  path = "...",
  review = true,
  review_max_lines = 240
)
```

The chosen budget is embedded in the opaque review cursor. Continuation calls do not need to repeat it.

## Review planning algorithm

For each top-level section:

```text
section <= budget
  -> one review unit

section > budget and has children
  -> parent preamble
  -> recursively plan direct child sections

section > budget and has no children
  -> one oversized review unit
  -> report oversized=true
```

### Parent preamble

If a parent is split, preserve the lines before its first child:

```text
## 1. Java                 <-+
intro paragraph              | preamble unit
diagram / explanation      <-+

### 1.1 Model             <- child unit
...
### 1.2 Layering          <- child unit
...
```

Preamble range:

```text
parent.start_line
  ..
first_child.start_line - 1
```

The direct child ranges already cover the remainder of the parent without overlap.

### Recursive decomposition

Example:

```text
1 Java                         830 lines
├─ preamble                     25
├─ 1.1 Model                   150
├─ 1.2 Layering                 35
├─ 1.3 Lifecycle               380
│  ├─ preamble                  30
│  ├─ 1.3.1 Create              70
│  ├─ 1.3.2 Runnable            50
│  └─ ...
└─ ...
```

A large child is recursively decomposed by its own children.

## Review unit model

Keep the existing response envelope stable, but enrich each review section/unit with:

```text
key?
title
level
start_line
end_line
range_kind = section | preamble
oversized?
```

Existing JSON fields remain:

```text
total_sections
current_section
covered_sections
remaining_sections
next_section
next_cursor
complete
```

They continue to represent ordered review units. This avoids creating a second traversal protocol.

## Cursor

Extend the opaque cursor with:

```text
path
next
signature
max_lines
```

The signature is calculated from:

```text
document path
review budget
planned review units
```

Therefore the cursor becomes stale when:

- document structure changes;
- review plan changes;
- a caller attempts to continue with a different explicit budget.

## Provider behavior

`document.markdown.goldmark` adds generic exact-range extraction:

```text
InspectRequest.range_start_line
InspectRequest.range_end_line
        ↓
validate range
        ↓
contentForLines(...)
        ↓
SelectedRange
```

Goldmark still does not own review planning, budgets, coverage, or cursors.

## Acceptance

The implementation was exercised against the real 2,271-line thread-scheduling draft with:

```text
review = true
review_max_lines = 300
related = true
```

Observed review plan:

```text
review units       = 33
largest unit       = 275 lines
oversized units    = 0
relation nodes     = 56
relation edges     = 56
relation warnings  = 0
```

Java / Go / CPython were decomposed by their existing heading hierarchy. Java's lifecycle section was further decomposed into `1.3.1 ... 1.3.6` units.

Cursor traversal consumed all 33 review units in order and ended with:

```text
complete = true
remaining_sections = 0
```

Every adjacent planned range was checked for exact continuity, so the covered article body contained neither overlaps nor gaps.

Acceptance status:

1. whole-document coverage reaches `complete=true`;
2. review units cover the outlined document body without overlap or gaps;
3. Java / Go / CPython are split into nested units instead of one huge payload;
4. the real article's largest review unit is 275 lines under the 300-line budget;
5. structurally unsplittable oversized units remain explicitly representable via `oversized=true`;
6. focused `section + include_content` remains compatible;
7. `related=true` remains compatible on the initial review call;
8. stale/cross-document cursor checks remain intact;
9. Core diff remains zero;
10. full tests and DevTool self-host verification are required before merge.

## Non-goals

This phase does not:

- split inside a paragraph/code block solely to meet a token budget;
- add a tokenizer dependency;
- add semantic retrieval;
- alter relation graph semantics;
- expose Markdown AST/provider-native methods to the Agent.

If real documents still contain oversized leaf sections after recursive heading decomposition, the next refinement should be **block-aware** splitting through a provider-neutral block/range contract, not raw line chopping.
