# Lane B implementation evidence

## Change boundary

- C04 lives in `KnowledgeReview` / its exact-tuple decision owner: the client reason textbox currently unmounts when Core erases a renewing projection. Preserve that editor's DOM, focus and selection while all candidate-derived material and granted actions require the current exact Query value. Candidate selection must reset the editor immediately, including before the replacement read returns.
- C08 lives in the auxiliary `active` prop and the optional candidate/proposal discovery observers. Disable those observers while hidden, retain their component instances and client input, and leave required source/target root reads and Core lease/lifecycle erasure active. Hiding one consumer must not cancel another visible consumer's shared Query.
- Expected production files: `knowledge-review.tsx`, `knowledge-panel.tsx`, `knowledge-proposal.tsx`, `knowledge-source-picker.tsx`, `detail-auxiliary.tsx`, and Core `knowledge-queries.ts`. Tests stay beside the actual shared components; pure lease/cache matrices remain in Core.
- Scope excludes Lane A fields/locales, Lane C memory/workflow, browser/live business writes, task status/global plan/spec changes and commits. Main owns integration and browser evidence.
- Read the full saved native hook, current task artifacts/research, package indexes, shared guides, root `CLAUDE.md`, and Impeccable craft floor before edits. Scoped production paths were clean at preflight. Impeccable context confirms this is an existing scoped repair; no product/design document bootstrap is needed.

## Regression sequence

### Pre-fix reproduction (2026-11-18)

Command:

```bash
pnpm --dir packages/views exec vitest run collaboration/knowledge-panel.test.tsx collaboration/detail-auxiliary.test.tsx collaboration/knowledge-source-picker.test.tsx --maxWorkers=2
```

- Result: 3 files, 57 tests; 9 failed and 48 passed. Exit 1.
- Actual C04 failures: the typed and empty-focused reason textboxes disappear during a held renewal; a transient failed renewal removes the human reason editor too.
- Actual C08 failures: hidden candidate reads increase from 1 to 4; each hidden proposal source discovery increases from 1 to 5 reads (memory current/history both increase); the drawer never supplies `active` to knowledge.
- No harness/schema failures. Candidate-switch/current-grant and overlapping-visible-observer controls already pass on the original production code and remain compatibility checks.
- Full output: `artifacts/qa/collaboration-impeccable-20261117/lane-b/red-initial.log`.
- Before production edits, extending the retained automatic delivery-proposal and hidden recovered-receipt integration coverage, so active propagation is tested through the real collection/form as well as isolated source-picker hooks.

Additional pre-fix integration command:

```bash
pnpm --dir packages/views exec vitest run collaboration/knowledge-panel.test.tsx --maxWorkers=2 -t 'disables both review|recovering an older stored proposal while hidden'
```

- Result: both selected tests failed; 47 skipped. Exit 1.
- Retained automatic correction/review observers shared a candidate key: hidden reads increased from 2 to 7. A recovered older proposal completed while hidden and immediately started a candidate read (0 to 1), before reopening.
- Full output: `artifacts/qa/collaboration-impeccable-20261117/lane-b/red-integration.log`.

### Repair and passing focused checks

- `KnowledgeDecision` retains a single keyed reason container for human text or an empty focused editing target. The reason field stays enabled unless an actual write/uncertain-intent lock applies. Approval siblings and current-grant actions disappear during renewal without reparenting the textbox. Candidate absence/inactivity prevents a new decision; authoritative denial removes the decision instance.
- `KnowledgeReview` is keyed immediately by selected candidate ID; its inner decision keeps the exact protocol/ID/revision/availability/version/hash tuple. Focus consumes a selection once, including selections established while hidden, so renewal/reopening never takes focus.
- Optional `active` defaults to true in standalone/managed use. Auxiliary visibility reaches source collection, review, proposal and every optional source-picker observer. Core candidate `enabled` combines activation with viewer validity while lifecycle/realtime bindings remain mounted. Required source/target root reads are unchanged. No per-hide shared-query cancellation, cache/lease edits, copied server state or dependencies were added.
- Actual hidden expiry/focus/reconnect tests retain editor nodes and chosen source IDs, prove no new optional reads, and verify fresh reads on reopening. A retained automatic correction plus review cannot keep their shared key active; another visible consumer continues its in-flight GET. A hidden recovered older proposal preserves the new proposal draft, sends no extra POST, and only starts the selected review read after reopening.

Commands and observed results:

```bash
pnpm --dir packages/views exec vitest run collaboration/knowledge-panel.test.tsx collaboration/detail-auxiliary.test.tsx collaboration/knowledge-source-picker.test.tsx collaboration/knowledge-automation.test.tsx --maxWorkers=2
```

- 4 files, 62 tests passed; exit 0. Log: `artifacts/qa/collaboration-impeccable-20261117/lane-b/green-views.log`.

```bash
pnpm --dir packages/core exec vitest run collaboration/knowledge-cache.test.ts collaboration/knowledge-lease.test.ts collaboration/memory-queries.test.ts collaboration/knowledge-sources.test.ts collaboration/knowledge-automation-queries.test.ts --maxWorkers=2
```

- 5 files, 25 tests passed; exit 0. Log: `artifacts/qa/collaboration-impeccable-20261117/lane-b/core-contracts.log`.

```bash
pnpm --dir packages/views exec eslint collaboration/knowledge-review.tsx collaboration/knowledge-panel.tsx collaboration/knowledge-panel.test.tsx collaboration/knowledge-proposal.tsx collaboration/knowledge-source-picker.tsx collaboration/knowledge-source-picker.test.tsx collaboration/detail-auxiliary.tsx collaboration/detail-auxiliary.test.tsx
pnpm --dir packages/core exec eslint collaboration/knowledge-queries.ts
```

- Both scoped lint commands passed (exit 0). Logs: `lane-b/lint-views.log` and `lane-b/lint-core.log` beneath the same artifact directory.
- Final source-picker test harness uses the required callable-store + `getState` mock shape. After that mechanical adjustment, `pnpm --dir packages/views exec vitest run collaboration/knowledge-source-picker.test.tsx --maxWorkers=2` passed all 4 tests (exit 0); its scoped eslint also passed. Logs: `lane-b/source-picker-final.log` and `lane-b/lint-source-picker-final.log` beneath the same artifact directory.
- `git diff --check` over all tracked Lane B production/test paths passed. Final tracked source diff was reviewed against HEAD; the new source-picker suite was reviewed separately. No relevant lane failures remain.

### Files and integration boundaries

Production: `packages/core/collaboration/knowledge-queries.ts`; Views `collaboration/{detail-auxiliary,knowledge-panel,knowledge-review,knowledge-proposal,knowledge-source-picker}.tsx`.

Tests: Views `collaboration/{detail-auxiliary,knowledge-panel,knowledge-source-picker}.test.tsx`. Evidence: this progress file and `artifacts/qa/collaboration-impeccable-20261117/lane-b/`.

Main retains full typecheck/lint, current-source Chromium fixture, bounded visual verdict, specification/audit updates and task completion. No live API writes, native device/screen-reader claims, commits, task-state edits or unowned changes were performed in this lane. Lane C owns actual memory-panel history activation and confirmation/workflow repairs. Existing in-flight hidden GETs remain permitted by Core's current lease/cancellation rules; this repair prevents new automatic reads and never cancels another visible observer.
