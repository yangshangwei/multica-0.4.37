# Implementation and verification

1. Read existing collaboration and issue-description-assist specs, current tests and source research. Snapshot the pre-existing one-line non-exact Close test selector change; review and retain it as the scoped ByRoleOptions typecheck repair; do not bundle unrelated edits.
2. Implement single-mounted expand/collapse shell, localized labels, reset and viewport constraints with focused interaction tests.
3. Add narrow reusable assist options for raw textarea and restricted/inert preview, exact label, input limit reason and current-owner fencing. Preserve default callers and test useful shared contracts.
4. Wire collaboration textarea adapter, sourceId/draft key, live locks/current scope, visible limits and layout callbacks. Tests cover actual draft changes, stale edits/late results/source changes, locks, undo whitespace, failures, stream vs create and default ordinary behavior.
5. Run targeted collaboration/ordinary assist/real-editor/locale tests, Views typecheck/lint. Use bounded test concurrency to avoid machine contention.
6. Build task-owned production component preview using existing fixture infrastructure and mock SSE/API; no paid model/real task submission. Inspect all representative sizes/themes/states in one batch and persist visual verdict before any corrections. Fix measured defects together, confirm once. Preserve existing appearance and native behavior.
7. Independent trellis-check full-scope review, final relevant tests/typecheck/lint, scoped detector, diff check and exact source provenance. Stop verification resources, update contracts, scoped commits and archive.

Commands:
```sh
pnpm --filter @multica/views exec vitest run collaboration modals/issue-description-assist.test.tsx modals/issue-description-assist-editor.test.tsx modals/create-issue.test.tsx modals/create-issue-dialog.test.tsx locales/parity.test.ts --maxWorkers=1
pnpm --filter @multica/views typecheck
pnpm --filter @multica/views lint
/Users/artisan/.agents/skills/impeccable/scripts/impeccable detect --json packages/views/collaboration/creation-form.tsx packages/views/collaboration/creation-description-assist.tsx packages/views/modals/issue-description-assist.tsx
git diff --check
```
