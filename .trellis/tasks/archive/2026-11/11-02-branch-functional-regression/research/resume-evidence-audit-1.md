# Supplemental evidence reconciliation — 2026-11-02

This is coordinator verification. Two native Trellis child dispatches failed with 429 before tool work; no independent review pass is claimed.

## Outcomes

| Run | Actual outcome |
| --- | --- |
| resume-handler-perf-1 | 2 PASS / 1 FAIL; exit 1. Closure P95 2,171ms exceeds frozen 2,000ms; disable and 12,000-issue triage passed |
| resume-handler-wire-live-1 | 2 PASS, race enabled: actual Go wire export and public GitHub skill import; exit 0 |
| resume-linux-conditional-1 | 7 PASS / 0 SKIP / 0 FAIL in Linux ARM64 containers; both real execution lanes exit 0 |
| resume-wire-schema-1 | All 7 actual Go response objects parsed through the candidate's existing TypeScript schemas and API parser; exit 0 |
| resume-linux-deb-install-1 | Actual ARM64 DEB upgrade and fresh/remove/reinstall/purge lanes passed; 87 assertions, both exit 0 |

Raw terminal Go events match the persisted outcomes and logged SHA-256 values. Every one of the 12 supplemented Go identities was an exact original skip. The derived view is **10,376 PASS / 1 FAIL / 6 SKIP**, with 10,491 passing subtests unchanged. It is not a new successful full-package run. research/go-current-summary.json remains the historical 82-package result; the new per-identity view is research/go-resume-current-summary.json. The extra seven schema checks do not change the original 13,082 TS result or 439 E2E inventory.

The six remaining skips have distinct causes:

- Missing PostgreSQL pg_bigm.
- Windows drive-root semantics and Windows ACP Unicode shell behavior need Windows.
- CLI unknown-field preservation and Webhook database-error injection remain explicit unimplemented test contracts.
- The current MCP catalog contains no public HTTP recipe; its lifecycle test deliberately skips.

Do not alter those preconditions, remove skips, modify assertions or introduce product fixes to manufacture a pass.

## Integrity and cleanup

The cleanup audit checked 4,614 prior task-file fingerprints. Only explicitly updated coordination documents changed at that point; original evidence and coverage.csv remained identical. The later implement.md update only records the now-executed DEB subgate. Final document validation checks this additional intended change separately.

Fresh read-only inspection showed zero connections in each new performance, wire/live and Linux database, no containers with either new task label, and no owned runner/schema-test processes. Databases and logs were retained. Evidence: evidence/resume-cleanup-1/run.json.

Remaining acceptance: the measured performance failure, real native Linux desktop, Windows and iOS sessions, installed macOS upgrade, other package/architecture combinations, Developer ID signing/notarization and commercial Provider/account/model calls. Commercial calls remain zero. No task archive, product fix, dependency change, commit, remote dispatch or publication occurred.

Changed materials: progress/report/defects/session prompt, task metadata, conditional matrix, the DEB subcheck in implement.md, resume-plan.md, new research summaries and new evidence directories. The only executable additions are private task verification drivers and preserved evidence copies. There was no product simplification or refactor. Full product lint/typecheck was not repeated; the actual package, Go conditional and TypeScript wire checks were run instead.
