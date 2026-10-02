# Windows x64 candidate acceptance

**Passed on 2026-10-02:** native Windows x64 candidate installation, upgrade,
uninstall and reinstall. The candidate is unsigned; S07 is not release-complete.

## Scope and provenance

The approved scope is a dedicated CI branch and disposable GitHub-hosted Windows
x64 acceptance. No main merge, release tag, public release or production deployment
is part of this run. macOS orchestrates the build but is not the Windows runtime.

- Candidate source: `0fb5f5e6b3848568ae21e2108264917ad6206f6c`.
- Branch: `ci/platform-admin-windows-20261002`.
- Run: https://github.com/yangshangwei/multica-0.4.37/actions/runs/36996436086
- Upgrade baseline: same-repository `v0.5.1` installer, verified against its
  published SHA-256 before execution.
- Candidate version: `0.5.2-rc.11.1`, shared by desktop and CLI.

## Native result

The lifecycle ran from `2026-10-02T10:44:59.7881153Z` to
`2026-10-02T10:45:53.0840852Z` (53.296 seconds).

| Step | Result |
| --- | --- |
| Install previous 0.5.1 | Passed |
| Upgrade to 0.5.2-rc.11.1 | Passed |
| Uninstall and check payload/registration/shortcuts | Passed |
| Reinstall candidate | Passed |
| Final uninstall and cleanup checks | Passed |

Both candidate CLI invocations reported `windows/amd64`, version
`0.5.2-rc.11.1`, commit `0fb5f5e6`. The installed desktop and both installers
reported Authenticode `NotSigned`.

Windows Desktop lock tests passed 19 tests. All 16 named native Go tests passed;
raw JSONL independently confirmed 20 concurrent identity stress passes with no
failure or skipped events. The skipped default desktop matrix is intentional:
this dispatch selected the dedicated Windows acceptance job.

Raw evidence and candidate files are preserved under
`.omx/reports/platform-admin/windows-candidate/run-36996436086/`.
A portable summary is tracked in `evidence/windows-candidate-summary.json`.

- Installer: `multica-desktop-0.5.2-rc.11.1-windows-x64.exe`, 177,841,317 bytes.
- Installer SHA-256: `f0b5a5eeded51fa7d1336bd2b46975a0c9ddc6d2292cebba457fd0b6e090fd47`.
- Downloaded artifact ZIP SHA-256 matches GitHub's artifact digest:
  `f5fd6e61c2c8728f609f25e2ca3c0237e08cf31bf20b19dfcd77ea528dad83aa`.
- Local installer SHA-256 independently matches the native report. Installer,
  blockmap and generated `latest.yml` are retained together; metadata was not
  published to an update server.

Primary changed files are `.github/workflows/desktop-smoke.yml`, Desktop
`scripts/accept-windows-installer.ps1`, `scripts/verify-go-test-events.mjs`,
`scripts/package.mjs`, `scripts/bundle-cli.mjs`, Main `managed-lock.ts`,
Go daemon `managed_lock*.go` / `managed_identity.go`, and their regression tests.
Version validation is shared instead of duplicated between packaging entry points.

## Failures retained and corrections

| Run | Finding and correction |
| --- | --- |
| 36980852796 | Windows PowerShell inherited the incompatible PS7 module path; set the child module path before ACL inspection. |
| 36981573300 | Concurrent ticket readers lacked Windows delete sharing; use READ/WRITE/DELETE sharing and preserve long paths. |
| 36983743805 | Atomic replacement still failed while a reader held the destination; retry the same temporary-file rename for at most one second without unlinking the owner. |
| 36987171833 | Two tests incorrectly simulated POSIX permissions with Windows mode bits; correct only the fixture. |
| 36988344442 | Directory enumeration exposed delete-pending tickets; retry the whole scan and discard partial results, failing closed on persistent denial. |
| 36990530193 | Native and stress tests passed but remote Git tags produced `0.5.1-142-g3a1f9d6f`, not newer than stable `0.5.1`; share a validated explicit candidate override instead of publishing a tag. Installation did not start. |

Ticket fixes preserve atomic publication and exclusion. Retry does not re-enter
the protected identity operation or skip unreadable owners. No new dependency or
separate lock implementation was added. Windows tests cover blocked publication,
timeout preservation, long paths, delete-pending scans and persistent denial.

## Local verification

After the version correction, Desktop Vitest passed 84 files / 949 tests. The
focused package, bundle and workflow suite passed 67 tests; Desktop Node typecheck,
focused ESLint and diff whitespace checks passed. These local checks do not prove
Windows installation. Earlier Mac Windows cross-compiles are supplementary only.

## Boundaries

Installer execution runs only the installed CLI `version --output json`; it does
not launch Electron or a managed daemon. Desktop PE numeric ProductVersion and
x64 architecture are checked; the CLI reports the full candidate version. The
report does not claim exact prerelease inspection inside installed app.asar.

The ACL test checks broad Everyone/Users/Authenticated Users grants on an actual
Go-generated identity under a temporary user-profile directory, not an exhaustive
named-user/adversarial audit. Native transport tests use loopback fixtures.

GUI, actual backend-connected flows, existing user-state retention, update-feed
upgrade, valid signing and offline certificate trust remain separate release gates.
`require_signed=false` records signatures without claiming signed acceptance.
S07 and production release remain open.
