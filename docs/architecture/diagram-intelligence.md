# Diagram Intelligence — Markdown ownership, Mermaid semantics, visual evidence

## Definition

DevTool gives Agents project-scoped structured engineering evidence. A diagram is a document asset, not a free-standing build system.

## Boundary

```text
Agent: diagram_context(path, analyze=true, render=true)
  → capability.diagram (thin Agent adapter)
  → service.diagram-context (inspection, aggregation, statuses)
      ├→ service.document-structure (configured Markdown provider / Goldmark)
      │   └→ fenced diagram locations, language, exact source
      ├→ service.diagram-intelligence (configured Merman provider)
      │   ├→ Merman LSP: document symbols and diagnostics in host .md coordinates
      │   └→ Merman Semantic Model: typed nodes, edges, subgraphs, participants
      └→ service.diagram-render (configured provider)
          └→ external CLI: Mermaid mmdc / PlantUML
```

- Core unchanged. No project-specific parser or renderer in BlogContent.
- A provider must be replaceable by TOML configuration, not by Core branches.
- Goldmark remains the **single owner** of Markdown headings, code-fence inventory and source positions. Marksman remains the owner of Markdown links/backlinks. Merman owns Mermaid-only language facts.
- The context service joins each Goldmark diagram to its smallest containing section and Merman's normalized graph, node line locations, diagnostics and cross-subgraph edge counts. Thus the Agent gets *section → diagram → node / edge* rather than two parallel raw LSP payloads.
- Merman analyzes embedded Mermaid fences with one whole-document LSP request per call; CLI parses each requested Mermaid source into a typed semantic model. The CLI never parses Markdown, and Merman's Markdown symbols never override Goldmark headings.
- `analyze=true` is an optional review intent, not an LSP method. Omit for quick inventory or render-only use. No Mermaid rename/writes are automatically applied, and no Provider methods are exposed as Agent tools.
- Graph output is bounded, with exact counts and `truncated`; missing provider or missing binary explicitly returns incomplete/unavailable, never a false clean diagnosis.
- Cross-subgraph edges and missing labels are **structural evidence**, not automatic claims of semantic correctness. The Agent must interpret graph relations against article content.
- `diagram_context` is a stable **validation/visual evidence** intent, not a proxy to `mmdc` or a substitute for `document_context`.
- No implicit package downloads or renderer installation. Unavailable renderer reports **unavailable**, never **valid**.
- Actual render is the syntax acceptance criterion; source heuristics do not count as renderer validation.
- External commands run without a shell, bounded by timeout and max source size.
- Artifact paths go to a provider-owned temporary location, not the content repository.
- User controls whether to analyze and when to render: `render=false` returns only diagram inventory; `render=true` verifies and generates SVG.
- Source-level semantic correctness (e.g., Go M/P mapping or GIL/OS Scheduler separation) remains an Agent review task; syntactically valid diagrams are not guaranteed accurate.

## Future asset/image context

An independent `asset_context` may add deterministic image metadata (dimensions, MIME, hash, EXIF presence, references, missing alt text, size, duplicates). Visual meaning belongs to multimodal model perception, not to metadata inference. Opt-in OCR and image recognition should be replaceable providers, not Core or default work.

## Usage

`diagram_context(objective="verify layer diagrams", path="docs/example.md", render=true)` returns diagram indices, fenced source ranges, content hashes, status and SVG artifact paths.

Configure `service.diagram-context` with `context.diagram.composite` and `service.diagram-render` with `diagram.render.cli`; install `mmdc` or `plantuml` only when rendering is needed. Missing executables produce `unavailable`; no source code is changed.


### Runtime dependencies

The configured Merman v0.8.0 executables are `merman-lsp` and `merman-cli` (see upstream releases). The provider does not install them. Set `cli_bin` and `lsp_bin` under `[extension.diagram-intelligence.settings]`. Keep `mmdc` as the site's existing display-fidelity renderer; Merman Semantic JSON is evidence for the Agent, not a new implicit image renderer.

Example:
```toml
[extension.diagram-intelligence.settings]
cli_bin = "merman-cli"
lsp_bin = "merman-lsp"
timeout_seconds = 40

[service.diagram-intelligence]
provider = "diagram.intelligence.merman"
```

`diagram_context(objective="review Java scheduling layers", path="article.md", analyze=true, index=1)` returns its canonical Markdown section, diagram fence location, graph nodes/edges/subgraphs, cross-layer relationship metrics, LSP-backed node source lines and diagnostic status. The diagram source itself is returned only when `include_source=true`. Rendering stays separate with `render=true`.
