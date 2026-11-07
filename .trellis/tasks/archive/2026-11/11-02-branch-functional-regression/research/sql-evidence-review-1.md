# Independent SQL diagnostic evidence review

**PASS WITH NOTES for the diagnostic evidence. QA-PERF-001 remains FAIL (2,171ms > 2,000ms), and the task remains in_progress.**

This is an independent Trellis check of the completed canonical SQL/transport diagnostic, focused SQL parity result, local prerequisite checks, cleanup evidence and current coordination documents. The reviewer connected to no database, ran no test/build/service, changed no product or prior evidence, and wrote only this report and its JSON companion. Earlier child 429 records remain historical; this completed review is separate from the coordinator fallback.

## Findings (fixed)

No fix was required. The final reports already preserve the required limits: EXPLAIN timing is not original batch latency, savepoint rollback does not undo physical WAL/cache effects, and read-transport parity is not proof of production concurrency/error semantics. The new snapshot assertion is correctly limited to entry count, three labels and non-null priority, rather than exact label/priority values.

## Findings (not fixed)

- The original failing P95 gate and remaining platform/authentication/updater/trust/Provider gates are outside the passing diagnostic scope and remain open.
- Existing event indexes are valid. The observed repeated bitmap scans and top-N sorts justify a stats/prepared-plan diagnostic; they do not yet prove a missing index or explain the exact earlier tail samples. No index or product change is justified by this review alone.
- The ABBA replay holds the original lock set already, has no active-task rows and absent target participation, and uses generic row-value consumption. First-lock waiting, nonempty results, existing targets, fresh-snapshot concurrency and typed no-row/error control flow require regression evidence before any product optimization.

These are documented scope limits, not undisclosed defects in the current evidence.

## Evidence verification

Independent recomputation matched all command-stream, test/runner/overlay/metric hashes and the sole terminal diagnostic PASS. All 6,000 saved plans use exact current generated SQL. The original 1,000 active-task reads, ordered source/target participation reads, exact write parameter populations and seven original batch phases reconcile with the production paths and lock order.

The three write EXPLAIN execution sums independently recompute to source leave **651.403ms**, target join **221.459ms**, and issue CAS **85.479ms**. Source and target event bitmap scans visit **3,003,000** and **1,001,000** rows across their respective 1,000 statements. Index/node counts, root WAL/buffer totals and 22/34/2/2 valid-ready-live index counts match the raw catalog and plans. Nested node timings were not double-counted.

All four 3,000-element result-digest vectors match, with 1,000 source participation rows and zero active tasks. Savepoint logical fingerprints match before/after. Serial/pipeline values and their means in the reports match the raw ABBA samples. EXPLAIN planning/execution sums are not subtracted from those client spans.

The fresh source diff and frozen test hash equal their recorded pins; the overlay only adds a nonexistent virtual test path. The 5,900-file starting ledger has changes only in the expected coordination documents; original raw evidence and the 439-case inventory are unchanged. Current progress/report/defects/matrix/session prompt consistently retain original FAIL and diagnostic limitations.

Saved cleanup streams and their hashes support zero connections in all five recorded task databases. The reviewer did not query a database. Fresh independent read-only checks found no selected task process or listener on the six recorded ports; the exact multica:// default remains /Applications/Multica.app and the whole same-bundle registration set matches the original capture. No application was launched or registration changed by this reviewer.

## Verification

- Lint/TypeCheck: not rerun; this batch changes only task records and private diagnostics, with no product/dependency change.
- Tests: reviewer did not rerun tests. Saved actual diagnostic compilation/execution and DB-free SQL parity test are PASS with matching hashes and terminal events.
- Static/evidence checks: JSON/CSV, SQL/parameter/plan arithmetic, source preservation, historical fingerprints, native cleanup and git diff --check PASS.

Exact reviewed hashes and fresh read-only native observations are in research/sql-evidence-review-1.json. Review conclusions apply to those hashes; later evidence or scope needs its own review.
