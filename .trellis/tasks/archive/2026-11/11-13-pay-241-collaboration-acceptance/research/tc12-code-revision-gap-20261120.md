# Research: TC-12 exact code revision regression gap

- Query: Determine the missing stimulus beyond the accepted S2 failure/QA/retry evidence and the smallest executable internal supplement.
- Scope: internal; read-only source/contract research, followed only by these task-local research files. No product changes, database connections, TestMain, model, payment, deployment, or Git operations were performed by the researcher.
- Date: 2026-11-20
- Active task: `.trellis/tasks/11-13-pay-241-collaboration-acceptance`
- Disposition: **FEASIBLE_SERVICE_ONLY_SYNTHETIC_CODE_REVISION_SUPPLEMENT**. Parent TC-12 remains PARTIAL even if this supplement passes. Implementation and execution are separate work, not results of this research.

## Findings

### Authoritative gap

The parent `test-matrix.csv:13` requires the failed test to retain its run, failing case and log, followed by authorized QA return and retest after modification. Its current evidence explicitly states that the two accepted runs used the same code. The current parent matrix remains 9 PASS / 11 PARTIAL / 4 NOT_RUN; `session-progress.md:3` records TC-04 as the latest accepted increment. Older research ordering is historical.

The S2 R33 report, `research/r33-full-s2-a01-a12-independent-check-20261118.md:128` under the archived S2 task, deliberately retains TC-12 PARTIAL: “same-code fault change is not business-code repair/retest.” Its A05 acceptance is valid for run/case/log association, QA return, distinct retry and retained decisions. No replay of its complete four-journey matrix is needed to discover the missing code change.

The canonical test demonstrates the precise limitation. `e2e/collaboration-stage-approvals.spec.ts:436` accepts a `fault` argument while retaining `gitInput.head`. At lines 464, 472 and 503 both report and preparation reference that same commit. Lines 511 and 550 execute the failure and success variants; lines 535–577 retain QA return, re-review, a distinct report and new human approval. Lines 485–491 explicitly retain the same prepared workspace and configuration. These are genuine platform/runtime observations, but changing `duplicate_effect` to `none` is not a code repair.

### Minimal bounded increment

Add only `server/internal/service/collaboration_code_revision_regression_test.go` for the implementation slice. One top-level integration test runs the whole ordered chain; normal and race each execute that same name once. This is **one unique test / two mode executions**, not a business-case count. The original failing local program and later passing local program are expected stimuli inside that test; they are not a failing Go verification round.

Each mode creates one task-owned local Git repository and two commits:

1. Commit A contains a deliberately defective small synthetic business function, one fixed regression runner, and one fixed input file. Run that exact runner/argv against A and require the expected nonzero exit and exact diagnostic, rather than inventing a failed report from a string switch.
2. Preserve head, tree, tracked file hashes, complete output, exit code and Git bundle bytes/digest. The runner/input hashes and command are fixed before either run.
3. After the platform has stored A's failed report and authorized QA has returned it to development, modify only the business implementation, create commit B with A as its direct parent, and record the actual patch. Require the runner and input blobs to be unchanged and the business blob/tree/head to change.
4. Execute the same command in the B checkout, require exit zero and the expected result, and preserve its raw evidence. No environment-based fault flag, substituted test oracle, changed timeout, or changed assertion can explain success.
5. Bind each result to its exact platform candidate, typed command, frozen test subject, run, attempt, report and current human decision.

The local implementation can use an existing installed interpreter, with a pinned executable hash/version and a small no-dependency runner. All child processes need an explicit minimal environment, a private home/config, no inherited DB or capability credentials, a bounded context, and recorded termination. The parent coordinates the final choice and private evidence root. Do not execute untrusted arbitrary repository hooks or global Git helpers; use task-local configuration and deterministic synthetic identity.

### Existing production owners and test reuse

