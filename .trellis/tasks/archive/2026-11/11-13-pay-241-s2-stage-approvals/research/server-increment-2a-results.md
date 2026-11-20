# S2 server increment 2a — results

- Owner: `/root/s2_server`. Scope: signer/current-authority binding, A6, R4, code-review A7, A8, the development/code_review/QA/rework A9 subset, reopen reset, and the two GitHub source-integrity repairs.
- Status: **implemented and narrowly verified; not whole S2 acceptance**. `stagePolicyActivationAvailable` remains `false`. QA decisions/reports, execution facts and completion/exit guards remain 2b–2d.
- Start HEAD `482cd7f683739aca72050b6b09fdfe724742a898`; report HEAD `8e824b3035c4f17a9a9dfa0504f1294fbcc9b20b`. The intervening commit is root's S5-only commit. This owner made no git writes.
- No API/daemon launch, real model, payment, GitHub call, deployment or rollback. All external facts and human signatures below are synthetic fixtures.
- `server-increment-2a-results.json` contains the exact 45 source/contract file paths and hashes, 34 command records with captured exit codes, two additional static checks, raw-log hashes, every named test terminal and parent migration evidence references.

## 1. Delivered behaviour

**Code and review:** A6 freezes the current root requirement, child semantic version, configured implementer, PR/head, integration identity/trust and optional existing M1 artifact observation. Code-candidate versions are immutable; supersession requires the current exact candidate. R1 lists at most 50 identity-only code candidates. R4 projects source evidence and decisions, or its exact source-denied variant. Normal ordinary-child status changes do not invalidate code identity.

**Strict CI:** a GitHub snapshot must be complete, nonempty, for the current head, observed no later than the database clock and within 30 minutes, with every check completed/success. The digest excludes refresh time and preserves check-set identity. VCS statuses without repository-level check identity remain unsupported, including a synthetic aggregate reporting success.

**Human decisions:** only the current configured reviewer may approve/return code; the candidate's implementer cannot review it, including after reassignment. Approval expiry is bounded by the database clock, CI/evidence horizon, 168-hour ceiling and original credential expiry. Returns require a reason and null expiry; failed evidence can be returned without pretending it passed. Optional references still require same-candidate/current-read validation.

**Authority and recovery:** A7 retains participant/account/role/policy identity, original JWT/PAT/password-session metadata and the original source member record identity. Consumption checks the original signer, current credential and the same source grant under live locks. Removing and re-adding membership cannot revive an approval; a fresh explicit signature works. Same-key writes/recovery retain exact candidate/decision identity, including an uncertain commit. Revocation appends a fact and preserves the original decision.

**Transitions:** development → code_review requires strict PR/CI evidence; code_review → QA requires a valid exact human approval. Rework to development consumes an exact returned decision. Reopen resets development and advances stage revision; old lifecycle approvals cannot be consumed. Source/CI/trust and current credentials are fenced/rechecked at consumption. No transition dispatches work.

**A5 clarification:** scope confirmation verifies the current human credential when writing, including immediately before recording the decision. It retains no credential expiry after commit. Lifecycle/enabled-policy-period/participant/account/role revision determine later scope applicability. Removing and restoring a role does not revive the old scope. This is the coordinator's explicit interpretation of 7.5 versus F01; no scope credential column was introduced.

## 2. GitHub producer repairs

1. `FetchPRSnapshot` rejects the entire response if any union node is unknown, null, malformed or lacks a check identity. Successful complete pagination sets the private completeness marker; the normal producer forwards it to storage. Migration 821 leaves old snapshots incomplete until refreshed through the fixed producer.
2. The PR upsert refuses older observations. Equal provider timestamps with different heads are ambiguous: the same SQL statement preserves the known head/metadata and invalidates strict snapshot completeness. Equal timestamps with the same head preserve normal metadata events.
3. A callback can win its upsert and then lose to a newer observation. Its derived-link write now locks and matches the exact PR head/timestamp; a superseded callback derives no further links, broadcast or completion intent. This closes the reproduced interleaving without adding a general webhook delivery system.

These are local producer/consumer contracts. They do not prove remote GitHub ordering, delivery reliability or real CI integration.

## 3. Files and migrations

