# Workbench failures on frozen v13

State: **proposed patch only; not applied or run**. Both failures are bound to the unchanged `85ef5d205` test. The private patch changes only `e2e/collaboration-workbench-web.spec.ts`.

## Observed failures

- The account-scope journey reaches a successful comment, then cannot click `Transfer` at line 127. Its actual page snapshot shows `Start work`, `Submit result`, `Request information` and a closed `More actions` disclosure. Transfer remains authorized inside that disclosure.
- The stale-lifecycle journey cannot find `Complete issue` at line 220. Its page snapshot shows an external current handler and no submitted result. The current initiator completion contract intentionally offers neither acceptance nor self-completion in that state. Renaming the selector alone cannot create a lawful completion request.

## First journey: presentation repair only

The proposal explicitly reveals `More actions` before the existing transfer and the recipient's positive transfer-affordance assertion. It reveals `Activity` before reading the retained handoff note. The former handler's negative transfer assertion uses `includeHidden: true`, so a closed disclosure cannot hide an unauthorized button.

The original old-comment anchor/focus, uncertain cancellation with one POST and original-key GET, comment/notification, account-header isolation, handoff destination, workbench lists, revocation, viewport and zero-page-error assertions remain. The synthetic unavailable-target setup and all original SQL are unchanged. No UI terminal transition is replaced with an API or SQL shortcut.

## Second journey: preserve a meaningful stale acceptance

Core invariants of the original case are: two isolated human accounts; a real UI lifecycle form; first POST held before reaching the real server; another human mutation changes the issue's revision; exact `409 stale_revision`; visible refusal without uncertain-result recovery; original input remains; explicit use-current action emits no POST; a new request key completes with the newly observed tuple; exactly two lifecycle POSTs, zero operation GETs, done/creator state and no AI dispatch/run.

Those invariants remain. The proposed legal setup is:

1. The external handler submits result and basis through the actual `Submit result` form. This creates the real delivery handoff to the creator. Read back the exact pending submission and its confirmation.
2. The creator opens `Accept and complete` for that ID and selects done. Hold the first lifecycle POST as before; assert all four original observation fields and exact `submission_id`.
3. The former handler leaves through the existing real `leaveThroughUI` helper. That account mutation changes the collaboration revision while the immutable submission remains pending and the creator remains authorized to accept it.
4. Release the original POST. Keep all original stale-refusal, preserved form/status, no automatic POST/GET and explicit retry checks. Additionally require the same submission identity before and after adopting the latest tuple.
5. The second UI acceptance uses a fresh key/current tuple and records the exact creator decision. Final status remains done, handler remains the creator and AI arrays remain empty. Reopen is discovered through `More actions` before the original positive affordance assertion.

### Why the concurrent action changes

The old test specifically used an external-handler transfer concurrent with an initiator completion form opened before any delivery. That combination is now unlawful. A pending formal result has already handed responsibility to the creator and explicitly blocks generic transfer; the external author is no longer the current handler. The new concurrent leave is a real revision-changing cross-account mutation that keeps exact acceptance eligibility, rather than bypassing the current policy.

A self-handling alternative was considered. A creator may legitimately open `Record result and complete`, but another account cannot transfer the creator's work. A second creator tab could transfer it away, which makes self-completion unavailable and can unmount the form. A transfer-away/transfer-back round trip would require a third interaction and introduce an intermediate authority loss/re-mount race; it would no longer reliably test preservation of one continuously valid form. Preventing those intermediate refreshes would require extra transport manipulation. That is less faithful to the original cross-account, explicit-retry invariant and substantially larger than the pending-result/leave case.

This is an explicit contract adjustment for independent review, not an assertion that the old handoff scenario passed. The original failure remains. If the coordinator needs the separate authority-loss/form-recovery behavior, it requires its own scoped test rather than silently conflating it with this stale-revision case.

## Verification boundary

`proposal.json` identifies original/proposed hashes and the single-file patch. TypeScript parse-only and `git apply --check` pass; neither is a runtime result. No tracked source, live v13 input, database, process or Provider was modified. After the original whole run and final audit are retained, independently review this contract adjustment, then run both workbench cases on the successor source with actual API/Web identity. Preserve failed and succeeding runs separately.
