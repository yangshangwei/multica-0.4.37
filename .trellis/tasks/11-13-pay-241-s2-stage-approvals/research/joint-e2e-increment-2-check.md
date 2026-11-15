# S2 joint E2E authoring increment2 — independent check

Completed: 2026-11-14T09:31:42.208724+00:00

**Authoring review complete after two local corrections. No open findings in this bounded review.** No scenario body was run; A05, A01–A12 and whole S2 remain NOT_RUN/NOT_ACCEPTED.

## Findings (fixed)

### E01 / P2

Source: `e2e/collaboration-stage-approvals.spec.ts:68`, `e2e/collaboration-stage-approvals.spec.ts:294`.

**Issue:** Scenario evidence recorded full gitInput and the native profile path, exposing absolute executable/bundle/repository/profile locations.

**Proof:** Source-extracted checks evaluated those actual evidence expressions against native-path sentinels; both failed before correction.

**Fix:** Project only actual commit heads, bundle digest/size/object format, marker and Git executable hashes. Record only the Desktop profile digest. Keep the original path variables, fixture directories and reproduction/storage behavior unchanged.

### E02 / P2

Source: `e2e/fixtures/collaboration-stage-approvals.ts:239`.

**Issue:** The new pre-trigger runtime-list request used TestApiClient.requestJSON, which supplies no AbortSignal/deadline, despite reusing S5 bounded freshness checks. Failed HTTP/transport reads also needed structured scenario evidence.

**Proof:** Extracted beforeTrigger executed only against fake dependencies; it failed the deadline test, then HTTP/transport observability tests before the final correction.

**Fix:** Use one explicit current-human GET to the same owner runtime path/execution workspace with the existing S5 10-second constant. Feed the unchanged runtimeAvailability validator on success, record safe HTTP/error facts on failure, throw without retry, and retain the existing outer cleanup. No global/shared-client/poller/provider timeout changed.

## Findings (not fixed)

None. No public wire/authorization behavior or unrelated framework code changed.

## Review evidence

- **Real Git preparation authoring:** Fixture creates two real local commits and a real bundle before adding dirty/untracked controls. Candidate2 names the bundle HEAD; provider marker digest and run preparation commit must match it. These Git operations were inspected, not executed by this reviewer. Source: `e2e/fixtures/collaboration-stage-approvals.ts:176`, `e2e/collaboration-stage-approvals.spec.ts:438`.
- **Provider compatibility and bounded output:** Existing --fixture-kind was present at recorded HEAD. New capability handshake occurs only with testReport:true; optional Git/failure release preserve default success/custom-provider behavior. Buffer is not embedded, so io.Copy cannot promote ReaderFrom around the limit. Dedicated regression and full race suite pass. Source: `e2e/fixtures/collaboration-host-runtime.ts:238`, `e2e/fixtures/collaboration-host-provider.go:27`, `e2e/fixtures/collaboration-host-provider.go:112`, `e2e/fixtures/collaboration-host-provider_test.go:146`.
- **Actual CLI/report/role flow:** Authored commands match actual CLI subject/submit/receipt syntax and unchanged UUID/run ACK fields. Bare-host absence remains distinct; failure/new run/keys/reports/binding and retained old failure are asserted. QA uses current real Q after stage reset, with a fresh reviewer signature; source-only null build never implies release execution. No actual CLI/API/report flow ran. Source: `e2e/fixtures/collaboration-host-provider.go:188`, `server/cmd/multica/cmd_collaboration_test_report.go:22`, `e2e/collaboration-stage-approvals.spec.ts:465`, `e2e/collaboration-stage-approvals.spec.ts:522`.
- **SQL and public account boundaries:** Counts join command/run session_id to collaboration_session.issue_id, matching actual schema. Email bridge matches exact account+password username and NULL email within verified task DB; it changes no membership/password/role row. Source membership remains public invite/accept/remove/rejoin. Source: `e2e/fixtures/collaboration-stage-approvals.ts:284`, `e2e/fixtures/collaboration-stage-approvals.ts:343`, `server/pkg/db/queries/collaboration.sql:179`, `server/pkg/db/queries/collaboration_runtime.sql:137`.
- **Freshness and teardown:** Managed runtime heartbeat verifies owner and keeps S5 15-second interval,10-second request bound and sticky errors. Cleanup closes owned daemons before joining heartbeats and restoring recorded gate booleans/monotonic epoch, then native/browser/SQL resources. Unsupported configured dependencies throw rather than silently pass/skip. Existing four portable cleanup tests passed. Source: `e2e/fixtures/collaboration-stage-approvals.ts:214`, `e2e/fixtures/collaboration-stage-approvals.ts:266`, `e2e/collaboration-stage-approvals.spec.ts:575`, `e2e/fixtures/collaboration-performance.ts:945`.

## Verification

- Five input source files and all17 historical command logs/exits match the author report; original increment1 and final-source snapshots remain unchanged.
- **Fake-provider Go race suite:**8 top-level /31 named PASS, zero FAIL/SKIP. Only the exact test executable served as the fake CLI, under a scrubbed environment.
- **Existing Host portability checks:**4 PASS.
- **Source-extracted fixture checks:**7 PASS, including path-free projection, unchanged human/workspace identity, one bounded availability request, visible non-retried failure, and optional/exact report handshake.
- **Targeted TypeScript:**pass. **Targeted ESLint:**pass, zero findings.
- **Playwright discovery only:**one scenario in one file. No browser or test body executed.

Initial privacy/deadline failures and subsequent missing-failure-evidence regressions remain in the private check directory. A later private harness comparison crossed VM object prototypes; normalizing those plain JSON values fixed the test harness, not product behavior. Final checks pass.

## Final source bindings

- `e2e/collaboration-stage-approvals.spec.ts` — `f8f0c32f7622ef4cb521bdfd6aec423906d7f955e3b9be7aea940f73d8883e73` (reviewer changed)
- `e2e/fixtures/collaboration-stage-approvals.ts` — `00405f3f5b6d978fe22ae54a35500a9481888a0cd4976aefe29df8a346dd3f27` (reviewer changed)
- `e2e/fixtures/collaboration-host-runtime.ts` — `88b357f0465cf6d4bb35036a683467f05173aaa64c282f24ad634df1baff3519` (unchanged)
- `e2e/fixtures/collaboration-host-provider.go` — `fc1095dc4fea11ad6a3e0a96647d99996687ba10937c00ebda53097f26fcd77f` (unchanged)
- `e2e/fixtures/collaboration-host-provider_test.go` — `4830a9dd03ea6e9212918418eb0654d62f77b0fbbff6e70df7bdc52d8ad9f4a3` (unchanged)

Only the scenario and stage-specific fixture changed. Shared Host runtime/provider defaults remain byte-identical to the submitted increment. Exact commands, source/log hashes, named Go terminals, dependency/context bindings and checker script are in `joint-e2e-increment-2-check.json`.

## Acceptance limits

The actual Git helper, prepared Host run, real run-local CLI subject/submit/receipt, failure settlement, UI selectors and complete cleanup remain unexecuted. Root must use the final post-2d API/CLI/production Web/matching Desktop and report-capable base Host provider on a fresh task database, with required mail/remote-refresh configuration controlled. The authoring checks do not establish real CI, deployment, payment, model execution or whole-phase acceptance.

No Core/Views/server/wire/spec/dependency/Git-index changes were made, and no actual API/DB/browser/daemon/model service ran. The existing fixture Git/profile storage behavior is preserved.