| Area | Files / responsibility |
| --- | --- |
| New stage service | `collaboration_stage_review_types.go`, `collaboration_stage_evidence.go`, `collaboration_stage_signer.go`, `collaboration_stage_review.go`, `collaboration_stage_review_view.go` under `server/internal/service/`. |
| Existing service wiring | `collaboration_stage.go` (kinds/closed confirmations/scope role revision), `collaboration_stage_actions.go` (A5 current credential), `collaboration_stage_view.go` (R1 discovery), `collaboration_workflow.go` (allowlists/source member locks), `collaboration_lifecycle.go` (reopen reset). |
| HTTP | New handler `collaboration_stage_review.go`; five routes in `cmd/server/router.go`; existing stage fixture cleanup and workspace deletion-manifest classification. |
| GitHub | Handler `github.go`, queries `github.sql` / `github_snapshot.sql`, `ghsnapshot/snapshot.go` / `refresh.go`; timestamp and typed-row fixture adjustments in existing tests. |
| Tests | New service `collaboration_stage_review_test.go`; new handler `collaboration_stage_review_test.go` / `github_stage_evidence_test.go`; extended snapshot and existing stage tests. The shared scope probe supports the new consumption interleavings. |
| SQL/generated | New `collaboration_stage_review.sql`; sqlc regeneration of its file, existing stage/GitHub files and models; `sqlc.yaml` excludes retained signer credentials from generated JSON. No generated code was hand-edited. |
| Documentation | `wire-contract.md` and the two 2a reports. No client or other-owner file was edited. |

| Migration | Content |
| --- | --- |
| 821 | Extend immutable candidate/decision ledgers; private signer metadata and expiry; immutable revocation table; GitHub snapshot completeness marker. No foreign key/cascade. |
| 822–824 | Three separate single-statement concurrent unique indexes: code version, revocation decision, revocation ID. |
| 825 | Check index availability and put retained-history rollback refusal before any new index can be dropped. |

Root applied 821–824 and then 825 to the four existing allocated databases. DDL hashes remained unchanged across each application. Root verified identities, ledgers, valid/ready indexes and the completeness column. Evidence under `/Users/artisan/.multica/acceptance/pay241-20261114-resume/`: `s2-inc2a-ddl-before.json`, `s2-inc2a-migrations.json`, `s2-inc2a-schema-audit.json`, `s2-inc2a-825-migrations.json`. This child ran no migration command. The new down guards were not executed by this child.

## 4. Commands and final evidence

All logs below are under `/Users/artisan/.multica/acceptance/pay241-20261113-01/logs/`, with prefix `s2-server-inc2a-`. Every DB-backed run used `run.mjs` and exactly the assigned package/config: `s2_impl_service`, `s2_impl_handler`, `s2_race_service`, `s2_race_handler`. The launcher checked database identity without printing credentials. The pure snapshot selector has no TestMain and selects no DB-backed refresh tests.

| Final check | Log suffix | Exit | Result |
| --- | --- | --- | --- |
| Service stage + affected workflow/delivery regressions | `service-normal-3.log` | 0 | 39 top-level / 107 named pass; zero fail/skip |
| Same service selector with `-race` | `service-race-2.log` | 0 | 39 top-level / 107 named pass; zero fail/skip |
| Handler stage + GitHub webhook/iteration regressions | `handler-normal-3.log` | 0 | 43 top-level / 72 named pass; zero fail/skip |
| Same handler selector with `-race` | `handler-race-2.log` | 0 | 43 top-level / 72 named pass; zero fail/skip |
| Pure snapshot fetch/decision tests with `-race` | `snapshot-race-1.log` | 0 | 6 top-level / 17 named pass; zero fail/skip |
| Build service, handler and server command | `build-3.log` | 0 | Final source compiles |
| Vet service, handler, snapshot and server command | `vet-1.log` | 0 | No diagnostics |
| sqlc generation | `sqlc-4.log` | 0 | Final query/schema generation |
| gofmt on all owned Go files; scoped diff check | `gofmt-1.log`, `diff-check-1.log` | 0 / 0 | Empty output |

Named totals include parent tests and subtests; they are not independent acceptance cases. Earlier normal/race passes and every intervening failure are retained, rather than overwritten. No full-package/full-repository suite or A01–A12 run is claimed. Builds/tests were paused during root's S5 timing window and resumed only after release.

## 5. Named behavioural inventory

