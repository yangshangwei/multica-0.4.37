# Research: remaining PAY-241 source stimuli after TC-06 and TC-12

- Query: Is another exact, useful internal stimulus still missing from TC-06/07/09–12/17–20 or TC-24 after the accepted supplements?
- Scope: internal requirements, accepted evidence, current production entrypoints and existing tests; no execution or external account discovery.
- Date: 2026-11-20
- Disposition: **NO_NEW_INTERNAL_STIMULUS_IDENTIFIED_IN_REVIEWED_SOURCE_CASES**. Resume S4 E01 when its bounded repository/CI inputs are supplied. Do not add another synthetic test merely to revisit an accepted contract.
- Parent: 24 cases remain **9 PASS /11 PARTIAL /4 NOT_RUN**. This research changes no result, code, specification or task state.

## Findings

The current CSV and latest TC-06/TC-12 checks supersede the old R33 missing-concurrency and same-code-retry qualifications. TC-06 now establishes the actual human-edit/runtime-reply contention, retained current human text, frozen original input and report linkage. TC-12 establishes an actual local synthetic business-code A→B repair under a fixed runner and input. Their stated remaining clauses concern real external sources. Another local reply/edit race, another A→B program, another gate matrix, or another mock provider would not reduce those clauses.

The archived S2 R33 check accepts the agreed code, evidence, authority and original-request contracts; S3 accepts R01–R09 recovery and provenance. Their historical parent counts and superseded pending items are not current task state. This research uses their accepted scope and the current parent records, without rerunning or reaccepting their historic source as a new HEAD pass.

### Exact residual clauses and minimum inputs

| Parent cases | Accepted internal coverage retained | Actual remaining stimulus | Minimum input before a meaningful next execution |
| --- | --- | --- | --- |
| TC-06 / TC-07 | Same-root candidate/runtime/report association; human text preservation; strict missing/failed/stale/head/trust CI gates | A real allowed branch/PR/head and a real complete CI check set ingested through the selected connector, including stale/replayed evidence | S4 **I02** provider/instance, repository ID/address, allowed test branch or existing PR, installation/connection reference; **I03** workflow/required checks and run/job/attempt lookup, build digest/retention; E01's relevant **I09** implementer/reviewer/QA identities |
| TC-09 | Platform report and source binding, not payment correctness | Sandbox payment produces the paid order and independently observable payment/order/fulfillment identities | **I04/I05** sandbox/API version, merchant/tenant, amount/currency/test payment instrument, order and payment ledgers, fulfillment queries and private credential references |
| TC-10 / TC-11 | Platform idempotency and controlled recovery, not payment idempotency or real callbacks | Same event delivered three times; real 45-second delay, timeout→success, out-of-order delivery and consumer failure/recovery, with independent domain ledgers | **I04–I06**, including callback address/event and delivery IDs, replay/signature configuration references, allowed delay/failure point and consumer recovery control |
| TC-12 | Actual local synthetic business-only A→B and fixed regression; production report/stage association and old failure/QA history | Repair and regression against the actual payment repository and its real execution source | **I02/I03** plus **I04–I06** where the regression crosses payment/order/callback domains; identified failing revision, fixed revision and unchanged relevant regression input; relevant human QA |
| TC-17 | Actual QA release-approval refusal and retained authorization evidence | QA denial at the selected actual nonproduction publishing boundary, then authorized exact-target execution | **I07** executor contract, nonproduction environment/target, current/next artifact digests, original-operation lookup and health query; relevant **I09** QA/release identities; repository/build identity from I02/I03 |
| TC-18 | Controlled metric, exact approval, one local rollback and original target receipt | Real target metric observation leads to a human-approved rollback of that actual nonproduction target | **I07/I08** previous artifact, real metric query/labels/window/threshold/rule version, permitted failure/recovery scope and authoritative rollback result query; designated release approver |
| TC-19 / TC-20 | Existing all-writer completion gates and accepted internal actor/request/run/source audit | Exact real repository/CI/payment/release/rollback/verification facts and corresponding current human decisions are joined to delivery | E01–E06 real outputs plus **I09** actual verifier/final acceptor; do not substitute synthesized report text or test-account signatures |
| TC-24 | Sampling tools and r02 rehearsal accepted at their original pre-S2 source | Complete final-candidate 100-observer/100-event and 100-run sampling | S2–S4 required feature changes stable, an identified final candidate, matching API/production Web/Desktop and controlled binaries, new owned database/run manifest, unchanged finite sample protocol |

