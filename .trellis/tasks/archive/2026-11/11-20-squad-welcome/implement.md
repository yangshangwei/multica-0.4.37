# Execution plan

Approval: the user approved the presented reference-led design and explicitly requested planning followed by execution on 2026-11-20.

- [x] Inspect page, creation chooser, tokens, navigation, docs routes, tests and applicable specs; preserve unrelated worktree changes.
- [x] Converge requirements, design boundary and validation plan; no open product decisions.
- [x] Curate context and activate task.
- [x] Trellis implement agent: add the shared welcome component, integrate successful-empty presentation, add bilingual content and central docs slug, update meaningful state/navigation/carousel tests; run scoped lint/typecheck/tests.
- [x] Main session: prepare local app/browser evidence while implementation runs; correct only stale documentation for changed entry paths.
- [x] Trellis check agent: review exact task diff, permissions/state semantics, localization, a11y and package boundaries; fix scoped mechanical failures.
- [x] Capture one batched desktop/light, dark, English and narrow pass; compare supplied reference with visual-verdict and run impeccable detector once. Apply all material fixes as one batch, confirm at most once in the build thread.
- [x] Independent finish review over screenshots and task contract; resolve material findings.
- [x] Update applicable squad view spec and record results and remaining evidence limits.
- [x] Commit task-owned feature and verification files.
- [x] Archive and record the session (session 175 in journal-4.md).

## Commands
- pnpm --filter @multica/views exec vitest run squads/components/squads-page.test.tsx [welcome test if added] locales/parity.test.ts
- pnpm --filter @multica/views typecheck
- pnpm --filter @multica/views exec eslint squads/components/squad-welcome.tsx squads/components/squads-page.tsx squads/components/squads-page.test.tsx [other task files]
- pnpm --filter @multica/docs exec vitest run lib/docs-bundle/docs-anchor-parity.test.ts lib/docs-bundle/generate-docs-bundle.test.ts
- Targeted browser check using an isolated test workspace and the repository's existing TestApiClient/session setup. Record host, launch mode and commit. Do not claim dev browser checks as production E2E.
- git diff --check -- <task files>

No Go/server tests are required because no server behavior changes. Broaden TypeScript checks to Web/Desktop if shared package validation or integration evidence indicates a need.

## Continuation evidence — 2026-11-20

- Full-scope Trellis check: 93 Views tests, 5 Docs tests, 28 Core tests; lint, Views typecheck and scoped diff checks passed. See `research/check-20261120.md`.
- The reference comparison found unresolved local radius aliases. Corrected the two CSS-module declarations with the exact runtime `--radius` formulas; final computed radius is 18px. Impeccable detector returned `[]`; final visual verdict is 94.
- Production API, Web and Electron renderer builds passed in task-owned isolated directories. Final browser replay passed all three complete journeys with no skips or unexpected/flaky failures. Creation chooser, docs, templates, manual/keyboard examples, both locales, dark, 390px, 1024×768 and 200% Web/Electron content were verified. See `artifacts/qa/squad-welcome-20261120/README.md` and `summary.json`.
- The final replay corrects evidence capture timing with stable painted-layout and viewport assertions. The original functional pass and stale native screenshots are retained separately; product bytes did not change between those replays.
- The inherited malformed-2xx list fallback remains documented in `design.md`; API/schema and packaged Main/installer or real-agent acceptance remain outside scope.
- Independent finish review passed with all twelve product/test source hashes matching the evidence manifest; see `research/finish-review-20261120.md`. The task's API, Web, renderer processes/ports and synthetic database container were cleaned up; `cleanup.json` records their identities and confirms private-state removal.
