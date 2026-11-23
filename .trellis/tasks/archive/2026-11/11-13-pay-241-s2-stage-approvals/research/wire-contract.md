# PAY-241 S2 stage wire contract

- Status: **frozen v1** (first freeze 2026-11-14, server lane). Section 13 lists later changes.
- Owner: server implement lane. The client lane codes against this file only.
- Contract id on the wire: `collaboration-stage-v1`, `protocol_version: 1`.
- Implementation state per action is the last column of the table in section 1.
  `inc1` = implemented by server increment 1. `inc2` = frozen contract, route not
  registered yet (any request answers the router's ordinary 404/405 with no JSON
  `code`); the client must treat a missing route as *unavailable*, never as passed.
  `inc2a` / `inc2b` identify registered routes with the step subset stated below;
  registration is not acceptance or permission to enable the project policy.
- This file describes transport only. It does not state that S2 is accepted, that a
  stage policy can be switched on, or that any external integration is verified.

## 1. Action index

| # | Action (`stage_confirmation.action`) | Method + path | Operation scope | State |
| --- | --- | --- | --- | --- |
| P1 | read project policy | `GET /api/projects/{projectId}/stage-policy` | — | inc1 |
| P2 | `revise_policy` | `PUT /api/projects/{projectId}/stage-policy` | project policy revision | inc1 (enabling refused, see 4.2) |
| P3 | recover P2 | `GET /api/projects/{projectId}/stage-policy/operations/{requestId}` | project policy revision | inc1 |
| R1 | read stage view | `GET /api/collaboration/issues/{id}/stage` | — | inc1 |
| R2 | list proposals | `GET /api/collaboration/issues/{id}/stage/proposals` | — | inc1 |
| R3 | read proposal | `GET /api/collaboration/issues/{id}/stage/proposals/{proposalId}` | — | inc1 |
| A1 | `configure_roles` | `POST /api/collaboration/issues/{id}/stage/roles` | issue workflow operation | inc1 |
| A2 | `create_proposal` | `POST /api/collaboration/issues/{id}/stage/proposals` | issue workflow operation | inc1 |
| A3 | `revise_proposal` | `POST /api/collaboration/issues/{id}/stage/proposals/{proposalId}/versions` | issue workflow operation | inc1 |
| A4 | `decide_proposal` (accept / reject; modify-accept = A3 then A4) | `POST /api/collaboration/issues/{id}/stage/proposals/{proposalId}/decisions` | issue workflow operation | inc1 |
| A5 | `confirm_scope` | `POST /api/collaboration/issues/{id}/stage/scope-confirmations` | issue workflow operation | inc1 |
| A6 | `create_code_candidate` (create and revise) | `POST /api/collaboration/issues/{id}/stage/code-candidates` | issue workflow operation | inc2a |
| A7 | `decide_stage` (code review, QA pass/return, release approval, rollback approval) | `POST /api/collaboration/issues/{id}/stage/decisions` | issue workflow operation | inc2c code_review, qa, release and rollback |
| A8 | `revoke_decision` | `POST /api/collaboration/issues/{id}/stage/decisions/{decisionId}/revocations` | issue workflow operation | inc2a |
| A9 | `advance_stage` | `POST /api/collaboration/issues/{id}/stage/advance` | issue workflow operation | inc2c development/code_review/qa/release/verification and rework |
| R4 | read evidence for a code candidate | `GET /api/collaboration/issues/{id}/stage/code-candidates/{candidateId}` | — | inc2c; test reports and authenticated execution history served |
| T1 | runtime test subject | `GET /api/collaboration/runtime/runs/{runId}/test-subject` | — | inc2b |
| T2 | runtime test report submit | `POST /api/collaboration/runtime/runs/{runId}/test-reports` | run event | inc2b |
| T3 | runtime test report receipt | `GET /api/collaboration/runtime/runs/{runId}/test-reports/by-request/{requestId}` | run event | inc2b |
| T4 | human test report read | `GET /api/collaboration/issues/{id}/runs/{runId}/test-reports` and `.../test-reports/{reportId}` | — | inc2b served; 8.8 |
| C1 | create typed test input | existing `POST /api/collaboration/issues/{id}/comments` with create-only `test_subject` | existing comment operation | inc2b served; 8.9 |
| O1 | recover any A-action | `GET /api/collaboration/issues/{id}/operations/{requestId}` (existing route) | issue workflow operation | inc1 for A1–A5; inc2b for served A6–A9 subsets |

Existing ordinary endpoints additionally answer the stage admission codes of
section 10; their request/response shapes are otherwise unchanged.

## 2. Transport rules shared by every route in this file

1. Authentication: the existing Auth-only account group for
   `/api/collaboration/issues/...` (ambient `X-Workspace-ID` / `X-Workspace-Slug`
   are discarded); the existing workspace-scoped `/api/projects/{id}` group with
   `RequireHumanActor` for P1–P3; the existing narrow `mcb_` bearer for T1–T3.
2. Every response carries `Cache-Control: no-store`.
3. Request bodies: `Content-Type: application/json` (else 415 `json_required`),
   one JSON object, **no** duplicate keys, **no** unknown fields, **no** trailing
   JSON, no case-alias keys (else 400 `invalid_request`). `null` is not accepted
   for a required field. Strings are valid UTF-8 without NUL.
4. Identifiers are canonical lowercase UUID strings, never the zero UUID.
5. **Stage versions are canonical decimal strings**: ASCII digits, no sign, no
   leading zero (except the single value `"0"` where the field allows "none"),
   at most 19 digits, value within signed 64-bit. This applies to
   `policy_revision`, `role_revision`, `stage_revision`, `scope_revision`,
   candidate `version`, `lifecycle_epoch`, `semantic_version` and every
   `expected_*` counterpart. Ordinary Issue `revision` keeps its existing numeric
   wire type and is not used by this contract.
6. Digests are 64 lowercase hex characters (SHA-256).
7. Timestamps are RFC3339 UTC. `expires_at` request values use UTC with at most
   microsecond precision and a literal `Z`.
8. Body size limits (decoded JSON bytes on the wire): A1, A4, A5, A8, A9, P2:
   64 KiB; A2, A3: 2 MiB; A6, A7: 256 KiB; T2: 2 MiB. Over limit: 413
   `request_too_large`. Field limits apply after decoding and are listed per action.
9. Error body for a refusal that was **not** stored: `{"code": "<code>", "error": "<code>"}`
   (ordinary-endpoint stage refusals of section 10 also carry `"message"`).
10. `Idempotency-Key` (1–128 characters) is required on every write (P2, A1–A9);
    missing or oversized: 400 `invalid_request_id`. T2 carries its own `request_id`.

## 3. Issue workflow operations (A1–A9): shared envelope

### 3.1 Request

Every A-action body contains the existing four workflow observations, all positive
decimal strings taken from the collaboration detail's `write_context`:

```json
{
  "expected_collaboration_revision": "12",
  "expected_lifecycle_epoch": "1",
  "expected_participant_epoch": "1",
  "expected_account_epoch": "1"
}
```

Malformed observation: 400 `invalid_version` (not stored). Action fields are added
to the same object. `expected_policy_revision` is `>= "1"`; `expected_role_revision`
and `expected_scope_revision` are `>= "0"` unless stated otherwise.

The operation identity is `(issue, authenticated human, Idempotency-Key)` with a
digest over the action kind and the complete decoded request. The client must
freeze the key and the exact body before sending.

### 3.2 Success

HTTP 200:

```json
{
  "code": "<success code>",
  "collaboration_revision": "13",
  "stage_confirmation": { "action": "...", "issue_id": "<uuid>", "...": "variant fields" }
}
```

`stage_confirmation` is a closed union discriminated by `action`; each action lists
its variant. It never contains prose, child lists, logs, file metadata or
credentials. The client retires its pending request only when `action`, `issue_id`
and every variant field that it already knew before sending equal the frozen
request, and every server-created identifier is a non-zero UUID. Fields of other
variants are absent, never null placeholders.

### 3.3 Stored refusal

A refusal decided inside the workflow transaction is committed with the operation.
The HTTP status is the stored status and the body is

```json
{"code": "<code>", "error": "<code>", "status": 409, "collaboration_revision": "12"}
```

with no `stage_confirmation`. It is final for that key: replaying the same key and
body returns the same refusal even if the state later changes. A new attempt needs
an explicit new key built from freshly read state.

### 3.4 Refusals that are not stored

No operation row exists; the same key may be sent again with the same body.

| HTTP | code | meaning |
| --- | --- | --- |
| 400 | `invalid_request`, `invalid_version`, `invalid_request_id`, `invalid_stage_request` | malformed transport or action field |
| 403 | `forbidden` | non-human / agent-actor / restricted credential |
| 404 | `not_found` | no current read authority for the issue |
| 409 | `idempotency_conflict` | same key, different kind or body |
| 413 / 415 | `request_too_large` / `json_required` | |
| 503 | `collaboration_busy`, `collaboration_unavailable` | lock contention after bounded retries / dependency failure; outcome of *this* attempt is "not written" only for `collaboration_busy`; a transport failure or 503 `collaboration_unavailable` after sending is **unknown** |

### 3.5 Recovery (O1) and uncertain results

`GET /api/collaboration/issues/{id}/operations/{requestId}`, original human only:

| Response | Meaning for the stage client |
| --- | --- |
| 200 `{code, collaboration_revision, stage_confirmation}` | original success; verify the variant as in 3.2 |
| 200 `{code, collaboration_revision, status}` (status >= 400) | original stored refusal; show that status/code, keep the form input |
| 409 `operation_incomplete` | not terminal; keep the original intent pending and ask again later |
| 404 `not_found` | **unknown** original result (never written, or not visible); keep the intent; never POST automatically |
| malformed / contradictory body | unknown; keep the intent |

GET never creates, replays or retries a write. Only an explicit user action, based
on a fresh authoritative read, may create a request with a new key. A user may also
explicitly resend the *same* key and body; the server then replays or evaluates it once.
Recovery needs only the current authenticated account; it grants no issue
authority and returns no content. The full child mapping of an accepted proposal
is read through R3 under current authority, never from a receipt.

