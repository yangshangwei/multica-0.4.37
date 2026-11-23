# AC93 mention admission final independent review — 2026-10-22

**Recommend AC93 passed at its assigned P5 backend trigger boundary.** The exact seven-state direct typed-comment matrix and canonical adjacent assertions satisfy the whole criterion. No product defect, unsafe acceptance shortcut or remaining P5 clause gap was found. This reviewer changes only this report and `mention-admission-final-review.json`; the coordinator owns ledger promotion and Lore integration.

## Findings (fixed)

None. The reviewed source is gofmt-clean. Exact current SHA-256 is
`07338b91526b867f32e0fdff2865b2c42d9bc4b8949d8fce03b9f029eede6f57`.
All18 protected production/helper sources remain equal to the implementer baseline.

## Findings (not fixed)

No product or acceptance issue remains at the assigned P5 boundary. Preserve these evidence limits: service-issued Claim/Permit is actual persisted admission authority; `managedProof` and success outcomes are synthetic protocol input. The suite does not launch a provider or establish natural kernel process start/termination. Web/Electron UI, full Main/installer, other OS and paid provider/account/rollout evidence remain separate parent scopes.

The first two failed attempts remain explicit fixture corrections, not current passing evidence: an initial query used nonexistent `consuming_root_run_id`; it was changed to the actual `consuming_run_id`. The next attempt assumed busy candidate list empty; production correctly returned the original permitted run for recovery. The corrected assertion requires that exact original run, one run row and the new input still waiting. No product source or schema changed.

## Whole-clause assertion assessment

The new `TestCollaborationMentionCommentAdmissionAcrossExecutionStates` directly calls **CreateComment** with a real typed `mention://agent/<registered resource UUID>` in each scenario. Its earlier setup/cancelled/original/pending inputs also use CreateComment. The target/consent/managed identity is real service fixture setup. No SubmitCommand invocation substitutes for the seven trigger cases.

| AC93 state | Actual receipt and proved behavior |
| --- | --- |
| Idle | Waiting/command_queued, exact comment/event/author/content/target/channel/batch and no group before List/Claim/Permit; later Claim freezes that exact input and Permit saves one attempt. |
| Busy | Waiting/command_queued/target_busy in a separate batch. Prior comment has real Claim/Permit, remains unknown/unsettled/unforced. Repeated polls return only that original recoverable run; new input stays waiting until original outcome/proof settlement. |
| Paused | Actual issue Stop precedes @. Comment saves waiting/issue_paused without changing controls; repeated polls expose no run. Explicit Resume precedes eventual exact Claim/Permit. |
| Stop unconfirmed | Original permitted run retains stop intent, unknown proof and unreleased reservation. @ saves waiting/target_paused. Resume refuses without state change; proof does not auto-resume; the same refused key remains identical. Only fresh explicit Resume starts the new input; original comment command remains stopped. |
| Resource withdrawn | Actual RevokeGrant follows prior permitted and waiting comment inputs. New human comment persists with blocked/target_unavailable, no command/batch/group identity. Controls/grants/proof/reservation and prior waiting input remain intact; polling cannot admit new work. |
| Author removed | Actual RemoveParticipant removes external author. Its new @ request returns not_found with no comment or operation. Pending input stays cancelled/participant_removed; original run remains stopped/unknown/reserved. |
| Whole issue terminal | Actual complete lifecycle precedes @. Human comment persists with blocked/issue_closed and no command/group. Pending command stays cancelled/issue_completed, old run remains stopped/unknown/reserved and polling starts nothing. |

Each legal waiting receipt is checked against exact persisted command identity and event/author/body/sequence, not a name or count alone. `mentionAdmissionClaim` checks issue/session/target/channel/batch/run, original body and immutable `collaboration_group_input.consuming_run_id`; Permit checks the actual saved attempt and in_flight command. A fresh run cannot reuse old group/run/launch nonce. This distinguishes saved valid waiting input from granted start authority at P5.

All seven scenarios then create plain human comments, unresolved `@AI` replies and typed-mention display edits. Each returns no target outcomes and leaves the full execution snapshot and original command rows unchanged. The canonical `TestCollaborationCommentSourceMemberAndPlainRepliesStayHumanOnly` additionally uses an **agent-authored parent**, and proves ordinary reply/unused target versions create no execution. Editing/deleting display in `TestCollaborationCommentInputsShareOneWaitingBatchAndSurviveEditing` preserves original frozen author/content. A source-only human can comment but typed AI feedback is not_participant and does not enroll the author.

