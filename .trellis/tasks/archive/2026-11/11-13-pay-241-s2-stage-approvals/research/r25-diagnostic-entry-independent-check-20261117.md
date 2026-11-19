# R25 independent diagnostic entry check

**Verdict: READY_R25_DIAGNOSTIC_ENTRY_SCOPED. Blocking findings: 0.** The exact private diagnostic source, matching compile-only harness and prepared runtime are ready for main's single API admission. The accepted draft SHA256 is `41221895acf5b97a8121f4f7c05803c850960e19612a69d7a3eea2ce597f463d` and all **39** direct references match. This does not accept full S2/A01–A12 or the parent24.

## Findings (fixed)

- File: `runtime/collect-runtime-database-r02.mjs`, `run-original-four-scenarios-r02.mjs`, `playwright-four-scenarios-r02.config.mjs`.
- Issue: prepared r01 derivatives pointed to a nonexistent R25 parent instead of the actual R24 parent and intentionally reused frontend.
- Fix: main created corrected r02 variants. The collector changes only its application label from R24; the runner changes only the fresh runtime path and corrected config filename; the config changes only the fresh runtime path. All original guards, assertions, deadlines and queries remain exact. Broken originals and preparation evidence are preserved. Repair manifest SHA256 `9bf9cdec1cea8dc6d1057c985d6aedb7e19a49e06391ac44b303ef272ef623f8`.

- File: private `independent-check/verify-source-build-r02.py`.
- Issue: the first reviewer reference classifier compared historical RED source origin with the current GREEN file.
- Fix: check the explicitly declared immutable before snapshot and exact AST declaration. The failed first checker and assertion receipt remain intact.

- File: private `independent-check/verify-runtime-preparation-r02.py`.
- Issue: the first reviewer compared canonical entries containing `size` with worker entries containing path/hash/mode as identical objects.
- Fix: compare the exact common fields and separately verify all5047 canonical file sizes. The failed first checker and assertion receipt remain intact.

## Findings (not fixed)

- Original R24 remains **3 PASS /1 FAIL /0 NOT_RUN, retry0**. Its successful submission200 precedes an actual candidate GET409. The original409 body, SQLSTATE and holder remain unknown. The existing preview route is correct; no product or waiter change was made or authorized by this review.
- Main still owns actual API/admin/platform preflight, one fresh complete four-case journey, final after/stop evidence and full S2/A01–A12. Static capture-policy readiness and four-case discovery do not close those gates.

## Verification

- Lint: **PASS recorded evidence** — standalone `go vet` exit0 and empty owned-file `gofmt -d`; independently rehashed. No new lint command was run under this assignment.
- TypeCheck: **PASS recorded compile-only evidence** — exactly one permitted matching Go harness build, exit0, no TestMain/DB/API/binary execution. TypeScript checks were not rerun; frontend bytes are unchanged.
- Tests: **PASS recorded capture-policy evidence** —3 top-level/68 named/66 leaf PASS, zero failures/skips,61 boundary cases and four exact AST bindings plus URL.Path coverage. RED has the same68 identities:58 PASS/10 FAIL. Corrected four-case discovery passes with zero test bodies. No suite or browser was rerun.

All5047 current private source files were independently hashed. Exactly `pay241DiagnosticPath` in one test-only file changes;5046 files, path set, modes and every byte outside the function remain exact. It admits only canonical nonzero UUID GET candidate reads with seven URL segments. R1/R2/R4/T4 and previewPOST are excluded. The4096-record cap,1MiB response Tee, raw base64, whole-stream hash/count/completeness,0600 exclusive sink/0700 parent and existing pgx/request/backend/error correlation remain unchanged. Production SQL, locks, authority, helpers and5/20/480-second limits remain exact. No production spec update is needed.

The actual diagnostic binary is `/Volumes/artisan/.multica-acceptance/pay241-r24-continuation-20261117-32uud2uo/r25-m1-candidate-diagnostic-r01/build/stage-router.test`, SHA256 `2dcecad3096983d825ad3c3274969097003d49e1a36d0cf22c3674729ba4b177`,118534498 bytes,0755. Its recorded Go1.27.0 darwin/arm64 metadata retains the original `86fea651…` ldflags and separately binds the diagnostic delta; there is no currentHEAD claim. Three existing Go products remain unchanged.

Actual artifact preparation is **239 paths =231 preserved paths +8 fresh empty0700 slots**, with all old inodes/UIDs/modes/sizes/hashes intact and staging/output roots0500/0700. API64922 is free; new socket/ready/logs/diagnostic/four-case outputs are absent. Accepted Web86953/64925 and renderer86966/64926 remain present; the stale R23 socket is preserved. Five model keys are explicitly blank, and existing synthetic administrator/auth/constructor/root configuration is unchanged.

Main's fresh recorded DB snapshot equals every one of the eight captured R24 after-stop history sets: gatefalse/true/false at epoch44,39 settled Host runs,26 complete artifacts, zero owned connections/unsettled/leases/unreleased/incomplete. All28 ordinary queue rows and the one unresolved historical intent remain unchanged. This reviewer issued no SQL. The fresh consuming audit binds3359 frontend inputs/1961 links/1676 outputs and5047 Go inputs/four products, drift0. Current frontend file/link manifests equal the accepted R24 manifests; the historical full source graph was not rerun.

Source/build check SHA256 `1fa7cb39ddb7cf442c42777fd28742d7d48dd260282e82828717be4f827042b5`. Runtime check SHA256 `c488d65ac02af9225169194cef929d7c4c15da0e5043dd4d05607a637b36879c`. Final reference check SHA256 `8933ba4149e3e9ba3e512517d581797ed2bd517f78b76f1e626d7db9bac01052`. Detailed [JSON](r25-diagnostic-entry-independent-check-20261117.json) SHA256 `4c2b1f43905cd4d91ca47a4fccb7a91f98051748d45b3381d4630bfd5f0a5d56`. This reviewer wrote only the two new research reports and private independent-check support, with zero source/configuration/history changes, SQL/API/browser/signals/build/test-suite/Git operations.
