# S2 2d continuation independent check — 2026-11-15

## Bounded verdict

**PASS for the three original RED repairs on exact frozen-green-r03. Full 2d is NOT ACCEPTABLE (known pending); S2 is not complete.** No new product counterexample appeared in this bounded check. The original implementation owner is already handling the separate second-batch locking, identity and final-horizon work; this review does not inspect or modify its live bytes.

- Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`.
- Exact source: `/Users/artisan/.multica/acceptance/pay241-20261115-continuation/server-2d/frozen-green-r03/source`; 4,954 regular files match the manifest before and after execution, with zero unlisted/hash/size mismatches.
- Manifest: `/Users/artisan/.multica/acceptance/pay241-20261115-continuation/server-2d/frozen-green-r03/manifest.json`; SHA256 `3e4bc0270a1e675ebb0a9cae4e479853393c6d796266d049f4e07f1728a3e7b3`. All source path/line references in this report mean this immutable r03, not the currently edited checkout.
- The four r02 861/864 submission DDL/query/generated hashes match the supplied source audit. No database was created, migrated, selected from alternatives, or dropped.
- Public `stagePolicyActivationAvailable=false` remains at `server/internal/handler/collaboration_stage.go:21`. No P2 policy was enabled, and no real API/browser/model/CLI/daemon/provider process was launched by this checker.
- This is package/router-fixture verification, not real-human sign-off, actual deployment/payment, production Web/Desktop acceptance or four E2E passes. Main reports r03 startup/auth/HTTP preflight only.

## Findings (fixed)

No product or implementation-test fixes were written by this checker. These three repairs were already made by the implementation owner and are now independently verified:

1. **Nil transaction/guard** — `server/internal/service/collaboration_stage_completion.go:344`. The old final check admitted nil transaction/empty preparation. It now rejects nil transaction or invalid issue/workspace IDs. `TestStageCompletionGuardRequiresPreparedTransaction` passes normal/race.
2. **Omitted managed-root guard** — `server/internal/service/collaboration_stage_completion.go:349`. A nil guard now rechecks current enabled-root state and refuses with `stage_incomplete` / `stage_preparation_required`. `TestStageCompletionGuardCannotBeOmittedForManagedRoot` passes normal/race. Successful submit/complete now calls the final check regardless of guard presence at `server/internal/service/collaboration_workflow.go:622`; R38 does likewise at `server/internal/service/admin_collaboration.go:289`.
3. **Exact 864 ledger** — `server/internal/service/collaboration_stage_completion.go:55`, `server/internal/service/collaboration_stage_policy.go:240` and `server/internal/service/collaboration_stage_policy.go:258`. Enabling holds the exact `864_collaboration_stage_submission_ready` ledger row as well as structural readiness before policy write and before commit. The missing-row seam refuses with zero policy rows; `TestStageCompletionPolicyActivationRequiresExact864Ledger` passes normal/race.

Actual callers were traced: submit prepares/binds at `server/internal/service/collaboration_handoff.go:324`; exact/self completion stores final context at `server/internal/service/collaboration_lifecycle.go:409`; workflow final check follows the original receipt; R38 final check follows audit/history/receipt and immediately precedes commit; ordinary update and system status call the same final function at `server/internal/handler/issue.go:4151` and `server/internal/service/issue_status_write.go:135`. This call-map does not prove the still-missing exact identity/horizon contract.

## Findings (not fixed)

These are **known second-batch source-contract/acceptance gaps, not newly reproduced failures**. The owner already discloses them at `research/server-increment-2d-continuation-20261115-results.md:153`. They block full 2d acceptance. Product/query/test/peer files are outside this checker write scope, so no speculative redesign or duplicate implementation/test work was performed.

- **Precise candidate/submission owning-transaction locks remain.** `server/internal/service/collaboration_stage_completion.go:134` and `server/internal/service/collaboration_stage_completion.go:320` still use the plain reads in `server/pkg/db/queries/collaboration_stage_review.sql:10` and `server/pkg/db/queries/collaboration_stage_submission.sql:5`. Static query/caller confirmation only; no invented new concurrency repro or pass.
- **Actual caller/participant/account and original operation/result binding remains.** The guard at `server/internal/service/collaboration_stage_completion.go:42` and final check at `server/internal/service/collaboration_stage_completion.go:373` need the owner's disclosed exact binding independently of M1. Wrong-tx/reused/unfinalized/corrupt-result/late-authority variants remain pending.
- **Complete final evidence horizons/current source/target checks remain.** Existing final helper and limited CI/decision bounds at `server/internal/service/collaboration_stage_completion.go:247` and `server/internal/service/collaboration_stage_completion.go:260` do not constitute the missing artifact/original issuer/rollback metric and post-receipt final-clock suite. No late-clock/two-order approval is claimed.
- **Combined gate proof remains incomplete.** Stage-enabled all M1 modes, R38, P != E and actual new F06 list/preview/zero-write/ACL/cursor normal/race are pending. The new F06 router tests belong to another lane; none were duplicated or substituted by the old M1 compatibility tests. Basic service self success exists only in the stage-enabled default M1/off fixture. Full self/observe/enforce, R38 and P != E positives remain pending.

## Verification

- Lint: **PASS**, `go vet` for the three exact packages plus read-only `gofmt -l` on 35 owned Go files (empty output).
- TypeCheck: **PASS, scoped Go compilation**, from actual normal/race package tests and vet. No TypeScript/pnpm or whole-server build claim.
- Tests: **PASS, bounded**. Eight actual named-test commands ran; every selected top-level name ran/passed, with zero fail, skip or missing names.

| Run | Package | Top-level PASS | Named PASS records | FAIL/SKIP | Exit |
| --- | --- | ---: | ---: | ---: | ---: |
| continuation-normal-r01 | ./internal/service | 5 | 11 | 0/0 | 0 |
| continuation-race-r01 | ./internal/service | 5 | 11 | 0/0 | 0 |
| completion-normal-r01 | ./internal/service | 8 | 16 | 0/0 | 0 |
| completion-race-r01 | ./internal/service | 8 | 16 | 0/0 | 0 |
| handler-compat-normal-r01 | ./internal/handler | 5 | 12 | 0/0 | 0 |
| handler-compat-race-r01 | ./internal/handler | 5 | 12 | 0/0 | 0 |
| router-compat-normal-r01 | ./cmd/server | 6 | 17 | 0/0 | 0 |
| router-compat-race-r01 | ./cmd/server | 6 | 17 | 0/0 | 0 |

**Counts are not additive:** the 11 canonical named records are contained in the larger completion selection's 16 records, not 27 independent cases. Parent/subtest terminal records, repeated normal/race runs and ordinary/M1 compatibility controls are not an S2 acceptance case total. Handler/router compatibility fixtures do not enable stage policy and are never counted as stage-enabled evidence.

All database commands used only the prescribed `run.mjs` identity launcher and the assigned `configs/check_service.json`, `configs/check_handler.json`, `configs/check_router.json`. Each actual database is `multica_pay241_20261115_s2_check_<profile>`, has 835 ledger rows ending at `864_collaboration_stage_submission_ready`, and has zero other connections before/after. HOME/TMP paths were proven outside Git. Same-DB normal/race commands were sequential. No `server ./...` or F06 acceptance harness ran.

Exact commands, times, selected top-level names, all terminal names, configuration/launcher/source fingerprints and raw command/stdout/stderr/exit/result hashes are in the JSON companion and `/Users/artisan/.multica/acceptance/pay241-20261115-continuation/server-2d-check/logs`. The actual eight test commands were:

```sh
# continuation-normal-r01
node /Users/artisan/.multica/acceptance/pay241-20261113-01/run.mjs /Users/artisan/.multica/acceptance/pay241-20261115-continuation/configs/check_service.json go -C /Users/artisan/.multica/acceptance/pay241-20261115-continuation/server-2d/frozen-green-r03/source/server test ./internal/service -run "^TestStageCompletion(Guard|PolicyActivation|StoredBlockers|Activity)" -count=1 -json

