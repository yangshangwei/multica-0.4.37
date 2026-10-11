# Research: administrator initialization and password contracts

- Query: Find the existing bootstrap, administrator locks/counting, restricted password sessions, recovery, audit/idempotency and error-display contracts; identify a minimal deployment-only initializer and its retry boundaries.
- Scope: internal; repository source inspection only. Planning is authorized; implementation is not.
- Date: 2026-10-11

## Findings

### Current contracts and files found

| File / anchor | Responsibility and relevant behavior |
| --- | --- |
| `server/cmd/server/main.go:302` | Dispatches recovery and platform-admin CLI commands before listeners/workers. Ordinary startup only prepares the platform organization at line 393; it does not create administrators. |
| `server/cmd/server/platform_admin.go:22` | `platform-admin bootstrap --user UUID --reason ...`; requires password mode and an explicit existing account. This CLI does not itself require the admin feature flag. |
| `server/cmd/server/platform_admin.go:62` | Validates reserved `admin` slug, usable indexes/control schema, internal organization and workspace backfill. A migration ledger alone is insufficient because a concurrent index build may be invalid. |
| `server/internal/service/platform_admin.go:491` | `Bootstrap`: one transaction, explicit ready account, role grant, credential version bump/revocation, deployment-operator operation and audit. |
| `server/internal/service/platform_admin.go:117` | `LockPlatformAdminMutation`: shared advisory lock first, then sorted user-row locks. Can be called with no user IDs to lock before creating a new user. |
| `server/pkg/db/queries/admin.sql:12` | Advisory lock key is `70856394211471801`; current effective-admin query joins roles, users and credentials. |
| `server/internal/service/platform_admin.go:148` | `CountEffectiveSuperAdmins` also applies the emergency UUID/email denylist. `EnsureEffectiveSuperAdmin` rejects a zero result. |
| `server/internal/service/platform_admin.go:62` | Authorization requires password mode, full JWT/cookie session, current credential version, active/non-denylisted user, suitable DB role, active internal organization. |
| `server/pkg/db/queries/admin.sql:47` | `GetPlatformAdminAccount` reads version, restricted/disabled state and role in one snapshot, preventing an old-version/new-role combination. |
| `server/internal/service/platform_password_recovery.go:39` | Deployment recovery changes credentials under shared locks; never grants roles or restores disabled accounts. Explicit break-glass permits zero effective administrators until change completes. |
| `server/internal/auth/password_session.go:91` | JWT checking derives `Change` from current DB `must_change_password`, not a client/JWT role or restriction claim. |
| `server/internal/auth/password_session.go:142` | Restricted JWT allows only `GET /api/me`, `POST /auth/logout`, and its setup/change endpoint. |
| `server/internal/middleware/password_auth.go:82` | Derived credentials must pass current version checks; restricted JWT requests outside the allowlist receive 403 `password_change_required`. |
| `server/internal/handler/auth_password.go:128` | Registration creates user + credential atomically, respects signup policy, and creates no platform role. Login at line 172 does not require signup to remain enabled. |
| `server/internal/handler/auth_password.go:341` | Password change verifies current password, computes KDF before locks, locks user, rechecks version/hash, increments version, clears restriction, revokes derived credentials, commits and returns a new session. |
| `server/internal/auth/password_revoke.go:17` | Revocation marks PATs revoked, deletes task/daemon tokens and stops owned runtime work; preserves identity, memberships, installations and history. |
| `server/internal/auth/password.go:22` | Usernames trim and lowercase ASCII letters/digits/underscore; `platform_admin` is valid. Display name is a separate field. Hashing uses salted PBKDF2-SHA256 at line 68. |
| `server/pkg/db/queries/user.sql:16`, `server/pkg/db/queries/password.sql:7` | Reusable insert primitives; credentials begin at positive `session_version=1` and accept `must_change_password=true`. |
| `server/migrations/462_password_credentials.up.sql:2` | Credential storage contains hash only, with version and restriction flag. No recoverable password is stored. |
| `server/migrations/463_password_user_unique.up.sql:1`, `464_password_username_unique.up.sql:1`, `469_platform_role_user_unique.up.sql:1` | Concurrent unique indexes protect one credential per user, one normalized username and one role per user. |
| `server/migrations/465_platform_admin.up.sql:25` | Operation/audit rows support `deployment_operator` with NULL human actor IDs; operation and audit are transaction-bound metadata. |
| `server/migrations/471_admin_operation_actor_key_unique.up.sql:1` | Idempotency unique index excludes `actor_id IS NULL`, so it does **not** deduplicate deployment-operator CLI calls. |
| `packages/views/auth/password-form.tsx:78`, `password-error.ts:38` | Shared form catches `ApiError`, maps controlled codes to localized messages, retains failed input and shows `role="alert"`; unknown codes intentionally show a generic message. |

