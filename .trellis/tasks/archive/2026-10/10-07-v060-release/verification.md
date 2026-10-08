# v0.6.1 verification ledger — published and verified

Release: https://github.com/yangshangwei/multica-0.4.37/releases/tag/v0.6.1

The user selected v0.6.1 after the live security gate found GO-2026-6629 in the
old v0.6.0 candidate. The original v0.6.0 tag remains at
`082723894306372ec1dd46ea6c08934468681e7d`. The new stable tag and published
release use `501f4b55f1675e59e46b86d094131d44bff1efd8`, with x/text v0.41.0 and
the corrected Windows candidate-version test. The security fix is also merged
into the active development branch; its committed application trees match the
release source exactly.

## Final v0.6.1 delivery

- Full main CI `37704376562`, canonical release CI `37709633747`, and fresh
  Windows native acceptance `37709646248`: **success**. No vulnerability bypass.
- Linux: **9 archive checks and 16 actual v0.5.5 → v0.6.1 upgrade checks passed**.
  Original accounts/sessions, workspace/project/task data, upload, configuration,
  overlays and volumes remain usable; migrations and health pass. The original
  v0.5.5 database backup was restored into a separate rehearsal environment.
- Windows: newly built v0.6.1 installer passed native installation, retained
  business-state upgrade and installed automatic update from `0.5.2-rc.11.1`.
  Downloaded installer bytes match the native report; original update metadata
  and blockmap are published unchanged.
- Actual Linux API/CLI and extracted Windows CLI contain **x/text v0.41.0**.
  Final Linux saved-image, extracted-binary and running-container digests match;
  the proof was refreshed after the last build retry.
- Exact successful CI changelog is embedded, bundled, installed and published.
  Feed SHA-256: `a5a26484daf3246dbaef9fc69ebc8503bca3273b076e27469f1f5c4446ee5c7e`.
- All **22 published assets** were downloaded and independently checked against
  staged bytes and GitHub SHA-256 digests. The original manifest has 19 entries;
  the additive provenance clarification has its own checksum sidecar.
- The clarification corrects one historical E2E baseline label: baseline is
  `8ae60f71b21685d87a8a2e706d7c2de964615050`, accepted pre-fix source is
  `082723894306372ec1dd46ea6c08934468681e7d`. Git confirms identical application
  and E2E trees between them. Installer hashes and prior immutable assets were
  not changed by the clarification.

| Final artifact | Bytes | SHA-256 |
| --- | ---: | --- |
| `multica-server-upgrade-v0.6.1-linux-amd64.tar.gz` | 295440970 | `05b0f1705cae069ad9cabdcd6512cbfa48b6460ce097eb698943a88622dccd92` |
| `multica-desktop-0.6.1-windows-x64.exe` | 178294871 | `23afe03d3d677153a6c953ba69848ef231935804a166150fd9c133d08c34ec9e` |
| `multica-v0.6.1-upgrade-docs.zip` | See asset metadata | `4a2b6ef6bad43b0a15e97c80789c2980b662d96d02074c90fd58cea2783ae0b5` |

Final evidence: `.artifacts/v0.6.1/{linux-package-verification.json,
windows-package-verification.json,release-review.json}` and
`release-publication/final-download-verification.json`. Clean handoff is
`.artifacts/v0.6.1/delivery/`; actual downloaded copies are in
`download-verification/`. Private rehearsal backups remain outside both.

Linux uses amd64 containers on an ARM Mac, not native Linux hardware. The
frontend needed a recorded builder-only 4 GiB heap override; source and runtime
environment were unchanged. Source authentication guides remain exact, with
`VERSION-NOTES.zh-CN.md` explaining v0.6.1 names. Windows is unsigned, business
acceptance uses a synthetic provider, and automatic update proved full-download
fallback, not differential updates. Production rollback was not performed.

The following sections retain the earlier local v0.6.0 stage as historical
evidence. Those old binaries do not contain the security fix and were not
published as the final release.

Repository: yangshangwei/multica-0.4.37. Branch: codex/projects-p1.
Local evidence root: `.artifacts/v0.6.0/` (ignored; private fixture credentials
must never be uploaded). Prior complete binary release: v0.5.5.

## Completed baseline

- `PATH=/opt/homebrew/opt/libpq/bin:$PATH make check`: exit 0.
- API/Web source HEAD at launch: `8ae60f71b`; production Web, one worker,
  zero retries, isolated API and Go databases.
- Static checks: 15 tasks passed; UI exports passed.
- TypeScript: core 2746, views 6197, docs 62, web 282, desktop 962;
  total **10249 tests passed**.
