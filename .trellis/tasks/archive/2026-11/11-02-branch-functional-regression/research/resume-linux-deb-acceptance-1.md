# Linux ARM64 DEB package-manager acceptance — 2026-11-02

**PASS for the exercised package-manager lifecycle. Native Linux desktop acceptance remains BLOCKED.**

Candidate product source: 7af9ab9905c98c77bb091223ebe687c639475bb4. Packaging source: 639bf6ef85ad894553b244873c2ed8d9f9dd29b2; its server, apps and packages Git trees equal the candidate. Original application directory bytes were preserved by building from a private copy. No old installation result was reused.

## Artifacts

| Input | Version | SHA-256 |
| --- | --- | --- |
| Historical baseline DEB | 0.6.5~179~g1bfd169dc | 7e23d62da6189d128bb08dd2caa011a8ca61ea6ad5256d346c331ec794b1c346 |
| Current candidate DEB | App 0.6.8-rc.20261102; Debian 0.6.8~rc.20261102 | 44f3a4c5f74e91205bea4d398a81ed737efa49d9062a9f4bc4c2234c588fb01a |

Candidate artifact (167,919,816 bytes):

/Volumes/artisan/code/2026/multica-0.4.37/.omx/private/branch-functional-regression-20261102/resume-linux-deb-1/package/multica-desktop-0.6.8-rc.20261102-linux-arm64.deb

Build evidence: evidence/resume-linux-deb-1/build.json, build.stdout, build.stderr and original-input manifests. The command used installed electron-builder 26.8.1 with --prepackaged, --linux deb --arm64 --publish never, explicit candidate version and a new private output directory. Only the copied input gained resources/apparmor-profile and resources/package-type; the original directory stayed unchanged. Tool downloads during that build are recorded in its log; no product dependencies were changed.

## Actual execution

Both newly named containers used Debian 12 / Linux ARM64 image sha256:0d433ecf8ede66777494051568fd760468be288fd99d36d80bc68d022769471c, --network none, read-only installer mounts, and no host HOME or Docker socket mount. Each started without an installed multica package. The preflight's exit 1 is the expected package-absence probe; it is not a failed installation.

| Container lane | Actual commands and result | Evidence |
| --- | --- | --- |
| Upgrade | dpkg -i baseline.deb → configured baseline → dpkg -i candidate.deb → configured candidate; **40 assertions PASS**, exit 0 | evidence/resume-linux-deb-install-1/upgrade.jsonl |
| Fresh lifecycle | dpkg -i candidate.deb → remove → reinstall → purge; **47 assertions PASS**, exit 0 | evidence/resume-linux-deb-install-1/fresh-lifecycle.jsonl |

The candidate version compares greater with dpkg --compare-versions. Actual DEB control records, maintainer scripts and hashes were inspected. Each installed stage checked:

- Main executable, ASAR and bundled CLI against the extracted payload; candidate payload also matched the pinned application directory.
- Configured dpkg status, version and architecture, empty dpkg --audit, and successful dpkg --verify.
- /usr/bin/multica-desktop alternative targeting /opt/Multica/multica-desktop; the separate /usr/bin/multica synthetic CLI sentinel was preserved and never owned by the package.
- Desktop Exec/name/icon/StartupWMClass, x-scheme-handler/multica, and at least eight installed hicolor icons. These are installed declarations, not proof of an active desktop protocol owner or running window association.
- Four synthetic user files, including the actual runtime configuration location /home/qa/.multica/desktop.json, retained identical bytes, permissions and ownership. Server/update addresses, profile/preferences, synthetic session metadata and draft state were preserved. Remove/purge also removed the desktop launcher.

The synthetic token and authentication-mode marker are test-created retention sentinels. No real account was accessed, no app was launched, and no authentication or browser/IPC migration was exercised.

## Scope, cleanup and replay

docs/offline-upgrade.zh-CN.md and docs/desktop-intranet-update-runbook.zh-CN.md were reviewed. This is desktop package installation only: the server database and authentication mode were not changed. A real delivery must retain the deployment's selected device/password mode, follow the documented server/account migration ordering when applicable, and separately verify login, restart, retained state and actual updates on the target platform.

Both containers automatically exited and were removed; the runner rechecked their exact names and the final cleanup checked both task labels. Original baseline/candidate archives remained unchanged. Evidence and package bytes are retained; no host installation or external publication occurred. See evidence/resume-linux-deb-install-1/run.json and evidence/resume-cleanup-1/run.json. The container runner and host driver are preserved with the evidence.

This does not close native Linux focus/tray/window-manager identity, actual authentication/IPC/business journeys, the automatic update mechanism, other architectures/formats, signatures, downgrade/rollback, Windows, or macOS installed upgrades.

Product lint/typecheck was not repeated because no product code or dependencies changed. Verification was the actual package lifecycle and evidence/hash checks above. Independent Trellis agents failed before tool work with 429; these are coordinator-executed and coordinator-reviewed results.
