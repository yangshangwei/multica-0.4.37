# Windows business, retained-state and release acceptance

**Native business, retained-state upgrade, installer cleanup and installed-client
update passed on 2026-10-02.** Trusted signing, offline certificate trust and an
actual preproduction deployment remain open. S07 is not release-complete.

## Provenance

- Run: https://github.com/yangshangwei/multica-0.4.37/actions/runs/37016263509
- Tested source: `6c293e136fd6e63afc56fd725427d3acde44b583`.
- Candidate: `0.5.3-rc.17.1`, Windows x64, Authenticode `NotSigned`.
- Installer SHA256: `6d172bc4d875421ff3a7df46389ac8c3d9a92a9f4506028213d2f8a38438444f`.
- Candidate artifact: `11231380422`; downloaded ZIP digest verified against GitHub:
  `9ca8a30269195f41ce56bfccb69db499e7e89bdfa72496d3b96d3dfa5865f534`.
- Baseline: exact accepted `0.5.2-rc.11.1` artifact from run `36996436086`,
  SHA256 `f0b5a5eeded51fa7d1336bd2b46975a0c9ddc6d2292cebba457fd0b6e090fd47`.

Published v0.5.1 predates password/managed implementation. Its installer lifecycle
was verified separately; this run proves feature-capable candidate-to-candidate
continuity, not public v0.5.1 password migration. Later documentation commits do
not change the candidate's tested source SHA. No main merge, release tag, public
release or production deployment occurred.

Downloaded installer SHA256 independently matches the native lifecycle report.
See `evidence/windows-business-summary.json` for the compact verified summary.

## Executed native scenarios

| Lane | Actual evidence |
| --- | --- |
| Installed business | Real Electron main/preload/renderer and bundled CLI, actual UI server setup/password login, installation enrollment and workspace binding, administrator API installation observation, successful task/result, administrator cancellation confirmed with provider exit, WebSocket reconnect retaining PID and identity. |
| Retained-state upgrade | Same browser profile reopens authenticated without login; configuration, installation ID, namespace, key hash, workspace and completed task are retained. An offline queued task retains its ID, executes after upgrade and has exactly one fixture-provider start. |
| Installer cleanup | Old install, candidate upgrade, uninstall, reinstall and final uninstall all pass. Runtime-created owned protocol is removed; a fixture handler reassigned to another path is preserved. |
| Installed update | Actual production updater check reports the candidate available, downloads it, invokes quit/install and replaces the installed application. Relaunched app reports 0.5.3-rc.17.1; update settings and a test localStorage marker remain. |

Six baseline and nine candidate business case events passed. Main Windows Desktop
lock suite: 19/19. Three additional required repetitions each selected the one
concurrent publication case and passed (four unrelated cases filtered in each).
Sixteen named native Go tests and twenty concurrent identity repetitions passed;
raw Go event logs contain no failures or skips.

Local final Desktop suite: 84 files / 951 tests passed. Node typecheck, focused
ESLint, fixture/proxy/update helper tests, and package/workflow checks passed.
Windows compiled and executed the real server, migrations, installed apps and
native test provider. The backend health PID and exact source commit matched.

## Update-service rehearsal

The previous accepted installer was also published with existing tooling to
isolated local storage and served by a separate loopback Nginx container. Complete
SHA512, HEAD 200, Range 206, metadata no-cache and restart persistence passed.
The owned container/network was removed and artifacts retained. See
`evidence/update-feed-rehearsal.json` and ignored
`.omx/reports/platform-admin/release-rehearsal/`.

The Windows updater run used manual check → automatic download → explicit
quit/install. Its old blockmap was absent, so electron-updater fell back to a
successful **full download**. Differential updating and the scheduled background
check trigger are not claimed. The browser marker is a test storage value, not
real browser history. Business history retention is established by the separate
business harness. The automatic relaunch is followed by an owned-process stop and
a Playwright relaunch with the same explicit test profile for inspection.

## Failures preserved and fixes

| Run | Result / correction |
| --- | --- |
| 37007494697 | Cancelled before acceptance to add backend PID/source provenance. |
| 37007750028 | Backend was ready in preparation but unavailable in later test step; start backend and business test in the same Windows shell lifetime. |
| 37009670617 | Login/enrollment passed, task timed out. Production correctly filters inherited MULTICA_*; use WINDOWS_BUSINESS_STATE_DIR for fixture-only context and test the filtering contract. |
| 37012593077 | Business and retained-state cases passed, uninstall left runtime protocol registration. Add a narrowly ownership-checked NSIS customUnInstall hook; preserve other handlers and skip update uninstall. |
| 37015148138 | One concurrent lock-test child did not reach its barrier; prior helper hid the child failure. Preserve this inconclusive failure, surface child errors, and add three required repetitions. The original cause is still unknown; no claim that a production lock defect was fixed. |
| 37016263509 | All selected workflow stages passed, including protocol ownership checks and installed-client update. |

Production change: `apps/desktop/build/installer.nsh` plus its
`electron-builder.yml` inclusion. Other changes are the Windows acceptance workflow,
lifecycle/business/updater scripts, synthetic native provider, fixture tests and
lock-test diagnostics. Existing artifact tooling, protocol helpers and safety
checks are reused; no application dependency was added. The new uninstall hook's
update-skip branch was source-reviewed but not exercised by the older baseline's
uninstaller, which predates the hook.

## Evidence and remaining gates

Tracked sanitized reports:

- `evidence/windows-business-baseline.json`
- `evidence/windows-business-verify.json`
- `evidence/windows-business-lifecycle.json`
- `evidence/windows-installed-update.json`

Raw reports, run metadata and candidate bytes live under
`.omx/reports/platform-admin/windows-business/run-37016263509/`. Private test
credentials/state were never uploaded. Owned apps/daemons/backend/PostgreSQL and
fixture protocol/home state were cleaned; hosted runner disposal finishes teardown.

Boundaries: administrator interactions use real APIs, not the admin-page UI.
Tasks use a deterministic native provider and no personal/model credentials.
Signing Secret and deployment Environment lists were empty, no signing material
or approved preproduction target was supplied, and the actual installer is
unsigned. Trusted signing, offline chain/timestamp validation and actual
preproduction/production maintenance remain unperformed. Reference hardware,
cold-storage and extended failure gates also remain in S07.