### Why automatic initialization needs a dedicated path

`Bootstrap` explicitly refuses absent credentials and `MustChangePassword` at `platform_admin.go:535–543`, grants only an existing account, increments its version, and calls `EnsureEffectiveSuperAdmin` before commit at line 581. A newly created restricted administrator intentionally contributes **zero** effective administrators: `admin.sql:20` excludes `must_change_password=true`. Reusing this method therefore rejects the required initial state.

Creating an unrestricted account first, bootstrapping it, then setting the restriction creates a privilege window and several independent commits. Setting the restriction later via recovery is also refused for the last administrator unless explicit break-glass is used. Neither sequence meets atomic provisioning.

Keep the current manual bootstrap/recovery semantics. A dedicated first-time service may create the new restricted principal under a narrowly documented exception to the last-effective-admin invariant. Do not relax `EnsureEffectiveSuperAdmin`, `Authorize`, `ChangeRole`, or existing recovery behavior. The explicit manual bootstrap may still grant a named ready account when only unusable administrators remain; that is an authorized recovery behavior already tested at `platform_admin_test.go:637`, not a suitable automatic predicate.

### Minimal service and database boundary

Recommended responsibility split for the parent's selected post-health offline-upgrade CLI:

1. CLI requires **both** password mode and enabled platform administration before creating a credential file. It validates current deployment/organization/schema, prepares a cryptographically random password and its existing `HashPassword` output outside database locks, and owns protected file I/O. Registration endpoints, signup policy and ordinary server startup are not involved.
2. A deployment-only `Initialize...InTx(ctx, tx, params)` receives a stable non-secret intent UUID, expected organization/deployment identity, fixed normalized username, precomputed password hash, reason and request ID. No existing target account ID, arbitrary role or caller-selected username should be an initializer option. It receives only the hash; no plaintext enters database query parameters, operation digests, audit or diagnostics.
3. The transaction takes `LockPlatformAdminMutation(ctx, tx)` before decisive eligibility/marker reads. Recheck the feature/mode gate at the service boundary or pass a validated explicit configuration dependency; manual `Bootstrap`'s password-only gate must not silently define this new CLI's gate.
4. Existing terminal marker wins. Otherwise check retained administrator evidence as below. Suppressed deployments get only a terminal suppression marker, with no credential mutation. An occupied normalized username without qualifying prior-admin evidence is a conflict; never adopt, reset, rename or promote it.
5. For an eligible empty deployment, insert a **new** user with no workspace membership or fake email, insert its credential with `must_change_password=true`, insert its `super_admin` role, and persist the terminal marker + deployment-operator operation + audit in the **same** transaction. `SetPlatformRole` is an upsert (`admin.sql:4`); it is safe here only with the just-created UUID. The username unique index settles a concurrent ordinary registration; roll back the newly inserted user on collision.
6. A new principal has no prior valid credentials. Initial version 1 is sufficient; do not invent an extra version bump or run broad revocation for unrelated users. A successful forced change will advance to version 2 through the existing change path. This reasoning applies only because the initializer cannot reuse an existing account.
7. The caller commits only after the corresponding file/intent is durably installed. Return a structured, non-secret outcome (`created`, `already_initialized`, `skipped_existing_admin`, conflict/unavailable) and stable marker/intent/user/operation IDs. A commit error means outcome unknown until reconciled, not permission to regenerate or reset.

