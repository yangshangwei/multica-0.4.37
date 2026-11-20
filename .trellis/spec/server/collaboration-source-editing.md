# Source collaboration views and daily editing

## 1. Scope / Trigger

Apply to mixed ordinary/collaboration source issue reads, source UI actions,
creation fields, source structure and iteration membership, and human comment
editing/resolution/reactions. These contracts extend the account workbench;
they do not open the default creation/execution gates or complete P7/P8.

## 2. Signatures

- Mixed issue DTOs have optional `collaboration: {creator, handler,
  has_external_collaborators}`. Both accounts are minimal `AccountSummary`.
  `external_collaborators=true` filters list/open/grouped requests; table queries
  use the corresponding boolean filter before pagination and counts.
- `POST /api/collaboration/issues` accepts initial nonterminal `status`,
  `priority`, dates, `label_ids`, `properties`, `project_id`, `parent_issue_id`,
  `stage`, `current_iteration_id` and its `expected_iteration_revision`.
- `GET .../creation-options?source_workspace_id=...` and
  `GET .../issues/{id}/field-options` accept optional `parent_search` (200 Unicode
  characters). Parent references return at most 200 rows and `parents_truncated`.
- `PATCH .../issues/{id}` retains its metadata/status split. Source structure
  joins the metadata operation; it is not a generic issue write.
- `PUT .../issues/{id}/iteration` carries the ordinary issue `expected_revision`
  as a decimal string, collaboration write context, `current_iteration_id`,
  optional target `expected_iteration_revision`, `iteration_reason` and explicit
  `allow_completed` where the ordinary membership contract requires it.
- Comment PATCH/DELETE retain `expected_comment_revision`. Dedicated POST
  `.../comments/{commentId}/resolve|unresolve`, POST/DELETE
  `.../comments/{commentId}/reactions/{emoji}` and `.../reactions/{emoji}` use
  `Idempotency-Key`; GET `.../reactions` returns private aggregate counts.
- GET `.../discussion/operations/{requestId}` recovers the caller's original
  discussion receipt. It includes issue/comment/request/action, `emoji` (null
  for resolution), status/code and protocol version 2.

## 3. Contracts

The server derives mixed-source identity from `issue_collaboration` and fails
unavailable when relationship/required-account lookup fails. A present malformed
marker must not become an ordinary editable issue. Absent/null remains the
approved ordinary/older-server contract; preserve an already-known collaboration
identity across partial cache responses. Never probe every list row or invent
source membership for an external assignee.

All source detail mounts, including previews and native windows, classify before
mounting ordinary detail hooks. The source bridge reuses `CollaborationDetail`.
Cross-assignee moves open transfer; terminal moves open lifecycle; metadata and
nonterminal status edits use shared original-key workflows. Column-local layout
reorders send only `position` and the ordinary revision, with no collaboration
revision/history/notification side effect. Ordinary issue interactions remain
unchanged.

Source structure requires both current creator/handler authority and a source
member row actually held by the transaction. Ancestor validation rejects cycles
and uses the existing source/I1 fence and retry behavior. A combined metadata
edit advances the ordinary issue revision once. Iteration membership borrows the
existing Prepare/Commit transaction, versions and reason checks; it does not
create a parallel membership owner. Initial fields, first membership and private
draft consumption commit with the same created issue and original receipt.
Ordinary workspace iteration previews/operations reject explicit `move` batches
containing a collaboration issue at the shared lifecycle preparation boundary.
This rejects the whole mixed batch before reference capture or membership writes;
server-owned period close, handoff and rollover keep their existing behavior.

Outside editors receive minimal unopenable project/parent/iteration references,
never source catalogs. Label/property values use issue-scoped definitions without
definition-management rights. Actor properties remain member-only; exact account
resolution and explicit selection do not grant workspace access or AI authority.

Source-parent options reuse `ListCollaborationSourceParents`: empty search returns
the latest eligible rows by number descending, with a 201-row read projected as
200 rows plus `parents_truncated`. Keep current workspace/self/admission filters
and existing terminal/collaboration-parent eligibility. Title substrings, exact
bare numbers and UUIDs remain supported. Complete displayed identifiers match
case-insensitively using the same `util.ResolveIssuePrefix(ws.IssuePrefix, ws.Name)`
as display formatting, supplied as `identifier_prefix`. Do not strip an arbitrary
prefix or turn a full identifier into numeric title matching. Numeric and legacy
prefixes follow that resolver. The shared
[source picker](../views/frontend/collaboration-source-picker.md) owns visible
browsing and pending/error/retry feedback.

