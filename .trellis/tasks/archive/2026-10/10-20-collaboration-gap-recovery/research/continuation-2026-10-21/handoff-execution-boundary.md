# AC06/16/28/30 handoff and execution boundary

Status: new missing backend assertions and canonical adjacency pass. AC06/16/28/30 now have current backend-primary evidence for the previously missing clauses; root owns independent audit/promotion and no ledger is changed by this lane.

Change boundary: one new `server/internal/service/collaboration_handoff_execution_boundary_test.go` plus this report and unique verification logs. Existing production/helpers, SQL/spec/matrix, other agents' source and runtime/native services remain read-only. Tests reuse the actual managed fixture and workflow/consent/control/status/comment/protocol services. Synthetic sealed service proof is not kernel/process evidence.

The two gaps are (1) creator/current/former handler each executing submit→Stop→Resume after actual invite/transfer, including the newly invited actor using the unchanged existing grant with frozen original-author inputs, and (2) actual managed-permit execution surviving SubmitAcceptance, recording AI success and a human stage/test-pass comment without whole completion. Existing canonical tests supply squad/all-owner/no-extra-target, paused acceptance, information return and explicit creator/admin lifecycle matrices; the new tests cover their missing current positive boundaries rather than duplicating those matrices through UI.

Each run uses the coordinator's exclusive schema-737 `service_s5` DB slot with the ambient agent CLI guard, serial race and exact source hashes. Any first failed attempt stays recorded; the final report will identify only clauses actually proven and leave native/provider/packaging boundaries explicit.

## Final implementation and behavior

Changed source: new `server/internal/service/collaboration_handoff_execution_boundary_test.go`, SHA-256 `2711e8e6aea0e111f5438834e1bb5f116c2b1ffb7b071d4b9c39e8b434baefb6`. No existing production/helper change, dependency, migration, generated SQL, spec/matrix or other agent's code was made.

`TestCollaborationTransferredHandlersShareExistingGrantAndControls` creates a real managed collaboration/owner grant, invites an actual new password participant through `InviteParticipant` and transfers responsibility through `Transfer`. Former participation remains active; the newcomer gains no source membership or personal resource grant. The exact pre-existing owner grant and target/channel/session remain unchanged by invitation/transfer. Original transfer receipt is recovered through the canonical actor-owned GET lookup; current transfer privilege is not granted back to the former handler.

Creator, current handler and former handler each then use the **same unchanged grant** to submit an exact-author input, activate the queue, claim/permit through the managed protocol, Stop its original run, receive immutable `stop_unconfirmed` on early Resume, accept the original synthetic proof and explicitly Resume with a fresh request. The refused old key stays refused and cannot become future intent. Each command/author/body/issue/target/claim identity remains exact; its old stopped input stays terminal. No per-participant reconsent occurs and counts remain one target/one grant. The current handler is the newly invited account, so this is a real positive AC16 newcomer-use test rather than another established participant.

`TestCollaborationActiveAcceptanceAndResultsRequireExplicitCompletion` submits actual handler input, resumes, claims and issues a real managed launch permit. With the run still unknown/unsettled and a retained waiting input, `SubmitAcceptance` moves responsibility/status to creator/in_review while the complete execution-owned snapshot remains identical. Original run/grant/prompt/attempt/capability/reservation/termination state is retained; no Stop/proof is manufactured, and waiting input remains waiting. Former handler retains submit/comment/control but no transfer/new acceptance privilege. Original acceptance replay is unchanged.

The real permitted business capability then successfully records AI success after acceptance; original proof settles only that execution. A human former-handler stage/test-pass comment is actually persisted through `CreateComment`, carries no AI targets and changes none of the execution rows. Issue stays in_review, active, lifecycle epoch1, with creator responsible. A former-handler complete is rejected; only a fresh explicit creator lifecycle operation marks done, closes the session and cancels the waiting input. This closes the exact AI success+human test-pass+active-acceptance missing correlation. Existing adjacent information return/stage/paused acceptance/admin tests supply the remaining clauses at their canonical layer.

The narrow execution snapshot includes full session, channel, target, grant, command, batch, group, input/dependency, run, termination, attempt, capability and reservation rows. It excludes only `collaboration_session.event_seq`, because human invite/transfer/acceptance/comment events legitimately advance that common ordering fact. It does not strip queue/control/hold/proof or resource state. Full `controlStateSnapshot` is used when checking exact replay/no-op state.

## Retained attempts

| Attempt | Actual outcome | Explanation |
| --- | --- | --- |
| `handoff-execution-boundary-focused-first-20261021` | Exit1, both new tests fail | The new test compared session event_seq during legitimate human events. Receipts were success; snapshot assumption was wrong. Only the test snapshot was narrowed. |
| `handoff-execution-boundary-focused-fixed-20261021` | Exit1; active acceptance/results test passes, transfer test fails before role actions | Test incorrectly re-POSTed Transfer after losing current-handler role. Product correctly forbids; recovery uses existing actor-owned GetWorkflowOperation. |
| `handoff-execution-boundary-focused-recovery-20261021` | Exit0, 2 top-level/5 including subtests pass, no fail/skip | Correct snapshot and GET recovery; package4.569s. |
| `handoff-execution-boundary-adjacent-final-20261021` | Exit0, 14 top-level/36 including subtests pass, no fail/skip | Final current test plus canonical boundaries, package11.856s. |

