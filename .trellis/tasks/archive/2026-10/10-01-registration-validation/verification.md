# Verification

## Result

Registration errors use the selected interface language. Usernames accept digits at every position and preserve leading zeros. Guidance recommends an employee ID and explicitly allows numeric usernames.

## Changed files

- `packages/views/auth/password-form.tsx`: translate validation codes and use a localized fallback instead of raw error messages.
- `packages/views/locales/{en,zh-Hans}/auth.json`: employee-ID guidance and validation/fallback messages.
- `packages/views/auth/password-form.test.tsx`: numeric submission and Chinese error regressions.
- `server/internal/auth/password.go`: remove the first-character exception from username validation.
- `server/internal/auth/password_test.go`: numeric/leading-zero/boundary normalization cases and invalid-character coverage.
- `server/internal/handler/auth_password.go`: specific username/name/password validation codes.
- `server/internal/handler/auth_password_test.go`: numeric account registration/login/revocation and error-code regressions.

## Evidence

- Red phase: all eight updated/new form cases failed on the original behavior; numeric normalization and registration failed; account validation returned generic codes.
- `pnpm --filter @multica/views test auth/password-form.test.tsx auth/login-page.test.tsx`: 47 passed.
- `pnpm --filter @multica/core test auth/password.test.ts auth/store.test.ts`: 20 passed.
- `pnpm --filter @multica/views test locales/parity.test.ts`: 60 passed.
- `pnpm --filter @multica/views typecheck`: passed.
- `pnpm --filter @multica/views exec eslint auth/password-form.tsx auth/password-form.test.tsx`: passed.
- `go vet ./internal/auth ./internal/handler`: passed.
- `go test ./internal/auth ./internal/handler -run 'TestPassword|TestNormalizeUsernameEmployeeID' -count=1 -v`: passed against a freshly migrated isolated database. Numeric accounts completed registration, duplicate-name rejection, login, password change and session revocation.
- `git diff --check`: passed.

The default local database lacked the existing password migrations. Verification instead created a task-only database, ran all existing migrations, tested, then dropped that database. Logs: `/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-registration-dUEVjx/`.

## Limits

- The existing Redis integration test skipped because `MULTICA_TEST_REDIS_URL` was not configured; Redis behavior is unchanged.
- Browser/Electron end-to-end testing and full repository suites were not run. The shared form tests cover rendering, accessible guidance, submission and localization; Go tests exercise the real registration/login handlers and database.
- Existing binaries must include the updated backend validation to accept numeric usernames. No running development services were restarted.
- No dependencies, schema migrations or unrelated source changes were added.