### 3.6 Refusal codes shared by A1–A9 (stored unless noted)

| HTTP | code | condition |
| --- | --- | --- |
| 403 | `forbidden` | human without current participation for this action |
| 409 | `stale_authority` | lifecycle / participant / account epoch differs |
| 409 | `stale_revision` | collaboration revision differs |
| 409 | `stage_policy_disabled` | root has no project, or the project's current stage policy is absent/disabled |
| 409 | `stage_policy_changed` | `expected_policy_revision` differs from the current revision |
| 409 | `stage_roles_unconfigured` | action needs a role configuration and none exists |
| 409 | `stage_roles_changed` | `expected_role_revision` differs |
| 403 | `stage_role_required` | actor is not the configured holder of the required role |
| 409 | `issue_closed` | root is done/cancelled or its queue is closed |
| 503 | `stage_schema_unavailable` (not stored) | stage tables or indexes are not ready |

## 4. Project stage policy (P1–P3)

Scope: source workspace project. Authority follows the existing project delivery
policy write transaction: verified human credential, current workspace member,
`owner`/`admin` for writes, project row locked. The policy is independent of the
M1 delivery policy, the task AI policy and any Git automation switch.

### 4.1 P1 read

`GET /api/projects/{projectId}/stage-policy` → 200

```json
{
  "contract": "collaboration-stage-v1",
  "project_id": "<uuid>",
  "policy_revision": "0",
  "enabled": false,
  "reason": "",
  "created_by": null,
  "created_at": null,
  "activation_available": false
}
```

No revision yet is virtual revision `"0"`, disabled. `activation_available` is the
server's statement whether `enabled: true` can currently be written (see 4.2).
Errors: 403 `stage_policy_denied` (not a current human member), 404
`stage_policy_project_not_found`, 409 `stage_policy_busy`, 503
`stage_schema_unavailable`.

### 4.2 P2 revise

`PUT /api/projects/{projectId}/stage-policy`, `Idempotency-Key` required.

```json
{"expected_policy_revision": "0", "enabled": true, "reason": "why"}
```

All three fields required. `reason`: 1–1000 Unicode characters after trimming.
`expected_policy_revision` `>= "0"`.

Success 200:

```json
{
  "code": "stage_policy_revised",
  "stage_confirmation": {
    "action": "revise_policy",
    "project_id": "<uuid>",
    "policy_revision": "1",
    "enabled": true
  }
}
```

The confirmation has no `issue_id`, candidate or decision. Same-value writes are
allowed and create a new revision. Enabling creates only the revision: it assigns
no role, approves no child and starts or stops no work.

Refusals (never stored; the revision table holds successes only):

| HTTP | code | condition |
| --- | --- | --- |
| 400 | `invalid_stage_policy_request` | malformed body |
| 400 | `invalid_request_id` | bad `Idempotency-Key` |
| 403 | `stage_policy_denied` | non-human credential or not a current member |
| 403 | `stage_policy_management_denied` | member without owner/admin |
| 404 | `stage_policy_project_not_found` | project not in this workspace |
| 409 | `stage_policy_revision_conflict` | expected revision differs |
| 409 | `idempotency_conflict` | same actor+project+key, different body |
| 409 | `stage_policy_busy` | lock contention; nothing written |
| 503 | `stage_policy_unavailable` | **temporary gating**: `enabled: true` is refused over HTTP until every completion/submit/project-detach/delete gate of server increment 2 exists. `activation_available` is `false` while this holds. `enabled: false` is not affected |
| 503 | `stage_schema_unavailable` | stage schema not ready |

Replay: the same actor, project, key and body returns the original success body
(the original revision, even if newer revisions exist). It is evaluated after the
current credential/member check and before role and revision checks.

### 4.3 P3 recover

`GET /api/projects/{projectId}/stage-policy/operations/{requestId}` → 200 with the
same body as the original P2 success, for the original actor only. 404
`stage_policy_operation_not_found` means the original result is unknown or was a
refusal; it is never inferred from "the current policy happens to be equal". The
client must not PUT automatically; an explicit resend of the same key and body is safe.
There is no in-progress state: a revision is either committed or absent.

## 5. Read projections (R1–R3)

Authority: current human read authority for the collaboration issue (existing
`ResolveIssueAccess`). Loss of authority answers 404 `not_found`. Stage data is
part of the protected collaboration content. A schema that is not ready answers
503 `stage_schema_unavailable`; the client shows *unavailable*.

`Account` is the existing collaboration content account summary
`{"user_id","name","avatar_url","external"}` (the same projection as comment
authors; `external` is a boolean: not a current source-workspace member). A role
whose account no longer resolves is `null`.

### 5.1 R1 stage view

`GET /api/collaboration/issues/{id}/stage` → 200

```json
{
  "protocol_version": 1,
  "issue_id": "<uuid>",
  "collaboration_revision": "12",
  "lifecycle_epoch": "1",
  "policy": {"contract": "collaboration-stage-v1", "project_id": "<uuid>", "state": "enabled", "policy_revision": "1"},
  "managed": true,
  "needs_confirmation": true,
  "missing": ["roles", "scope"],
  "stage": {"current": "development", "stage_revision": "1"},
  "role_revision": "0",
  "roles": null,
  "scope_revision": "0",
  "scope_state": "none",
  "scope": null,
  "children": {"items": [], "truncated": false},
  "code_candidates": {"items": [], "truncated": false},
  "capabilities": {"configure_roles": false, "create_proposal": true, "decide_proposal": false, "confirm_scope": false}
}
```

- `policy.project_id` is `null` when the root has no project. `policy.state` is
  `enabled`, `disabled` (latest revision disabled) or `none` (no revision or no
  project). `managed` is `true` only for `enabled`.
- When `managed` is `false`: `needs_confirmation` is `false`, `missing` is `[]`,
  `stage` is `null`, every capability is `false`; stored roles/scope/children are
  still projected as history.
- `stage.current` ∈ `development | code_review | qa | release | verification`.
- `missing` is a subset of `["roles", "scope"]`, in that order.
  `needs_confirmation` equals `missing` being non-empty.
- `scope_state` ∈ `none | current | lifecycle_changed | policy_changed | signer_changed`.
  Only `current` satisfies the scope requirement. `signer_changed` means the
  confirming product approver is no longer the configured approver, or their
  participant/account generation changed.
- `roles` when configured:

```json
{
  "role_revision": "2",
  "configured_by": "<uuid>",
  "configured_at": "2026-11-14T00:00:00Z",
  "business_owner": Account, "implementer": Account, "product_approver": Account,
  "code_reviewer": Account, "qa_approver": Account, "release_approver": Account
}
```

- `scope` when a confirmation exists (also when it is no longer current):

```json
{"scope_revision": "1", "decision_id": "<uuid>", "lifecycle_epoch": "1", "policy_revision": "1",
 "role_revision": "1", "confirmed_by": "<uuid>", "confirmed_at": "...", "child_issue_ids": ["<uuid>"]}
```

- `children.items` are the ordinary issues **governed** by this root: every
  ordinary descendant whose nearest collaboration ancestor is this root (the
  walk goes at most 64 levels down and stops at a nested collaboration issue),
  plus issues still mapped to this root after leaving that subtree. At most 200
  (`truncated: true` beyond), ordered by issue number:

```json
{
  "issue_id": "<uuid>", "identifier": "ABC-12", "title": "...", "can_open": true,
  "creator_type": "member",
  "origin": "materialized",
  "attached": true,
  "depth": 1,
  "content_digest": "<hex64>",
  "semantic_version": "1",
  "approved_lifecycle_epoch": "1",
  "approval_current": true,
  "execution_gated": false,
  "proposal_id": "<uuid>", "ordinal": 1,
  "code": {"pull_request": {"provider": "github", "pull_request_id": "<uuid>", "head_sha": "<40 or 64 hex>"}},
  "code_state": "current",
  "pull_requests": [ {"provider": "github", "pull_request_id": "<uuid>", "head_sha": "<40 or 64 hex>"} ]
}
```

  `depth` is 1 for a direct child, 2 and more below it, and `null` for a mapped
  issue that left the subtree (`attached` is then `false`).
  `pull_requests` are the pull/merge requests the server currently finds linked
  to that issue (reference-only mentions excluded) with their server-observed
  heads; identifiers only, no title, URL, repository or author. `provider` is
  `github` or `vcs`. This is database-observed state, not a live remote fetch.
  `code` is the code version the latest scope confirmation carried forward for
  that issue, or `null`. `code_state` ∈ `none | current | changed | unavailable`:
  `none` = nothing confirmed; `current` = that pull request is still linked with
  the confirmed head; `changed` = it is linked with another head; `unavailable` =
  it is no longer linked. `code_state` is evidence state only: it never changes
  `scope_state`, `approval_current` or `execution_gated`. Increment 2 consumers of
  a confirmed code version require `current`.

  `origin` ∈ `materialized | confirmed | none`. `semantic_version`,
  `approved_lifecycle_epoch`, `proposal_id`, `ordinal` are `null` when not
  applicable. `content_digest` is the digest of current title+description only
  when `can_open:true`; `can_open:false` requires explicit `null`, never a dummy
  digest. A5 uses a currently source-readable child's digest. `execution_gated` is `true`
  when machine execution of that child is currently refused by section 10.
  `can_open` follows the existing ordinary-children rule (source members only).
- `capabilities` are display hints only; every write is rechecked by the server.
  Clients use `=== true`.
- From increment 2a, `code_candidates` discovers code candidates for reloads and
  other reviewers. It is `{items: [CandidateRef & {state}], truncated: boolean}`;
  `state` is `current | superseded | stale`. Entries contain exactly
  `candidate_id`, `version`, `content_digest`, `state`, newest first, at most 50.
  No source identity or content is included. An older response may omit the
  field; that means discovery is unavailable, not that no candidates exist.

### 5.2 R2 proposal list

