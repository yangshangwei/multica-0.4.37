# Quality Guidelines

> Code quality standards for frontend development.

---

## Overview

<!--
Document your project's quality standards here.

Questions to answer:
- What patterns are forbidden?
- What linting rules do you enforce?
- What are your testing requirements?
- What code review standards apply?
-->

(To be filled by the team)

---

## Forbidden Patterns

<!-- Patterns that should never be used and why -->

(To be filled by the team)

---

## Required Patterns

<!-- Patterns that must always be used -->

(To be filled by the team)

---

## Testing Requirements

<!-- What level of testing is expected -->

(To be filled by the team)

---

## Code Review Checklist

<!-- What reviewers should check -->

(To be filled by the team)

---

## Desktop Shell Entry Invariants

### `onboarded_at` is the only door into the dashboard

`apps/desktop/src/renderer/src/App.tsx` routes on two signals: `onboarded_at != null`
and workspace count. A user with a workspace but no `onboarded_at` is sent to the
onboarding overlay, not the dashboard — `CreateWorkspace` deliberately does not
stamp the flag (only `CompleteOnboarding` and `AcceptInvitation` do).

**Consequence for any new path that provisions a user server-side** (device auth,
SSO, imports, seeded demo accounts): stamp `onboarded_at` in the same transaction,
or the client will hold that user behind an onboarding flow it has nothing to
onboard. `MarkUserOnboarded` is idempotent (`COALESCE(onboarded_at, now())`), so
calling it is safe on a returning user.

Verified by: `server/internal/handler/auth_device.go` (device provisioning),
`TestDeviceLoginProvisionsIdentityAndWorkspace` asserts the response carries a
non-null `onboarded_at` for exactly this reason.

### A workspace is not usable without its status catalog

Creating a `workspace` row is not enough: `issue_status` carries the 7 built-ins per
workspace (MUL-6243) and an issue cannot be created before its status resolves.
Any code that creates a workspace outside `CreateWorkspace` must call
`issuestatus.Ensure` in the same transaction. The seed is idempotent and
concurrency-safe, so calling it for a pre-existing workspace is a cheap self-heal.

## Isolated Electron browser acceptance

### Password migration must refresh the daemon's own credential

Password setup/change preserves the user ID but revokes existing PATs. A matching
`.desktop-user-id` therefore does not prove a cached daemon PAT is still valid.
`syncToken` probes that exact cached token with `GET /api/me`; only HTTP 401
triggers an automatic `POST /api/tokens` using the current renderer session.
Keep the cached PAT on 2xx, 403, 5xx, and network failures. Never treat an
inconclusive probe as permission to rotate credentials or log out the user.

Manual `daemon:reauthenticate` requests a fresh PAT before replacing the profile.
Mint failure must preserve the existing token, owner sidecar and running process.
When a credential changes for the same user, restart through the existing
lifecycle gate so the daemon loads it. Async probes and queued credential work
must not cross a logout, server change or newer session.

A CLI startup failure can precede the ten-second polling grace period. Probe on
that failure and expose `auth_expired` only for a definitive rejection. Keep the
Settings → Daemon sign-in action visible even when stopped; a lingering
`starting` state must not disable manual recovery. The main lifecycle gate
serializes that action with an in-flight start.

Regressions: `daemon-manager-auth.test.ts` exercises the real manager and IPC
handlers with isolated disk/network/process fixtures; `daemon-settings-tab.test.tsx`
checks the persistent action, pending state and external-daemon boundary.

### Desktop daemon subprocesses use the selected server profile

Start, stop, and local runtime probes must use `desktopSpawnEnv()` from
`apps/desktop/src/main/daemon-manager.ts`. The Desktop-owned profile determines
the server; do not inherit `MULTICA_SERVER_URL`, which the CLI resolves ahead of
that profile. A Makefile `include` followed by `export` can retain literal quotes
around an environment value, failing URL parsing before CLI discovery. Even a
valid inherited URL can silently select the wrong server. Preserve the corrected
PATH and provider-specific executable overrides.

The regression matrix lives in `daemon-manager-managed-recovery.test.ts`.
When reproducing startup issues, inspect current process/log timestamps and the
profile health endpoint. Restart with the original `DESKTOP_RENDERER_PORT` and
`DESKTOP_APP_SUFFIX`; both affect the renderer session and its stored login.

### Served renderer configuration

Electron fixtures using a served renderer must explicitly configure both the
local API URL and its isolated `DATABASE_URL`. Check that the API origin used
by `TestApiClient` matches the native runtime-config fixture before creating
accounts. The renderer origin must also be present in that API's CORS allowlist.
An OPTIONS 200 without `Access-Control-Allow-Origin` is a rejected preflight,
not successful authentication; it can leave the shell on its loading/login
view. Serve built renderer assets on the task environment's allowed frontend
port after stopping its Web listener, or configure an origin in that isolated
environment. Do not weaken application CORS to make a test harness pass.

`e2e/retained-languages-desktop.spec.ts` checks these prerequisites and runs the
real renderer/preload with native agent services isolated. It preserves the
profile between process launches to test language persistence after restart.
