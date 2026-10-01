# Six-character passwords — verification

## Changes

- `server/internal/auth/password.go`: minimum lowered from 12 to 6 Unicode characters in the validator shared by registration, login, setup, password change and recovery. Maximum 128 characters / 512 bytes and hashing remain unchanged.
- `server/internal/handler/auth_password.go`: password-change validation message updated.
- `packages/views/auth/password-error.ts`: six-character threshold for detailed errors, retaining recognition of older API messages.
- `packages/views/locales/{en,zh-Hans}/auth.json`: help, validation and actual-length messages updated to 6–128.
- `server/internal/auth/password_test.go`: five/six-character boundaries, Unicode, spaces, existing long passwords, upper bound and six-character hashing.
- `server/internal/handler/auth_password_test.go`: six-character registration/login/change and account setup; five-character rejection.
- `packages/views/auth/password-error.test.ts` and `password-form.test.tsx`: six-character submission, bilingual help and detailed five-character rejection.

## Evidence

- Before implementation, backend tests rejected six-character passwords and frontend tests reported the old 12-character minimum.
- `pnpm --filter @multica/views test auth/password-error.test.ts auth/password-form.test.tsx locales/parity.test.ts`: 93 passed.
- Views typecheck and scoped ESLint: passed.
- `go vet ./internal/auth ./internal/handler`: passed.
- `go test ./internal/auth ./internal/handler ./cmd/server -run 'TestPassword|TestNormalizeUsernameEmployeeID|TestReadRecoveryPassword' -count=1 -v`: passed in a freshly migrated isolated database; database removed afterward.
- `git diff --check`: passed.
- Backend logs: `/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-password-six-IuUM7W/`.
- Restarted only the local API using its explicit password-mode registry environment. Health reports pid 41057, started 2026-10-01T02:54:32Z, and config confirms password authentication.
- Live probes: five-character registration returns `invalid_password` with the 6–128 rule; a six-character login for a synthetic nonexistent account reaches `invalid_credentials` instead of failing format validation. No account was created by these probes.

## Limits

The unchanged Redis integration test skipped because `MULTICA_TEST_REDIS_URL` was unset. Browser/Electron end-to-end testing was not run. No dependencies or migrations were introduced.
