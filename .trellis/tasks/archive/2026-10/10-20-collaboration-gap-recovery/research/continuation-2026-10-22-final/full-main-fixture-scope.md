# Installed full Main fixture boundary

Existing task: `10-20-collaboration-gap-recovery`, S6/AC31. No task creation,
planning replacement, product edits, native IPC replacements or acceptance
promotion is performed by this fixture slice.

The existing `collaboration-account-electron.mjs` extracts production routing
functions and supplies named native service stubs. It proves those renderer and
IPC functions, while installed startup remains a separate evidence scope. The
new `collaboration-installed-main.mjs` launches the exact electron-builder Linux
arm64 executable with `_electron.launch({ executablePath })`. The original
`app.asar` provides the complete Main, preload, renderer, daemon manager, runtime
configuration loader and updater. The fixture observes `app.isPackaged`, app name,
version, default OS home, default userData, renderer URL and SHA-256 of executable,
ASAR and bundled CLI. It runs only the bundled CLI's version command directly.

Files in this implementer's ownership:

- `e2e/collaboration-installed-main.spec.ts`: production Web and installed Main
  use the same public control fixture and issue UUID. Web invites/transfers a
  dedicated fresh zero-workspace password actor, the installed password form
  logs in, controls B/A/S and the whole issue, rejects an unproven Resume,
  preserves waiting cancellation/history, transfers back and restarts the full
  installed app without seeding auth or replacing native APIs.
- `e2e/fixtures/collaboration-installed-main.ts`: host-side JSON-lines transport
  through the configured Docker image. One UUID-named container is owned by each
  instance. The repository and package mount read-only; named package volume is
  supported when host bind sharing is unavailable.
- `e2e/fixtures/collaboration-installed-main.mjs`: Node22/Xvfb controller. Its
  stdin carries passwords; actual login API produces the session. The controller
  does not return a password/token or expose raw native logs. Actual `/root`
  files provide runtime config, `autoStart:false` and `automaticUpdates:false`.
- `research/continuation-2026-10-22-final/full-main-fixture-*`: exact static
  command/results and this bounded handoff; root owns builds, Docker image,
  current E2E receipts, ledger/spec and commits.

Launch prerequisites remain explicit environment inputs:
`COLLABORATION_INSTALLED_IMAGE`, `COLLABORATION_INSTALLED_APP_VOLUME` or
`COLLABORATION_INSTALLED_APP_DIR`, public task API/password/managed configuration,
and matching production Web. Container endpoints are the existing API/Web ports
with `host.docker.internal`, and no server CORS/defaults are altered. The current
packaged file renderer intentionally uses its production `webSecurity:false`
policy. Root's package matches pinned Electron39.8.7 and current application bytes.

The remote `simulatedDaemon.permitPendingInput` comes from the existing public
control fixture. It genuinely registers/claims/permits an immutable input but
executes no provider and supplies no outcome or proof. The resulting unproven
server barrier is preserved. This fixture does not establish paid provider,
kernel containment, released old-client, installer, Windows or macOS evidence.
Production creation/execution/rollout gates retain their existing scope.

First focused E2E attempt stopped before native launch: invite returned200,
then immediate transfer observed an old collaboration revision and returned409.
The fixture now reloads at that explicit action boundary before opening transfer;
the expected200 remains. Canonical `CapabilitiesFor` also confirms that a former
handler who remains an effective participant retains `control_ai:true`, while
`transfer:false` after responsibility returns to creator. The test asserts that
real role distinction instead of treating all participant control as handler-only.

Second focused attempt ran the complete packaged Main: password form200,
authenticated account read200, targetB Stop/Resume200, commentA/B201, and a
waiting cancellation200. The runner's response predicate timed out because a
global sequence-only button selected A's lane while expecting B's command UUID.
Sequence1 is local to each lane. The corrected selector first resolves the exact
B resource row and its existing lane ancestor, then selects that lane's sequence1
button; the response still must name B's exact command UUID and return200.
Metadata confirms package39.8.7, `/root`, default userData and zero renderer errors.
The root's raw second result and sanitized attachment remain retained. No product
API or control behavior was changed in response to either fixture failure.

Static verification initially run:

- `node --check e2e/fixtures/collaboration-installed-main.mjs`: exit0.
- `pnpm exec playwright test e2e/collaboration-installed-main.spec.ts --list`:
  exit0, one registered test identity; listing is not executable acceptance.
- Scoped strict `tsc` for the new spec/TS adapter: exit0.
- Base ESLint at root with Node/browser fixture globals: three files, zero
  errors/warnings. An app-local ESLint invocation ignored paths outside its base;
  that ignored run is not counted as validation.
- `git diff --check` for the owned E2E files: exit0.

Execution is coordinated by root using `admin-test.py` and the task-owned package
volume. Current E2E results are recorded separately; no AC31 promotion is inferred
from successful static checks or the independent startup smoke.

## Current focused executable receipt

`full-main-current-third.stdout` records the sole installed full Main test as
passed: expected1, unexpected0, skipped0, flaky0; duration39.412293s, started
2026-10-22T13:34:01.393Z. The exact raw report SHA-256 is
`fc03372f3252b54432d3979835ed518b71ed73dab121a46efac17999cc7dccf3`.

The `installed-full-main-journey` attachment correlates original issue
`01a163d1-9ab4-7b5a-8041-a250dd2594cb` across Web and actual full Main. Actual
runtime is Linuxarm64 Electron39.8.7, packaged version
`0.6.5-collaboration.63c0beeb5`, default `/root` home and
`/root/.config/Multica` userData. ASAR SHA-256 is
`35690546d3148df7c702ae39a5589e684eeb379893e34e79b2438ada578b3ea5`;
bundled Linux CLI SHA-256 is
`d4f9091acf534ec4fff9f3110d063ac58ead160b4badedf0607a5b989edfc732` and its
actual version command returns `v0.6.5-100-g8ecb98883-dirty`. App and CLI version
strings are intentionally recorded separately from root's application byte
identity; the fixture does not relabel the historical snapshot HEAD.

Password form status200, Web→native transfer and native→Web transfer both use
the same issue/session, and current handler/capability differences follow the
actual role. Installed B target Stop/Resume, A unconfirmed Resume409, whole
Stop/partial Resume, S Stop/Resume and final whole Stop all have exact original
request keys. A's unproven run
`69c3b213-6c64-4aad-a63b-b62994dc224e` remains paused; remote protocol simulation
supplies no provider/outcome/proof. B waiting input
`04f39a6a-5086-4e89-99d1-fb9e8b497211` is manually cancelled by the installed
handler and its immutable record survives full package restart and Web reads.
The same ASAR/profile and one account tab restore after restart, native daemon
is stopped, renderer errors are empty, and all185 observed account requests
(15POST) have no ambient workspace header. POSTs advertise the actual packaged
version/Linux identity. No credential-bearing keys were found in the journey
attachment; auth/private logs remain outside evidence.

The final fresh static receipt in `full-main-fixture-static-results.json` records
syntax, strict/no-unused TypeScript, root base lint, test registration and owned
diff checks all exit0. Root still owns independent whole-clause review and
acceptance-matrix/spec/commit decisions. This executable slice adds current
installed Main evidence; it does not close AC32/43 or claim other OS installers.
