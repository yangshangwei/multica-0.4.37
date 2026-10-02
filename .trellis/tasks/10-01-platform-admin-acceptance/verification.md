# S07 integration acceptance

Latest continuation (2026-10-02): [capacity follow-up](capacity-followup.md) records optimized SQL, explicit PostgreSQL shared memory, a 660-second rerun with zero observer HTTP errors, passing loaded-browser/pagination targets, raw ACK recovery in 15.779 seconds, and remaining ten-second ACK threshold failures. The baseline below is retained as historical evidence.

Updated 2026-10-02. S01–S06 baseline: `1c56f6cd6`, plus the reviewed S07 corrections described below. Local security, browser, native macOS and restore verification has passed. Capacity sampling and separate recovery probes are complete, with the limits below. Windows/distribution and production release gates remain open; the whole parent task is not marked complete or released.

## Corrective changes and review

- All 43 guarded down migrations now acquire ordered `ACCESS EXCLUSIVE NOWAIT` locks before inspecting retained evidence and keep the checks and DDL in one `DO` block. Four real concurrent-first-write cases reproduced data loss before the fix. The complete migrations race suite passes.
- Managed runtime reads use the existing instrumented `RuntimeLookup`, preserving request context and transaction queries. Two old test fixtures now carry the actual claim/admission snapshot; additional assertions prove a superseded delivery cannot cancel a newer one. Organization-owned alert and detector history stays retained after workspace deletion.
- Native Main maps `darwin` to protocol `macos` and atomically writes private `0600` daemon profiles. The previous writer created `0644` files rejected by Go. New/existing permissions and open-reader tests failed before the fix and pass afterward.
- The fullcheck task env explicitly uses `legacy` and disables managed-installation/platform-admin flags, preserving the source password deployment. The missing override was reproduced with the script fixture and corrected. Independent review caught the incompatible managed flag; the final regression covers all three settings and source preservation.
- Operator guides use the real `super_admin` role and audited recovery arguments. The Chinese embedded docs bundle is regenerated.

Independent review found no remaining P1/P2 findings in these corrections. No dependency was added, no original workspace authorization was removed, and no production deployment occurred.

## Full local verification

`PATH=/opt/homebrew/opt/libpq/bin:$PATH make check ENV_FILE=/tmp/platform-admin-resume-classic.env` completed with exit 0.

| Check | Result |
| --- | --- |
| Lint and TypeScript checking | Passed |
| TypeScript tests | 815 files, 9,528 tests passed |
| Go tests | Full repository race suites passed, including guarded agent subprocess tests |
| Go static analysis | `go vet` passed |
| Launcher/script regressions | Passed |
| Production Web build | Passed |
| Legacy-mode browser suite | 131 passed, 33 specialized-environment tests skipped, zero failures/retries |
| Final check-env correction | `check.test.sh` red/green, final exit 0; `bash -n` passed |
| Knip | Exit 1 on the existing 9 unused files, 1 dependency and 1 devDependency; no new finding |

The final shell correction followed fullcheck and produces the same explicit `legacy/false/false` settings used by the passing run. Its orchestration regression was rerun; the full product suite was not needlessly repeated for that identical environment output.

Raw log: `.omx/reports/platform-admin/s07-resume/make-check-legacy.log`. Launcher provenance: `/Users/artisan/.multica/dev/envs/check-20261001235137-90166/verification.running.json`. Log/provenance hashes and counts are captured in `.omx/reports/platform-admin/s07-resume/verification-summary.json`.

Earlier failed attempts are retained and excluded from passes: inherited password mode in a whole-package test, analytics-disabled config-test mismatch, and the invalid literal `MULTICA_AUTH_MODE=classic`. The server accepts `legacy` or `password`; “classic” is only a descriptive suite label.

## Password-mode production Web

All six actual specs passed independently, with zero skips, retries or flaky results. Each uses a distinct synthetic administrator to preserve the real account rate limit.

| Scope | Result | Playwright duration |
| --- | --- | --- |
| Access, zero workspace, cookie, revocation | 1 passed | 18.511 s |
| Installation read model | 1 passed | 2.678 s |
| Execution metadata and content boundaries | 1 passed | 3.331 s |
| Admission/cancellation controls and receipts | 1 passed | 3.237 s |
| Accounts and forced recovery | 1 passed | 4.740 s |
| Overview, health, audit and alerts | 1 passed | 4.931 s |