`GET /api/collaboration/issues/{id}/stage/proposals?limit=1..50&cursor=<opaque>`
(default limit 20; duplicate/unknown query keys: 400 `invalid_stage_request`;
a cursor bound to another issue/viewer: 400 `invalid_stage_request`).

```json
{
  "issue_id": "<uuid>",
  "items": [{
    "proposal_id": "<uuid>",
    "state": "pending",
    "version_count": "2",
    "created_at": "...",
    "current": {
      "candidate_id": "<uuid>", "version": "2", "content_digest": "<hex64>",
      "parent_candidate_id": "<uuid>", "lifecycle_epoch": "1", "stale": false,
      "submitted_by": "<uuid>", "created_at": "...",
      "summary": "...", "child_count": 3
    },
    "decision": null
  }],
  "next_cursor": null
}
```

`state` ∈ `pending | rejected | accepted`, derived from the newest version.
`stale` is `true` when the candidate belongs to an earlier root lifecycle.
`decision` is the `Decision` of 5.3 for the newest version, or `null`.
Order: newest proposal first. Pages never repeat a proposal; a non-advancing
cursor must be refused by the client.

### 5.3 R3 proposal detail

`GET /api/collaboration/issues/{id}/stage/proposals/{proposalId}` → 200, or 404
`stage_proposal_not_found`.

```json
{
  "issue_id": "<uuid>", "proposal_id": "<uuid>", "state": "accepted",
  "versions": [Candidate], "versions_truncated": false,
  "decisions": [Decision],
  "children": [{"ordinal": 1, "issue_id": "<uuid>", "identifier": "ABC-12", "title": "...", "can_open": true, "attached": true}]
}
```

`versions` holds the newest 50 versions, newest first. `children` is non-empty only
for an accepted proposal and lists the materialized ordinary issues by ordinal.

```json
Candidate = {
  "candidate_id": "<uuid>", "version": "2", "content_digest": "<hex64>",
  "parent_candidate_id": "<uuid>" | null, "lifecycle_epoch": "1",
  "state": "pending" | "superseded" | "rejected" | "accepted",
  "submitted_by": "<uuid>", "created_at": "...",
  "content": ProposalContent,
  "source": ProposalSource
}
Decision = {
  "decision_id": "<uuid>", "step": "proposal", "decision": "accepted" | "rejected",
  "candidate_id": "<uuid>", "version": "2", "content_digest": "<hex64>",
  "role": "product_approver", "role_revision": "1", "policy_revision": "1", "lifecycle_epoch": "1",
  "actor_user_id": "<uuid>", "reason": "...", "created_at": "...",
  "accepted_with_modification": false
}
```

`accepted_with_modification` is `true` when the accepted version has a parent
version and was submitted by the accepting approver (the modify-accept sequence).

## 6. Shared stage objects

```json
ProposalContent = {
  "summary": "<0..16384 bytes>",
  "children": [ {"title": "<1..500 chars>", "description": "<0..65536 bytes>", "priority": "none"} ]
}
```

- `children`: 1 to 20 entries; array order is the fixed ordinal (1-based).
- `title` is trimmed, non-blank, at most 500 Unicode characters; `description`
  at most 65,536 bytes; `priority` required, one of `urgent | high | medium | low | none`.
- The whole canonical content is at most 512 KiB.
- v1 children carry no assignee, project, label or date: materialized issues are
  created unassigned in status `todo`, inherit the root's project, and are
  assigned afterwards through the ordinary assignment entry points.
- `content_digest` is the SHA-256 of the server's canonical JSON of the content
  (keys in the order above, no insignificant whitespace). The client never
  computes it; it echoes the server value.

```json
ProposalSource =
    {"kind": "manual"}
  | {"kind": "comment", "comment_id": "<uuid>", "comment_revision": "3"}
  | {"kind": "run", "run_id": "<uuid>"}
```

A request sends exactly one of these shapes. The server verifies that the comment
or run belongs to this issue and is currently readable, and stores its own facts;
read projections add server fields: `"author_type"` and `"author_id"` (comment),
`"agent_id"` (run) and `"digest"` (comment content digest). A client can never
state an agent author.

```json
CandidateRef = {"candidate_id": "<uuid>", "version": "2", "content_digest": "<hex64>"}
```

## 7. Increment-1 actions

### 7.1 A1 configure roles — kind `stage_configure_roles`

Body: observations plus

```json
{
  "expected_policy_revision": "1",
  "expected_role_revision": "0",
  "roles": {
    "business_owner_user_id": "<uuid>", "implementer_user_id": "<uuid>",
    "product_approver_user_id": "<uuid>", "code_reviewer_user_id": "<uuid>",
    "qa_approver_user_id": "<uuid>", "release_approver_user_id": "<uuid>"
  }
}
```

All six roles required. One person may hold several roles, except the two
separations below.

Authority: human; current participant of the issue; **current source-workspace
owner or admin**. Occupational titles grant nothing.

Success: `stage_roles_configured`, or `stage_roles_unchanged` when the
configuration equals the current one (no new revision, no event).

```json
"stage_confirmation": {"action": "configure_roles", "issue_id": "<uuid>", "role_revision": "1"}
```

Stored refusals in addition to 3.6:

| HTTP | code | condition |
| --- | --- | --- |
| 403 | `stage_configuration_denied` | actor is not a current source owner/admin |
| 409 | `stage_self_review_forbidden` | implementer equals code reviewer |
| 409 | `stage_qa_release_conflict` | QA approver equals release approver |
| 409 | `stage_role_target_unavailable` | a named account is not a current, eligible participant |

Effects: one immutable role revision, one `stage_roles_configured` event. No
approval, child or execution changes. Changing roles makes approvals that depend
on the previous configuration unusable (see `scope_state` and increment 2).
Signer-expiry boundary: none (a configuration is not a signature).

### 7.2 A2 create proposal — kind `stage_create_proposal`

Body: observations plus

```json
{"expected_policy_revision": "1", "content": ProposalContent, "source": ProposalSource}
```

Authority: human current participant. An agent or `mcb_`/task credential can never
call this route; machine output becomes a proposal only when a human files it.

Success: `stage_proposal_created`

```json
"stage_confirmation": {"action": "create_proposal", "issue_id": "<uuid>", "proposal_id": "<uuid>",
  "candidate_id": "<uuid>", "version": "1", "content_digest": "<hex64>"}
```

The client matches `action`, `issue_id` and `version == "1"`, and accepts the new
`proposal_id` / `candidate_id` / `content_digest` (no pre-known UUID exists).

Stored refusals in addition to 3.6: 409 `stage_source_unavailable` (comment/run
missing, deleted, of another issue, or `comment_revision` not current). Malformed
or over-limit content: 400 `invalid_stage_request` (not stored).

Effects: one immutable candidate (version 1, pending). No issue is created and
nothing becomes executable. Roles need not be configured to file a proposal.

### 7.3 A3 revise proposal — kind `stage_revise_proposal`

Body: observations plus

```json
{"expected_policy_revision": "1", "expected_candidate": CandidateRef,
 "content": ProposalContent, "source": ProposalSource}
```

`expected_candidate` must be the proposal's newest version. Revision is allowed
while that version is pending, rejected, or stale (earlier lifecycle).

Authority: human current participant (this includes the product approver, which is
how modify-accept saves its visible new version).

Success: `stage_proposal_revised`

```json
"stage_confirmation": {"action": "revise_proposal", "issue_id": "<uuid>", "proposal_id": "<uuid>",
  "candidate_id": "<uuid>", "version": "2", "content_digest": "<hex64>",
  "parent_candidate_id": "<uuid>"}
```

The client matches `action`, `issue_id`, `proposal_id`, `parent_candidate_id ==
expected_candidate.candidate_id` and `version == expected version + 1`.

Stored refusals in addition to 3.6:

| HTTP | code | condition |
| --- | --- | --- |
| 404 | `stage_proposal_not_found` | no such proposal under this issue |
| 409 | `stage_candidate_changed` | `expected_candidate` is not the newest version / digest differs |
| 409 | `stage_proposal_already_materialized` | proposal was accepted; file a new proposal |
| 409 | `stage_proposal_unchanged` | content digest equals the newest version of the current lifecycle |
| 409 | `stage_source_unavailable` | as A2 |

Effects: a new immutable version; the previous version stays readable as
`superseded` (or `rejected`).

### 7.4 A4 decide proposal — kind `stage_decide_proposal`

Body: observations plus

```json
{"expected_policy_revision": "1", "expected_role_revision": "1",
 "candidate": CandidateRef, "decision": "accept", "reason": "..."}
```

`decision` ∈ `accept | reject`. `reason`: required 1–2000 characters for `reject`,
optional (may be `""`, at most 2000) for `accept`; the field itself is required.

**Modify-accept** is two explicit requests by the product approver: A3 saves the
modified content as a visible new version, then A4 accepts exactly that
`CandidateRef`. The server never rebinds an accept to a newer version.

Authority: human; current participant; the **configured product approver** of the
current role revision. `accept` additionally requires the approver's **current
source-workspace membership** (ordinary issue creation right) and the workspace
issue entitlement.

Success: `stage_proposal_accepted` or `stage_proposal_rejected`

```json
"stage_confirmation": {"action": "decide_proposal", "issue_id": "<uuid>", "proposal_id": "<uuid>",
  "candidate_id": "<uuid>", "version": "2", "content_digest": "<hex64>",
  "decision": "accepted", "decision_id": "<uuid>", "role_revision": "1", "policy_revision": "1"}
```

`decision` in the confirmation is `accepted | rejected`. A stored rejection is a
successful saved human decision (HTTP 200), not a refusal.

Stored refusals in addition to 3.6:

| HTTP | code | condition |
| --- | --- | --- |
| 404 | `stage_proposal_not_found` | |
| 409 | `stage_candidate_changed` | `candidate` is not the newest version / digest differs |
| 409 | `stage_candidate_not_pending` | newest version already rejected |
| 409 | `stage_candidate_stale` | candidate belongs to an earlier root lifecycle; revise first |
| 409 | `stage_proposal_already_materialized` | proposal already accepted |
| 409 | `stage_needs_confirmation` | accept only: roles or existing-child scope not confirmed for the current lifecycle/policy |
| 403 | `stage_source_creation_denied` | accept only: approver is not a current source member |
| 409 | `stage_child_creation_refused` | accept only: an ordinary issue rule refused a child (duplicate title, issue limit, project gate, …); **nothing** of the decision, mapping or numbering is kept |

Effects of accept, all in one transaction: the decision, 1–20 ordinary child issues
created through the ordinary issue creation path (creator = approver, parent =
root, unassigned, `todo`), and the unique proposal→child mapping. No task is
enqueued and no execution event is published; assigning or running a child stays
subject to the ordinary invoke/triage/manual/Stop gates and to section 10. A
proposal can be accepted once; a second accept with another key is refused
`stage_proposal_already_materialized` and creates nothing.

Signer-expiry boundary: an accept is consumed in its own transaction and has no
standing validity. The decision row stores the signer's participant and account
generation and credential kind for audit.

### 7.5 A5 confirm existing-child scope — kind `stage_confirm_scope`

Used after first enablement and after every reopen. Body: observations plus

```json
{"expected_policy_revision": "1", "expected_role_revision": "1", "expected_scope_revision": "0",
 "children": [
   {"issue_id": "<uuid>", "content_digest": "<hex64>",
    "code": {"pull_request": {"provider": "github", "pull_request_id": "<uuid>", "head_sha": "<40 or 64 lowercase hex>"}}},
   {"issue_id": "<uuid>", "content_digest": "<hex64>", "code": null}
 ],
 "reason": ""}
```

`children`: 0–200 entries, unique `issue_id`; the explicit set of existing ordinary
issues governed by this root (any depth, from R1) that the product approver
carries into the current lifecycle, each with its current content digest **and
the code version carried forward**. An empty array is a valid explicit
confirmation. `reason`: required field, 0–2000 characters.

`code` is **required** on every entry and is exactly one of:

- `null` — an explicit "no code version is carried forward for this issue";
- `{"pull_request": {"provider", "pull_request_id", "head_sha"}}` with all three
  fields, `provider` ∈ `github | vcs`, a full lowercase 40- or 64-hex `head_sha`.

An absent `code`, any other shape, an unknown sub-field, an abbreviated or
upper-case head: 400 `invalid_stage_request` (not stored). The reference is a
claim: increment 1 already verifies it in the confirming transaction. The server
accepts it only when it finds that pull request linked to that issue in the
source workspace (not a reference-only mention) with **exactly** that head; it
never records an unverifiable reference as confirmed. The client takes the values
from `pull_requests` of R1; it never invents them. What increment 1 does not do:
it checks no CI, trust revision or artifact (those belong to code candidates,
A6), and the head is the server's stored observation, not a live remote fetch.

Authority: human; current participant; the configured product approver.
Selecting any child additionally requires current source read authority; a
remembered digest from before access loss is not permission to confirm it.
An empty explicit scope selection retains its existing contract.

Success: `stage_scope_confirmed`

```json
"stage_confirmation": {"action": "confirm_scope", "issue_id": "<uuid>", "decision_id": "<uuid>",
  "scope_revision": "1", "lifecycle_epoch": "1"}
```

The client matches `action`, `issue_id`, `lifecycle_epoch ==
expected_lifecycle_epoch` and `scope_revision == expected_scope_revision + 1`.

Stored refusals in addition to 3.6:

| HTTP | code | condition |
| --- | --- | --- |
| 409 | `stage_scope_changed` | `expected_scope_revision` differs, a listed issue's current content digest differs, or a stated `head_sha` is not the pull request's current head |
| 409 | `stage_scope_child_unavailable` | a listed issue is not currently source-readable, missing, not ordinary, not admitted, mapped to another root, or not currently governed by this root (not below it, or below a nested collaboration issue) |
| 409 | `stage_scope_code_unavailable` | a stated pull request is not linked to that issue (unknown, another issue's, another provider's, or a reference-only mention) |

One refused entry refuses the whole confirmation; nothing is written.

Effects: one immutable scope decision that records, per listed issue, its
identity, content digest and code version (or explicit none); each listed issue
keeps its identity and gets (or refreshes) its mapping and confirmed code version
for the current lifecycle. Nothing is created, assigned, enqueued or
re-dispatched; issues that are not listed stay unconfirmed. Work that is already
running is not stopped.

After confirmation a new head makes that issue's `code_state` `changed`
(`unavailable` when the link is gone): the confirmed code version is then not
current and cannot be consumed as reviewed code. It does not reopen the scope,
because a head moves with every push of ordinary development; the approver
re-confirms that issue, or increment 2 binds the new head in a code candidate.

Signer-expiry boundary: the A5 write rechecks the current verified human
credential. A committed scope confirmation has no time limit and does not later
expire with that JWT/PAT. It stops being
current when the root lifecycle changes, the policy is disabled and re-enabled,
the role revision changes (moving and restoring a role does not revive it), or the signer's participant
or account generation changes (`scope_state`).

## 8. Increment-2 actions (frozen contract; availability in section 1)

### 8.1 Standing approvals and the signer-expiry boundary

Decisions of A7 are standing approvals consumed by later transitions. Each stores
the actual signer, role and role revision, policy revision, lifecycle epoch, the
signer's participant/account generation, verified credential class and a finite
`expires_at`. The request must carry `expires_at`; it must not exceed
`max_expires_at` published for that decision in R4 (the minimum of the server
horizon — currently 168 hours —, the evidence horizon and a finite signer
credential expiry). An over-limit value is refused `stage_expiry_exceeds_limit`
(400, not stored); it is never shortened silently. Consumption rechecks, at the
database clock: expiry, that the signer is still the configured holder with the
same participant/account generation and a still valid credential, the role and
policy revisions and the exact evidence. Failure answers 409
`stage_decision_expired`, `stage_signer_changed`, `stage_roles_changed`,
`stage_policy_changed` or `stage_evidence_changed`; a human signs again.
The private signer binding also retains the original source-workspace member
record identity. Removing and re-adding that membership cannot revive a standing
approval, even when the account, participant and configured role are unchanged.
This grant identity is never exposed in R4 or a confirmation.

### 8.2 A6 create/revise code candidate — kind `stage_create_code_candidate`

```json
{"expected_policy_revision": "1", "expected_role_revision": "1",
 "child_issue_id": "<uuid>",
 "supersedes": CandidateRef | null,
 "pull_request": {"provider": "github" | "vcs", "pull_request_id": "<uuid>", "head_sha": "<40 or 64 hex>"},
 "artifact": {"approval_id": "<uuid>"} | null}
```

Authority: human current participant. The implementer is copied from the role
configuration, never from the request; the submitter is recorded separately.
Success `stage_code_candidate_created`:

```json
"stage_confirmation": {"action": "create_code_candidate", "issue_id": "<uuid>", "candidate_id": "<uuid>",
  "version": "1", "content_digest": "<hex64>", "parent_candidate_id": null}
```

Stored refusals: `stage_needs_confirmation` (409), `stage_child_unconfirmed` (409,
child has no current mapping), `stage_pull_request_unavailable` (409, not linked to
the root/child, reference-only, or not readable), `stage_head_changed` (409),
`stage_trust_changed` (409), `stage_artifact_unavailable` (409),
`stage_candidate_changed` (409, `supersedes` is not the newest version).

### 8.3 A7 decide stage — kind `stage_decide`

```json
{"expected_policy_revision": "1", "expected_role_revision": "1", "expected_stage_revision": "1",
 "step": "code_review" | "qa" | "release" | "rollback",
 "decision": "approve" | "return",
 "candidate": CandidateRef,
 "evidence": {
   "ci_digest": "<hex64>" | null,
   "test_report_id": "<uuid>" | null,
   "build": {"artifact_digest": "<hex64>", "environment": "<1..128 chars>"} | null,
   "deployment_id": "<uuid>" | null,
   "rollback_target_digest": "<hex64>" | null
 },
 "depends_on": ["<decision uuid>"],
 "reason": "...",
 "expires_at": "2026-11-21T00:00:00.000000Z"}
```

Required evidence for **approve** per step: `code_review` → `ci_digest`; `qa` → `test_report_id`
and one valid code-review approval in `depends_on`; `release` → `build` and one
valid QA approval; `rollback` → `deployment_id`, `rollback_target_digest` and the
release approval of that deployment. `return` needs `reason` (1–2000) and no
`expires_at` (`null`). The current configured role may return a candidate with
failed or unsettled evidence; passing evidence and a valid preceding approval
are not prerequisites for return. Any optional evidence/dependency supplied on
return must still belong to that candidate/issue and be currently readable; it
need not be passing or currently authorizing. Return binds the exact candidate
and current workflow/policy/role/stage revisions and does not approve anything.
Role per step: code reviewer, QA approver, release
approver, release approver. The implementer may not sign `code_review`
(`stage_self_review_forbidden`, 409); QA cannot sign `release`/`rollback`
(`stage_role_required`, 403).

Success `stage_decision_recorded`:

```json
"stage_confirmation": {"action": "decide_stage", "issue_id": "<uuid>", "step": "qa", "decision": "approved" | "returned",
  "candidate_id": "<uuid>", "version": "1", "content_digest": "<hex64>", "decision_id": "<uuid>",
  "role_revision": "1", "policy_revision": "1", "expires_at": "..." | null}
```

Stored refusals: `stage_ci_missing`, `stage_ci_failed`, `stage_ci_stale`,
`stage_ci_unsupported` (VCS without repository identity), `stage_head_changed`,
`stage_evidence_changed`, `stage_evidence_unavailable`, `stage_dependency_invalid`,
`stage_test_report_unusable`, `stage_execution_fact_missing`,
`stage_candidate_changed`, `stage_stage_changed` (all 409). An approval is an
authorization fact only; it never states that a deployment or rollback happened.

