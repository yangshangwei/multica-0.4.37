# R20 independent check of retained R19 evidence

Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`

## Verdict and scope

The retained R19 result, source bindings, artifact history and stop receipts are independently consistent. R19 remains **0 PASS / 1 FAIL / 3 NOT_RUN**, with no automatic retries. Its first failure is the original five-second source-denial banner assertion in `collaboration-stage-approvals.spec.ts:351`. This review does not accept S2, A01–A12, the parent matrix, or any external integration.

The observed banner failure has a concrete shared-client explanation: an authoritative R1 source denial cancels/scrubs R4, while `StageCodePanel` only renders the source-denial banner when an R4 record exists. The separate `55P03` does not justify weakening database locks. No production or tracer file was changed in this review.

This assignment used file reads, hashing, recorded-result parsing and static source inspection only. It ran no Go, lint, type-check, database query, API request, browser, build or process signal. Process and database statements below refer to the retained receipts at their recorded times, rather than a fresh live observation.

## Source and retained-file verification

Rehashed and checked the recorded byte, size, permission and link-target identities:

| Bound material | Independent check |
| --- | --- |
| Required live-binding references | 169 references, no mismatch |
| Frontend test graph | 3,359 declared files and 1,961 links, no mismatch |
| Production frontend graph | 3,358 declared files and 1,961 links, no mismatch |
| Actual R18 server source reused by R19 | 5,045 declared files, no mismatch |
| ROOT production dependencies | 1,458 declared files equal to the runtime source, no mismatch |
| Historical staging/output filesystem | 101 recorded inode/device/mode/byte identities preserved |
| R19 staging generations | Eight recorded empty, unused directories preserved |
| Retained artifact outputs | Eight tar files and their member names, sizes and hashes match |
| R19 runtime output | 14 recorded output files preserved |

The combined file audit hashed 13,428 distinct file paths, with zero mismatches. An additional recursive check of the eight primary records found 105 reference objects representing 85 distinct file/hash bindings, also with zero mismatches. These counts describe different audits and must not be added as independent business assertions.

The five source files relevant to the client failure have identical bytes in the test graph, production graph and ROOT:

- `packages/views/collaboration/stage-code-panel.tsx`
- `packages/core/collaboration/stage-source-cache.ts`
- `packages/core/collaboration/stage-queries.ts`
- `packages/core/collaboration/stage-code-presentation.ts`
- `packages/core/collaboration/stage-presentation.ts`

This verifies the declared source graphs and their existing dependencies. It does not claim equivalence of the entire ROOT checkout, newly undiscovered source paths, or a fresh product build.

## Original result and source-denial ordering

The original reporter, stdout and stderr hashes match `runtime/four-scenarios-r01/result.json`. Its four-case denominator, `retry=0`, first failure and three unstarted scenarios are retained.

The diagnostic raw file has 874 records, including 293 HTTP responses: 255 HTTP200, 37 HTTP503 and one HTTP404. Response hashes and source identities match the retained excerpt. The important original reviewer sequence is:

| Raw line / trace | UTC time on 2026-11-16 | Original response |
| --- | --- | --- |
| 805 / 270 | 12:01:26.005911 | R4 `LockArtifactMembers` returns PostgreSQL `55P03` |
| 807 / 270 | 12:01:26.007431 | Same R4 request returns HTTP503 `collaboration_unavailable` |
| 820 / 275 | 12:01:26.109018 | R1 HTTP200: both children have `can_open=false` and `content_digest=null` |
| 825 / 276 | 12:01:26.146042 | R4 HTTP200: exact candidate, `baseline=null`, `blocked_evidence=1` |
| 859 / 290 | 12:01:26.739337 | R1 HTTP200 repeats the current source denial |
| 868 / 293 | 12:01:26.783603 | R1 HTTP200 repeats the current source denial |

The first R1 denial at line820 precedes the denied R4 response at line825. The denied R4 contains zero decisions, test reports, execution facts, targets, execution operations and rollback observations. It retains four non-signable blocker entries; it is incorrect to describe every array, including `signable`, as empty.

Relevant client behavior in those byte-identical sources:

1. `stage-presentation.ts:10` already derives workspace source denial from an R1 child with `can_open === false`.
2. `stage-queries.ts:42` calls `eraseCollaborationStageSource` before returning that R1 to observers.
3. `stage-source-cache.ts:17` advances the source generation and line24 cancels private secondary queries. Lines42 and50 discard late results from the previous generation. Lines27–32 preserve an already accepted anonymous R4 identity and its original read/error state, but cannot preserve a result that never became current.
4. `stage-code-panel.tsx:33` sets `record=null` when the query has an error or no available data. Lines62–64 place the source-denial banner inside `record &&`.

Consequently, a current denied R1 can remove or invalidate R4 as intended and suppress the very banner that should explain the denial. The current R1 is sufficient to explain source denial even when the private R4 record was correctly scrubbed. A component regression must reproduce this missing-record boundary; copying a successful R4 response into a mock alone would miss it.

The raw does not preserve a browser-side cache-transition trace, so the exact callback interleaving is a source-supported explanation, not a claim that a particular late result was observed entering the browser cache. The original failed UI assertion and the missing-record render behavior are concrete evidence for the regression.

## Passive tracer and connection errors

The actual reused source is `r18-stage-execution-read-lock-fix-r01/runtime/build-source-r01/server/cmd/server/pay241_read_diagnostics_test.go` (231 lines). It installs a `pgx.QueryTracer` at line78, rather than a DBTX, Rows or Scan wrapper.

- Lines124–154 add context values, derive a SQL hash and record a narrowly permitted issue identity. They do not rewrite SQL, arguments, deadline or cancellation.
- Lines157–190 observe the returned error type, PostgreSQL code, source stack and `conn.PgConn().PID()`. They issue no query and do not drain or close Rows.
- Lines198–208 return the original write length from the body tee. Lines211–229 use Chi's response-writer wrapper to observe the handler response without changing its status/body.

The declared and installed dependency is pgx `v5.9.2`. Its `tracer.go:11` documents the returned-context contract. `rows.go:151` closes result readers before calling `TraceQueryEnd` at line186. `pgconn.go:658` implements `PID()` as an in-memory field read. No SQL reentry, cursor interference, explicit cancellation or query-result mutation was found in the private tracer.

The raw has 39 failed query records: 29 `*errors.errorString`, nine `*pgconn.connLockError`, and one `*pgconn.PgError` with `55P03`. All nine `connLockError` records are rollback attempts after a preceding failed query in the same request (raw pairs14/15, 24/25, 103/104, 108/109, 120/121, 207/208, 293/294, 446/447 and456/457).

The tracer records the connection error type but not its status or the preceding error message. pgx `pgconn.go:771` uses `connLockError` for busy, closed and uninitialized connections. Neither raw log includes those error messages. These records therefore do not prove a busy-connection bug, request cancellation or a specific root cause for the earlier failures. They also do not prove the tracer introduced those errors. Synchronous file encoding adds scheduling overhead, which this static review did not measure.

The `LockArtifactMembers` query is already `FOR SHARE NOWAIT`. The original requester received source-safe HTTP200 projections shortly after its transient rejection. Retain the rejection and all other HTTP503s; do not report that all503s disappeared or remove the membership fence to make the UI assertion pass.

## Recorded resource and history classification

Independently recomputing `database-after-stop-r01.json` from its recorded rows gives:

- Eleven task rows: nine failed, one deferred and one running. No prepare lease or execution binding remains. These history rows were preserved, not deleted or manually rewritten.
- Nineteen Host runs, all with `settled_at`; no unsettled run or unreleased reservation remains.
- Eight artifact executions, all succeeded with matching retained tar outputs.
- Two stage intents and three observations; one intent remains unresolved/unknown. No rollback metric was recorded, because the metric scenario did not run in R19.
- Gate booleans are `enabled=false`, `creation_enabled=true`, `host_execution_enabled=false`; epoch remains20. The snapshot has zero other database connections.

The stop receipt records API34208 and bridge34204 absent, no API64922 listener, and no forced signals. Web39744 on64923 and renderer39748 on64924 remain held by the peer and were not signaled. The eight unused R19 generations and all historical sealed/unused inputs are preserved. No evidence establishes that the retained task rows are permanently inert.

`after-audit-preparation-r01.json` is explicitly a preparation receipt. Actual after-state evidence is the separate `after-four-files-history-r01.json`, then the stop receipt and database-after-stop snapshot. The preparation status alone must not be described as an executed after audit.

## Handoff

Implement the source-denial explanation in the shared code panel using the existing current R1 denial signal, independently of an R4 record. First add the component regression for a denied R1 with no record, including pending/error states and absence of private details or privileged actions. Keep the existing source-generation scrub, source authority and all SQL locks. Rebuild affected Web/Desktop outputs after the reviewed product change and bind a fresh runtime round; retain R19 and all earlier results.

Lint, type-check and tests were **not run** under this read-only review assignment. The implementation/check lane must supply those results and a fresh actual four-scenario result before S2 acceptance changes.
