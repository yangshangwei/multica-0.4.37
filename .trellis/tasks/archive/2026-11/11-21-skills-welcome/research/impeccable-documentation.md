# Skills welcome documentation

Date: 2026-11-21. Disposition: preserve the incumbent Multica visual system.

## Preservation decision

This finished surface is the approved Operate-mode extension described in
`prd.md` and `design.md`. It lives in the existing Skills collection on Web and
Desktop. The supplied reference governs its composition; the existing Multica
components, runtime tokens, fonts and role-named type scale govern its visual
implementation.

The documenter role and `impeccable/reference/document.md` were read directly.
Their ordinary-extension preservation path applies: no new world or system
change was approved. Root `PRODUCT.md`, `DESIGN.md` and
`.impeccable/design.json` are absent and remain absent. Creating them here would
promote one collection state into a new product-wide authority. No global
tokens, component rules or speculative visual prohibitions were introduced.

The durable executable contract already lives in
`.trellis/spec/views/frontend/skill-welcome.md` and its frontend index entry.
Those files, the approved PRD/design, and the existing visual system are
preserved. This report records checked evidence; it does not duplicate the
contract as another normative token catalogue.

## Incumbent system checked against the finished build

Paths in this report are relative to the repository root unless stated
otherwise.

| System evidence | Finished welcome evidence | Preservation result |
| --- | --- | --- |
| `packages/ui/styles/tokens.css`, light and `.dark` runtime definitions | `skill-welcome.module.css` uses `--surface`, `--surface-border`, `--muted`, `--muted-foreground`, `--brand`, `--info-foreground`, `--success-foreground`, `--menu-shadow`, `--surface-shadow` and `--radius` | The scene and panels resolve the existing runtime variables. No hardcoded replacement palette or `--color-*` theme aliases were introduced as runtime values. |
| `packages/ui/styles/tokens.css`, role-named type scale | Heading uses `text-display`; introduction uses `text-body-lg`; captions use `text-title-sm`; scene copy uses `text-body`/`text-caption`; example marks use `--text-micro` | The observed size roles remain 36/40px, 15/22px, 16/24px, 14/20px, 12/16px and 11/15px respectively. The introduction's local 28px leading matches the existing agent welcome. These are references to the established scale, not new tokens. |
| `apps/web/app/globals.css`, `apps/web/app/layout.tsx`, `apps/desktop/src/renderer/src/globals.css`, and shared `base.css` | The welcome inherits the Inter-based UI stack and system CJK fallbacks; illustrative file content uses `--font-mono` | No surface-specific font import, family override or marketing type ramp was added. Both apps import shared UI tokens and base styles. Web's `.landing-light` overrides are scoped to the separate landing surface. |
| `packages/views/agents/components/agent-welcome.tsx` and `.module.css` | `skill-welcome.tsx` and `.module.css` retain the same quiet introduction, clipped scene track, semantic blue dotted ground, rounded panels and manual controls | The existing welcome composition is extended locally. The common 100rem layout bound, 34rem intro bound, 28rem track, 1.5rem track gap, 60rem two-column container query and 24rem narrow adjustment remain recognizable. Skills-specific scene height, wrapping and padding accommodate five methods. |
| `packages/ui/components/ui/button.tsx` | The create action uses `Button`; docs use `buttonVariants({ variant: "ghost" })`; arrows use outline `Button` | Primary, ghost and outline treatment, focus ring, disabled behavior and theme inversion remain owned by shared primitives. Local 44px targets follow the existing agent welcome. The primary is dark in light mode and follows the incumbent light fill in dark mode. |
| `packages/views/skills/components/skills-page.tsx` and existing collection header | Welcome replaces only the successful empty directory branch; `showCreate={!showWelcome}` suppresses the duplicate header create action | Existing page chrome and explicit template-market action remain. Loading, list errors and populated/filter recovery remain collection concerns. No welcome route, store or persisted completion state was introduced. |

The outer example cards use the incumbent `--menu-shadow`, a border and
`--radius × 1.8`; inner illustrative panels use `--surface-shadow` and
`--radius × 1.4`. The brand-tinted dotted ground uses the same 5% fill, 15% dot
mix and 1.25rem grid as the agent welcome. These are established source patterns,
not a new material or elevation vocabulary.

## Local direction and product truth retained

- The quiet introduction keeps the compact Skills badge, headline, muted
  explanation, one primary create action and book/docs link. The badge, dotted
  ground and nested example panels are explicit local reference obligations;
  this pass does not turn them into requirements for every Multica surface.
