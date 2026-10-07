# Research: Backend remediation plan for R2, R5 and R6

- Query: Find the smallest verified design for plugin session actor isolation and project-resource uniqueness/partial-update serialization; specify implementation ownership and deterministic regressions.
- Scope: internal, planning only. No production code, tests, specs, git operations, task activation, database mutation or real-agent execution in this research pass.
- Date: 2026-10-07
- Baseline: task PRD/report identify 4a9be02e5; references below are from the currently shared working tree, without checking out or reverting anything.

## Findings

### Recommendation

R2 needs two production edits: the human-only route gate on the session bridge and a backstop in pluginSessionCaller. Keep the existing classifier and dedicated plugin token path.

R5/R6 need one transaction spanning the resource-set lock, fresh row/set reads, compatibility merge, validation and write. Reuse runProjectTransactionAtIsolation with READ COMMITTED and the existing exclusive LockProjectForExecutionSquad query. Calling runProjectTransaction unchanged would be incorrect here: its REPEATABLE READ snapshot can remain older than a preceding resource insert even after the project lock is acquired, because resource writes do not update the project tuple. No migration or SQL/sqlc change is necessary for this design.

### Files found and responsibility

| File | Relevant responsibility |
|---|---|
| server/cmd/server/router.go | Registers nine shared plugin actions at /v1 and at the session bridge, plus the session-only hook endpoint. |
| server/pkg/publicapi/v1/routes.go | Canonical action paths and method ledger. |
| server/internal/middleware/auth.go | Authenticates task/cloud/PAT/JWT callers and stamps authoritative actor source. |
| server/internal/middleware/password_auth.go | Password-mode credential support and task source stamping. |
| server/internal/middleware/plugin_auth.go | Dedicated /v1 boundary accepting mpi_/mpc_ only. |
| server/internal/handler/actor_guards.go | RequireHumanActor and isMachineCredentialActor; existing task/cloud machine classification. |
| server/internal/handler/plugin_action.go | pluginSessionCaller converts authenticated user to member; pluginTokenCaller separately implements installation/callback identities. |
| server/internal/handler/plugin_hook.go | UI/manual hook invocation uses pluginCaller then requires a member. |
| server/internal/service/plugin_action.go | Checks enabled installation and granted scopes, intentionally not authentication principal type. |
| server/internal/handler/project_resource.go | Standalone create/update, local-directory duplicate detection and old/new client label/ref merge. |
| server/internal/handler/project_write_fence.go | Workspace/member fences, isolation selection, bounded rollback-and-retry transaction runner. |
| server/internal/handler/project.go | Existing exclusive project lock usage, bundled resource create, deletion permissions and error-response sentinel precedent. |
| server/pkg/db/queries/project_resource.sql | Writes already take workspace KEY SHARE then project NO KEY UPDATE, but reads/merges occur before these statements today. |
| server/pkg/db/queries/project.sql | LockProjectForExecutionSquad is FOR UPDATE; LockProjectForAssociation is only FOR SHARE. |
| server/pkg/db/queries/subscriber.sql | Per-workspace/user advisory fence and active-member SHARE lock. |
| server/internal/testutil/ | Fixture rows/cleanup and testutil.Call request/response helpers. |
| server/internal/handler/project_resource_test.go | Existing lifecycle, daemon uniqueness, stale-ref rename and capability compatibility tests. |
| server/internal/handler/project_update_concurrency_test.go | progressTxStarter/progressHookTx wrappers; observed DB lock waits and membership revocation tests. |
| server/internal/handler/project_association_concurrency_test.go | Commit barriers and parent-lock lifetime checks. |
| server/internal/service/triage_project_resource_fence_test.go | Existing execution fingerprint/resource-write mutual exclusion tests. |
| server/cmd/server/machine_credential_mint_test.go | Real Auth route fixture for task tokens and human PAT/JWT controls. |
| server/cmd/server/plugin_action_routes_test.go | Existing trust-boundary routing and removed-alias assertions. |
| server/internal/handler/plugin_action_test.go, plugin_callback_test.go, plugin_hook_test.go, plugin_password_test.go | Plugin fixtures, install/callback identity, scope, hook and password revocation coverage. |
| server/internal/service/plugin_hook_transport_test.go | Local TLS hook receiver, HookClient/DevOrigins setup and signed callback controls. |
| server/internal/auth/cloud_pat_test.go | Local Fleet /api/v1/pat/verify stub contract. |
| server/cmd/server/platform_admin_auth_test.go | Versioned JWT/PAT/task fixture construction and cookie/CSRF request pattern; reuse small helpers, not admin policy. |
| scripts/go-test-with-agent-cli-guard.sh, scripts/test-go.sh, Makefile | Guarded Go checks, full race entrypoint and pinned sqlc generation. |