Every earlier explicitly cancelled command remains cancelled_by_participant. After comment, edit, poll and Resume, prior commands cannot be rewritten or revived; the stopped and removal/terminal cancellation states have explicit final assertions. All new comments are dispatch_eligible=false and ordinary task rows remain absent. `TestLegacyExecutionCannotInheritCollaborationComments` goes further than a count: actual ordinary service trigger/coalescing writers, SQL trigger/coalescing writers and ordinary claim refuse private shared-comment input, retaining the queued task on claim refusal.

The snapshot uses the tracked full control ledger: channels/targets/groups/runs/grants, attempts/capabilities/reservations/terminations/delegations and session controls. It excludes human events and event_seq legitimately changed by comment creation. Only successful staging appends omit new command/allocation counters; every prior command is separately compared byte-for-byte and the new command is exact. Blocked/inert paths include complete batches. No proof/control/hold/authorization barrier is silently excluded.

## Actual executed evidence

Go1.26.6, darwin/arm64, task-owned `service_s5` schema737, agent CLI guard,
serial `-race -p 1 -parallel 1 -count=1 -timeout=30m`.

| Run | Result |
| --- | --- |
| Focused final `mention-admission-focused-candidate-fixed-20261021` | 1 top-level/all7 states passed, no fail/skip; wall18.353s |
| Canonical adjacency `mention-admission-adjacent-final-20261021` | 8 top-level/23 including subtests passed, no fail/skip; wall21.736s |
| Exact HEAD7794f57d9 + only new test | 1 top-level/all7 states passed, no fail/skip; wall21.194s |
| Fresh coordinator `mention-admission-coordinator-final-20261022` | 8 top-level/23 including subtests passed, no fail/skip; wall14.860s |

Final current command from `server/`:

```bash
GOTOOLCHAIN=go1.26.6 python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s5 \
  bash ../scripts/go-test-with-agent-cli-guard.sh \
  go test -json -race ./internal/service \
  -run '^(TestCollaborationMentionCommentAdmissionAcrossExecutionStates|TestCollaborationCommentCommitPreservesValidTargetAndHumanWhenAnotherTargetWasRevoked|TestCollaborationCommentSourceMemberAndPlainRepliesStayHumanOnly|TestCollaborationCommentInputsShareOneWaitingBatchAndSurviveEditing|TestCollaborationRuntimeHoldBlocksClaimAndPermit|TestCollaborationLifecycleCloseAndReopenPreservePermanentQueue|TestCollaborationQueueResumeRejectionNeverTurnsIntoDeferredIntent|TestLegacyExecutionCannotInheritCollaborationComments)$' \
  -p 1 -parallel 1 -count=1 -timeout=30m
```

The JSON report retains each actual PASS/fail event line, exact argv/results/raw hashes/source provenance and the first unsuccessful attempts. Final current raw SHA-256 is
`0f662a842e70e3c2eb02b4e19a20d6bbd4f65e1503d129ec744988a03c57e92c`;
isolated HEAD raw SHA-256 is
`54ae94bb8c2de128843f6120ebf77af23dd0c9e864fa4a1d1f250c8ebf144ab7`.

The final adjacent source-member/reply, mixed target refusal, edit/frozen-input, hold Claim/Permit, close/reopen, refused Resume and legacy service/SQL/claim identities are actual passing cases. They are assessed by their assertions above; package counts do not replace the criterion.

## Verification

- Lint: reviewer gofmt -d passes with empty output; exact-source recorded go vet -p1 ./internal/service passes with empty diagnostics.
- TypeCheck: final-source package compilation in the actual race commands passes.
- Tests: focused, adjacent, isolated HEAD and fresh coordinator executions pass the cited identities, with no final failures/skips.
- Reviewer execution: reads/gofmt only, no DB/build/browser/runtime commands.

## Commit and acceptance handoff

The isolated snapshot is HEAD `7794f57d969fb583ddf02c830f6d1ab72186595d` plus only
`server/internal/service/collaboration_mention_admission_states_test.go` at the
exact reviewed hash. It passes all seven states without shared dirty product
source. The slice can be committed independently with Lore and its scoped
report/raw/static evidence. Keep actual service admission and synthetic proofs
explicit in Tested/Not-tested; no provider/kernel/native rollout claim follows.

The original AC93 definition and P5 owner are unchanged. Recommend whole-row
passed after this independent review; matrix/catalog/spec/progress/index edits
remain coordinator-owned. Task and parent stay in_progress until their remaining
criteria are satisfied, and production creation/execution gates remain closed.