- The first scene contains all five real methods: manual, template, local,
  URL and runtime. Its labels and icons match `create-skill-dialog.tsx`; the
  highlighted URL row remains illustrative. The real create action opens the
  existing creation session rather than treating a mock method row as a control.
- All three scenes carry the localized Example label. Scene content is static;
  the apparent assignment row, agent names and file update perform no business
  reads or writes. One-to-many assignment is explicit rather than automatic.
- English and Simplified Chinese copy in the existing `skills.json` namespaces
  says that edits apply to future executions, leave already-running work
  unchanged, and that runtime copies are file snapshots without continuous
  synchronization. This agrees with `apps/docs/content/docs/skills.zh.mdx`.
- The docs link resolves through `paths.docsPage(DOCS_SLUGS.skills)` to the
  current workspace's `/{slug}/docs/skills`; the retained market action resolves
  through `paths.marketplaceTab("skills")` to
  `/{slug}/marketplace?type=skills`. `useWorkspacePaths()` binds both to the
  route's workspace. Shared `AppLink` preserves the Desktop shareable absolute
  href while routing the original workspace path through its navigation adapter.
- Slides are selected manually without a timer. Inactive cards remain faded
  previews and are both `aria-hidden` and `inert`; position buttons expose
  pressed state, arrows disable at endpoints and a polite status names the
  selection. `key={wsId}` resets selection on workspace changes.
- Container queries stack the introduction and primary action before the
  examples in narrow space. The scrollable root, clipped track, wrapping scene
  text and narrow padding preserve reachability. Reduced motion removes track
  and opacity transitions; position-mark transitions also respect it.

## Finished evidence inspected

Read the full welcome TSX/CSS module and its declaration, the relevant SkillsPage
header/query/render branches, the actual method chooser, both welcome locale
objects, workspace path builders, shared navigation, shared button, shared tokens
and the existing agent welcome. No product, test, spec, runtime-token or shared
primitive file was edited by this documentation pass.

Visually inspected the supplied `research/reference.png` and these finished
captures under `.impeccable/review/skills-welcome/`:

- `desktop-en.png`: quiet introduction, single primary create action, five-method
  scene, complete active card and faded adjacent preview in the existing shell.
- `desktop-dark-zh.png`: existing dark palette, localized type and shared
  primary-action inversion remain coherent with the shell.
- `mobile-390-en-examples.png`: wrapping methods and caption, full active card,
  and reachable position/arrow controls after vertical scrolling.
- `native-narrow-scrolled.png`: actual Electron pane preserves the assignment
  scene and reachable manual controls in its scroll container.
- `reuse.png`: illustrative SKILL.md edit, future-run claim, running-work
  qualification and visible runtime-snapshot language.

All 16 PNG copies named in
`artifacts/qa/skills-welcome-20261121/review-screenshots.json` were independently
checked for PNG dimensions and SHA-256 agreement with that inventory. Only the
five captures listed above were visually sampled in this pass. The independent
finish review in `research/impeccable-finish-review.md` reports `ship` and no
material fixes after inspecting the full inventory. The existing detector
artifact at `artifacts/qa/skills-welcome-20261121/impeccable-detector.json` is
`[]`; it was read, not rerun.

## System summary

1. Palette: existing theme-aware neutral canvas and surfaces, restrained brand
   blue, and semantic success/info ink.
2. Type: incumbent Inter/CJK UI stack and role-named scale; mono only for the
   illustrative skill/file content.
3. Hierarchy: quiet introduction, one create action, then explanatory examples
   within the existing collection shell.
4. Material: existing borders, radius multipliers and soft semantic shadows;
   active scene remains opaque while the neighboring preview fades.
5. Behavior: manual accessible scenes, actual workspace navigation, container
   reflow, vertical scrolling and reduced-motion support.

## Drift and verification limits

No unresolved visual-system drift or material defect was found in the sampled
finished surface. No craft-floor defect was canonized as a rule, and no unrelated
incumbent drift was repaired. The API's already-documented malformed-response
fallback to an empty list remains an inherited functional limitation; it is not
evidence of historical first use and is not a visual-system decision.

This pass changed documentation only. It verified source agreement, sampled the
finished captures and checked inventory integrity; it did not rerun application
tests, browser interactions or the mechanical detector. Those remain the parent
verification lane and the existing finish-review evidence.
