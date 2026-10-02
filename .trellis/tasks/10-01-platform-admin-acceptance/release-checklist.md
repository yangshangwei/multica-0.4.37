# Platform administration release checklist

This section records rollback readiness only. It does not mark the full S07
acceptance, native clients, capacity, or production deployment complete.

## Rollback safety — S07-R01

- [x] Reproduce the concurrent first-write race with actual down SQL in private
  schemas: audit, legacy revocation marker, enrollment challenge, queue clock.
- [x] Lock all checked and DDL-target tables before retained-data checks, using
  deterministic order and fail-fast `ACCESS EXCLUSIVE NOWAIT`.
- [x] Keep each check and protected DDL in one `DO` block; retain the existing
  populated-data refusal and prove empty-maintenance rollback still works.
- [x] Cover every guarded down file from 465–511 (43 files); no up migration or
  migration number changed.
- [x] Document that disabling feature flags is not maintenance mode and the
  migration advisory lock does not stop application writers.
- [x] Complete independent review of the corrective diff: no P1/P2 findings in
  rollback locks, transaction-scoped runtime lookups, retained alerts or the
  native private-profile write correction.
- [x] Complete final shared verification: fullcheck passed, plus six password-mode
  browser specs, native macOS and final shell regression. See verification.md.
- [ ] In the actual approved deployment maintenance window, record a backup,
  binary/schema compatibility, writer shutdown and connection drain, and
  schema/index/trigger checks before restoring service. This has not run here.
- [x] Run a local synthetic database dump/restore and verify credential versions,
  roles, installation epochs, admission policy, pending operations and audit.
  All 20 table fingerprints matched; see [backup-restore.md](backup-restore.md).
- [ ] Validate the actual production backup plan, including deployment secrets,
  storage and recovery objectives; the local database drill does not cover them.

Treat `55P03` as a live-connection refusal and `P0001` as retained evidence.
Neither permits forced data removal. Keep maintenance active after a partial
multi-file failure and inspect actual schema plus migration bookkeeping before
choosing a compatible recovery or forward fix.

Implementation evidence and the original two-connection schedule are retained
under `.omx/reports/platform-admin/s07-review/`; final check results must be
included in the S07 verification report before a release claim.

## Remaining acceptance before release

- [x] Record 1,000-installation, one-million-history and ten-observer measurements.
- [x] Record controlled role revocation and owned-database recovery evidence.
- [x] Optimize overview and measure loaded browser rendering: overview API P95
  2.060 s, browser overview P95 1.901 s, zero observer HTTP errors with explicit
  256 MiB PostgreSQL shared memory. See capacity-followup.md for changed environment.
- [ ] Address or explicitly accept the 1,000 ten-second ACK timeouts and native
  fleet fallback/margin limits; raw ACK receipt reached all runtimes in 15.779 s.
- [x] Verify selected native Windows managed-runtime/ACL tests and unsigned x64
  old-version installer upgrade, uninstall and reinstall; see windows-acceptance.md.
- [ ] Verify signed Windows installation, GUI/backend-connected workflows, existing
  user-state retention and automatic-update-feed behavior.
- [ ] Record reference-environment/cold-storage and longer failure recovery.

The measured local paging target passed; this checklist intentionally does not
mark the entire S07 or production release complete.

## Native fleet continuation — 2026-10-02

- [x] Validate three real runtimes on one daemon under the thousand-client background workload; all 12 synthetic tasks and per-runtime HTTP heartbeat fallback independently verified.
- [x] Fix measurement proxy control-frame/cancellation defects and preserve invalidated prior runs.
- [x] Add real managed lost-response recovery/admission regression; do not weaken the duplicate-execution window.
- [ ] Resolve or explicitly accept ~109.9-second reconnect task starts (original 90-second observation failed), two unexplained extra native reconnects, and final paging P95 834.2 ms / synthetic ten-second ACK failures.

See native-load-recovery.md; the successful recovery matrix is separate from latency, capacity and distribution release acceptance.

## Connection and contention follow-up

- [x] Reproduce server read timeouts with classified diagnostics; correct earlier control-frame-only interpretation.
- [x] Fix scoped RPC detachment and reduce same-call managed authorization queries without caching authority between frames.
- [x] Verify full local rerun: no extra native closes or ten-second ACK timeouts, reconnect starts 39.355–39.548 s, every paging group P95 ≤289.3 ms, loaded Web P95 ≤901.9 ms. See connection-followup.md.

The older local latency checkboxes above describe the preceding runs and are superseded by this evidence. Distribution/Windows and external deployment/reference checks are not closed.

## Windows candidate continuation — 2026-10-02

- [x] CI run 36996436086 accepted unsigned candidate 0.5.2-rc.11.1 from 0fb5f5e6b.
- [x] Native Desktop lock tests (19), named Go tests (16), concurrent identity stress (20 passes).
- [x] Previous 0.5.1 install → candidate upgrade → uninstall → reinstall → final uninstall.
- [ ] Signed distribution, offline trust, GUI/backend integration and retained real-user state.

See windows-acceptance.md for provenance and limitations. This closes the bounded
Windows candidate CI task, not the remaining S07 release gates.


## Installed Windows business and update continuation — 2026-10-02

- [x] Real installed UI login/enrollment/workspace binding and actual backend task completion, administrative API cancellation and reconnect.
- [x] Candidate-to-candidate retained session, configuration, identity, workspace/completed history and same queued task executing exactly once.
- [x] Fix runtime-created protocol residue on uninstall; native test preserves another handler.
- [x] Production installed updater manual check, full download, quit/install and retained settings/test browser storage.
- [x] Isolated Nginx transport/restart rehearsal and native CI run 37016263509.
- [ ] Trusted signing/offline certificates and an actual approved preproduction environment.
- [ ] Scheduled update trigger, differential download, and public v0.5.1 password/profile migration (not claimed by the candidate baseline).

This supersedes prior open GUI/backend/fixture-retention items within the stated
scope. Admin-page UI, real model execution, arbitrary production profiles and
external reference/production gates are not implied. See windows-business-release.md.
One prior lock-test barrier failure remains unrooted; current full suite plus
three mandatory repetitions passed, and child diagnostics now preserve errors.