The first two are fixture expectation errors, not current product failures or evidence that source authorization should be relaxed. Raw logs and exact command/result/source hashes remain; no filename is used as a substitute for actual events.

## Commands and evidence

All DB commands ran from `server/` with `GOTOOLCHAIN=go1.26.6`, isolated `service_s5` through migration737, ambient agent CLI guard, `-race -p 1 -parallel 1 -count=1 -timeout=30m`. No provider executable/account is resolved or launched. Existing fixture gate state is changed/restored only in that isolated test DB; production gates/services and native slots remain unchanged. The DB slot was released immediately after final result to the source-authority lane.

```sh
GOTOOLCHAIN=go1.26.6 python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s5 \
  bash ../scripts/go-test-with-agent-cli-guard.sh go test -json -race ./internal/service \
  -run '^(TestCollaborationTransferredHandlersShareExistingGrantAndControls|TestCollaborationActiveAcceptanceAndResultsRequireExplicitCompletion|TestCollaborationWorkflowInviteTransferPreservesExecution|TestCollaborationQueueRejectsStaleIdentityAndMachine|TestCollaborationTargetStopLeavesOtherTargetAndIssueActive|TestCollaborationQueueResumeRejectionNeverTurnsIntoDeferredIntent|TestCollaborationOwnerConsentIsExplicitScopedAndIdempotent|TestCollaborationConsentUsesCompleteSquadRoster|TestCollaborationResourceOfferSquadStillRequiresEveryOwnerConsent|TestCollaborationSquadInputRechecksEveryResourceConsent|TestCollaborationAcceptanceSubmissionReturnsOpenIssueToCreator|TestCollaborationInformationChainReturnsLayerByLayer|TestCollaborationLifecycleAuthorityAndExplicitReopen|TestAdminCollaborationLifecyclePreservesCreatorAndRealActor)$' \
  -p 1 -parallel 1 -count=1 -timeout=30m
```

Final result: exit0, all14 expected top-level identities pass, 36 total pass identities, zero fail/skip, package11.856s, elapsed16.790s, empty stderr, start `2026-10-21T14:13:26.074643+00:00`. Exact commands/results are `handoff-execution-boundary-adjacent-final-20261021-command.json`/`-result.json`/`.jsonl`/`.stderr`; raw SHA-256 `93e09c491eb014ecb1b3a5ea1c9362fcfbfa46041193d5e8f1933e10f58d5c1d`. All eleven recorded production/helper/test hashes stayed unchanged. Focused successful raw SHA-256 `61fdfdc895616be19c55fef6c30ac37663c02a721cbc0dbb0f3131e26fca79c2`; earlier red raw hashes `2e18bf1d1b4fe4462fb8fbe4f0313fb641a6fb12a4491eadbe8a64e7d50ee858` / `aa331b0a9d26ba48fb01f98034f3bc2d32245b264cf9f0c7a70f6c4fd53a2232` remain intact.

The final selector includes original canonical gaps from AC06/28/30 as actual passes, rather than only source inventory: stale/machine queue denial, per-target Stop, no-deferred Resume, paused acceptance and information-chain return. AC16 complete squad owner consent/no-independent-member target/no implicit owner invitations is verified by the current original owner/roster/offer/squad tests. Explicit creator/admin lifecycle authority is independently exercised. Results do not treat the canonical tests' fixture run rows as native process evidence.

Fresh static commands:

```sh
# From repository root:
gofmt -d server/internal/service/collaboration_handoff_execution_boundary_test.go
# From server/:
GOTOOLCHAIN=go1.26.6 go vet -p 1 ./internal/service
```

Both exit0 with no output/diff, vet1.634s, source unchanged. Command/streams are retained in `handoff-execution-boundary-static-final-20261021*`. TypeScript is not applicable to this Go-only slice; no full-suite or browser revalidation was repeated.

## Remaining limits and handoff

This lane proposes full **backend-primary** AC06/16/28/30 coverage based on the new tests plus the exact canonical adjacency. Root owns independent review, protected matrix changes and Lore staging/commit. No new feature or source defect remains to fix. The new single file uses tracked managed, workflow/control and status/comment fixtures; root can prove HEAD+only file isolation outside the dirty checkout before committing.

Real provider processes, natural kernel termination proofs, native Web/Electron flows, installer/full Main and other-OS/runtime/storage fault acceptance are untested here and remain separately attributed. The actual service-issued permit is a live authorization/barrier assertion, not proof that a provider ran. AI success and terminal proof in this suite are synthetic service protocol inputs. No account quota/rollout/deployment or parent archive is authorized or performed.

Changed files: one new service test, this report and unique handoff-execution-boundary evidence streams/JSON. Simplification: reused current managed/workflow/status/queue primitives; no production utility or layer was added.
