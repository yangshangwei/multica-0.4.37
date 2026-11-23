# Linux arm64 packaged candidate

**Directory package and DEB build/static payload inspection passed. No app, CLI, installer, daemon, database or container was run by this lane.**

Candidate `1a32ca36c2c4f4455a01bdd39d0e0ab4db20cccc`; app `0.6.7-476-g1a32ca36c`; Debian `0.6.7~476~g1a32ca36c`; Electron39.8.7 Linux arm64.

## Artifacts

- Directory: `/Volumes/artisan/code/2026/multica-0.4.37/.omx/private/core-e2e-20261104/linux-package/directory-v2/linux-arm64-unpacked`
- DEB: `/Volumes/artisan/code/2026/multica-0.4.37/.omx/private/core-e2e-20261104/linux-package/deb-v2/multica-desktop-0.6.7-476-g1a32ca36c-linux-arm64.deb`
- Environment snippets: `evidence/linux-package/directory.env` and `deb.env`. Select only one mode and clear conflicting native/AppDir/AppVolume/DEB variables.

| Artifact | SHA-256 |
| --- | --- |
| asar | `5da9b5eb86ffd6513f21acdab19a57c755807c2620833a8e77714a01997afd8d` |
| executable | `379334fb6707615062d32fcc481a8a8d3140938531378ccba14f3fa169665043` |
| cli | `7cd6e1b4364842c98760983f5e248e4e2de8dcbe49701846907b3f5c120ad648` |
| deb | `1d19374fbb82b9f1dea694804cd7481981871c6517ba6653edd47b501c63af0b` |

## Source to payload proof

Existing locked dependencies installed offline in independent worktree. All495 copied Electron out files match the parent same-candidate build before/after copy and inside packaged ASAR. Linux arm64 CLI cross-built with CGO0, GOMAXPROCS2, Go-p2 and exact candidate version. CLI/Electron ELF headers identify AArch64.

Fresh v2 directory has80 regular files, unchanged throughout DEB generation. DEB staging retained every original file byte; only apparmor-profile and package-type were added. All82 staged files match DEB tar payload. ASAR/exe/CLI hashes also match the first build. Source package.json is unchanged; parent live out/resources/bin/package.json remain unchanged.

DEB: package multica; architecture arm64; desktop Exec `/opt/Multica/multica-desktop %U`; icon multica-desktop; StartupWMClass Multica. Archive does not claim `/usr/bin/multica`. This is archive inspection, not dpkg install or native acceptance.

## Retained harness failures

1. First DEB build exited0; its whole-source-directory equality guard failed because builder adds packaging metadata. Original output and06-postcondition-failure.json retained.
2. Fresh v2 builds exited0; check incorrectly expected three added files. app-update.yml already existed and stayed byte-identical; only two files were added.09-postcondition-failure.json retained; independent full-manifest inspection passed without another build.
3. Desktop-entry static checker expected quotes around a path with no spaces. Actual valid unquoted Exec failed that assumption.11-desktop-inspector-failure.json retained. Follow-up shlex parsing requires exact executable and%U argument and retains all payload checks.

Build exits, private-checker failures and subsequent static proofs are separate. No product source/threshold changes.

## Runtime handoff

Use existing e2e/collaboration-host-installed.spec.ts with parent-owned password/API/Host fixture environment. Pinned image IDs and artifact hashes are in env snippets. Executor must verify actual Docker realpath mounts and Playwright dependency resolution inside container before fixture entry. Files being under /Volumes/artisan/code alone are not a mount proof. See JSON prerequisites.

No native desktop, installation/upgrade, real Provider or release-signature acceptance is claimed.

## Packaging source mode restoration

Final audit found electron-builder changed eight tracked PNG icon modes from0755 to0644 in the dedicated packaging worktree. Every byte matched HEAD. The observation is preserved in13-icon-mode-observation.json; only those eight modes were restored to tracked0755. Final worktree status is clean, and package artifacts were not changed. Parent checkout remained untouched.
