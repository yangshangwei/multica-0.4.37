# Research: Production account epoch writers and durable invalidation

- Query: Map production account disable/restore, global credential revocation, password security reset and identity migration writers for execution-contract E07; identify the smallest transaction-safe integration, exclusions, durable fanout reuse and real PostgreSQL tests.
- Scope: internal; current source and current task contracts only, no history search.
- Date: 2026-10-13

## Findings

### Conclusion

There are **six production calls** to `auth.RevokePasswordCredentials(ctx, tx, targetUserID)`. Together they cover all discovered database account disable/restore, password setup/change/recovery, platform role elevation and deployment bootstrap writers. All already hold the target user row lock and borrow the caller's transaction. This helper is the smallest common point for atomic collaboration epoch advancement plus durable invalidation intent. Its existing signature need not change.

Do not implement invalidation by adding hooks only to the HTTP handlers or to `publishPasswordRevocation`: the deployment recovery path has no live publisher, and uncertain commits can succeed without publishing. Do not synchronously lock every source session from the credential transaction. E02 explicitly requires a short account mutation followed by durable, ordered fanout.

No independent account UUID/email merge or replacement writer was found. The actual account identity migration is the legacy-login-to-password `PasswordSetup` path, which keeps the same user UUID and already calls the shared revocation helper. Daemon hostname migration and installation rebinding are resource identity changes, not account migrations.

### Production writer inventory

| Entry point / exact implementation | Existing writes and authority | Required E07 integration |
| --- | --- | --- |
| `Handler.AdminDisableUser` → `adminChangeAccount` (`server/internal/handler/admin_user.go:154`, `:163`) → `PlatformAdminService.ChangeAccount` (`server/internal/service/platform_account.go:62`) | Platform advisory lock then sorted actor/target user locks at `:132`; `SetPlatformUserDisabled` at `:178`; configured password version bump at `:205`; global revocation at `:220`; audit at `:233`; commit at `:236`. | Advance target account epoch and persist intent exactly once in this transaction, including an unconfigured legacy account with password version 0. Do not advance the acting administrator's epoch. |
| `Handler.AdminRestoreUser` (`admin_user.go:157`) → `ChangeAccount(Action="restore")` | Clears `disabled_at` at `platform_account.go:186`; also bumps password version and calls the same global revocation. Emergency-denylisted targets remain refused. | Advance again on successful restore. Retain the earlier disable event; restoration must not erase pending invalidation or make pre-disable commands/grants valid. |
| `Handler.AdminRecoverPassword` (`admin_user.go:160`) → `ChangeAccount(Action="recover-password")` | `RevokePlatformLegacySessions` at `platform_account.go:196`; change an existing hash/version or create restricted credentials at `:200`/`:202`; shared revocation at `:220`. | Covered by common helper. Include both existing-password and first-password branches. Replay is checked at `:146–151`, before writes, so reusing an operation key must not increment again. |
| `Handler.PasswordSetup` (`server/internal/handler/auth_password.go:267`) | Owns tx at `:285`; `LockPasswordUser` at `:293`; rechecks user/migration window; creates credential at `:318`, updates profile at `:323`, global revocation at `:328`, commits at `:333`. | The real identity/security migration integration point. Preserve user UUID, membership and collaboration history; invalidate prior authority using the existing fence if the account already participated before switching auth mode. |
| `Handler.PasswordChange` (`auth_password.go:341`) | Owns tx at `:381`; user lock at `:388`; locked session/hash validation at `:401–408`; hash/version change at `:414`; global revocation at `:419`; commit at `:424`. | All successful password changes already revoke all derived credentials, so all advance collaboration epoch. This includes forced recovery completion and ordinary same-value changes; rejected forced same-value changes at `:410` do nothing. |
| `RecoverPasswordForDeploymentInTx` (`server/internal/service/platform_password_recovery.go:39`) | Borrowed `pgx.Tx`; `LockPlatformAdminMutation` at `:50`; create/change credential at `:82`/`:90`; shared revocation at `:110`; operation/audit at `:130`/`:141`. | Covers normal and explicit break-glass deployment recovery. Caller `runPasswordRecovery` (`server/cmd/server/password_recovery.go:25`) begins at `:64`, calls at `:69`, commits at `:76`, and runs before listeners/background jobs (`:23`). Intent must survive until the server next reconciles it. |
| `PlatformAdminService.ChangeRole` (`server/internal/service/platform_admin.go:198`) | Owns tx; shared sorted lock at `:232`; `elevated` at `:304` means no role → any role, or observer → super_admin; only that branch bumps version (`:306`) and globally revokes (`:314`). | Elevation is an existing global credential revocation and must advance target epoch. Role removal, demotion and unchanged role do not currently revoke all credentials; do not broaden those operations into collaboration cancellation. |
| `PlatformAdminService.Bootstrap` (`platform_admin.go:496`) | Existing ready user only; own tx at `:510`; sorted user lock at `:515`; version bump at `:554`; shared revocation at `:564`; audit/marker then commit at `:592`. | Deployment bootstrap is another global credential-revocation writer. Covered centrally, including no HTTP post-commit publisher. |

