# S06 integration verification

Worktree `/Volumes/artisan/code/2026/multica-platform-admin`, baseline `8ba975947`. Updated2026-10-02. Backend, UI, bounded review/checks and production browser/visual acceptance are complete. S07 remains outstanding.

## Implemented

- Overview, workspace metadata, service health, audit trail and readonly deployment settings, with six implemented administration destinations and grouped section links. The original access/scope information remains available in a disclosure on the overview.
- Finished-window outcome/usage metrics use completed_at; zero denominator is no-samples, absent usage is unknown, and no token estimate is presented as a bill. Finished drilldowns use time_basis=finished; explicit nonterminal state_scope=current admits all-age active work without creating an unbounded historical feed. History counts, links and timezone use the same returned window.
- Queue observations distinguish exact new transitions from conservative migration observations. Migration500 stamps existing queued rows at the observation time; it does not invent historical queue entry. Exact percentile samples exclude observation/unknown clocks.
- Fixed alert rules for queued age, per-attempt failure and proved installation loss with in-flight work. Condition state is independent of handling state; historical failure dedupe persists after closure. Detector leases, bounded200-row batches, fixed cycle horizons, repeated scans and nonblocking failure hints recover late commits without infinite tail starvation. Missing source evidence is unknown; source failures pause both opening and recovery. Confirmed global task deletion can resolve a queue condition without claiming execution success.
- Acknowledge/assign/close use current authority, expected version, original-key operations and atomic audit. Only eligible active complete-password superadmins can be assigned. Failed execution closure requires a controlled disposition; linked successful retry must be a verified same-organization descendant. Failed conflict receipts support safe client recovery.
- New audit writes capture the actor display name; historical missing snapshots remain unknown. Audit metadata projects closed safe values and omits arbitrary diagnostics/configuration. Settings expose configured retention values only and explicitly disable automatic deletion.
- Health samples actual DB and heartbeat-read availability, task sweep query cycles, cancellation reconciliation, and persisted detector progress. Never-run workers remain unknown and stale progress is explicit. A first-cycle detector failure cannot be hidden by another unobserved rule.

## Schema and checks

Migrations500–511 add queue-clock/actor-snapshot fields, alert/lease state and independent concurrent indexes. The queue-clock migration regression proves old observed/new exact timestamps, re-entry timing and rollback refusal. Startup verifies the new indexes and observation trigger. SQL is generated from query sources only.

- Alert service17 regressions: race23.827s; combined alert/read/S03 handler race18.681s. Includes fixed horizons, below-cursor late commits, same-fingerprint races, failure dedupe, source/lease/checkpoint failures, actor/assignee checks, failed receipts and deletion vs unscoped evidence.
- Read backend final peer repairs: race8.758s, vet and diff check. Includes all-age current drilldowns, finished-window counts, inactive alert first-seen windows, unknown metrics, audit code projection and DB-backed liveness consistency.
- Parent core21 files/126 tests and views16 files/103 tests passed; later shell/overview corrections passed8 tests. Core/views/Web typechecks, scoped lint, Go vet and migration package tests passed. Knip remains the same11 original-main findings, with no new findings.
- Parent private-schema startup/worker checks passed4.351s; queue-clock migration test passed0.605s. Existing task sweeping cadence/order and business predicates are retained; health observes query-cycle results.
- Handler tests are serialized against the shared TestMain database fixture. All Go tests use the agent-CLI guard and `source /tmp/platform-admin-go-tests.env`, pointing to the task-owned Go database. No real provider or model account was used.

## Review corrections

Reciprocal backend review closed moving scan horizons, deleted-task recovery, inactive alert count/window mismatches, historical resolution/status code projection, and uncovered live-state quality. UI review also separated historical failure facts from live conditions, preserved windows/timezones through drilldowns/pagination, and reused S04 request recovery for alert operations.

Detailed evidence: [alerts-verification.md](alerts-verification.md), [frontend-verification.md](frontend-verification.md), `.omx/reports/platform-admin/client-s06-read-handoff.md`. Parent logs are retained in `.omx/reports/platform-admin/s06-integration/`.

## Production acceptance

The final source-stable API/Web snapshot used source fingerprint `feece9dd97892358868da74aeb67255d424420b4b964b29f93ec4aac0a3e8509`, embedded baseline `8ba975947`, Web production build `qqJz8iWP5TzQKonMU7MFF`, API PID79272 and Web PID79634 at ports18393/13313. The API runs the normal development launcher with a production-built Web; this is local acceptance, not a deployed release.

`platform-admin-observability.spec.ts` and the updated S01 `platform-admin.spec.ts` passed together: **2/2, 22.682s, zero skips/retries/failures**, on2026-10-02. Results/provenance: `.omx/reports/platform-admin/s06-production-fixed/`. This covers finished/current drilldowns, overview semantics, old active alerts, lost-response original-key recovery, disposition closure, observer write denial, all section navigation, audit redaction, and390px mobile overflow. Browser page errors were empty. Final fixture cleanup query found zero S06 password accounts and zero S06 workspaces.

The first production run exposed a real boundary bug: alert audit snapshots can contain `resolution_code: null`. A red/green core schema regression now accepts null while preserving the closed code vocabulary. Its11-test suite and core/views typechecks passed before the final rebuild. The original failing report remains in `s06-production/`.

Desktop overview/audit and mobile alert screenshots were inspected. The final visual verdict is pass93/100, persisted at `.omx/state/platform-admin-observability/ralph-progress.json`; primary mobile navigation intentionally scrolls within the shell without document overflow.

## S07 handoff

Native installed-app/Windows, full classic regression, capacity and rollout remain S07. A Windows environment question is pending. S07 review has identified a concurrent first-insert versus rollback emptiness-check race in new down migrations; this is a release blocker to correct with protective locks and concurrent regression evidence before rollout acceptance. No production deployment or rollback has occurred.
