# Password migration cutoff diagnosis

**Confirmed test configuration defect; not a demonstrated product defect.** No live environment was changed and no replay was run.

Both `password-binding-desktop.spec.ts:83` and `password-migration.spec.ts:58` mint the legacy session with `iat = floor(cutoff / 1000) - 60`. Retained pw32/pw34 configs set cutoff to `2026-11-05T00:00:00Z`, deriving `iat=2026-11-04T23:59:00Z`. Both original runs happened around `2026-11-04T17:09Z`, so the token timestamp was approximately6h50m in the future. The configured UTC date is later than the actual run; the reason that date was selected is not established by these records.

`server/internal/middleware/password_auth.go:66` enables `jwt.WithIssuedAt()` and rejects an invalid token before session lookup. `server/internal/auth/password_session.go:135` independently rejects `issued.Time.After(now)`. `password_config.go:19` only validates the UTC format and ordering of cutoff/deadline; it does not guarantee this test's derived issued-at is historical.

Original evidence agrees: pw32 fails at line87 because `requires_account_setup` is undefined before native UI launch; pw34 legacy setup fails at line69 waiting for `#password-name`. Its separate temporary-recovery case passes. No response status is inferred solely from the undefined assertion.

Preserve both original failures. Use two new numbered runs/fresh dedicated databases and the same specs, selecting a definitely historical cutoff, for example `2026-11-04T00:00:00Z`, and a deadline later than execution. Root preflight should assert the derived issued-at is at or before current UTC with margin, while retaining cutoff<deadline. Do not weaken JWT validation, assertions or timeouts. Corrected execution remains NOT_RUN until new evidence exists.

Exact timestamps, values and source references are in `legacy-lanes-review.migration-note.json`. This review read only existing source and retained run/config metadata; no tests, build, services or DB access occurred.