| File / symbol | Role in the increment |
| --- | --- |
| `server/internal/service/collaboration_test_report_test.go:27`, `newStageRuntimeFixture` | Existing managed service fixture, roles, synthetic PR/CI, candidate and review. |
| `server/internal/service/collaboration_test_report_test.go:84`, `input` | Typed existing comment includes exact candidate reference and actual Agent target. |
| `server/internal/service/collaboration_test_report_boundaries_test.go:20`, `startStageReportRuntime` | Existing comment → resume → list → claim → permit sequence; expand locally to retain command and original comment request identities. |
| `server/internal/service/collaboration_runtime_preparation_capabilities_test.go:19`, `preparationRegistrationRequest` | Register the real local commit and bundle digest through the existing capability owner. |
| Same file, line 46, `refreshPreparationConsent` | Explicit reconfigure and renewed owner consent after changed preparation identity. |
| `server/internal/service/collaboration_runtime_preparation_test.go:21`, `claimPreparedCandidate` | Real service claim produces and validates frozen preparation/inputs. |
| Same file, line 35, `permitPreparedClaim` | Real service permit mints the run-local business credential. |
| `server/internal/service/collaboration_test_subject.go:189`, `freezeRuntimeTestSubject` | Freezes command/candidate/preparation; rejects missing Git and commit mismatch at lines 225–229. |
| `server/internal/service/collaboration_test_report.go:141`, `RuntimeTestSubject` | Reads the currently authorized frozen subject for the run-local credential. |
| Same file, line 162, `RecordRuntimeTestReport` | Validates exact identity/subject, derives source identity, and appends the one immutable report event per attempt. |
| Same file, line 123, `RuntimeTestReportReceipt` | Original-key minimal recovery after settlement/configuration change. |
| `server/internal/service/collaboration_squad_test.go:80`, `finishSquadRun` | Existing service outcome/proof settlement; **synthetic managed proof**, not an actual daemon process or namespace test. |
| `server/internal/service/collaboration_stage_review_test.go:51`, `code` | Creates a new candidate with exact `Supersedes`; retains its predecessor and increments version. |
| Same file, `advanceParams`, `reviewParams`, `evidence` | Reuses current observations and real role-bound transitions/decisions. |
| `server/internal/service/collaboration_stage_review.go:66`, `CreateStageCodeCandidate` | Checks current prior candidate and PR head, appends immutable version with parent ID. |
| Same file, line 201 / 405 | Real `DecideStage` / `AdvanceStage` owners; no parallel workflow. |
| `server/internal/service/collaboration_test_report_view.go:188` | Applicability is current-state projection; old A becomes stale after B while report bytes remain immutable. |

The actual transport entrypoints already exist at `server/cmd/server/router.go:1420`–1422 for subject, report and original receipt, and lines 1696–1697 for current-authority human report reads. `server/internal/handler/collaboration_test_report.go` parses the existing run-local capability and exact wire shape. This supplement calls the same service owners directly; it does **not** newly prove those HTTP adapters, production router, CLI, browser, password login or daemon checkout.

### Exact A → QA return → B sequence

Use `newStageRuntimeFixture(t, false, setup)`. In its setup callback, set `s.head` and the synthetic PR/check snapshot head to the real A commit before the helper's `s.approved` call. Do not pass `git=true`: the fixture's local `head` is fixed to 40 `b` characters and is independently used for its automatic Git registration after the setup callback. Register the real Git capabilities explicitly afterwards.

For every explicit registration: build `preparationRegistrationRequest`, assign Git `{ObjectFormat:"sha1", CommitOID:<real head>, BundleDigest:<actual bundle SHA256>}`, call `RegisterRuntimeCapabilities`, reload `f.runtime` from `GetAgentRuntime`, and call `refreshPreparationConsent`. The reload is required so the next registration uses the current `ExpectedRuntimeConfigEpoch`.

A chain:

- Create typed comment with `f.input`, retain its request and returned command ID; resume/list/claim/permit.
- Require claimed `Preparation.Git` and `RuntimeTestSubject.Subject.SourceRevision` to equal actual A. Bind preparation command IDs to the returned comment command.
- Execute local A and submit `RecordRuntimeTestReport` using the actual permit token. Derive the case outcome from the observed exit and use the actual output in inline logs. Retain source/tool/runner/input/output hashes alongside the report linkage.
- Settle failure via the existing outcome/proof owner. Verify automatic candidate report listing and exact failed case/log, with applicability `execution_failed`.
- Call human QA `DecideStage` with decision `return`, the exact A candidate and report ID, and a fresh original request. Retain the returned decision and `GetWorkflowOperation` receipt. Then call `AdvanceStage(...,"development",returnDecisionID)`.

B chain:

- Commit only the business code repair and independently prove parent/patch/blob differences. Update the **synthetic fixture** PR and CI snapshots to B; this is explicitly not a remote CI observation.
- `next := f.stage.code(t, &oldCandidate)`; assert ID/digest differs, version increments, and immutable parent ID points to A.
- Advance to `code_review`, record a fresh human review with `f.stage.reviewParams(next,"approve")`, then advance to `qa` using that new review ID. Set `f.candidate=next`.
- Register B preparation and refresh explicit consent. A failed execution may pause the target: read current target and explicitly use target-scoped `ResumeQueue` if required. `f.resumeManaged` only resumes issue scope. The exact target form exists at `collaboration_runtime_test.go:234`; canonical E2E line 549 does the same.
- New typed comment, new run, new attempt identity, new report request, actual B execution, report and successful settlement. Same test suite/case/input/runner/argv; different source/candidate/binding/run/report identities.
- Verify B is usable; human QA approves exact B report with the fresh B code review dependency. It is enough to stop at this approval; release/deployment and the parent close are outside the increment.

### Immutability and recovery denominator

Capture A after its failure is settled and the QA return/transition is committed. At B completion compare the complete retained rows for A's run, attempts, run events (including test subject/report/outcome/proof), command and bound subject, code candidate, original code-review/QA decisions and original operation receipts. Capture the row sets as canonical full-row JSON in private evidence, not merely counts. Preserve original bytes of A's Git bundle, files and execution output.