# continuation-race-r01
node /Users/artisan/.multica/acceptance/pay241-20261113-01/run.mjs /Users/artisan/.multica/acceptance/pay241-20261115-continuation/configs/check_service.json go -C /Users/artisan/.multica/acceptance/pay241-20261115-continuation/server-2d/frozen-green-r03/source/server test ./internal/service -run "^TestStageCompletion(Guard|PolicyActivation|StoredBlockers|Activity)" -count=1 -json -race

# completion-normal-r01
node /Users/artisan/.multica/acceptance/pay241-20261113-01/run.mjs /Users/artisan/.multica/acceptance/pay241-20261115-continuation/configs/check_service.json go -C /Users/artisan/.multica/acceptance/pay241-20261115-continuation/server-2d/frozen-green-r03/source/server test ./internal/service -run "^TestStageCompletion" -count=1 -json

# completion-race-r01
node /Users/artisan/.multica/acceptance/pay241-20261113-01/run.mjs /Users/artisan/.multica/acceptance/pay241-20261115-continuation/configs/check_service.json go -C /Users/artisan/.multica/acceptance/pay241-20261115-continuation/server-2d/frozen-green-r03/source/server test ./internal/service -run "^TestStageCompletion" -count=1 -json -race

# handler-compat-normal-r01
node /Users/artisan/.multica/acceptance/pay241-20261113-01/run.mjs /Users/artisan/.multica/acceptance/pay241-20261115-continuation/configs/check_handler.json go -C /Users/artisan/.multica/acceptance/pay241-20261115-continuation/server-2d/frozen-green-r03/source/server test ./internal/handler -run "^(TestStagePolicyHTTPEnablingIsUnavailableUntilEveryGateExists|TestDeliveryCompletionR38UnenforcedLegacyEvidenceRemainsAdministrative|TestDeliveryCompletionR38BindsActualPendingSubmission|TestDeliveryCompletionCollaborationActualSourceAndOriginalRecovery|TestDeliveryCompletionCollaborationGrantAndRefusalDoNotBindAttachments)$" -count=1 -json

