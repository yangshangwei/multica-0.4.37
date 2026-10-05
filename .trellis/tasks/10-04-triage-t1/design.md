# Triage T1 technical design

Status: implementation design, based on reviewed v0.2 PRD. The user explicitly requested design, decomposition and execution in this branch; routine decisions below use the PRD defaults.

## Decisions and alternatives

Reuse Issue as the one content/attachment/comment identity. Add `issue.admission_status` (`not_required` default, `pending`, `accepted`, `rejected`, `duplicate`) and a workspace-scoped `issue_triage` annotation, plus settings, append-only action ledger and import batch/row records. Candidate project/assignee live in the annotation until acceptance. This prevents existing project and assignee behavior from observing a premature formal association.

Rejected: a second independent task entity, because it duplicates content, identity, permissions and attachments. Rejected: labels or user-editable issue metadata, because generic writes could bypass admission and audit. Rejected: placing candidate project/assignee in formal columns, because many execution/statistic consumers assume those relationships are active.

The existing canonical status catalogue remains unchanged. Pending issues store backlog; acceptance chooses an active backlog/todo-category key; rejection/duplicate store cancelled with the distinct admission result. `not_required` and `accepted` are the only formal states. Unknown future states fail closed at execution boundaries.

## Ownership and tables

- `issue.admission_status`: server owned; never accepted through ordinary issue PATCH/metadata. Expose it in HTTP and realtime Issue DTOs; older responses default to not_required on the new client.
- `workspace_triage_settings`: workspace identity, enabled, acceptance status, require_priority, responsibility_mode (`none`, `notify`, `assign`), responsibility_member_id, revision. No row means supported but disabled. Ordinary workspace settings JSON cannot modify it.
- `issue_triage`: issue/workspace identity, candidate project and assignee pair, reviewer user ID, first/round entered timestamps, round, snoozed_until, duplicate target ID plus historical identifier, source (`manual`, `csv`), original link/external ID, import batch/file/row attribution. Keep history even when referenced target/member disappears. No FK or cascade.
- `triage_action`: identity, workspace/issue, actor, request_id, payload digest, expected revision, action, round, before/after and changed fields, reason, time, execution status/error/task ID. A unique workspace/actor/request_id key gives durable replay; changed payload under the same key is 409. Task ID is retained after execution finishes.
- `triage_import_batch` / `triage_import_row`: authenticated owner, immutable content digest/mapping/raw rows, preview/resolution, row selection/override, per-row stable request ID, resulting issue/error. Preview may persist temporary batch records but must not create issues or notifications. A batch belongs to its workspace and submitting actor. Mapping changes create a new preview, never modify a committed batch.
- `triage_notification`: durable dedup/outbox identity, action or batch/round source, recipient, inbox item ID/delivery state. Insert result inbox records under the same transaction or dispatch retryably with a stable source key. Check recipient membership when delivering; send no external messages.

New tables have NOT NULL identifiers, with uniqueness implemented through separate `CREATE UNIQUE INDEX CONCURRENTLY` migrations. All other indexes are separate concurrent migrations too. Use migration 513 onward; reserve 512 for the unrelated MCP branch. Rehearse populated rollback refusal and empty rollback in private schemas. Cleanup follows application transactions and workspace deletion manifest.

## Transactions, races and execution

Intake uses IssueService.CreateInTx with AllowDuplicate=true (similar titles cannot silently vanish), backlog and no formal project/assignee. Validate attachments, dates, titles, labels and candidates with existing scoped validators. Commit issue, annotation, request/action identity and notification together. Publish a dedicated `triage:updated` event after commit, not the ordinary create event that could trigger implicit assignment.

Lock order: workspace/key-share existence and settings enable fence; sorted relevant subscriber/revocation advisory guards before member rows; status catalogue shared lock if applicable; referenced squad/agent and other ordered targets; sorted issue rows; action/import-row results. Existing issue/task owner locks must be reconciled with the audit findings rather than reversed. Follow the lifecycle NOWAIT/retry-on-rolled-back-contention protocol for overlapping owner locks; do not hold an action lock while waiting for an issue lock held by a decision waiting for that action. Settings disable and intake/reopen use the same workspace-scoped advisory lock, including when no settings row exists. Disable checks every pending item including snoozed. Membership and referenced-entity access are checked again within the final write transaction. Use revision compare-and-swap; conflicts return current state without replacing input.

