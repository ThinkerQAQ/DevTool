# Diagram Intelligence — source → validation → render evidence

## Definition

DevTool gives Agents project-scoped structured engineering evidence. A diagram is a document asset, not a free-standing build system.

## Boundary

```text
Agent: diagram_context(path, render=true)
  → capability.diagram (thin Agent adapter)
  → service.diagram-context (inspection, aggregation, statuses)
      ├→ service.document-structure (configured Markdown provider / Goldmark)
      │   └→ fenced diagram locations, language, exact source
      └→ service.diagram-render (configured provider)
          └→ external CLI: Mermaid mmdc / PlantUML
```

- Core unchanged. No project-specific parser or renderer in BlogContent.
- A provider must be replaceable by TOML configuration, not by Core branches.
- The existing Markdown parser owns diagram-fence detection, preserving exact source and line anchors.
- `diagram_context` is a stable **validation/visual evidence** intent, not a proxy to `mmdc` or a substitute for `document_context`.
- No implicit package downloads or renderer installation. Unavailable renderer reports **unavailable**, never **valid**.
- Actual render is the syntax acceptance criterion; source heuristics do not count as renderer validation.
- External commands run without a shell, bounded by timeout and max source size.
- Artifact paths go to a provider-owned temporary location, not the content repository.
- User controls when to render: `render=false` returns only diagram inventory; `render=true` verifies and generates SVG.
- Source-level semantic correctness (e.g., Go M/P mapping or GIL/OS Scheduler separation) remains an Agent review task; syntactically valid diagrams are not guaranteed accurate.

## Future asset/image context

An independent `asset_context` may add deterministic image metadata (dimensions, MIME, hash, EXIF presence, references, missing alt text, size, duplicates). Visual meaning belongs to multimodal model perception, not to metadata inference. Opt-in OCR and image recognition should be replaceable providers, not Core or default work.