API `18393`/PID `27576`; Web `13313`/PID `27823`, production build `mJM8Cycc2jLu0e5ZYRDUw`; both source IDs `a2c1a96af0547272d6fd1d9d1cb31ed3f8dab3cfd05626e01aac2318e83a4edc`. Database: `multica_platform_admin_resume_password_20261002`. The API uses the local Go launcher; the Web uses production build/start. No actual employee account was used.

Raw per-suite Playwright JSON and `.stderr.log` files are under `.omx/reports/platform-admin/s07-resume/password-*`; `password-summary.json` contains the six checked results. The initial wrapper could not parse dotenv's informational line preceding JSON and exited 1 despite all six child exits being 0; parsing the retained JSON payloads confirmed all counts/errors and repaired the summary. No test result was changed or failure rerun into a pass.

## Native macOS and restoration

[Native acceptance](native-acceptance.md) records the real Main/preload/file renderer and source-built CLI, including multi-window/restart, account drain, password revocation, two WS reconnects, durable cancellation receipts across daemon restart, a lost successful ACK and duplicate replay. Native run 5 passed in 33.929 s; all six daemon and two fixture provider PIDs exited. Desktop 914 tests, typecheck and lint also passed.

[Backup and restore](backup-restore.md) records a real custom-format dump and restore into a new isolated database. All 20 table fingerprints matched, including credential versions, binding epochs, stopped admission and pending operations/audit. This is a synthetic local database rehearsal, not a production recovery objective.

## Acceptance mapping

| AC | Concrete evidence | Status |
| --- | --- | --- |
| AC01 | S01 real router credential matrix and fresh cookie/zero-workspace browser flow | Local pass |
| AC02 | PAT-to-JWT denial, JWT-to-PAT/PAT renewal, role/version races; full Go race | Local pass |
| AC03 | S05 target-only credential/WS revocation, forced recovery browser; native version rotation | Local pass |
| AC04 | S02 Ed25519 identity/binding tests; native two-workspace/shared installation/restarts | Local pass; distribution upgrade remains open |
| AC05 | S02 separate client/daemon/engine observations; native reconnect and source failure tests | Local pass; bounded database probe also passed |
| AC06 | S03 actual SQL keysets/filter/content matrix and fresh execution browser spec | Local pass |
| AC07 | S04 claim-path fences and real native process cancellation/outbox | Local pass |
| AC08 | S04 original-key/follower/restart/late-ACK races; native lost ACK and duplicate success | Local pass |
| AC09 | S06 detector/episode/recovery races and fresh observability browser flow | Local pass |
| AC10 | S03/S06 window/timezone/unknown usage/private projection tests and browser drilldowns | Local pass |
| AC11 | S01/S05 last-admin races, reauth/audit rollback and full Go race | Local pass |
| AC12 | Legacy wire tests, guarded rollback, macOS native restart and local restore | Partial: Windows/old distribution upgrade unverified |
| AC13 | Isolated 1,000-installation/1,000,300-execution/10-observer harness | Measured with pagination pass and reconnect/loaded-overview limits |

## Remaining gates

[Capacity verification](capacity-verification.md) records the uninterrupted 660.85-second run: all pagination groups P95 ≤800 ms (worst 767.8 ms, 17 samples), 2,242 observer HTTP requests with no errors, and separate browser P95 ≤2 seconds. The loaded 31-day overview P95 was 2,808.4 ms; browser defaults use a 24-hour window, so the timings also differ in query scope. All 1,000 sockets returned by 3.173 s and all 3,000 runtime identities had a successful measured ACK by 28.234 s, while 1,000 ACKs exceeded the ten-second harness threshold. Another 32 recorded heartbeat errors came from deliberate disconnect/teardown. Preserve those failures and the distinction between connection recovery, ACK receipt and native fallback.

Five-observer role revocation/restoration and an eight-second owned-database pause/recovery passed. Cold storage, loaded-browser timing, extended failure and native HTTP fallback were not measured. Read-only EXPLAIN found a modest 8.6% gain from a single filter but unchanged large aggregation/sort work; no speculative SQL change or broad WebSocket authorization rewrite was added. Loaded overview performance and reconnect timing/accounting remain explicit follow-up items.

Windows runtime/ACL, signed installation, old-binary upgrade, clean reinstall and real keychain/protocol registration remain unverified. The existing Windows-environment question from the earlier session is not repeated. Production maintenance, deployment and production recovery are not authorized or performed. Keep S07 and the parent open until required external acceptance is supplied or the user explicitly changes that scope.
