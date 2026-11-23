# Research: Windows x64 v0.6.6 release

- Query: Reproduce the previous Windows x64 local intranet release from committed main, preserving the installer, original blockmap and latest.yml, with truthful unsigned and validation evidence.
- Scope: Internal code/configuration, installed dependency sources and local tool-cache inventory. No builds, git operations, native Windows execution, external machine probes or secret inspection.
- Date: 2026-10-13
- Frozen source: `4797e6d971296e56f3edc4f6cb6d3f218d9f8d85`, supplied and confirmed by the lead session. Lead states packaging sources in this checkout equal that commit; this researcher did not run git.
- Delivery: `dist/release/v0.6.6/`.

## Findings

### Recommended execution

Use an isolated checkout of the frozen commit, with its own writable desktop build outputs. The previous release rejected a candidate built in a concurrently edited shared checkout; its replacement used a clean detached worktree and verified bundled Go `vcs.modified=false`. Do not share `apps/desktop/resources/bin`, `apps/desktop/out` or `apps/desktop/dist` between packaging lanes.

From the prepared isolated source root, the standard packaging command is:

```bash
MULTICA_DESKTOP_VERSION=0.6.6 CSC_IDENTITY_AUTO_DISCOVERY=false \
  pnpm --filter @multica/desktop package -- --win --x64 --publish never
```

Equivalent command from that checkout's `apps/desktop/`:

```bash
MULTICA_DESKTOP_VERSION=0.6.6 CSC_IDENTITY_AUTO_DISCOVERY=false \
  node scripts/package.mjs --win --x64 --publish never
```

The baseline configuration already has the release file allowlist which v0.6.5 supplied via an external JSON config. Start with this standard configuration; the historical `_build/windows-x64/electron-builder.worktree.json` no longer exists and must not be referenced as a live file.

The wrapper performs these steps in order (`apps/desktop/scripts/package.mjs:382`):

1. Validates the explicit SemVer and target matrix.
2. Deletes that checkout's entire `apps/desktop/dist` directory (`:391`). Preserve earlier evidence outside it before rebuilding.
3. Runs a fresh `electron-vite build`, covering main, preload and renderer (`:402`). Directly invoking electron-builder on old `out/` is unsafe.
4. Overrides packaged `package.json.version` with `0.6.6` without modifying the tracked `0.1.0` source value (`:327`, `:433`).
5. Runs `bundle-cli.mjs --target-platform win32 --target-arch x64` (`:455`). It inherits the same explicit version.
6. Runs electron-builder for Windows x64 only with `--publish never`. A single target uses ordinary `apps/desktop/dist`, not `dist/win-x64`; matrix builds use scoped output directories (`:453`).

Expected artifacts:

```text
apps/desktop/dist/multica-desktop-0.6.6-windows-x64.exe
apps/desktop/dist/multica-desktop-0.6.6-windows-x64.exe.blockmap
apps/desktop/dist/latest.yml
apps/desktop/dist/win-unpacked/Multica.exe
apps/desktop/dist/win-unpacked/resources/app.asar
apps/desktop/dist/win-unpacked/resources/app.asar.unpacked/resources/bin/multica.exe
```

Preserve the three distribution artifacts byte-for-byte with the existing collector. With `RELEASE_066_DIR` set to the **absolute** final delivery directory outside the isolated source's desktop dist:

```bash
bash scripts/desktop-updates.sh collect apps/desktop/dist "$RELEASE_066_DIR"
```

The collector also includes matching `.7z` artifacts if present. Check its resulting inventory and distinguish installer payload archives from the three required public Windows artifacts. Do not rewrite generated YAML, rename architecture channels, or mix v0.6.5 blockmaps with v0.6.6 bytes.

After packaging, validation and final collection, produce the Windows verification JSON and checksums from the actual delivered bytes, recording the exact source, effective packaging configuration, versions, signed status, test exit codes and evidence paths. Preserve originals and logs until final acceptance; all v0.6.5 `_build/` helper scripts/logs were removed after delivery.

### Toolchain and cached prerequisites

Observed on this machine using command lookup, version commands and bounded directory inventory:

