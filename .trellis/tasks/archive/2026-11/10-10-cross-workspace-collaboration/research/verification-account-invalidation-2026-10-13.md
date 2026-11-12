# Account security invalidation — implementation evidence

Date: 2026-10-13. Narrow E07 production-writer slice; P4/P5 remain incomplete.

## Implemented boundary

`auth.RevokePasswordCredentials` now advances an existing neutral collaboration
account fence and inserts durable account invalidation intent in the caller's
credential/account transaction. The six existing callers already hold the target
user row lock; administrative callers retain their platform advisory lock and
sorted actor/target locks. The new SQL takes no source issue/session locks and
does not create collaboration state for an account without a fence.

The replay identity is explicit: `password:<new session_version>` for password
setup/change, and `admin:<persisted operation UUID>` for account governance,
role elevation, bootstrap and deployment recovery. The original proposal to key
only by password version was rejected after finding the real disable/restore
branch for legacy users without a password credential. `credential_version` is
nullable provenance; it is never an invented version 0. Bootstrap and deployment
recovery now invoke revocation after their existing operation insert, within the
same transaction and before audit/commit.

The SQL CTE advances the fence and inserts its immutable user/epoch/key/reason
record atomically. Repeating the same key does not increment twice. Outbox,
audit and epoch-overflow errors fail the security transaction; no partial
disable, credential, token, fence or intent survives. Earlier pending records
remain after fast restore. Existing memberships and collaboration participation
are preserved.

## Files owned by this slice

- `server/internal/auth/password_revoke.go`
- `server/pkg/db/queries/collaboration_invalidation.sql`
- `server/internal/handler/auth_password.go` — two call sites only
- `server/internal/service/platform_account.go`
- `server/internal/service/platform_admin.go`
- `server/internal/service/platform_password_recovery.go`
- `server/internal/service/platform_admin_test.go` — private-schema clone list
- `server/internal/service/collaboration_account_invalidation_test.go`
- `server/internal/handler/auth_password_collaboration_test.go`
- `server/internal/handler/auth_password_test.go` and
  `server/internal/handler/password_claim_test.go` — existing fixture call keys

The parent owns migrations 613–616, generated sqlc code and schema regression
coverage. This slice does not claim those checks as its own.

## Verification

Database: task-owned `multica_collaboration_workflow_1791893622377`, freshly
migrated through 616 by the parent. Source the private environment with export
enabled; never print its contents:

```sh
set -a
source /tmp/multica-collaboration.Qp1TA6/workflow-env
set +a
[[ "$DATABASE_URL" == *"/multica_collaboration_workflow_1791893622377"* ]] || exit 1
```

Commands run from `server/`:

| Command | Result |
| --- | --- |
| `go test -race ./internal/service -run '^TestCollaborationAccountInvalidation' -count=1 -v` | PASS; 8 top-level tests plus 6 subtests, zero skips/failures; 71.668s |
| `go test -race ./internal/handler -run '^TestPasswordCollaborationInvalidation' -count=1 -v` | PASS; 3 tests, zero skips/failures; 20.656s |
| `go vet ./internal/auth ./internal/service ./internal/handler` | PASS, exit 0 |
| Existing platform regression suite (command below) | PASS; 41 top-level tests plus 38 subtests, zero skips/failures; 159.774s |
| `gofmt -l` on all modified Go files, scoped `git diff --check`, whitespace scan of all 11 owned source/test files | PASS, no formatting errors |

```sh
go test -race ./internal/service -run '^TestPlatform(Admin|Account|PasswordRecovery)' -count=1 -v
```

Combined final runs: 52 top-level tests plus 44 subtests passed, zero skips or
failures. The existing suite covers administrative initialization, last-admin
protection, role/account mutation, credential revocation, uncertain commit
recovery, break-glass and audit rollback under the revised common primitive.

Logs live under `/tmp/multica-collaboration.Qp1TA6/` as
`workflow-account-service.log`, `workflow-account-handler.log`, and
`workflow-account-regression.log`.

The initial service invocation sourced unexported variables and skipped its DB
tests; it is not passing evidence. The simultaneous handler invocation was
interrupted during compilation with no TestMain/test output. Both commands were
rerun with exported environment and an explicit task-database name guard; only
those final runs are counted above.

New real-service evidence covers:

- Disable, identical operation replay, rapid restore, legacy disable/restore
  without a credential row, and legacy first-password recovery.
- A real `SubmitCommand` saved before disable retains immutable old epoch/content.
  An undelivered intent created before disable is refused as `stale_authority`
  after restore even with a valid current password session; a fresh epoch intent
  succeeds. This does not claim actual promoter admission or cancellation fanout.
- All six production call-site categories, including actual HTTP setup/change,
  role elevation, bootstrap and deployment recovery. Acting-admin authority is
  not revoked in place of the target.
- Outbox insertion failure, later audit failure and bigint epoch exhaustion each
  roll back account, credential, PAT, fence and intent writes.
- Same-key borrowed-transaction replay, source issue lock independence, and a
  deterministic first-participation race observed through `pg_blocking_pids`:
  account disable waits on the shared user lock, then invalidates the newly
  committed fence.
- Ordinary accounts create no fence/outbox. Actual login/logout, JWT minting,
  profile edit, PAT creation/renewal/single revocation, role removal and rejected
  forced same-password change preserve collaboration epochs.

## Remaining boundaries

- No account invalidation consumer/fanout is implemented here. Pending intents
  still need ordered source convergence to cancel obsolete waiting commands,
  revoke grants and persist pause/stop effects.
- No positive Resume, promoter/claim/permit, runtime enrollment, business token
  or native termination proof is implemented or enabled by this slice.
- Generic runtime task cancellation remains the existing behavior; it is not
  proof that collaboration execution ended and cannot clear its run barrier.
- Compiled emergency-denylist edits, JWT-secret rotation and future independent
  identity-migration/global-revoke writers have no database transaction in this
  path. They require explicit persistent invalidation integration; removing a
  deployment denylist entry alone does not prove old intent was invalidated.
- No source-workspace departure writer was changed. The existing account
  participation-preservation contract still applies.
- No real provider CLI, external AI account, native process containment,
  Web/Desktop E2E, full `make check`, commit or deployment was run by this slice.

Public creation and positive execution remain gated. These results are narrow
writer/transaction evidence, not complete acceptance of any P4/P5 feature.