Reviewer and responsibility identities always store user UUIDs, not member-row UUIDs, and use current workspace membership for validity. Issue deletion removes live triage annotations but preserves consumed request/action tombstones and successful import-row/external-ID provenance. A replay whose resulting issue was deleted returns a deleted-result conflict/reference instead of recreating it. Retained history may redact deleted content but keeps identity and decision metadata; deleting the entire workspace removes all triage-owned rows. Duplicate target deletion never erases the historical target identifier.

Ordinary acceptance commits admission and all requested fields atomically with its action record and no task enqueue or automatic-execution event. No implicit project squad inheritance. Candidate project/assignee become formal only in that transaction. Reopen is limited to rejected/duplicate, requires enabled settings and reason, restores pending/backlog and a new round; it cannot reopen accepted/not_required.

Accept-and-execute is explicit and separately authorized for persisted agent or squad leader. First validate admission and requested execution configuration. Commit the accepted task and pending execution intent. Prepare remote overlays outside locks; then use PrepareIssueTaskEnqueue / EnqueuePreparedIssueTaskInTx with reauthorization and an action-row lock, persisting the resulting task ID in the same transaction as task insertion. Only then FinalizeIssueTaskEnqueue. A failure leaves accepted plus execution=failed, and retry-execution resumes the same action, never accepts again. A known task ID is always returned even after the task is terminal. An uncertain commit is reconciled from action ID; never blindly replay enqueue.

The acceptance action stores the authorized assignee type/id, resolved squad leader, project/resource selection and relevant execution context fingerprint. An execution attempt compares the persisted context before external preparation and inside the enqueue transaction; a changed assignee/context is a conflict, not permission to run the replacement. The user must make a fresh explicit normal execution decision. Queue actor provenance remains the original human authorizer, rechecked for current membership and invoke permission.

All fresh enqueue, merge/planned-input, retry, claim and start paths must check formal admission from authoritative DB state. Handler checks supply understandable 409 errors, while shared service/SQL guards protect non-HTTP callers. Merely checking a supplied stale Issue struct is insufficient. A database task-owner fence may reinforce the existing canonical lock function, but does not replace application guards. Generic issue status/assignment/batch/subissue/quick-action/lifecycle requests cannot mutate nonformal execution state; safe human content/comments remain usable and mentions stay text without dispatch. Agent/tool content mutations remain subject to existing actor authorization. Existing formal workflows remain covered by regression controls.

Pending-era comments retain a persisted no-dispatch decision, or an equivalent authoritative admission-at-comment-time fence. Delayed event delivery after acceptance must not turn a prior text mention into new work. Guard both original dispatch and retries/merges/planned-input recovery; checking only the current accepted state is insufficient.

## Queries, events and compatibility

Ordinary lists, grouped/table/facet queries, search defaults, children, dashboard/member/agent aggregates and project totals exclude nonformal issues. Explicit triage/history queries expose them under current workspace membership. Detail links remain readable with an admission explanation; duplicate target selection searches only formal same-workspace tasks including closed tasks. Project statistics keep `(done + cancelled) / formal total` unchanged for existing workspaces.

Dedicated triage events invalidate workspace settings/queue/count/detail/history keys and affected ordinary task/project projections. New clients also refresh due queues every <=30 seconds. Read-time timestamp predicates make snoozed tasks recover after server restart without requiring a timer to mutate business state. Due reminders use a once-per-round/snooze outbox identity. Notification archive/read does not touch triage.

GET triage/settings proves server support (`supported:true`). New frontend treats endpoint 404 as unsupported; actual network/permission failures remain errors, not disabled. Navigation is shown only when enabled, while direct history access remains available after disabling. Installed old clients may read existing formal tasks normally; server refuses nonformal status/dispatch writes with an intelligible reason and issue link. New direct IssueDetail suppresses nonformal execution controls and routes to review. No mobile UI is required in T1; server protections are mandatory.

## HTTP contract