| Item | Observed / required |
| --- | --- |
| Host | Darwin arm64 |
| Node | `/Users/artisan/.nvm/versions/node/v22.23.2/bin/node`, `v22.23.2` |
| pnpm | `/opt/homebrew/bin/pnpm`, `10.28.2`; matches root `package.json:40` |
| Go | `/opt/homebrew/bin/go`, `go1.27.0 darwin/arm64`; `server/go.mod:3` requires at least `1.26.6`, CI chooses latest `1.26.x` |
| Docker CLI | `/usr/local/bin/docker`; engine/image availability was not probed by this researcher |
| Electron lockfile | `39.8.7` |
| electron-builder / app-builder-lib lockfile | `26.8.1` |
| electron-vite / electron-updater | `5.0.0` / `6.8.3` |
| Transitive inspection tools | `@electron/asar 3.4.1`, `resedit 1.7.2`, `7zip-bin 5.2.0` already installed |
| Windows Electron ZIP | `/Users/artisan/Library/Caches/electron/electron-v39.8.7-win32-x64.zip` exists |
| Host Electron ZIP | `/Users/artisan/Library/Caches/electron/da565d375204a0dabf609e4e54bd2cdb853d18c44438472916d6d3ee0aac9581/electron-v39.8.7-darwin-arm64.zip` exists |
| NSIS | `~/Library/Caches/electron-builder/nsis/nsis-3.0.4.1-nsis-3.0.4.1/` and `nsis-resources-3.4.1-nsis-resources-3.4.1/` exist; native `mac/makensis` exists |
| Resource tools | `~/Library/Caches/electron-builder/winCodeSign/winCodeSign-2.6.0/rcedit-ia32.exe` and `rcedit-x64.exe` exist |
| Bundled Wine | `~/Library/Caches/electron-builder/wine/wine-4.0.1-mac/` exists, including `bin/wine64` |
| PATH tools absent | `wine`, `wine64`, `7za`, `pwsh`; absence on PATH does not imply electron-builder cannot use cached bundled tools |

For 7-Zip use the installed package path or resolve the package, not bare `7za`:

```text
node_modules/.pnpm/7zip-bin@5.2.0/node_modules/7zip-bin/mac/arm64/7za
node_modules/.pnpm/7zip-bin@5.2.0/node_modules/7zip-bin/mac/x64/7za
```

Cache presence proves availability of files only, not that archived Wine/resource tools execute successfully on this macOS. No cache archives were executed or independently checksum-verified here. Normal `pnpm install --frozen-lockfile` is the CI dependency contract; dependency installation in the isolated build environment is the implementer's responsibility. The local pnpm version command emitted the existing warning that root `pnpm.onlyBuiltDependencies` / `pnpm.overrides` are no longer read from package.json; do not silently rewrite repository configuration for this release.

### Resource editing, signing and archive pitfalls

- `CSC_IDENTITY_AUTO_DISCOVERY=false` follows v0.6.5 and CI smoke usage; it is not a substitute for inspecting final PE certificate tables. Report Authenticode `unsigned` only after checking the delivered installer and desktop binary. Never claim SmartScreen or offline certificate trust from a checksum.
- Unsigned packaging still edits Windows executable resources. Installed `app-builder-lib/out/winPackager.js:126` sets ProductName, FileDescription, version strings, company name and icon; on Darwin it invokes the app-builder `rcedit` command (`:184`). Cached Wine/resource-editor incompatibility can therefore fail an unsigned build.
- Do not blindly add `-c.win.signAndEditExecutable=false` to bypass a resource error: `winPackager.js:231` returns without editing the desktop executable, leaving upstream Electron product/version/icon metadata unless another verified pre-packaging resource-editing step replaces it. It can also affect signing behavior for other executables. The installed `resedit` package could support an isolated fallback, but no surviving previous fallback implementation was found. Any such fallback needs its own documented effective config and final PE resource checks; do not present it as already proven.
- NSIS uninstaller creation is separate. `app-builder-lib/out/targets/nsis/NsisTarget.js:336` first builds the uninstaller; modern macOS follows `UninstallerReader.exec` and can fall back to a VM, while the other path invokes Wine. A successful renderer build alone does not establish that NSIS assembly succeeded.
- `apps/desktop/electron-builder.yml:5` now includes only `out/**`, `resources/**`, `package.json`, plus production dependencies; `!dist/**` prevents nested build-output contamination. LICENSE and NOTICE are extra resources (`:31`). Check the actual ASAR for environment files/tool state rather than relying solely on configuration.
- `asarUnpack: resources/**` (`electron-builder.yml:40`) places the Go CLI outside the archive at the expected runtime path. An ASAR entry marked unpacked must be verified against its physical unpacked file.
- `bundle-cli.mjs:114` builds with `CGO_ENABLED=0`, `GOOS=windows`, `GOARCH=amd64`, and `main.version`, `main.commit`, `main.date` ldflags. Source output is `server/bin/windows-amd64/multica.exe`, then copied into desktop resources. Missing Go is only a warning in this script (`:155`), so the release must independently require Go and verify the resulting bundled binary. Do not accept the runtime auto-download fallback for an intranet installer.
- macOS attempts ad-hoc `codesign` even on the bundled Windows file and ignores failure (`bundle-cli.mjs:179`); that does not constitute Windows signing.
- The NSIS bootstrap may be PE machine `0x014c` (32-bit) while its installed payload is x64. Verify `Multica.exe` and bundled `multica.exe` are `0x8664`, optional-header magic `0x20b`. Rejecting the installer solely for its bootstrap architecture is incorrect.

