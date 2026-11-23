# Repository Findings — Skills Welcome

Inspected on 2026-11-21. This is repository evidence for planning, not implemented behavior or a claim that product checks have passed.

## Supplied Reference and Existing Visual System

- The user supplied the welcome-page screenshot and explicitly invoked Impeccable. The original image is retained as `reference.png` beside this file (3116×1216 source pixels).
- `packages/views/agents/components/agent-welcome.tsx` already implements the reference's composition with static examples, a manual three-position carousel, hidden/inert inactive cards and workspace routes.
- `packages/views/agents/components/agent-welcome.module.css` provides a current visual reference for grid proportions, clipped track, brand-tinted dotted scene, semantic runtime tokens and reduced motion. Read and adapt the relevant patterns; do not modify that feature or extract a global abstraction for this change.
- `.trellis/spec/views/frontend/component-guidelines.md:139` records analogous first-use state/navigation/accessibility contracts. Its runtime-CSS-token section at `:201` warns that `@theme inline` aliases are not guaranteed runtime variables.
- `packages/ui/styles/tokens.css` owns the role-named type scale and theme aliases. Follow its actual runtime tokens and avoid arbitrary type/color replacements.

## Collection State and Query Ownership

- `packages/views/skills/components/skills-page.tsx:710` reads the skill query separately from agents, members and runtimes. `:936` computes the raw total and `showEmpty`; `:970` gives list errors precedence, `:994` handles loading and `:998` chooses the empty branch.
- The current empty content is the small `EmptyState` at `:483`; it offers creation and market discovery. The new surface replaces this branch, rather than creating a route.
- `:937` onward computes category emptiness using unfiltered facets. The populated-directory branch retains search, category navigation, counts, list/card views, selection and virtualized scrolling. Preserve it.
- `packages/core/workspace/queries.ts:165` scopes the list query to workspace and forwards the AbortSignal; template queries begin at `:172` and are separate.
- `server/internal/handler/skill.go:400` lists saved workspace summaries. `server/internal/handler/skill_template.go:26` explicitly lists catalog templates without materializing skills. Saved built-in-role instances count as real collection content; catalog-only templates do not.
- `packages/core/api/client.ts:5778` uses `parseWithFallback(..., EMPTY_SKILL_LIST)`. `packages/core/api/skill-template-client.test.ts:270` confirms malformed reads can become `[]`. Successful Query status cannot independently validate the raw response; preserve and disclose this inherited API limit.

## Real Creation, Navigation and Focus

- `packages/views/skills/components/skills-page.tsx:205` defines the header actions; market uses `paths.marketplaceTab("skills")`, docs use `paths.docsPage(DOCS_SLUGS.skills)`.
- `packages/views/skills/components/create-skill-dialog.tsx:70` defines the method union; `:77` lists manual, template, local, URL and runtime. This is five ways, whereas the supplied screenshot shows only four.
- The same dialog owns import safety, local-picker user gesture, template editor retention and recovery. The welcome must open it rather than reproduce those forms.
- The creation session retains its originating ref in `skills-page.tsx:733`; `:1187` mounts the workspace-keyed dialog and resolves final focus to a connected opener or the header create button.
- `.trellis/spec/views/frontend/skill-market-discovery.md` requires an empty library to stay in place until explicit market navigation. Retain that contract and the distinction between template discovery and saved instances.

## Product Copy Constraints

- `apps/docs/content/docs/developers/conventions.zh.mdx:138`: Chinese prose keeps `skill` lowercase; section headings may say Skills. Use 智能体, 运行时 and 工作空间 consistently.
- `apps/docs/content/docs/skills.zh.mdx` documents all five creation methods, source URL limits and runtime-copy snapshots. One skill may be explicitly added to multiple agents; edits apply from subsequent executions, not to already running work.
- Keep the screenshot's headline. Refine illustrative/update text for those documented facts; no new claims of automatic assignment, execution or continuous runtime synchronization.
- The current page exposes a supporting-data warning for failed member/agent/runtime queries. The illustration must use local synthetic labels and must not require those queries to succeed.

## Existing Test and Acceptance Seams

- `packages/views/skills/components/skills-page.test.tsx:44` mocks query data/loading/error but does not yet model explicit success/paused states. Update that rig when adding a success predicate.
- The page suite's market-entry group at `:242` verifies explicit market navigation and empty-library retention; later groups cover built-in identity, row/source navigation and category/view wiring. Extend these contracts rather than moving them into an app test.
- Actual creation methods are already covered by `create-skill-dialog.test.tsx` and their panel suites. The welcome suite should prove the entry/focus wiring, not repeat the import matrix.
- `packages/views/locales/parity.test.ts` is available for bilingual key parity. The views package exposes `vitest run`, `tsc --noEmit` and `eslint .` scripts.
- `e2e/skill-library-accessibility.spec.ts` is the relevant directory regression; add a focused welcome journey for compiled layout, real navigation, manual carousel and collection transition.
- `.trellis/spec/web/frontend/e2e-run-environment.md:9` requires production Web for browser acceptance. Keep environment/process provenance and isolate synthetic workspace fixtures. A shared renderer or DOM-only assertion is not packaged Electron acceptance.

## Decision Inventory

- The supplied reference resolves composition; the existing product resolves tokens, route APIs, five real creation methods and update semantics.
- The recommended scope preserves the existing unfiltered-empty entry point and normal populated collection. New persistence, permanent banners, native mobile and backend changes are unnecessary for the request.
- Task creation/planning has explicit consent. The complete proposed plan still requires the user's subsequent approval before implementation. No product code has been edited in this planning turn.
