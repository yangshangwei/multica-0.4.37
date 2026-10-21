# AC93 direct comment mention admission states

Change boundary: add only `server/internal/service/collaboration_mention_admission_states_test.go`, this report and unique `mention-admission-*` evidence. Existing products/helpers/specs/matrix/catalog/progress/index and other agents remain read-only. This executes the current S6 plan; it does not create or replan a task.

The missing clause is the complete direct `CreateComment` typed @ trigger correlation across idle, busy, paused, unconfirmed Stop, resource revocation, removed author and terminal issue, rather than only independently exercising SubmitCommand/Stop primitives. Reuse tracked managed/queue/comment/status/control fixtures and service-issued Claim/Permit; keep exact comment/event/command/target/channel/batch correlation and no ordinary dispatch. Plain comments/replies and even typed-mention edits remain execution-inert; unknown Stop retains its original proof/capacity and cannot gain deferred Resume intent. Sealed outcomes/proofs are synthetic service input; no provider is resolved or launched.

The coordinator exclusively reserved migration737 `service_s5` for this lane. Commands use the ambient CLI guard and serial race; all first failures/source hashes remain. The lane releases the slot immediately after final DB verification and performs no later DB command. Root owns independent review, acceptance promotion, spec integration and Lore commit.

## Final behavior and exact clause

`TestCollaborationMentionCommentAdmissionAcrossExecutionStates` directly drives typed `[@A](mention://agent/<actual UUID>)` through CreateComment in each of AC93's seven named states. Every setup command is itself created through the comment service; SubmitCommand is not used by this test. The actual resource is registered and consented by the tracked managed fixture.

| State / subtest | Current receipt and execution behavior |
| --- | --- |
| `idle` | Typed comment persists once and returns waiting/command_queued with original event/command/author/target/channel/batch. Before List/Claim/Permit it has no group or launch identity. The managed daemon later claims the exact frozen body/author/input and obtains one persisted attempt. |
| `busy` | A real prior typed-comment Claim/Permit stays unknown/unsettled and has no forced Stop. New comment gets a separate waiting batch. Repeated poll returns only the original recoverable run; it creates no second run and does not inject the new input into the current prompt. Synthetic success/proof of original then allows one new exact-input Claim/Permit. |
| `paused` | An explicit whole-issue Stop precedes @. Comment returns waiting with issue_paused, changes no control/hold or execution state, and repeated poll creates no run. Only a fresh explicit whole Resume precedes actual exact-input Claim/Permit. |
| `unconfirmed_stop` | Actual target Stop of a permitted original precedes @. New typed comment waits with target_paused. Original run remains stopped/unknown/unsettled with unreleased reservation and unconfirmed termination. Resume refuses stop_unconfirmed without state change. Synthetic original proof leaves queue paused and old refused key unchanged; only fresh explicit Resume starts new input. Original stopped command stays stopped. |
| `revoked_resource` | Actual RevokeGrant stops the existing permitted original and preserves prior waiting input. New typed @ comment persists for the human, but its target is blocked/target_unavailable with no command/batch/group. Poll cannot launch. Snapshot retains grant/hold/proof/capacity and old waiting input. |
| `removed_author` | RemoveParticipant removes the external active author and cancels its pending input, leaving original stopped/unknown proof/reservation. Removed author tries typed @ CreateComment and receives not_found with no comment, operation or input. Remaining creator's plain/reply/typed edit remains authorized and inert. |
| `terminal_issue` | Explicit creator completion closes the whole issue while original permitted run awaits proof and pending input becomes cancelled/issue_completed. A current participant can save human typed @ content, but target is blocked/issue_closed with no command/batch/group. Poll, plain comments/replies and typed edits do not reopen, resume or clear the original barrier. |

Across all seven cases, an earlier actual comment command is explicitly cancelled through CancelWaitingCommand and stays cancelled_by_participant. Typed comment receipt event/sequence/issue/author/body/parent are correlated to actual comment and collaboration_event rows; every accepted command matches the same original event/author/body/typed target/channel/batch/sequence. Managed Claim checks exact command, author, original body, issue/session/target/channel/batch/run and immutable collaboration_group_input.consuming_run_id. A typed display edit cannot change that frozen body.

Plain comments and unresolved-name replies carry deliberately present but unused target version observations and return no target outcomes. Editing an actual typed comment (or the new plain comment for the removed-author case) returns no targets and changes none of the execution snapshot or old commands. Every comment remains dispatch_eligible=false and the ordinary agent_task_queue count stays zero. Current adjacent legacy service/SQL coalescing/claim refusals supply the actual ordinary-writer boundary.

The private snapshot reuses the existing complete controlStateSnapshot. It excludes human event rows and only session.event_seq, because comments legitimately create events. For a valid new input only, it also excludes new commands and two channel allocation counters; the caller separately compares every prior command byte-for-byte and the new command exactly. Full channel control/pause/group, target hold, grants, runs/attempts/business capabilities/reservations/terminations remain compared. For blocked/inert paths it also compares full batches; no fields representing proof/control/permission are excluded. Idle/busy/paused/proof cases obtain actual service-issued permits; synthetic outcome/proof are never described as provider execution.

## Retained attempts