Only an authorized ordinary human comment's author can edit/delete it. Replies,
history, tombstones, delivered files and frozen AI inputs retain their identities.
The sibling comment list and composer use distinct `comments:`/`composer:` keys
per issue. Reusing the bare issue ID for both leaves duplicate DOM across refreshes;
keep the composed-detail regression as well as isolated list tests.
Thread resolution and own reactions use dedicated SQL without advancing content
revision/updated_at or sending notifications/AI inputs. Ordinary reaction routes
must not bypass that service.

New comment edits validate 65,536 UTF-8 bytes before persisting an outgoing
request. Comment PATCH alone permits a 512 KiB JSON envelope to carry legal
escaped content and recover old oversized drafts; other workflows remain 64 KiB.
An old oversized edit can persist a content-free refusal. Only a correlated
HTTP 200 own-result response retires that original key; local validation, 202 or
missing results do not prove its outcome. Preserve the user's text and explicit
version rebase after a confirmed conflict. Reaction receipts must echo the exact
emoji; request reuse with another target/action/emoji refuses.

Mixed issue schemas, query strings, error logs and JSON syntax failures are
redacted. A source workspace receives only
`{"type":"collaboration:workbench_changed","payload":{}}`, through the retained
outbox with stable `:source` identity. It receives no issue ID, author, body or
execution metadata. Normal scope authorization still applies. Client handlers
accept only the empty payload and invalidate current authorized source queries;
they never copy an account DTO into an ordinary issue cache or trigger writes.

## 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| External handler requests source catalogs/structure | No catalog; source write denied |
| Source membership changes during lock acquisition | Reject without relying on an unlocked late membership |
| Parent cycle or invalid source reference | Atomic refusal; other field changes roll back |
| Stale iteration/issue/collaboration version | Original refusal preserved; input requires explicit rebase |
| Move/remove an existing iteration without a reason | Refuse without changing membership |
| Ordinary iteration `move` contains a collaboration | Reject the whole batch; use its dedicated workflow |
| Done issue enters active iteration | Require ordinary `allow_completed` confirmation |
| Comment edit exceeds UTF-8 content limit | Keep text; new request is not persisted/sent |
| Lost original oversized edit response | Exact retry/read settles the persisted refusal |
| Reaction receipt omits or changes emoji | Keep original request pending |
| Recovered old-page comment enters the loaded page | Same component/editor identity and input survive |
| Source relay fails after account delivery | Event stays pending and reuses delivery IDs |
| Malformed mixed-source response | No private payload/validation details in logs |

## 5. Good / Base / Bad Cases

- Good: an external handler updates labels, while the source creator changes
  iteration with a reason; both see the same UUID and current source lists.
- Base: ordinary issue editing, table controls and source layout reordering
  retain their existing semantics.
- Bad: treating a shared card drag as generic assignment, or treating an absent
  operation result as permission to discard the original request.

## 6. Tests Required

Source handler tests own projection/count/child and machine boundaries. Service
source-field tests own authority, atomicity, cycles, iteration and creation;
discussion tests own revisions, notifications, original receipts and rollback.
HTTP comment-limit tests cover encoded envelopes separately from UTF-8 content.
Core source/redaction, field-action, comment-action, discussion and invalidation
tests own wire and recovery matrices. Views test shared interactions and paging
identity. `e2e/collaboration-editing-web.spec.ts` and the dedicated editing scenario
in `e2e/collaboration-account-desktop.spec.ts` exercise actual client wiring.

`TestCollaborationSourceParentSearchMatchesDisplayedIdentifiers` owns full/case-
insensitive identifiers, actual numeric/legacy prefixes, wrong-prefix handling,
title/bare-number/UUID preservation and source/admission isolation.
`TestCollaborationSourceParentSearchFindsIdentifierBeyondRecentWindow` pins the
recent-200 boundary and a successful exact lookup of an older task.

## 7. Wrong vs Correct

Wrong: optimistic generic source writes, source-directory queries from an external
field editor, or a local pending-key deletion after a byte-limit error.

Correct: current object/source authority, original-key shared operations, private
reference projections, and an exact persisted result before retiring a request.