# handler-compat-race-r01
node /Users/artisan/.multica/acceptance/pay241-20261113-01/run.mjs /Users/artisan/.multica/acceptance/pay241-20261115-continuation/configs/check_handler.json go -C /Users/artisan/.multica/acceptance/pay241-20261115-continuation/server-2d/frozen-green-r03/source/server test ./internal/handler -run "^(TestStagePolicyHTTPEnablingIsUnavailableUntilEveryGateExists|TestDeliveryCompletionR38UnenforcedLegacyEvidenceRemainsAdministrative|TestDeliveryCompletionR38BindsActualPendingSubmission|TestDeliveryCompletionCollaborationActualSourceAndOriginalRecovery|TestDeliveryCompletionCollaborationGrantAndRefusalDoNotBindAttachments)$" -count=1 -json -race

# router-compat-normal-r01
node /Users/artisan/.multica/acceptance/pay241-20261113-01/run.mjs /Users/artisan/.multica/acceptance/pay241-20261115-continuation/configs/check_router.json go -C /Users/artisan/.multica/acceptance/pay241-20261115-continuation/server-2d/frozen-green-r03/source/server test ./cmd/server -run "^(TestCollaborationStageRoutesAreRegisteredAndEnablingStaysUnavailable|TestDeliveryCompletionCollaborationRouterActualSourceAndRecovery|TestDeliveryCompletionEnforcementRouterOrdinaryAndMove|TestDeliveryCompletionEnforcementRouterBatchCommittedPrefix|TestDeliveryCompletionEnforcementRouterCreateAndCommentChild|TestDeliveryCompletionWebhookRouterExactGrantConsumesWithState)$" -count=1 -json

