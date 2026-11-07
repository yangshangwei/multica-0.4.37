# Desktop remaining-acceptance readiness — 2026-11-02

The current Mac distribution bytes are intact and match the candidate's recorded product payload. **Windows native acceptance, Linux native desktop acceptance, signed/notarized delivery, and installed-client upgrade remain open.** A Linux ARM64 package-manager lane is now concretely preparable because a verified historical baseline DEB and a local dependency-ready Docker image exist.

This pass performed local read-only discovery and artifact checks only. It did not rerun any completed product test, start a service/container, install an application, mount an image, use an agent account, submit notarization, or dispatch remote CI. Product candidate remains `7af9ab9905c98c77bb091223ebe687c639475bb4`. All raw command records and additional structured observations are in `evidence/resume-desktop-readiness-1/`; original evidence is unchanged.

## New observations

| Check | Actual result | Evidence |
| --- | --- | --- |
| Product provenance | `server`, `apps`, and `packages` Git trees equal between candidate `7af9ab990` and artifact source `639bf6ef8` | `artifact-byte-verification.json` |
| Pinned payloads | All 7 recorded path checks match SHA-256; Mac `MacOS/Multica` and `MacOS/multica` refer to the same bytes on this filesystem | `artifact-byte-verification.json` |
| Mac distribution | Version `0.6.8-rc.20261102`; DMG and ZIP SHA-512/size match `latest-mac.yml`; 3 ZIP payload members match pinned ASAR, executable and CLI | `mac-distribution-integrity.json` |
| Mac DMG integrity | `hdiutil verify` exit 0, image checksum valid | `commands.json`, `mac-dmg-verify` |
| Mac signature structure | `codesign --verify --deep --strict --verbose=2` exit 0 | `commands.json`, `mac-signature-verify` |
| Mac distribution trust | Signature is `adhoc`; `TeamIdentifier=not set`; `stapler validate` exit 65, “does not have a ticket stapled to it” | `commands.json`, `mac-signature-display` and `mac-stapled-ticket` |
| Local native runners | macOS 26.2 ARM64. No common Windows/Linux VM CLI on PATH, no corresponding app in the explicit locations checked, and no matching running VM executable | `tool-paths.json`, `vm-app-paths.json`, `commands.json` |
| Linux container runner | Docker context `desktop-linux`; `multica-collaboration-deb-acceptance:20261026` is present as Linux ARM64, immutable image `sha256:0d433ecf8ede66777494051568fd760468be288fd99d36d80bc68d022769471c` | `commands.json`, `docker-image-platforms` |
| Candidate installers | The known candidate packaging tree has Mac DMG/ZIP and Linux ARM64 directory application. No candidate DEB/RPM/Windows installer was found in the bounded artifact inventory | `artifact-inventory.json` |
| Linux upgrade baseline | Existing DEB `multica-desktop-0.6.5-179-g1bfd169dc-linux-arm64.deb`: package `multica`, Debian version `0.6.5~179~g1bfd169dc`, architecture `arm64`; SHA-256 matches its original build record | `linux-baseline-deb.json` |

An intact ad-hoc signature is not Developer ID signing. The failed ticket check is retained as an actual failed check; it is not reclassified as a successful notarization or a product runtime failure. No keychain identity list was read, so this pass does not claim that signing credentials are absent everywhere. The local runner search is also bounded: it does not establish the availability or absence of remote machines.

## Concrete next execution lane: Linux package manager

Baseline artifact:

`/Volumes/artisan/code/2026/multica-0.4.37/.omx/reports/collaboration-deb-install-20261026/package/multica-desktop-0.6.5-179-g1bfd169dc-linux-arm64.deb`

Its SHA-256 is `7e23d62da6189d128bb08dd2caa011a8ca61ea6ad5256d346c331ec794b1c346`. The old result is not reused; only the verified artifact is suitable as an upgrade input.

Candidate source directory:

`/Volumes/artisan/code/2026/mc-collaboration-packaging-639bf6ef8-20261102/dist/local-candidate-639bf6ef8/linux-arm64-dir/linux-arm64-unpacked`

The existing packaging tool supports the same `electron-builder --prepackaged <directory> --linux deb --arm64 --publish never` operation used for the baseline. A new run should use the matching candidate checkout's `apps/desktop` as its working directory, an unused task-owned output directory, and `-c.extraMetadata.version=0.6.8-rc.20261102`. Record builder/configuration hashes and verify both source payload preservation and the new DEB's extracted ASAR/executable/CLI hashes before installation. Do not run the old launcher.

Run the following lifecycle only in a newly named, task-labeled container pinned to the image digest above, with `--network none`, read-only package mounts and no user HOME or Docker socket mount:

1. Record OS/architecture and assert no `multica` package exists. Inspect both DEB control records and verify the candidate version compares greater than the baseline with `dpkg --compare-versions`.
2. Install the baseline with `dpkg -i`, assert package status/version, `/usr/bin/multica-desktop` alternative and installed payload hashes. Create explicit synthetic user configuration/state sentinels in the disposable container.
3. Install the candidate with `dpkg -i`; assert package version, candidate hashes, desktop entry/icon/protocol files and sentinel preservation. Check the desktop executable never takes ownership of `/usr/bin/multica`, the separate Go CLI name.
4. If the lane includes uninstall/reinstall, inspect the actual maintainer-script contract before choosing removal/purge behavior; preserve expected user state and retain each command result separately. Do not assume downgrade support from a successful upgrade.
5. Stop only the newly allocated container, retain its evidence and package bytes, and update a separate package-manager subgate. This cannot close native Linux desktop/window-manager, authentication/IPC business journey, signed release, RPM, AppImage or Windows acceptance.

The dependency-ready image and historical build command make this a practical next step. No new candidate DEB has been built by this readiness pass, so no current package install or upgrade is yet claimed.

## Other remaining gates and recovery

- **Windows native install/upgrade:** provide an actual isolated Windows x64 runner and a candidate installer tied to the fixed source. `.github/workflows/desktop-smoke.yml` already defines a `windows-acceptance` job and installed collaboration/business switches; source/version/previous-tag inputs must be pinned before any separately authorized remote dispatch. The local stable Windows installers found under `dist/release/` are baseline candidates, not current-product results. `require_signed=false` cannot satisfy the signature gate, and Windows ARM64 remains separate.
- **Linux native desktop:** the Docker image can cover package-manager behavior but does not provide a real desktop session. Use a dedicated Linux VM/host and installed candidate to check window-manager identity/icons, native focus, tray/close, protocol ownership, auth/IPC and retained state after upgrade.
- **Mac signing/notarization:** obtain a correctly Developer ID-signed candidate and notarization/ticket evidence; validate exact distribution bytes on the intended target. The current ad-hoc candidate remains useful for already-passed local runtime proof only.
- **Mac installed upgrade:** a valid current DMG alone does not establish an installed older-client migration. Choose an explicit baseline/version and disposable profile, preserve authentication mode and server/update settings, exercise actual installation and update, and verify retained user state. The older local `0.6.5-185-g61ff581d2-dirty` DMG is unreviewed as an upgrade baseline.

## Verification and changes

Only this research note and the new `evidence/resume-desktop-readiness-1/` files were written. No source fix or simplification was needed. Lint/typecheck/product tests were not repeated because this is documentation plus read-only readiness evidence, and the user explicitly asked to preserve completed testing. The structured evidence JSON was parsed and the expected pins, exit codes and distribution checks were inspected. Remaining risks are the open platform/trust/installed-upgrade gates above, plus the deliberately limited search for remote infrastructure.
