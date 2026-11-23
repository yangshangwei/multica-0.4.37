# S2 increment 2b — artifact report read-authority correction

Status: **implemented; focused normal/race/static checks passed; independent acceptance pending**. S2B-C01 is corrected in the current source, but the reviewer owns closure. The project policy activation gate remains false. No 2c execution-ledger implementation, migration or whole-S2 acceptance is claimed.

## Behavior corrected

T4 now shares R4's repeatable-read transaction that permits M1 authority row locks. Before cursor validation, sequence lookup or page filtering, it reads the run's original frozen test subject and checks that exact retained artifact through `lockDeliveryArtifactInTx`. A missing report or an empty filtered page cannot bypass this source check. Legacy authorized runs without a frozen subject still return an empty list.

The artifact check uses the frozen subject's original child, approval, execution, spec, receipt and artifact identities. It deliberately does not require the old candidate to remain current, its PR head to remain unchanged, or a failed test to become passing. Current M1 principals and credentials are still required and locked through transaction completion. Report payloads must agree with that original subject; T4 errors discard the entire response projection.

QA optional report references use the same artifact read check before the report is accepted as evidence. An authorized manager can cite failed evidence; an ordinary source member or a demoted manager cannot. A reason-only return remains available. QA standing/final consumers pass their actual transaction through the same helper. R4 retains its existing current-evidence observer and source-withheld projection.

## Regressions and first failures

- The initial inherited artifact fixture omitted `agent_approval_request.executed_at`; **red-1** stopped during candidate setup. It is retained as a fixture failure, not as the vulnerability reproduction.
- Correcting only that fixture produced **red-2**: five real failures—ordinary source member T4 detail/list, optional QA report reference, and demoted manager detail/list all succeeded incorrectly. Authorized initiator/manager and R4-denial preconditions succeeded. The original report and artifact were created through actual packager bytes, stage/comment/claim/report services; the retained M1 approval/execution rows are explicitly synthetic fixtures, not proof of M1 approval HTTP acceptance.
- **green-1** passed the same initial vulnerability regression after the product correction.
- Later tests use a second database connection and canonical `UpdateMemberRole`: list/detail/R4/QA-first holds the member lock and the competing update receives `55P03`; demotion succeeds after the original transaction completes. If demotion commits after the repeatable-read access snapshot but before the artifact lock, T4 receives `40001` and returns no page/detail, then a fresh request receives `not_found`.
- Actual two-attempt host-permit/report history produces a real manager cursor; after demotion the cached cursor and report ID cannot restore access. The test launches no model or external Provider process.
- Other checks cover artifact-bound empty pages before cursor parsing, the unbound-run empty-list control, ordinary source member denial, positive manager QA citation, demoted QA citation denial, reason-only return, and unchanged readable failed history after candidate supersession and PR-head change.
- An external `service_test` test reuses the same test-only fixture export and invokes the actual T4 HTTP handler adapters. Positive responses are `no-store`; member/demoted-manager list/detail failures contain exactly `code` and `error`, without report/case/log/count data. This is handler/service evidence with fixture authentication context, not a launched router/auth/browser journey.
- **extended-1** retained a handler test setup mistake (replacing the request context discarded chi route parameters). **extended-2** retained a new cursor test's `int64`/`int` compilation error. **extended-3** retained a QA race seam using the wrong canonical query name; `hook=0` identified the unexecuted assertion. Each was corrected in test code, with no relaxation of product assertions, timeout, pool or cleanup rules.

## Final verification

Counts include named subtests, overlap, and must not be added as acceptance cases.

| Final command scope | Named pass / fail / skip | Exit |
| --- | ---: | ---: |
| service `^TestStage` | 125 / 0 / 0 | 0 |
| service focused artifact/QA/source/expiry `-race` | 25 / 0 / 0 | 0 |
| handler `^TestStage` | 44 / 0 / 0 | 0 |
| handler `^TestStage -race` | 44 / 0 / 0 | 0 |
| affected service/handler `go vet` | static | 0 |
| `go build ./...` | static | 0 |
| frozen-overlay `gofmt -l`; scoped `git diff --check` | empty output | 0 |

All four DB-backed commands use the original exact-package `run.mjs` profiles, migrated through828 by the coordinator. No database was created, migrated, dropped or switched by this lane. All four final package processes have ended and their profiles were released for independent checking. One mistyped handler launcher path failed before the private profile could be read or Go launched; `handler-launch-path-error.json` records the attempted command and honestly marks its uncaptured exit. It is not a test case or a product failure.

## Source freeze and ownership

The JSON companion binds **14 server source files**, **14 raw logs with direct exit sidecars**, capture metadata and a final current/frozen hash check. The explicit overlay is:

`/Users/artisan/.multica/acceptance/pay241-20261114-resume/s2-server-inc2b-acl-fix/source`

Apply it over the previously checked 4,889-file reconstruction at `s2-server-inc2b-check/source`; five overlay paths are new. Do not silently fill from live files. Earlier 2b reports and their frozen sources are unchanged.

- Product correction: shared evidence reader, report view, QA, review view, and transaction propagation through review/test-subject consumers.
- Test correction: artifact fixture/matrices, actual handler adapter test, and the inherited stage/runtime fixture setup extensions.
- Explicit inherited supplement: `handler/collaboration_stage_test.go` contains the already-existing source-denied digest fixture adaptation (workspace timestamp14:38Z), absent from the older checked reconstruction. This lane did not modify it; it is bound because the current handler verification uses it.
- Explicit pre-existing 2c drafts: `collaboration_stage_execution_types.go`, `collaboration_stage_execution_serialization_test.go`, optional review DTO fields and the private `stageReviewPayload.Execution` member. They were present before this correction; formatting was completed and current compilation plus the original-request serialization tests now pass. This does not implement or certify release/rollback execution.

No git writes, new dependencies, policy activation, CLI behavior change or migration was performed. Specs and commits remain coordinator-owned.

## Remaining work

Independent Trellis review must decide S2B-C01 closure against this freeze and its runtime evidence. After that handoff, the same server owner continues the already-frozen2c contract, then2d all-writer gates. Full A01–A12, production Web/matching Desktop, actual router/CLI/owned-Git journey and real external integrations remain separate. The parent matrix is unchanged. The eight pre-existing F2 migration-cleanup omissions remain unmodified.
