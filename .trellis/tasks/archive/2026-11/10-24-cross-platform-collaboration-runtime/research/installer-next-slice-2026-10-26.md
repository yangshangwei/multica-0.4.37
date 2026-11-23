# Research: Linux arm64 installed-package acceptance

- Query: Find an immediately executable installation acceptance step after the completed 58-case collaboration inventory, while preserving the accepted artifact bytes and personal host configuration.
- Scope: Internal repository and installed build-tool source; no builds, installations, service operations, authenticated accounts, or publication.
- Date: 2026-10-26

## Findings

### Recommended slice

Build a Debian arm64 distributable from the accepted Linux arm64 directory package, install it with `dpkg` in a fresh UUID-owned Linux container, and run the existing full Main host-control journey against the actual installed files. This adds package-manager installation evidence to the existing directory-package evidence. The Provider runtime remains a separate native macOS fixture daemon; retain that distinction in the resulting record.

The accepted source is `1bfd169dca196dfd22bc1429a2933204d7f9e05b`. The existing package manifest is `.trellis/tasks/10-24-cross-platform-collaboration-runtime/research/continuation-package-2026-10-26.json`; it pins the directory, Electron executable, ASAR and bundled CLI. Preserve those files, the earlier reports and the original Docker-125 prerequisite failure.

### Build without destroying the accepted directory

The installed `electron-builder --help` advertises `--prepackaged` / `--pd` as the path to an existing packaged app to wrap in a distributable. Use the package-source checkout, existing builder configuration and a new output directory:

```sh
pnpm --dir <package-source>/apps/desktop exec electron-builder \
  --linux deb --arm64 \
  --prepackaged <accepted-linux-arm64-unpacked> \
  --publish never \
  -c.directories.output=<new-task-owned-output> \
  -c.extraMetadata.version=<version-read-from-accepted-ASAR>
```

Read the accepted ASAR's `package.json` version first, and require the resulting Debian package version to match it. Do not infer the version from the root checkout's current `package.json` or tags. Verify the original three artifact hashes before and after the packaging operation.

Do not run `apps/desktop/scripts/package.mjs` for this repack: lines 390–394 remove `apps/desktop/dist`, and subsequent steps rebuild renderer and bundled CLI. That would consume the accepted directory and change the evidence scope. `--prepackaged` should preserve the accepted Electron/Main/preload/renderer/CLI payload.

### Actual installation layout

The locally installed builder is `app-builder-lib@26.8.1`. Its source gives the exact layout:

- `node_modules/.pnpm/app-builder-lib@26.8.1_dmg-builder@26.8.1_electron-builder-squirrel-windows@26.8.1/node_modules/app-builder-lib/out/appInfo.js:54` selects `productName`, and line 55 sanitizes it without lowercasing. Repository `apps/desktop/electron-builder.yml:2` sets `productName: Multica`.
- The same package's `out/targets/LinuxTargetHelper.js:8` sets the installation prefix to `/opt`.
- `out/targets/FpmTarget.js:190` maps the entire existing directory to `/opt/${appInfo.sanitizedProductName}`: **`/opt/Multica`** for this application.
- `apps/desktop/electron-builder.yml:85` sets `executableName: multica-desktop`.
- `out/targets/LinuxTargetHelper.js:94` derives the desktop launch path `/opt/Multica/multica-desktop`.
- `out/targets/FpmTarget.js:201` installs `/usr/share/applications/multica-desktop.desktop`.
- `templates/linux/after-install.tpl` registers `/usr/bin/multica-desktop` through `update-alternatives` or a symlink. It does not install a competing `/usr/bin/multica`.
- `apps/desktop/electron-builder.yml:103` pins `StartupWMClass: Multica`; line 110 selects the repository's icon set.

Validate this layout from the produced `.deb` metadata and a real successful installation rather than trusting research alone. Expected installed payload paths are `/opt/Multica/multica-desktop`, `/opt/Multica/resources/app.asar`, and `/opt/Multica/resources/app.asar.unpacked/resources/bin/multica`.

### Minimal fixture change

`e2e/fixtures/collaboration-installed-main.ts:33` currently accepts only a directory/volume and mounts it read-only at `/opt/multica`; lines 43–46 start a task-owned container and run the existing protocol controller. Add a separate explicit `.deb` mode while leaving this existing directory mode intact. Require one selected mode, an existing pinned local package and an explicitly selected local image.

