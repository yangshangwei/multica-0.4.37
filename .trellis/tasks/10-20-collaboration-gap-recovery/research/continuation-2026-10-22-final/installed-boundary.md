# Installed-client evidence boundary — 2026-10-22

This is the existing S6 AC31/32 lane. It creates no task or replacement plan.
Only `installed-*` evidence and the focused `e2e/collaboration-installed-clients.spec.ts`
with its local fixture are owned here. Product code, gates, shared launchers,
acceptance ledger, specs and commits remain coordinator-owned.

The next bounded behavior check executes the already packaged `/app/multica`
from cached OCI image
`sha256:8d5ede4df2fa4b66d499803b95428b729fcb6ba753b3800e9b931b968ee16104`.
Its current network-free version receipt is `v0.6.7`, commit
`9caea35808afc8d05fb6135bfc74d4bfb0b45cf1`, Go `1.26.9`, Linux/amd64.
The original delivery record explicitly says local offline artifacts and no
GitHub publication; published-registry trust is not established.

The existing public control fixture supplies actors, issue, resource consents,
paused commands and one server-issued Claim/Permit with withheld proof. The
test executes real old CLI status/comment/rerun requests while paused, after
Stop remains unconfirmed and after the author is removed, compares exact
server state/history and keeps a successful ordinary command as a control.
No provider or daemon process, synthetic proof or live provider account is
needed. These are additional old packaged-CLI requests; a complete installed
Desktop/daemon compatibility matrix remains separate.

Containment uses `docker create --network none --read-only --cap-drop ALL
--security-opt no-new-privileges`, an explicit fixture relay executable copied into the task-owned container
before start and an ephemeral private Multica config root. A standard-library relay exposes
only container loopback HTTP and carries requests over Docker stdin/stdout.
The host accepts only the exact expected legacy paths and forwards to the
coordinator's loopback task API. Credentials pass through private stdin and
child environment only; they never appear in arguments, evidence or a trace.
Neither host `HOME` nor `CODEX_HOME` is reassigned. No dependency is installed,
no user config is accessed and no image entrypoint/upgrade command runs.

## Full Main requirements found

- `runtime-config-loader.ts:85` uses `app.getPath("home")/.multica/desktop.json`.
- `daemon-manager.ts:64` uses `os.homedir()/.multica/desktop_prefs.json`; default
  `autoStart` is true. `setupDaemonManager` also performs real CLI bootstrap.
- `daemon-profile.ts:52` uses `os.homedir()/.multica/profiles/<profile>`.
- Production Main pins the name and acquires the instance lock at startup;
  `--user-data-dir` can isolate that lock/updater/device data, but cannot change
  the three home-rooted paths above.
- The current Go CLI supports `MULTICA_TASK_CONFIG_ROOT`, including safe named
  profiles. Desktop does not consume it for its own paths; passing it only to
  the CLI would split Desktop and CLI state and is not an isolation solution.

The local `/Applications/Multica.app` is an adhoc-signed dirty build
`0.5.2-86-g711569e04-dirty`, not proven released bytes. It has not been launched.
The cached Linux AppImage `dist/release/v0.5.3/...AppImage` likewise has a local
source-plus-patch origin. A safe current full Main run requires an actual
disposable OS user/container with its default home isolated, production
packaged Main/preload/renderer, the matching bundled CLI and actual settings
that disable daemon/update auto-start. No service stubs or path mocks can
substitute for this evidence. The coordinator owns that environment/build.

`installed-inventory.json` records current source/artifact hashes, exact image
version and the above distinctions. AC31/32 are not promoted by this inventory.

## Planned verification commands for this bounded slice

Compile the standard-library-only relay with `GOOS=linux GOARCH=amd64
CGO_ENABLED=0 go build -o <task-temp>/installed-cli-relay
e2e/fixtures/collaboration-installed-cli-relay.go`.

Run the focused Playwright case with the coordinator's existing admin test
wrapper and explicit `COLLABORATION_INSTALLED_CLI_RELAY` and image digest. Check
the two focused TypeScript files with strict tsc and repository ESLint, and run
current canonical legacy/CLI/recovery race adjacency. Save every attempted
command/result rather than infer installed/provider/full-product success.