The first three rows share one call site, giving six total call sites. `rg -n 'RevokePasswordCredentials' server --glob '!**/*_test.go'` is the concise complete current call-site check.

### SQL writer inventory

- `server/pkg/db/queries/admin_user.sql:1` — `SetPlatformUserDisabled` is the sole production `user.disabled_at` writer; disabling also permanently sets `legacy_password_sessions_revoked_at`, while restoring retains it.
- `server/pkg/db/queries/admin_user.sql:35` — `RevokePlatformLegacySessions`; only the administrative password-recovery branch calls it.
- `server/pkg/db/queries/password.sql:7` — `CreatePasswordCredential`; initial registration/initial administrator creation are new identities, whereas setup/recovery of existing users are covered above.
- `server/pkg/db/queries/password.sql:11` — `ChangePasswordCredential` increments `session_version`, with expected-version CAS and signed-bigint exhaustion guard. All three production call sites are in the inventory above.
- `server/pkg/db/queries/admin.sql:22` — `BumpPlatformUserAuthVersion`; all production callers are account disable/restore, role elevation and bootstrap above.
- `server/internal/auth/password_revoke.go:17` — `RevokePasswordCredentials` updates all user PATs and deletes all user task/daemon tokens (`:19–27`), discovers all owned runtimes (`:28`), cancels their tasks (`:40`), and forces runtimes offline (`:44`). It currently changes no collaboration fence, event, participant, grant or run.
- `server/pkg/db/queries/collaboration.sql:21` / `:152` — only `EnsureCollaborationAccountFence` and `GetCollaborationAccountFence` exist. `Ensure` returns an existing epoch unchanged; its conflict update is not an invalidation.

### Transaction and lock primitives to reuse

1. `LockPlatformAdminMutation(ctx, tx, userIDs...)` (`platform_admin.go:117`) takes `LockPlatformAdminChanges` first, sorts UUIDs and deduplicates them (`:122–140`), then calls `LockPasswordUser` for each. Password-only flows must not acquire the platform advisory lock after holding a user lock (`:115–116`).
2. `LockPasswordUser` (`password.sql:16`) is `SELECT id FROM "user" WHERE id=$1 FOR UPDATE`. Despite its name, this is the actual neutral user-row fence and works for users without password credentials.
3. `LockCollaborationUsers` (`collaboration.sql:15`) locks full user rows with `ORDER BY id FOR UPDATE`. Creation locks creator/handler before ensuring account fences (`collaboration.go:162–218`). Queue mutation locks the actor before workspace/issue/session and then reads the account fence (`collaboration_queue.go:346–388`). These locks conflict with the existing auth writers already.
4. `auth.LockPasswordSession` (`server/internal/auth/password_session.go:154`) locks the same user and rechecks the source password version. It is a credential-minting fence, **not** a revocation trigger.
5. `db.New(tx)` / `Queries.WithTx(tx)` (`server/pkg/db/generated/db.go:21`, `:29`) bind existing sqlc queries to a borrowed transaction. The new account primitive should accept `pgx.Tx`, not a pool-backed `*Queries` argument whose transaction ownership is ambiguous.

