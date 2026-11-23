# Documentation diagram rendering research

Read-only product inspection on 2026-11-07. Only this research file was written. The existing in-app and public docs paths do not currently render Mermaid fences as diagrams, despite having reusable rendering components and installed dependencies.

## Current pipeline

1. Authors write `apps/docs/content/docs/*.zh.mdx`.
2. `scripts/generate-docs-bundle.mjs` calls `jsxToDirective`, retaining ordinary fenced Markdown and producing the committed Go bundle.
3. In-app `packages/views/docs/docs-content.tsx:197-209` unwraps every `pre > code`, detects language, then always renders `@multica/ui/markdown` CodeBlock. A `mermaid` fence therefore currently shows source.
4. The public Fumadocs config `apps/docs/source.config.ts:8` has empty `mdxOptions`. Both `apps/docs/app/[lang]/page.tsx` and `[lang]/[...slug]/page.tsx` spread the default MDX component map without a Mermaid component. The existing `apps/docs/components/mermaid.tsx` is currently unused by these routes.
5. The generator intentionally rejects unknown JSX. `jsx-to-directive.test.ts:116` explicitly expects `<Mermaid chart="graph TD;" />` to throw. Code fences are masked and restored verbatim by the converter.

## Recommended minimal contract

Use ordinary closed `mermaid` code fences as the single authoring format, with prose headings/captions outside the fence and Mermaid `accTitle` / `accDescr` where supported. Do not author Mermaid JSX in the MDX source, widen the JSX whitelist, add raw HTML, or add generated SVG assets alongside the same graph without a concrete need.

### In-app reader

Add exact language-token handling in the existing DocsContent fence branch. Reuse the leaf `../editor/mermaid-diagram` directly, not the editor barrel or full RichContent renderer. DocsContent already documents why docs and product rich content have different pipelines; diagrams do not justify importing issue mentions, HTML execution or streaming semantics into docs.

A small docs-only wrapper can own the prose spacing, diagram caption/discovery affordance and near-viewport loading. Existing `RichFenceBlock` provides Mermaid lazy mounting, but its module also imports the HTML preview and product code syntax dispatcher. For the smallest isolated dependency surface, use MermaidDiagram directly, or a narrowly scoped wrapper around existing LazyRichBlock if long multi-diagram pages require it. Keep non-Mermaid fences on the current CodeBlock path; `mermaidx` must remain plain code.

### Public Fumadocs: no new plugin is needed

The **installed primary source** already exposes the needed transformer:

- `apps/docs/node_modules/fumadocs-core/dist/mdx-plugins/index.d.ts:231-240`: exports `remarkMdxMermaid(options?)`, described as converting `mermaid` codeblocks into `<Mermaid />`.
- Its implementation at `dist/mdx-plugins/index.js:1297-1322` visits `code`, requires exact `node.lang === 'mermaid'` plus nonempty value, then produces an MDX flow element named `Mermaid` with a string `chart` attribute containing `code.trim()`.
- `fumadocs-mdx/dist/chunk-FSZMKRVH.js` inserts custom remark plugins before structure extraction and all rehype plugins, so the fence is converted before Shiki highlighting.

Minimal wiring:

```ts
import { remarkMdxMermaid } from 'fumadocs-core/mdx-plugins';
// Existing defineConfig call:
mdxOptions: { remarkPlugins: [remarkMdxMermaid] }
```

Import and bind the existing `Mermaid` component in the MDX component maps of **both** `[lang]/page.tsx` and `[lang]/[...slug]/page.tsx`. Keep those maps in sync. No new dependency, AST transform, source extraction from highlighted spans or changes to `jsxToDirective` are needed. The bundle generator reads source fences independently of this compile-time MDX transformation.

## Existing leaf reuse and limits

`packages/views/editor/mermaid-diagram.tsx` already provides:

- Dynamic Mermaid import, base theme from host CSS variables with canvas conversion from oklch to sRGB.
- `securityLevel: strict`, `htmlLabels: false`, `suppressErrorRendering: true`.
- Rendered SVG in an empty `sandbox=""` iframe; no script or same-origin allowance.
- Natural-size rendering, horizontal scrolling, drag-to-scroll versus tap distinction.
- Copy-source and expand controls; full-screen dialog via MermaidViewer.
- Stable reserved loading height and session layout cache.
- Theme mutation/media-query re-render while retaining open viewer, zoom and old image until replacement is ready.
- Parser-error explanation with source fallback and copy action.

`mermaid-viewer.tsx` provides focus trap/restore, Escape, scroll lock, fit/actual-size, pan, keyboard/pinch zoom, source toggle and `.mmd`/SVG/PNG export. It uses host-side controls without relaxing iframe sandbox.

Important integration limits:

