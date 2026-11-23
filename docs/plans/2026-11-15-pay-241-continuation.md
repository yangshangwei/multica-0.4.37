# PAY-241 Continuation Implementation Plan

> Continue the approved existing Trellis tasks and retained implementation. The prior coordinator and source owner completed their current turns. TUI thread `01a1df98-6557-7f20-9fae-693f06996942` now continues their sealed r07 GREEN with one independent check owner; no duplicate implementation lane or Trellis task is created.

**Goal:** Finish the remaining S2 transaction boundaries and advance PAY-241 through real stage, recovery, integration and final performance acceptance without overstating partial evidence.

**Architecture:** Preserve the existing collaboration and M1 completion transactions. Close the original r03 service race verification gaps with separately bound evidence, accept the final source and P2 readiness contract, then run matching API/CLI/Host/production-Web/Desktop builds in task-owned environments. S3 follows accepted stable S2 references; S4 requires real nonproduction external identities; S5 formal acceptance uses the resulting candidate. The r02 harness rehearsal remains historical.

**Tech Stack:** Go, PostgreSQL, sqlc, Next.js, Electron, shared Core/Views, Vitest and Playwright.

Knowledge P6 checkpoint: private candidate `4e0f7cbe...` now passes its complete
production build and both actual Web/Desktop journeys: two PASS, zero failures,
skips or retries and stable source. Four services are stopped with cleanup zero;
final resource audit confirms no owned runtime process, six free ports and zero
browser/C3 DB connections. Final independent private AC1–AC12 is accepted with
zero blockers in [the final private review](../../.trellis/tasks/11-14-project-knowledge-automation/research/p6-independent-final-candidate-ac-review-20261115-a710db0a.json).
Current ROOT Core107/Views46 tests, two typechecks and 34 scoped knowledge lint
files pass; four containing-tree suites also pass all67 tests with zero
failures/skips/todos in [the container receipt](../../.trellis/tasks/11-14-project-knowledge-automation/research/p6-root-knowledge-container-verification-bUtC74-dd29b635.json).
Current full ROOT Go build/vet also pass with 12,173 stable inputs; the full
11,028-file/32-link intake is readonly and not a freeze. The selected 113-pair
preflight confirms zero knowledge incompatibilities.
Shared ROOT final integration remains open and follows the accepted P2 r07 source. Both minimal fixture repairs are
already in ROOT and have exact hashes and narrow patches in
[the binding receipt](../../.trellis/tasks/11-14-project-knowledge-automation/research/p6-root-narrow-fixture-binding-b7dc8f78.json).
The detail actor matches the passed private source while retaining the existing
comment-menu differences. The bounded password/managed-profile documentation
review is also PASS with zero blockers; it records the existing password-change
rejection guard in [the review receipt](../../.trellis/tasks/11-14-project-knowledge-automation/research/p6-password-managed-spec-check-52067d2e.json). See [current knowledge progress](../../.trellis/tasks/11-14-project-knowledge-automation/research/progress-2026-11-15.md).

## Baseline and scope

- Continue `.trellis/tasks/11-13-pay-241-s2-stage-approvals`; do not create another Trellis task.
- HEAD at takeover: `8e824b3035c4f17a9a9dfa0504f1294fbcc9b20b`; 860 pre-existing worktree entries must be preserved.
- S1 is archived. S2 2c/client3d/E2E4 retain bounded independent checks. The r03 2d deltas have scoped ROOT integration and handler/router normal/race checks; full service race verification and accepted final source binding remain open.
- The parent matrix has 24 cases: four PARTIAL and 20 NOT_RUN. Preserve its historical columns and full denominator.
- No new dependencies, foreign keys, broad refactor, real model account, real funds, production deployment, commit, branch or reset.
- Preserve each recorded private evidence location. The latest retained raw singleton log is under `/Volumes/artisan/.multica-acceptance/pay241-20261115-continuation/resume-r05/capacity-r03/artifact-env-probe-r01/logs/`; its actual HOME/TMP/GOTMP fixtures are under trusted Data directories. Do not infer artifact-root ancestry from the log storage path; secrets never enter repository reports.