Existing queue/source ordering: actor → optional gate (`resume_queue`) → workspace → issue → `issue_collaboration` → session → participant → account read; run/command locks follow. See `collaboration_queue.go:337–388`. The account writer should not enter that chain: E02 (`execution-contract.md:32–54`) says account governance locks → epoch + durable event, then **no holding the user lock while waiting across source sessions**. Fanout batches use stable issue UUID order and NOWAIT source locks. Never add an auth → service import: service already imports auth; `internal/collaboration` also imports auth (`access.go:8`).

### Smallest concrete production integration

Implement a neutral borrowed primitive in the existing `internal/auth` package, for example:

```go
// Caller holds the target user row FOR UPDATE and owns commit/rollback.
// Advances only an existing collaboration fence; no source/session locks.
func InvalidateCollaborationAccountInTx(
    ctx context.Context, tx pgx.Tx, userID pgtype.UUID,
) error
```

Call it once near the start of `RevokePasswordCredentials`, **before the `len(ids)==0` early return at `password_revoke.go:36`**. This preserves the six call sites and avoids an import cycle or a handler/service injection refactor. It must not be gated by `PasswordMode`: the fence is neutral even though the current common caller has a password-oriented name. A content-free reason such as `credentials_revoked` is enough for the common invalidation record; existing administrative audit already records whether the root operation was disable, restore, recovery or elevation. If distinct cause names are needed by the implementation, pass an explicit internal reason through these six calls rather than inferring it from request paths.

Inside the borrowed primitive:

1. Read/lock the existing account fence under the caller-held user lock. Missing fence is a no-op: E07 establishes it lazily at first participation/authorization, so an account that has never acquired collaboration authority has no old command/grant to invalidate. Because creation holds the same user lock, a concurrent first participation either commits before invalidation and gets advanced, or starts after the account operation and captures fresh state. Do not use missing-row fallback for an actual collaboration admission; those paths must still fail closed. No eager fence creation during ordinary password registration is necessary.
2. Advance an existing positive epoch by one, update its timestamp and reject `MaxInt64` exhaustion. Distinguish exhaustion from absent fence; never treat a guarded UPDATE returning no row as automatic success without checking which condition applies.
3. Insert an immutable durable account invalidation event with `user_id`, the new `epoch`, a non-secret kind, creation time and pending/reconciliation state. Use `(user_id, epoch)` as its unique identity. Event insertion failure must roll back password/user/token/audit changes with the caller transaction. Preserve every event across fast disable → restore; a mutable disabled flag is not durable intent.
4. Return without publication or source locking. The transaction owner's existing uncertain-commit/idempotency behavior remains authoritative; never replay the security mutation automatically merely because a wakeup or commit response failed.

A **small dedicated account invalidation outbox table** is the clean minimum additional persistence because existing tables are all scoped differently (next section). New indexes must be concurrent, one SQL statement per index migration; no foreign keys/inline unique constraints. Extend retained-data down guards for the new table. It is not enough to bump `epoch` and emit an in-memory event.

For P4 integration, add a polling `CollaborationService` reconciler that consumes the durable account events. If delivered in stages, the first stage can prove the production writer + atomic durable intent; it must still report fanout as pending and keep public execution gated until the consumer and all E01 readers are implemented.

### Durable fanout and existing reusable storage