## Deterministic source authority and external iteration evidence

`collaboration_source_authority_race_test.go` holds the real leading writer at
its transaction commit and observes the follower with `pg_blocking_pids` before
release. Run both orderings for project/parent and iteration changes versus
handler transfer and source-member departure. A departed or former noncreator
handler cannot use stale source authority; a winning source change commits once
before the competing writer. Target iteration metadata revision is an additional
fence. Refused writes leave issue fields/membership/history untouched, while
legitimate human changes preserve the retained execution rows. Do not replace
these writer barriers with sleeps or a generic comment-permission assertion.

`TestCollaborationExternalHandlerIterationStatisticsPreviewAndHistory` uses
the ordinary source iteration HTTP start/end/rollover owners with an external
collaboration assignee. The source caller can use complete statistics, preview
and frozen history; the external handler still receives no source catalogs or
maintenance authority. Responses retain assignee ID/type without email, username
or private parent body. A missing source member join may yield an unavailable
assignee name; this test does not establish the separate external display-name
UI contract. Source structure and iteration membership remain separate writes.

These tests prove database authority, version and projection boundaries. Full
source-role/reason/request-separation acceptance additionally uses
`TestCollaborationSourceStructureAndIterationRejectNonManagers` for source-only
member/owner, other full participants and external/departed managers. Each actual
structure/iteration branch refuses with an immutable own receipt and unchanged
source/business/execution snapshots; read references expose no unselected catalog.
`TestCollaborationIterationRemovalRequiresReasonAndOriginalRefusal` verifies
missing and whitespace-only removal reasons for creator and current source
handler, then one fresh explicit reasoned removal. Old refused keys never
recompute after it. `TestCollaborationIterationPUTRejectsMetadataBeforeEffects`
requires strict rejection of title/project/status/stage mixed into the dedicated
PUT before effects, then a legal separate membership write and identical replay.
These direct tests supplement the real writer races; generic metadata capability
or ordinary iteration `move` assertions do not prove these dedicated branches.

## Ordinary children and recorded executions

The R51/R52 increment now supplies the shared ordinary-child projection,
content-free child refresh hints, retained run-record drilldowns and compatible
result links. See [the executable contract](collaboration-records.md) for the
current source/open authority, original ordinary writer ownership and record
validation. This does not establish the reverse R58 parent-wakeup direction or
the entire P8 compatibility matrix.

## Current content contention and resolution acceptance

`TestCollaborationCommentEditDeleteBothCommitOrdersRetainHistory` uses public
EditComment/DeleteComment with an actual PostgreSQL commit barrier and blocked
writer observation. Edit-first preserves its revision-2 success and original
stale-delete refusal; an explicitly fresh delete makes the final tombstone.
Delete-first preserves its success and refuses both the old edit and every
fresh resurrection attempt. Compare exact persisted receipt JSON, rather than
nil/allocated-empty slice identity when an optional empty attachment list has
the same `omitempty` wire. Both orders retain create/edit/delete actor/time,
reply anchor, delivered private attachment bytes, and frozen AI input/group/run
state. Do not use sleep or replace a real writer with a fabricated response.

`TestCollaborationDiscussionResolveUnresolveUsesCurrentReadBasis` verifies both
an external participant with no source membership and a source member with no
participation. Resolve/unresolve alter only the defined resolution metadata and
own exact receipts/events. They do not change content revision/history, AI,
collaboration notifications or source inbox. Old resolve/unresolve keys never
recompute after a later opposite action; lost read/comment authority refuses a
new mutation while own content-free recovery stays available. Both positive
paths must run; a successful resolve plus a forbidden unresolve is not complete
thread-operation evidence.

### Human edits during run-bound replies

`TestCollaborationHumanEditRuntimeReplyBothCommitOrders` composes the author's
`EditComment` with an admitted run's `CreateRuntimeComment` replying to that
human comment. These are distinct content identities, so both writes succeed:
the current human body becomes V2/revision2 and the Agent reply keeps its own
comment, parent, Agent and run identity. The already-dispatched command, group
input, prompt and typed test subject retain V1 and their original candidate.
Current discussion and frozen execution input intentionally show different
versions; a display edit does not amend the admitted execution or dispatch again.