Use exact original report request/body/token with `RecordRuntimeTestReport` and `RuntimeTestReportReceipt`; require the original minimal ACK and one `test_report` row. Recover A's QA decision through original `GetWorkflowOperation`, and if explicitly tested, replay the exact original `DecideStage` body/key. Compare immutable rows before and after these recovery calls to prove no duplicate or rewritten business history.

A's human report detail must still contain the same `report_id`, digest, source subject, timestamps, cases and log bodies. Its current applicability changes to `stale` after B under `testReportApplicability`; that expected projection change is not historical mutation. Do not demand equality of dynamic `CheckedAt`, signability horizons or current candidate projections. Verify the B QA decision references B's report and fresh review, never the old failure or old signer receipt.

Use one minimal negative control relevant to version correspondence if the implementation needs it: a B approval referencing A's old report must fail under the existing gate with no business mutation. Do not expand this into the already accepted full S2/TC-13 denial matrix.

Existing fixture `Cleanup` registrations remove transient task rows after the test. Record exact pre-cleanup full histories and distinguish them from what remains in the dedicated database after process exit; never claim all rows persist in SQL merely because snapshots were retained. Parent coordination decides whether this slice needs a separately retained fixture lifecycle. No cleanup may touch another session's resources or the old acceptance databases.

### Verification and expected accounting

The main session must freeze the candidate/test source and allocate a fresh service-only database per normal/race execution. Research does not grant database execution permission. Safe compile-only command shape, from the frozen `server/` root: `go test -c -o <private-test-binary> ./internal/service`. It does not execute TestMain. Formatting and `go vet ./internal/service` are separate static checks.

After the main session binds a private configuration/permit to that exact source and test, suggested execution shape is `go test -count=1 -json -timeout <unchanged-approved-bound> -run '^TestStageCodeRevisionRegressionRetainsFailedHistory$' ./internal/service`, with `-race` in the independently provisioned race invocation. The final exact name/timeout belongs in the preregistered execution plan; this example is not a run result. Require the expected named PASS terminal, zero FAIL/SKIP/unfinished, process exit and normal/race name equality. An empty filter is never compile-only.

Retain each actual failing verification round under its own source/config/database identity. If a fixture assumption fails, keep the failed round, change only the proven fixture fault, and run the whole one-test chain in new databases. Product changes are authorized only if actual failure establishes a product defect and the parent explicitly narrows that repair.

## Related specs and references

- `.trellis/workflow.md`: continue existing task; implement/check role separation and task-local evidence.
- `.trellis/spec/server/collaboration-stage-approvals.md`: current human/source authority, historical failed-report reads and original decision recovery.
- `.trellis/spec/server/collaboration-records.md`: immutable run/attempt identity and source-owned records.
- `.trellis/spec/server/database-test-isolation.md`: package/mode owned DB, explicit environment, retained failures and no TestMain compile shortcut.
- `.trellis/spec/server/verification-environment.md`: exact source/config/tool/command permit and complete test-event inventory.
- Archived S2 `research/test-report-contract.md`: `runtime_reported` and source-binding semantics; a business capability is not execution attestation.
- Archived S2 `report.md` and R33 independent check: accepted historical run/QA evidence and explicit same-code limitation.
- Parent `research/remaining-internal-acceptance-20261119.md:97`: a specifically scoped local fixture code revision is available as an internal supplement.
- S4 `research/next-inputs-20261119.md:7`: I02–I09 remain absent for real repository/CI, payment/fulfillment, callbacks, nonproduction deployment/rollback, metrics and real reviewers.
- External documentation: none needed or fetched. The slice uses inspected repository contracts, installed tools and no new SDK/dependency.

## Caveats / Not Found

This increment proves real local **synthetic** code change plus service-layer source/report/QA correspondence. The candidate's GitHub/CI facts, managed runtime identity, permit/proof and human actors remain explicit fixtures. It does not prove real payment business code, remote repository/CI, actual daemon checkout, namespace containment, new HTTP/browser/Desktop interaction, real model quality, deployment, performance or business signoff.

No available contract turns a local synthetic repair into real PAY-241 payment acceptance. TC-12 remains PARTIAL, the 24-row parent denominator remains unchanged, and S4 external input requirements remain open. S1/S2/S3 and TC-02/13 accepted evidence stays accepted; no task recreation or historical rerun is justified by this research.

## Implementation findings retained during execution

The first normal round failed before local A execution because target Resume
preceded issue Resume. The second reached A failure/QA return but stopped while
refreshing B preparation: the still-active target requires explicit Stop before
reconfiguration. Both complete failed rounds remain separate from a later pass;
no business assertion or timeout was relaxed.

The reused `refreshPreparationConsent` calls `ReconfigureTarget` from
`collaboration_pending_configuration_fixture_test.go`. That retired pre-consent
mutation is a synthetic test fixture, with no production handler. This supplement
therefore covers production report/stage owners with fixture managed
preparation/consent/settlement; it does not newly prove production configuration
Switch or actual daemon checkout. Runtime Git registration is still bound to the
real local commit and bundle.