Latest PAY owner callback: unified test-only source has 4,968 inputs, manifest
`fffeec5a7c48ad8fee7a4bc0933e0a0416d1357fc8ec56c93cc77b0e06c3bda4`,
116 actual AST tests (completion37 + foundation79) and four fresh DB configurations.
The owner and the existing two checkers verified those bindings. Complete normal
testing now passes all 116 top-level tests and 321 named records, with zero
failure/skip/missing/duplicate/unfinished results and no source/configuration drift.
Both complete race partitions also pass: completion37/128 named and
foundation79/193 named, for 116 top-level tests and 321 named records in each
mode. Race raw hashes match the sealed checker reports; completion source
byte/mode/configuration snapshots are equal and foundation reports zero drift.
The complete new independent judgment is now **CODE_ACCEPTED_R06**, with zero
current code blockers, covering W01–W18/Q01–Q12 code contracts. See
[the normal/race observation](../../.trellis/tasks/11-14-project-knowledge-automation/research/pay241-current-normal-race-116-pass-observation-ef9fe50a.json). The Data HOME/TMP/GOTMP singleton
already passed under its unchanged two-second deadline with one invocation;
that covers environment recovery. Original r03/V01/V02/full S2 remain
historical NOT_ACCEPTABLE; their root causes are not reclassified. The current
R06 code is accepted, while P2=false, products=0/4 and full S2 remains open until
P2 r07 and actual runtime acceptance. See
[the formal code callback](../../.trellis/tasks/11-14-project-knowledge-automation/research/pay241-code-accepted-r06-bound-callback-16483c51.json). See
[the running/binding callback](../../.trellis/tasks/11-14-project-knowledge-automation/research/pay241-owner-s2-running-and-final-binding-callback-718237a5.json)
and [the retained singleton receipt](../../.trellis/tasks/11-14-project-knowledge-automation/research/pay241-owner-data-singleton-callback-6daea70e.json).

The owner's current gate is P2 r07: the minimum three existing handler/test files
and 17 fresh environment profiles are assigned to the original owners. Retain
the real native PGTx schema negatives, disable/history, K06 consumed-M1/JWT
HTTP-close/restart/reopen/original-receipt/no-new-consumption, and migration862 original-key
recovery checks. Public/live P2 remains false. Final integration follows accepted
P2 code → actual readiness/enable/disable/original-key recovery → frozen matching five binaries →
production Web/Desktop with actual API/WS binding → four scenarios plus A01–A12
and cleanup → S3. ROOT hold is released. The two knowledge fixture deltas join the
final integration graph separately from the sealed fffeec server candidate and
comment-thread frontend changes.

## Task 1: Complete and independently verify S2 2d

**Owner:** the current PAY-241 TUI coordinator and native `p2_r07_check` lane. The original server owner handed off the immutable three-file r07 GREEN and is idle. Continue that exact source, retaining the original implementation ownership and evidence.

**Files:** necessary S2-only deltas in `server/internal/service/collaboration_stage*`, collaboration handoff/lifecycle/workflow and administrative completion, `delivery_completion*`, ordinary status writers, matching handlers/router, `server/pkg/db/queries`, generated sqlc files, migrations and same-layer tests. Use the existing PRD/design, frozen wire contract and [retained exhaustive entry/transaction checklist](../../.trellis/tasks/11-13-pay-241-s2-stage-approvals/research/final-acceptance-matrix-preflight-20261114.md).

1. Retain the r03 source, integration audit, complete normal/race inventories and original failed logs. Never replace peer-owned files with an old whole-file snapshot. The 17 integrated paths/19 comparisons and 4,755 protected inputs establish that bounded merge only.
2. Continue R05-V01 first-SQL and R05-V02 late-fact diagnoses on their separately registered private inputs. Verify actual secure-root HOME/TMP/GOTMPDIR ancestry before interpreting artifact-dependent tests; preserve original source, deadlines, assertions and new ENV/ENOSPC failures separately.
3. Close the missing execution witnesses: V01 must reach the result-injection seam and identify any earlier SQL error; V02 needs actual initial state/calls/clock/terminal evidence. A narrow pass or fixture ENV failure does not close either original r03 race leaf.
4. Reconcile any reviewed correction with exact candidate/submission/handoff/operation locks, final DB clock, signer/current authority, M1 off/observe/enforce and original-key recovery in the existing writers. Preserve the completed immutable submission, activity and independently checked F06 slices.
5. Integrate only reviewed owned deltas, preserving knowledge hooks/migration865, the stronger ROOT canonical test and other peer work. Reuse retained DDL evidence; any new DDL requires a fresh identity/hash check and separate concurrent index migrations, with no destructive history downgrade.
6. Bind the final source and run the required service normal/race and static checks. Retained handler/router scoped passes apply to their exact inputs; refresh affected closures after changes. Use owned DB configurations and the checked private launcher.
7. Independently reconcile W01–W18/Q01–Q12 and the complete source verdict. Only then verify actual positive/negative P2 readiness, authorized enable/disable and original-key recovery against the complete schema/writer contract. P2 currently remains false.

**Gate:** no open S2 blocking verification gap, accepted final source binding, all required writer/admission tests passing, and actual P2 readiness/activation checks supported by the complete schema and writer contract. The historical r03 verdict remains NOT_ACCEPTABLE. Current R06 code/W/Q is accepted with zero code blockers; actual P2 r07 and source/runtime acceptance remain open.

