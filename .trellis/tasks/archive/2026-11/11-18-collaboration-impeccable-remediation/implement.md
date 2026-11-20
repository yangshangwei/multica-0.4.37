# Collaboration Impeccable Remediation Implementation Plan

**Goal:** Fix C01–C09 and provide regression evidence for their actual behavior.

**Architecture:** Keep the existing shared ownership: caller-side picker
filtering, client-owned inputs outside private projection gates, protected Query
read visibility, and local dialog geometry. Main session coordinates bounded
Trellis implement/check lanes and owns integration, evidence and spec updates.

**Tech Stack:** React/TypeScript, TanStack Query, Base UI, shared Tailwind CSS,
Vitest/Testing Library and Playwright Chromium.

## Preflight and planning review

- [x] User authorized repairs/regression and consented to task creation/planning.
- [x] C01–C09 and preserved contracts recorded in `prd.md` and `design.md`.
- [x] Capture intended file baselines in `research/source-baseline.json`; only
  collaboration locale files have existing dirty hunks among planned source
  paths. Do not overwrite or include those hunks.
- [x] Reviewed completed planning artifacts and research against the original
  audit and authorized acceptance; independent review corrections are recorded
  in `research/planning-review.md`.
- [x] Both context manifests contain 13 real spec/research entries and passed
  `task.py validate`.
- [x] Main activated this task after artifact review; status is `in_progress`.
- [ ] Each implementer reads relevant specs and Impeccable craft floor before
  UI edits. No subagent recursion or unowned file edits.

Every behavior fix begins with a failing regression, verifies the correct
failure, then uses the smallest implementation that makes it pass. CSS-only
geometry/target fixes use the retained real-browser failing baseline rather
than assertions mirroring class names.

## Lane A: choices, labels and initial fields (C01/C02/C03/C06/C09)

Ownership:

- `packages/views/issues/components/pickers/property-picker.tsx`
- `packages/views/issues/components/pickers/custom-property-picker.tsx`
- Relevant adjacent component suites, adding a component suite only where the
  existing `.test.ts` is a pure helper owner.
- `packages/views/collaboration/field-inputs.tsx` and `.test.tsx`
- `packages/views/collaboration/creation-form.tsx` and `.test.tsx`
- Necessary new keys only in `packages/views/locales/{en,zh-Hans}/collaboration.json`;
  use existing issue picker strings where available.

Steps:

1. Add real-component failures for >7-choice exact/partial/no-match search and
   Enter, actual multiple-selection pressed state and field input names.
2. Run the new tests and retain the original failures. Include empty-row clear,
   reopen/reset, and current selection in the actual component coverage.
3. Wire local option filtering and the existing empty state. Add explicit
   toggle state without disturbing listbox `optionId` roles or command rows.
4. Label text/number/URL input and apply coarse-pointer row/input floors and
   narrow popover bounds; do not replace the picker shell.
5. Add stage invalid-input regressions, attributing schema errors to the actual
   input and retaining the generic catalog failure. Implement localized inline
   association without reimplementing the schema range.
6. Run the lane suites and account picker/create/IME compatibility tests.

Focused command:

```bash
pnpm --dir packages/views exec vitest run issues/components/pickers/property-picker.test.tsx issues/components/pickers/custom-property-picker.test.ts issues/components/pickers/custom-property-picker.test.tsx collaboration/field-inputs.test.tsx collaboration/creation-form.test.tsx collaboration/account-picker.test.tsx locales/parity.test.ts
```

Only include a newly created component test path once it exists. Record test
counts and exact filenames; do not report skipped or missing paths as tested.

## Lane B: exact review input and knowledge visibility (C04/C08)

Ownership:

- `packages/views/collaboration/knowledge-review.tsx`
- `packages/views/collaboration/knowledge-panel.tsx` and `.test.tsx`
- `packages/views/collaboration/knowledge-proposal.tsx`
- `packages/views/collaboration/knowledge-source-picker.tsx`
- `packages/views/collaboration/detail-auxiliary.tsx` and `.test.tsx`
- `packages/core/collaboration/knowledge-queries.ts` and applicable hook tests
  if necessary; no lease erasure rule or cache-body retention changes.

Steps:

1. Add a same-tuple delayed-renewal regression that starts with a focused reason
   input and selection, and proves node/focus/selection continuity.
2. Verify its pre-fix failure. Extend existing tuple-change, unavailable,
   restricted and private-body erasure regressions instead of duplicating Core
   schema/permission matrices.
3. Preserve the client input lifetime while candidate-derived material and
   decision permissions remain tied to the current exact authorized tuple.
