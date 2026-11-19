# PAY-241 S3 R04 field-CAS independent check — 2026-11-18

**ACCEPTED for the bounded service/HTTP regression increment; zero blocking findings.** This accepts the two field-specific test files and the five-line handler fixture repair. Full S3 remains in progress. Recommend parent TC-16 **PARTIAL**, yielding **2 PASS / 12 PARTIAL / 10 NOT_RUN** across the unchanged 24-case denominator.

The resumed reviewer read the S3 requirements/design/implementation, final S2 binding, CAS intake, TestMain diagnosis, relevant check context and the two updated server spec appendices. It inspected the actual writers and barrier helpers, reconciled retained execution evidence and completed only the newly permitted ROOT compile/vet/format checks. It performed no new database, TestMain or business-test execution, API/browser journey, process control, staging, commit or push.

## Findings (fixed)

- File: `server/internal/handler/handler_test.go:184`.
  Issue: the shared TestMain seeded an `agent_invocation_target`, but its teardown omitted that relation. Original handler R03 had all three named tests passing while one target remained; the run was NOT_ACCEPTED. A separately retained TestMain-only RED reproduced that residue with zero named test bodies.
  Fix: the handler implementation owner added exactly five lines to delete targets whose owning agents join the actual handler-test workspace slug, using the existing parameterized, checked `pool.Exec` pattern before workspace/user deletion. Mode `0755` and every other fixture byte remain unchanged. The seed stays available throughout the package. Fresh R04 TestMain-only GREEN and handler normal/race all finish with all 30 audited relations empty and zero connections. No code repair was needed by this resumed reviewer.

- File: private `independent-check/resume-final-r01/evidence-reconciliation-r02.json`.
  Issue: the first resumed audit assumed the first added unified-diff line began the SQL block; the diff aligned a repeated closing brace there instead.
  Fix: the corrected receipt proves that removing the one exact five-line SQL block reproduces R03 bytes, and that it precedes both parent deletions. The original audit and its false alarm are retained. This correction changes audit evidence only; no Go code, database run or static command was repeated.

## Findings (not fixed)

No unresolved implementation finding was identified within this increment. The following required work remains outside its scope and was deliberately left to the parent task:

- Current authenticated API/router, production Web and matching Desktop composition for field contention and original-request recovery. These regressions use legacy-auth JWT principal fixtures, direct service calls and actual `Handler.UpdateIssue` calls through `httptest`; they do not prove password/login/router or native execution journeys.
- Ordinary requests that omit `expected_revision` retain the existing optional-version contract. The new child test proves explicit numeric CAS only; it does not make versions mandatory for all clients.
- Remaining S3 recovery: complete 55-comment pagination and WebSocket reauthentication, mixed human/Agent notifications and deduplication, revoke/read protection, original upload recovery, and the structured blocking/repair/history contracts. No complete R01–R09 or TC-16 PASS claim is warranted.
- Real model, payment/fulfillment, external sandbox/CI/deployment/rollback evidence, final performance sampling and business signatures remain S4/S5 work. No whole-ROOT acceptance is claimed.

## Verification

- **Lint: PASS** — current ROOT `gofmt -l` returned no paths for exactly the three reviewed Go files; `go vet ./internal/service ./internal/handler` exited 0. Both outputs and error streams were empty.
- **TypeCheck: PASS (Go)** — current ROOT service and handler test binaries compiled with `go test -c`; neither binary was executed. TypeScript lint/typecheck is not applicable to this Go-only increment and was not rerun.
- **Tests: PASS for the retained named increment** — exact normal/race sets, one run and one passing terminal per expected identity, no missing/unexpected/duplicate/unfinished/skip/fail/parser errors. The resumed checker read saved raw evidence; it did not rerun these database profiles.

| Accepted profile | Bound source | Top / named / leaf | Final fixture audit |
| --- | --- | --- | --- |
| service-normal | R03 | 2 / 7 / 5 | 30 relations zero; connections 0 |
| service-race | R03 | 2 / 7 / 5 | 30 relations zero; connections 0 |
| handler-normal-r04 | R04 | 1 / 3 / 2 | 30 relations zero; connections 0 |
| handler-race-r04 | R04 | 1 / 3 / 2 | 30 relations zero; connections 0 |