### 8.4 A8 revoke decision — kind `stage_revoke_decision`

```json
{"expected_policy_revision": "1", "expected_role_revision": "1", "reason": "<1..2000>"}
```

Authority: the original signer while still holder of that role, or a current
source owner/admin participant. Success `stage_decision_revoked`:

```json
"stage_confirmation": {"action": "revoke_decision", "issue_id": "<uuid>", "decision_id": "<uuid>", "revocation_id": "<uuid>"}
```

Stored refusals: 404 `stage_decision_not_found`, 409 `stage_decision_not_revocable`
(already revoked, consumed by an irreversible fact, or a proposal/scope decision).
Revocation is a new terminal fact; the original decision stays readable.

### 8.5 A9 advance stage — kind `stage_advance`

```json
{"expected_policy_revision": "1", "expected_role_revision": "1", "expected_stage_revision": "1",
 "target_stage": "code_review" | "qa" | "release" | "verification" | "development",
 "candidate": CandidateRef,
 "decision_ids": ["<uuid>"]}
```

Forward moves require: `code_review` ← current PR/head/CI evidence of the candidate;
`qa` ← valid code-review approval; `release` ← valid QA approval; `verification` ←
valid release approval **and** a recorded external deployment fact. `development`
is the rework move and needs a `returned` decision. Authority: business owner or
the role that signed the unlocking decision. Success `stage_advanced`:

```json
"stage_confirmation": {"action": "advance_stage", "issue_id": "<uuid>", "stage": "qa", "stage_revision": "2"}
```

`decision_id` is absent because advancing records no human decision. Stored
refusals: `stage_stage_changed`, `stage_transition_invalid`,
`stage_needs_confirmation`, and every consumption code of 8.1/8.3 (409).

### 8.6 R4 code-candidate evidence (frozen)

`GET /api/collaboration/issues/{id}/stage/code-candidates/{candidateId}` → 200, or
404 `stage_candidate_not_found`. Authority: current issue read authority; source
evidence the viewer may not read is counted in `blocked_evidence` and never named.

```json
{
  "protocol_version": 1,
  "issue_id": "<uuid>",
  "checked_at": "...",
  "candidate": {
    "candidate_id": "<uuid>", "version": "1", "content_digest": "<hex64>", "parent_candidate_id": null,
    "lifecycle_epoch": "1", "state": "current" | "superseded" | "stale",
    "submitted_by": "<uuid>", "implementer_user_id": "<uuid>", "role_revision": "1", "created_at": "...",
    "baseline": {
      "requirement_digest": "<hex64>",
      "child_issue_id": "<uuid>", "child_semantic_version": "1",
      "pull_request": {"provider": "github", "pull_request_id": "<uuid>", "head_sha": "<hex>", "trust_revision": "1"},
      "artifact": {"approval_id": "<uuid>", "artifact_digest": "<hex64>"} | null
    }
  },
  "observed": {
    "pull_request": {"state": "current" | "head_changed" | "unlinked" | "trust_changed" | "unavailable", "head_sha": "<hex>" | null},
    "ci": {"state": "passed" | "missing" | "failed" | "stale" | "unsupported" | "unavailable",
           "ci_digest": "<hex64>" | null, "head_sha": "<hex>" | null, "observed_at": "..." | null, "check_count": 0}
  },
  "test_reports": [
    {"report_id": "<uuid>", "run_id": "<uuid>", "attempt": "1", "source_kind": "runtime_reported",
     "reported_result": "passed" | "failed" | "incomplete",
     "applicability": "usable" | "stale" | "execution_failed" | "execution_unsettled",
     "case_count": 3, "failed_case_count": 1, "received_at": "..."}
  ],
  "decisions": [
    {"decision_id": "<uuid>", "step": "code_review" | "qa" | "release" | "rollback", "decision": "approved" | "returned",
     "state": "valid" | "expired" | "revoked" | "signer_changed" | "roles_changed" | "policy_changed" | "evidence_changed" | "superseded",
     "actor_user_id": "<uuid>", "role": "code_reviewer" | "qa_approver" | "release_approver",
     "role_revision": "1", "policy_revision": "1", "created_at": "...", "expires_at": "..." | null, "reason": "...",
     "evidence": {"ci_digest": null, "test_report_id": null, "build": null, "deployment_id": null, "rollback_target_digest": null},
     "depends_on": ["<uuid>"],
     "revocation": {"revocation_id": "<uuid>", "actor_user_id": "<uuid>", "created_at": "...", "reason": "..."} | null}
  ],
  "execution_facts": [
    {"fact_id": "<uuid>", "kind": "deployment" | "rollback", "source_kind": "integration",
     "environment": "...", "operation_id": "...", "result": "succeeded" | "failed",
     "artifact_digest": "<hex64>", "previous_artifact_digest": "<hex64>" | null,
     "approval_decision_id": "<uuid>", "observed_at": "..."}
  ],
  "signable": [
    {"step": "code_review", "allowed": true, "max_expires_at": "...", "blockers": [StageBlocker]}
  ],
  "blocked_evidence": 0
}
```

`signable` has one entry per step in the order `code_review, qa, release,
rollback`; `allowed` is the viewer's current display hint and `max_expires_at` is
`null` when not allowed. These hints govern **approve only**, not return. The
client may offer return based on the current role and stage even when approval
is blocked; the server independently rechecks that authority and the exact
candidate/revisions. `StageBlocker` is defined in section 10.2. Lists are
bounded to 50 entries each, newest first; an approval or a human note never
appears in `execution_facts`. Enum values not listed here render as unknown and
disable the action.

R4 has an explicit source-denied variant. If the viewer can read the
collaboration issue but cannot currently read the candidate's source evidence,
the candidate's identity/attribution/version fields remain and `baseline` is
`null`. `observed.pull_request` is `{state:"unavailable",head_sha:null}`;
`observed.ci` is
`{state:"unavailable",ci_digest:null,head_sha:null,observed_at:null,check_count:0}`.
`test_reports`, `decisions` and `execution_facts` are empty arrays. Every
`signable` entry has `allowed:false`, `max_expires_at:null` and a content-free
`stage_evidence_unavailable` blocker with no object identifier. In this variant
`blocked_evidence` is `1`, indicating one withheld source-evidence group, not a
count of its hidden rows. It does not disclose whether reports, decisions or
execution facts exist. Loss of issue read authority still returns 404.

### 8.7 T1–T4 typed runtime test report

Transport is exactly `research/test-report-contract.md` sections 1, 4 and 5:

- T1 `GET /api/collaboration/runtime/runs/{runId}/test-subject` (bearer `mcb_`):
  200 `{version: 1, issue_id, run_id, subject: {candidate_id, candidate_version, candidate_digest, binding_digest, source_revision, artifact_digest}}`, where `artifact_digest` is hex64 or explicit `null` for source-only testing;
  409 `test_subject_unavailable` / `test_source_unbound`.
- T2 `POST /api/collaboration/runtime/runs/{runId}/test-reports` (bearer `mcb_`),
  body `{version: 1, identity, request_id, report: {subject, suite_id, cases[1..256], logs[1..64]}}`;
  200 ACK `{version: 1, issue_id, run_id, request_id, report_id, report_digest, accepted: true}`.
  Refusals: 400 `invalid_test_report`, 403 `forbidden`, 409 `test_subject_mismatch`,
  `test_report_conflict`, `test_report_already_recorded`, `test_report_log_not_retained`,
  `test_subject_unavailable`, `test_source_unbound`, 413 `request_too_large`.
  One immutable report per run attempt; the server derives `reported_result`;
  an ACK never means QA passed.
- T3 `GET .../test-reports/by-request/{requestId}` (bearer `mcb_`): the same ACK,
  or 404 (unconfirmed; the producer keeps the original request and never changes the key).
- T4 human reads return the report with cases/log references under current issue
  and source authority, `source_kind: "runtime_reported"` and an `applicability`
  of `usable | stale | execution_failed | execution_unsettled`.

### 8.8 T4 human report reads (frozen before 2b implementation)

Both routes use the existing authenticated human collaboration account scope,
discard ambient workspace headers and return `Cache-Control: no-store`. Current
issue/run and source-evidence read authority are required on every request.
Neither a prior report ID nor a cursor grants read authority.

`GET /api/collaboration/issues/{id}/runs/{runId}/test-reports?limit=20&cursor=<opaque>`:

```json
{
  "version": 1, "issue_id": "<uuid>", "run_id": "<uuid>",
  "items": [{
    "report_id": "<uuid>", "report_digest": "<hex64>", "agent_id": "<uuid>",
    "attempt": "1", "source_kind": "runtime_reported",
    "subject": {"candidate_id": "<uuid>", "candidate_version": "1", "candidate_digest": "<hex64>",
      "binding_digest": "<hex64>", "source_revision": "<full lowercase commit>", "artifact_digest": null},
    "reported_result": "passed", "applicability": "usable",
    "case_count": 2, "failed_case_count": 0, "received_at": "<UTC timestamp>"
  }],
  "next_cursor": null
}
```

`limit` is 1–50, default 20. Rows are newest run-event sequence first. The opaque
cursor binds issue/run/viewer and an initial sequence watermark; it cannot page
into another run/account or import newer events into an existing page sequence.
No total count is returned. An authorized run with no reports returns an empty
array and null cursor. Unknown/duplicate query keys, invalid limit or cursor:
400 `invalid_test_report_query`.

`GET .../test-reports/{reportId}` returns the same single item fields plus:

```json
{
  "version": 1, "issue_id": "<uuid>", "run_id": "<uuid>",
  "report_id": "<uuid>", "report_digest": "<hex64>", "agent_id": "<uuid>",
  "attempt": "1", "source_kind": "runtime_reported",
  "subject": {"candidate_id":"<uuid>","candidate_version":"1","candidate_digest":"<hex64>",
    "binding_digest":"<hex64>","source_revision":"<full lowercase commit>","artifact_digest":null},
  "reported_result": "failed", "applicability": "execution_failed",
  "case_count": 2, "failed_case_count": 1, "received_at": "<UTC timestamp>",
  "suite_id": "payment-regression",
  "cases": [{"id":"callback-delay","outcome":"failed","log_index":0},
    {"id":"callback-duplicate","outcome":"passed","log_index":1}],
  "logs": [
    {"kind":"record","event_id":"<uuid>","message_index":2,"body_sha256":"<hex64>","body":"<retained redacted body>"},
    {"kind":"inline","body_sha256":"<hex64>","body":"<redacted report input>"}
  ]
}
```

