# Implementation and verification plan

## Planning

- [x] Record authorization, audit requirements and current dirty-source reality.
- [x] Create PRD, design and implementation plan with acceptance mapping and ownership.
- [x] Finish independent research; curate implement/check manifests (12 validated entries each).
- [x] Save source/git baseline and review final planning artifacts (49 source files and starting git status/diff).
- [x] Activate task after review; user explicitly authorized development after the presented audit. PRD convergence and boundary review found no unresolved product choices.

## Execute and check

- [x] Run existing targeted regressions before new product changes.
- [x] Dispatch Trellis lanes to review/test existing repairs and fix demonstrated local gaps.
- [x] Review chat focus, selection/navigation, Query recovery and Tabs/reorder boundaries.
- [x] Verify Gantt contrast, viewport dates, row virtualization and keyboard focus.
- [x] Check enabled narrow/zoomed pagination and reduced-motion geometry.
- [x] Integrate, verify locale parity, task-scoped lint and whitespace.
- [x] Run workspace typecheck and broader relevant tests after lane fixes.

## Electron and completion

- [x] Use isolated real preload/renderer; preserve user windows and business data.
- [x] Capture one batched round for all ten groups, both themes/locales and critical widths.
- [x] Verify cold/cached 500 recovery, 404, keyboard/focus, contrast and extreme-range metrics.
- [x] Get independent visual verdict; save .omx/state/desktop-core-impeccable-fixes/ralph-progress.json (round 2, 93/100, pass).
- [x] Run detector once on task UI and inspect findings.
- [x] Update executable code-spec and separate repair report; preserve original audit.
- [x] Final independent check and current-source evidence hashes (`evidence/final-source-hashes.json`).
- [x] Commit task hunks with Lore trailers; archive and journal the completed task.

## Commands

```sh
pnpm --filter @multica/views exec vitest run chat/components/chat-window.test.tsx chat/components/chat-queue.test.tsx chat/components/use-chat-input-focus.test.ts issues/components/list-view.test.tsx issues/components/gantt-view.test.tsx agents/components/agents-page.test.tsx agents/components/agent-overview-pane.test.tsx inbox/components/inbox-page-recovery.test.tsx projects/components/projects-page.test.tsx projects/components/project-detail.test.tsx skills/components/skill-detail-page.test.tsx triage/triage-page.test.tsx locales/parity.test.ts
pnpm --filter @multica/desktop exec vitest run src/renderer/src/components/tab-bar.test.tsx src/renderer/src/components/tab-bar.accessibility.test.tsx src/renderer/src/components/desktop-layout.test.tsx
pnpm typecheck
pnpm --filter @multica/views exec eslint <task-owned files>
pnpm --filter @multica/desktop exec eslint src/renderer/src/components/tab-bar.tsx src/renderer/src/components/desktop-layout.tsx <adjacent tests>
git diff --check
```

Record actual outputs under evidence/. Pure tests own matrices; DOM/browser checks own interaction wiring, actual styles and real inert/virtualization. Repeat broader checks only after changes/failures. Record remaining external limitations without checking an unverified acceptance criterion.

## Final visual correction checkpoint

- Core verification before the last compact toolbar correction: views 406 tests, desktop 111 tests, workspace typecheck 9/9, targeted lint 0 errors / 2 existing project warnings, detector 0 findings.
- Native recovery, chat visibility/portal/focus, keyboard selection/title/Tabs/reorder, reduced-motion interruption and bounded Gantt checks passed; see evidence JSONs.
- Visual round 1 scored 86/revise: compact selected-agent toolbar wrapped the count and obscured filters/headers. The remaining repair is local toolbar presentation, then one batched confirmation round and final checks.
- The implementation landed in `ee5922c9e`; closure metadata and archive remain to be committed. Unrelated working-copy changes are preserved.

## Final closure checkpoint

- Round 2 independent visual verdict is **93/100, pass**; the corrected compact
  toolbar is single-line for the count and leaves filters, headers and launcher
  unobscured at the effective 450px viewport.
- Fresh `pnpm typecheck` passed all 9 workspace tasks. Fresh `git diff --check`
  passed with no output. Current-source hashes for all 49 baseline files are in
  `evidence/final-source-hashes.json` (17 differ from the recorded pre-task
  baseline, as expected for the repair scope).
- `verification.json` records the final evidence, limitations and the existing
  unrelated `apps/web/next-env.d.ts` working-copy change.