| Existing storage / code | Reuse | Constraint |
| --- | --- | --- |
| `collaboration_event` (`571_collaboration_schema.up.sql:138`; `collaboration.sql:55–60`) | Per-issue control/history event and pending outbox record after invalidation changes commit. | `issue_id NOT NULL`, positive session event sequence. Cannot represent a single account event before discovering issues. No production collaboration outbox delivery/reconciliation consumer exists yet. |
| `collaboration_notification` (`571…:149`; `collaboration.sql:62`) | Current-participant notification intent, idempotent recipient/issue/event sequence. | Human notifications are not a security work queue; recipient filtering does not prove affected resources have been stopped. |
| `collaboration_command.author_user_id/author_auth_epoch` (`571…:67–79`) | Find stale author intents independently of current membership and current enabled state. | Compare persisted epoch with the invalidation threshold; an old event must not cancel a fresh command captured after that event. Preserve immutable content, sequence and history. |
| `collaboration_grant.owner_user_id/owner_auth_epoch` (`571…:49–65`) | Find stale owner grants and affected runs; author and resource owner are different predicates. | Do not fan out only through current source-space membership or only command authors. Current participant membership alone is also insufficient for historical unsettled runs. |
| `collaboration_run`, `collaboration_session`, `collaboration_termination` | Existing pause/stop/claim-generation/termination-stop SQL in `collaboration.sql:184–233`. | Current `StopQueue` owns an authenticated transaction and needs current human request versions. Do not call it as the worker wrapper. Extract/reuse only a borrowed trusted-system stop primitive under appropriate source locks. |
| `AdminOperationService.Reconcile/Run` (`server/internal/service/admin_operation.go:326`, `:396`) and `LeaseDueAdminCancellations` (`admin_operation.sql:71`) | Existing bounded polling, database leasing and retry scheduling pattern; server starts it at `cmd/server/main.go:666`. | These tables/queries are organization- and `task.cancel`-specific; do not put neutral account invalidation into fake administrative operations or infer process proof from their confirmation enum. |
| `collaboration_operation` (`571…:123`) | Existing human idempotency-receipt pattern can inform per-issue deduplication. | CHECK restricts scope to `create|issue`; requires actor. It is not an account outbox. |

Worker rules derived from E01/E02/E07:

- The committed account epoch rejects stale submit/admission/business capability immediately; eventual fanout converges queues and visible state. All future promote/claim/finalize/permit/business-token readers must lock and compare both author and owner account epochs. The present foundation checks only human queue operations and does not yet implement these positive execution readers.
- Find stale authored waiting commands, stale owner grants and unsettled runs tied to either, even when the account was already restored or its participant relation has changed. Scope matching by saved old epochs (`< event.epoch`), not simply by `user_id` or `disabled_at`. New authority created after the event must survive delayed processing.
- Work in bounded stable issue-UUID order; use NOWAIT source locking and durable retry. Never advance a cursor past a busy/unprocessed issue. Idempotently commit each issue's mutations, history event and progress together; repeating a worker after a crash must not keep incrementing `control_version` or `claim_generation`.
- Reuse D06 outcomes and E03 proof guards: invalidate affected grants, cancel stale waiting authority, request affected run stop, and pause where the authority/resource loss requires it. Preserve other authors' waiting work and participants/history. A restore never reactivates a session, revives a cancelled command or revalidates an old grant.
- `StopQueue` currently computes server-proven no-permit evidence before overwriting progress (`collaboration_queue.go:181–199`). Any permitted or inconsistent run retains the barrier; generic engine cancellation, forced runtime offline, disconnect or task completion is not termination proof. Preserve run fence, launch nonce and historical termination credential identity.
- Auth's current broad `CancelAgentTasksByRuntimeOrAgent` (`runtime.sql:307–334`) has no purpose filter and can cancel engine rows for owned runtimes. Keep its ordinary behavior, but P4 must route collaboration execution effects through the durable source/run protocol. Adding an epoch alone does not close this legacy engine-writer gap.

### Explicit exclusions: do not advance the account epoch