For the `.deb` mode, mount only the `.deb` read-only at a task-owned path such as `/acceptance/package.deb`; do not bind the directory package into `/opt`. In the container controller, install via the real package manager, require a zero exit code and an installed package status, inspect architecture/version, and then launch the installed `/opt/Multica/multica-desktop`. Capture installation stdout/stderr privately rather than forwarding it into the line-oriented protocol.

`e2e/fixtures/collaboration-installed-main.mjs:13` and line 14 currently hardcode `/opt/multica` executable/ASAR paths; line 187 pins the corresponding `file:` renderer path; line 225 pins bundled CLI. Those should select `/opt/Multica` only for the explicit `.deb` mode. Keep fresh-container `/root` and `/root/.config/Multica` checks, actual password-form authentication, native IPC, original receipts, original timeout budgets and existing test behavior.

`e2e/collaboration-host-installed.spec.ts` already verifies a real three-launch host lifecycle: initial execution, scoped Stop, app restart while paused, retained session/directory identity, mixed-author queued input, explicit Resume, terminal evidence and capacity release. Reuse those assertions for the installed `.deb` variant; add installation provenance, rather than creating an unrelated happy-path smoke.

Required additional installation facts: Debian package SHA-256, package name/version/architecture, successful `dpkg` status, actual executable/ASAR/CLI hashes matching the accepted directory, actual `file:` renderer location under `/opt/Multica`, desktop entry and CLI-name separation. Do not add network package downloads or bypass dependency validation to obtain a pass.

### macOS safety finding

The current packaged Main has no existing task-specific override for all host configuration paths. `apps/desktop/src/main/runtime-config-loader.ts:84` fixes runtime config at `app.getPath("home")/.multica/desktop.json`; `apps/desktop/src/main/daemon-manager.ts:67` fixes preferences at `os.homedir()/.multica/desktop_prefs.json`; `apps/desktop/src/main/daemon-profile.ts` derives daemon profile locations from the actual home. A Chromium `--user-data-dir` option would not isolate these files. Development-mode userData separation is not a packaged full Main equivalent. A native macOS installed-app run therefore needs a separately designed safe host-user/config strategy and should remain pending in this slice.

## Related specs and files

- `.trellis/spec/desktop/frontend/intranet-updates.md`: artifact/version provenance; actual installed-OS evidence is separate from download checks.
- `.trellis/workflow.md`: resume the active task; research is persisted separately from implementation.
- `docs/desktop-intranet-update-runbook.zh-CN.md`: desktop upgrades preserve authentication mode and user identity; installation does not migrate the server.
- `docs/offline-upgrade.zh-CN.md`: authentication and migration controls remain deployment-owned.
- `apps/desktop/scripts/package.mjs`: full build wrapper and destructive-to-output-directory preamble.
- `apps/desktop/electron-builder.yml`: executable, branding, icons and target configuration.
- `e2e/fixtures/collaboration-installed-main.ts` / `.mjs`: existing full Main controller and container transport.
- `e2e/collaboration-host-installed.spec.ts`: accepted host-control journey to reuse.

## External references

The installed `electron-builder` CLI help links `https://www.electron.build/linux` and `https://www.electron.build/configuration`. Build/layout conclusions above were checked against the installed v26.8.1 implementation, rather than inferred from external examples. No web retrieval was needed.

## Caveats / Not Found

- This research does not establish that the selected existing Docker image already has every Debian runtime dependency. The real install must decide that; a configured prerequisite failure is a failure, not a successful skip.
- Post-install scripts may select SUID sandbox permissions when Docker denies user namespaces. The existing root/Xvfb test uses `--no-sandbox`; record this container acceptance scope and do not treat it as a desktop sandbox, AppArmor, elevation or distro-wide assurance.
- A Debian package-manager install plus full Main journey does not prove macOS/Windows installation, signatures/notarization, auto-update, rollback, in-place upgrades preserving an older real client's state, authenticated vendor Providers, or production rollout.
- No existing general-purpose macOS packaged configuration/profile override was found in the inspected runtime/daemon files.
- The `.deb` result must be recorded as new evidence; do not rewrite the 58 original identities or describe the preserved source passes as fresh root-HEAD executions.
