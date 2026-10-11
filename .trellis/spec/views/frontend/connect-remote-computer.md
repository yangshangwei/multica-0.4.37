# Add a computer: commands and connection confirmation

## 1. Scope / trigger

Read when changing `ConnectRemoteDialog`, daemon installation configuration,
runtime inventory parsing, or connection detection. Web and Desktop mount the
same dialog; current-device OS is not evidence of the target computer's OS.

## 2. Signatures

- `GET /api/config` exposes optional `cli_install_command` and
  `cli_install_commands: { windows?: string; macos?: string; linux?: string }`.
- `ApiClient.listRuntimes({ workspace_id, owner: "me" })` parses the response
  with `AgentRuntimeListSchema`; a malformed inventory rejects the query.
- `connectionCommands(platform, serverURL, appURL)` returns setup, configuration,
  interactive login and daemon-start strings, or `null` for unusable URLs.
- `computerBaseline(runtimes, userId, wsId)` and
  `newlyConnectedComputer(runtimes, baseline, userId, wsId)` own machine identity.
- Installation and address-help links use `DOCS_SLUGS.cli` and
  `DOCS_SLUGS.selfHostQuickstart`, covered by the bundled-document registry.

## 3. Contracts

Installation settings flow from `MULTICA_CLI_INSTALL_COMMAND_WINDOWS`, `_MACOS`
and `_LINUX` through Go config, `AppConfigSchema`, `AuthInitializer` and
`configStore.cliInstallCommands`. Preserve `MULTICA_CLI_INSTALL_COMMAND` for
older clients and as a Unix fallback. All four variables must reach Compose,
Helm and offline delivery. They are public commands, never credentials, and the
backend does not execute them. `setDaemonConfig` clears omitted old values.

An explicit system command wins. The legacy command applies to macOS/Linux only.
Public defaults apply only when every custom command is empty; a missing system
in an internal distribution shows the deployment-local CLI guide.

The first successful inventory fetched after opening is the baseline, including
offline computers. Cached data and failed reads cannot establish it. Scope to
the current user, workspace and local mode; compare runtime ID, daemon ID and
normalized device name. An existing computer's additional provider, replacement
daemon or ordinary reconnect is not a newly added computer. A candidate requires
online status and a reported computer name. Display actual reported system and
runtime names, never the selected system as a device fact.

`workspace_id` is an essential inventory scope fact and must be a nonempty
string, like the backend's `AgentRuntimeResponse`. Never default a missing or
malformed scope to `""`: filtering that row out would create an incomplete
baseline and misidentify it as new when the field recovers. Legacy display
fields (`custom_name`, `profile_id`, visibility) may still default safely.

WS registration/reconnection invalidates workspace queries; payloads never
confirm success. Poll while waiting. Query failure/browser offline displays an
interruption with retry while retaining the reliable baseline; it does not prove
the remote computer is offline. Remount session state when user/workspace changes.

## 4. Validation and error matrix

| Input / observation | Required result |
| --- | --- |
| No selected system | No install or connection commands; all tabs unselected |
| Both URLs genuinely unconfigured | Managed-cloud commands |
| One missing URL, malformed authority, credentials, control characters, loopback or wildcard host | No setup/configuration/token path; keep install and local address guide |
| Control-only URL with the other URL empty | Invalid address, not managed-cloud fallback |
| Apostrophe or shell metacharacters in a valid URL | One quoted argument; Unix apostrophe escaping or PowerShell doubled apostrophes |
| Malformed runtime list, including an invalid row | Query error, not an authoritative empty list |
| Genuine `[]` inventory | Valid empty baseline |
| Unknown registration payload or existing computer reconnect | Refresh and continue waiting |
| Clipboard failure | Visible manual-copy instruction; command remains selectable |
| Platform switch / dialog reopen | Clear copy feedback / reset system selection; preserve expanded help only during the same open session |

Reject controls before deciding that trimmed-empty URLs mean cloud. `URL()` repairs
inputs such as `https:/host`; require explicit HTTP(S) authority syntax first.
Apply loopback/unspecified IPv4 checks to normalized dotted-quad addresses, not
DNS prefixes: `127.api.example.com` and `0.packages.internal` are valid hostnames.
Future server enum strings must survive parsing and fail safely in consumers.

## 5. Good / base / bad cases

- Base: select Windows; copy the PowerShell installer and a single-line setup.
- Good: an internal Unix-only command keeps Windows on its installation-help
  state; valid new owned runtime data confirms a machine with its actual OS.
- Bad: infer Windows by replacing `.sh` in an arbitrary command, let a token
  command bypass an invalid URL, or treat any daemon event as successful setup.

## 6. Required tests

- `connect-remote-commands.test.ts`: canonical installer, URL and shell matrices.
- `connect-remote-machine.test.ts`: canonical identity, ownership, workspace,
  mode/status, provider additions and missing-name matrices.
- `connect-remote-dialog.test.tsx`: real shared Tabs/Dialog keyboard and focus,
  selection/reopen, copy feedback, separate configure/login/start, fresh baseline,
  query recovery and factual success actions. Do not duplicate helper matrices.
- Core schema/client/config/initializer tests: old and malformed config, resetting
  stale commands, malformed list rejection versus genuine empty inventories.
- Go config, Compose/Helm and offline tests: actual variable forwarding and
  regenerated deployment-local documentation.
- Production browser: both locales, narrow scrolling/overflow, real clipboard,
  stale-machine notifications, query failure/retry and actual success facts.

## 7. Wrong versus correct

Wrong: cast a `daemon:register` payload, save its ID and show success immediately.
Correct: invalidate the scoped inventory, compare parsed results with the locked
baseline and navigate using the resulting grouped machine ID.

Wrong: paste `multica login --token` and `multica daemon start` together.
Correct: copy configuration first, run interactive login separately, wait for
login to succeed, then copy the start command.