1. `editor/styles/mermaid.css` is entirely scoped under `.rich-text-editor` for inline styling; MermaidDiagram does not itself import that stylesheet. Import only the relevant style file and scope only the diagram wrapper, or provide a docs-specific equivalent selector. **Do not wrap the entire DocsContent in editor styles**, which changes all prose, headings and code. ZoomCanvas imports its own CSS.
2. The in-app theme currently supplies only primary fill/border/text, line color and `fontFamily: inherit`. Public Mermaid supplies a more complete semantic theme (clusters, notes, actors, signals). State/sequence/subgraph diagrams therefore require actual light/dark capture; do not assume node text alone proves all graph elements have contrast.
3. In-app iframe documents do not currently set a concrete body font family even though Mermaid's theme fontFamily is `inherit`. Because an iframe does not inherit the host document's fonts, verify rendered typography/CJK metrics and, if needed, pass the concrete computed host font to the iframe or theme rather than using another arbitrary font.
4. Public Mermaid renders strict-mode SVG directly into a div and has no zoom/fullscreen/source UI. Its code handles render() rejection, but a failure of the outer dynamic import is not caught. It uses `fontFamily: inherit` and some light/dark tokens, but should set `htmlLabels: false` if export is introduced.
5. Public Mermaid IDs currently strip only `:` from React useId; the in-app leaf strips all characters outside `[a-zA-Z0-9_-]`. Match the robust sanitization if editing public code.
6. Existing in-app inline scroll uses `justify-content: center`; very wide diagrams need narrow-viewport verification so the leading edge remains reachable. Existing full-screen view is already suitable for large graphs.
7. `apps/docs` depends on core UI but **not** `@multica/views`; importing the editor leaf directly into public docs would introduce application i18n/context and a package dependency. Reuse each surface's existing renderer and common authoring format instead of coupling docs-site to views for this task.

## Minimal expected change set

- `packages/views/docs/docs-content.tsx`: exact Mermaid fence dispatch.
- Optional `packages/views/docs/docs-diagram.tsx` and scoped styles: local presentation/lazy-loading integration.
- `packages/views/docs/docs-content.test.tsx`: one Mermaid handoff case plus exact-token/non-Mermaid and preservation checks.
- `apps/docs/source.config.ts`: existing remarkMdxMermaid plugin.
- `apps/docs/app/[lang]/page.tsx`, `apps/docs/app/[lang]/[...slug]/page.tsx`: map Mermaid.
- `apps/docs/components/mermaid.tsx`: bounded loading/error/overflow/accessibility improvements if required for the public surface.
- Selected `.zh.mdx` pages: diagrams paired with short explanations, existing headings/anchors preserved.
- `server/internal/docs/content/**`: regenerated only after authors finish.

Any broad changes to shared Mermaid leaf/style affect task/chat/editor surfaces. Prefer local wrapper changes unless a reproduced shared defect requires the shared fix and its canonical regression test.

## Verification with useful boundaries

- Bundle tests: confirm Mermaid fences survive source conversion and generation unchanged (including `<` and quoted labels), while arbitrary Mermaid JSX remains rejected.
- In-app integration: exact Mermaid fence invokes diagram with original source; non-Mermaid and `mermaidx` retain code; raw HTML remains blocked.
- Reuse existing leaf/viewer suites for strict/sandbox, errors, theme, focus, zoom, export and touch interactions; do not copy their full matrix into docs tests.
- Public transformer: use the installed plugin with a representative fence; actual docs build must compile all added Mermaid graph strings into bound components.
- Run real Mermaid parser/render against **every authored diagram**; mocked component tests cannot establish valid diagram syntax or readable labels.
- Capture the real in-app and public readers at wide/light, wide/dark and narrow sizes. Check all diagram types chosen, clipping, overflow, loading/anchor stability and full-screen keyboard operation. For a large graph, verify initial leading edge and all labels are reachable.
- Confirm browser console has no Mermaid parse failures; no network/CDN is required for the lazy engine.
- Run docs package tests/typecheck/build, scoped views tests/typecheck/lint, generator/byte parity/link validation and Go embed checks appropriate to changed bundles.

## Existing useful tests

- `packages/views/editor/mermaid-diagram.test.tsx`: strict config + no HTML labels, natural-size sandbox, source copy, tap/drag separation, theme retention and invalid-source fallback.
- `packages/views/editor/mermaid-viewer.test.tsx`: dialog focus/Escape/scroll-lock, fit and zoom limits, pointer/keyboard handling, source toggle and export options.
- `packages/views/rich-content/rich-fence-block-ssr.test.tsx`: stable server/client first frame in the existing lazy shell.
- `apps/docs/lib/docs-bundle/jsx-to-directive.test.ts`: unknown JSX guard and fence preservation.
- `packages/views/docs/docs-content.test.tsx`: code, directives, deep headings, links, assets and raw HTML isolation.