### Reusable validation and evidence plan

Run the narrow packaging/update contract suite from the isolated root:

```bash
pnpm --filter @multica/desktop exec vitest run \
  scripts/package.test.mjs \
  scripts/bundle-cli.test.mjs \
  scripts/update-artifacts.test.mjs \
  scripts/verify-updates.test.mjs \
  scripts/configure-updates.test.mjs
pnpm --filter @multica/desktop typecheck
pnpm --filter @multica/desktop lint
node --test scripts/desktop-updates.test.mjs
```

These `.test.mjs` packaging files use Vitest, not Node's test runner. v0.6.5 recorded five contract files / 135 tests plus desktop typecheck/lint; do not copy those counts forward without running the current suite. Any broader checks belong to the lead's release verification plan.

Offline payload verification should reproduce the previous evidence shape:

1. Read the packaged ASAR `package.json`; assert product `Multica`, desktop version `0.6.6`, entrypoint `./out/main/index.js`.
2. Recompute ASAR integrity hashes for packed entries and check unpacked entries against physical files. Use existing `@electron/asar` inspection APIs (`getRawHeader`, `listPackage`, `extractFile` as appropriate) and Node crypto. The previous full archive verifier was a temporary helper and is no longer available.
3. Compare every fresh `out/renderer` file with its packaged ASAR counterpart; retain a manifest. Assert main/preload entrypoints are present and the renderer includes its HTML and chunks. Inspect archive inventory for `.env*`, private tool/session state and recursively included `dist`.
4. Inspect PE machine, version resources and certificate table for the installer and packaged desktop; inspect x64 machine for bundled CLI. `resedit` can read resources without executing Windows binaries.
5. Run host Go's read-only `go version -m <packaged-multica.exe>`; assert `GOOS=windows`, `GOARCH=amd64`, `main.version=0.6.6`, expected commit/revision and `vcs.modified=false`. This reads build info; it does not execute Windows code.
6. Extract the NSIS embedded payload with package-provided 7-Zip, then byte-compare at least `Multica.exe`, `resources/app.asar`, and `resources/app.asar.unpacked/resources/bin/multica.exe` with `win-unpacked`. The previous successful extraction reported expected extra data after the archive (NSIS wrapper/tail). Do not mistake an extraction warning alone for failure; still require successful extraction and byte equality.
7. Collector validation proves latest.yml references, exact sizes and Base64 SHA-512. Separately save SHA-256 for the original EXE, blockmap and YAML, and include them in the final release checksum manifest.

HTTP verification should use the production Nginx config in a temporary loopback-only Docker project and separate storage, never the user's live feed. `docker-compose.desktop-updates.yml` uses Nginx pinned to `sha256:985220252f3863977e468f611ef118ebd01421289dd86ee1ae99cb068c3bce2b`; its public mount is read-only and its wrapper starts with `--pull never`. Inspect/prepare the pinned image before starting. The previous run used local tag `multica-desktop-nginx:985220252f38-amd64`, but its presence today has not been verified.

With `RELEASE_066_HTTP_STORAGE` set to an isolated absolute storage directory:

```bash
node apps/desktop/scripts/update-artifacts.mjs publish \
  --source "$RELEASE_066_DIR" --destination "$RELEASE_066_HTTP_STORAGE"
```