An `InTx` boundary matches `RecoverPasswordForDeploymentInTx`; document rollback ownership. The command can perform a read-only preflight to avoid unnecessary KDF/file work, but the transaction's locked decision is authoritative. File publication must follow that authoritative eligibility decision or remain an explicitly uncommitted staged intent; a racing existing administrator must not leave a misleading ready-password file.

### Terminal marker and exact retained-evidence predicate

Recommend a **database-scoped singleton**, with immutable creation identity, not a fresh row keyed only by a changeable `MULTICA_DEPLOYMENT_ID`. Record the expected deployment UUID/organization as provenance and reject mismatch; changing configuration must not reopen initialization in the same database. Suggested durable states are `created` and `skipped_existing_admin`. Record stable intent ID, created user/operation IDs where applicable, and timestamps; no secret or password hash is needed in the marker.

Resolution recorded after planning review (2026-10-11): the parent selected preservation of **any current platform role, including observers**, as an already-administered deployment. Manual bootstrap/grant preservation markers may have `deployment_id=NULL`, retaining their existing configuration preconditions; only automatically `created` markers require the matching deployment UUID. These are resolved choices in `design.md`, not open decisions for implementation.

For a deployment without a marker, the selected predicate preserves a current platform role or retained **known super-administrator** provisioning through any of:

```sql
EXISTS (SELECT 1 FROM platform_role_binding)
OR EXISTS (
  SELECT 1 FROM admin_operation
  WHERE kind = 'user.role.bootstrap' AND state = 'applied'
)
OR EXISTS (
  SELECT 1 FROM admin_audit_event
  WHERE target_kind = 'user' AND phase = 'applied'
    AND action IN ('user.role', 'user.role.bootstrap')
    AND (before_state->>'role' = 'super_admin'
      OR after_state->>'role' = 'super_admin')
)
```

This is a proposed query, not existing code. Evidence: `Bootstrap` always writes `user.role.bootstrap`/`applied` at `platform_admin.go:568`; `ChangeRole` records before/after role snapshots at lines 315–317, with `phase=applied` from line 333. Scan globally like existing platform roles; do not join current user/credential rows or filter disabled/restricted accounts away. Retained historical rows must remain evidence after their user/role is removed. The singleton marker, when present, supersedes this historical inference.

| Observed state | Safe automatic result |
| --- | --- |
| Ready super-admin binding, any username | Terminal suppression; preserve all accounts, roles and credentials. |
| Super-admin binding with pending forced change, disabled user, denylisted identity, absent credential/user | Same suppression; missing/broken identity is an operator problem, not permission to create another administrator. |
| Role revoked/demoted/deleted but prior super-admin operation/audit remains | Terminal suppression based on history. No automatic regrant. |
| Existing `created` marker but the created account is now disabled, recovered, demoted, deleted or missing | Preserve marker and account state; report it. Never recreate based on current effective count. |
| Existing `skipped_existing_admin` marker but roles/audit later removed | Preserve suppression. Marker is the lasting once-only decision. |
| No marker/evidence; fixed username belongs to ordinary account | Conflict, zero account/role changes. Do not use a display-name match. |
| Observer-only deployment with no historical super-admin | Terminal suppression, per the parent's resolved choice to preserve any current platform role. |
| No marker, bindings or retained history because of historical manual deletion/retention | The repo has no independent “ever initialized” fact. Absence cannot prove never administered; document operator-managed legacy exceptions. |