| Flow | Current evidence / intended boundary |
| --- | --- |
| Ordinary login/re-login, browser reload, JWT expiry or successful JWT parsing | `PasswordLogin` only locks and validates then emits a session (`auth_password.go:225–249`); legacy lookup/creation is `auth.go:174`; JWT parsing is `middleware/auth.go:245`. No security mutation occurs. No separate refresh-token/account-wide logout endpoint was found. |
| One logout | `Handler.Logout` (`auth.go:733`) only clears auth cookies. Persistent dispatch intent remains. |
| Fresh token issuance / renewal | `IssueCliToken` (`auth.go:694`), password variant (`auth_password.go:432`), PAT creation/renewal and `LockPasswordSession`. PAT renewal extends its own expiry (`personal_access_token.go:268–285`). A user lock by itself must never increment the neutral epoch. |
| Revoke one PAT or expire/delete individual task/daemon tokens | `Handler.RevokePersonalAccessToken` (`personal_access_token.go:311`) revokes a single id and invalidates that hash cache. Scoped token revocation is not global account revocation. Expiration cleanup in task service is similarly not an account event. |
| Source workspace leave/kick | `DeleteMember`/`LeaveWorkspace` (`workspace.go:648`, `:709`) call `revokeAndRemoveMember` (`workspace_revoke.go:43`). It owns a workspace-first tx, removes that workspace's resources/subscriptions/member. D01/R36 (`design.md:47–49`) explicitly preserves participant, creator/handler and neutral account epoch. Resource loss separately revokes its affected grants and pauses execution. Never reuse the full member-removal helper for account revocation. |
| Managed installation renew/rebind; legacy daemon hostname migration | `ManagedInstallationService.bind` (`managed_installation.go:434`) handles binding epoch at `:537–569` and revokes the old binding at `:550`; `daemon.go:740–753` describes legacy runtime merge. Their E07 effect is scoped installation/grant invalidation, not every command issued by the account. |
| New user registration or initial administrator creation | Password registration creates a new identity; `CreatePlatformAdminInitializationInTx` (`platform_admin_initialization.go:144`, `:160–172`) creates a new restricted user/credential/role and deliberately never takes over an existing user. There is no prior account authority to revoke. The later first password change is covered above. |
| Profile edits, onboarding, ordinary device identity reuse | `user.sql:21–87` changes display/profile/preferences/onboarding fields, never UUID/email/auth identity. `provisionDeviceIdentity` (`auth_device.go:291`) reuses its deterministic email identity or creates a fresh user. These are not account migrations. |
| Platform role demotion/removal/no-op | `ChangeRole` only globally revokes in its explicit `elevated` branch. Do not broaden this product behavior while connecting E07. |

### Meaningful PostgreSQL regression plan

Drive the real services/handlers. Directly incrementing the fence proves only the comparison already covered by `TestCollaborationQueueRejectsStaleIdentityAndMachine` (`collaboration_queue_test.go:313`, direct SQL at `:324`). It does not prove the production security writers.