Start the separate Compose project with `DESKTOP_UPDATES_STORAGE` set to that directory, `DESKTOP_UPDATES_BIND=127.0.0.1`, and a free fixed port. Publish writes `<storage>/public`; mount that directory, not the packaging dist. Keep metadata backups/locks outside public. Once healthy, use the actual port:

```bash
node apps/desktop/scripts/verify-updates.mjs \
  --url http://127.0.0.1:PORT/desktop \
  --metadata latest.yml --expected-version 0.6.6
```

`verify-updates.mjs:32` provides JSON evidence for metadata `no-cache`, exact version, HEAD `200`, Range `206`/correct Content-Range, full length and SHA-512. It rejects redirects, unsafe references and transformed content. It also checks blockmap transport, but the typical latest.yml does **not** contain a source blockmap checksum: its `checksumVerified:false` is correct and must remain truthful (`:87`). Independent local source/copy SHA-256 checks can establish preservation separately. Restart the fixture container and verify artifact retention if reproducing the full storage contract, then tear down only that fixture.

### Native Windows environment and available acceptance

No documented reachable local Windows VM, SSH/WinRM host or persistent test workstation was found in the inspected release/configuration documents. No machines, credentials, or external connectivity were probed. The documented execution environment is the opt-in GitHub-hosted `windows-latest` job in `.github/workflows/desktop-smoke.yml:116`.

The actual workflow supports `candidate_version` (`:14`), so a future explicitly authorized CI run can build exactly `0.6.6`; the prose in `WINDOWS_ACCEPTANCE.md:17` only describes the older default RC form. `windows_acceptance=true` selects Windows x64 only. `previous_tag` downloads the previous installer and `.sha256` from the same GitHub repository (`:219`), so local-only `dist/release/v0.6.5` availability does not establish that `previous_tag=v0.6.5` will work. This workflow builds its own installer; it does not validate the exact bytes of a macOS-built local installer by default.

Reusable drivers:

- `apps/desktop/scripts/smoke-windows-installer.ps1`: native Windows install, PE architecture and bundled CLI version; no GUI/business acceptance.
- `apps/desktop/scripts/accept-windows-installer.ps1`: baseline install, upgrade, uninstall, reinstall; signature evidence; native versions, registry/shortcut checks. Its `Assert-DisposableRunner` (`:19`) deliberately requires real Windows x64 and a disposable GitHub-hosted runner, refuses existing Multica installations/profiles, and must not be bypassed on a personal machine.
- `windows_business_acceptance=true` additionally selects real installed business/state retention and updater fixtures. The workflow currently pins an older accepted baseline from run `36996436086`, version `0.5.2-rc.11.1`, SHA-256 checked (`desktop-smoke.yml:206`); current artifact availability is unverified. It does not exercise a v0.6.5-to-v0.6.6 path by default.
- `apps/desktop/scripts/windows-update-acceptance.ps1` and `windows-update-probe.mjs`: installed updater against a loopback feed, installed version and retained markers/settings. The Windows business state directory contains credentials and must not be uploaded.

If no Windows runner is used in this local-delivery scope, keep these fields explicitly false: native installation, native GUI, native upgrade/profile retention, Windows trust. Wine execution would not replace these acceptance checks. HTTP feed validation is separate from actual installed-client updating.

### Previous release evidence and relevant files

