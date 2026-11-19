# S2 increment 2c — independent implementation check

- Reviewer: `/root/s2_check_resume`, 2026-11-14.
- Verdict: **CHANGES REQUESTED** for two reproduced P2 findings. Existing normal/race/static checks pass, but do not cover these counterexamples. This report is the intermediate correction handoff; it does not accept2c, enable stage policy, or accept whole S2.
- Source: the independently checked4,894-file ACL base plus63 explicitly owned S2 overlay files, reconstructed as4,938 files. Every original source hash remained unchanged during this review. One additional diagnostic test exists only in the reviewer's private copy and has its own manifest.
- Ownership: no live product, test, database lifecycle, configuration, migration or Git changes. The sole server implementation owner should make the corrections and supply a new bounded freeze. All six granted package profiles are released.

## Findings (fixed)

None by this reviewer. The implementation's original-issuer artifact authority fix was verified: actual Prepare refuses a demoted original code reviewer with `stage_signer_changed` and zero adapter calls. Its original red and subsequent normal/race evidence remain preserved.

## Findings (not fixed)

### S2C-C01 — Record failure renews the five-second finalization budget (P2)

**Source:** `server/internal/service/collaboration_stage_execution.go`, `enterStageExecution`.

After adapter entry, the method creates a five-second background context for `RecordStageExecutionObservation`. If that call fails, `finish("unknown")` creates a new five-second context for `recordStageInternalObservation`. The fallback therefore receives a later deadline rather than the remaining original budget. If recording consumes nearly its deadline, total finalization may approach ten seconds.

**Executed counterexample:** `TestStageExecutionCheckFinalizationKeepsOneDeadline` uses the existing real execution fixture, returns an observation-authentication error after the one controlled callback, and captures deadlines at the actual Record and fallback transaction starts. The fallback deadline was10.203459ms later. The test intentionally does not sleep five seconds: the two different absolute deadlines establish that the budget restarted. The receipt remained unknown with no fact and exactly one callback; no false execution success or retry was observed.

**Correction:** create one five-second finalization context for the complete post-entry recording/fallback sequence and reuse its remaining budget. If it expires, preserve the unresolved history and error; do not extend the deadline, manufacture persistence or repeat the adapter invocation. Keep the separate pre-entry zero-call path and its bounded cleanup.

### S2C-C02 — R4 marks a release approval valid despite its original reviewer losing artifact authority (P2)

**Source:** `server/internal/service/collaboration_stage_execution_view.go`, `stageExecutionReviewState`, and `collaboration_stage_execution_approvals.go`, `stageExecutionApprovalChain`.

The ordinary read evaluator checks the current decision's own M1 artifact authority. The release/rollback-specific evaluator then calls `stageExecutionApprovalChain` and can convert `evidence_changed` to `valid`, but that recursive chain does not apply the original M1 artifact read gate to every upstream issuer. Final execution consumption correctly does so through `lockStageFinalSigners`; the read projection can disagree with it.

**Executed counterexample:** `TestStageExecutionCheckR4ReflectsOriginalArtifactReaderLoss` creates the existing valid code-review→QA→release chain, demotes the original reviewer from artifact manager to ordinary source member, and reads R4 as the still-authorized release reader. R4 returns:

| Decision | Observed current state |
| --- | --- |
| Original code review | `signer_changed` |
| QA depending on it | `evidence_changed` |
| Release depending on that QA | **`valid`** |

The caller's baseline remains readable, establishing that this is the upstream signer's loss of authority. In this scenario the release signable hint was already blocked by QA, and actual Prepare returned `stage_signer_changed` with zero calls. **This is a current approval-state defect, not a new execution authorization bypass.** No claim is made here about an unexecuted rollback UI variant.

**Correction:** make the release/rollback read evaluator apply the existing current issuer/M1 checks to the full returned dependency chain before reporting `valid`. Reuse the existing authority boundary and maintain the anonymous current-reader source-denied projection. Preserve the real execution refusal and original receipt behavior.

## Source and evidence integrity

- Verified the final result binding, all63 overlay/base hashes, all4,938 final source files,30 raw log/direct-exit pairs and their capture metadata. Parsed named inventories match. Final isolated verification records have `source_unchanged:true`.
- The earlier mixed-workspace service Go exit0 with `source_unchanged:false` remains an identity failure and is not final S2 proof. Its raw files remain untouched.
- Independently confirmed the same three current workspace differences: `workspace_delete_manifest_test.go`, generated `models.go`, and `sqlc.yaml`. The private S2 reconstruction contains no844–859 migration or knowledge-automation implementation. No live shared file was copied to fill dependencies.
- The root migration report and its seven referenced artifacts match their recorded hashes. Source inspection confirms13 single-statement concurrent-index migrations, corresponding retry registrations, readiness shape checks and retained-history guards at843 and829. The checker ran no migrations. Root's actual migration runs, stopped-admission precondition, synthetic retained intent and eight existing F2 failures remain separately identified.
- Added diagnostic source is `collaboration_stage_execution_check_diagnostics_test.go` in the private copy only; `diagnostic-source.json` records its hash and actual argv. The4,938 original product/test hashes still match after diagnostics.

## Verification

All database suites ran serially through root's identity-checking launcher and exact package profiles, on the independent S2-only copy. The long-lived harness flag was absent and its test was never selected. Named counts overlap and are not additive acceptance cases.

| Independent command scope | Result |
| --- | --- |
| Service `^TestStage` |145 PASS,0 FAIL,0 SKIP |
| Service execution/ACL/QA/expiry `-race` |44 PASS,0 FAIL,0 SKIP |
| Handler stage normal / race |49 /49 PASS,0 FAIL,0 SKIP |
| Bounded router normal / race |4 /4 PASS,0 FAIL,0 SKIP |
| Affected service/handler/router `go vet` |PASS |
| Full backend `go build ./...` |PASS |
| Frozen overlay `gofmt -l` |PASS, empty output |
| Additional private diagnostics |**0 PASS,2 FAIL,0 SKIP**; both findings above reproduced |

Reviewed the actual production admission/observation chain, its private atomic entry guard and definite-commit ordering, physical-target conflict domain, conditional before/after bindings, original human versus integration identity, terminal/native identity checks, native metric ambiguity and stream lock, A8 intent consumption, A9 selected-fact derivation, and R4 source projections. No additional concrete finding was identified in that bounded review.

The real router tests check actual auth/handler adapters and artifact create→human approval→physical package routes, including tar bytes/digest/EOF. Their controlled source configuration and retained fixtures are not actual deployment providers. The long-lived JSONL harness, full public A7/internal producer/UI journey and production Web/Desktop were inspected or compiled where reported by the owner, not launched by this reviewer. The private control code is test-only; normal production construction has no adapter and remains unavailable.

## Handoff and limits

Private evidence is under:

`/Users/artisan/.multica/acceptance/pay241-20261114-resume/s2-server-inc2c-check/`

All six package processes have ended; no profile remains occupied by this checker. The same server owner should address S2C-C01/C02, retain this first failure report, and return a source-bound correction for independent recheck before2d. The original issuer red→green, all successful baseline checks and these new failures remain separate records.

Stage activation stays false. All2d writer/final-guard/activity/M1 work, the complete A01–A12 matrix, actual long-lived harness and Web/Desktop/Git/CLI journey, real S4 integrations and parent acceptance remain open. No real model/account, payment or external deployment operation was performed.