## Task 2: Run matching S2 product acceptance

**Owner:** coordinator for owned environment/builds; bounded E2E fixes go to a separate `trellis-implement` agent owning `e2e/collaboration-stage-*.spec.ts` and their stage fixtures only.

1. After the S2 source/W/Q/P2 gate, bind the final server delta, independently checked client3d/E2E4 graph, dependency links, configuration and output hashes. Preserve ROOT/private differences explicitly; a shared fixture repair is not whole-source parity.
2. Build API, matching CLI/daemon/base-Host harness, production Web and Desktop native/preload/renderer. Rebuild whenever a consumed source hash changes.
3. Check allocated ports, private database identity, password authentication, exact allowed origins, browser cache and test-only control-pipe ownership before starting listeners.
4. Run `e2e/collaboration-stage-approvals.spec.ts` first, then `e2e/collaboration-stage-execution.spec.ts`, with one worker and zero retries. Retain every original failure before a correction and rerun only after the changed source is rebound.
5. Reconcile all four scenarios with A01-A12, including independent issuer/consumer and self/R38 completion positives that the four browser scenarios alone do not establish.
6. Perform independent phase review and restore owned processes, sockets and gate values. Keep databases and raw failures as evidence.

**Gate:** four actual scenarios executed and passed, A01-A12 independently supported, no synthetic CI/controlled adapter relabelled as real S4 evidence. Only then may S2 be marked complete.

## Task 3: Complete S3 recovery acceptance

**Files:** `.trellis/tasks/11-13-pay-241-s3-recovery` planning/research first; implementation and same-layer tests are limited by its approved artifacts.

1. Reuse the independently reviewed retained v2 block/recovery planning contract while S2 finishes; the controlled timeout child is the only completed S3 acceptance slice.
2. Refresh source/lock references against accepted final S2 before dependent implementation; preserve the archived timeout proof rather than reopening it without cause.
3. Verify priority/handler races, 55-comment cross-page recovery, WebSocket reauthentication, mixed mention deduplication, revoked authority, block recovery, output-upload idempotency and late results.
4. Run the existing owning-layer regression suites plus the real Web/Desktop recovery journeys; preserve the original operation identities and every bad outcome.

**Gate:** all S3-required behaviors have independent evidence; the timeout child alone never closes S3.

## Task 4: Resolve S4 external prerequisites and run available cases

**Files:** `.trellis/tasks/11-13-pay-241-s4-external-integrations/research` readiness records and its existing intake register.

1. Reconcile I01-I10 with actual supplied nonproduction payment/fulfillment, repository/CI/build, release/rollback, metric-rule and reviewer identities; report only presence and safe identifiers, never credentials.
2. Do not substitute Multica Cloud Stripe billing, a fabricated target or controlled S2 adapter for the PAY business system.
3. Run each of E01-E07 only after its actual target, access and rollback limits are registered. Retain missing cases as NOT_RUN with the exact missing input.

**Gate:** required real external cases pass. If inputs are absent, record the concrete blocker and continue safe independent preparation; do not claim parent completion.

## Task 5: Run S5 on the final candidate and reconcile total acceptance

**Files:** existing S5 harness, `.trellis/tasks/11-13-pay-241-s5-performance-acceptance/research`, and the retained parent [test matrix](../../.trellis/tasks/11-13-pay-241-collaboration-acceptance/test-matrix.csv), [gaps](../../.trellis/tasks/11-13-pay-241-collaboration-acceptance/gaps.md) and [session progress](../../.trellis/tasks/11-13-pay-241-collaboration-acceptance/session-progress.md).

1. Freeze the accepted S2-S4 candidate and matching builds in a new registered run and database.
2. Execute 100 distinct participants, 10,000 formal propagation samples and 100 independent result/terminal writebacks. Keep warmup separate, missing samples in the denominator and actual Web/Desktop DOM measurements separate.
3. Independently recompute p95 and audit source/build/configuration identity and resource restoration. Do not reuse r02 rehearsal as TC-24 acceptance.
4. Reconcile all 24 parent cases and actual human acceptance evidence. Update only conclusions proven by the current run.

**Gate:** propagation p95 at most two seconds, result writeback p95 at most five seconds, all required cases and external evidence accounted for, and no unresolved blocking defect. Otherwise retain the parent task as open.

## Progress reporting

At every gate, record changed paths, exact frozen inputs, fresh command results, independently reviewed findings and remaining external inputs. Task status never substitutes for executed acceptance. This request authorizes continued implementation and verification, not a Git commit or production action.

## Execution checkpoint — 2026-11-15