| File | Purpose / useful references |
| --- | --- |
| `CLAUDE.md:250` | Release rules; include authentication mode/migration instructions when handing off installers; ordinary upgrade preserves authentication mode |
| `.trellis/workflow.md` | Research artifacts persist under active task; planning precedes execution |
| `.trellis/spec/desktop/frontend/index.md` | Desktop spec index |
| `.trellis/spec/desktop/frontend/intranet-updates.md` | Authoritative artifact, publication, feed, configuration, provenance and native acceptance boundaries |
| `apps/desktop/package.json:19` | Build/package/test/typecheck/lint scripts |
| `apps/desktop/scripts/package.mjs:96` | Shared explicit version parsing |
| `apps/desktop/scripts/package.mjs:327` | Builder argument/version/channel mapping |
| `apps/desktop/scripts/bundle-cli.mjs:114` | Go cross-build and ldflags |
| `apps/desktop/electron-builder.yml:124` | NSIS target and original Windows artifact naming |
| `apps/desktop/electron.vite.config.ts:10` | Sandboxed preload must bundle toolkit preload dependency |
| `apps/desktop/build/installer.nsh` | Windows protocol cleanup include; retain this config in any override |
| `apps/desktop/scripts/update-artifacts.mjs:55` | Collection inventory; version/reference/size/SHA-512 checks, immutable publication |
| `apps/desktop/scripts/verify-updates.mjs:32` | Real HTTP full-download verifier |
| `scripts/desktop-updates.sh:146` | Collector/publisher/config/verifier wrapper command routing |
| `docker-compose.desktop-updates.yml`, `deploy/desktop-updates/nginx.conf` | Separate pinned Nginx service; read-only public store; GET/HEAD/Range and no-cache metadata |
| `dist/release/v0.6.5/windows-verification.json` | Exact predecessor build provenance, PE/ASAR/CLI checks, payload comparison, loopback HTTP evidence and native limitations |
| `dist/release/v0.6.5/latest.yml` | Generated Windows x64 metadata shape; references only the EXE checksum/size |
| `dist/release/v0.6.5/README-v0.6.5.zh-CN.md:133` | Desktop upgrade and acceptance documentation shape |
| `dist/release/v0.6.5/desktop-intranet-update-runbook.zh-CN.md:215` | Build/collect; `:260` HTTP validation; `:387` release checklist and native installed-client acceptance |
| `.github/workflows/desktop-smoke.yml:116` | Native disposable Windows x64 acceptance workflow |
| `apps/desktop/scripts/WINDOWS_ACCEPTANCE.md` | Driver scope and explicit signed/native caveats |
| `docs/plans/2026-10-02-windows-acceptance-plan.md:11` | Native hosted CI is authoritative; Wine is not Windows acceptance |
| `.trellis/tasks/archive/2026-10/10-01-platform-admin-acceptance/windows-acceptance.md` and `windows-business-release.md` | Historical native acceptance of older candidates only |

v0.6.5 reported source `7ab8d038963dd920a611047997ac1904ee69a1eb`, EXE SHA-256 `5c63b0865b1249e77408d2812222f612a8726e966dd3ffa50310a0aa329bda8d`, unsigned installer/desktop, 32-bit NSIS bootstrap carrying x64 payload, full ASAR integrity and payload comparisons, loopback Nginx `206` + complete SHA-512 success. Its `native_windows_installation_tested`, GUI and upgrade/profile-retention fields are false. Those results are precedent, not evidence for v0.6.6.

### External references and versions

No external documentation was fetched; dependency implementation was inspected locally at the lockfile versions above. Relevant upstream reference links for an implementation contingency:

- electron-builder code signing: https://www.electron.build/code-signing.html
- electron-builder multi-platform builds: https://www.electron.build/multi-platform-build.html
- Electron ASAR tooling: https://github.com/electron/asar
- `resedit` resource tooling: https://github.com/jet2jet/resedit-js
- Node child process handling of Windows `.cmd` scripts (already cited by repository wrapper): https://nodejs.org/api/child_process.html#spawning-bat-and-cmd-files-on-windows

## Caveats / Not Found

- This was research only. No package, test suite, HTTP service, artifact verification, Windows binary or native acceptance was executed.
- Cache inventory is not tool compatibility proof. The standard cross-build may expose a resource-editor/Wine issue; the previous temporary config and helper scripts are explicitly deleted historical paths.
- Current committed file allowlisting removes the previously documented need for a custom allowlist JSON. Any new resource-editing workaround must preserve that allowlist, LICENSE/NOTICE, NSIS include and original update metadata.
- Frozen source identity came from the lead; the researcher did not inspect git. The build owner must capture clean checkout and binary VCS provenance.
- Go installed here is 1.27.0 whereas CI selects 1.26.x; record the actual chosen toolchain or pin 1.26.x in the build environment if matching CI is desired.
- GitHub Actions is a documented native execution path, not an established accessible environment in this session. No workflow was dispatched, and no remote baseline/artifact availability was checked.
- A missing `.omx/wiki` directory was detected during a documentation search; the follow-up searched only existing roots. No surviving resource-editor fallback implementation was found in task/workspace/docs notes.
- The predecessor runbook uses old example versions and a historical verification boundary. Carry operational guidance forward, but author current v0.6.6 release evidence instead of claiming those old examples describe this delivery.