4. Add hidden/read-visibility regression that opens a candidate, deactivates the
   panel, invalidates/advances the interval, and verifies no new candidate read.
   Verify current private data is still erased at expiry and reopening performs
   current authorized reads without focus leakage.
5. Propagate `active` only to the relevant knowledge review/read observers; keep
   necessary root source/target status and recovery intact.
   Include retained proposal source discovery observers, particularly the delivery
   candidate sharing a Query key with the review. Preserve client editor nodes
   at stable React positions; do not disable them because a read is pending.
6. Coordinate C08 context-history changes with Lane C; Lane B does not write
   `memory-panel.tsx`.

Focused commands:

```bash
pnpm --dir packages/views exec vitest run collaboration/knowledge-panel.test.tsx collaboration/detail-auxiliary.test.tsx collaboration/knowledge-automation.test.tsx
pnpm --dir packages/core exec vitest run collaboration/knowledge-cache.test.ts collaboration/knowledge-lease.test.ts
```

Use the actual discovered cache/lease test paths and relevant existing query
suite. Node-only schema tests retain `// @vitest-environment node`; DOM hooks
remain in their current environment.

## Lane C: context confirmation, historical visibility and workflow reads (C05/C07/C08)

Ownership:

- `packages/views/collaboration/memory-panel.tsx` and `.test.tsx`
- `packages/views/collaboration/workflow.tsx` and `.test.tsx`

Steps:

1. Add failed-then-successful `status-options` test proving retry within the same
   dialog, unchanged typed reason, pending submit lock and no action POST.
2. Verify the expected missing-retry failure, then connect existing refetch and
   loading feedback. Keep all current status/permission guards.
3. Add context history visibility regression: open history, hide the panel,
   trigger interval/refocus/invalidation, then reopen. Preserve loaded page
   state and editor/selected version while unnecessary reads are disabled.
4. Apply the active gate to historical and selected-version reads. Current
   context root status remains available for toolbar recovery.
5. Fix purge/restore confirmation locally with viewport bounds, a scrolling
   middle and accessible nonshrinking header/footer. Bound legal multiline
   input and coarse-pointer actions. Keep the shared AlertDialog API unchanged.
6. Run restore/purge permissions/snapshot and original receipt suites.

Focused command:

```bash
pnpm --dir packages/views exec vitest run collaboration/memory-panel.test.tsx collaboration/workflow.test.tsx collaboration/workflow-upload.test.tsx collaboration/source-bridge.test.tsx
```

## Main integration, browser checks and independent review

1. Review each lane's task diff against saved working baseline. Reconcile only
   the task-owned changes; do not reset other work or stage broad directories.
2. Run one combined focused suite including all changed components, locale
   parity and protected knowledge/history compatibility tests.
3. Upgrade the retained original-component audit fixture to assert repaired
   outcomes, including the new selected-state semantics. Rebuild current source
   with its manifest; do not reuse a stale built bundle.
4. Capture one batched Chromium round: 1280px, 320px, 390px and short-window
   layouts as needed, light/dark, coarse pointer and enlarged text. Assert:
   search-to-commit, no-match Enter, input names, >=44px targets, legal
   1,000-byte multiline reason, title/CTA reachability and no horizontal overflow.
   Include same-tuple renewal/focus in real Chromium if the fixture supports it.
5. Record geometry after fonts/finite animations/painting settle. Use a bounded
   visual verdict and fix all observed defects in one batch; at most one
   confirmation batch afterward. Do not start a polishing loop.
6. Run `pnpm typecheck` and `pnpm lint`. Run meaningful shared Views/Core
   regression suites and static detector. Broaden testing when failures or
   public shared primitive risk justify it, keeping unrelated failures explicit.
7. Dispatch independent `trellis-check` with real `check.jsonl` context. It may
   fix mechanical in-scope failures and report judgment/scope decisions to main.
8. Sync exact behavior to picker/creation, knowledge review/lease and auxiliary
   visibility specs; update audit report with a C01–C09 closure/evidence table.
9. Verify final task-owned diff, evidence paths, no pending relevant failures,
   and report actual performed checks and any material untested device surfaces.

## Finish and rollback

Main follows the repository's finish/commit contract, preserving unrelated
existing hunks. Commit messages use intent plus applicable Lore trailers and
the required conventional prefix. Never deploy, package, push or change live
business data as a side effect of this task. Per-lane source changes can be
reverted independently; no migrations or durable-state transformations exist.

## Completed outcome

All three implementation lanes and main integration are complete; see `verification.md` for actual commands/evidence and untested device surfaces. The final root typecheck and lint, combined shared regressions, real Chromium batch, detector and independent review passed. Specification updates capture the executable contracts. No additional visual iteration was necessary.
