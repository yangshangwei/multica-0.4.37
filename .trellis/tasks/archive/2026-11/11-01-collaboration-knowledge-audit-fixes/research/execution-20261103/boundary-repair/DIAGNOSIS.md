# Boundary E2E failures on frozen v13

State: **diagnosis and proposed single-file patch only**. The running candidate remains `85ef5d20578f63c7c4f879f035acbd414e80f306`; no tracked source was written and no test, DB, service or Provider operation was started.

## Observed failures and root cause

1. **Gap 5** fails at `collaboration-boundaries-web.spec.ts:186`: the test directly asks the creator to complete work still assigned to an external handler, and receives `delivery_required`. The current `prepareCompletion` in `collaboration_lifecycle.go` requires a valid pending submission with exact `submission_id`, or the initiator's own result/basis when the initiator is the current handler. The frozen test has neither. This is the documented external-delivery requirement, not a workspace deletion defect.
2. **Gap 7** hangs on the `Manage AI resources` click. The actual `0-trace.trace` has `call@50` without a matching after event until the total test timeout. `browserContext.close` is cleanup after that timeout. The current page snapshot shows a closed `AI execution` disclosure; `Manage AI resources` is inside it. The already-committed resource journey uses `revealCollaborationDisclosure(page, "AI execution")` first.
3. **Smaller gaps** hangs on its initial `Edit issue` click in the workflow's first div. That action is now inside `More actions`; the existing `revealCollaborationEditingAction` helper opens the disclosure and proves the requested action is visible before returning it.

The same cases have additional directly related setup drift after those first failing operations: the activity record is inside `Activity`; the creator's participant invitation is inside `More actions`; and the workflow now derives actions from the exact delivery state as well as role. The creator does not get an acceptance/self-completion action before an external result exists. A handler in Todo sees `Start work` and `Submit result`. Non-handling participants and source readers have a nonprivileged `Comments` primary action. The current workflow, labels, `presentation.ts` and the retained creator page snapshot establish these differences.

## Proposed minimal repair

`boundary-contract-repair.patch` changes only `e2e/collaboration-boundaries-web.spec.ts`.

- Gap 5 prepares a real text-only result using the existing `write`, `readIssue` and `observed` helpers: current handler status options → `POST /acceptance-submissions` with result/basis/current observations → creator reads the exact current submission → fresh complete options and `POST /lifecycle` with that ID. Success HTTP status, receipt identity and final accepted decision are asserted.
- Formal submission returns responsibility to the creator. The final read still comes from the original external handler, proving retained access, while asserting that the current handler is the creator and the retained result author is the original handler. Keeping the old final handler ID would contradict the required real workflow.
- Gap 7 uses the already-committed disclosure helper before resource management and before examining the creator/handler's expanded more actions. Its exact action inventories match the current state. Inventory collection uses `includeHidden: true`, so an unauthorized button cannot disappear from the assertion simply because its disclosure is closed. The source reader's only action is `Comments`, not a manager operation.
- The smaller case reuses the existing editing helper and explicitly opens `Activity` and `More actions` at the relevant ordinary discovery steps.

The 21 direct wrong-role POST attempts and their exact `403 forbidden` comparison remain byte-identical. The workspace inventory helper and all workspace deletion refusal, membership, ordinary sibling, comments, attachment bytes, command-queue and quiet-notification checks remain. Timeouts, retries, runtime enrollment, production permissions and product code are unchanged. No SQL is used to fabricate a terminal lifecycle or submission.

## Evidence and remaining verification

`proposal.json` binds the original test hash, proposed copy, patch, live failure excerpts and original trace hashes. `syntax.json` records TypeScript parse-only success (5.9.3, no parse diagnostics). `git apply --check` succeeded without applying the patch. Read-only comparisons verified the unchanged refusal/inventory blocks.

The original v13 failures are the actual red observations. The proposed fix has **not** been exercised; syntax and patch applicability do not establish a green result. After v13 ends and its final identity audit is retained, a separate reviewer must inspect this exact patch. The coordinator then owns a newly recorded directed run of these three cases on the successor source, with real API/Web identity and fresh task database ownership, followed by the required successor full gate. Do not edit the running v13 candidate or rewrite its failed result.
