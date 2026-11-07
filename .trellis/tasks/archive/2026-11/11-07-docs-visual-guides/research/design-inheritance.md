# Diagram design inheritance

Task: `11-07-docs-visual-guides`
Mode: **Read** — a local extension of existing documentation.
Evidence baseline: current working-tree implementation and the three existing task screenshots below, inspected on 2026-11-07. This is a design record, not a new visual review or proof of a deployed release.

## Scope and visual authority

The direction in `research/design-brief.md` remains authoritative for this task: use diagrams to explain a responsibility, decision or sequence, retain short labels and nearby explanatory prose, and preserve readable vector output. The brief records that Impeccable context was already loaded for this session. No additional context initialization or visual iteration was run for this record.

The application reader and public documentation site intentionally have different visual identities. The sources below, rather than a new shared palette or typography scheme, define how each renders the same diagram content.

| Surface | Typography and reading structure | Color and frame inheritance |
| --- | --- | --- |
| Application reader | Retain `DocsPage`'s centered `max-w-3xl` article and existing prose styles. The editor Mermaid renderer resolves the host's computed font family into the SVG; it does not substitute a diagram font. The web host uses Inter with the existing Chinese-first platform fallback stack. Other hosts keep their own computed stack. | Retain product semantic tokens and their light/dark variants. `docs-diagram.css` makes the diagram canvas and toolbar use `--background`; the shared diagram wrapper retains its quiet one-pixel `--border` and existing radius. Changes stay scoped to `.docs-diagram`, without attaching editor prose styles to the article. |
| Public documentation | Retain the editorial layout and existing body rhythm: Inter with Chinese-first fallbacks, 15.5px body text and 1.7 line height; Source Serif 4 remains the heading/lede face. Mermaid reads the body's computed sans family, so diagram labels do not acquire the heading serif. | Retain `apps/docs/app/global.css`'s warm near-white paper and warm ink, warm dark theme, indigo interaction accents and ruled dividers. Mermaid reads the public host's tokens at runtime. Its existing rounded frame uses `border-border` and `bg-background`; the source disclosure keeps the same quiet separator. |

Sources: `packages/views/docs/docs-page.tsx`, `packages/views/docs/docs-content.tsx`, `packages/views/docs/docs-diagram.tsx`, `packages/views/docs/docs-diagram.css`, `packages/views/editor/styles/mermaid.css`, `packages/views/editor/mermaid-diagram.tsx`, `apps/web/app/globals.css`, `apps/docs/components/mermaid.tsx`, `apps/docs/app/global.css`, and `apps/docs/app/[lang]/layout.tsx`.

## Semantic strokes and reading behavior

Node, decision, actor and group boundaries carry meaning. Both renderers use the existing neutral `--muted-foreground` for meaningful outlines instead of the much quieter decorative `--border`. Node/group fills derive from `--muted` or `--background`; labels derive from `--foreground`. This preserves legibility in both themes without introducing a diagram-specific color language. Content must not supply hardcoded colors or Mermaid initialization directives. Existing renderer-specific theme settings remain owned by their respective hosts.

Keep each SVG at a readable natural width. A wide diagram scrolls inside its container; it must not widen the article or shrink labels to fit the column. Preserve `min-w-0` at the reader's flex boundary and diagram wrapper. Small diagrams remain centered at their natural size.

The application reader reuses `MermaidDiagram` and `MermaidViewer`. Copy/open controls stay visible in the reserved top area, outside the labels; the diagram scroller does not carry the toolbar away. Preserve the existing zoom, source, export, Escape and focus-return behavior. The docs wrapper uses 32px control targets and 44px targets for coarse pointers, with token-based hover and focus states. Public docs retain their own keyboard-focusable horizontal scroll region and source disclosure; this record does not imply they have the application's full-screen viewer.

No new font, dependency, raster asset or decorative animation is required. Mermaid is already declared in both consuming packages; the public compiler uses the existing Fumadocs Mermaid plugin. The extension reuses those capabilities and the established viewer rather than introducing another rendering or zoom system.

## Existing screenshot evidence

The supplied screenshots are cropped diagram captures, so observations are limited to those regions. They are sufficient to record inheritance; they do not independently prove whole-page typography, every responsive layout or interactive behavior.

| Existing capture | Observed inheritance |
| --- | --- |
| [Application, light, concepts](screenshots/native-light-concepts-0.png) | Light neutral canvas, dark labels, restrained node/edge strokes and a much quieter rounded outer frame. Copy/open controls remain above the diagram. The narrow graph is centered without filling the whole width. |
| [Application, dark, execution sequence](screenshots/native-dark-how-multica-works-0.png) | Dark host canvas, dark neutral actor fills, light text and legible actor/edge strokes. A horizontal scrollbar and the partial right edge show natural-width overflow rather than compressed labels. The canvas has no white iframe backing in this capture. |
| [Public site, light, architecture](screenshots/public-light-developers-architecture-0.png) | Warm paper canvas, warm neutral group surfaces and ink-colored labels. Server and execution-computer boundaries remain stronger than the outer frame. The wide diagram continues beyond the visible region; this crop does not show the source disclosure or whole article. |

## Maintenance constraints to preserve

1. Author ordinary fences with language exactly `mermaid`, an `accTitle`, an `accDescr`, and adjacent prose explaining the boundary or decision. Keep one purposeful question per diagram. Do not add JSX imports to pages or broaden the bundle generator's closed JSX whitelist.
2. Preserve the same diagram source through the Chinese bundle and public MDX pipeline. Chinese `.zh.mdx` edits require regeneration of the committed `server/internal/docs/content/` output. Keep `remarkMdxMermaid` and the `Mermaid` binding at both public page entry points.
3. Keep the application's near-viewport `LazyRichBlock` and existing renderer/viewer ownership. Preserve strict Mermaid rendering and the empty iframe sandbox. Both inline and viewer iframe documents must receive the host's computed `color-scheme`, so transparent SVGs do not acquire a white backing in dark mode.
4. Resolve actual host fonts and supported concrete colors for SVG/export output. Mermaid's color parser needs the existing conversion from modern CSS colors. Do not replace the real font stack with unresolved CSS inheritance inside an isolated SVG or export.
5. Preserve source/error fallbacks and accessible controls. A failed render must leave the source available. Theme changes, scrolling and zoom must retain text legibility.
6. Validation belongs to the existing task checks: parse diagrams and preserve their source; compile both public routes; exercise the real app reader; check light/dark and narrow layouts, error/source fallback, keyboard/zoom and focus return. Keep wiring coverage in `docs-content.test.tsx` and `mermaid-fences.test.ts`; renderer/viewer behavior remains in their existing suites.
7. Wait for the actual iframe SVG and fonts to paint before capturing evidence. SVG text present in `srcdoc` alone is insufficient. Capture tall diagrams within their scrolling ancestor or in sections. This record adds no screenshot cycle or implementation changes.

These constraints are inherited from `.trellis/spec/docs/frontend/docs-bundle.md`, especially its Mermaid section, and the cited implementations. This task-local record does not replace that spec.

## Existing design-document gap

`PRODUCT.md` and `DESIGN.md` are absent at the repository root, `apps/docs/`, and `packages/views/docs/`. This is an existing documentation gap, not evidence that either surface lacks an established visual system. The existing tokens, CSS, components and task brief provide sufficient visual authority for this extension.

No global product/design file or Impeccable sidecar was created or modified. A separate, explicitly scoped design-documentation task can capture the broader systems later; their absence does not block this local Read-mode extension. This record's only deliverable is `research/design-inheritance.md`.
