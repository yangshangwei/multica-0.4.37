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