Current code has no initializer marker in inspected migrations/queries. Role deletion at `platform_admin.go:287` leaves operation/audit evidence; those rows have no FK cascade. New markers must survive role/account deletion, credential reset, audit retention and ordinary upgrades. Down migrations need refusal guards for nonempty markers, including suppression-only rows; losing this table would silently re-enable creation. Follow `465_platform_admin.down.sql:4` and `admin_rollback_guard_test.go:121`: lock all checked/DDL-target tables deterministically with `ACCESS EXCLUSIVE NOWAIT`, then guard + DDL together. Every new unique/index build must remain its own `CREATE ... INDEX CONCURRENTLY` migration.

The ordinary operation helper cannot supply CLI idempotency: `CreateOperationInTx` requires a full human JWT at `platform_admin.go:393`, and `FindOperationByKey` scopes lookup to that actor at line 447. The unique index is partial on non-NULL actor IDs. Bootstrap/recovery also generate a new random key on each call. Use the terminal singleton/intent ID as the new CLI reconciliation contract; do not pretend an operation key already provides it.

### File intent, concurrency and uncertain commits

Prefer the terminal marker plus a **durable precommit file intent**, without a separately committed database `prepared` state. A prepared DB row could reserve a cross-host winner, but requires abandoned-intent ownership/takeover rules and still cannot recover a lost plaintext file after the account commit. The parent explicitly selected the smaller terminal-marker design.

- The protected file must carry or have securely associated non-secret provenance: format/version, intent UUID, database/deployment/organization identity, fixed username and creation identity. Persist raw password only there. Sync file contents and directory publication before the account transaction commits; mode `0600` alone does not establish durability. Never overwrite an unknown/mismatched existing file.
- Coordinate same-path writers with the filesystem protocol, and serialize all DB creation/suppression decisions with the existing platform advisory lock. Keep the lock until file durability + the mutation transaction are resolved. A preflight before taking this lock is advisory only. Multiple hosts sharing a DB but lacking a shared canonical file cannot automatically reconstruct the winner's file; return committed provenance and route to its owning deployment location.
- Reconcile under the **same advisory lock** on a fresh transaction/connection after uncertain commit. This waits for an earlier attempt still committing, then observes either its terminal marker or confirmed absence. Do not treat a connection error, lock timeout, or unlocked “not found” observation as permission to replace the candidate password.
- File present, marker absent: verify the file's intent/provenance; after locked eligibility recheck, reuse its same password. KDF hashing may use a new salt on retry, but the login secret must remain unchanged. A lost response must not create a new secret.
- Marker `created` with same intent and still-original credential: compare provenance and verify the candidate secret with `VerifyPassword` against the credential, never print either value or use hash bytes as an operation digest. If verification is computed before locking, re-read and compare the verified credential snapshot under lock, matching the existing login/change pattern.
- After a successful password change or later recovery, current hash/version no longer matches the initial file by design. Preserve the terminal marker and report credential advancement; do not interpret mismatch as permission to restore the initial password. Lock/read the target user row for coherent version/restriction/role diagnostics, following advisory-before-user order.
- A failure before commit rolls back user, credential, role, operation, audit and marker together. The durable uncommitted file can remain for same-intent retry. Cleanup only a proven uncommitted artifact that this attempt owns; never delete on an uncertain outcome.
- Marker committed but initial file missing is **not automatically recoverable** from the one-way hash. If first change is already complete, no initial secret is needed. If the initial credential remains pending, return a specific operator-recovery result rather than success or auto-reset. Recovery must target the marker's exact UUID and retain role/disabled state; with zero other effective super-admins, use existing explicit `--break-glass` semantics. `platform_password_recovery.go:98–108` and `password_recovery.go:76–83` establish this boundary. File-before-commit prevents the normal process-crash window; storage loss/manual deletion cannot be promised away.

### Forced-change gap and minimal UI handling

The existing session restriction is strong enough for the new principal: business routes and CLI/PAT minting are denied, and complete-admin authorization rechecks the DB role/version. `platform_admin_auth_test.go:180` already expects a restricted super-admin JWT to receive 403. Password change clears the flag and invalidates old JWTs/derived credentials in one transaction.

