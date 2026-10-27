# Desktop daemon diagnostics

## 1. Scope / trigger

Apply when changing device reconnect, managed installation challenge validation,
CLI bootstrap, daemon status IPC or Settings → Daemon. CLI setup failure and
registration failure are separate observations: a challenge rejection does not
prove a CLI download failed. Preserve the identity rules in
[managed installations](../../server/managed-installations.md).

## 2. Signatures

`apps/desktop/src/shared/daemon-types.ts` owns the IPC contract:

```ts
interface DaemonDiagnostic {
  code: DaemonDiagnosticCode;
  stage: "cli_setup" | "registration" | "binding" | "credentials" | "restart";
  occurredAt: number;
  httpStatus?: number;
  fields?: ChallengeMismatchField[];
}
interface DaemonDiagnostics {
  cli?: DaemonDiagnostic;
  connection?: DaemonDiagnostic;
}
// DaemonStatus.diagnostics?: DaemonDiagnostics
type DaemonReauthResult =
  | { ok: true }
  | { ok: false; reason: "session_invalid" }
  | { ok: false; reason: "transient"; diagnostic?: DaemonDiagnostic };
```

`DaemonDiagnosticCode` is a finite union of challenge, CLI, request and operation
failure categories. `ChallengeMismatchField` contains labels such as `account`,
`deployment`, `workspace` and `binding_epoch`, never expected/received values.
Main's `DaemonDiagnosticError` carries typed source evidence;
`daemonDiagnostic(error, stage)` projects it into safe data.

The internal `transitionManagedDaemon` options include optional
`onHandoffError(error: unknown): void`. It reports handoff preparation failure
without adding raw errors to `ManagedTransitionResult` or IPC.

`daemonAPI.getCliPath(): Promise<string | null>` returns the CLI binary selected
by Main's existing resolver, or null when none is usable. The
`daemon:get-cli-path` IPC replaces the former boolean installation check;
availability and the displayed path must come from this same result.

## 3. Contracts

- Main owns independent ephemeral CLI and connection diagnostics. Status events
  and `getStatus` must expose the same snapshot. Polling or Settings remount
  cannot erase an unresolved diagnosis.
- A diagnosis-only status update retains the complete last observed runtime
  metadata, especially `externallyManaged`, profile, endpoint and PID. Sending
  only `{ state }` would briefly remove external-management controls/guards.
- Successful CLI resolution clears CLI failure; successful reconnect clears
  connection failure. Starting a retry alone does not prove recovery.
- Account/server reset clears the old connection diagnosis. Use the existing
  managed generation/profile/target fences before publishing async failures;
  superseded work cannot contaminate the new context.
- Invalidate cached runtime metadata when the selected profile is invalidated,
  before reset emits a status. Fence adoption of awaited health results by
  generation, target and profile too: retaining the latest snapshot alone is
  insufficient if a late response can overwrite it with old-server metadata.
- Preserve typed failures through `configureManagedSession`; replacing them
  with a generic retained `managedSetupError` loses evidence on later retries.
- Transition handoff preparation reports through `onHandoffError` only for the
  current transition. Manager additionally checks the diagnostic attempt before
  publishing a binding failure. Keep the original exception and pending/busy
  result; reporting failures must not alter process start/drain decisions.
- Only the existing definitive session rejection authorizes logout. Transient
  validation/network/IPC failures preserve sign-in and get safe localized text.
- Diagnostics may include finite codes, stages, field labels, occurrence time
  and integer HTTP status. Do not forward arbitrary exception messages, raw
  response bodies, nonces, signed payloads, tokens, proofs or private keys.
- The renderer checks that a diagnostic code is a string before looking it up.
  Unknown or malformed codes/stages/fields use the safe finite fallback; do not
  coerce an arbitrary object into a translation key.
- Challenge classification must preserve every rejection guard and exact
  original signature bytes. Validate structure/integrity before attributing a
  failure to scope or time. Do not loosen expiry limits or downgrade auth.