There are **3 unique top tests, 10 unique named identities and 7 unique leaves**, yielding 20 named executions across four distinct, serialized, fresh accepted databases. Each saved before/after/final audit has the same 853-entry migration ledger. Service teardown temporarily closed creation; the coordinator restored its original flags with epoch **2→3**. Handler R04 preserved its original flags and epoch **2** throughout. The failed R03 handler, original TestMain RED and R01/R02 NOT_RUN records retain their dispositions. TestMain-only GREEN is teardown proof with zero named bodies, not an additional business test.

Service R03 evidence remains valid: the only R03→R04 change is the handler-owned `_test.go` fixture, which is not selected by service compilation/execution. No service R04 execution is claimed.

The four missing ROOT checks ran sequentially from `/Volumes/artisan/code/2026/multica-0.4.37/server`; Go commands used the permitted private `run-nodb.mjs`/`configs/nodb.json` wrapper:

```text
go test -c -o <evidence_root>/root-static/service.test ./internal/service
go test -c -o <evidence_root>/root-static/handler.test ./internal/handler
go vet ./internal/service ./internal/handler
gofmt -l <absolute paths of the three reviewed Go files>
```

Per-command receipts retain actual absolute argv/cwd, start/end, stdout/stderr, exit, tool/config/runner and source/binary hashes. All **2,943 server Go/module files** in each static snapshot stayed byte/mode identical. This pins these checks; it does not certify the whole repository or execute any package TestMain.

## Behavior and source binding

Priority identity tests prove same-key/same-priority convergence once, changed-priority `idempotency_conflict`, and distinct-key/same-revision `stale_revision`; only the winning field, revision and exact event apply. They retain original operation IDs, payload hashes and result JSON after a deliberate fresh successful request.

Priority/transfer tests use the existing real commit/PID barriers and release/cancel/join both writers on failure paths. Current authority precedes revision: transfer-first makes the former noncreator handler's edit **403 forbidden**; priority-first makes its old-revision transfer **409 stale_revision**. Actor-owned `GetWorkflowOperation` recovers the exact content-free original receipt after write authority is lost. Old POSTs remain forbidden; fresh authorized continuation preserves both earlier outcomes and retained execution/input facts.

The ordinary linked-child test calls the original atomic writer in both orders with numeric `expected_revision`. It checks one200/one409 and exact numeric `revision_conflict` fields, the winning field only, a fresh remaining-field request, exact committed ordinary update events, **zero parent-reference hints** and unchanged parent collaboration/history/execution. The final race raw stream contains distinct real holder PID witnesses for both orders and the explicit numeric 1→2 conflict / fresh revision3 continuation.

Final R04 manifest SHA256 is `1275d9709b94d55586d155cfec42886d14663ed9090d0da34cb7388d49e62dd3`; execution-plan R04 SHA256 is `60ebfd2c216985537886f1654575357e635740457da41e79a735d3411fe9f5a4`. All **5,056 R33 base paths are retained: 5,055 unchanged, one permitted fixture delta, plus two new files = 5,058 total**. Held R04 source bytes/modes and the three ROOT mirrors match. ROOT new files retain mode0644; the fixture retains mode0755. No production code, migration, dependency or new recovery abstraction belongs to this increment.

The main-owned appendices in `collaboration-source-editing.md` and `database-test-isolation.md` match these field, authority, hint and teardown contracts. No further spec/template/platform touch point was required by this test-only change.

## Evidence and handoff

Private evidence root: `/Volumes/artisan/.multica-acceptance/pay241-s3-cas-20261118-hxjmd77_`. Machine-readable details and hashes are in sibling `cas-independent-check-20261118.json`; fresh reconciliation is `independent-check/resume-final-r01/evidence-reconciliation-r02.json`, and the new ROOT receipts are `root-static/result.json`. Earlier independent audits and failed/raw evidence remain retained.

This reviewer wrote the two assigned review reports and private supports only. No owned technical review work remains. Main owns the final source seal, parent CSV/task/progress/spec/journal bookkeeping and subsequent implementation. Keep full S3 in progress; complete its current API/Web/Desktop composition and remaining recovery contracts before closing it.