**Gap:** `auth_password.go:350–410` accepts identical current/new passwords. It verifies current, hashes new and clears the restriction, with no inequality check. Thus an initial password can remain usable after “change,” contrary to AC3.

Recommend a server error such as HTTP 400 `password_unchanged` **only when the persisted credential is forced-change and the verified current/new strings are equal**. Do not trim or normalize passwords; spaces are supported. Apply before clearing the flag under the existing locked current-version/hash recheck; an early check after successful current-password verification can avoid wasted KDF, but must not weaken the locked-state contract. Preserve ordinary password-change behavior. Wrong-current-password and stale-version failures retain their existing statuses.

No form/state/API rewrite is needed to display this rejection:

- Add this explicit code to `packages/views/auth/password-error.ts:54` with a new locale key in `locales/en/auth.json` and `locales/zh-Hans/auth.json`. Suggested meaning: choose a password different from the temporary password. Unknown/malformed error bodies should still use the localized fallback.
- `PasswordForm` already catches it and renders its alert at lines 110 and 161; failure does not call `onSuccess`, clear the current values, mint a CLI token or list workspaces. `store.ts:70` only installs a new session after a successful API result, and `client.ts:1214` already parses password responses.
- A 400 validation error avoids authentication-expiry side effects. Keep full validation in the server; no new browser-only equality rule, extra state, or input-API fields are necessary.
- Canonical error-code mapping tests belong in `password-error.test.ts` (already node environment). One `PasswordForm mode="change"` integration regression should assert localized alert, preserved restricted state and no success/workspace/CLI continuation; do not duplicate the mapper's full matrix in DOM tests. Existing form tests at line 151 cover change-mode inputs and line 215 explicitly locate the error matrix in the pure helper.

### Existing evidence and recommended validation

Tests were inspected, **not executed**; this assignment is planning-only and allows writes only to this research directory. Meaningful existing tests to retain:

| Test source / anchor | Contract |
| --- | --- |
| `service/platform_admin_test.go:311` | Concurrent manual bootstrap yields one admin and requires a new login. |
| `service/platform_admin_test.go:253`, `:541` | Audit failure rolls back; commit-applied-but-response-lost fixture proves reconciliation without duplicate version/audit. Reuse that uncertain-commit fixture for the initializer. |
| `service/platform_admin_test.go:471`, `:628` | Pending/disabled/denylisted admins are excluded from **effective** counting; this exclusion must not leak into new initialization eligibility. |
| `service/platform_password_recovery_test.go:35`, `:92`, `:123`, `:145` | Last-admin break-glass, audit rollback, secret-free digest and concurrent recovery preserving one effective admin. |
| `handler/auth_password_test.go:296`, `:316` | Temporary-session business/CLI denial, change/version invalidation, username race and signup-closed login. |
| `cmd/server/platform_admin_auth_test.go:135` | Full router JWT/cookie/PAT/task/daemon/recovery authorization matrix, without workspace membership. |
| `handler/auth_password_revocation_test.go:18`, `handler/password_ws_test.go:67` | Disconnect the target rather than actor; revoke protected realtime operations. |
| `cmd/server/password_recovery_test.go:137` | Username-collision rollback on explicit recovery. |
| `migrations/platform_admin_migration_test.go:15`, `admin_rollback_guard_test.go:121` | No FK/index validity, retention rollback and concurrent writer fences. |

New canonical assertions should cover:

1. Closed-signup eligible deployment creates exactly one new user/credential/role/marker/operation/audit; fixed username, positive initial version, forced change, NULL operator actor, no workspace membership and no secrets/hashes in audit/digest/logs.
2. Repeated and concurrent initializer calls, concurrent manual bootstrap, and concurrent signup with the fixed username. Exactly one provisioning commit or preserved existing state; losing account insert rolls back. Same-name ordinary user is never adopted.
3. Eligibility matrix: ready/pending/disabled/denylisted super-admin; role revoked with bootstrap operation only or with before/after-role audit only; `created` and suppression markers after later role/account/audit deletion; observer-only deployment is preserved.
4. Fail user/credential/role/marker/operation/audit insert individually; no partial DB provisioning. Include invalid/missing marker and credential unique indexes, inactive organization, disabled feature/device mode and database faults.
5. File write/sync/publication failure prevents commit; precommit crash leaves reusable same-intent file; applied commit with lost response reconciles without a second credential/audit; unresolved DB state retains file and reports unknown. Missing/mismatched postcommit file does not reset any account.
6. Server forced-change same-password rejection preserves hash/version/flag and restriction. Different password succeeds; old password login and every old session fail; fresh session receives `/api/admin/me` with `super_admin`. Ordinary-password same-value behavior stays unchanged. Use real router checks for admin/business/credential endpoints rather than only invoking handlers with forged context.
7. Marker down-migration refuses both terminal states and active first writes; role/credential/audit retention cannot erase the once-only decision. Service fixture tables at `platform_admin_test.go:92` must clone the new marker table to keep global counts and singleton state isolated.
8. UI: pure new-code + malformed/unknown-body mapping; one localized forced-change form regression retaining restricted auth and preventing continuation.

Relevant future command families: from `server/`, `go test ./internal/service -run 'TestPlatformAdmin|TestPlatformPasswordRecovery'`, `go test ./internal/handler -run 'TestPassword|TestPlatformAdmin'`, `go test ./cmd/server -run 'TestPlatformAdmin|TestPasswordRecovery'`, and targeted migration tests; extend patterns for new initializer tests. DB-backed service tests explicitly skip without `DATABASE_URL` at `platform_admin_test.go:68`, so a green process exit without a configured migrated test DB is not sufficient evidence. Use repository pnpm scripts for the views helper/form checks plus lint/typecheck after implementation.

### Related specs and references

- `CLAUDE.md`: package boundaries; sqlc regeneration; no FK/cascades; concurrent single-statement index migrations; one canonical test layer; fixture-based DB tests.
- `.trellis/workflow.md`: planning must precede implementation; persist research; task creation does not authorize implementation.
- `.trellis/spec/server/security-boundaries.md:219`: password-mode session/minting fences, lock order, last-effective-admin and deployment-operator audit contracts.
- `.trellis/spec/server/offline-delivery.md:30`: current manual bootstrap procedure, preserved auth/signup policy and full-console verification prerequisites.
- `.trellis/spec/core/frontend/api-client-auth-epoch.md`: validation errors must not disturb session-epoch handling.
- `.trellis/spec/views/frontend/quality-guidelines.md`, `apps/docs/content/docs/developers/conventions.mdx:108`: shared form tests and maintained English/Simplified Chinese locales.
- External references: none consulted, per dispatch. Repository versions: Go 1.26.6, pgx v5.9.2, JWT v5.3.1 (`server/go.mod:3`, `:15`, `:18`).

## Caveats / Not Found

- No code, tests, migration, task status, runtime state or git operations were changed. This file contains proposed contracts, not an implemented initializer.
- The chosen terminal marker + durable file protocol cannot reconstruct plaintext after both its original file and other authorized copies are lost. Existing recovery is an explicit reset with audit, not automatic retry.
- Historical detection is evidence-based; manually deleted pre-marker roles **and** historical operation/audit records are indistinguishable from never-initialized state. Do not promise universal inference for that case.
- Observer-only handling is resolved: preserve it. All current platform bindings and known super-admin history remain evidence regardless of current effectiveness.
- Existing platform audit projection allows `requires_password_change`, not `must_change_password` (`handler/admin_audit.go:38`). For a new visible initialization snapshot use the existing safe projected name; do not expand arbitrary snapshot exposure.
- The existing offline-delivery spec also requires managed-installation/deployment configuration to verify overview/installations. The initializer alone must not silently enable those switches; the parent's delivery investigation owns that integration.