- Go race suites and `go vet -p 2 ./...`: passed; agent CLI guard active.
- Production Playwright baseline: **247 passed, 54 skipped, zero failed**.
  All 54 skipped identities were subsequently executed successfully: 25
  legacy/provider/Desktop/I1, 28 password/admin, one device-auth case.
  **301 unique E2E cases passed; zero unverified skips.**
  `e2e-coverage.json` records each supplemental identity and report hashes.
  Eighteen initial runner failures (browser cache and CLI path prerequisites)
  were corrected and rerun; no application assertions were weakened.
- `go -C server tool govulncheck ./...`: no vulnerabilities found.
- Production Desktop build: passed.
- Release/changelog/Windows-candidate tests with Docker smoke enabled:
  **77 passed, zero skipped**, including the final overlay image-identity guard.
- Selfhost config and shell syntax checks: passed.
- Source and release fixes pushed to `codex/projects-p1`: `5ecade152`, then
  `082723894` (Windows CRLF test normalization). Application and E2E source
  bytes are unchanged from the baseline; this is checked by the coverage audit.
- Windows native run `37651811895` was observed **success** before access was
  restricted. It covers managed protocol, installer lifecycle, retained business
  state and installed automatic update for candidate version `0.6.0`.
  The previous run `37650452921` failed in the new test's CRLF handling before
  packaging; both LF and CRLF now pass. Artifact recovery is completed below.

Baseline log: `make-check.log`. Provenance retained at
`~/.multica/dev/envs/check-20261007152623-4598/`.
API/Web stopped and isolated Go database dropped by the completed check.

## Local package delivery — 2026-10-08

The user resumed package preparation and selected reuse of this task. Network,
Docker and local Git/task writes are available again. Both artifacts use frozen
source `082723894306372ec1dd46ea6c08934468681e7d`; later HEAD commits only change
tests. No application source was modified during this continuation.

Clean handoff directory: `.artifacts/v0.6.0/delivery/`. It contains the two main
artifacts, original Windows update metadata/blockmap, Chinese guides, SHA-256
sidecars, `SHA256SUMS-v0.6.0.txt` and `verification-v0.6.0.json`. Do not distribute
the parent artifact directory: the rehearsal contains private credentials and
database backups.

| Artifact | Bytes | SHA-256 |
| --- | ---: | --- |
| `multica-server-upgrade-v0.6.0-linux-amd64.tar.gz` | 295434534 | `3291c1f9256d6e22cfb89da493c0d3fb9e630f6d5afac720be7b08a41a08f887` |
| `multica-desktop-0.6.0-windows-x64.exe` | 178291663 | `b175cb717a13350b53043896ea02534057753a6e2bf6ae64de609af65cc1fc64` |

- Windows run `37651811895` reconfirmed successful. Both original GitHub artifact
  archive digests, every extracted member, accepted installer SHA-256, update
  metadata size/SHA-512 and source identity match. Native lifecycle, business and
  automatic-update evidence was reused, not rerun. Actual native upgrade baseline
  is `0.5.2-rc.11.1`, and automatic update used full-download fallback. Installer
  and Desktop remain unsigned; differential updates and offline certificate trust
  are not claimed. Report: `windows-package-verification.json`.
- Linux production build completed with TypeScript checks. All 8 archive checks
  passed: final archive/internal image checksums, file bytes, three amd64 image
  configs/layers, frozen source, both authentication-guide copies, retained feed
  history and fixture-secret scan. macOS AppleDouble metadata was removed by
  re-archiving unchanged files with `COPYFILE_DISABLE=1`; the initial hash in
  `linux/build.log` is superseded by the final sidecar and report.
- Actual disposable v0.5.5 → v0.6.0 upgrade exited 0. All 16 runtime checks passed:
  old/new sessions, user/workspace/project/issue data, upload sentinel, volume
  identity, 18 preserved environment settings, overlay, backups, migration 566,
  health, CLI, exact installed feed and three running amd64 images. Backup remains
  at `linux-rehearsal/backups/local-v060-n0r36w6a/`. Report:
  `linux-package-verification.json`.
- Independent review repeated archive/installer verification and live health,
  source, platform, old-session/data, upload and backup checks. No local-delivery
  blockers remain; review: `package-review.json`.
- Original six Chinese document files and their ZIP were reused and their saved
  digests verified. Existing authentication modes remain unchanged.

Linux acceptance ran in amd64 Docker emulation on an ARM Mac, not native Linux
hardware. Backup creation was verified; database restore/production rollback was
not performed. Prior full lint/typecheck/test/E2E evidence remains reused because
application and packaging source were unchanged.

## Completion

The local v0.6.0 stage was superseded by the user-approved v0.6.1 security release
above. Formal publication and complete asset download verification are finished;
no release work remains pending in this task.
