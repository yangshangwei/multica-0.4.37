# R22 actual original-product RED outcome: setup blocked

Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`.

## Verdict

The original observation is complete, but **the expected RED gate is NOT ACCEPTED**. Actual result is4 top-level/8 named/7 leaf failures,0 PASS/SKIP, with every leaf stopping in shared fixture setup at SQLSTATE42703. No intended coexistence failure, passing writer fence/current-authority control, product GREEN or R20 holder conclusion is proved by this run.

Original result SHA256 `3716fbe02686096cef2439c196c57b280ff157611fabc678dff6ff58eacfe431`; raw stdout SHA256 `805de1d17c7f24a73d933ce32c12da0bf84180f2ac6988b720899054e341d02a`; consumed binding SHA256 `5894150ac36a765dfbecdb2c679095fd57bc9b6b637b115b35d896fde650d109`. All originals remain retained.

## Actual failure and schema

Seven leaf error outputs report `column "issue_id" does not exist (SQLSTATE 42703)`. Raw SQL diagnostics identify `SELECT count(*) FROM collaboration_run WHERE issue_id=$1`. The shared fixture supplies this query at `collaboration_runtime_capabilities_read_concurrency_test.go:51`, before it returns to any concurrency or authority test body. The enclosing authority parent is the eighth named failure.

Real migration571 defines `collaboration_run` with `session_id` (`:82–110`); `issue_id` is on `collaboration_session` (`:35–47`) and `collaboration_run_event` (`:112–120`). Migration620 retains/reintroduces current `session_id` during execution cutover (`:82–99`), with no run issue_id column. Generated `CollaborationRun` likewise has SessionID and no IssueID. `testutil.Count` forwards the supplied SQL through real QueryRow/Scan and fails the caller on the database error; it does not fake the response.

The initial actual capability registration/GET, QA artifact R4 and matching-account/binding prerequisite guards occur before this count and were passed. The held read/read barriers, distinct-PID comparisons, changed-summary writer and four authority mutations occur after fixture return and were **not reached**. These are seven instances of the same test-setup defect, not seven platform defects.

## Earlier review limitation

The earlier corrected static boundary review remains unchanged at JSON SHA256 `847336263df74c20bf1d75d428d85e100c4814a3e35761fb5a3d45e539c43ce1`. It correctly checked the valid FileCount31 correction, real Go types, authority/barrier graph and immutable inputs, but **missed the new count-query's schema assumption**. The reviewer did not follow that raw SQL predicate to the actual collaboration_run DDL. Its earlier accepted static status is historical and does not establish a passed setup or control.

Future boundary checks should match every new raw fixture/effect-count SQL column to current frozen DDL/generated models and require actual progression to the named barrier before accepting concurrency evidence.

## Raw, command and after checks

The exact eight-name run/terminal inventory matches the consumed command, with one package FAIL, no duplicate terminal, parse error, build event or skip. Actual `go test -json ./internal/service` used the corrected `source-control-r02/server`, anchored four-test selector, count1/five-minute budget and one freshcapread_red_r22 profile. Its12 bound inputs, including the earlier static review, rehash correctly. Command exit1 and result duration24.618s are retained; “observation complete” describes recorder/inventory/cleanup completion, not expected-RED success.

Both stored source audits record5047 files, zero drift. The exact database before/after identity is `multica_pay241_20261116_s2_capread_red_r22`, both other connections0;853 ledger entries and663 index definitions are unchanged. All17 required fixture tables, including runs/run events/queues/artifacts and source/authority rows, are0 afterward. No fixture resource leak is recorded.

The database is not wholly empty: setup-generated config invalidations21→77 and knowledge invalidations18→39 remain. These retained counts are not erased, hidden or called a fixture leak. This review made no new/current DB/process probe.

## Next gate and ownership

Freeze a separate **test-only r03** that counts the real `collaboration_run.session_id` using `f.issue.SessionID`, or an equivalent exact session→issue join. Keep the zero-run assertion, valid31 summary, real read/writer/authority controls, five-second contexts, original inventory and cleanup unchanged. Preserve all original setup-blocked evidence and use a fresh coordinator-bound DB/source/command for the next RED.

An actual expected RED requires the two coexistence failure paths plus a passing real writer control and four passing authority leaves to reach their full assertions. No product repair is accepted from this setup-blocked observation.

Only this Markdown and its JSON companion were written. No source/test/parent/task/plan/spec or old report was changed. No Go/tests/DB/API/build/process/Git action was executed by the reviewer. R20 holder and full S2/business acceptance remain open.
