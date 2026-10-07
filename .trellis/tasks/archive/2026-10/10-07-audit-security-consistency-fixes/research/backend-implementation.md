# Backend implementation: R2 / R5 / R6

Date: 2026-10-07. Owner: `audit_backend` / dispatched `trellis-implement`.
Scope: backend bridge principal boundary and standalone project-resource concurrency only. No dependencies, migrations, SQL/sqlc changes, commits or production operations.

## Files changed

- `server/cmd/server/router.go`: `RequireHumanActor` immediately after Auth on the browser bridge group.
- `server/cmd/server/plugin_action_routes_test.go`: genuine task/PAT/JWT/cookie/cloud-verifier router matrix in legacy and password modes, same/cross-workspace installations, all ten bridge operations, no mutation/hook assertions, human controls, dedicated installation/callback controls and attribution.
- `server/internal/handler/plugin_action.go`: first-operation machine-source backstop in `pluginSessionCaller`.
- `server/internal/handler/plugin_action_test.go`: task/cloud sources rejected before accessing an absent service, independent of routing.
- `server/internal/handler/project_resource.go`: READ COMMITTED project transactions; exclusive scoped parent lock before fresh child reads, conflict validation, capability checks, append-position calculation and partial-field merge. Events remain after committed success. Request-owned label/position decoding is outside retries; stored-state decisions are recomputed inside every attempt.
- `server/internal/handler/project_resource_concurrency_test.go`: deterministic distinct-user create/create, update/update, create/update, ref plus label/position, different-daemon append-position tests; commit-lock/event, write-failure and commit-failure assertions.
- This implementation record.

Simplifications: reuse `RequireHumanActor`, the existing machine classifier, project transaction/retry runner, exclusive project lock and legacy label helpers. Change `findLocalDirectoryConflict` to accept transaction-bound queries. Reuse existing test fixture/HTTP helpers and transaction wrappers. No second transaction service, duplicate authorization classifier, new API revision field or schema constraint.

## Isolated environment

Created exactly one disposable database, `audit_backend_a096cd642bf243da`, on `127.0.0.1:5432`, role `multica`. The database name is also recorded in `/tmp/multica-audit-backend-db`; no credentials are stored in this task document. Every DB test/migration shell supplied an explicit `DATABASE_URL` targeting this database. Never used the default application DB, real agents or installed agent CLIs.

- Maintenance connection: local `postgres` database, only `CREATE DATABASE audit_backend_a096cd642bf243da`.
- Migration: `go -C server run ./cmd/migrate up`; exit 0, through migration `566_iteration_notification_day` (590 ledger rows).
- Preflight: `psql "$DATABASE_URL" -w -v ON_ERROR_STOP=1 -Atc 'SELECT current_database(), current_user, count(*) FROM schema_migrations GROUP BY 1,2'` returned `audit_backend_a096cd642bf243da|multica|590`.
- Migration log: `/tmp/multica-audit-backend-migrate.log`.
- Cleanup completed after independent review at the leader's explicit request. Verified `pg_stat_activity` contained zero connections to `audit_backend_a096cd642bf243da`, executed `DROP DATABASE audit_backend_a096cd642bf243da` without `FORCE`, and verified `pg_database` contained zero matching databases. Only this task-created database was dropped; no credentials were printed.

## Red evidence before production edits

All Go tests ran through `bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -p 2 -parallel 2 ... -count=1 -v`, with the explicit disposable `DATABASE_URL`.

1. `./internal/handler -run TestProjectResourceConcurrent`
   - `/tmp/multica-audit-resource-red.log`; exit 1; eight failing test/subtest events, zero skips.
   - Create/create, update/update and create/update each returned two successes, zero conflicts, and two target-daemon rows.
   - Concurrent label/ref edit lost the execution path; concurrent position/ref edit lost position 7 back to 0.
   - Two different daemons appended at one shared position.
   - Harness observes both request backend PIDs in direct/transitive lock waits on a fixture-held project lock; distinct active users prevent the subscriber advisory fence from concealing the stale child snapshot. Deadlines/cancellation, rollback and joins clean up blocked requests. An initial harness setup incorrectly replaced chi's URL context; that was corrected before recording the behavioral red run above.
