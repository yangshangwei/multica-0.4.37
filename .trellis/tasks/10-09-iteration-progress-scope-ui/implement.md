# Implementation and verification plan

## Planning / baseline

- [x] Record the user's approval of the concrete audit proposal and creation/development request.
- [x] Create PRD, technical design and execution plan; fold resolved questions into the final PRD.
- [x] Read project rules, scoped specs and reuse/cross-layer guidance.
- [x] Save pre-existing source baseline under /tmp/multica-iteration-ui-baseline.rF9dTQ.
- [x] Run initial iterations suites: views 10 files / 115 tests; core 10 files / 87 tests, both pass.
- [x] Curate both context manifests and complete independent plan review; clarify retry limits, sequence ordering, cancelled-only scope reopens and combined frozen-closeout coverage.
- [x] Activate this task after the plan artifact review; Progress and Activity implementers dispatched with disjoint ownership.

## Execution

- [x] Progress lane first adds/adjusts meaningful behavioral regressions, then changes the compact header, three delivery summaries, historical closeout and chart/data disclosures. Preserve existing frozen-chart coverage.
- [x] Activity lane first adds pure/event and recovery regressions, then builds the complete activity query, semantic projection/grouping/filtering, and scope panel. Preserve the API and statistics formulas.
- [x] Integrate the activity panel in the page using the agreed contract; add bilingual copy through separate locale subtrees.
- [x] Adapt only necessary existing e2e table-disclosure interactions; no loss of frozen-value assertions.

## Check and visual pass

- [x] Run changed pure/DOM suites while iterating. Avoid duplicated helper matrices in component tests.
- [x] Run the existing iterations suites, locale parity and package typecheck/lint: core 120 tests, views + locale 194 tests, no lint errors (27 existing out-of-scope warnings).
- [x] Dispatch independent trellis-check over this task's scoped diff and accepted artifacts; root typecheck 9/9, E2E discovery 5 tests, whitespace checks pass.
- [x] Inspect real content in one batched desktop/narrow and light/dark round: 8 Chinese screenshots saved, independent visual verdict 95/pass.
- [x] Persist visual-verdict JSON to .omx/state/iteration-progress-scope-ui/ralph-progress.json; no visual rework was required.
- [x] Run the Impeccable detector once on final changed UI: exit 0, zero findings.
- [x] Verify frozen-history, complete chronology/retry and retained writing/recovery behavior through the regression suites.
- [ ] Complete live keyboard/All activity and English interface confirmation. The first live script incorrectly expected arrow-only activation; the existing Tabs use focus plus Enter/Space. The corrected attempt was blocked by current localhost network permissions (EPERM), not a product assertion.
- [x] Close the latest task-only verification window after localhost access is restored; metadata/helper is in /tmp/multica-iteration-verify.R1ngwl/. Resolved 2026-10-09: the host Electron (inspector 9244) has exited and the current dev Desktop started 06:55, after the window was created at 03:24; the window used an in-memory partition, so nothing persisted to clean.

## Commands

```sh
pnpm --filter @multica/core exec vitest run iterations
pnpm --filter @multica/views exec vitest run iterations
pnpm --filter @multica/views exec vitest run locales/parity.test.ts
pnpm --filter @multica/core typecheck
pnpm --filter @multica/views typecheck
pnpm --filter @multica/core lint
pnpm --filter @multica/views lint
pnpm typecheck
node /Users/artisan/.agents/skills/impeccable/scripts/detect.mjs --json packages/views/iterations/iteration-page.tsx packages/views/iterations/iteration-history.tsx packages/views/iterations/iteration-progress.tsx packages/views/iterations/iteration-events.tsx packages/views/iterations/iteration-events-view.tsx
```

Use the repository's production Web E2E environment rules if browser E2E is run; do not blame dev-server cold compilation for feature failures. No Go changes are planned. Scope a lint fallback to changed files only if unrelated baseline issues block a package-wide command, recording the existing issue clearly.

## Completion

- [x] Update the iteration UI contract and index references with executable behaviors, error matrix and canonical tests.
- [x] Review changes against the saved baseline; core index changes are additive exports, and both locale files preserve every prior value outside the two new owned subtrees. No unrelated files were staged.
- [ ] Follow the Phase 3 commit/wrap guidance, recording actual verification and any remaining limits.
- [ ] Archive this task only when all acceptance work is complete; retain visual evidence and journal the result.

Local checkpoint: session 49 was recorded with --no-commit. The current environment makes .git read-only and rejects localhost CDP connections; no commit or archive has been claimed. See verification.json for exact evidence and remaining work.

## Risk / rollback checkpoints

If task identities, authorization fencing, frozen sources, query completeness or existing form state regress, stop the dependent lane and fix before visual polish. If another writer touches an owned file, integrate its change rather than resetting it. Retain all necessary fields behind disclosures; removal from the default view is not deletion of history.
