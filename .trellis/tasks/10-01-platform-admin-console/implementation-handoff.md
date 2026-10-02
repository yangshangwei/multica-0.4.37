# Platform administration implementation handoff

Updated 2026-10-02 after local S07 verification. Resume the existing work in
`/Volumes/artisan/code/2026/multica-platform-admin`, branch
`feat/platform-admin-console`. The original `multica-0.4.37` checkout contains
historical untracked planning copies and now points here. Do not redo S01–S06.

## Delivered implementation and verification

S01 `fa28e28a2`; S02/S03/S05 `a38158def`; S04 `8ba975947`; S06 `1c56f6cd6`.
Read the subsequent S07 commits in `git log` for rollback locks, instrumented
runtime transactions, private desktop profile writes and isolated check settings.

The canonical current evidence is [S07 verification](../10-01-platform-admin-acceptance/verification.md),
with [native macOS](../10-01-platform-admin-acceptance/native-acceptance.md),
[capacity](../10-01-platform-admin-acceptance/capacity-verification.md),
[backup/restore](../10-01-platform-admin-acceptance/backup-restore.md), and
[release checklist](../10-01-platform-admin-acceptance/release-checklist.md).

- Full make check passed: 9,528 TS tests, full Go race/vet, production Web,
  131 browser passes and 33 specialized-environment skips.
- Six password-mode production browser specs passed separately, without skips
  or retries and with independent synthetic administrator accounts.
- Current-source macOS Main/preload/file renderer/Go CLI passed real multi-window,
  restart, account drain, revocation, reconnect and lost cancellation receipt flows.
  No real provider/model account ran; six daemon/two fixture-provider PIDs exited.
- Actual dump/restore preserved all 20 checked table fingerprints.
- Capacity: 660.85 seconds, 1,000 installations/3,000 runtimes/one million history/
  ten observers; all pagination P95 ≤800 ms, worst 767.8 ms with 17 samples.
  Observer HTTP errors zero. Separate browser rendering met two seconds.
- Capacity limits: loaded 31-day overview P95 2.808 s; browser uses 24 hours and
  ran outside the load. 1,000 heartbeat ACKs exceeded the harness ten-second
  threshold. All runtime identities had successful measured ACKs by 28.234 s;
  late ACK accounting/native fallback and margin require further measurement.
- Five-observer revocation/restoration and an eight-second isolated PG pause
  recovered correctly; local tests do not establish production failover.
- Knip has the same 11 pre-existing findings; no new finding was introduced.

Independent reviews found no remaining P1/P2 issue in corrective code. No SQL
performance change was made: same-snapshot EXPLAIN showed only 8.6% benefit from
an extra current-state filter, with the large finished/usage sort unchanged.
Preserve the measured baseline and do not claim that hypothetical change fixes it.

## Remaining work and status

S07 and the parent remain `in_progress` with `current_phase=local_acceptance_reviewed`.
They are not marked released. No push, merge, release tag or deployment occurred.

1. Improve/measure loaded 31-day aggregation and reconnect ACK accounting/margin
   on the intended environment; preserve authorization, unknown usage and windows.
2. Measure browser rendering during the fleet workload and storage-cold/extended
   outage behavior where isolated infrastructure makes that meaningful.
3. Obtain native Windows runtime/ACL, signed installation, old-binary upgrade and
   clean reinstall evidence. The earlier Windows-environment question already
   exists; do not repeat it or substitute macOS source-build evidence.
4. Complete actual deployment maintenance/backup/key/storage compatibility and
   production release gates only within explicitly authorized deployment scope.

## Local evidence and recovery

Raw logs/harnesses are under `.omx/reports/platform-admin/`; nonsecret summaries
are also preserved in S07 `evidence/`. Private fixture env copies live under
`.omx/reports/platform-admin/s07-resume/private-env/` (0700 directory/0600 files).
Never print or commit them. `/tmp/platform-admin-resume-*.env` can disappear on
reboot; restore only the needed private fixture copy, not a real user credential.

The root password environment's API/Web were stopped after verification; its DB
`multica_platform_admin_resume_password_20261002` and private registry
`.omx/reports/platform-admin/s07-resume/runtime/` remain for inspection.
The task-only Go DB `multica_platform_admin_resume_go_20261002` is separate.
Serialize handler TestMain instances per DB. Never destroy shared PostgreSQL or
unrelated databases to recreate a fixture.

Capacity's run is `s07-20261001212929-d12b32`, under the ignored S07 capacity
folder. Its manifest contains exact ownership labels and current private ports;
restarts can reassign Docker ports, so verify them before reuse. Fixtures and
backup artifacts are preserved. Do not reseed a nonempty database or reuse old
process IDs. The report records cleanup and the source snapshot hash.

The suite name “classic” is descriptive; the actual server setting is `legacy`.
`check.sh` now sets `legacy` and disables managed/platform features only in its
disposable env copy. Password administration still needs a separate real
production-Web run. Do not weaken login/reauth limits; use distinct test actors.

Last migration is 511. New indexes require standalone CONCURRENTLY migrations;
no foreign keys/cascades. SQL sources precede sqlc generation. Preserve the
separate execution timestamp, state_version and claim_generation fences.
