# macOS actual Main upgrade and retained profile — 2026-11-02

**PASS for actual baseline-to-candidate Main startup, isolated configuration/IPC reads and synthetic localStorage retention.** Real business login, credential migration, automatic updater and signed delivery remain open.

This supplements the earlier file-install pass and resolves its startup protocol-registration prerequisite with a measured capture/cleanup boundary. It does not rewrite the earlier no-launch result.

Latest runtime evidence is evidence/next-mac-runtime-2/. The first run's Main/preload/localStorage checks passed, but its incomplete synthetic server configuration produced an authentication-configuration error in both screenshots. That original run is retained; it is not normal authentication UI evidence. Run 2 corrected only the synthetic API response to include password_auth_available=true with auth_mode=password and device_auth_available=false, then required the real username/password fields and password form to be visible on both versions. Those UI assertions and screenshots pass. Neither run submits a real login.

## Actual inputs and execution

The immutable ZIP distributions already verified in research/mac-upgrade-acceptance-1.md were copied into a new private stable installed/Multica.app path:

| Stage | Actual Main version | ASAR SHA-256 |
| --- | --- | --- |
| Baseline | 0.6.5-185-g61ff581d2-dirty | da566a4d55d35948fdf8150251e5980bbfcc6ffd0845f659f8f02737fe4201c6 |
| Candidate | 0.6.8-rc.20261102 | 128ff6f87dc18b78355ee5161171a86ec7828b18d5b87751d0f8142dd81b8485 |

Installed application prefix:

.omx/private/branch-functional-regression-20261102/next-mac-runtime-1/installed/Multica.app

The actual Playwright Electron launcher started each immutable application executable, with no alternate Main, patched ASAR, replaced IPC handler, injected credential or automatic retry. Both reported packaged=true, darwin/arm64, the expected exact version, and the same actual appPath under the private installation. Each returned Electron home and userData inside the explicit task acceptance home. Baseline Main was closed before replacing the app at the same path, and candidate Main was then launched with that retained profile.

Assertions read the real preload APIs: desktopAPI.runtimeConfig, daemonAPI.getPrefs/getStatus and updater.getPreferences. Both read the configured synthetic loopback server/update addresses, daemon autoStart=false and automaticUpdates=false; daemon state was stopped. The runtime configuration bytes, including an unknown field, remained identical across both launches.

The baseline's real renderer wrote a clearly synthetic localStorage marker and flushed default-session storage. The candidate renderer read exactly the same marker after app replacement. This establishes retained localStorage at the actual installed file origin; it does not establish a logged-in session or product-specific draft migration.

The startup server was a task-owned loopback synthetic HTTP endpoint. It returned only a synthetic password configuration/health response and rejected other endpoints; there was no real business authentication or Provider. Its observed requests were /api/config and /health. Run 2 attached a BrowserContext observer after Electron launch and before firstWindow, plus a later page observer. They observed only the loopback origin. Neither constitutes an OS-level egress trace or coverage of requests before Electron launch resolved.

## Native protocol capture and restoration

Before launch, the native NSWorkspace query recorded the exact multica:// default application as /Applications/Multica.app, bundle ai.multica.desktop, plus existing same-bundle application registrations.

Both task app launches left the exact default path unchanged and added only the private installed app to the queried registration set. Cleanup unregistered that newly added task-owned path with lsregister -u. The final NSWorkspace query returned the original exact default and the original registration set. No default-handler change was needed, and the fallback NSWorkspace restoration API was not invoked.

The user installation was never launched, copied over or edited. No preexisting registration was removed. The local SDK permits exact URL-based recovery but some schemes may prompt; this run did not exercise or require that recovery path. Do not treat an untested fallback as proof for another scheme or machine.

## Evidence, cleanup and limits

Raw stages, main/preload observations, protocol before/after/final, request summaries, command records and screenshots are in evidence/next-mac-runtime-2/, with the original narrower run retained in next-mac-runtime-1/. Each exact driver is preserved with matching execution SHA-256. The candidate, baseline and retained profiles remain in private task storage.

Final process/port/config/source integrity inspection is recorded separately in evidence/next-cleanup-2/run.json; next-cleanup-1 retains the pre-run2 cleanup. No host app, real account, paid Provider or product source was changed; no remote dispatch/publication occurred. Product lint/typecheck was not repeated for this verification-only work.

The historical baseline is a dirty build with verified exact bytes but unreconstructed dirty source diff. This result applies to that exact ZIP and current candidate, not every 0.6.5 installation. Ad-hoc signature structure is not Developer ID or notarization. Actual business credential migration, authenticated API journey after upgrade and automatic update installation remain NOT_RUN. Prior full-Main business results are preserved separately and are not substituted for those upgrade-specific checks.