| Step | Current state | Next verifiable gate |
| --- | --- | --- |
| 1. S2 2d | r03 service normal: 116 top/321 named PASS. Handler/router scoped normal/race PASS. Original full service race: 318 PASS/3 named FAIL, representing two independent leaves and one parent; its verdict remains NOT_ACCEPTABLE. New sealed fffeec normal: 116 top/321 named PASS, zero fail/skip/missing/duplicate. New complete race also PASS: 116 top/321 named, zero fail/skip/missing/extra/duplicate/unfinished. Formal CODE_ACCEPTED_R06 accepted with zero current code blockers; P2 r07 minimum3 existing-file repair/17 fresh profiles preparing, actual P2 false and accepted final source binding pending. | Code/W/Q accepted; independently accept P2 r07 against native schema negatives, disable/history/862 original-key recovery, then bind the final source and actual P2 readiness/enable/disable. |
| 2. S2 product acceptance | Four actual scenarios remain 0/4 and A01–A12 open. Prior r03 build/auth/frontend proof is preflight only. Knowledge P6's private fixture checks/build/two journeys pass; shared ROOT and the S2 graph still need their own final binding. | Build the accepted S2 candidate with matching API/CLI/Host/Web/Desktop and actual health symbols, then execute four scenarios and A01–A12 without skips. |
| 3. S3 recovery | Retained v2 planning has independent review; only the controlled timeout child is completed. Recovery acceptance awaits accepted final S2 source/lock references. | Resume priority/handler/CAS and real recovery journeys after stable accepted S2; keep timeout history distinct. |
| 4. S4 integrations | E01–E07 NOT_RUN; real PAY sandbox/CI/release/metric identities remain unavailable. | Register actual nonproduction targets and access limits; do not substitute local controlled adapters or Cloud billing. |
| 5. S5 final acceptance | Gated; r02 remains only a harness rehearsal. Parent matrix remains 24 cases: four PARTIAL, 20 NOT_RUN. | Freeze accepted S2–S4, run formal performance samples and independently reconcile all 24 cases. |

The current capacity followup's full37 run has 40 PASS/88 FAIL among 128 named terminal records. It fails in the secure-root artifact fixture environment, before the target injection, and supplies no first-SQL witness for the original r03. The separate late-fact followup also fails before Prepare. These ENV failures cannot explain or replace the original result-corruption/late-conflict records. The owner is correcting actual trusted Data0700 HOME/TMP paths; its latest read-only message identifies Go1.27 `t.TempDir()` consuming `GOTMPDIR`, so moving only TMPDIR leaves the artifact root on the external volume. The corrected Data0700 HOME/TMP/GOTMP singleton subsequently passed, with its raw hash verified; the unified complete normal/race code verdict remains pending. No guard or shared mount is changed; ENOSPC/NOT_RECORDED and original r03 failures remain retained.

Current S2 evidence is in [the complete r05 code review](../../.trellis/tasks/11-13-pay-241-s2-stage-approvals/research/server-increment-2d-final-code-check-r05.md), [the scoped handler/router review](../../.trellis/tasks/11-13-pay-241-s2-stage-approvals/research/server-increment-2d-handler-router-check-r05.md), [the source coordination audit](../../.trellis/tasks/11-13-pay-241-s2-stage-approvals/research/coordination-freeze-r05.md) and [the separately bound diagnostic continuation](../../.trellis/tasks/11-13-pay-241-s2-stage-approvals/research/server-increment-2d-resume-r05-results.md). The DDL conclusion remains six checked old raw controls plus one separately executed immutable supplement; the old seventh gap and failed allocator probe are preserved. F06 is now independently checked in the matching r03 handler/router scope. Earlier first-increment preflight remains in [its original checkpoint](../../.trellis/tasks/11-13-pay-241-s2-stage-approvals/research/continuation-validation-preflight-20261115.md).

The dependency order is S2 transactions/W01–W18/Q01–Q12 → actual P2 readiness → four product scenarios/A01–A12 → S3 recovery → real nonproduction S4 E01–E07 → formal S5 candidate performance/parent24. Safe independent diagnosis and intake preparation may continue in parallel with knowledge P6, subject to the current owned resource/source window. Knowledge P6 remains open for final private AC and shared ROOT integration: the
current 4e production build/start and both complete journeys pass, with zero
failure/skip/retry and stable source. Owned services are stopped/cleanup0 and the
final resource audit passes. ROOT comment-menu/Views differences are retained;
the two shared fixture deltas have exact narrow binding receipts. See
[the knowledge checkpoint](../../.trellis/tasks/11-14-project-knowledge-automation/research/progress-2026-11-15.md).
Neither private knowledge success nor r02 rehearsal closes PAY-241.