| Contract | Canonical assertions |
| --- | --- |
| Candidate discovery/versioning | `TestStageCodeCandidateDiscoveryEmpty`, `TestStageCodeCandidateVersionsAndReviewerIsolation`, `TestStageCodeCandidateRejectsUnboundInputs`: bounded identity projection, exact supersession, wrong PR/head/child/artifact refusal, no candidate on refusal. |
| Review/transition/recovery | `TestStageCodeReviewTransitionsAndOriginalKey`: PR/CI → review, no QA without approval, exact decision/recovery, changed payload conflict, reopen reset. `TestStageCodeReviewConcurrentIntentAndLostCommit`: concurrent identical intent creates one decision; unknown commit recovers by original key. |
| Strict CI | `TestStageCodeReviewStrictCIRefusals`: absent/incomplete/wrong-head/stale/future/running/skipped/neutral/failed checks cannot advance. `TestStageCodeVCSWithoutRepositoryCheckIdentityIsUnsupported`: apparently successful VCS statuses cannot substitute. |
| Source identity and locks | `TestStageCodeReviewDigestAndSourceDrift`: refresh time stays stable; head/check/trust/child/requirement drift blocks. `TestStageCodeReviewFencesChecksAndTrustThroughConsumption`: real second-connection head/check/trust writers fail against signing and advancement fences. |
| Signer authority | `TestStageCodeReviewSignerRevocationAndExpiryBounds`: participant/account/PAT/password/source-access changes; invalid horizon is not stored; real expiry prevents consumption. `TestStageCodeReviewSourceMembershipRejoinDoesNotReviveApproval`: source rejoin does not revive old approval, fresh signature succeeds. |
| Human decisions | `TestStageCodeReviewReturnAndSourcePrivacy`, `TestStageCodeReviewRevocationIsAppendOnly`: failed CI can be returned, rework consumes return, revocation is immutable/recoverable, source-denied view contains no source references/history. Reviewer isolation also covers creator, machine, agent-header and original implementer cases. |
| A5 | `TestStageScopeRoleRestorationDoesNotReviveApproval`, `TestStageScopeWriteRequiresCurrentCredentialWithoutRetainedExpiry`, plus existing first-enable/reopen/verified-code tests. |
| HTTP | `TestStageHTTPCodeReviewContractAndProtectedEvidence`, `TestStageHTTPReviewStrictBodies`: explicit nullable fields, closed bodies, source-denied R4, valid blocker stages, A6/A7/A8/A9/O1, lost current read authority. |
| Producer source gaps | `TestFetchPRSnapshotRejectsIncompleteNormalizedCheckSet` and full-pagination positive; `TestStageGitHubOlderEventCannotRestoreOldHeadOrLinks`: older event, equal-timestamp ambiguity, and post-upsert interleaving. Existing metadata/synchronize and other webhook regressions remain passing. |

## 6. First failures and other failed attempts

| Log suffix | What actually failed |
| --- | --- |
| `snapshot-red-1.log` | Unknown/null/malformed/nameless nodes produced a partial usable snapshot. |
| `discovery-roles-red-1.log` | Discovery absent; role restoration revived an earlier scope. |
| `github-event-red-3.log` | Older callback restored old head or derived an old issue link. |
| `github-link-race-red-1.log` | Callback lost after upsert but still derived an old link. |
| `scope-credential-red-1.log` | Expired JWT successfully confirmed scope. The revoked-PAT subcase already failed `not_found`; its original assertion was too narrow and was corrected to accept that early denial. |
| `membership-rejoin-red-1.log` | Rejoining source membership revived the old standing approval. |
| `github-equal-time-red-1.log` | Equal timestamp/different head changed the mirror/link or left strict CI usable. |

Other retained failures are not presented as behavioural proof: `github-event-red-1` had a missing fixture issue prefix; `github-event-red-2` failed compilation during a draft type collision; `build-scaffold-1` found an incorrect project-query name; `github-equal-time-green-1` passed the new ambiguity assertion but exposed a pre-existing fixture using identical timestamps for different heads. The synchronize fixture now states a strictly newer timestamp, retaining its original new-head/mergeability assertions.

New code/review journey tests that passed on first execution are characterization/regression coverage, not retroactively claimed red-first evidence.

## 7. Remaining boundaries and next handoff

- Public enabling and full readiness remain 2d, after every completion/submission/project-exit writer consumes the stage guard in all M1 modes. Stage activity safe projections also remain 2d.
- The coordinator's 2d transport constraint preserves R38's existing `{code,error,request_id}` error envelope, rollback on failure and GET recovery of successful receipts only. Do not infer a stored R38 failure receipt from generic workflow behaviour. Project DELETE/delete-impact fields must be confirmed with the client when those guards are served.
- 2b first freezes T4 list/detail/cases/logs/errors/pagination and the create-only comment `test_subject` DTO. Wire 8.7's `artifact_digest` must then allow the source-only null variant already specified by `test-report-contract.md` §4. Client 3b2 needs that interface and a reachable test initiation path. No T4 shape was invented here.
- QA reports/decisions and later transitions remain 2b; release/rollback/execution facts remain 2c. Current R4 report/execution arrays are empty and these signing steps are unavailable.
- Selected artifacts reuse the existing M1 observer for binding, current authority and consumption, including protected R4 projection. Unknown artifact refusal is tested; a complete positive S2 artifact journey was not exercised here and belongs in the later integrated acceptance.
- The `ghsnapshot` DB-backed refresh package was compiled but not executed using an unallocated database. Root owns the additional migration/down and broader package verification. Pure producer and service-consumer tests do not claim real GitHub integration.
- Low-entropy child-content digest guessing remains flagged for independent privacy review. Unrelated Autopilot batch-assignment and `report_snapshot*` deletion-manifest failures were not changed or rerun.
- No production-router/password-auth journey, Web/Desktop acceptance, real model/payment/deployment or complete S2 acceptance was performed.

The server owner stops at the 2a report boundary for coordinator review and the 2b continuation.