### R2 root-cause proof and complete route inventory

The authority chain is visible end to end:

1. middleware/auth.go:61 deletes a client-provided X-Actor-Source. The task branch at :93 sets the token owner's user, actual agent/task/workspace and task_token source at :110–118. The cloud branch sets cloud_pat at :178. Password mode similarly stamps task_token in password_auth.go:66.
2. router.go:1568 applies general Auth to the bridge without RequireHumanActor. The routes do not pass through Workspace middleware.
3. plugin_action.go:127 reads only X-User-ID, authorizes the client-selected installation, derives its workspace and checks the owner's membership there; :158 returns Type member. It neither rejects machine provenance nor preserves the task-bound workspace.
4. service/plugin_action.go:46 only checks installation existence, enabled state and scopes. Those are necessary but cannot restore the lost principal boundary.
5. plugin_hook.go:35 calls pluginCaller, then :42 only checks actor.requireMember. The elevated member reaches the UI/manual hook and becomes its member callback principal.

All session routes currently present, from router.go:103 and :1570 and publicapi/v1/routes.go:14:

| Method | Session route |
|---|---|
| GET | /api/plugin-bridge/v1/context |
| GET, PATCH | /api/plugin-bridge/v1/issues/{issue_ref} |
| GET, POST | /api/plugin-bridge/v1/issues/{issue_ref}/comments |
| GET | /api/plugin-bridge/v1/storage/{scope} |
| GET, PUT, DELETE | /api/plugin-bridge/v1/storage/{scope}/{key} |
| POST | /api/plugin-bridge/v1/hooks/{key} |

There are ten method/path operations and one session prefix, not multiple surviving session aliases. The same nine resource operations also exist under /v1 behind PluginBearerOnly (router.go:1554); /v1/hooks/{key} is absent. /api/v1/plugin/context is a removed legacy prefix, explicitly expected to return 404 in plugin_action_routes_test.go:46. Preserve that assertion; do not restore the alias. The daemon route /api/daemon/tasks/{id}/plugin-hooks uses DaemonAuth and InvokeAgentPluginHook and is a different supported machine surface, not a session alias. Plugin management and surface-launch routes are also outside this finding.

R2 implementation shape:

- Add r.Use(handler.RequireHumanActor) immediately after Auth in the bridge-only group. Do not put this on registerPluginActionRoutes, the generic Auth middleware or the /v1 group.
- Make the first operation in pluginSessionCaller an isMachineCredentialActor check, returning 403 via publicapiv1.WriteProblem with the existing forbidden code and a human-only explanation. This protects direct invocation/future route registrations before installation lookup, member lookup or action data reads/writes. It also covers InvokePluginHook without adding a parallel guard there.
- Keep pluginCaller dispatch to pluginTokenCaller before reaching the session backstop. No change to token prefixes, callback identity, scopes, membership/revocation checks, or the classifier's deliberately supported human PAT behavior.
- Reuse actor_guards.go:125 classifier exactly: task_token and cloud_pat are denied; human JWT/cookie and mul_ PAT pass. Do not substitute resolveActor, X-Agent-ID, token prefix guessing, password-session-only guards or a JWT-only rule. Unknown actor source semantics are explicitly documented and out of scope.
- Route rejection retains the existing RequireHumanActor App API error shape; direct Action-handler rejection uses its existing Public API problem writer. Auth errors already occur before the Action DTO contract. No new response schema is needed because no success DTO changes.

R2 regression plan:

1. Extend server/cmd/server/plugin_action_routes_test.go with real NewRouterWithOptions tests using enabled plugin flags, two workspaces, the same human as an active member of both, and valid installations/scopes/issues in each. Use task_token fixtures from machine_credential_mint_test.go:26–49. Every operation above must receive 403 for valid mat_ against same-workspace and cross-workspace installations. Submit forged X-Actor-Source: member and alternate workspace/agent headers in a representative case; real Auth must overwrite them and the gate must still deny.
2. Use valid request bodies, valid issue ETags for PATCH, existing storage keys for GET/DELETE, and declared UI/manual hooks. A malformed request or missing scope is not proof of actor isolation. Assert issue/revision unchanged, no new comment/storage/invocation rows, and zero local hook receiver requests after denial. For reads, assert denial rather than content and include direct-backstop tests with no usable PluginService so an attempted installation lookup would fail the test.
3. Repeat task/JWT/cookie/mul_ controls in legacy and password auth modes. Password fixtures carry current auth_version and a completed account; cookies include matching CSRF headers for mutations. Unsupported/revoked credentials retain their preexisting authentication rejection.
4. For legacy mcn_, construct a local Fleet verifier response (auth/cloud_pat_test.go:37) returning the fixture owner and set MULTICA_CLOUD_URL before building the local router. Verify all bridge methods return 403 after successful Auth. In password mode mcn_ is intentionally unsupported and remains 401 at Auth (password_auth.go:46); do not demand 403 there or add Fleet fallback.
5. Prove permitted humans work: JWT bearer, cookie+CSRF and real mul_ PAT should successfully read and write through the bridge and invoke a declared hook. For JWT/PAT actor attribution, assert member ID plus via_plugin_id. Keep a forged machine-source human control to prove Auth discards caller headers.
6. Keep /v1 supported modes: real mpi_ install token read/comment writes as plugin; real mpc_ user callback writes as its member; event callback writes as plugin. Repeat in password mode with source-version validation and existing revocation controls. JWT/cookie/mul_/mat_/mcn_ cannot use /v1. Use the local TLS hook receiver and configured HookClient/DevOrigins/Callbacks; no external plugin endpoints.
7. Add focused direct tests in plugin_action_test.go for the pluginSessionCaller backstop (task_token/cloud_pat denied before service access, human session allowed), and ensure a machine InvokePluginHook reaches no outbound transport. Existing password/callback suites remain canonical for detailed revocation/scope matrices; avoid cloning those matrices into every route case.

### R5/R6 root-cause proof

R5: CreateProjectResource calls findLocalDirectoryConflict at project_resource.go:515, but takes the parent lock only inside CreateProjectResource SQL at project_resource.sql:31. Two requests can both read an empty set, then serialize their INSERTs with different local_path values. The full-ref unique constraint does not conflict for those values. Update follows the same sequence at project_resource.go:622 and :730. A row lock on the target resource alone cannot protect a uniqueness predicate across two different resource rows.

R6: UpdateProjectResource reads existing at project_resource.go:590, builds nextRef/nextLabel/nextPosition from that row, then project_resource.sql:53 writes all three values after acquiring its internal parent lock. Two partial edits based on the same old row both succeed; whichever writes last restores at least one old value. The current stale-ref regression at project_resource_test.go:1563 exercises sequential old-client snapshot semantics, not overlapping server reads.

The child CTE lock is intentionally important for execution safety: triage_project_resource_fence_test.go:14 verifies resource mutations wait while execution fingerprinting holds project FOR SHARE. Keep those SQL fences even after adding the earlier handler lock. Bundled CreateProject already validates duplicate daemon IDs in its input and creates a new parent/resources in one transaction (project.go:439 and :534), so it does not need this standalone-write repair.

### Exact transaction, permissions and compatibility design

Use runProjectTransactionAtIsolation(ctx, workspaceID, parsedUserID, pgx.ReadCommitted, callback) directly for these two handlers. Existing runner at project_write_fence.go:86 owns bounded retries (40001, 40P01, 55P03), rollback and commit. Its lock order is:

1. SET TRANSACTION ISOLATION LEVEL READ COMMITTED, READ WRITE, before any query.
2. LockWorkspaceForChatSessionCreate: workspace FOR KEY SHARE.
3. LockSubscriberWrites: existing transaction advisory fence for workspace/user.
4. LockActiveMember: current member row FOR SHARE, with membership failure propagated.
5. In callback, LockProjectForExecutionSquad(projectID, workspaceID): project FOR UPDATE.
6. Fresh resource read/list and all derived decisions, followed by resource INSERT/UPDATE (which reacquire already-owned workspace/project fences and then the child row).
7. Commit through the runner; then build/publish the response event and return the HTTP success.

Do not use LockProjectForAssociation (FOR SHARE): compatible shared locks do not serialize writers and later lock upgrades add deadlock risk. Do not change the default runProjectTransaction isolation for unrelated callers. Do not use the delete-named wrapper for resource writes. A separate named lock query or transaction wrapper is unnecessary unless implementation readability demands it; the existing exclusive query already serves general project writes in project.go:647.

