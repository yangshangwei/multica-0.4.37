# Independent cached-plan and read-semantics evidence review

**PASS WITH NOTES for the completed diagnostics. QA-PERF-001 remains FAIL at 2,171ms > 2,000ms; required acceptance gates remain open.**

Independent reviewer: native Trellis check child `/root/cached_semantics_evidence_review`. This review covers the two completed implementation-child slices plus the coordinator's separately numbered no-statistics reprepare control. It connected to no database, ran no test/build/service or account action, changed no product/harness/prior evidence, and writes only this report and its JSON companion. Child self-verification, coordinator verification and this independent conclusion remain separate.

## Findings (fixed)

No blocking evidence or product issue required a fix. No raw evidence was edited. The reviewed reports already distinguish actual cached-plan capture from EXPLAIN substitution, typed original execution from generic replay, complete logical rollback from physical reset, and diagnostics from acceptance.

## Findings (not fixed)

- Original P95 FAIL and remaining platform/authentication/updater/trust/Provider gates are outside these passing diagnostics. They remain open and the task remains in_progress.
- No-statistics plans retain repeated bitmap scans and sorting after both cache reuse and fresh preparation; ANALYZE with replanning selects the existing ordered index path. This narrows the controlled fixture's mechanism but neither reconstructs the original tail samples nor isolates one marginal column statistic. It does not justify another index or product ANALYZE.
- Cached-plan original passes exercise production typed mutation consumers but deliberately roll back at the Commit boundary. Replays consume generic values, reuse captured write arguments, and do not rerun service decisions. No durable endpoint-commit or future optimization-equivalence claim is supported.
- The small typed/protocol test proves first acquisition, waited RC statement visibility, nonempty/empty/existing/missing rows, generated Scan/ErrNoRows and server-error/Close rollback. Existing real handler/service regressions add authority, CAS and owner rollback proof. Malformed decode, delayed Rows.Err and the eventual implemented optimizer's complete scheduling remain unproved.
- Six-table business fingerprints exclude notifications and cannot undo WAL, cache, dead tuples, reltuples/relpages or cumulative ANALYZE tracking. Instrumented durations cannot be subtracted from earlier endpoint/P95 or added to ABBA transport savings.

These are accurately documented scope limits, not hidden defects in the raw evidence.

## Actual cached-plan verification

All **14,000 actual auto_explain plans** were independently reparsed from raw backend notices: 6,000 in run1 and 8,000 in run2. Every SQL text matches current generated canonical SQL, every bound parameter vector matches its captured batch entry, and every saved parsed plan matches its notice. Counter deltas, prepared names/times, backend identity, table/column statistics, valid/ready/live index definitions, node counts, WAL/buffer totals and duration arithmetic reconcile.

Both no-statistics baselines in run1 finish before its eight explicit ANALYZE statements. Same prepared names/times on backend2643 retain bitmap+sort in the cache control, then use `iteration_event_sequence` Backward without sorting after ANALYZE. Source/target event rows drop from 3,003,000/1,001,000 to 1,000/1,000 per selected 1,000-statement batch.

Run2 additionally resets only its dedicated backend7356 with `pgx.Conn.DeallocateAll`. The local pgx5.9.2 implementation resets server and client caches. Raw reset snapshots prove zero selected prepared statements, identical no-statistics table counts/settings/indexes, and no column statistics. Fresh preparation has new prepare_time and5custom/995generic selections per query, while deterministic SQL-hash names stay the same. It still scans/sorts3,003,000/1,001,000 event rows. Only the later explicitly analyzed stage selects the backward existing-index path with1,000/1,000 rows. Generic plan selection itself is therefore insufficient to explain a poor path.

Independently recomputed run2 notice-duration sums are source/target710.095/234.867ms original,690.996/244.794ms cache control,667.284/236.153ms fresh prepare, and16.908/11.184ms after ANALYZE. They are instrumented execution values, not original tail latency or forecast savings.

Both original command sequences retain3,058 commands and seven ordered1,000-statement membership batches. All three run1 and all four run2 complete transaction rollbacks recover their respective initial six-table fingerprints. Post-rollback column statistics disappear while physical estimates/tracking can remain, matching the reports.

## Typed reads and current product regressions

The task adapter feeds exact canonical SQL and parameters through separate pgx batch statements into the existing generated typed scanners. Complete serial/pipeline structures match for four active tasks, an empty issue, current source, left target and missing target. ErrNoRows is followed by successful consumption of subsequent results.

