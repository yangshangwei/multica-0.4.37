# v0.6.6 local release verification

## Scope and source

- User-approved frozen main source: `4797e6d971296e56f3edc4f6cb6d3f218d9f8d85`.
- Delivery: `dist/release/v0.6.6/`; build/evidence/private fixtures remain in `.artifacts/release-v0.6.6/`.
- Application source and Windows installer bytes are unchanged during completion. Linux archive finalization synchronizes prepared documentation; the only build overlay increases V8 heap for the Web build RUN, with no runtime `NODE_OPTIONS`.
- No production deployment, GitHub publication or tag creation.

## Artifacts

| Artifact | Bytes | SHA-256 |
| --- | ---: | --- |
| Linux amd64 server upgrade | 295661175 | `80b43b34eb06f1e0db4b673f365f1e70f907caac9969cec9fecaa382b1f881da` |
| Windows x64 installer | 179209872 | `e3fe9227158b60ce089415ec86ed79cd2804895fd15bfea5c33c4995d07f483b` |

The final Linux archive supersedes the initial build-script archive `02ae1f530772ef38197ff4de940244d362a6aa2716332160897e704afc73385d` after documentation finalization. Both packaged guide copies match the delivered generic guide. The exact final archive was extracted for acceptance. Original Windows EXE, blockmap and `latest.yml` remain together with validated sizes and SHA-512 references.

## Runtime acceptance

- Password-mode v0.6.5 upgrade: original account/session/password, workspace, issue, attachment, saved iteration opt-out, secrets, deployment UUID, ordered overlays, mounts and volumes retained. Backups and persisted image selection verified.
- Device-mode v0.6.5 upgrade: original device identity/session/data retained; platform administration remains disabled, initialization reports `skipped_mode_disabled`, no administrator credentials or initialization row created.
- Both upgrades retain all 594 migration ledger entries, numeric ceiling 570, no migration delta and zero invalid indexes.
- Fresh administrator with signup disabled: `initialized` then `already_initialized`, 0600 single-link credentials, unchanged retry bytes, second maintenance host rejected without credentials, forced first change, unchanged temporary password rejected, role/overview/accounts/installations verified.
- Linux credential-file tests exercise real interrupted publication, no-clobber, unsafe-file and fsync behavior. Final saved/running backend, Web and PostgreSQL configs are Linux amd64; binaries identify the frozen source and version. Embedded and mounted changelog bytes agree.
- Windows: installer extraction, x64 Desktop/CLI, version, source revision, clean VCS, all 47,451 ASAR files, compiled output and licenses verified. Nginx full download/checksum and Range 206 pass before and after restart.

## Source checks

| Check | Result |
| --- | --- |
| Lint / typecheck / Go vet / selfhost configuration | Pass; 31 existing ESLint warnings, no errors |
| Release Node | 93 pass, four explicit Docker-smoke opt-ins skipped |
| Windows packaging and updater | 136 pass |
| Core delta | 134 pass |
| Desktop delta | 86 pass |
| Views delta | 158 pass |
| Go auth/changelog/config | 218 pass events, including subtests |
| Go handler/service integration | 174 pass events, including subtests |
| Go cmd/server, actual rerun | 63 pass events; all 25 required top-level tests run and pass, zero skips |

Original 16 desktop/views failures are retained. Both isolated files and complete original selections pass on unchanged source, assertions and timeouts; complete reruns keep the original `maxWorkers=2`. Failures were sensitive to load/timeouts, with no product defect reproduced; exact host resource cause was not measured.

The original `cmd/server` package exited zero during PostgreSQL recovery (`57P03`) without executing tests. Successful SQL readiness followed by a separate guarded rerun and a mandatory named-event audit closes that gap. The spec now requires actual test events rather than package exit success.

## Retained failures and fixture corrections

The password-mode verification initially reached final result construction after its assertions, then timed out reading `/healthz`. That attempt remains failed with its original log/hash. A separate successful recovery rechecks all retained state, current administrator access, old-password rejection, final binaries and health. The independent fresh case completes the entire first-change flow again. All three final backend health checks report database and migrations OK.

Harness-only corrections normalize Docker Desktop's `/host_mnt` bind-source alias and interpret omitted `requires_password_change` according to the frozen Go `bool,omitempty` contract. Independent negative/contract review confirms identity, mount, permissions and credential assertions remain enforced. No application or source-test changes.

## Evidence and limits

- `evidence/verification-v0.6.6.json` and `evidence/SHA256SUMS-v0.6.6.txt` retain the delivered aggregate and inventory.
- `.artifacts/release-v0.6.6/checks-evidence/checks-summary.json` retains commands, original failures and verified log hashes.
- `.artifacts/release-v0.6.6/linux-evidence/linux-verification.json` records all three scenarios, artifact provenance, failed-attempt recovery and cleanup.
- `.artifacts/release-v0.6.6/final-review/` contains the independent artifact, feed, harness and final-delivery audits.

Native Windows GUI/installation/upgrade/profile retention, elevation, SmartScreen and Authenticode trust are untested; the Windows payload is unsigned. Linux tests use Docker Desktop amd64 emulation on macOS ARM64, not target hardware qualification. Real intranet/production and whole-repository/mobile suites are outside this local acceptance. Four opt-in Docker smoke tests remain explicitly skipped.

All task-owned disposable database/Redis, upgrade projects, volumes, networks and fresh-administrator container were removed after evidence capture. Private fixtures and backups stay outside delivery with mode 0600.

## Final gate

Independent review passed with the declared limits. All 22 delivered files are accounted for; the checksum manifest verifies 21 files and the documentation ZIP contains 13 safe, byte-identical files. No unresolved release blockers or known fixture-value leaks. The delivered checksum manifest SHA-256 is `16bb956f47141d967534f8575b65beed5e667abaf144cc2bd3e1aee95fcfec9d`.
