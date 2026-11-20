# Independent output allocation review

Verdict: **PASS** for the current explicit output allocation after the URL-query fix. Review performed without editing the fixture source or touching historical state.

## Reviewed identities

- `e2e/fixtures/collaboration-runtime-output-allocation.mjs`: SHA256 `757a5f1ea5db962577a888142a80f88191c3fab4c96bbbeec248a7f75277e160`
- `e2e/fixtures/collaboration-runtime-output-allocation.node-test.mjs`: SHA256 `cb27b5781218a7ac5b8d96cef6e390e384fdd68a8af71ee85e04f74f37582d01`
- Current allocation record: SHA256 `58c7a36fc3627d189757afee2b85315187382f7e37da287fac1b2fdc91e9f5d9`
- Task: `.trellis/tasks/11-20-full-e2e-regression-20261120`
- State: `/Users/artisan/.multica/dev/envs/check-20261120013547-41191`
- Database: `multica_qa_20261120_output_18581`

## Findings and correction

The original guard checked `URL.hostname` and `URL.port`, but `pg.Client` honors connection query overrides. A credential-free, nonconnecting counterexample admitted a pinned loopback URL ending in `?host=example.invalid` while the effective `pg` host was `example.invalid`. A `?port=5432` probe likewise changed the effective port. The fixture owner added rejection of URL queries and fragments, plus regression cases for both explicit and historical admission.

Independent probes now reject `?host=example.invalid`, `?port=5432`, `?dbname=postgres` and `#historical` before any filesystem access. No network or database connection was made for these probes.

The historical branch retains the previously authorized exact database/storage predicate. Preserving that behavior is an explicit task constraint; adding unrelated historical hardening is outside this review. The current runner sets all four explicit allocation keys, so it never enters that branch. Partially configured explicit keys continue to fail closed.

The new Node tests now explicitly skip platforms without `process.getuid`; the explicit allocation guard remains fail-closed there. POSIX state and storage require UID ownership and mode 0700, and the regular allocation file requires mode 0600. Canonical path and ancestor symlink checks precede reading the allocation bytes.

## Actual runner checks

Loaded the exact environment recorded by the output lane using repository environment helpers. Instrumented filesystem operations to reject any `pay241-s3-output-20261119` access. Admission returned `mode=explicit`, read only the current `output-allocation.json`, and made zero historical accesses.

- All four explicit allocation keys are present and the record pin matches.
- The actual database URL has no query. Nonconnecting `pg.Client.connectionParameters` resolves host `127.0.0.1`, port `56259`, and database `multica_qa_20261120_output_18581`.
- State and upload directories are mode 0700, the record is mode 0600, and all are owned by UID 501.
- The retained API configuration matches the recorded deployment ID.
- A late live API probe found port 18581 already closed. No restart was attempted. This review binds to retained startup configuration; the owning lane remains responsible for browser execution and cleanup evidence.

`withOutputRuntime` invokes the guard before gate writes, account registration, storage inspection and daemon launch. The runner separately verifies owned API health and actual database identity before starting Playwright. Preparation verifies runtime API/WS origins and deployment configuration before writing the pinned allocation.

## Verification

- Guard tests: **20 passed, 0 failed, 0 skipped** on this POSIX host; see `output-guard-independent.log`.
- Targeted UI tests: **20 passed, 0 failed** across `builtin-squad-catalog.test.tsx` (11) and `use-squad-for-project-dialog.test.tsx` (9); see `security-unit.json` and `security-unit.log`.
- Fresh forced core/views/web lint and type checks: **9 tasks passed**, 0 errors and 27 existing views lint warnings; see `review-static.log`. Root E2E `.mjs` helpers are covered by the Node tests, not those workspace lint tasks.
- No unresolved blocking finding remains within the authorized fixture-guard scope.

The root owns the new Web/Desktop/API rebuild and the 14-case security/core targeted replay. The stage lane's earlier three passes bind to its retained production build, not to that future rebuild.