# router-compat-race-r01
node /Users/artisan/.multica/acceptance/pay241-20261113-01/run.mjs /Users/artisan/.multica/acceptance/pay241-20261115-continuation/configs/check_router.json go -C /Users/artisan/.multica/acceptance/pay241-20261115-continuation/server-2d/frozen-green-r03/source/server test ./cmd/server -run "^(TestCollaborationStageRoutesAreRegisteredAndEnablingStaysUnavailable|TestDeliveryCompletionCollaborationRouterActualSourceAndRecovery|TestDeliveryCompletionEnforcementRouterOrdinaryAndMove|TestDeliveryCompletionEnforcementRouterBatchCommittedPrefix|TestDeliveryCompletionEnforcementRouterCreateAndCommentChild|TestDeliveryCompletionWebhookRouterExactGrantConsumesWithState)$" -count=1 -json -race
```

The original `continuation-red-r01` (8 named PASS / 3 FAIL), `continuation-green-r01` (11 PASS), `completion-baseline-r02` (16 PASS) and `draft-admission-r01` (3 PASS / 2 FAIL records) were hash/count checked and retained. The draft `deploymentunknown` failure is not attributed or rewritten to green. Main separately reports DDL **39/39 PASS (6 old raw + 1 supplement)**; that is external root-owned proof, not this checker's test/migration count.

## W01–W18 bounded classification

`coveredactualtest` means only the described slice has an actual frozen-r03 named test. `codeonly` means a frozen caller/query/adapter was read but the required behavior was not demonstrated here. `notproven` means the required r03 proof is absent. **No row below establishes its complete requirement.** This reuses the existing W/Q denominator; it is not a new full-phase proof campaign.

| ID | State | Actual bounded boundary | Still pending |
| --- | --- | --- | --- |
| W01 | coveredactualtest | Stage-enabled M1/off submit refusal and verified submission with one immutable binding. | Observe/enforce, attachments and final-clock/locking combinations. |
| W02 | coveredactualtest | Stage-enabled M1/off exact submission completion and original-key replay. | Enforce terminal consumption, legacy/corrupt binding and late-change combinations. |
| W03 | coveredactualtest | Stage-enabled M1/off self-result refusal, successful completion, one binding and replay. | Stage-enabled observe/enforce self positives, attachment variants and final-clock cases. |
| W04 | codeonly | Actual R38 writer invokes final guard after history and original receipt at admin_collaboration.go:290. | All stage-enabled R38 positives/negatives, org/caller/final-clock and P != E controls. |
| W05 | coveredactualtest | Ordinary M1 PUT compatibility only; the S2 stage policy is not enabled in these fixtures. | Stage-enabled root/old-client/Agent boundary and independent child combinations. |
| W06 | coveredactualtest | Ordinary M1 Move compatibility only; not stage-enabled proof. | Stage-managed root/child semantics and concurrent candidate staleness. |
| W07 | coveredactualtest | Ordinary M1 sequential batch committed-prefix compatibility only. | Stage root batch pre-effects refusal and repeated-grant/key controls. |
| W08 | coveredactualtest | Ordinary M1 create/comment-child/create-as-done compatibility only. | Stage-enabled human/machine decomposition, inherited project and child controls. |
| W09 | coveredactualtest | Ordinary signed GitHub exact-grant M1 completion compatibility only. | Stage-enabled root/system refusal and VCS/current-trust combinations. |
| W10 | notproven | No CLI/daemon/provider process was started by this checker. | Matching CLI/API and run-local machine completion boundary. |
| W11 | codeonly | EditIssue calls CheckDeliveryProjectChangeInTx at collaboration_edit.go:98. | Stage x M1 exit/clear refusals and policy/complete contention. |
| W12 | codeonly | Impact calls CheckStageProjectDeleteInTx at project.go:846; delete calls CheckDeliveryProjectDeleteInTx at project.go:920. | Actual stage-enabled impact/delete responses and lock contention. |
| W13 | notproven | No managed root ordinary/batch delete control selected in this bounded run. | Pre-cancel/pre-cleanup root and ordinary-parent/collaboration-child refusal. |
| W14 | notproven | Earlier 2c admission evidence is not promoted to r03 acceptance. | Child semantic/parent/project/delete staleness at actual completion consumption. |
| W15 | notproven | No enqueue/start suite rerun; original failed draft-admission evidence remains retained and unattributed. | Combined lifecycle/admission guard and both real commit orders. |
| W16 | codeonly | Workflow final-stage call is restricted to successful submit/complete; actual cancel branch remains separate. | Stage-enabled cancel/reopen/new scope, original consumed-key recovery and approval non-revival. |
| W17 | codeonly | Four F06 adapters/routes and explicit child-source/64 KiB/five-second handling read in frozen source. | New F06 list/preview service/handler/router normal/race, done child, ACL, cursor and zero-write proof; another lane owns these tests. |
| W18 | coveredactualtest | Closed safe activity metadata and original stored stage-blocker GET/replay with no added event. | Actual HTTP/activity/outbox/timeline, all invalid retained variants and concurrent authority. |

W states: 9 coveredactualtest / 5 codeonly / 4 notproven, not a completion ratio. Ordinary controls in W05–W09 prove only their stated M1 compatibility.

## Q01–Q12 bounded classification

| ID | State | Actual bounded boundary | Still pending |
| --- | --- | --- | --- |
| Q01 | coveredactualtest | Only stage-enabled default M1/off submit/exact/self positives and refusals. | Full off/observe/enforce x submit/submission/self/R38. Old M1 router/handler green does not prove stage-enabled behavior. |
| Q02 | coveredactualtest | Nil transaction/guard and omitted managed-root preparation refuse at actual final guard. | Wrong tx, reused/unfinalized guard, precise caller/operation/result and allowed-after variants. |
| Q03 | notproven | Final call location is read after original receipt, but no complete final-clock or two-order proof. | Known second-batch artifact/issuer/rollback-metric horizons and caller/role/participant/account/source/trust races. |
| Q04 | coveredactualtest | Successful submit/self retains exactly one immutable stage submission row. | Precise candidate/submission row locks and missing/legacy/corrupt semantic/operation bindings. |
| Q05 | notproven | Distinct issuer/consumer M1 controls ran without stage enabled; they are compatibility evidence only. | Stage-enabled P != E, outside-source consumer, revoked/expired issuer/consumer, admin org and privacy controls. |
| Q06 | codeonly | New F06 read adapters and five-second transaction wiring inspected. | Zero authorization/evaluation/submission/status/attachment/queue writes and done-child exact source/root binding. |
| Q07 | codeonly | F06 handler rejects query on preview, enforces 64 KiB and delegates strict parse/bounded candidate query. | Actual new endpoint strict malformed bodies, paging/cursor/empty and mid-revocation normal/race. |
| Q08 | coveredactualtest | Original stage-incomplete refusal retains status/blockers through policy disable and GET/replay without events; completed same key replays. | Commit unknown, late required-write failure, reopen/current-authority and actual lost-response HTTP. |
| Q09 | coveredactualtest | Missing exact 864 ledger seam refuses policy write with zero policy rows; public activation remains false. | Full writer readiness, table/index/trigger/partial migration and safe final activation; no P2 was enabled. |
| Q10 | notproven | No combined policy/project completion/contention or ordinary-child independence campaign run. | Both commit orders and stage x M1 project/target/lifecycle controls. |
| Q11 | codeonly | 4954-file immutable identity and four exact r02 DDL/query/generated hashes verified before/after. | DDL is separately checked by root; main reports 39/39 PASS (6 old raw + 1 supplement), not this checker's migration/test count. |
| Q12 | notproven | Exact manifest, three permitted DB profiles, original failures and zero owned connections retained. | Four real Web/Desktop scenarios and their final API/CLI/Host/source/config/gate/resource proof; current root startup/auth preflight is not stage-enabled acceptance. |

Q states: 5 coveredactualtest / 3 codeonly / 4 notproven, not an acceptance percentage. Full stage-enabled Q01/Q05, final-clock Q03 and new F06 Q06/Q07 remain pending.

## Controlled integration boundary and handoff

- Exact r03 can support bounded startup/auth/HTTP preflight and existing ordinary/M1 compatibility, with public stage activation kept false; the synthetic stage-enabled package fixture results are retained separately. **Do not accept complete 2d, enable P2, mark four Web/Desktop scenarios PASS, archive S2 or commit on this result.**
- Next independent review must bind the original owner's separately immutable second-batch source, close the three known contract gaps and consume the dedicated F06 lane's actual route evidence; rerun the changed-source final-clock/rollback/authority/M1/self/R38/P != E controls. Do not test the current live peer files to make r03 pass.
- Main owns eventual spec synchronization for `.trellis/spec/server/collaboration-stage-approvals.md` and `.trellis/spec/server/delivery-completion-enforcement.md` after executable future contracts are proven. Current activation/full-writer warnings must remain. Shared specs were not changed here.
- Changed files: only this MD/JSON pair and private evidence/helper under `/Users/artisan/.multica/acceptance/pay241-20261115-continuation/server-2d-check`. Simplifications: none; no product, implementation-test, peer-file or dependency change. Zero owned test processes/connections remain. No Git commit.