These are conditional requirements for each named slice, not a demand to supply all I02–I09 before starting any S4 work. **E01 is the smallest next source slice:** I02/I03 and the E01 subset of I09 can unlock it without waiting for payment or deployment inputs. Provider selection still matters: the current strict CI reader supports the GitHub check-snapshot contract; repository-less generic VCS status explicitly remains unsupported. Inspect the selected real provider's contract before determining whether an adapter change is required.

### Production source owners and existing proofs

| File / location | What it establishes, and why another local test does not settle the residual clause |
| --- | --- |
| `server/cmd/server/router.go:1519` | `/api/webhooks/github` is the public signed GitHub ingress. The VCS callback is separately registered at line1531. The existence of these routes is not evidence that the PAY repository is connected. |
| `server/internal/handler/github.go:1459`, `:1491` | CI webhook bodies identify PR refresh work. Their contents do not become trusted displayed check results; `triggerPRRefreshFromCIEvent` queues an API refresh. Signing a new synthetic callback would still be synthetic evidence. |
| `server/internal/integrations/ghsnapshot/client.go:1`, `:84` | GitHub App authentication and installation-token API reads are the existing source owner. A configured client and scoped installation are real prerequisites; do not infer them from local login availability. |
| `server/internal/integrations/ghsnapshot/refresh.go:121`, `:203` | Existing queued refresh uses `FetchPRSnapshot`, then applies a head-guarded snapshot. Local queue/fetch tests already cover the internal mechanism. |
| `server/internal/integrations/ghsnapshot/snapshot_test.go:46`, `:70`, `:124` | Existing tests reject incomplete check sets, paginate contexts and reject head changes during pagination. Their fake GitHub transport does not establish remote CI identity. |
| `server/internal/integrations/ghsnapshot/refresh_db_test.go:215`, `:290`, `:380` | Existing tests cover head-SHA guards, an old in-flight head with trailing refresh and replacement of the check-run set. |
| `server/internal/handler/github_stage_evidence_test.go:37` | Existing synthetic signed callback test exercises the real handler, older/equal-time ambiguous head and link derivation race. Repeating it would not fill E01. |
| `server/internal/service/collaboration_stage_evidence.go:71`, `:98` | Current GitHub CI must have a complete nonempty check set at the exact head, current observation time, all checks completed/success and stable evidence digest. Generic VCS returns `unsupported`. |
| `server/internal/service/collaboration_stage_review.go:66`, `:201`; `collaboration_stage_review_test.go:158`, `:185`, `:459` | The existing candidate/decision owners and tests cover strict CI refusals, source drift and source/trust fences. These are already within accepted S2. |
| `server/cmd/server/router.go:1420`, `:1696`, `:1724` | Existing runtime subject/report/ACK, human report reads, candidate/evidence and stage-decision routes already join the execution and human-review chain; no new parallel workflow is warranted. |
| `server/internal/service/collaboration_test_report.go:141`, `:162`, `:247` | Subject/report service owns run-bound validation and stores `source_kind=runtime_reported`. It does not certify that the producer executed the real payment code. |
| Archived S2 `research/test-report-contract.md:62` | Explicitly states that an mcb capability proves authorized prepared-source association, not filesystem/test-execution attestation, and cannot be called `remote_ci_verified`. |
| `server/internal/service/collaboration_concurrent_content_test.go:26`; `collaboration_code_revision_regression_test.go:27` | Current tests explicitly state synthetic PR/CI and managed preparation/consent/settlement boundaries. Production Switch or daemon checkout is not claimed. That limitation alone does not introduce a new PAY requirement beyond the exact remaining real-source clause. |
| `server/internal/service/collaboration_stage_execution_types.go:11`, `:24` | `StageExecutionSources` is injected by trusted server construction; proof bytes have no public write endpoint. A real executor/metric source must be selected and bound before real publishing/rollback acceptance. |
| `server/internal/service/collaboration_stage_execution.go:204`; `collaboration_stage_execution_test.go:50`, `:171`, `:265`, `:308` | Existing methods/tests cover release/rollback/original receipt, metric ambiguity, late authenticated history and unavailable production source. Repeating a controlled source is not an external adapter validation. |
| `server/cmd/server/collaboration_stage_execution_control_boundary_test.go:99` | Existing `TestStageExecutionRouterProductionConstructorHasNoSource` explicitly checks that ordinary production construction does not install the simulated source. The test controller is a test-only entry, not a general publishing API. |