Case outcomes are exactly `passed | failed | error | skipped`; `log_index` and
`message_index` are nonnegative JSON integers. Cases/logs retain their original
order; `failed_case_count` counts failed/error. Limits remain 1–256 cases and
1–64 logs, each resolved body at most 64 KiB; inline bodies total at most 64 KiB.
Detail resolution never fetches a filesystem path or remote URL and never
truncates a body into apparent completeness. Record logs retain their original
event/index/hash and resolve only that immutable body; inline logs never acquire
fabricated event identifiers.

Applicability is current, separate from immutable `reported_result`: mismatched
current candidate/lifecycle/source or unavailable required retained evidence is
`stale`; otherwise failed/forced-failure execution is `execution_failed`;
otherwise unconfirmed execution/settlement is `execution_unsettled`; only the
remaining source-bound, settled successful execution is `usable`. `usable` is
not QA approval. Source-only `artifact_digest:null` never proves a build.

Both reads return 404 `not_found` for missing/foreign objects or lost issue/source
authority, without cases/logs. An unavailable retained log on an otherwise
readable detail returns 409 `test_report_log_not_retained`; malformed/unavailable
retained report storage returns 503 `test_report_unavailable`, with no partial
detail. Error bodies are `{code,error}`. List summaries contain no case names,
suite text or log bodies. T4 is separate from the old run-record item union.

### 8.9 C1 create-only typed test input (frozen before 2b implementation)

The existing `POST /api/collaboration/issues/{id}/comments` accepts one optional
new field, only on creation:

```json
"test_subject": {"candidate_id":"<uuid>","candidate_version":"1","candidate_digest":"<hex64>"}
```

Omission preserves ordinary comments. Explicit null, unknown/duplicate/case-alias
keys or malformed references are 400 `invalid_test_subject`. Comment edit and
runtime comment endpoints do not accept this field. All existing comment fields,
typed Agent/target mention syntax, target versions, suppression, attachments,
idempotency key and comment receipt/recovery envelopes remain unchanged.

For a test input, `execution_view_version` is 6; lifecycle, participant and account
observations must be positive strings. The body must contain at least one
existing typed execution target with its existing target/version observations;
plain prose never selects a target. A current human with existing SubmitCommand
and resource-invocation authority may initiate tests; profession is irrelevant.
The server additionally requires a managed QA-stage issue, current scope/roles,
the exact current candidate and its valid code-review approval, plus current
source read authority. It freezes the matching approval/code/artifact facts,
not a caller-supplied success flag.

Subject validation precedes comment creation and attachment binding. Refusals
include 409 `test_subject_target_required`, `stage_policy_disabled`,
`stage_needs_confirmation`, `stage_stage_changed`, `stage_candidate_changed`,
`stage_candidate_stale`, `stage_dependency_invalid`, `stage_evidence_unavailable`
and existing standing-approval consumption codes. Normal auth/authority refusals
retain existing envelopes. Subject is part of the original comment request
digest. Recovery does not bind new commands or relabel an old subject.

The existing comment receipt stays authoritative per target: only an actual
successfully appended command gets a subject binding; blocked targets get none.
Comment creation/queuing never means a test ran or passed. First claim uses the
actual frozen prompt command IDs and exact Git preparation to freeze one
consistent original subject as a run event. Distinct subjects refuse with 409
`test_subject_ambiguous`; missing Git refuses `test_source_unbound`; wrong
commit/artifact refuses `test_subject_mismatch`. No subject leaves the legacy run
path unchanged. Old runs never acquire the current candidate at upload time.
Superseded subjects retain original report history and cannot satisfy current QA.

### 8.10 Increment 2c release, rollback and execution reads (frozen 2026-11-14)

This amendment supersedes the earlier incomplete release/rollback details in
8.3–8.6. The routes remain the existing A7/A8/A9/R4 routes; these additions are
frozen but unserved until the server's 2c result says otherwise. No public
execution/fact/metric writer or verifier registration endpoint is added.

A7 adds `build.target_digest` and optional `evidence.rollback_observation`:

```json
"evidence": {
  "ci_digest": null,
  "test_report_id": null,
  "build": {"artifact_digest":"<hex64>","environment":"<1..128 UTF-8 bytes>","target_digest":"<hex64>"},
  "deployment_id": null,
  "rollback_target_digest": null,
  "rollback_observation": null
}
```

For rollback the required reference is:

```json
"rollback_observation": {"observation_id":"<uuid>","observation_digest":"<hex64>"}
```

- Release approve is signed in `release`, requires the exact selected build,
  required target digest from R4, and exactly one current valid QA approval in
  `depends_on`. The candidate's server-observed artifact, that QA's selected
  report's non-null artifact digest, and the requested build must agree.
  Source-only reports cannot satisfy release. `deployment_id`,
  `rollback_target_digest` and rollback observation are null/absent.
- Rollback approve is signed in **`verification`**; `rollback` is a decision
  step, never a stage. `deployment_id` is the server UUID of the selected
  successful **deployment fact**, not an intent or native operation ID.
  `depends_on` contains exactly that deployment's current valid release
  approval. The required metric reference identifies current, authenticated,
  unexpired anomalous evidence for that exact deployment and target.
- Rollback v1 supports only the deployment fact's **immediately previous
  observed artifact**. `rollback_target_digest` must equal it; null/unknown
  previous artifact refuses. No latest or arbitrary historic artifact is chosen.
  `build` is null for rollback; its environment/target come from the selected
  deployment and are re-resolved. The provider must support conditional target
  state for that original deployment; otherwise execution is unavailable.
- Reason-only return retains the existing role and stage rules, with no
  expiry or requirement for passing preceding evidence. Any supplied evidence
  remains same-candidate/deployment/target and currently readable. Non-rollback
  steps reject a non-null rollback observation. Code-review/QA requests must
  omit `target_digest`; their old five nullable evidence fields retain their
  original digest/receipt semantics. The new optional metric member is omitted
  from internal request canonicalization when absent/null; private resolved
  source fields never enter the human request digest.
- A8 refuses `stage_decision_not_revocable` once an execution **intent commits**
  for that approval, including pending, unknown, failed and proven-not-started
  history. This consumes one execution opportunity and is not proof of a
  deployment or an external cancellation. A new explicit approval may authorize
  new work after all current checks; it never restores the old opportunity.
- A9 `release → verification` retains the existing body and requires exactly
  one selected current-valid release approval in `decision_ids`. Derive its
  unique successful deployment fact through that approval's one intent; never
  select the latest fact or let a new approval endorse an old deployment.
  A failed, unknown, missing, wrong-target or wrong-artifact result refuses.
  Late authenticated results remain history while current approval/role/source/
  lifecycle/expiry checks can still block A9. No renewal or endorsement action,
  implicit re-invocation or automatic stage advance exists.

R4 adds three arrays, each bounded to 50. A 2c response always includes them;
absence on an older response means unavailable/unknown, not an empty successful
execution history. Targets are ordered by environment then digest; retained
operations/metric observations are newest received/created first with UUID as
stable tie-breaker. There is no hidden total count.

```json
"execution_targets": [{
  "environment":"staging", "target_digest":"<hex64>", "source_kind":"integration"
}],
"execution_operations": [{
  "intent_id":"<uuid>", "kind":"deployment",
  "approval_decision_id":"<uuid>", "environment":"staging",
  "target_digest":"<hex64>", "artifact_digest":"<hex64>",
  "state":"pending", "external_operation_id":null, "fact_id":null,
  "created_at":"<UTC timestamp>"
}],
"rollback_observations": [{
  "observation_id":"<uuid>", "version":"1", "observation_digest":"<hex64>",
  "deployment_id":"<successful deployment fact uuid>",
  "environment":"staging", "target_digest":"<hex64>", "artifact_digest":"<deployed hex64>",
  "condition":"anomaly", "state":"current",
  "window_start":"<UTC timestamp>", "window_end":"<UTC timestamp>",
  "observed_at":"<UTC timestamp>", "valid_until":"<UTC timestamp>",
  "summary":"<redacted, at most 2000 UTF-8 bytes>"
}]
```

Operation `kind` is `deployment | rollback`; state is
`pending | unknown | not_started | succeeded | failed`. An external operation ID
is nullable until authenticated and, when present, a nonempty native string of
at most 256 UTF-8 bytes. `fact_id` is non-null only for confirmed succeeded/failed
execution, never pending/unknown/not_started. `artifact_digest` names the
**intended** artifact and cannot establish which artifact is live.

Metric condition is `anomaly | healthy | unknown`; public state is
`current | ambiguous | superseded | expired | unavailable`. `version` is a
server append version, independent of the private provider ordering version.
The registered source determines query/rule identity, native ordering and finite
freshness. Transport arrival never renews that horizon. Equal native version
with different canonical content makes the stream ambiguous, retains both
observations, and blocks both new rollback approval and an existing approval's
consumption. Only a strictly newer authenticated observation can resolve it;
older delayed observations cannot clear ambiguity. New healthy/unknown evidence
or expiry also invalidates the earlier anomaly.

Existing `execution_facts` gains required `intent_id` and `target_digest` on each
2c row. Its `operation_id` is a nonempty **native** external operation ID (up to
256 UTF-8 bytes), not necessarily UUID. `result` remains `succeeded | failed`;
`artifact_digest` is the intended artifact, and a succeeded fact additionally
proves the observed resulting artifact equals it. The previous-artifact field
is nullable and describes an actual observed predecessor only. Approvals,
notes, pending/unknown/not_started operations and metric rows are never facts.