1. **Disable → immediate restore before worker execution:** Extend `TestPlatformAccountDisableRestoreKeepsHistoryAndNeverRevivesCredentials` (`platform_account_test.go:18`). Create actual collaboration participation first; save command/owner authority at epoch 1; run real `ChangeAccount` twice. Assert epoch 3, two immutable pending events, old authority rejected even while enabled again, other account epochs unchanged. Reconcile later; old affected work remains invalid and session never auto-resumes.
2. **Legacy target without password credential and runtime-free target:** Extend `TestPlatformAccountLegacyDisableRestoreRevokesMigrationSession` (`platform_account_test.go:112`), plus a target with zero owned runtimes. Their existing collaboration fences must still advance and record events despite auth version 0 and the shared helper's early return.
3. **Every common caller:** Extend the real password setup/change handler tests (`auth_password_test.go:118`, `:50`, `:348`) and service tests for administrative recovery, deployment recovery/break-glass, role grant/promotion and bootstrap. Assert exactly one epoch increment and one event per successful security transaction. Assert password version still changes according to its existing independent rules. A fresh first-user registration produces no invalidation.
4. **Atomic rollback:** Use existing transaction wrappers (`platformAdminFailAuditStarter`, `queueFaultStarter`) or an equivalent scoped fault to fail outbox insertion after epoch update, fail audit after revocation, and fail a known-uncommitted commit. Assert disabled state/hash/version/PAT/task/daemon state, epoch, event and admin operation all roll back. Test `MaxInt64` collaboration epoch exhaustion separately from password-version exhaustion. Include last-admin rejection and stale actor/hash/version rejection.
5. **Uncertain committed response + same-key replay:** Extend `TestPlatformAdminUncertainCommitCanBeRecoveredWithoutApplyingTwice` (`platform_admin_test.go:541`) and account recovery replay (`platform_account_test.go:59`). A committed-but-error response leaves one epoch/event; recovery/replay changes neither. This also proves fanout cannot depend on `CredentialsRevoked=true` being returned to a publisher.
6. **Two-connection serialization:** Barrier a real account mutation against `SubmitCommand` while they contend for the same user row. Mutation wins → old expected account epoch is rejected; submit wins → old immutable command commits and is picked up by the later invalidation. Repeat first fence creation versus reset to prove no missed first-participation intent. Existing `platform_account_test.go:151`, `:200` and `collaboration_queue_test.go:344` show useful concurrent-test patterns.
7. **Durable worker recovery/idempotence:** Commit account invalidation with no event publisher/worker running (deployment path); start reconciliation later. Hold one source lock to force NOWAIT retry, crash/fail between issue batches, then retry. Assert no missing issue, no duplicate issue event/version increment and no completed event while skipped work remains. Process disable and restore events late, and preserve a legitimately new command/grant captured after the relevant event threshold.
8. **Author versus owner scope:** Use different author/resource-owner users and at least two sessions. Disable/reset the author; then independently reset owner. Ensure each discovers the appropriate stale commands/grants/runs, retains unrelated authors' waiting work, and never transfers attribution to owner. Include a revoked historical participant owning an unsettled run so fanout cannot rely only on active `issue_participant`.
9. **Termination barrier preservation:** A seeded run with any issued permit stays unsettled after invalidation despite generic task cancellation/offline state; fence/launch nonce/historical termination token stay unchanged. A server-proven zero-permit run may settle only under the same guards used by Stop. No real provider executable required. Positive admission/native-proof correctness remains a separate P4/P5 gate.
10. **Negative controls:** Real login/logout, PAT renewal/single-PAT revocation, profile edit, source workspace leave and platform demotion leave account epoch and invalidation event count unchanged. Source departure preserves collaboration participation/lifecycle control; scoped resource invalidation can still pause the relevant session.

**Critical fixture change before any such tests:** `newPlatformAdminFixture` (`platform_admin_test.go:90–103`) clones a fixed table allowlist and sets `search_path=private_schema,public`. Add `collaboration_account_fence`, the new account invalidation table, and every collaboration table used by the new fixture/consumer to the private clone list. Otherwise the shared auth helper can silently resolve those tables in `public`. Other fixed-list private fixtures must be checked similarly. Use task-owned PostgreSQL and existing `testutil.Fixture`/`testutil.Call`; this research ran no DB tests.

### Files found