2. `./cmd/server -run TestPluginBridgePrincipalBoundary`
   - `/tmp/multica-audit-plugin-red.log`; exit 1; 67 failing and 32 passing test/subtest events, zero skips.
   - Genuine task tokens returned 200/201/204 across the bridge, including local TLS hook invocation; cloud machine credentials also reached member actions in legacy mode.
   - Protected issue title/revision, comments and storage changed; hook invocation records and outbound hook calls appeared.
   - Human controls and password-mode unsupported cloud rejection already passed.
3. `./internal/handler -run TestPluginSessionRejectsMachineBeforeInstallationLookup`
   - `/tmp/multica-audit-backstop-red.log`; exit 1; both machine sources reached the deliberately absent service instead of early 403. The test catches that lookup panic and reports the boundary failure.

## Green verification

The following are completed real runs, not audit-baseline evidence. All commands use the same guarded Go prefix and explicit disposable DB URL noted above, except `go vet`/gofmt/diff checks.

| Command suffix / check | Evidence | Result |
|---|---|---|
| `./internal/handler ./cmd/server -run 'TestProjectResourceConcurrent\|TestProjectResourceTransactionCommitBoundary\|TestPluginSession\|TestPluginBridgePrincipalBoundary' -count=1 -v` | `/tmp/multica-audit-backend-final.log` | Exit 0; 117 pass events, zero skips. Handler 3.682s, router 2.553s. Includes worktree-capable runtime/path+mode concurrent edits and forged human machine-source header controls. |
| `./internal/handler -run 'TestProjectResource\|TestCreateProject(AttachesResources\|WithResourcesEchoesCount\|RollsBackOnInvalidResource\|BundledLocalDirectoryDaemonConflict\|GatesWorktreeLocalDirectory)\|TestPlugin\|TestCallback\|TestInvokePluginHook\|TestPasswordCallback\|TestProjectAssociation\|TestProjectUpdateMembership' -count=1 -v` | `/tmp/multica-audit-handler-neighbors.log` | Exit 0; 147 pass events, zero skips, 9.694s. Includes old-client rename, stale ref label, capability, association and membership fences. |
| `./cmd/server -run 'TestPlugin(ActionRoute\|Bridge\|Session)\|TestMachineCredential' -count=1 -v` | `/tmp/multica-audit-router-final.log` | Exit 0; 116 pass events, zero skips, 2.439s. Removed alias and credential-mint boundary retained. |
| `./internal/service -run 'TestTriageProjectResourceWritesSerializeWithExecutionSnapshot\|TestPasswordCallback\|TestHook' -count=1 -v` | `/tmp/multica-audit-service-final.log` | Exit 0; 18 pass events, zero skips, 1.988s. Execution snapshot blocks all resource write kinds. |
| `./internal/middleware -run 'TestAuth_\|TestPlugin\|TestPassword' -count=1 -v` | `/tmp/multica-audit-middleware-final.log` | Exit 0; 25 pass events, two optional Redis skips, 1.775s. |
| `./cmd/server -run TestPluginBridgePrincipalBoundary -count=1 -v` | `/tmp/multica-audit-plugin-verified.log` | Exit 0; 99 pass events, zero skips, 2.739s after final test-fixture error checks. |
| `go -C server vet -p 2 ./...` | `/tmp/multica-audit-vet.log` | Exit 0, no diagnostics. |
| `gofmt` changed Go files; `git diff --check` | Shell output | Clean. |

The two optional Redis skips are `TestAuth_PATCacheHit` and `TestPluginRateLimitIsPerCredentialAndUsesStableProblem` because `REDIS_TEST_URL` is unset. No `redis-server`/`redis-cli` executable is installed. No shared Redis database was connected or flushed. All DB-backed/new regression suites ran with real PASS events; there was no TestMain database skip.

The leader's added server security/project-resource specs were reviewed and match the implementation: Auth then human gate plus handler backstop; workspace/member fences then project FOR UPDATE; READ COMMITTED fresh child reads; commit-before-event publication.

## Remaining boundaries / follow-ups

- Independent review is complete and the disposable database has been removed after confirming zero active connections. Any future rerun requires a newly created isolated database.
- Optional Redis cache/rate-limit checks remain unrun unless integration provisions its own isolated Redis instance.
- Existing duplicate resource data is not automatically repaired. Future direct resource writers must join the same parent-lock and validation contract.
- Explicitly resending an old full execution ref retains the existing compatibility semantics; this repair protects omitted fields and preserves legacy rename behavior.
- Full repository Go/TS/native verification and task integration remain leader-owned. No backend commit was created.