READ COMMITTED is essential: the runner's workspace/member queries take an RR snapshot before waiting on the project. A preceding resource-only transaction does not update the project tuple, so FOR UPDATE alone does not force a serialization failure/new snapshot under RR. At RC, the next separate resource SELECT after acquiring the project lock sees the preceding child's commit. The exclusive parent lock prevents later compliant resource writers during the read/merge/write sequence. Do not fold fresh resource reads into the same statement that waits for the parent lock.

Preserve existing resource permissions. These handlers currently require authenticated workspace membership and allow the existing machine/agent semantics of resource routes. The transaction runner's active-member recheck is the correct reuse. projectHumanActor would introduce a new human-only product restriction; requireProjectAdministrator is for deletion and would introduce a new role restriction. Use validated current user UUID as the member-fence actor, not either helper. Existing router Workspace middleware and loadProjectForResource enforce workspace ownership, but the transaction must recheck the project with its workspace after locking, handling deletion as 404. Resource IDs must still belong to the locked project and workspace.

Create callback:

- Parse/decode and normalize request-owned fields once before retries.
- Acquire the exclusive scoped parent lock, then call conflict detection using qtx, with no excluded resource.
- Perform capability validation through transaction-bound queries; preserve its existing 422 body. Count default append position within this transaction; check the count error instead of silently ignoring it.
- Insert using qtx and keep the returned row for post-commit publication. Map daemon duplicate and existing full-ref uniqueness violations to the existing 409 messages.

Update callback:

- Decode the raw field-presence map once before retries. Do not reread the request body in a callback.
- Acquire the exclusive scoped parent lock, then GetProjectResourceInWorkspace using qtx; compare resource.ProjectID to the locked project. No existing resource snapshot used for merging may come from outside the callback.
- Rebuild nextRef, refProvided, refRenameOnly, nextLabel/labelCleared and nextPosition from that fresh row on every attempt. Keep localDirectoryRefDiffersOnlyByLabel, withLocalDirectoryRefLabel, unknown stored JSON keys, explicit null/empty label clears, and null/omitted position semantics unchanged.
- Check daemon uniqueness using qtx and exclude the freshly loaded row ID. Preserve the worktree gate only for genuine execution-ref edits, including its legacy rename exemption and its existing runtime selection behavior.
- Update with qtx; keep success publication and response outside the retry callback.

The smallest helper adjustment is the following package-local signature (only the create/update callers use it today):

    func findLocalDirectoryConflict(ctx context.Context, q *db.Queries, projectID pgtype.UUID, resourceType string, normalizedRef json.RawMessage, excludeID pgtype.UUID) (bool, error)

Its list read uses q.ListProjectResources; each caller supplies the transaction qtx. The exact existing transaction signature to reuse is:

    func (h *Handler) runProjectTransactionAtIsolation(ctx context.Context, workspaceID, actorID pgtype.UUID, isolation pgx.TxIsoLevel, fn func(pgx.Tx, *db.Queries) error) error

The callback calls qtx.LockProjectForExecutionSquad with db.LockProjectForExecutionSquadParams{ID: projectID, WorkspaceID: workspaceID}; this is FOR UPDATE. The transaction argument is pgx.ReadCommitted. For capability validation, a callback-local Handler copy with Queries=qtx can reuse requireWorktreeCapableDaemon without mutating the shared Handler or changing bundled-create callers. If it writes an existing validation error, return errProjectResponseWritten and return without a second response after the runner; project.go:672–778 already uses this convention. An implementation may instead factor a small error-returning capability helper, but it must preserve the 422 payload, keep query errors visible and avoid a broad refactor. Never call the resource handler recursively with a Handler copy or begin a nested transaction.

No external work or events inside the callback. Failed attempts must discard attempt-local rows; publish only the final committed row once. Commit error is not success and must not produce an event. Existing DeleteProjectResource CTE already takes a conflicting parent lock, so it participates without a handler rewrite. A resource deleted while an update waits becomes 404 after the fresh read. Preserve bundled-create and execution fingerprint behavior.

### Deterministic database regressions that fail before the fix

Add server/internal/handler/project_resource_concurrency_test.go. Use dbfx.Project, dbfx.User, dbfx.Member, dbfx.Runtime and dbfx.Insert; use testutil.Call and the existing URL-param helpers. Copy testHandler per request rather than changing the shared global during goroutines.

Test harness:

1. Seed one project and two distinct active member users. Distinct users matter: the shared runner serializes the same user's transactions on LockSubscriberWrites before they reach the project; a same-user-only test would conceal the RR phantom bug.
2. Acquire a fixture transaction on one connection and lock the project FOR NO KEY UPDATE. This blocks the current write CTEs and the proposed earlier exclusive lock.
3. Acquire one dedicated pgx connection per request. Set each handler copy's Queries=db.New(conn) and TxStarter=conn; record each backend PID. No tracer or production hook is needed for the primary red/green tests.
4. Start both HTTP handlers with context deadlines and buffered result channels. Use another connection to query pg_stat_activity/pg_blocking_pids until both request PIDs are in real lock waits with a direct or transitive dependency on the fixture holder. PostgreSQL can queue the second tuple lock behind the first request, so requiring the holder as both direct blockers is incorrect. A small bounded poll interval is fine; elapsed time alone must never satisfy the barrier.
5. Release the holder only after observing both waits, then await both responses and inspect committed rows. Always release locks/connections and join goroutines on failure; use request context deadlines instead of leaked blocked goroutines.

Before the fix both requests have completed the stale reads/checks by the time they block in their SQL writes. After the fix they block before the authoritative reads. The same behavior-focused assertion therefore proves red and green without introducing an assertion about a chosen implementation hook.

Cases:

| Proposed test | Requests and final assertion |
|---|---|
| TestProjectResourceConcurrentCreateDaemonConflict | Empty set; POST two different paths on one daemon. Exactly one 201 and one 409; exactly one row for project/daemon. |
| TestProjectResourceConcurrentUpdateDaemonConflict | Two rows on distinct daemons; PUT both to a third daemon with different paths. Exactly one 200 and one 409; loser unchanged and one row for target daemon. |
| TestProjectResourceConcurrentCreateUpdateDaemonConflict | POST a new target-daemon row versus PUT an existing row to that daemon. Exactly one success and one 409, regardless of winning method. |
| TestProjectResourceConcurrentPartialEdits | Same row; one request changes path/execution mode, the other only label or only position. Both 200 and final row contains both changes. Include a capable runtime fixture when changing to worktree; verify both label homes remain synchronized. |
| TestProjectResourceConcurrentDifferentDaemons | Two POSTs for different daemons both succeed; serialized default positions come from current count. Confirms the fence does not incorrectly impose one resource per project. |

The partial-edit final-state assertion is order-independent: on the old implementation either last writer loses the other's change, while on the fixed implementation either serialization order preserves both. Keep sequential old-client rename tests intact; do not claim the fix can infer whether an explicitly resent stale execution-ref was intended as an old-client rename or a deliberate ref replacement. That ambiguity is outside R6's omitted-field contract.

Use existing progressTxStarter/progressHookTx and projectAssociationCommitStarter only for additional boundary proofs: pause a successful resource write before commit, assert another scoped connection cannot acquire the project and no EventProjectResourceCreated/Updated has published; inject write/commit failure and assert unchanged resource state and zero events. Event collection must be synchronized under race checks and scoped to the fixture project. The existing rollbackOnCommitTxStarter in invitation_test.go:156 is available in the same handler package.

Run the existing project deletion/association and TriageProjectResourceWritesSerializeWithExecutionSnapshot tests unchanged to catch lock-order or execution-snapshot regressions. Add a narrow membership-revoked-while-waiting control if the resource integration uncovers a mismatch; use the runner's established permission behavior, not a new authorization policy.

### Implementation file list and ordered checks

Expected production writes: server/cmd/server/router.go, server/internal/handler/plugin_action.go, server/internal/handler/project_resource.go. Expected test writes: server/cmd/server/plugin_action_routes_test.go, server/internal/handler/plugin_action_test.go, new server/internal/handler/project_resource_concurrency_test.go; extend existing plugin password/hook tests only where the real-router controls cannot cover a required path. No actor_guards, auth middleware, service, migration or generated-code change is required by this plan.

Order: add regressions and record the expected failures; implement R2 guard/backstop; implement R5/R6 shared transaction; run narrow behavior tests; run neighboring lock/auth suites; finish with race and Go vet. Do not describe the existing audit run as proof that new regressions pass.

