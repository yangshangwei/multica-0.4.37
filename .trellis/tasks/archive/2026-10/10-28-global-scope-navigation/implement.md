# Implementation plan

## Ordered work

- [x] Review selected user proposal, inspect current code and existing task, record PRD/design and two platform research notes.
- [x] Curate implement/check manifests, activate this task.
- [x] Shared lane: add global switch and tests/locales, remove sidebar duplicate; preserve collaboration filters/scroll and distinguish resumed vs explicit anchors.
- [x] Desktop lane: remembered workspace pointer with reset/migration/prune regressions; connect no-path resume, move global row outside canvas, adjust fixed sidebar/native toolbar geometry and shell tests.
- [x] Web/Core lane: identity-scoped last destinations, persistent shell/controller, account route-key restoration and tests; preserve explicit links and zero-workspace auth behavior.
- [x] Integrate shared interfaces; run focused regression suites, relevant package lint/typechecks and static diff/import review.
- [x] Verify real rendered roundtrip and fixed position in Web and Electron when available, with screenshots and runtime provenance. Record exact gaps instead of treating mocks as native proof.
- [x] Trellis check agent reviews complete task diff; fix substantiated findings and rerun affected checks.
- [x] Update navigation specs and task evidence, commit only this task's files with Lore trailers, archive only when acceptance is complete.

## Ownership

- Shared implement agent: `packages/views/layout`, affected shared collaboration detail and tests, shared navigation/restoration interface only if coordinated, layout locales.
- Desktop implement agent: `apps/desktop/src/renderer/src` only and desktop regression selectors in directly related E2E by coordination.
- Web implement agent: `apps/web`, `packages/core/navigation`, related exports and tests.
- Main: task artifacts, integration, E2E evidence, specs, final review and commit. Existing unrelated working changes remain intact.

## Commands

Use package-local `pnpm --filter <package> exec vitest run <files>` for focused tests; `pnpm --filter <package> typecheck` and `lint` for `@multica/core`, `@multica/views`, `@multica/web`, `@multica/desktop`. Inspect `package.json` and runtime ownership before commands. Actual Web E2E targets production Next; use the existing isolated Electron fixture for native acceptance. No AI provider/runtime needed for navigation fixtures.

## Risk gates

- Do not alter existing tab history/eviction contracts or use workspace mirror as last-space storage.
- Keep account access independent of workspace list loading/error/empty.
- Scope route/memento state to identity, avoid render-time cross-viewer flashes.
- Test explicit same-anchor refocus as well as ordinary resume.
- Fixed Sidebar uses viewport positioning: offset it with the new shell row, do not cover the switch when expanded.
- Tests asserting old Collaboration nav / Workspaces select must move to actual new switch semantics, without weakening underlying journeys.
- No backend changes, new dependencies, broad cleanup or staging of unrelated changes.

## Evidence

Source and actual-client verification completed; see [verification.md](verification.md). Implementation committed as cad3fa410; task ready for archive.
