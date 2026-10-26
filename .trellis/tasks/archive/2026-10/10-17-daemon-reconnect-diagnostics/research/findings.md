# Investigation findings

## Source and limits

`apps/desktop/src/main/managed-installation.ts:214-253` signs challenges. Its rejection at line 240 combines protocol, UUID/decimal syntax, nonce length, purpose, deployment/organization, user/auth version, installation/workspace/daemon/epoch, fingerprint, method/path, digest, exact body payload, envelope consistency and expiry.

This is a Desktop Main identity check, not proof that the Windows installer failed. No Windows-specific scope calculation defect has been confirmed. The affected machine, its logs and live server have not been inspected.

## Reproduced time-window case

The server challenge TTL is 120 seconds (`server/internal/service/managed_installation.go:198`). Desktop accepts expiry only within `(local_now, local_now + 300]`. Read-only execution using the actual `ManagedInstallation.signChallenge` and repository Node/Go fixture produced:

| Desktop clock relative to server | Result |
| --- | --- |
| 181 seconds behind | Original generic scope error |
| 180 seconds behind | Accepted |
| Equal | Accepted |
| 119 seconds ahead | Accepted |
| 120 seconds ahead | Original generic scope error |

Network delay changes the margins. This proves a possible trigger, not the user's root cause. UI wording should say expired/outside allowed time range and suggest checking device/server time, without claiming which clock is wrong.

## Reconnect flow

Settings calls `renderer/src/platform/daemon-reauth.ts`, which reads user/session and invokes `daemon:reauthenticate`. Main calls `syncToken(..., true)` before restart. Actual session rejection returns `session_invalid`; other failures return a generic transient message. Renderer keeps transient failures signed in but uses an English raw-error toast.

`managed-session.ts` requests bounded challenge JSON, constructs expected scope from the current session, deployment and exact request body, then invokes `signChallenge` for enrollment/binding. All signature and ownership checks must remain intact. The full flow fetches config, validates the JWT/PAT identities via `/api/me`, loads the per-deployment installation, enrolls, then prepares per-workspace binding before restart. `configureManagedSession` currently replaces its retained setup error with a generic message, so source classification alone will not fix later status feedback.

`win32 → windows` is already mapped and covered by `daemon-manager-managed-recovery.test.ts`; no missing Windows OS mapping was found. Windows path/ACL/lock errors normally originate earlier and use other messages. Multiple backend instances behind one selected address should share deployment identity and correct time; this is a support check for a persistent deployment mismatch, not a confirmed configuration issue.

## CLI is a separate failure

`daemon-manager.ts:632` resolves bundled, managed/downloaded and PATH candidates and validates execution. `cli-bootstrap.ts` fetches GitHub checksums/assets, verifies them, extracts with system `tar`, and installs the binary. Windows 10+ includes `tar.exe`; actual missing tools, antivirus blocking or ACL failures require observed evidence.

`bootstrapCli` at line 1519 sets `cli_not_found` only when no usable CLI resolves. `fetchHealth` returns early for that state. The scope exception does not directly set it. Both conditions can coexist, and Settings currently shows neither underlying failed step.

## Existing regression owners

- `main/managed-installation.test.ts`: shared Go signature fixture and rejection tests.
- `main/managed-session.test.ts`: enrollment/binding flow.
- `main/daemon-manager-auth.test.ts`: credential preservation and reauth.
- `main/daemon-manager-managed-recovery.test.ts`: managed lifecycle/generation.
- `renderer/src/platform/daemon-reauth.test.ts`: logout versus transient failures.
- `renderer/src/components/daemon-settings-tab.test.tsx`: action guards and localization.
- `renderer/src/components/daemon-runtime-card.test.tsx`: second reconnect caller.

These paths are relative to `apps/desktop/src/`. Specs: `server/managed-installations.md`, `desktop/frontend/connection-recovery.md`, and `views/frontend/settings-recovery.md` under `.trellis/spec/`.

## Initial Windows diagnostic steps

1. Windows Settings → Time & language → Date & time: verify correct date/time and synchronize the clock. For a domain-managed machine, use the organization's time service; persistent failures require checking server time too.
2. Verify Settings → Server connection and the signed-in account target the intended deployment. Retry after switching completes to obtain a fresh challenge.
3. Investigate CLI setup separately using its observed stage: runtime availability, download network/proxy, architecture, protection history and directory permissions.
4. Record category/phase/time, client version and selected server for support. Do not share tokens, signed payloads or `.multica/management` identity files, and do not delete identity as a routine recovery step.

## Native evidence boundary

The investigation ran on macOS with fixture data. It verifies source behavior and the clock-trigger case, not native Windows installation, ACLs, antivirus behavior or the user's server configuration.