Use `runtimeSourceCommitStarter` around the existing commit barrier so the held
transaction actually owns the source locks. Observe `pg_blocking_pids` before
release in both orders and join both writers before fixture cleanup. Preserve
original rows and explicitly account for new events and session sequence;
complete replay snapshots must compare without removing changing business fields.

Human create/edit replay returns the original content-free receipt, even when
the current comment has a later revision. Runtime comment replay instead requires
live business authority before reading its original receipt; verify it before
settlement. Original report ACK recovery has its own post-settlement contract.
Do not infer post-settlement comment-write permission from report recovery.
These service tests use synthetic PR/CI and managed preparation/settlement facts;
they do not prove remote Git, daemon execution, model quality or ordinary-comment
administrator permissions.


## Current source surface and phase evidence

`e2e/collaboration-source-surface.spec.ts` uses production Web and actual Electron
with the same source UUID to verify list/board/Gantt/comment external identity,
external filtering and immutable cross-assignee/terminal drag refusal. A valid
nonterminal drop enters the shared versioned workflow; ordinary cards keep their
canonical `POST /api/issues/:id/move` with adjacent anchors. Column-local shared
reorder sends only `position` and ordinary `expected_revision`; task-built
`issue reorder --before` persists with no collaboration revision, history or
notification change. Preserve exact event business fields while asynchronous
`outbox_state` delivery is observed separately.

`TestCollaborationSourceManagerStageOnlyEditRecordsOnce` pins creator and current
source-handler stage-only1→2, exact original receipt and actor/request/before/after
history. `TestCollaborationSourceStageTransferBothCommitOrders` and
`TestCollaborationSourceStageDepartureBothCommitOrders` use real commit/PID lock
barriers to reject old roles and source membership in both orders; fresh old-role
keys remain denied. Ordinary best-effort parent stage wakeup remains its existing
handler owner and is not a durable delivery contract.

## Enforced project association changes

The later [transactional completion contract](./delivery-completion-enforcement.md)
adds explicit authorizations and current checks in actual writers. The historical
observe/hint increment above remains non-authorizing; its private evidence and
original actor/retention boundaries remain. Current source-policy enforcement,
project-change rules and distinct R38 handling follow that shared contract.

## Priority/handler conflicts and original receipts

`TestCollaborationRecoveryPriorityConcurrentIdentity` owns priority-only
same-key/same-payload convergence, changed-priority idempotency refusal and
different-key/same-revision competition. Inspect the winning priority, exact
ordinary/collaboration revisions, event identity and persisted operation hash/JSON;
the existing title race alone does not identify a priority winner.

`TestCollaborationRecoveryPriorityTransferCommitOrders` uses the existing real
commit/PID barriers in both orders. Current authority precedes revision: a former
noncreator handler's priority edit after transfer is forbidden403, while an old
revision transfer after a priority commit is stale409. After losing write authority,
recover the actor's content-free original result with `GetWorkflowOperation`;
reposting an old key does not restore authority. Fresh authorized continuation
must preserve earlier success/refusal receipts and retained execution identities.

`TestCollaborationOrdinaryChildPriorityAssigneeCASCommitOrders` owns the ordinary
linked-child field pair: numeric `expected_revision`, one200/one409 with exact
`revision_conflict`, `resource_type=issue`, child `resource_id`, and numeric
expected/actual revisions. Only the winning field changes; a deliberate fresh
request applies the other field once. Ordinary omitted-version compatibility
remains unchanged. Priority/assignee-only edits emit no parent-reference hint:
that publisher is selected only for parent, title or admission changes. Keep the
exact committed ordinary update events and unchanged parent collaboration state.

These Go service/HTTP regressions use legacy-auth JWT fixtures and actual
PostgreSQL writer waits. They do not replace current password/API/Web/Desktop
composition or the remaining S3 recovery/notification/upload/blocking matrix.

The separately accepted current-password R06 composition now verifies genuine
Web/Electron priority/transfer requests in both submission orders, exact
original GET results, deliberate committed-response loss and a fresh authorized
UI continuation. Ordinary child numeric CAS is explicit API evidence; its
unmodified picker writes still omit the optional version and are recorded as
compatibility evidence. Earlier service/PID-lock tests and these current
submission-order journeys keep their distinct denominators. See
[recovery QA](../web/frontend/collaboration-recovery-qa.md) for actual native
transport, UI pending-intent retirement and retained-failure contracts. Complete
S3 and parent TC-16 are not accepted by this bounded field composition.
