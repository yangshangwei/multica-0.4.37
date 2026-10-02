# macOS native acceptance

Latest continuation: [native recovery under fleet load](native-load-recovery.md) validates 12 tasks on a real three-runtime daemon, while retaining ~109.9-second reconnect starts, 834.2 ms paging P95 and other open timing/connection limits. It does not mark S07 released.

**Passed on 2026-10-02.** The real source-built Electron Main, preload, file renderer and bundled Go CLI completed the isolated native run in 33.929 seconds. This proves current macOS source integration; it does not prove a signed/installed distribution or Windows behavior.

## Provenance

- Checkout: `/Volumes/artisan/code/2026/multica-platform-admin`.
- Commit: `1c56f6cd6c16f11327aee2f3f000e6b19e401d0b`, plus reviewed S07 working-tree corrections.
- Native source hash: `1786be8d436abcbad92135d9e78881560c924c15e048b5cab63fee0c0ef3c43b`.
- Build manifest: `.omx/reports/platform-admin/native-harness/build-5.json`; SHA-256 `3b23f7dcadf1e19ec17d35ce29f451645d5cd60d0343a6362260c712e8ec001c`.
- Run evidence: `.omx/reports/platform-admin/native-harness/run-5.json`; SHA-256 `3131c24283332f5b5f8f5575a02bfb98833f32c30f6a273a6e9be11d1047ec08`.
- Run UUID: `211edff1-c65b-4e3b-a705-bcae307b7d52`; UTC interval: `2026-10-01T23:49:29.120Z`–`2026-10-01T23:50:03.049Z`.
- API: `http://127.0.0.1:18393`, PID `53042`, commit `1c56f6cd6`. Actual `/health` PID and commit matched the fresh manifest. API source ID: `ffd54e7d60705f276d989da2d48adf2feefe23ff951f4e73d57e1417865cd876`; configuration ID: `463e2ab95f84438fe38747277eb1a4f10c606a27d072684506584c69f0128b1f`.
- Database: `multica_platform_admin_resume_password_20261002`. The driver asserted the connected database name and operator's exact database URL.
- Native API traffic used the task-owned fault proxy at `http://127.0.0.1:18493`. Electron loaded the actual compiled `file://` renderer. Web was explicitly **not exercised**; no `.next` output or Web listener was used.

The API started before the final desktop-only config writer correction. Its manifest remains an API/backend source fact, not an identical full-tree snapshot to the final native build.

| Built artifact | SHA-256 |
| --- | --- |
| Main | `1f574329c4a3e602d4cbabd3e175c47474a65fd06fa2cdccfb44a8594926197d` |
| Preload | `09b9473aa3881fe9c8f6d0b8f7de167a1c3b028dbac1db13b00986f54339bc7a` |
| Renderer entry | `e7cbfb9d45204a1ffc88c39518ecaf358814da8c698e9dbb986742cabc706440` |
| Bundled Go CLI | `d2c9dc89b05c042ccae9450a1eaba919d803c5167bf4874bd50a15bdedd2df14` |

The build manifest also hashes the entire compiled output tree. Runtime compared source identity and every artifact against it before launching.

## Observed scenarios

| Scenario | Actual evidence |
| --- | --- |
| Password login and managed enrollment | A reached an authenticated management session with exactly two owned workspace bindings. Installation identity mode was `0600`; renderer metadata contained only the expected proof-envelope fields. |
| Daemon reconnect | The proxy disconnected two real `/api/daemon/ws` connections; two new daemon connections arrived while PID `89892` and installation identity stayed unchanged. |
| Second native issue window | A second real Electron window shared the installation and daemon PID. |
| Daemon restart | Real stop/start changed PID `89892` to `89910`, retaining installation, namespace and public key. |
| CLI restart without Main | After Electron exited, the source-built CLI started PID `89925` using saved credentials with no running Main handoff. |
| Electron retained-data restart | Relaunched Electron adopted PID `89925` and retained the same identity. |
| Cancellation outbox and lost response | Real synthetic task `01a0f9df-9dc8-7dc3-b3b4-574b9face717` was cancelled through the administrator API. One receipt received 503. A private durable `stopped` receipt was observed and provider PID `89948` was verified exited. Restart changed daemon PID `89925` to `89958`; the proxy dropped the first successful receipt response after actual server acceptance. A second successful attempt confirmed original operation `a42f01bf-d4bd-42e4-b941-861ac6c8781d`, and the acknowledged receipt was removed. |
| Account A → B drain | A real held task stayed owned by A during authenticated drain (`active_task_count=1`). Releasing it led to A daemon exit before B started. B had a distinct managed daemon UUID and only B's workspace; A's task ownership/runtime and historical binding rows were unchanged. |
| Password revocation | Changing B's password made the old JWT return 401. Fresh renderer login advanced managed scope and metadata from auth version `1` to `2`. |

