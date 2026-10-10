# Implementation and verification plan

## Planning handoff

- [x] User authorized task creation from the four reviewed improvements.
- [x] Requirements, design, acceptance mapping and evidence recorded.
- [x] Concrete spec/research entries curated in both context manifests.
- [x] User explicitly approved implementation after the final planning summary: “开始修改业务代码” (2026-10-10).
- [x] Activated the task after explicit approval; implementation and verification completed.

## Cleanup and implementation sequence

1. Read the full current `.trellis/spec/core/frontend/iteration-operations.md` directly: it exceeds the automatic per-file context limit, so manifests inject its index plus the task research instead. Record the current diff and inspect existing regression coverage. Preserve unrelated iteration/settings/spec/task edits.
2. Confirm coverage for counters, current/original sources, frozen labels, tab retention and pagination. Add missing behavior regressions before changing those contracts; do not test CSS class choices.
3. Compact the parent summary and remove the repeated inner heading; pass existing phase information to list presentation.
4. Build the bounded-search toolbar, lightweight filter entry and separate grouping control while preserving options and metadata query behavior.
5. Add derived condition summaries and reuse one reset action in non-empty and empty states. Preserve typed assignees, unknown choices and grouping.
6. Simplify row metadata and improve title/status/assignee hierarchy; retain positive rollovers, review warnings and long titles.
7. Update both locales; reuse primitives/tokens and add no dependencies.
8. Run focused tests and required broader checks. Use the Trellis implement/check agent lane after implementation approval.
9. Capture bounded visual evidence, update durable iteration presentation contracts, and record completion evidence.

## Canonical behavior coverage

Extend `packages/views/iterations/iteration-details.test.tsx`, `iteration-page.test.tsx` and `iteration-history.test.tsx` according to their current ownership:

- Phase-aware scope availability, including a genuinely started empty baseline (AC3).
- Matching feedback versus unchanged whole-period counters (AC4).
- Non-empty result summaries, individual removal, reset/group preservation and empty recovery (AC5).
- Tab retention, cursor resets and no all-pages fetch on filter open (AC6).
- Null versus unknown names, blocked text, rollover and review threshold (AC7).
- Real popup keyboard handling and existing loading/error/permission states (AC8-AC9).

Commands from repository root, after checking installed tooling:

```bash
pnpm --filter @multica/views test iterations/iteration-details.test.tsx iterations/iteration-page.test.tsx iterations/iteration-history.test.tsx
pnpm --filter @multica/views test locales/parity.test.ts
pnpm lint
pnpm typecheck
pnpm test
pnpm check:ui-exports
/Users/artisan/.agents/skills/impeccable/scripts/impeccable detect --json packages/views/iterations
```

Use a focused helper test only if a justified helper is introduced. Backend changes are out of scope; revisit this plan and run Go checks if that boundary changes.

## Visual and runtime verification

- Verify checkout/process ownership with `make status`; the review found ownership mismatch.
- Use `e2e/iterations-audit-desktop.spec.ts` and its existing setup after reading the E2E environment contract.
- Compare the two-task planned case with the preserved reference size/theme/zoom (AC1).
- Batch wide/narrow panels, English/Chinese, long names, active conditions, grouping, started-zero-baseline and frozen states. Retain 44px coarse targets.
- Follow impeccable's bounded inspection: one round, one batch of fixes, at most one confirmation. Use visual-verdict before the next visual edit and store its result under `.omx/state/iteration-task-tab-density/ralph-progress.json`; this does not activate an OMX runtime workflow.
- Do not infer keyboard, contrast, overflow or request behavior from a screenshot.

## Completion and rollback

Map each AC to an observed result, test or explicit blocker. Record exact commands and failures, separating baseline failures from regressions. Review only task-owned hunks and update the durable spec as needed. Roll back only those hunks; no database rollback is required.

Task creation was documentation-only; implementation results are recorded in `verification.md`.
