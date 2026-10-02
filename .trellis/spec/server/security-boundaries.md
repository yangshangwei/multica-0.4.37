# Security Boundaries: Credential Minting and Automation Principals

> Contracts from the 2026-09-06 audit closure (`fix/audit-open-findings`).
> Each boundary has a regression test that fails when the boundary is removed.

---

## Scenario 1: Machine credentials cannot mint human credentials

### 1. Scope / Trigger

Any route or handler that mints a new human credential (JWT or personal access
token). A `mat_` task token or `mcn_` cloud PAT carries its runtime owner's
user id, so without a gate a running agent can mint a clean human credential
and re-enter everywhere the machine credential is refused
(credential laundering).

### 2. Signatures

- `POST /api/cli-token` — wrapped `r.With(handler.RequireHumanActor).Post(...)`
  in `server/cmd/server/router.go`
- Route group `/api/tokens` (list, create, renew, revoke) — `r.Use(handler.RequireHumanActor)` inside the group
- Backstop in both minting handlers: `IssueCliToken`
  (`server/internal/handler/auth.go`) and `CreatePersonalAccessToken`
  (`server/internal/handler/personal_access_token.go`) start with
  `if isMachineCredentialActor(r) { writeError(w, http.StatusForbidden, ...); return }`

### 3. Contracts

- `isMachineCredentialActor(r)` (already in `handler/actor_guards.go`)
  classifies `task_token` and `cloud_pat` as machine; human JWT, cookie and
  `mul_` PAT callers stay human.
- The daemon's `mul_` PAT renewal and the CLI browser-JWT → PAT exchange keep
  working (both are human actors under the classifier).

### 4. Validation & Error Matrix

| Caller | `/api/cli-token` | `/api/tokens/*` |
|---|---|---|
| `mat_` task token | 403 | 403 (list, create, renew, revoke) |
| `mcn_` cloud PAT | 403 | 403 |
| human JWT / cookie | 200 | 200/201/204 as before |

### 5. Good/Base/Bad Cases

- Good: router gate + handler backstop both present (fail-closed twice over,
  mirroring `DecideAgentApproval`).
- Base: human caller — behavior identical to before the gate.
- Bad: relying on router wiring alone; a future route re-registration would
  silently reopen the chain.

### 6. Tests Required

- `go test ./cmd/server -run TestMachineCredential` — task token gets 403 on
  all five endpoints, PAT row count unchanged, agent autonomy level unchanged;
  human JWT control still gets 200/201.

### 7. Wrong vs Correct

#### Wrong

```go
r.Post("/api/cli-token", h.IssueCliToken) // machine actors can mint a JWT
```

#### Correct

```go
r.With(handler.RequireHumanActor).Post("/api/cli-token", h.IssueCliToken)
```

---

## Scenario 2: Agent-created triggers act as the authorizing human

### 1. Scope / Trigger

Any write that records an automation's authorization principal. `created_by`
on a trigger is the immutable principal every later dispatch acts as, so an
agent actor's task token must not spend its runtime owner's invoke rights on
standing automation.

### 2. Signatures

- `requireAutomationPrincipal(w, r, workspaceID) (pgtype.UUID, bool)` in
  `server/internal/handler/autopilot.go`, built on `resolveActor` +
  `invokeOriginatorFromRequest`
- Used by `CreateAutopilotTrigger` for both create paths: the schedule path
  and `createWebhookTriggerWithMintedToken` (new `principalID` parameter)

### 3. Contracts

