# Technical design

## Boundary

Preserve failure evidence at the source and carry an allowlisted diagnostic through existing daemon IPC into Settings. The current gap is missing cause/stage information, not missing permission to sign. Keep all authorization checks and CLI resolution behavior.

## Data flow and contract

1. `ManagedInstallation.signChallenge` produces typed validation failures. CLI bootstrap/resolution preserves the observed failure stage.
2. Daemon manager owns separate ephemeral latest CLI and latest connection diagnostics and adds occurrence time.
3. Existing `DaemonStatus` and reauth results expose optional structured diagnostics through preload.
4. A renderer mapping converts codes/field labels to localized summary and steps. Settings retains both categories, while reconnect uses a short localized toast.

Proposed shared shape in `apps/desktop/src/shared/daemon-types.ts`:

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
```

Use finite unions for observed categories and mismatch labels. No raw message, arbitrary details, payload, token or stack enters this contract. Keep `ok`/`reason` on the reauth result; consolidate its existing duplicate definitions in main/preload when adding the diagnostic field. Missing/unknown diagnostics receive a safe fallback, preserving existing callers.

## Challenge classification

Split the combined condition in `managed-installation.ts` into focused validation groups without dropping conditions. Decode/canonical structure checks precede field use. Inconsistent envelope, body or hash must not be mislabeled as clock skew. Only compare timestamps after validating decimal syntax.

- `expires_at <= now`: expired; possible clock ahead or delayed request.
- `expires_at > now + 300`: outside the allowed time window; possible clock behind or server clock/configuration problem.
- Scope mismatch: safe labels for purpose/request, account/auth version, deployment/organization, installation/workspace/daemon/epoch and key fingerprint, without values.
- Invalid or inconsistent challenge: canonical data, envelope, body and digest violations.

Retain exact original signing bytes, the current TTL window, ownership guards and no-downgrade behavior. Do not compensate for clock skew or claim the UI can prove its source from expiry alone.

## CLI setup

`cli-bootstrap.ts` owns download/checksum/extract/install stages; `resolveCliBinary` owns execution validation and fallback selection. Record a failed diagnosis only after all candidates fail; a usable fallback resolves setup failure. Preserve bundled/managed/downloaded/PATH order and existing singleflight semantics. Prefer source-typed errors over parsing arbitrary text with regexes.

`cli_not_found` means no usable CLI was resolved. A registration error must not replace this state. Where no cause is recorded, explain that the CLI is missing or unusable without asserting a download failure.

## Lifecycle

Use existing generation/profile/server guards to fence connection diagnostics. Status events and `getStatus` share one snapshot. Polling cannot erase a failure merely because the process is stopped; a corresponding successful action clears it. Context reset removes old connection diagnostics and superseded callbacks cannot re-publish them. CLI failure remains independently visible until resolved. Keep these values in Main memory, not Zustand or disk.

Preserve the structured cause through `configureManagedSession`'s catch, which currently retains only a generic `managedSetupError`, so subsequent start/retry/status paths keep the original diagnosis. Retain `enroll` versus `bind` provenance where known. Existing safe HTTP status can be projected without exposing response text; parsing new backend error-code fields is not required for this scope.

Review identified an additional cause-loss boundary: `transitionManagedDaemon`
catches handoff preparation failures and returns the existing `switching` state.
Add an optional internal `onHandoffError(error: unknown): void` callback around
its three handoff preparation paths. Preserve the same transition result,
busy/switching behavior and original exception flow. The callback only reports
for the current transition; manager additionally checks its diagnostic attempt
before projecting/publishing the safe binding diagnosis. Raw errors stay in Main
and are never added to the transition result or exposed through IPC. This is a
bounded internal propagation fix for the approved binding diagnostics.

Only definitive session rejection keeps the existing logout behavior. Unexpected IPC failures become safe localized feedback; raw exceptions must not be shown. Do not introduce a second credential retry loop.

## Settings presentation

Reuse Settings cards, semantic tokens and existing locales. Display distinct CLI setup/device reconnect summaries with ordered next steps next to the diagnostics area. Explanations wrap instead of using the existing one-line truncated row. Reuse the mapping for toast and settings, using the established i18n facility or a typed caller translation function; do not create a parallel i18n instance.

Windows steps check time synchronization, selected server/account, relevant download network/proxy, architecture, protection history and filesystem permissions. Never advise deleting `.multica/management`, bypassing checksums or disabling antivirus. Registration fails in Main before daemon startup, so daemon logs may not contain it; avoid claiming otherwise.

## Expected changes

| Files | Purpose |
| --- | --- |
| `main/managed-installation.ts` and test | Categorize rejection at the authoritative validation boundary |
| `main/cli-bootstrap.ts` and focused test | Preserve observed CLI setup stages |
| `main/daemon-manager.ts` and auth/recovery tests | Independent diagnostics, persistence, clearing and generation guards |
| `main/managed-transition.ts` and test | Preserve background handoff failure evidence through an internal guarded callback |
| `shared/daemon-types.ts`, optional focused diagnostic helper/test | Finite shared IPC contract and safe projection |
| `preload/index.ts`, `preload/index.d.ts` | Consistent result/status exposure |
| `renderer/src/platform/daemon-reauth.ts` and test | Safe localized feedback preserving logout semantics |
| `renderer/src/components/daemon-settings-tab.tsx` and test | Persistent diagnosis and initial steps |
| `renderer/src/components/daemon-runtime-card.tsx` if needed | Preserve the other reconnect caller with localized feedback |
| `packages/views/locales/{en,zh-Hans}/settings.json` | Desktop-only bilingual strings |
| Relevant desktop/managed installation spec | Record the verified contract |

Paths above are relative to `apps/desktop/src/` unless otherwise specified. A focused extraction is allowed for duplicate result types or a reused diagnostic mapping only; lock behavior with regressions first. No unrelated shared UI, routing or server API changes.

## Compatibility, verification and rollback

No dependency, schema migration, persisted configuration or deployment change. Installed client/server protocol bytes remain unchanged. Use the existing shared Go/Node signature fixture, validator rejection matrix, manager lifecycle tests and focused renderer wiring tests. Mock subprocesses; do not execute user-installed CLIs. A normal code revert rolls back the change.

Native Windows execution, antivirus/ACL behavior and the user's server remain unverified until actually exercised. A simulated Windows test or macOS run does not prove those outcomes.