A real source-participation first-acquisition wait is identified by waiter/holder PIDs and `pg_blocking_pids`. After holder commit, the locked source's changed started flag, later current-target read and active-task count4→3 all match the saved facts. This is a protocol proof with a deliberately direct holder; it is not a safe-placement proof for a lifecycle optimizer. Injected SQLSTATE22012 reaches the result and Close, and owner rollback removes the preceding marker write.

Saved race executions independently reconcile to handler4top+6sub PASS and service8top+8sub PASS, with no FAIL/SKIP. Source inspection confirms the real handler tests exercise both revocation commit orders; service fixtures' no-op authorization callback is correctly disclosed. CAS, final snapshot failure, swapped scopes, rollover/time and running-task preservation claims match the specific executed tests. The two cached diagnostic identities also each pass once. None increments original Go/E2E inventory.

## Identity, history and cleanup

All command streams, test/runner/overlay/metric and recorded artifact hashes match. Three source checkouts retain pinned HEAD, product/test patch and frozen-test hashes; additive overlay targets remain absent on disk. All5,949 starting task files have no missing or unexpected changes; all36 other workspace/prior-task starting contents are preserved, with the intentional append-only spec update described in the final supplement. The439-case inventory remains intact and PASS by current identity. The original30-measurement P95 failure and its raw logs remain intact.

Fresh read-only process/port/protocol/container/mount checks found no selected owned process, listener on six recorded ports, task container, task mounted image or new protocol registration. Exact multica:// default remains /Applications/Multica.app and the entire preexisting same-bundle set matches. Saved coordinator cleanup records zero connections in all task databases; the reviewer did not connect to a database. Latest cleanup2 including cached_diag_2 was also verified: all nine recorded databases have zero connections.

The coordinator's observed429 and empty intermediate-message delivery are recorded separately from successful child execution and this completed independent review. No child tool-work failure is inferred from empty message payloads.

## Verification

- Lint/TypeCheck: not rerun; no product/dependency changes and no broad suite repeats authorized by this evidence review.
- Tests: reviewer did not rerun tests; actual saved diagnostic and focused race executions above are PASS with matching hashes and terminal events.
- Evidence/static checks: SQL/parameter/plan arithmetic, all recorded hashes, source/history preservation, Python AST, Go formatting and git diff --check PASS.
- Current batch progress/report/defects/matrix/session prompt now reconcile with the new runs and required open gates. Final document consistency is recorded below. No raw-evidence blocker was found.

Exact reviewed hashes and recomputations are in `research/cached-semantics-evidence-review-1.json`. Review applies to those hashes; later scope or evidence requires its own reconciliation.



## Final supplement: reprepare, current documents and cleanup2

The coordinator's separately numbered run2 is included in this completed review, with all8,000 additional plans and reset facts independently recomputed as described above. New hashes for run2/report/control plan, coordinator verification and cleanup2 are retained in the JSON companion.

Current progress, report, defects, conditional matrix, session prompt and task metadata consistently describe the completed statistics/cache/reprepare and typed-semantic diagnostics, the minimum possible future scope, original2171msFAIL and unclosed required gates. Matrix31rows are21PASS/1FAIL/6BLOCKED/3NOT_RUN; the three new PASS rows are explicitly diagnostic only. Task status remains in_progress. Review-status links may be updated after this completed review without changing raw results.

All5,949 historical task files were rechecked after document updates, with changes only to the eight expected coordination records and no missing file or changed historical raw evidence. Of36other preserved workspace/prior-task files,35remain identical; the coordinator appended the new prepared-plan/statistics contract to database-test-isolation.md. Its entire starting13147-byte content remains an exact prefix, so preexisting edits are preserved. The appended rules match this review's actual capture, reset, stats and rollback evidence; no product boundary or SQL was changed. The JSON companion records exact current hashes.

Cleanup2 command streams match their recorded hashes and confirm all9task database connections0, six ports closed, no task process/container/mount/protocol registration and original exact default/registration set preserved. Reviewer connected to no database.

The original saved P95 log was independently reassembled from same-test Go Output fragments. Its35closure samples minus5warmups leave30measurements; the frozen sorted[28] is2171.000ms and terminal action isFAIL. No test was rerun or acceptance status reclassified.

Final verdict: **PASS WITH NOTES for the scoped diagnostic evidence**, with no unresolved blocking evidence defect. Original performanceFAIL and required acceptance gates remain open.
