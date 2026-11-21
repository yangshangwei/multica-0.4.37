# Implementation plan

## Approved scope and change boundary

The user approved the previously presented unified left-sidebar proposal and explicitly requested design plus implementation. The behavior gap is a workspace-only left directory and a separate account workbench; the change belongs in shared sidebar presentation and both platform shells. It does not belong in backend ownership, workspace identity stores or task-detail controls.

Expected files: shared `layout/app-sidebar*`, a Core navigation-preferences leaf/tests/export, collaboration notifications/workbench wiring/tests, layout locales, Web shell/account/dashboard wiring/tests and Desktop shell/toolbar/tests. Spec/docs and a focused E2E journey complete delivery. Only necessary route/icon/shared UI export adjustments may extend this list; no dependencies or backend changes.

## Ordered work

- [x] Inspect incumbent navigation, restoration contracts and screenshot; create task and capture existing dirty locale baseline.
- [x] Finish bounded Web/Desktop shell research and review the design against actual providers.
- [x] Curate implement/check manifests, review final artifacts and activate task.
- [x] Shared implementation lane: regression tests, Core group preferences, account-aware sidebar, scoped New menu and notifications, locale parity, workbench deduplication.
- [x] Web lane: regression tests, route/shell integration, exact resume and no-workspace handling.
- [x] Desktop lane: regression tests, sidebar mounting across scopes, native/history geometry and gated workspace effects.
- [x] Integrate focused tests; run affected lint and type checks, resolve all task-owned failures.
- [x] Trellis independent quality review and safe local fixes.
- [x] Production build plus Web/native browser journey and batched desktop/compact visual QA, with at most one corrective round.
- [x] Update affected specs/docs, record evidence and final remaining risks.
- [x] Commit only task-owned changes with Lore decision trailers; finish/archive/journal under task ownership.

## Checks

Use repo-installed pnpm/Vitest, selecting exact test files while iterating. Final affected checks: `pnpm --filter @multica/core typecheck`, `pnpm --filter @multica/views typecheck`, `pnpm --filter @multica/web typecheck`, Desktop's declared typecheck script; lint changed TS/TSX in each package; locale parity; `git diff --check`. Broaden only for failures or unresolved integration concerns.

Before commit, compare dirty baseline, inspect every staged path/hunk, and leave unrelated market/docs/env/e2e changes untouched. Locale files require staging only our added hunks. Generated artifacts or runtime evidence must not expose auth tokens or credentials.
