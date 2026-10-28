# Navigation quality review

Reviewed 2026-10-28 by the dispatched Trellis check agent. Scope was the global navigation task only; unrelated dirty files were excluded. Read the complete saved hook context, CLAUDE.md, PRD, design, implementation plan, check manifest, and relevant frontend quality guidance. No commits or builds were performed.

## Findings (fixed)

1. `packages/core/navigation/scope-history.ts:10` failed ESLint `no-control-regex`. Replaced the regex with equivalent backslash and character-code checks, preserving rejection of control characters and spaces. Expanded the existing invalid-destination test with null, newline, and space examples.
2. `apps/web/platform/navigation.tsx:78–106` published a new `navigationKey` before Next rendered the requested destination. A global Workspace resume from an anchored account detail could therefore make the still-mounted outgoing detail treat its old anchor as a fresh explicit open and overwrite the user's later scroll position. The main agent explicitly authorized this local correctness repair after the initial report. Added committed focus intent tied to the rendered location; pending navigation retains the outgoing key/resume state. Added a delayed-route-commit regression in `platform/navigation.test.tsx`, retaining repeated same-destination explicit-open coverage.
3. `packages/views/layout/global-scope-navigation.test.tsx` passed Playwright's `exact` option to React Testing Library `getByRole`, causing three TypeScript errors. Removed the unsupported option; string accessible-name matching already provides the desired semantics.

Changed by this reviewer: the five code/test files above plus this report. No new dependencies, routing API changes, broad refactors, or unrelated warning cleanup.

## Findings (not fixed)

- No additional confirmed product logic defect remained in the reviewed task changes.
- Full package lint reports 27 existing Views warnings and one existing Desktop `tab-content.tsx` hook warning. These are outside the task's changed lines and were left untouched; lint exits successfully.
- Spec synchronization remains assigned to the main agent. Record the fixed shell row/toolbar clearance, `lastWorkspaceSlug`, full account route memento keys and identity clearing, resume-versus-explicit focus intent, and the pending Next destination regression. The Desktop account-navigation spec still refers to the removed Workspaces combobox in its historical source acceptance section.
- Actual production Web/Electron roundtrip, geometry, 200% zoom, language, and native input acceptance belong to the main agent's runtime lane. DOM tests and static classes do not establish those results. This reviewer did not run builds or E2E as instructed.

## Verification

| Check | Result |
| --- | --- |
| `pnpm --filter @multica/core lint` | Pass, after local repair |
| `pnpm --filter @multica/core typecheck` | Pass |
| `pnpm --filter @multica/views lint` | Pass, 27 existing warnings |
| `pnpm --filter @multica/views typecheck` | Pass, after test repair |
| `pnpm --filter @multica/web lint` | Pass, rerun after focus fix |
| `pnpm --filter @multica/web typecheck` | Pass, rerun after focus fix |
| `pnpm --filter @multica/desktop lint` | Pass, one existing warning |
| `pnpm --filter @multica/desktop typecheck` | Pass, Main and renderer |
| Core `navigation/scope-history.test.ts` | 4 tests pass |
| Web `platform/navigation.test.tsx`, `platform/scroll-restoration.test.tsx`, `platform/scope-shell.test.tsx` | 27 tests pass |
| Views global switch, sidebar, collaboration detail, memory panel, account workbench | 53 tests pass |
| Scoped `git diff --check` | Pass |
| New shared/Core files checked for prohibited router/DOM/storage/environment imports | Pass |

The implementation agent separately reported 1,245 passing Desktop tests; this review inspected the changed Desktop boundary tests and ran its lint/typechecks without duplicating that full suite.

## Coverage assessment

Canonical tests cover independent full workspace/account destinations, invalid/global URLs, identity changes and late writes, authoritative membership fallback versus unknown membership, and Desktop persistence/pruning/reset of the remembered workspace pointer. Platform tests cover explicit versus resumed navigation, active-scope no-op, header clearing before account publication, removed/empty/failed membership, and late workspace fetch cancellation. Shared tests cover keyboard/current-state semantics, scroll restoration versus anchor focus, repeated explicit focus, and not reopening a dismissed memory-version dialog on global resume while preserving an explicit reopen.

The newly added Web regression fills the material gap where the router accepts a push while the outgoing route remains mounted. Browser verification should still exercise an anchored account detail after scrolling elsewhere, then a one-click Workspace → Collaboration return, so actual browser scroll events and Next commit ordering are covered together.