- The server currently issues a 120-second challenge. Desktop accepts its
  expiry within `(local_now, local_now + 300]`. Expiry alone does not prove which
  clock is wrong; latency also matters. Suggest checking device/server time and
  obtaining a fresh challenge instead of compensating in the validator.
- CLI failure stage comes from bootstrap/resolution evidence; successful
  fallback resolves setup failure. Do not change candidate ordering or bypass
  checksum verification as part of improving diagnostics.
- Settings shows the actual selected executable path beneath CLI status. Keep
  spaces, non-ASCII characters and the filename intact, with selectable text
  that wraps at narrow widths. Do not infer a default installation directory;
  missing and failed lookups must not display an invented path.
- Render persistent English/Chinese summaries and ordered steps in Settings.
  Long explanations wrap. A short toast reuses the same diagnosis mapping.
  Windows guidance must not recommend deleting identity files or disabling
  security software. Registration failure can precede daemon log creation.

## 4. Validation and error matrix

| Observation | Required result |
| --- | --- |
| Valid shared challenge fixture | Original signature unchanged |
| Invalid canonical/envelope/body/hash data | Invalid-challenge diagnosis; no signature |
| Valid data with wrong scope | Safe mismatch field labels; no signature |
| Expired or excessive-future expiry | Distinct time diagnosis; no signature |
| All CLI candidates unusable | CLI setup failure with observed stage |
| CLI unavailable and reconnect fails | Both diagnoses remain visible |
| Poll / Settings remount | Retain unresolved diagnoses |
| Corresponding action succeeds | Clear that category only |
| Account/server changes / late old response | No previous-context connection diagnosis |
| Unknown error or IPC exception | Safe fallback, no raw text and no logout |

## 5. Good / base / bad cases

- Good: Settings shows a CLI download error and a registration account mismatch
  independently, each with relevant steps. The running state remains truthful.
- Base: an older/unknown diagnostic is absent. Show a generic safe setup or
  reconnect explanation without inventing a network cause.
- Bad: every challenge failure says "installation failed", retries discard the
  explanation, or the UI offers identity deletion or weaker validation.

## 6. Tests required

- Challenge tests: shared Go/Node fixture, each scope guard, malformed data,
  integrity-before-time precedence and time-window boundaries, including
  accepted expiry at `now + 1` and `now + 300` seconds.
- CLI tests: synthetic download/checksum/extract/install/validation failures,
  fallback success and no execution of user-installed agent CLIs.
- Manager tests: separate categories, read/push consistency, persistence,
  success clearing, context reset, late-result fencing and retention of
  external-management/runtime metadata on diagnosis-only status pushes.
- VM-based manager fixtures must provide `electron.app.getPath("home")` from
  their existing temporary root, for example `getPath: () => root`. Mocking
  `os.homedir()` alone no longer isolates managed sessions or preferences.
  An omitted `getPath` throws before credential probes and produces secondary
  health/restart expectation failures. Keep the production Electron home
  boundary and the original assertions; update the fixture's Electron double.
- Transition tests: current failures report for stopped, same-scope and
  scope-switch paths; no start/drain follows a failed preparation; superseded
  failures do not report. Manager tests prove background refresh retains the
  projected binding diagnosis without exposing the original error.
- Renderer tests: existing logout semantics, localized safe feedback, both
  categories, remount behavior and existing disabled/external-management guards.
- Isolated visual fixture: both locales, themes, narrow width and wrapping.
  Browser mocks prove presentation only; they do not establish native Windows
  ACL/antivirus behavior or the affected user's live server configuration.

## 7. Wrong vs correct

```ts
// Wrong: arbitrary source text may contain credentials and gives no stable category.
return { ok: false, reason: "transient", message: String(error) };

// Correct: project only explicit diagnostic data and translate it in the renderer.
return { ok: false, reason: "transient", diagnostic: daemonDiagnostic(error, "registration") };
```
