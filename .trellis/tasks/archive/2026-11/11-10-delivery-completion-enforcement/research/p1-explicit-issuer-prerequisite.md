# P1 prerequisite: explicit issuer password authority

Date: 2026-11-11. Implementer: native `trellis-implement` / `implement_observer_core`.
Scope: one shared auth helper extraction, one service call change and one new
DB regression file. Prior P0 source/run/review reports remain unchanged.

## Root cause and contract

`CheckArtifactCredentialInTx` accepts explicit verified credential metadata, but
its final password check called `auth.CheckPasswordVersion`. That function reads
`PasswordSessionFromContext(ctx)` and, when the ambient session has a managed
binding, requires that session to identify the requested user/version. A valid
retained issuer A therefore failed under a different current managed consumer B,
even though both identities were independently current and valid.

The P0 extraction preserved the old same-subject request behavior. This new
cross-subject case is required by P1's explicit issuer/consumer model; it does not
retroactively invalidate or rewrite the earlier same-subject compatibility runs.

## Test first

Before any production edit, added only
`handler/delivery_completion_issuer_credential_test.go` with
`TestDeliveryCompletionExplicitIssuerCredentialIsolation`. It uses actual
`passwordRegister` JWT authentication through middleware for issuer A, and an
existing `managedDaemonSetup` with a real token, installation/binding/password
rows for consumer B. The ambient context is built from that actually validated
managed token; it is not a fabricated success response.

The test independently confirms A's current credential and B's current managed
binding before exercising their separation. Other cases mutate actual owned
issuer rows for version change, required password change, disable and deletion;
legacy controls preserve cross-user rejection and current managed-binding
revocation. Each explicit-credential check borrows a real test transaction.
This is a helper regression, not complete authorization-router or consumer-action
acceptance; P1's real route must separately authenticate and authorize B.

The main session ran the new test in its fresh task-owned
`enf_p1_issuer_red01` database before the fix. `runs/p1-issuer-red01.json`
retains exit 1 with 6 named passes and 2 named failures: the one actual behavioral
failure `current_issuer_under_different_managed_consumer` plus its containing
parent test. Its failure is `session is no longer valid`; it is not a compile or
fixture failure. The revocation and legacy controls passed. The frozen test's
SHA-256 is unchanged after implementation.

## Minimal correction

- `auth/password_session.go`: `CheckPasswordVersion` remains the context adapter.
  `CheckPasswordSessionVersion(ctx, q, session)` is the new explicit-session
  entry. Both call the same private `checkPasswordVersion`, retaining the exact
  database user/password-version/must-change/managed-binding checks. No second
  password authorization algorithm or context impersonation was introduced.
- `service/artifact_authority.go`: only its final password check changes to
  `CheckPasswordSessionVersion(ctx, q, s)`, where `s` is the already-explicit
  issuer credential snapshot. Its existing account/credential/binding locks,
  principal match, setup/change refusal, expiry and other fences remain intact.
- The new test file is unchanged between the recorded red and the fixed source.

The legacy entry still checks its own ambient managed binding, including
user/version mismatch and revocation. No existing managed authority is relaxed;
consumer authority remains a separate required gate.

## Verification and retained failures

- `p1-explicit-issuer-build01`: full backend build passed after this correction.
- `p1-explicit-issuer-vet01`: exit 1, original log retained. Concurrent ledger
  implementation had just added `service/delivery_completion.go` with unfinished
  references to completionStoredRequest, completionConsumerAuthority,
  completionRevalidateEvidence, completionIssuerCredential and completionReceipt.
  The main session and ledger owner confirmed that file was still being written.
  This lane read the exact diagnostics and did not retry or edit the other
  owner's code. This is a shared in-progress compile failure, not this test's
  behavior red and not a passing vet result.
- `gofmt` and scoped `git diff --check` passed. The frozen test hash was rechecked.
- Fresh green/race and the final coherent backend vet remain owned by the main
  session after the ledger is compile-stable; no green claim is made here.
- This lane did not run any database test, allocate/migrate/clean databases,
  operate containers, call providers, commit, push, deploy or send external
  messages. Build/vet used the private launcher with no database selected.

One read-only query guessed `queries/user_password.sql`; the actual file is
`queries/password.sql`. The rejected read was followed by `rg` location, with no
DB action. This lookup failure is not product red and does not replace any old
failure record.

## Frozen source after correction

| File | SHA-256 |
| --- | --- |
| `server/internal/auth/password_session.go` | `69530f6da81e79658f47e28f4db801d651a6603b9fd7bea621aaecf8e393d33c` |
| `server/internal/service/artifact_authority.go` | `0159c94982a4a98138dc52a921882c9ebc64fabd97d5eb873cf1ee219810bce8` |
| `server/internal/handler/delivery_completion_issuer_credential_test.go` | `9758018037f36255281dd6e141aee5ce182a428f10841ed98b6dcf6a3f7afc66` |