Source-denied R4 retains the exact 8.6 variant: baseline null, unavailable PR/CI,
all reports/decisions/facts/**new arrays** empty, `blocked_evidence:1`, and only
anonymous evidence-unavailable blockers. Nil/unconfigured production execution
sources expose no targets and cannot approve/admit execution; retained safe
history remains separate from current applicability. Raw samples, proof bytes,
credentials, local integration IDs and the physical conflict key are private.

New stored A7/A9 refusal and R4 blocker codes, all HTTP409 when used as writes:
`stage_execution_unavailable`, `stage_execution_target_changed`,
`stage_execution_pending`, `stage_execution_conflict`,
`stage_rollback_observation_unavailable`, `stage_rollback_observation_changed`.
Existing role/candidate/evidence/policy/signer/expiry codes retain their meanings.
Malformed required refs/digests are400 `invalid_stage_request`, not stored.
Unsupported or ambiguous target mappings refuse; a foreign target's unresolved
operation causes an anonymous pending refusal without its IDs or row count.

The internal dispatch/finalization and test-only control protocol is frozen in
`research/server-increment-2c-control-contract.md`; none of those DTOs are public
HTTP capabilities. Entry into verification is not proof of health/payment checks.
The same registered source dependency must be propagated through every 2d
submission/completion/project-exit writer and its private final guard.

## 9. Activity, notifications and realtime

Each successful A-action that changes state writes one collaboration event whose
`kind` equals the success code (`stage_roles_configured`, `stage_proposal_created`,
`stage_proposal_revised`, `stage_proposal_accepted`, `stage_proposal_rejected`,
`stage_scope_confirmed`, and in increment 2 `stage_code_candidate_created`,
`stage_decision_recorded`, `stage_decision_revoked`, `stage_advanced`) and one
reference-only notification of the same kind per other active participant, through
the existing event sequence/outbox (no second outbox, deduplicated by event).
Until the activity allowlist of increment 3 lands, the existing activity API
projects these events as kind `other`. The safe projection will contain only stage,
candidate/decision identifiers, versions and the acting account.
Stage Query data must be invalidated on the existing collaboration issue
invalidation; no new realtime channel exists.

## 10. Stage codes on existing endpoints

### 10.1 Machine admission of ordinary issues (increment 1, served)

An ordinary issue is **governed** by its nearest collaboration ancestor (parent
chain, at most 64 levels up). The gates below apply when that root is *managed*
(its project has an enabled stage policy) — to its direct children **and to every
deeper descendant**. Unrelated issues, issues under an unmanaged root and disabled
projects keep their exact previous behaviour. A human's manual creation,
comment-to-child and later parent attach keep their previous rights at every
depth and never approve an AI proposal or create a proposal mapping.

Source finding behind the subtree rule: before it, a task token / agent actor
could create or attach an issue below an ordinary child of a managed root
(`IssueService.CreateInTx`, `updateIssueAtomicallyOnce`) and that issue could be
enqueued and started (`checkIssueExecution`, `startTaskAtomically`); no existing
rule prevented it. That path is now refused and tested at each of those symbols.

| Entry point | HTTP | code | condition |
| --- | --- | --- | --- |
| issue create (`POST /api/issues`, quick-create completion, any other creation with a parent) | 409 | `stage_proposal_required` | a machine-driven creation (task token, agent actor headers, agent creator) names a parent that is a managed root or is governed by one |
| issue update / move / batch update | 409 | `stage_proposal_required` | a machine-driven update attaches an issue to such a parent |
| the two rows above | 409 | `stage_ancestry_unresolved` | machine-driven only: the parent chain cannot be resolved within 64 levels (very deep chain or a parent cycle), so it is not provably outside a managed root. Humans are not affected |
| task enqueue (assign, mention, rerun, squad leader, lifecycle handoff, triage accept-and-execute, deferred fallback creation) and daemon task start / wait-for-directory | 409 | `stage_child_unapproved` | machine-origin issue governed by a managed root with no proposal/scope mapping |
| same | 409 | `stage_scope_unconfirmed` | mapped machine-origin issue whose approval belongs to an earlier root lifecycle |
| same | 409 | `stage_ancestry_unresolved` | agent-created, unmapped issue whose parent chain cannot be resolved within 64 levels |

Machine-origin means: materialized from an accepted proposal, or created with an
agent creator. Human-created issues keep their ordinary execution contract at
every depth. A mapping is the issue's persistent identity: removing or replacing
its parent does not erase it. An existing machine-origin descendant (created
before enablement, or attached by a human) becomes executable only after the
product approver lists it in A5. Proposals materialize direct children only.
An approved or scope-confirmed child does not authorize further machine
decomposition in v1. A human must propose another direct child or create the
descendant manually through the existing human path.

Bodies: create/update `{"code", "error", "message"}` (the existing delivery
refusal shape), the whole write rolled back; batch update reports the item through
the existing committed/rejected/not_attempted/unknown result. Daemon task start
and wait-for-directory:
the existing `{"error": "<code>"}`. Lifecycle handoff: `{"code", "error"}`.
Enqueues that the server performs as a best-effort side effect of another write
(assignment, mention) do not fail that write; the task is simply not created.
Claiming (queued → dispatched) is not the gate; the start transaction (dispatched
→ running) is, so a task queued before the gate applied stays unstarted until the
issue is confirmed.

### 10.2 Incomplete stage evidence on completion and exit paths (frozen for increment 2, not served)

When a stage policy is enabled, the existing writers below refuse with these
codes. The M1 completion contract stays independent and may refuse first or
additionally with its own codes.

```json
StageBlocker = {"code": "<blocker code>", "stage": "development" | "code_review" | "qa" | "release" | "verification",
                "candidate_id": "<uuid>", "decision_id": "<uuid>", "child_issue_id": "<uuid>"}
```

`code` and `stage` are always present; the three identifiers are present only when
the blocker concerns that object. A blocker never carries prose, names, logs or
evidence content, and evidence the caller may not read is reported as
`stage_evidence_unavailable` without an identifier. Blocker codes (closed set v1;
unknown codes render as unknown and still block):

| Blocker code | Missing item |
| --- | --- |
| `stage_roles_unconfigured`, `stage_scope_unconfirmed` | role configuration / current existing-child confirmation |
| `stage_not_reached` | the root is not yet in `verification`; `stage` is the current stage |
| `stage_code_candidate_missing`, `stage_candidate_stale` | no current code candidate / requirement, child, head or build changed since it was frozen |
| `stage_pull_request_unavailable`, `stage_head_changed`, `stage_trust_changed` | PR unlinked or unreadable / new head / integration trust changed |
| `stage_ci_missing`, `stage_ci_failed`, `stage_ci_stale`, `stage_ci_unsupported` | no check set for the head / a check not `success` / observation older than 30 minutes / VCS without repository identity |
| `stage_code_review_missing`, `stage_qa_missing`, `stage_release_approval_missing` | no valid approval of that step for the exact candidate |
| `stage_test_report_unusable` | QA depends on a report that is stale, failed in execution or unsettled |
| `stage_execution_fact_missing` | no recorded deployment (or rollback) fact for the approved build |
| `stage_decision_expired`, `stage_decision_revoked`, `stage_signer_changed`, `stage_roles_changed`, `stage_policy_changed` | a required approval no longer applies; `decision_id` names it |
| `stage_evidence_changed`, `stage_evidence_unavailable` | bound evidence differs / cannot be read by the caller |
| `stage_submission_unbound` | the delivery submission is not bound to the verified candidate |

Refusal body = the stored workflow refusal of 3.3 plus the blocker list:

```json
{"code": "stage_incomplete", "error": "stage_incomplete", "status": 409, "collaboration_revision": "12",
 "stage_blockers": [ {"code": "stage_ci_missing", "stage": "code_review", "candidate_id": "<uuid>"} ],
 "stage_blockers_truncated": false}
```

`stage_blockers` holds 1–32 entries ordered by stage order, then code;
`stage_blockers_truncated` is `true` when more exist. The list is part of the
stored refusal and of its O1 recovery (identifiers only, so it survives access
loss). It describes the moment of refusal; the client re-reads R1/R4 for current state.

| Existing endpoint | HTTP | code | Body / effect |
| --- | --- | --- | --- |
| `POST /api/collaboration/issues/{id}/acceptance-submissions` | 409 | `stage_incomplete` | body above; no attachment is bound and no submission is created |
| `POST /api/collaboration/issues/{id}/lifecycle` with `action: "complete"` (exact submission or self-result) | 409 | `stage_incomplete` | body above; no status, queue or delivery change |
| R38 administrative completion of a disabled creator's task | 409 | `stage_incomplete` | that endpoint's existing refusal envelope plus `stage_blockers` and `stage_blockers_truncated`; the administrator gains no stage exemption |
| `PATCH /api/collaboration/issues/{id}` moving a managed root to another project or clearing its project | 409 | `stage_policy_exit_required` | stored refusal of 3.3, no blockers; the stage policy must be explicitly disabled first |
| `DELETE /api/projects/{projectId}` while its stage policy is enabled | 409 | `stage_policy_exit_required` | `{"code", "error", "message"}`; nothing deleted; delete-impact reads report the same code as a blocking reason |
| `POST .../lifecycle` with `cancel` | — | — | not a completion; not gated |
| `POST .../lifecycle` with `reopen` | — | — | allowed; the new lifecycle resets the stage to `development` (`stage_revision` + 1) and needs a new A5; approvals of the earlier lifecycle cannot be consumed |
| ordinary `PUT /api/issues/{id}`, move, batch, delete on a governed ordinary issue | — | — | not refused by the stage gate; a content/project/parent change advances `semantic_version`, and candidates that depend on the old version or on a deleted issue report `stage_candidate_stale` / `stage_evidence_unavailable` when consumed |
| deleting the managed root itself | 409 | `unsupported_collaboration_action` | unchanged existing collaboration boundary |
| GitHub/VCS merge auto-advance | — | — | unchanged: it never completes a collaboration issue |

A replayed original completion that was already consumed returns its historical
receipt and is not re-evaluated against the current stage.

## 11. Summary of success codes and confirmation variants

| Action | Success code(s) | `stage_confirmation` fields |
| --- | --- | --- |
| P2 | `stage_policy_revised` | `action, project_id, policy_revision, enabled` |
| A1 | `stage_roles_configured`, `stage_roles_unchanged` | `action, issue_id, role_revision` |
| A2 | `stage_proposal_created` | `action, issue_id, proposal_id, candidate_id, version, content_digest` |
| A3 | `stage_proposal_revised` | A2 fields + `parent_candidate_id` |
| A4 | `stage_proposal_accepted`, `stage_proposal_rejected` | `action, issue_id, proposal_id, candidate_id, version, content_digest, decision, decision_id, role_revision, policy_revision` |
| A5 | `stage_scope_confirmed` | `action, issue_id, decision_id, scope_revision, lifecycle_epoch` |
| A6 | `stage_code_candidate_created` | `action, issue_id, candidate_id, version, content_digest, parent_candidate_id` |
| A7 | `stage_decision_recorded` | `action, issue_id, step, decision, candidate_id, version, content_digest, decision_id, role_revision, policy_revision, expires_at` |
| A8 | `stage_decision_revoked` | `action, issue_id, decision_id, revocation_id` |
| A9 | `stage_advanced` | `action, issue_id, stage, stage_revision` |

## 12. Compatibility notes for the client

- An older server has none of these routes: R1 404 without a JSON `code` means the
  stage capability is *unknown/unavailable*; never render gates as passed.
- The collaboration detail DTO is unchanged by increment 1.
- New enum values (stages, states, codes) need a `default` branch that renders
  *unknown* and disables the action.
- All stage objects are server state: React Query only, keyed under the existing
  collaboration issue key; pending intents persist the frozen key + body only.

## 13. Changes after first freeze

1. (2026-11-14, before any client code was reported) Section 5: `Account.external`
   is the existing boolean content-account field, not a nullable value; an
   unresolvable role account is `null`. Reason: the server reuses the existing
   content account projection instead of adding a second account shape.
2. (2026-11-14) Section 10: the enqueue entry list names the paths that actually
   pass the shared gate ("deferred promotion" removed, "deferred fallback
   creation", "squad leader" and "wait-for-directory" added) and states the exact
   body per endpoint and that claim is not the gate. No code or status changed.
3. (2026-11-14, coordinator review item 1) A5 and R1: every `children` entry of A5
   now has a **required** `code` field (explicit `null`, or one pull request
   reference with provider, id and full head) that the server verifies in
   increment 1; new stored refusal `stage_scope_code_unavailable`;
   `stage_scope_changed` also covers a stale head. R1 child items gain `code`,
   `code_state` and `pull_requests`. A moved head sets `code_state: "changed"`
   and does not reopen the scope. Server: migration 820.
4. (2026-11-14, coordinator review item 2) Section 10.1 and R1/A5: machine
   admission covers the whole subtree governed by a managed root, not only its
   direct children; new code `stage_ancestry_unresolved`; R1 `children` lists the
   governed subtree with a new `depth` field (`attached` is now "inside the
   subtree"); A5 accepts governed descendants and `stage_scope_child_unavailable`
   means "not governed by this root".
5. (2026-11-14, coordinator review item 3) Section 10.2 added (frozen for
   increment 2): `stage_incomplete` with `stage_blockers` on submit / complete /
   R38, `stage_policy_exit_required` on project move/clear/delete, and the
   behaviour of cancel, reopen and ordinary child writes. Section 8.6 now freezes
   the R4 shape (404 code `stage_candidate_not_found`). Nothing of this is served.
6. (2026-11-14, increment 1b completion) Section 10.1 explicitly states that an
   approved child cannot authorize further machine decomposition. Batch attach
   now retains `stage_proposal_required` / `stage_ancestry_unresolved` in the
   existing per-item refusal; daemon wait-for-directory uses the documented 409
   `{"error":"<code>"}` just like start. A5 with non-null code now holds the
   existing M1 PR/link `SHARE MODE NOWAIT` fence through commit; contention uses
   the existing bounded workflow retry / `collaboration_busy` path. A5 null
   remains an explicit no-code choice. No request or success shape changed.
7. (2026-11-14, increment 1b review limitation) The child `content_digest` remains
   SHA-256 of current title/description. It permits equality comparison and
   offline guesses of low-entropy descriptions; it is not a confidentiality
   proof. External child readers still receive it under the existing R1
   projection. The coordinator retains this privacy boundary for independent
   review; increment 1b does not change the frozen digest protocol or claim that
   only equality is disclosed.
8. (2026-11-14, before increment 2a implementation) R1 gains bounded identity-only
   `code_candidates` discovery as defined in 5.1. R4 `candidate.baseline` becomes
   nullable with the exact source-denied projection above; no hidden source
   rows, reasons or counts leak through its other arrays or blockers.
9. (2026-11-14, before increment 2a implementation) R4 `signable.allowed` and
   `max_expires_at`, plus mandatory successful step evidence and valid preceding
   approvals in A7, govern approve only. Return uses a reason, null expiry,
   current role/stage and exact candidate/revisions; optional supplied evidence
   still requires same-object/current-read verification. Failed/unsettled
   evidence does not prevent return. No action request or success shape changed.
10. (2026-11-14, increment 2a boundary) T4 list/detail/case/log/pagination DTOs
    remain pending the server's increment 2b freeze. The client consumes only
    the frozen R4 test-report summaries until then; this is not an implemented
    or inferred T4 contract.
11. (2026-11-14, coordinator clarification during 2a) Section 7.5 A5 is an
    immediate scope confirmation: its write requires a current human credential,
    while later JWT/PAT expiry does not expire the confirmed scope. Its retained
    applicability is lifecycle/policy/participant/account/role revision. Section
    8.1's finite expiry and original credential identity/current validity apply
    to A7 standing approvals. This narrows design F01's general wording; no scope
    credential column or new scope expiry rule is introduced.
12. (2026-11-14, 2a current-authority review) A7 binds the original source member
    record as well as current source membership. Consumption rechecks that exact
    identity under the live member lock. Rejoining permits a fresh signature,
    never reuse of the old approval; public DTOs are unchanged.
13. (2026-11-14, 2a connector review) An older GitHub observation cannot overwrite
    a newer mirror or derive links after losing to a newer observation. Equal
    provider timestamps with different heads are ambiguous: the same upsert
    statement invalidates strict snapshot completeness while preserving the
    known head/metadata, and the callback derives no links or completion intent.
    Later valid source evidence is required. Equal timestamps with the same head
    retain the existing metadata-event behaviour. This does not claim remote
    integration acceptance or a general webhook delivery/reconciliation system.
14. (2026-11-14, coordinator confidentiality correction, supersedes item 7) R1
    child `content_digest` becomes nullable: `can_open:false` requires null;
    `can_open:true` requires hex64. No dummy digest is substituted. Nonempty A5
    selection requires current source read authority even for a cached digest.
    Identifier/title visibility and historical minimal receipts are unchanged.
    Old 2a evidence stays preserved; this correction is recorded separately.
15. (2026-11-14, before 2b implementation) Sections 8.8/8.9 freeze full T4
    list/detail/case/log/error/pagination and create-only comment test_subject
    transport. Subject `artifact_digest` permits source-only null, matching
    test-report-contract §4. Existing mcb identity, one report per run/attempt,
    historical receipt before current-business-lock lookup, and original comment
    transport/receipt semantics remain unchanged.
16. (2026-11-14, increment 2b implementation) T1–T4 and C1 are registered;
    A7 QA approve/return, A9 QA→release and QA→development, and R4 test-report
    projections are served. Reports remain runtime_reported and have no implicit
    QA or stage effects. Release/rollback decisions and execution facts await 2c;
    project policy activation and the final writer inventory await 2d. Cursor
    integrity binds the original run/issue/viewer/watermark using a private
    per-run secret already retained with the source binding; no secret or
    manifest digest enters a public response. This records implementation,
    not independent acceptance or real external execution.
17. (2026-11-14, before2c implementation) Section8.10 freezes the accepted
    2c review amendments: exact target/build and metric refs; bounded R4 target,
    operation and metric arrays; physical-target uncertainty across local
    bindings; synchronous service-owned one-shot entry after definite commit;
    durable equal-native-version metric ambiguity; verification-stage rollback
    to the immediately previous artifact; A8 consumption at intent commit; and
    A9 fact derivation only from the selected current-valid approval. Existing
    code-review/QA canonical request identities must remain unchanged. No
    public producer/simulation/endorsement endpoint is introduced. 2b results
    remain historical; this freeze is not implementation or acceptance.
18. (2026-11-14, before2d implementation) The approved F06 mechanical read/activity
    handoff is frozen in `research/server-increment-2d-client-contract.md`:
    collaboration-root and R38 candidate/preview routes with explicit ordinary
    child source, numeric root/child issue revisions, existing closed M1 target
    variants and server-derived consumer; additive source-safe stage blockers
    and bounded activity metadata. These routes/fields remain unserved until2d.
    Ordinary preview behavior, artifact authority and the human approval rules
    are unchanged. This docs-only freeze permits parallel client work, not
    policy activation or2d completion.
19. (2026-11-14, increment2c implementation) A7 release and verification-stage
    rollback, A8 consumption at committed execution intent, A9 exact selected
    release-approval fact derivation, and R4 target/operation/metric arrays are
    implemented. The internal producer is server-constructed; production has
    no adapter by default, and no public producer/simulation route exists.
    Each consumed original artifact-bound signer also retains the existing M1
    source-read authority; another manager consumer cannot substitute after
    that authority is withdrawn. Focused isolated-source normal/race/router
    evidence is recorded in `research/server-increment-2c-results.md`; full
    independent acceptance and2d policy activation remain separate. The
    controlled JSONL harness is test-only and its full Web/Desktop journey
    still waits for2d and coordinator execution.