- `server/internal/auth/password_revoke.go` — common borrowed credential-revocation boundary.
- `server/internal/auth/password_session.go` — password-session checks and shared user-row lock for minting.
- `server/internal/auth/temporary_disabled_users.go` — hard-coded emergency denylist, not a transactional writer.
- `server/internal/handler/auth_password.go` — password setup/change/login and post-commit disconnect publisher.
- `server/internal/handler/auth.go` — legacy login lookup, JWT issuance, logout and profile writes.
- `server/internal/handler/auth_device.go` — deterministic device identity reuse/creation.
- `server/internal/handler/personal_access_token.go` — scoped PAT issuance/renewal/revocation.
- `server/internal/handler/admin_user.go` — HTTP account governance delegates to service.
- `server/internal/service/platform_account.go` — disable, restore and administrative password recovery.
- `server/internal/service/platform_admin.go` — sorted user locking, role elevation, bootstrap, audit/idempotency.
- `server/internal/service/platform_password_recovery.go` — deployment recovery with borrowed tx.
- `server/cmd/server/password_recovery.go` — deployment recovery transaction owner, before server startup.
- `server/internal/service/platform_admin_initialization.go` — new restricted administrator creation, not takeover.
- `server/internal/service/collaboration.go` — first participation fence initialization and sorted user locking.
- `server/internal/service/collaboration_queue.go` — current account comparison, Stop guards and per-issue events.
- `server/internal/service/admin_operation.go` — durable leased reconciliation pattern, scoped to task cancellation.
- `server/internal/handler/workspace_revoke.go` — workspace-scoped departure transaction and effects.
- `server/internal/service/managed_installation.go` — installation/binding epoch changes and renewal.
- `server/pkg/db/queries/{password,admin,admin_user,user,collaboration,runtime,admin_operation}.sql` — authoritative security/lock/fanout query definitions.
- `server/migrations/571_collaboration_schema.up.sql` — account fence, command/grant/run and issue-only outbox schemas.
- `server/internal/service/{platform_account,platform_admin,platform_password_recovery,collaboration_queue}_test.go` — existing real DB test/fault/concurrency primitives.
- `server/internal/handler/{auth_password,auth_password_change,auth_password_revocation}_test.go` — real handler credential behavior and target-versus-actor disconnect tests.

### Related specs and external references

- Task `research/execution-contract.md` E07 (`:121–136`) fixes neutral account epochs and exclusion of ordinary login/logout/expiry; E02 (`:32–54`) fixes short account transactions and durable ordered fanout; E01 requires author + owner comparisons at every execution boundary.
- Task `implement.md:115`, `:132` requires registering real epoch writers and durable invalidation that survives rapid restoration.
- Task `design.md:47–49` preserves participation on workspace departure; D06 (`:102–124`) defines cancellation, pause and proof rules; D07 (`:137–143`) retains historical termination capabilities after account revocation.
- `.trellis/spec/server/security-boundaries.md` — password revocation retains memberships/installations; KDF outside locks; platform advisory → sorted user order; transaction/audit/idempotency; publish target disconnect after commit.
- `.trellis/spec/server/collaboration-foundation.md` — current fence comparisons are incomplete without E07 production integration; execution remains gated.
- `CLAUDE.md` — migration/index constraints, generated sqlc workflow, real DB fixtures and evidence requirements.
- External documentation was not needed for this source mapping and none was fetched. Local versions: Go `1.26.6`, `github.com/jackc/pgx/v5 v5.9.2` (`server/go.mod:3`, `:18`); generated db wrapper identifies sqlc `v1.31.1` (`server/pkg/db/generated/db.go:3`).

## Caveats / Not Found

- No production neutral epoch increment or account invalidation outbox exists in the inspected source. All proposed helper/table/reconciler names are recommendations, not existing APIs.
- No independent user-ID/email merge/reassignment handler, global logout-all endpoint, or generic legacy-JWT global revocation service was found. `PasswordSetup` is the existing identity migration to integrate. Future real migration/global-revoke writers must use the neutral borrowed primitive; do not build a speculative migration API now.
- The emergency denylist is compiled code (`temporary_disabled_users.go:12`). Adding/removing an entry is not a DB write transaction, so it cannot automatically persist E07 invalidation. Current collaboration checks reject listed accounts, but removing a denylist entry alone cannot prove durable revocation of old intent. Treat any operational change to that list as needing the existing persistent account-security path or an explicit migration; do not claim this helper covers external configuration/code edits or JWT secret rotation.
- Password setup has no old collaboration authority within the current restricted setup session, but may migrate an existing legacy user with retained collaboration data after deployment auth-mode change. Do not omit it just because the migration session cannot issue new commands.
- The foundation does not implement full positive admission, grant/enrollment mutation, native termination or collaboration event delivery. Proving the six security writers advance epochs and write intent does not prove those later execution contracts.
- Read-only research apart from this artifact. No source/config/task metadata edits, git operations, DB tests or dependency changes were performed.