The run recorded 13 successful case events, including two Electron launches. Enrollment, binding, management IPC, process lifecycle and task APIs were real. Only the provider executable was a deterministic fixture with zero external/model calls; the proxy injected the stated transport faults.

## Corrections and verification

The retained earlier correction maps Node `darwin` to protocol `macos` without changing the discovery OS normalizer. Wiring tests also cover `win32 → windows` and `linux → linux`.

Run 4 exposed another real defect: Main wrote profile `config.json` as `0644`; Go's `persistManagedProfileMarker` rejected it with `management identity permissions must be private`, so no managed control endpoint was published. That failed run remains preserved. The correction writes a unique exclusive `0600` temporary file, syncs and closes it, then atomically replaces the profile. This repairs existing permissive files and preserves complete content for already-open daemon readers. All Main config writes use the existing serialized writer; no persistence layer or dependency was added.

- Three regressions were observed failing before the fix: new-file mode, existing-file mode, and an open reader being overwritten. The focused manager suite then passed **15/15**.
- Full desktop tests passed **83 files / 914 tests**, 13.58 seconds.
- Full desktop Node and renderer typechecks passed.
- Desktop ESLint: **0 errors**, one pre-existing `tab-content.tsx` hook-dependency warning.
- Fault proxy: **3/3** tests passed for HTTP failure/recovery, owned WebSocket disconnect, and successful upstream response loss/retry.
- Independent read-only review found no P1/P2 issue in the final correction. POSIX mode assertions skip Windows; no Windows ACL claim is made.
- Source build 5 and native run 5 passed after the correction. Public build/run evidence contains no managed credential prefixes.

Product files changed: `apps/desktop/src/main/daemon-manager.ts` and `apps/desktop/src/main/daemon-manager-managed-recovery.test.ts`. This is the tracked acceptance summary; drivers and raw evidence remain in the ignored task harness directory.

## Isolation and cleanup

Only repository Electron and the source-built CLI ran. All 26 discovered provider override keys were contained: one pointed to the task fixture, the others to absent paths under its private root. The child environment inherited no account credentials, proxies or provider configuration.

Separate private home, userData, appData, sessionData, logs, crash and temporary directories, mock keychain and suppressed OS protocol registration kept the installed app and user accounts outside the fixture. `/Applications/Multica.app` and real provider accounts were never opened.

All six daemon PIDs (`89892`, `89910`, `89925`, `89958`, `89997`, `90011`) were verified exited. Cleanup closed owned Electron/proxy processes and deleted exactly the three created workspace IDs through the API. There were no cleanup errors. The two synthetic accounts and private root `/private/tmp/multica-native-s07-6d0W6Y` were retained as evidence.

A separate post-run check reconfirmed all six daemon PIDs and both synthetic provider PIDs were absent, found no unsupported provider invocation, verified profile config mode `0600`, and confirmed the owned proxy/daemon ports `18493`/`20069` rejected connections.

## Remaining limits

Windows runtime/ACL, signed or installed distribution, real keychain/protocol registration, old-binary upgrade and clean reinstall remain untested. This run uses a deterministic provider, not real model execution. Separate S07 production Web, capacity and release gates retain their own evidence requirements.
