# Detailed registration errors — verification

## Diagnosis

The API on `localhost:18573` was still running the pre-fix server (pid 21050, started 2026-10-01T02:04:52Z). A registration request using a numeric username and an intentionally invalid password returned `invalid_request` with the old leading-letter requirement. The UI's generic-code fallback hid the actual cause.

## Changes

- `packages/views/auth/password-error.ts`: localized message selection with explicit compatibility mapping for known older API validation messages; detailed username/name/password lengths and character restrictions. Modern error codes take precedence. Unknown bodies remain localized.
- `packages/views/auth/password-form.tsx`: replace the nested error-selection chain with the presentation helper; retain rate-limit timing and entered values.
- `packages/views/locales/{en,zh-Hans}/auth.json`: actionable field-specific messages with actual character counts, old-server username-rule guidance, connection and service failures.
- `packages/views/auth/password-error.test.ts` and `password-form.test.tsx`: canonical error matrix plus Chinese rendering and count interpolation.

## Verification

- Reproduced the vague old-server and short-password messages in two failing component tests before the fix.
- `pnpm --filter @multica/views test auth/password-error.test.ts auth/password-form.test.tsx locales/parity.test.ts`: 90 passed.
- `pnpm --filter @multica/views test auth/login-page.test.tsx`: 39 passed.
- `pnpm --filter @multica/views typecheck`: passed.
- `pnpm --filter @multica/views exec eslint auth/password-form.tsx auth/password-form.test.tsx auth/password-error.ts auth/password-error.test.ts`: passed.
- `git diff --check`: passed.
- Verified process cwd and environment ownership, then restarted only the API with `make down ARGS='--components api'` / `make up C=api ENV_FILE=/Users/artisan/.multica/dev/envs/check-20260928023904-13728/check.env`, adding the installed libpq tools to PATH. API health now reports pid 37064, started 2026-10-01T02:44:58Z; `/api/config` confirms `auth_mode=password`.
- Live invalid registration probes returned 400 with `invalid_password` for a numeric username plus a nine-character password, `invalid_username` for invalid characters, and `invalid_name` for a blank display name. These validation requests created no accounts.

## Environment note

The first plain `make up C=api` reused the registry ports but loaded the root environment before the manifest. Its health check passed, but an auth probe found password authentication disabled. Restarting with the explicit registry `ENV_FILE` restored the existing password-mode configuration. Future starts for this registered environment must pass that file; no startup-script changes were bundled into this UI fix.

## Limits

No browser/Electron end-to-end run was performed. Unknown backend failures without a recognized code/message still use a localized fallback rather than inventing a cause. No dependencies or database migrations were added; existing migrations were checked by the environment startup command.
