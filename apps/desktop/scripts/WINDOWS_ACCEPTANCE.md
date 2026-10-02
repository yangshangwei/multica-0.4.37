# Windows candidate acceptance

Run the existing **Desktop Smoke Build** workflow with `windows_acceptance=true`
on the exact candidate branch. This selects one GitHub-hosted Windows x64 job;
the original Linux/multi-architecture smoke matrix remains the default.

The acceptance job:

1. Parses and fixture-tests the PowerShell lifecycle driver.
2. Executes sixteen named managed identity/transport tests natively and rejects
   missing tests, package failures and skipped parent or child cases. The ACL
   case inspects a real generated identity under a new temporary directory in
   the runner's user profile, checking broad Everyone/Users/Authenticated Users
   grants. It is not an exhaustive named-user or adversarial ACL audit.
   A separate twenty-run concurrency stress check guards Windows ticket replacement/deletion.
3. Builds the checked-out source with `package.mjs --win --x64 --publish never`.
   The job sets `MULTICA_DESKTOP_VERSION=0.5.3-rc.<run_number>.<run_attempt>`
   for both desktop and bundled CLI, and uses the same value in lifecycle checks.
   This is candidate metadata only; it does not create or publish a release tag.
   The selected previous version must have a lower numeric version.
4. Downloads the selected same-repository stable release (default `v0.5.1`)
   and validates the previous installer against its published SHA-256.
5. Installs the previous version, upgrades to the candidate, uninstalls,
   reinstalls the candidate and uninstalls again. It checks installed PE
   architecture, desktop/CLI versions and leftover registry/shortcut state.
6. Uploads reports even on failure, and uploads candidate installer/update
   artifacts only after successful acceptance.

The lifecycle driver refuses personal machines and persistent self-hosted
runners, existing Multica installations/processes/profiles and reparse-point
temporary roots. It installs under a unique RUNNER_TEMP directory using per-user
NSIS options. A failure leaves diagnostics for disposable-runner teardown.

`require_signed=true` requires valid Authenticode on both installers and the
installed desktop binaries. No signing secret is automatically loaded by this
workflow, so unsigned builds report that limitation and cannot pass strict
signed acceptance. The default records signature status without claiming trust.

Reports do not establish GUI launch, backend-connected workflows, existing user
state retention, automatic-update-feed behavior, offline certificate trust,
Windows ARM or ia32 acceptance. Those remain separate release gates. The Go
transport tests use loopback fixtures and do not run personal agent accounts or
models. Installer execution runs only the bundled CLI's `version` command.

Local macOS checks can validate YAML, Node contracts and cross-compile Windows
Go binaries, but native results must come from the Windows job. Record the
workflow run ID, head SHA, candidate version and artifact hashes in the S07
report; never interpret compilation or workflow preparation as Windows passes.

## Opt-in installed business and update acceptance

Set both `windows_acceptance=true` and `windows_business_acceptance=true`.
This prepares a disposable runner-local PostgreSQL cluster and real Go backend.
It uses the hash-pinned accepted `0.5.2-rc.11.1` installer as a feature-capable
baseline: public `v0.5.1` predates password login and managed identity.
The installed baseline logs in and enrolls; after upgrade, the same session,
configuration, identity, completed history and queued task are checked. Candidate
execution, administrative cancellation and transport reconnect use the real
backend/daemon with a contained synthetic provider, never a personal model account.

The subsequent updater probe invokes the installed client's real updater against
a loopback feed and checks the installed version plus retained settings/browser
marker. It does not bypass signature policy or establish signed/offline trust.
Only sanitized named JSON reports are uploaded. The business state directory
contains private credentials and must never be uploaded. Backend cleanup is an
always-run step; final runner disposal handles failed partial preparation.
