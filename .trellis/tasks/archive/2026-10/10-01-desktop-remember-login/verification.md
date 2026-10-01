# Remembered Desktop sessions

## Finding

The installed build already persists the issued session token via the shared auth store and restores it through `AuthInitializer` on startup. The desktop shell waits for authentication before choosing the login page, so a valid stored session never needs a second password submission. Closing the application does not call explicit logout.

Development Electron and `/Applications/Multica.app` use different profile directories. An earlier login made only in the development app does not automatically appear in the installed app. Once logged in to the current installed app, subsequent launches use its stable user-data directory.

## Changes

- `packages/core/platform/auth-initializer.test.tsx`: regression for password login, fresh-store restoration without an unauthenticated transition, password not persisted, explicit logout, and subsequent signed-out startup.
- `e2e/password-session-persistence.spec.ts`: real built renderer/preload in separate Electron processes sharing an isolated on-disk profile. Verifies one password submission across restart, session restoration, and revoked-token cleanup on a later restart.
- `e2e/fixtures/changelog-electron.cjs`: records auth-session reports for the integration assertion; keeps native daemon and agent services isolated.

No alternate token store or password replay mechanism was added because the existing implementation already fulfills the requested behavior.

## Verification

- Core auth initializer/store/password suites: 57 tests passed.
- `pnpm exec playwright test e2e/password-session-persistence.spec.ts --reporter=line`: passed (three Electron launches: password login, remembered-session restore, revoked-session rejection).
- Core typecheck and scoped ESLint: passed.
- Electron fixture `node --check`: passed.
- `git diff --check`: passed.

## Scope

The configured token lifetime uses the server default of 30 days. Explicit logout, revocation/password changes, and expiry require authentication again; transient network failures retain the token and recover. The Electron integration test uses a local simulated API and isolated native services, so it does not touch the user's current session or create a real account.
