# Private adapter-v4 independent review

Verdict: **changes required** for the Go fallback boundary. Existing positive evidence is valid, but one new unsafe ordinary-test path remains open. No reviewer execution was used to demonstrate it.

AV4-1 (high), `adapter-v4/check-bin/go:18`: when both `MULTICA_TASK_REAL_GO` and `MULTICA_TASK_PACKAGE_DB_MODE` are missing, v4 now resolves real Go to `/opt/homebrew/bin/go`. An ordinary `go test <package>` then takes the `mode !=1` forwarding branch before any explicit database/default-database/remote checks. The original adapter stopped on missing REAL_GO. This newly reachable path is broader than the intended stripped-environment sealer `go run`; DB-backed TestMain can consequently fall back to its business/default database.

Limit missing-REAL_GO fallback to non-test or compile-only calls, or otherwise refuse ordinary tests without complete package-isolation context. A fake negative contract must verify refusal before real Go or SQL; preserve the existing owned clone and recursion behavior. Do not run an actual default-database test to reproduce it.

The reviewer independently verified these positive records:

- Original wrappers remain byte-identical (Go `2de26dbb…07db`, pnpm `e74949d2…3a27`); new wrapper/source hashes are recorded in JSON.
- Six existing fake contracts passed, and their raw log/hash matches. The initial five-pass/one-fail fake-guard setup error and original source remain retained. These contracts do not test the newly identified missing-mode ordinary-test path.
- Recorded stripped-environment Go1.26.6 version forwarding succeeds under deny-network with no database context; that is an appropriate non-test fallback.
- Actual unchanged preparation Web and Desktop cases each passed once with zero skips/retries on product639, with original spec/helper bytes, raw execution/results/log hashes, task environment and controlled binary bindings. This proves the repaired sealer path; it does not cover unrestricted ordinary-test fallback.
- pnpm adds the default line/JSON reporter and output file, preserves explicit reporter selection, and invokes the actual browser through the candidate repository Agent CLI guard. No selector, worker, retry or timeout change was found; the fake guard is present and explicitly checked rather than bypassed.

The third complete run remains failed, native focus remains open/currently platform blocked, and the PG capacity incident remains separate. Re-review the corrected wrapper source/contract binding before using it for a new full check. The reviewer ran no tests, DB, browser, service, CLI, container, Provider or git/source mutation and wrote only the assigned review files.
