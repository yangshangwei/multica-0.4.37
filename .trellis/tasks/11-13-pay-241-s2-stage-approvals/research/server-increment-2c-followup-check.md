# S2 increment 2c — independent correction recheck

- Reviewer: `/root/s2_check_resume`, 2026-11-14.
- Verdict: **PASS for the bounded correction; S2C-C01 and S2C-C02 closed.** No open product finding remains from the original2c review. The same server owner may proceed to2d. This does not accept the complete S2/A01–A12 journey or enable policy activation.
- Source: the reviewed4,938-file S2-only base plus the explicit three-file follow-up, independently reconstructed as4,939 files. No live shared files, peer knowledge source,844+ migration or prior private diagnostic were copied into the reconstruction.
- Ownership: this report, its JSON companion and private verification evidence only. No product/test fix, migration, configuration, gate or Git write by this reviewer.

## Findings (fixed)

### S2C-C01 — one post-callback finalization deadline

`collaboration_stage_execution.go:enterStageExecution` now creates one five-second background context after the callback and passes that exact context to authenticated Record and the internal unknown fallback. The fallback cannot renew the deadline. The mutually exclusive pre-entry cancellation/expiry branch has its own bounded five-second, zero-call `not_started` cleanup.

The canonical tests compare actual transaction-context deadlines for normal fallback, caller cancellation after the callback, and unavailable fallback. Failed bounded finalization returns an error and leaves original pending history; it does not claim persisted unknown/success or repeat execution. Independent checks also retained the existing pre-entry cancellation, panic/transport uncertainty and late authenticated historical-result behavior.

### S2C-C02 — R4 checks the full original issuer chain

`collaboration_stage_execution_view.go:stageExecutionReviewState` now passes the full chain returned by `stageExecutionApprovalChain` through the existing `lockStageFinalSigners` check before reporting `valid`. Each original issuer's current M1 artifact-read authority is required; the current reader's manager privilege cannot substitute for it.

The canonical release and rollback tests first establish a valid chain, then demote the original code reviewer while preserving the current release reader's own artifact access. R4 retains all history, marks the affected transitive approvals non-valid and keeps the corresponding hint disabled, including rollback. Actual Prepare remains refused with `stage_signer_changed` and no additional callback. This aligns approval-state projection with the existing execution guard; it does not close a newly discovered execution bypass.

## Findings (not fixed)

No remaining C01/C02 issue was found. The first broad implementation race run remains a separate, unattributed verification failure:48 named passes and one55P03 at a `github_installation` SHARE NOWAIT lock. The later single-test recheck and complete49-test race run passed without source/assertion/timeout/pool changes.

Root's retrospective audit found no then-active backend, lock or vacuum. Later autovacuum timestamps cannot identify the earlier blocker. This recheck preserves the original failed run and the unknown cause; it does not relabel it as an autovacuum failure, a demonstrated product defect or a passing run.

## Verification

The checker independently verified the result binding, three overlay/base hashes, all4,939 frozen files, ten raw log/direct-exit pairs, capture metadata, original-review references and retrospective lock-audit hash. Canonical red evidence contains seven named failures against the old product bytes, including renewed deadlines and both release/rollback projection errors; the unchanged canonical test file subsequently passes.

| Independent check | Result |
| --- | --- |
| Focused service normal |15 PASS,0 FAIL,0 SKIP |
| Same focused service `-race` |15 PASS,0 FAIL,0 SKIP |
| Service/handler/router `go vet` |PASS, empty output |
| Three-file `gofmt -l` |PASS, empty output |
| Final frozen source audit |4,939 matching hashes; no differences |

The focused selection includes the seven canonical regression names plus existing pre-entry cancellation, ambiguous commit, panic/transport, late history, original issuer revocation and unavailable/source-denied controls. Counts overlap with prior verification and must not be added as business cases. The owner's broader49-test normal/race and4-test router evidence was hash-checked rather than unnecessarily repeated. The original2c independent145/44 service,49/49 handler,4/4 router and build/static results remain bound to their own earlier freeze.

The two used service profiles ran serially through root's identity-checking launcher. All checker processes have ended and all six package profiles are released. The long-lived harness was not selected or launched. No database lifecycle, external provider, model/account, browser or Desktop operation occurred.

Private source and exact commands/log hashes:

`/Users/artisan/.multica/acceptance/pay241-20261114-resume/s2-server-inc2c-followup-check/`

## Next dependency

Proceed with the existing2d writer, final-guard, M1 and activity work under the same server owner. Preserve the original changes-requested report and both original/canonical failures. Stage policy activation remains false; complete A01–A12, production Web/matching Desktop, actual long-lived router/CLI/Git journey and real S4 evidence remain separate required acceptance.