The ordinary-comment machine/admin authority task is unrelated to this research's candidate/source path and remains under its separate ownership. No authority fix is proposed here.

### TC-24 is feasible as a new rehearsal, but that is not the outstanding final acceptance

S5 `prd.md` P01–P05 fixes 100 distinct password users/sockets, 100 formal writes and 10,000 correlated observer samples, plus 100 independent runs for each writeback measure. Missing/late/unrun samples remain in the denominator. P06/P07 concern full parent/business acceptance, not an extra performance percentile.

S5 `design.md` §8 and `implement.md` §3 explicitly require S2–S4 needed changes to stabilize before final sampling. S2/S3 are now accepted; unknown selected S4 adapters and external contract requirements leave the final candidate unbound. The rehearsal tooling has already been implemented, independently checked and run completely in r02. Repeating it now would produce another current-source rehearsal, **not TC-24 PASS**, and no specific tooling defect or unproven accounting rule was found in this bounded review to justify another rehearsal as the next minimum increment.

When final-candidate readiness is established, use the existing three phases and unchanged deadlines, pool/load/sockets and raw denominator. Bind final product and fixture source/configuration/binaries first; retain the full new measurement and any failed runs. Do not inherit r02's timing, infer current throughput from static source equivalence, or turn a few real pages into 100 rendered pages.

## Files found / related specifications

The production and test files above are the bounded source map. The adjacent JSON fingerprints them with the authoritative parent CSV, latest TC-06/TC-12 checks, archived S2/S3 accepted reports, S4 PRD/design/input sheet and S5 PRD/design/implementation/rehearsal review.

Related specifications: `.trellis/spec/server/collaboration-stage-approvals.md` (current source/signers/evidence), `.trellis/spec/server/collaboration-source-editing.md` and `collaboration-runtime-output.md` (accepted content ownership), `.trellis/spec/web/frontend/collaboration-performance-qa.md` (finite sampling and rehearsal/final-candidate separation), `.trellis/spec/server/database-test-isolation.md` and `verification-environment.md` (future execution must be newly owned and source-bound).

External references: none fetched. No new SDK, dependency or unknown provider contract was selected. Once the provider is supplied, its actual documentation/version is an execution prerequisite rather than something guessed here.

## Caveats / Not Found

- This is a bounded static research result for the named case family, not a proof that the whole repository has no possible bugs or useful tests. TC-01/03 semantics and TC-04/14 real-model questions are separately researched.
- No source or test was executed. Existing test names describe inspected coverage; their accepted historical runs retain their original source and scope. No new HEAD compile, normal/race, UI or performance pass is claimed.
- The shared branch moved during research. Reading `.git/HEAD` and its referenced file first observed `6d4c98561860b11070d5c816a8ce01b582445f45`, then `624eb71b96de1f1945d269c47378d6a31005ccd9` at fingerprint capture. The JSON binds actual file bytes, not an assumption that every shared file equals that commit. Main must independently reconcile current HEAD and any outgoing peer commits before Git operations.
- No DB/API/network/model/payment/deployment/process-control or Git command ran. No credential store was inspected. Only this research pair was written; no child agents were spawned.
- The minimum next action is to record a supplied real E01 repository/CI binding and E01 human roles, then reassess that selected connector against existing contracts. Until that input or a confirmed narrower requirement changes, preserve the residual cases and avoid another synthetic acceptance loop.