| Attempt | Actual result | Explanation |
| --- | --- | --- |
| mention-admission-focused-first-20261021 | Exit1,0 pass/8 fail identities | New assertion used nonexistent consuming_root_run_id. Generated schema identifies consuming_run_id. Only new test query corrected; no product/DB schema changed. |
| mention-admission-focused-schema-fixed-20261021 | Exit1,6 state passes/2 fail identities | Test assumed busy candidate list empty. Actual candidate query legitimately returns the original permitted run for recovery. Test now requires exact original run, no new run and new input still waiting. |
| mention-admission-focused-candidate-fixed-20261021 | Exit0,1 top-level/8 including seven states, zero fail/skip | Final source directly passes every AC93 state; package7.035s, elapsed18.353s. |
| mention-admission-adjacent-final-20261021 | Exit0,8 top-level/23 including cases, zero fail/skip | Final source and all7 canonical neighbors pass; package10.529s, elapsed21.736s. |

First failures are new fixture/query expectations, not current product failures. Raw JSONL and original source hashes are retained; their results are not counted as final acceptance evidence.

## Commands and provenance

All database commands ran from server/ with GOTOOLCHAIN=go1.26.6 and exclusively reserved task-owned migration737 service_s5. Wrapper and guard paths were verified before execution. No provider executable/account, native environment or production switch was accessed.

```sh
GOTOOLCHAIN=go1.26.6 python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s5 \
  bash ../scripts/go-test-with-agent-cli-guard.sh go test -json -race ./internal/service \
  -run '^(TestCollaborationMentionCommentAdmissionAcrossExecutionStates|TestCollaborationCommentCommitPreservesValidTargetAndHumanWhenAnotherTargetWasRevoked|TestCollaborationCommentSourceMemberAndPlainRepliesStayHumanOnly|TestCollaborationCommentInputsShareOneWaitingBatchAndSurviveEditing|TestCollaborationRuntimeHoldBlocksClaimAndPermit|TestCollaborationLifecycleCloseAndReopenPreservePermanentQueue|TestCollaborationQueueResumeRejectionNeverTurnsIntoDeferredIntent|TestLegacyExecutionCannotInheritCollaborationComments)$' \
  -p 1 -parallel 1 -count=1 -timeout=30m
```

Final pass includes all8 expected top-level identities, including named legacy service_trigger/service_coalesced/sql_trigger/sql_coalesced/claim cases. Each command/result manifest retains exact args, cwd, timestamps, before/after new/source hashes and raw JSONL. Empty stderr and package PASS were checked. Final DB slot was immediately released to root; no later DB command was run.

| Evidence | SHA-256 |
| --- | --- |
| Final new test source | `07338b91526b867f32e0fdff2865b2c42d9bc4b8949d8fce03b9f029eede6f57` |
| Focused green JSONL | `5ebcced6881e9140fdc8ec8e240d7ae9633d74dc1c498d270829c3317b1ee93d` |
| Final adjacency JSONL | `586f0fd5fd06aae1cd3e1aab40b23419fa86308ace139dd95d307cfe1037c334` |
| First failed JSONL | `18bf83f49f818134607d7945d1c5c495ce856cbb6592a4d7e570d0b833d483c6` |
| Schema-fixed failed JSONL | `2fd73abba2644e4484b43dc2857b414668f7e81c7027c7d95aa7d86855455b8f` |

Final source is byte-identical in focused green and adjacency. All18 read-only production/helper/canonical files from mention-admission-states-baseline-20261021.json remain unchanged; no existing source/migration/dependency was modified. Both earlier source snapshots are recorded in their manifests, not relabeled as final source.

Fresh static commands:

```sh
# Repository root:
gofmt -d server/internal/service/collaboration_mention_admission_states_test.go
# server/:
GOTOOLCHAIN=go1.26.6 go vet -p 1 ./internal/service
```

Both exit0, empty diagnostics/diff, source unchanged. Raw outputs and timings are retained in mention-admission-static-final-20261021*. TypeScript/frontend lint and browser/Electron were not applicable to this Go-only slice and were not rerun.

## Recommendation and limits

Recommend AC93's complete direct trigger matrix as passed at primary owner P5 after independent review, using the new exact seven cases plus actual adjacent hold/lifecycle/replay/legacy boundary results. This lane does not edit or promote the matrix. Root owns current-source/HEAD isolation, spec integration, evidence catalog and the verified Lore commit; child/parent remain in_progress.

Changed files: only the new service test, this report and unique mention-admission evidence files. Simplification: tracked service/fixture/snapshot/status/control primitives are reused; no product abstraction/dependency/migration/generated SQL added. Current product defect: none found. Final source is an acceptance regression for existing behavior, not a new behavioral feature.

Actual managed service writes/permits/proof barriers are verified, while AI success and sealed termination proof remain synthetic protocol inputs. This suite does not launch a provider or prove natural kernel termination, unsupported OS containment, installed Main/installer, paid provider version/account/quota or rollout. It adds no new Web/Electron evidence; those full/client/native criteria remain separately attributed. Production creation/execution gates stay closed.

## Implementer self-review

The exact original AC93 clause at parent acceptance.md:105/P5 was reread against final source and actual adjacent PASS entries: direct typed @ in all7 states; current permission, Stop/proof and terminal fences; no ordinary fallback; distinguish saved waiting versus actual permitted input; plain comments/replies and even typed edits remain inert. The new test never calls SubmitCommand. The adjacent source-member/plain-reply canonical includes a plain reply to an agent-authored parent; the legacy boundary runs actual service and SQL coalescing plus claim refusals, not only a queue-count assertion. Every previously recorded missing_current_test_evidence identity is an actual PASS in the final scope.

No remaining whole-clause gap at the assigned P5 boundary or product defect was identified by this implementer self-review. Report/JSON/new-source whitespace, all seven executed state identities, raw/source hash consistency and valid JSON were checked after writing. This self-review does not replace a separate checker; root retains independent audit/promotion/commit responsibility. The database remains released with no further DB command.
