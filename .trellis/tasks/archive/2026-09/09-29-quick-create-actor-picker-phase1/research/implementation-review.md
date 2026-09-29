# Independent implementation review

Reviewed on 2026-09-29 in `/Volumes/artisan/code/2026/multica-actor-picker-phase1` against the saved pre-task tree from `/tmp/multica-actor-picker-01a0ed8c/baseline-tree.txt`. This review covers the task delta, preserving unrelated baseline edits. No commits, dependency changes, task-state changes, or visual edits were made by the reviewer.

## Verdict

**Code review approved after one scoped fix.** No remaining concrete defects were identified in the reviewed implementation. Final browser, desktop, narrow-viewport and 550-actor performance acceptance remains owned by the leader/E2E lane; this note does not substitute unit tests for those measurements.

## Files checked

- `packages/core/issues/stores/quick-create-store.ts` and `.test.ts`
- `packages/views/modals/quick-create-actor-picker-model.ts` and `.test.ts`
- `packages/views/modals/quick-create-actor-picker.tsx` and `.test.tsx`
- `packages/views/modals/quick-create-issue.tsx` and `.test.tsx`
- `packages/views/issues/components/pickers/property-picker.tsx` and `.test.tsx`
- New `create_issue.actor_picker` locale keys in English and Simplified Chinese

The review followed `check.jsonl`, PRD, design, implementation plan, preference/interaction research, authoritative repository conventions, and the Trellis check-agent instructions.

## Issue found and fixed

### Import-time hydration reads auth before initialization

The new explicit module-level `persist.rehydrate()` synchronously entered `onRehydrateStorage`, whose in-memory invalidation passed through the persistence write gate and read the auth singleton. Importing consumers could therefore access auth before their dependency graph finished initializing. The existing issues-page suite reproduced a `ReferenceError: Cannot access 'mockAuthUser' before initialization` before collecting any tests; the leader independently confirmed the original suite passed unchanged.

Removed the redundant eager hydration call. The registered workspace activation callback still hydrates the store, and the mounted agent-create panel hydrates when an authenticated unchanged scope requires it. No catch-all exception handling or unrelated mock changes were introduced.

Added a core import-lifecycle regression. Before the fix it observed three auth reads during module initialization and failed; after the fix it observes zero. Existing workspace, malformed-storage, stale-read, logout-generation, and immediate pre-microtask write guards remain passing.

## Additional coverage

- Both ordinary and comment-context pending requests now change the picker selection before acceptance and assert that history records the original submitted actor, with the origin workspace/user capture checked.
- Both creation branches assert that rejected requests do not record recent usage.
- Three parent integration cases cover delayed preference hydration: last-successful fallback waits, explicit actor remains authoritative, and unfinished draft actor remains authoritative. The prompt survives each transition.

## Contract assessment

- Durable state persists allowlisted typed actor references only; favorites preserve pin order, recent entries are deduplicated/capped, legacy last actor seeds only absent recent history, and malformed destination data defaults without cross-workspace carryover.
- Scope readiness compares live workspace/user/session identity. Hydration completion and accepted-submit writes reject stale identities; preferences are hidden and pin writes blocked until ready.
- Available directory projection precedes preference resolution. Full saved descriptions participate in case-insensitive search, name/pinyin ranks remain ahead of description-only hits, and type filtering intersects the entire catalog.
- Pin buttons are siblings of primary candidate buttons. Focus recovery, IME protection, optional external navigation resets, typed-search priority, and pagination highlight preservation use real PropertyPicker/Base UI tests.
- Loading, uncached failure, cached refresh failure and definitive empty results remain separate. Agent-only filtering does not wait for squad data; squad choices still require eligible leaders.
- Form integration retains draft/project/upload/version/permission contracts and squad submission identity. No backend, schema, runtime-selection or hidden-instruction changes were introduced.
- Existing last-actor/keep-open fields retain their persisted shape. An old UI can ignore the extra fields; an old client's later write may drop the new local arrays, as documented in the approved rollback contract.

## Verification executed by reviewer

| Command | Result |
| --- | --- |
| `pnpm --filter @multica/core exec vitest run issues/stores/quick-create-store.test.ts` before fix | Expected red: import-auth regression failed, 26 other tests passed |
| `pnpm --filter @multica/views exec vitest run issues/components/issues-page.test.tsx` before fix | Reproduced import-time ReferenceError; no tests collected |
| `pnpm --filter @multica/core exec vitest run issues/stores/quick-create-store.test.ts platform/workspace-storage.test.ts platform/session-cleanup.test.ts platform/storage-cleanup.test.ts drafts/cleanup-registry.test.ts` | 46 tests passed, 5 suites |
| `pnpm --filter @multica/views exec vitest run issues/components/issues-page.test.tsx` after fix | 10 tests passed |
| `pnpm --filter @multica/views exec vitest run modals/quick-create-issue.test.tsx modals/quick-create-actor-picker.test.tsx modals/quick-create-actor-picker-model.test.ts issues/components/pickers/property-picker.test.tsx issues/components/pickers/assignee-picker.keyboard.test.tsx modals/quick-create-scenario.test.ts modals/create-issue-dialog.test.tsx modals/create-issue.test.tsx locales/parity.test.ts` | 236 tests passed, 9 suites |
| `pnpm --filter @multica/core typecheck` | Passed |
| `pnpm --filter @multica/views typecheck` | Passed |
| Scoped ESLint over the core store/test and all 8 task views source/test files | Passed |
| `git diff --check` | Passed |

The broad core `/mcp` diagnostic failure and unchanged Knip findings were reported by the leader as independently reproduced baseline issues; they were not altered or relabeled as passing here. Broad checks, E2E results, screenshots and browser performance measurements belong in the leader's final verification artifact.
