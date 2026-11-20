# Completion plan — 2026-11-20

The user asked to continue this existing task and archive it when complete. This resumes implementation/check work; it does not create a new task.

## Scope and ownership

- Preserve the four approved feature groups and the explicit out-of-scope polish decisions.
- Main owns task artifacts, two existing frontend spec updates, final acceptance reconciliation, scoped commits, archival, and journal recording.
- `blocked_browser` owns `e2e/quick-create-blocked-feedback.spec.ts`, isolated acceptance services and `artifacts/qa/quick-create-blocked-20261120/`.
- `blocked_check` owns scoped product/regression fixes, source review, lint/typecheck and unit verification.
- `blocked_design_review` owns independent design Assessment A; detector/evidence Assessment B is isolated from A until A is complete.
- Existing unrelated dirty files are preserved. No remote push or release is part of this task.

## Verification and delivery

1. Add real-browser coverage for the new blocked focus/recovery and keyboard boundaries; measure both themes and language/viewport layouts.
2. Reuse source-bound existing 23-case E2E evidence for unchanged paths; rerun affected paths after any product fix in an isolated production build.
3. Complete a full-scope independent check, rerun necessary tests, and review actual screenshots. Keep failed receipts alongside successful follow-ups.
4. Reconcile each PRD acceptance item to executed evidence and record any bounded limitations.
5. Commit this task's coherent implementation/tests, then its specs/acceptance records. Archive only this task and record its session without staging unrelated work.

## Newly identified boundary

`submit()` must decide whether description content exists from the live editor. Combining a stale debounced `hasContent` value with live Markdown can leave a just-cleared editor silently blocked without receiving focus. The check agent owns a failing regression and minimal correction; no visual redesign or submission-permission change is intended.

## Bounded fixes accepted during review

- Empty `anchorCommentId` keeps the prior ordinary-create semantics through a truthy source requirement.
- Rejecting a recommendation whose live context changed at adoption shows the same stale status as effect-driven invalidation.
- Each correction has a failing regression captured before its fix; existing typed-content, attachment-only and valid-context behavior stays covered.

## Commit grouping

1. One coherent product/test commit: the state helper/export, the four shared components and their tests, bilingual copy, affected existing E2E selectors, and the new focused browser spec. These share localization/import contracts and must remain buildable together.
2. Specs and acceptance records: the two frontend specs, original critique, final review and source-bound verification evidence, and completed task checklist.
3. Task archive, then session journal, each scoped separately. Use no-commit bookkeeping modes and explicit commits to preserve the workspace Lore message convention and unrelated dirty journal entries.

This grouping replaces the original five feature-slice suggestions because their overlapping component and locale edits are already integrated; splitting them would require reconstructing unverified intermediate states.

## Browser evidence correction

The first geometry batch exposed an RCP-2 failure when the recommendation prerequisite wrapped between the recipient and its permanent hint (24/12px on English desktop and up to 72/12px narrow). After a recorded visual verdict, the hint was moved into the existing picker child group without changing copy or CSS. The next confirmation uses the isolated candidate build.

The initially reused shared Next build changed externally during later probes. Those attempts are retained as invalid infrastructure evidence; they cannot establish a product regression. Final acceptance runs from the task-owned private production build instead.
