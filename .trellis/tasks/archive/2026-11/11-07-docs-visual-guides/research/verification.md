# Diagram verification

## Scope and environment

- Current working tree on `codex/cross-workspace-collaboration`, extending the prior docs audit.
- 12 Mermaid diagrams in 10 Chinese pages; standard fences preserved in the 44-page embedded bundle.
- Native checks mount the real `DocsPage`/`DocsContent` and leaf/viewer inside an isolated Electron BrowserWindow with the actual desktop renderer CSS/fonts. Query data is prefilled from the generated bundle; no authentication, user database writes or daemon/AI execution occurs. This is renderer integration evidence, not a full application login/navigation journey.
- Public checks use the actual optimized Next/Fumadocs build at localhost:4057, with light/dark and narrow viewports.
- Impeccable context loaded once; mode Read; inherited documentation design. Mechanical detector returned `[]`. Independent visual review requested four bounded fixes; see visual-review.md.

## First pass

- 12/12 diagram sources parsed by the installed Mermaid package.
- 24 native SVG outputs and 24 public SVG outputs produced across light/dark, no uncaught page errors.
- Native 520px viewport revealed whole-page horizontal overflow. DOM diagnosis proved the outer docs flex child's min-width:auto caused a 768px minimum width; min-width:0 restored width520 without shrinking SVG text.
- Native dark iframe canvases were white although SVG and body were transparent. Host color-scheme dark versus srcDoc normal caused the Chromium canvas mismatch. Inheriting color-scheme fixed the temporary DOM witness.
- The first native light squads screenshot captured a pre-paint iframe; the tall tasks screenshot was clipped by its scrolling ancestor. Confirmation capture waits for visible inner SVG/fonts and expands the capture window for tall figures.
- Independent semantic review corrected the deferred promotion label: due time alone does not admit a queued task; runtime and pending-slot conditions remain.

## Automated checks

- Focused Views tests: 85 passed before the final visual fixes; final affected results recorded below when complete.
- Docs tests: 64 passed, including Mermaid source/transformer parity and ordinary language preservation.
- Views/Docs typecheck and scoped ESLint passed in implementation and independent review.
- Initial docs production build passed (104 static pages).
- Bundle byte comparison: 103 files equal; 347 internal links and 55 Markdown image references resolve.
- Go embedded docs tests passed.

## Confirmation

- One coordinated product fix batch resolved the diagnosed canvas/width defects, strengthened semantic shape outlines and removed incidental blue cluster fills from public diagrams.
- The first confirmation harness attempt waited for requestAnimationFrame inside an empty sandbox and did not complete; it was stopped. The capture procedure now verifies inner SVG layout and settles painting from the host without relaxing the iframe sandbox.
- Final confirmation: 24 native + 24 public diagram renders, zero uncaught page errors; all diagram screenshots refreshed at the original paths.
- Native 520px light/dark and 390px dark show no document overflow; the 390px sequence diagram scrolls to its right edge within the diagram.
- Keyboard opens the native viewer; zoom changes its transform, fit restores framing; source is visible; Escape closes and restores focus. Theme changes keep the viewer open and update its sandbox document.
- Invalid Mermaid source shows its error/source fallback and subsequent prose remains visible.
- Post-fix targeted Views tests: 66 passed (same core suites, with the theme regression first observed failing, then passing); Views/Docs typechecks, scoped lint and diff check passed. The existing additional 19 lazy/SSR cases passed earlier and their code did not change.
- Final public production build passed (104 generated pages); byte comparison remains 103 files, 347 internal links and 55 image references with zero failures.
- Capture records: rendering-results.json; visual disposition recorded separately in visual-verdict.md. Screenshot assets are retained locally under research/screenshots/ and ignored by Git.

## Reproduction

Copy preview.html to apps/desktop/src/renderer/docs-visual-check.html and preview.tsx to apps/desktop/src/renderer/src/docs-visual-check.tsx temporarily. The fixture uses the actual desktop Vite server at :5666 and generated bundle files through Vite filesystem URLs. Start the built docs server at :4057. Run verify-rendering.cjs from the repository root. Its isolated Electron main starts no daemon or account session. Remove those two temporary renderer entries afterward. This fixture records the current checkout location; update the filesystem URL in preview.tsx if the checkout moves.