All paths below are workspace-scoped under existing `/api` authentication and `X-Workspace-ID`; handlers recheck human actor for review/config/import, so machine credentials cannot approve via a runtime owner's user ID. Machine explicit intake uses the existing actor principal and never grants review authority.

Human-only decisions check both `isMachineCredentialActor` and `resolveActor`: legacy authenticated X-Agent-ID/X-Task-ID agent actors are also denied even when their credential class is a human PAT. Current membership/role is revalidated inside write transactions.

Canonical contract is `api-contract.md`. HTTP wire names are snake_case. Responses use the existing IssueResponse. Mutation malformed 2xx is an error in the client. Request UUIDs are generated once per user intention and preserved on retry. Permissions are 403, missing scoped entity 404, validation 400, revision/idempotency/disabled conflicts 409; infrastructure failure remains 5xx. Batch results classify failures per row.

## CSV and batch semantics

Use Go encoding/csv with strict valid UTF-8/BOM support; no GBK, new dependency or client-side comma splitting. Initial limits are 5 MiB and 1,000 data rows, title 500 Unicode code points, description 1 MiB and external ID 1,000 UTF-8 bytes, benchmark before release and document observed baseline; oversize/invalid encoding rejects the whole upload. Required title and existing text/date constraints are validated before preview. Header auto-map accepts supported English/Chinese names, and the mapping UI can adjust or ignore each column. Reject empty or duplicate trimmed headers and multiple mappings to one non-ignore field; retain distinct source headers for explicit mapping. State/iteration/attachments are never adopted; expose warnings.

Resolve candidates in the actor's workspace (project/label by unique name, member by email, agent by name); missing/ambiguous/foreign values are warnings with cleared fields. No auto-created project or label. External IDs default skip across workspace/batch, with explicit per-row import-anyway override; this cannot use an absolute external-ID unique constraint. Serialize external-ID resolution/commit with a scoped advisory key. Similar title only yields a candidate warning. Commit receives selected row numbers and duplicate overrides; errors cannot be forced through. Each row is atomic and results durable, so retry processes only unsuccessful rows. Download failures with original mapped values and errors using real CSV escaping. Summary notifications are batch-level and stable on retry.

Batch review first previews exact selected pending IDs/revisions and then commits only explicitly selected valid items. No implicit select-all, bulk duplicate or bulk execute. Keep successes, conflicts, invalid items and permission changes visible; retries retain only unfinished row keys.

Both batch preview and batch commit enforce the server-side action allowlist accept/reject/snooze/assign_reviewer; a forged direct commit cannot request accept_and_execute, duplicate, unsnooze or reopen. Global history is action/round based (not a current-item list) and includes stable CSV batch summary entries. Reopening or changing current content does not remove past results or replace their historical field snapshots.

## Shared interface

Extend Multica's existing UI visual system (Operate mode): sidebar entry with global actionable count, queue filters above list, stable list/detail split, embedded IssueDetail for existing comments and attachments, action controls and immutable history alongside it. Compact view navigates list/detail with preserved selection/filter/scroll. Reuse Base UI components, typography and semantic tokens; no new visual identity. Settings uses existing SettingsTab/Section/Row layout. Create/accept forms reuse draft pickers and editor without create-mode or run-preview side effects.

J/K, 1/2/3, H, C work only with queue focus and no editable target or portal. Every action remains clickable/labeled. Refocus the next visible row after successful decision, or the correct empty-state heading. Errors/conflicts preserve drafts. CSV flow is file -> column mapping -> row preview/selection -> confirmed results/retry; progress is shown for slow requests. Localize English and Simplified Chinese.

## Verification and rollout

The task acceptance ledger retains all 37 T1 acceptance cases. Add failing tests before behavior changes. Test actual PostgreSQL transactions, sibling-action races, stale membership, settings/intake race, import retry and terminal-task retry. Include formal-workflow controls. Fixtures/fake runtime prove enqueue without running installed agent CLIs.

Run targeted tests, package lint/typecheck, Go vet, required full checks, then production Web + shared Desktop route flows, screenshot/visual verdict and responsive keyboard audit. Record representative CSV/list baseline and lock limits. Document defaults, new API/CLI error behavior, migration order, no-loss downgrade refusal and feature disable procedure. No production deployment, release or merge is included in this request.