- member actor → principal is the member themselves (today's behavior);
- agent actor → principal is the task's `originator_user_id`, the same human
  `canInvokeAgent` judges every direct assignment by;
- no human in the chain (`originator_user_id` NULL, terminal task) → 403 and
  no trigger row.
- `autopilot.created_by`, `published_by` and the rule-version publisher keep
  the acting identity (the token's user); `requireAutopilotWrite` still
  evaluates the acting identity. Config responsibility and run authorization
  are separate concepts (MUL-6951) — only the second changed.

### 4. Validation & Error Matrix

| Actor | `created_by` recorded | No originator |
|---|---|---|
| member | themselves | n/a |
| agent with originator | the originator | n/a |
| agent without originator | — | 403, no row |

### 5. Good/Base/Bad Cases

- Good: agent-created trigger whose originator cannot invoke the private
  target is still created; every dispatch is refused with
  `invocation_not_allowed`, exactly like a human creator without access.
- Base: member creator records themselves.
- Bad: recording the runtime owner because the token carries their user id.

### 6. Tests Required

- `go test ./internal/handler -run TestCreateAutopilotTrigger_` — created_by
  is the originator (schedule + webhook), private target enqueues nothing
  unless the originator may invoke it, NULL originator gets 403 with no row,
  member control records themselves.

### 7. Wrong vs Correct

#### Wrong

```go
CreatedByID: publisherID, // the credential's user: the runtime owner
```

#### Correct

```go
principalID, ok := h.requireAutomationPrincipal(w, r, workspaceID)
if !ok { return }
// ...
CreatedByID: principalID, // the human the delegation chain names
```

---

## Scenario 3: Lifecycle follow-up reuse authorizes the persisted assignee

`POST /api/issues/{id}/lifecycle-handoffs` can reuse a child through
`follow_up_issue_id` or the issue service's duplicate-title result. Either
path can enqueue the child's existing agent or squad leader, so workspace
membership and validation of a requested replacement assignee are insufficient.

- `authorizeLifecycleFollowUp` in `server/internal/handler/lifecycle_handoff.go`
  applies `validateAssigneePair` to the **persisted** assignee before writing
  follow-up evidence, source metadata, audit comments, or queued tasks.
- The shared invoke gate evaluates the effective human originator for agent
  callers and the leader for squad assignees. Reuse does not change ownership.
- Both reuse and creation preserve the validator's HTTP status: lack of invoke
  permission is 403; invalid assignee configuration is 400.
- Explicit follow-ups must be children of the source. Duplicate lookup remains
  scoped to workspace, project, and parent, so an identically titled child of
  another source is not reused.

Regression coverage:
`go test ./internal/handler -run 'TestCreateLifecycleHandoff(ReuseAuthorization|CreatePreservesAssigneeErrorStatus|DuplicateTitleKeepsParentScope)$'`
with a configured test database. It covers explicit and duplicate reuse of
agent and squad assignees, authorized controls, error statuses, parent scope,
and unchanged issues/comments/tasks on authorization denial.

Lifecycle evidence remains structured in storage, but the issue metadata API
contract allows only string, number, and boolean values. HTTP and WebSocket
renderers share `util.IssueMetadataForResponse`, which sends structured values
under `lifecycle_handoff`, `lifecycle_handoff_history`, `lifecycle_rca_evidence`,
and `lifecycle_rca_unknowns` as JSON strings. This also repairs reads of existing
rows for installed clients without a migration. Internal history and prevention
deduplication must read raw metadata through `util.JSONObjectOrEmpty`.

## Scenario 4: Lifecycle evidence and dispatch are one committed operation

`lifecycle_handoff_transaction.go` owns one transaction for child creation/reuse,
child metadata, source latest/history, audit comment and queue insertion. It uses
`IssueService.CreateInTx` and `TaskService.EnqueuePreparedIssueTaskInTx`; ordinary
`IssueService.Create` retains its existing wrapper behavior. Never call the
ordinary Create/enqueue wrappers inside the lifecycle transaction: they commit or
publish independently. Prepare external MCP overlays before locks; recheck the
agent/runtime, current authorization and trusted task attribution inside them.

- Workspace KEY SHARE fence comes first. For title creation, the existing
  duplicate advisory lock precedes owner/issue/counter locks. Existing source and
  child rows use NO KEY UPDATE NOWAIT, sorted by UUID; owner locks also use
  NOWAIT. This avoids waiting on a SourceContext create that already holds the
  source before taking duplicate/counter locks. Retry only a rolled-back lock or
  stale-snapshot attempt, never a commit with an uncertain outcome.
- Whole metadata retains the 8 KiB `pg_column_size(jsonb)` constraint. Preserve
  unrelated keys as RawMessage (large JSON integers must not round through
  float64), append then cap history at 20, trim only oldest history to fit. If
  latest plus the current history entry cannot fit, return 400 and roll back all
  issue, counter, audit and task changes. Budget both source and child metadata.
- Agent callers propagate the live, workspace-scoped task belonging to the
  resolved actor. Originator and accountable remain distinct; neither an old
  child's creator nor a runtime owner grants the current invocation rights.
- Compatible pending issue/agent tasks reuse their real ID and return 201, with
  unchanged note/attribution and no duplicate task notification. Compare squad
  leader role, runtime, note, attribution and existing head-SHA semantics as
  well as the database's issue/agent unique key. Incompatible work returns 409
  with no writes. A unique violation must roll back its savepoint before reading
  the winner; the outer transaction cannot continue in aborted state.
- Commit precedes all events and wakeups. Notification failure does not convert
  a committed operation into a failed HTTP request; daemon claim-candidate polls
  can still discover the durable row. A failed/uncertain commit logs source,
  child and task IDs for reconciliation and is never automatically replayed.

Regression: `lifecycle_handoff_atomic_test.go` covers provenance, notes,
whole-state rollback at each required write/commit, size boundaries, compatible
and incompatible queue reuse, concurrent history/metadata, SourceContext lock
order, external-overlay preparation, and poll recovery without notification.
`lifecycle_handoff_authorization_test.go` retains the persisted-assignee matrix.


## Password-mode sessions and derived credentials

`MULTICA_AUTH_MODE=password` is a local/self-hosted mode; Fleet/device/email
sign-in is not a fallback. A human principal is insufficient for account
binding: setup/change requires a validated user JWT/cookie session. Old JWTs
are confined to an explicit, fixed migration window. Temporary recovery
passwords only authorize profile/change/logout.

Every personal credential carries the source account version, including PAT,
task/daemon tokens and member-actor plugin callbacks. Minting uses the same
user-row lock as password changes and rechecks the source version inside the
transaction; never upgrade a delayed request by reading the newest version.
Password-mode checks bypass legacy PAT/daemon authorization caches. DB faults
return503; an actual invalid/revoked session returns401.

Realtime and daemon WebSockets validate both directions: before incoming RPC,
heartbeat or subscription side effects, and before outgoing protected events.
Preserve the authenticated context in daemon connections so downstream claim
transactions receive the original version. Stateless CDN wildcard cookies are
not issued in password mode. Password JSON endpoints require application/json
so cross-site simple forms cannot inject an account session.

Revocation preserves memberships, agents and independent plugin installations.
Do not call the full member-removal operation: only reuse its scoped query
primitives. A daemon's task cancellation watcher stops on credential rejection
(401), while network/503 failures remain retryable. Once credentials exist,
down migrations must refuse to drop their table or uniqueness constraints.

Regression sources: auth/password*_test.go, handler/auth_password_test.go,
handler/password_ws_test.go, handler/plugin_password_test.go,
migrations/password_rollback_test.go and daemon/daemon_test.go.


## Platform administration in password mode

`MULTICA_PLATFORM_ADMIN_ENABLED=true` exposes the Web-only `/admin` surface and
`/api/admin/*` routes outside workspace middleware. It does not grant a role.
The pre-listener `platform-admin bootstrap --user <UUID> --reason <reason>`
command initializes an existing completed password account, atomically revokes
its old credentials, and records a deployment-operator audit.

- Platform authorization permits only complete password JWT/cookie sessions.
  `GetPlatformAdminAccount` reads password version, account state, and role in
  one snapshot; separate reads can combine an old version with a newly granted
  role. PATs cannot mint JWTs through `/api/cli-token` in password mode, while
  the shared password-session lock remains PAT-capable for PAT renewal.
- Role and credential recovery transactions take the shared platform advisory
  lock, then sorted user-row locks. Recheck actor authority and the password
  verification snapshot under those locks. Compute KDFs before opening locks.
- Effective-super-admin counting must use the service helper, including the
  temporary emergency denylist that password login still enforces. A blocked
  UUID/email must not be the last nominal administrator. Ordinary recovery
  cannot remove the last usable administrator; deployment break-glass recovery
  is explicit, audited, and still requires the target to change the password.
- Role grants/promotions increment the target account version and revoke old
  credentials. Publish disconnects only after commit and pass the target UUID
  explicitly; the actor remains the audit author.
- Operation creation and audit share the mutation transaction. Actor/org/key
  lookup supports uncertain-response recovery, and only non-secret fields
  enter the payload digest. The HTTP DTO omits digests and password state.
- New workspaces acquire their internal organization within the creation
  transaction. Inactive organizations and absent/failed assignments abort the
  transaction; an INSERT SELECT with zero rows is not a successful assignment.
  Workspace deletion removes its mapping but preserves platform roles/audits.
- Management responses include `no-store` before authentication, including
  credential and CSRF rejections. A missing/inactive organization is dependency
  failure (503), not an unsupported endpoint (404).

Regression sources: service/platform_admin_test.go,
service/platform_password_recovery_test.go, handler/admin_auth_test.go,
handler/admin_organization_test.go, cmd/server/platform_admin_auth_test.go,
and migrations/platform_admin_migration_test.go.

### Rollback guards must fence concurrent writers

An `EXISTS` check alone does not protect retained security data from rollback.
Under READ COMMITTED it can miss an uncommitted first insert; later DDL waits
for that writer and can then delete its successful commit. Acquire the checked
and DDL-target tables in deterministic name order with `ACCESS EXCLUSIVE
NOWAIT` before checking retained records. Keep the check and protected DDL in
one `DO` block so the locks cannot end between them. A busy table is a refusal,
not permission to wait out a writer or remove its data.

The migration runner's advisory lock serializes runners only. A multi-file
rollback requires maintenance mode with all API/background writers stopped
throughout the loop; individual file locks do not span the full rollback.
Disabling the admin UI is insufficient because cancellation reconciliation
intentionally continues. Preserve the existing data-present refusal and verify
both populated and empty-maintenance cases in private fixture schemas.

Regression: `migrations/admin_rollback_guard_test.go` exercises the actual down
SQL with two connections and checks all 43 guarded platform down files in the
465–511 range. Never run this rehearsal against the application schema.