All commands below run from repository root with DATABASE_URL explicitly set to a unique disposable database. Both handler and router TestMain default to the application's multica database when unset and exit successfully on an unreachable DB; preflight the explicit URL and require real test/subtest pass events with no "Skipping tests"/"Skipping integration tests" message. Do not rely on process exit zero alone.

    /opt/homebrew/opt/libpq/bin/psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -c 'SELECT current_database(), current_user;'
    go -C server run ./cmd/migrate up
    bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -p 2 -parallel 2 ./cmd/server -run 'TestPlugin(ActionRoute|Bridge|Session)|TestMachineCredential' -count=1 -v
    bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -p 2 -parallel 2 ./internal/handler -run 'TestProjectResourceConcurrent|TestPluginSession' -count=1 -v
    bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -p 2 -parallel 2 ./internal/handler -run 'TestProjectResource|TestCreateProject(AttachesResources|WithResourcesEchoesCount|RollsBackOnInvalidResource|BundledLocalDirectoryDaemonConflict|GatesWorktreeLocalDirectory)|TestPlugin|TestCallback|TestInvokePluginHook|TestPasswordCallback|TestProjectAssociation|TestProjectUpdateMembership' -count=1 -v
    bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -p 2 -parallel 2 ./internal/service -run 'TestTriageProjectResourceWritesSerializeWithExecutionSnapshot|TestPasswordCallback|TestHook' -count=1 -v
    bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -p 2 -parallel 2 ./internal/middleware -run 'TestAuth_|TestPlugin|TestPassword' -count=1 -v
    go -C server vet -p 2 ./...

Use gofmt on changed Go files. The integration owner can run the already approved broader Go suite with the same isolated DATABASE_URL through bash scripts/test-go.sh --race; do not invoke make test blindly because its environment-loading/database-provisioning steps may select a checkout database. If a SQL query is changed despite the reuse recommendation, regenerate only via make sqlc (pinned sqlc v1.31.1) and include the generated diff and existing fence regressions in review.

For the isolated database, connect to the local postgres maintenance database, create a uniquely generated lower-case audit database name, record that exact name, export DATABASE_URL for only this test shell, migrate it, and drop only that created database after all fixture pools/test processes exit. Do not use the existing application database, broad database cleanup or any real agent CLI. No DB was created during this research pass.

### Related specs and version references

- CLAUDE.md: backend UUID validation; preserve API compatibility; no new dependencies; no foreign keys/cascades; concurrent indexes in one-statement migrations; fixture/testutil use; guarded agent execution.
- .trellis/workflow.md: this is planning research; complex design/implementation artifacts precede task activation.
- .trellis/spec/server/security-boundaries.md: existing machine-credential classifier; router guard plus handler backstop; human PAT compatibility; password-mode callback version/revocation behavior; events after commit.
- .trellis/spec/server/project-execution-squad.md: exclusive project serialization, installed-client compatibility and resource parsing/daemon semantics.
- Versions from source: server/go.mod declares Go 1.26.6, pgx/v5 v5.9.2, chi/v5 v5.3.0; Makefile pins sqlc v1.31.1. CLAUDE.md identifies PostgreSQL 17 in CI.
- External reference for review: PostgreSQL 17 transaction isolation, https://www.postgresql.org/docs/17/transaction-iso.html ; explicit row locks, https://www.postgresql.org/docs/17/explicit-locking.html . These links were not fetched during this pass; the proposal is grounded in inspected SQL, the runner's existing RC-delete rationale and repository tests.

## Caveats / Not Found

- Source proof only in this pass: no new HTTP/database exploit, regression run or fix is claimed. The original report also labels R2/R5/R6 as source-proven findings. Implementation must collect the planned failing/passing evidence.
- No surviving session aliases besides /api/plugin-bridge/v1 were found. /api/v1/plugin is deliberately removed; /v1 and daemon hooks keep their separate token contracts.
- Existing full-ref uniqueness is insufficient, but adding a daemon expression unique index is not required to serialize all current production resource writers. Searches found standalone writes and bundled new-project creation only. A future new direct SQL writer must join the same invariant; the retained SQL lock alone does not perform daemon uniqueness validation.
- The default RR runner is not safe for this child-set phantom. This is the main implementation trap; same-user-only tests can mask it because the earlier advisory member fence serializes those callers.
- Existing duplicate daemon rows are not repaired automatically; remediation here prevents new duplicates. Existing explicit stale full-ref client writes retain their current replacement/rename heuristics. Neither data cleanup nor a new CAS/API field is in scope.
- Resource routes must not accidentally acquire projectHumanActor or administrator restrictions while adopting the shared transaction fences. R2's human-only rule applies to the plugin session bridge.
- There were no product simplifications or code changes in this planning pass. The chosen design reuses guards, locks, retry/error conventions and compatibility functions instead of adding a service layer, new schema or dependencies.
