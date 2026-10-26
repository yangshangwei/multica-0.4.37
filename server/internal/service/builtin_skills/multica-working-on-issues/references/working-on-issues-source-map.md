# working-on-issues source map

Evidence layer for `SKILL.md`. Every contract the skill states is traced to a
source file and symbol here. Historical PR/status rows retain their original
`feat/builtin-skills` line citations and drift notes; re-confirm an exact line
before using it. Triage references use symbols so integration edits do not turn
line movement into a misleading contract.

## Triage admission and human decisions

Collaboration overrides for protected objects:

| Source/editing behavior | Source |
| --- | --- |
| Complete ordinary-child references and source-only open authority; ordinary writes publish issue-scoped postcommit hints without collaboration mutation | `server/internal/handler/collaboration.go`; `server/pkg/db/queries/collaboration_source.sql`; `server/internal/service/collaboration_children.go`; `server/internal/handler/collaboration_children_test.go` |
| Source-owned paged records preserve exact run prompts, nullable usage, configured provenance and current issue access after engine cleanup | `server/internal/service/collaboration_records.go`; `server/internal/handler/collaboration_records.go`; `server/pkg/db/queries/collaboration_records.sql`; `server/internal/service/collaboration_records_test.go` |
| Explicit bounded runtime records remain separate from sealed proof; matching squad activity can receive one immutable telemetry attachment | `server/pkg/protocol/collaboration_record.go`; `server/internal/daemon/collaborationrun/runner.go`; `server/internal/service/collaboration_runtime_capabilities.go` |
| Exactly one terminal group event freezes current participant recipients and final-leader run anchor; AI results never impersonate a human operator | `server/internal/service/collaboration_result_notifications.go`; `collaboration_groups.go`; `collaboration_result_notifications_test.go`; `server/cmd/server/collaboration_records_routes_test.go` |
| Server-derived source marker, external filter/counts, safe machine child projection and shared detail controls | `server/internal/handler/collaboration_source.go`; `server/pkg/db/queries/collaboration_source.sql`; `packages/views/collaboration/source-bridge.tsx` |
| Initial fields and source structure/iteration use existing transactions, held membership, versions and explicit reasons | `server/internal/service/collaboration_creation_fields.go`; `collaboration_source_fields.go`; `collaboration_iteration.go`; `packages/views/collaboration/workflow.tsx` |
| Ordinary iteration move preview/apply rejects shared or mixed selections before reference capture; automatic lifecycle operations remain available | `server/internal/service/iteration_lifecycle_preview.go` (`prepareLifecycle`) |
| Discussion changes preserve content versions, replies and original emoji receipts; UTF-8 refusals recover without local key deletion | `server/internal/service/collaboration_discussion.go`; `collaboration_comments_edit.go`; `server/internal/handler/collaboration_comment_limits_test.go`; `packages/core/collaboration/comment-actions.ts`; `discussion.ts` |
| Source ticks carry no private identifiers/content and only invalidate authorized caches; mixed source logs are redacted | `server/internal/service/collaboration_realtime.go`; `packages/core/collaboration/invalidation.ts`; `packages/core/api/issue-source-redaction.test.ts` |

| Behavior | Source |
|---|---|
| Legacy issue mutations and private machine detail access are refused for collaboration; position-only updates use a separate checked path | `server/internal/handler/handler.go` (`loadIssueForUser`, `loadIssueForUserWithCollaborationUpdate`); `server/internal/handler/collaboration_boundary.go` |
| Mixed update/delete batches refuse before mutation/cancellation; legacy comment writes and rerun cannot bypass the collaboration service | `server/internal/handler/issue.go` (`BatchUpdateIssues`, `BatchDeleteIssues`); `server/internal/handler/comment.go`; `server/internal/handler/task_lifecycle.go` (`RerunIssue` using the guarded loader) |
| Ordinary enqueue/claim/retry/start reject collaboration sources and shared trigger/coalesced content; ordinary children and pure parent references retain their own execution contract | `server/internal/service/issue_admission.go` (`checkOrdinaryExecutionSource`, `checkIssueExecution`); `server/internal/service/issue_task_transaction.go` (`EnqueuePreparedIssueTaskInTx`); `server/migrations/650_collaboration_legacy_execution_fence.up.sql` (`lock_issue_execution`); `server/internal/service/legacy_execution_boundary_test.go` |
| Claim finalization revalidates persisted source, runtime and content provenance before minting credentials | `server/internal/service/task.go` (`FinalizeTaskClaim`); `server/internal/service/legacy_execution_boundary_test.go` |
| Negotiated v6 reads preserve fixed modes, independent outcome/termination/cleanup and complete input-reader history; v5 host graphs are unavailable without losing human access, and host AI writes require explicit v6 comprehension | `server/internal/handler/collaboration_execution_version.go`; `server/internal/service/collaboration_execution_view.go` (`ExecutionView`); `server/internal/service/collaboration_host_compatibility.go`; `packages/core/collaboration/execution-schema.ts` |
| Host execution reuses ordinary Provider backends with per-launch permission, exact nonce/receipt recovery, independent sessions and no OS sandbox claim; Windows wrappers require their managed Job lifecycle | `server/internal/daemon/collaborationrun/host_runner.go`; `server/internal/daemon/collaborationrun/host_ledger.go`; `server/pkg/agent/launch_lifecycle.go`; `server/pkg/agent/proc_windows.go`; `server/internal/service/collaboration_host_runtime.go` |
| A stored original outcome can recover its ACK after settlement/revocation without a new business write; pending recording never blocks reliable managed termination | `server/internal/service/collaboration_runtime_capabilities.go` (`RecordRuntimeOutcome`); `server/internal/service/collaboration_outcome_receipt_test.go`; `server/internal/daemon/collaborationrun/host_runner.go` |
| Target holds and channel pause have independent versions; target Resume is atomic, issue Resume has separate channel/hold decisions, and original-key control recovery never turns an old refusal into a future intention | `server/internal/service/collaboration_control.go`; `server/internal/service/collaboration_queue.go` (`GetControlOperation`); `server/internal/handler/collaboration_control.go`; `packages/core/collaboration/control-schema.ts`; `packages/core/collaboration/control.ts` |
| Runtime teardown and binding replacement save target holds and exact dependent-group stops in the resource transaction; unknown commit is not retried and historical termination proof remains available | `server/internal/service/collaboration_resource_invalidation.go`; `server/internal/service/runtime_teardown.go` (`WithRuntimeResourceTransaction`); `server/internal/service/managed_installation.go`; `server/internal/service/collaboration_resource_invalidation_test.go` |
| Channel FIFO skips held/invalid input; root return requires whole-group nonlaunch, while M11 return checks every immutable prompt reader and retains original history | `server/internal/service/collaboration_batches.go`; `server/internal/service/collaboration_groups.go`; `server/internal/service/collaboration_input_readers.go`; `server/migrations/724_collaboration_input_readers.up.sql` |
| Final leader result follows an immutable group-allocated turn; ambiguous active legacy order refuses upgrade and settled history is preserved | `server/migrations/725_collaboration_leader_turn.up.sql`; `server/migrations/726_collaboration_leader_turn_identity.up.sql`; `server/pkg/db/queries/collaboration_runtime.sql`; `server/internal/service/collaboration_squad_order_test.go` |
| Existing issue get/comment list/add and squad activity use a complete narrow run identity; writes require their original request UUID and unsupported operations never fall back to human profiles | `server/cmd/multica/cmd_collaboration.go`; `server/cmd/multica/cmd_agent.go`; `server/internal/cli/collaboration.go`; `server/cmd/multica/cmd_collaboration_test.go` |
| Shared-context show/update use the same narrow identity and allowlist; update needs a canonical base version and request UUID, refuses local file links and external files before HTTP, and each refusal names its next action | `server/cmd/multica/cmd_collaboration_memory.go`; `server/cmd/multica/local_path_link_lint.go` (`rejectLocalPathLinks`); `server/internal/cli/collaboration_memory.go`; `server/cmd/multica/cmd_collaboration_memory_test.go` |
| Host claims declare `memory_version`; the base is read after the permit and enters only the prompt, a fresh-session retry reads it again, and a failed read never launches the provider | `server/internal/daemon/collaborationrun/host_runner.go`; `server/internal/daemon/collaborationrun/host_memory.go`; `server/internal/daemon/collaborationrun/host_memory_test.go` |
| Runtime memory routes share the comment gate plus `memory_mode`; AI writes are redacted before the byte limit and `(run_id, request_id)` replay comparison; the base is frozen on first read | `server/internal/service/collaboration_memory_runtime.go`; `server/internal/handler/collaboration_memory_runtime.go`; `server/pkg/db/queries/collaboration_memory.sql` |
| Optional absolute cli_executable is verified and mounted read-only; five business identity variables reach the provider without managed/termination credentials | `server/internal/daemon/collaboration.go`; `server/internal/daemon/collaborationrun/controller_linux.go`; `server/internal/cli/collaboration.go` |
| Actual daemon machine-slot polling, registered credentials, contained CLI and separate leader/member/continuation proofs work at capacity one | `server/cmd/server/collaboration_daemon_cli_routes_test.go`; `server/internal/daemon/collaboration_router_test.go` |
| Cancellation/Switch original-key recovery emits exactly seven content-free fields after access loss; fresh denial clears private cache, while missing/malformed results keep the original key pending | `server/internal/handler/collaboration_commands.go`; `server/internal/handler/collaboration_switch.go`; `packages/core/collaboration/command.ts`; `packages/core/collaboration/switch.ts` |
| Execution configuration and relevant invoke/member/skill-binding changes persist invalidation in the writer transaction; background convergence preserves valid newer consent and does not pause unrelated channel work | `server/migrations/704_collaboration_config_invalidation.up.sql`; `server/internal/service/collaboration_config_invalidation.go`; `server/internal/handler/agent.go`; `server/internal/handler/collaboration_config_invalidation_test.go` |
| Verified credential class and separate membership/participation control authority; reorder does not advance collaboration revision | `server/internal/auth/principal.go`; `server/internal/collaboration/access.go`; `server/internal/service/collaboration.go` (`Reorder`) |
| Human edits and directed handoffs reuse original versions/keys; stale status confirmations stay refused. A committed collaboration child close can wake its ordinary parent through the existing stage barrier | `server/internal/service/collaboration_edit.go`; `server/internal/service/collaboration_handoff.go`; `server/internal/service/collaboration_lifecycle.go`; `server/internal/handler/collaboration_lifecycle.go` |
| Public human creation is registered behind the default-disabled gate, with atomic attachment binding and original-key recovery | `server/cmd/server/router.go`; `server/internal/handler/collaboration_creation.go`; `server/internal/service/collaboration.go` (`CreateInTx`); `server/migrations/612_collaboration_default_gate.up.sql` |
| Authorized creation refusals persist outside the rolled-back creation savepoint; a correlated own-result HTTP 200 permits explicit new submission while old keys stay sealed | `server/internal/service/collaboration_creation_result.go`; `server/internal/service/collaboration.go`; `server/internal/handler/collaboration_creation.go`; `packages/core/collaboration/creation.ts` |
| Private draft objects are encrypted; old public/signed file paths refuse their namespace; native byte capabilities recheck current authority | `server/internal/service/collaboration_attachments.go`; `server/internal/service/collaboration_attachment_access.go`; `server/internal/handler/collaboration_attachments.go`; `server/internal/handler/collaboration_attachment_capability.go`; `server/internal/storage/collaboration_private.go` |
| Comment display removal preserves delivered issue files and immutable command attachment references | `server/internal/service/collaboration_comments_create.go`; `server/internal/service/collaboration_comments_edit.go`; `server/internal/service/collaboration_batches.go`; `server/pkg/db/queries/collaboration_attachments.sql` |
| Account/issue delivery preserves credential class, rechecks at final send and uses retained invalidation facts; reconnect performs reads only | `server/cmd/server/collaboration_scope_authorizer.go`; `server/internal/realtime/collaboration.go`; `server/internal/service/collaboration_realtime.go`; `packages/core/realtime/collaboration.ts` |
| Resource and disabled-participant management use scoped summaries and owner-only current installation identity | `server/internal/service/collaboration_resources_view.go`; `server/internal/service/collaboration_resource_options.go`; `server/internal/handler/collaboration_resources.go`; `packages/views/collaboration/resource-panel.tsx` |
| Boundary regressions and ordinary-task controls | `server/internal/handler/collaboration_boundary_test.go`; `server/internal/handler/issue_batch_test.go` |

| Behavior | Source |
|---|---|
| Only `not_required` and `accepted` are formal; unknown values fail closed; execution checks read persisted state | `server/internal/admission/admission.go` (`Formal`, `Check`); `server/internal/service/issue_admission.go` (`checkIssueExecution`) |
| Explicit intake defaults to pending/backlog; candidates remain separate from formal project/assignee; agent intake retains Contributor authority | `server/internal/handler/triage.go` (`CreateTriageItem`, `createTriageItemInTx`) |
| Human review verifies credential class and resolved actor; settings require owner/admin; workspace membership rechecked under write fences | `server/internal/handler/triage.go` (`triageHumanActor`, `beginTriageWrite`, `UpdateTriageSettings`) |
| Triage defaults to enabled for missing/new settings; saved choices and explicit administrator disable are preserved | `server/internal/handler/triage.go` (`GetTriageSettings`, `UpdateTriageSettings`), `server/migrations/567_workspace_feature_defaults.up.sql` |
| Candidate visibility does not grant invocation; acceptance validates current project/assignee authority | `server/internal/handler/triage_actions.go` (`triageValidateCandidates`, `triageValidateReferences`, `applyTriageAcceptance`) |
| Generic protected writes return `triage_review_required` with issue identity/path; status, assignment, project, parent, stage and position do not grant admission | `server/internal/handler/issue_admission.go` (`writeIssueAdmissionError`, `issueAdmissionMutation`); `server/internal/handler/issue.go` (`UpdateIssue`, `BatchUpdateIssues`) |
| Plain acceptance commits fields/audit without enqueue; explicit acceptance/execution has its own action and original-human retry | `server/internal/handler/triage_actions.go` (`actOnTriageItem`, `retryTriageExecution`) |
| Saved execution context binds assignee, squad leader, runtime and project/resource/content inputs; current permission/context is checked before preparation and again before task insertion | `server/internal/handler/triage_actions.go` (`triageExecutionSnapshot`, `retryTriageExecution`) |
| Task identity and action result commit together; a known task is returned on retry and uncertain commit is reconciled | `server/internal/handler/triage_actions.go` (`retryTriageExecution`); `server/internal/service/issue_task_transaction.go` (`EnqueuePreparedIssueTaskInTx`) |
| Pending-era comments retain no-dispatch eligibility across later admission, including trigger/coalesced recovery | `server/migrations/535_triage_execution_fence.up.sql`; `server/pkg/db/queries/comment.sql`; `server/internal/service/issue_admission.go` (`checkIssueExecution`); `server/internal/handler/comment.go` (`suppress_execution` event field) |
| Lifecycle, direct enqueue/retry, claim and start cannot bypass persisted admission | `server/internal/handler/lifecycle_handoff_transaction.go`; `server/internal/handler/daemon.go`; `server/internal/service/task.go`; `server/internal/service/issue_task_transaction.go`; `server/internal/handler/triage_boundary_test.go`; `server/internal/service/triage_admission_test.go` |
| Notifications deliver independently of review; closing the feature requires no pending rows and preserves history | `server/internal/handler/triage_notifications.go` (`queueTriageNotification`, `deliverTriageNotificationsOnce`); `server/internal/handler/triage.go` (`UpdateTriageSettings`); `server/internal/handler/triage_queries.go` (`GetTriageHistory`) |
| Deleted manual/action results keep consumed request identities; successful CSV row replay returns the original identity | `server/internal/handler/triage.go` (`CreateTriageItem`); `server/internal/handler/triage_actions.go` (`actOnTriageItem`); `server/internal/handler/triage_import.go` (`commitTriageImportRow`); `server/pkg/db/queries/triage.sql` |
| Triage APIs are workspace-scoped; no triage CLI subcommands are registered | `server/cmd/server/router.go` (`/api/triage` route group); `server/cmd/multica/` (command registration) |

These admission checks qualify the normal status and sub-issue side effects
below. In particular, a triage batch's `accept` action cannot use a `todo`
assignee as an implicit execution trigger. The operator contract and rollback
rules are in `docs/triage.zh-CN.md` and `.trellis/spec/server/triage.md`.

## `multica issue pull-requests` — read PR links from Multica

| Behavior | File:line | Drifted from |
|---|---|---|
| CLI command `pull-requests <id>` (alias `prs`) | `server/cmd/multica/cmd_issue.go:105` | `:104` |
| `runIssuePullRequests` handler | `server/cmd/multica/cmd_issue.go:507` | new citation |
| Calls `GET /api/issues/<id>/pull-requests` | `server/cmd/multica/cmd_issue.go:522` | `:522` (unchanged) |
| API route registration | `server/cmd/server/router.go:480` | `:480` (unchanged) |
| Handler `ListPullRequestsForIssue` → `Queries.ListPullRequestsByIssue` | `server/internal/handler/github.go:687,692` | `:466` |
| Row → response mapper `issuePullRequestRowToResponse` | `server/internal/handler/github.go:205` | `:149` |

The CLI resolves the issue ref, GETs the endpoint, and (for `--output json`)
prints the raw `{"pull_requests": [...]}` body. Only `--output` is accepted; the
default `table` shows `NUMBER STATE TITLE URL`.

## `multica issue comment update` — revision-checked edits

| Behavior | File:line |
|---|---|
| Cobra route, positive revision requirement, conflict retry guidance and side-effect help | `server/cmd/multica/cmd_issue.go:282` (`issueCommentUpdateCmd`, `init`) |
| Input sources share existing escape, byte decoding and workdir guards | `server/cmd/multica/cmd_issue.go:45` (`resolveTextFlag`, `ensureFileFlagWithinWorkdir`) |
| PUT sends only content and expected_revision; JSON stdout and success stderr; API errors return without retry | `server/cmd/multica/cmd_issue.go:2112` (`runIssueCommentUpdate`) |
| Workspace-scoped comment lookup, author/admin permission, optional attachment replacement and strict revision checks | `server/internal/handler/comment.go:3181` (`UpdateComment`) |
| Conditional content write and cancellation commit together; re-trigger uses the editing action's authority | `server/internal/handler/comment.go:3280` (`sourceTaskID`, `strictContentEdit`, `retriggerEditedComment`) |
| Conflict response is HTTP 409 with expected/actual revision | `server/internal/handler/handler.go:641` (`writeRevisionConflict`) |
| Route, request, file/stdin, output, mandatory revision and 403/409 regression coverage | `server/cmd/multica/cmd_issue_test.go` (`TestIssueCommentUpdateCommandRegistration`, `TestRunIssueCommentUpdate*`) |

## PR response shape

`GitHubPullRequestResponse` struct: `server/internal/handler/github.go:58`. JSON
fields the agent can read off each element of `pull_requests`:

- `provider` (`json:"provider"`, line 63)
- `number` (`json:"number"`, line 67)
- `html_url` (`json:"html_url"`, line 70)
- `title` (`json:"title"`, line 68)
- `state` (`json:"state"`, line 69) — the folded lifecycle enum (see below)
- `merged_at` (`json:"merged_at"`, line 74), `closed_at` (line 75)
- `mergeable_state` (`json:"mergeable_state"`, line 80) — mirrors GitHub; UI only
  surfaces `clean`/`dirty`, other values round-trip as unknown
- `snapshot_available` (`json:"snapshot_available"`, line 100) — for GitHub,
  true only when the App snapshot feature is enabled and the snapshot head
  matches the current PR head (`currentGitHubSnapshotAvailable`, lines 258-265)
- `mergeable` / `merge_state_status` (lines 90, 94) — conflict-only verdict vs
  the complete merge gate; "ready" requires `merge_state_status == "clean"`
- `checks_rollup` (`json:"checks_rollup"`, line 105) and run-level
  `checks_total` / `checks_passed` / `checks_failed` / `checks_running`
  (lines 111-114), plus `failed_check_names` (line 118)
- `checks_conclusion` (`json:"checks_conclusion"`, line 108) — coarse
  `"passed"`/`"failed"`/`"pending"` or `null`; GitHub derives it only from an
  available current-head snapshot (mapper lines 242-254), while self-hosted VCS
  providers use `aggregateChecksConclusion` (line 275)

There is **no** standalone `draft` or `merged` boolean in the response. The
PR lifecycle is encoded in the single `state` string by `derivePRState`
(`server/internal/handler/github.go:1317`):

```
merged   → if PullRequest.Merged
closed   → else if PullRequest.State == "closed"
draft    → else if PullRequest.Draft
open     → otherwise
```

`derivePRState` is called when the webhook upserts the row
(`server/internal/handler/github.go:1115`), so `state` is what the list endpoint
returns. "Is it merged?" = `state == "merged"` (or `merged_at != null`); "is it a
draft?" = `state == "draft"`. Combine with `checks_conclusion` for CI status.

## Two distinct webhook paths: link vs close-intent

Both run inside the `pull_request` webhook handler, gated by the workspace
auto-link flag (`workspaceAutoLinkPRsEnabled`, `github.go:1074`).

### Path 1 — link (title OR body OR branch)

- `extractIdentifiers` regex helper: `server/internal/handler/github.go:1028`
- driving regex `identifierRe` (`\b([a-z][a-z0-9]{1,9})-(\d+)\b`, case-insensitive):
  `server/internal/handler/github.go:490`
- call site: `server/internal/handler/github.go:727` —
  `extractIdentifiers(p.PullRequest.Title, p.PullRequest.Body, p.PullRequest.Head.Ref)`

Every `PREFIX-NUMBER` mention in **title, body, or branch** resolves to an issue
in the workspace and writes a link row (`LinkIssueToPullRequest`, ~`github.go:762`).
This is what `multica issue pull-requests` later reads back.

**Reference-only flag (MUL-3739).** The link row carries a `reference_only`
boolean (`migrations/127_issue_pull_request_reference_only.up.sql`). The handler
computes a `qualifyingIdents` set = identifiers in **title or branch** (any
`extractIdentifiers` match) ∪ **body closing keywords** (`closingIdents`). A
linked identifier NOT in that set was matched only by a bare body mention, so its
row is written with `reference_only = true`. Both `ListPullRequestsByIssue` and
`GetIssuePullRequestCloseAggregate` filter `AND NOT reference_only`, so
reference-only links are hidden from the CLI / UI PR list **and** excluded from
the auto-advance gate (an open body-only mention must not silently block the
issue from reaching `done` while invisible in the list). The row still exists for
edit-time close-intent tracking. `reference_only` follows the same
`preserve_close_intent` terminal gate as `close_intent`.

Drifted from the prior skill's `github.go:727` citation, which pointed at the old
call-site location for the link logic.

### Path 2 — close intent (title OR body only, keyword-adjacent)

- `extractClosingIdentifiers` regex helper: `server/internal/handler/github.go:1051`
- driving regex `closingIdentifierRe`
  (`\b(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?)[:\s]+([a-z][a-z0-9]{1,9})-(\d+)\b`):
  `server/internal/handler/github.go:501`
- call site: `server/internal/handler/github.go:736` —
  `extractClosingIdentifiers(p.PullRequest.Title, p.PullRequest.Body)` (no branch arg)

Only a `PREFIX-NUMBER` immediately after a closing keyword
(`Closes`/`Fixes`/`Resolves`, optional `:` then whitespace) sets the link row's
`close_intent` flag — the gate that auto-advances the issue to `done` on merge.
`Fix MUL-1` closes; `Fix login MUL-1` does not (adjacency). Branch names are
deliberately excluded (function doc, `github.go:1044-1050`): a branch like
`mul-1/fix-login` links but must never declare close intent.

Drifted from the prior skill's `github.go:736` citation.

Net: a bare title prefix (`MUL-2759: ...`) or a branch ref links only (shown in
the PR list); `Closes MUL-2759` links **and** records close intent; a bare body
mention with no title/branch ref and no closing keyword links as `reference_only`
and is hidden from the PR list.

## Status side effects (enqueue contracts)

These contracts apply to formal issues. Triage intake and ordinary acceptance
use their separate, non-dispatching paths above.

| Behavior | File:line | Drifted from |
|---|---|---|
| Create-time: agent-assigned, non-backlog issue enqueues immediately | `server/internal/handler/issue.go:2263-2264` | new citation |
| `shouldEnqueueAgentTask` returns false for `backlog` (parking lot) | `server/internal/handler/issue.go:2644-2648` | new citation |
| Backlog → non-backlog (not done/cancelled) enqueues on update | `server/internal/handler/issue.go:2537-2540` | `:2523` |
| Same contract in batch update | `server/internal/handler/issue.go:3021-3024` | new citation |
| Child → `done` notifies + wakes the parent, gated by the stage barrier | `server/internal/handler/issue_child_done.go:66` (`notifyParentOfChildDone`; doc comment at `:15`; barrier gate at `:115`) | func def `:51` |
| Status change (incl. → `cancelled`) does NOT cancel in-flight tasks; only issue deletion does (MUL-4465) | no-cancel note in `server/internal/handler/issue.go:2652-2658` (`UpdateIssue`) and `:3170-3171` (`BatchUpdateIssues`); deletion still cancels at `:2863` (`DeleteIssue`) / `:3239` (`BatchDeleteIssues`) via `CancelTasksForIssue` (`server/internal/service/task.go:1229`) | new citation |
| `StartTask` / `CompleteTask` do not write issue status (agent CLI owns progress) | `server/internal/service/task.go` (`StartTask` / `CompleteTask` comments) | new citation |
| Runtime brief: status written whenever the work changes it, mid-turn included — starting the issue's own ask → `in_progress` immediately (workflow step 3); delivery → `in_review`, continuing → `in_progress`, stuck → `blocked`; a turn producing none of the issue's own deliverable → no write at any point; the activity kind never decides (research/design/planning/review count as work when they are the ask); no assignee gate; squad leader dispatch is not delivery (MUL-6417) | `server/internal/daemon/execenv/runtime_config_sections.go` (`writeWorkflowIssue`) | new citation |
| Failed task may roll `in_progress` → `todo` when no active task remains | `server/internal/service/task.go` (`HandleFailedTasks`) | new citation |
| Custom statuses inherit their category's behavior in full; enqueue/park contracts resolve the effective category via `issuestatus.Effective` / `Resolve` (MUL-6243) | `server/internal/issuestatus/issuestatus.go` (`Effective`, `Resolve`) | new citation |
| Runtime brief lists the workspace's active custom statuses grouped by category; catalog rides the claim payload (MUL-6460) | `server/internal/daemon/execenv/runtime_config_sections.go` (`writeIssueStatusCommand`); claim injection in `server/internal/handler/daemon.go` (`buildClaimedTaskResponse`, status catalog block) | new citation |
| Literal-key exceptions to category rules: failed-task rollback writes the `todo` key; merged close-intent PR writes the `done` key | `server/internal/service/task.go` (`HandleFailedTasks`); `server/internal/handler/github.go` (merge close-intent path) | new citation |

Creation with `--status todo` (or any non-backlog status) on an agent-assigned
issue fires the agent immediately; `--status backlog` parks it with the assignee
set but no trigger. Promoting `backlog → todo` later fires it then (update path,
line 2537).

Moving an issue to `cancelled` used to call `CancelTasksForIssue` and stop every
active task on it (the old #940 behavior). MUL-4465 removed that from both
`UpdateIssue` and `BatchUpdateIssues`: a status flip — `cancelled` included —
never cancels tasks now. `CancelTasksForIssue` fires only from the issue-deletion
paths (`DeleteIssue` / `BatchDeleteIssues`), where the owning issue row is going
away, so no task is left orphaned.

## Ownership-only assignment and duplicate-run awareness

| Behavior | Source |
|---|---|
| `issue assign --no-start`, `issue update --no-start`, and `issue status --no-start` send `suppress_run=true` | `server/cmd/multica/cmd_issue.go` (`runIssueAssign`, `runIssueUpdate`, `runIssueStatus`) |
| Update and batch-update apply ownership while skipping dispatch when `suppress_run` is true | `server/internal/handler/issue.go` (`UpdateIssue`, `BatchUpdateIssues`) |
| Trusted direct self-assignment suppresses enqueue only when the target `(issue, agent)` already has a non-terminal task | `server/internal/service/issue_trigger.go` (`WillEnqueueRun`), `server/internal/handler/issue_trigger.go` (`shouldSuppressActiveSelfAssignment`) |
| Claim responses expose a bounded, workspace-scoped snapshot of the same agent's other dispatched/running/waiting issue tasks; queued tasks are excluded | `server/pkg/db/queries/agent.sql` (`ListActiveSiblingIssueTasks`), `server/internal/handler/daemon.go` (`buildClaimedTaskResponse`) |
| Daemon prompts point to the target's comment history and concrete sibling `run-messages` commands | `server/internal/daemon/prompt.go` (`buildActiveSiblingRunsBlock`) |

The self-assignment guard is intentionally pair-scoped. It does not treat
"this agent is busy on some other issue" as a reason to suppress a fresh
cross-issue handoff, because serial sub-issue promotion and handoff batches rely
on those assignments creating their normal queued runs. Triage acceptance is
not such a handoff and does not enqueue.

## Sub-issue stages (barrier wake)

| Behavior | File:line |
|---|---|
| `issue.stage` column (nullable, `>= 1`) | `server/migrations/123_issue_stage.up.sql` |
| Stage barrier: notify+wake fire only when the lowest unfinished stage is all-terminal; unstaged set = one implicit stage | `server/internal/handler/issue_child_done.go:231` (`stageBarrierClosed`) |
| Per-stage summary + next stage for the wake comment | `server/internal/handler/issue_child_done.go:254` (`stageProgressSummary`) |
| Collaboration complete/cancel returns a transient post-commit before/after effect; replay, refusal and uncertain commit do not dispatch it | `server/internal/service/collaboration_lifecycle.go` (`changeLifecycle`); `server/internal/service/collaboration_workflow.go` (`CollaborationWorkflowReceipt.ParentWakeup`); `server/internal/handler/collaboration_lifecycle.go` (`ChangeCollaborationLifecycle`); `server/internal/service/collaboration_parent_wakeup_test.go` |
| Ordinary parent wake rechecks current parent/shared status, exposes only the shared child's unlinked identifier/title, and retains stage and agent/squad guards | `server/internal/handler/issue_child_done.go` (`postChildDoneComment`); `server/internal/handler/collaboration_parent_wakeup_test.go` |
| `--stage` on `issue create` / `issue update` | `server/cmd/multica/cmd_issue.go:328,350` |
| `multica issue children <id>` (sub-issues grouped by stage) | `server/cmd/multica/cmd_issue.go:114,678`; stage `done` counting via `isTerminalChildIssue` (reads `status_category`, MUL-6243); route `GET /api/issues/{id}/children` → `ListChildIssues` |

Advancement is agent-driven: the server only detects the closed barrier and
wakes the parent assignee. Promoting the next stage's `backlog` sub-issues to
`todo` is the woken agent's decision, not a server side effect. When the woken
assignee (often a squad leader) decides the parent is complete, the system
comment explicitly asks for `multica issue status <parent-id> in_review`. Any
turn may move the status on its own too, judged from what the work changes
about the issue — there is no assignee gate (MUL-6417).

## Metadata CLI

| Behavior | File:line |
|---|---|
| `multica issue metadata set <issue-id> --key --value [--type]` | `server/cmd/multica/cmd_issue_metadata.go:80,109-111` |
| `multica issue metadata delete <issue-id> --key` | `server/cmd/multica/cmd_issue_metadata.go:93,113` |
| API routes (PUT/DELETE `/metadata/{key}`) | `server/cmd/server/router.go:478-479` |

`--value` is JSON-parsed by default (bool/number sniff); `--type` forces
`string`/`number`/`bool`.

## Custom properties CLI

| Behavior | File:line |
|---|---|
| `multica property list/get/create/update/archive/unarchive` | `server/cmd/multica/cmd_property.go` |
| `multica issue property list/set/unset` (name→id translation) | `server/cmd/multica/cmd_property.go` (`encodeIssuePropertyValue`) |
| Definition CRUD, admin gate, agent-actor rejection | `server/internal/handler/property.go` (`requirePropertyAdmin`) |
| Optional catalog icon field and allowlist validation | `server/internal/handler/property.go` (`PropertyResponse`, `validatePropertyIcon`) |
| Per-type value validation (self-correcting errors) | `server/internal/handler/property.go` (`validatePropertyValue`) |
| `actor` / `multi_actor` reference parsing, `member` as the only kind, 20-value cap | `server/internal/handler/property.go` (`actorPropertyKinds`, `parseActorRef`, `parseActorRefList`, `maxPropertyActorValues`) |
| Actor references are checked for workspace membership only | `server/internal/handler/property.go` (`resolveActorRefs`) |
| `--value` name / email / id → `member:<uuid>` resolution (same member lookup as `--assignee`) | `server/cmd/multica/cmd_property.go` (`resolveActorPropertyRef`, `memberOnlyKinds`) |
| Shared actor-reference types and helpers | `packages/core/types/property.ts` (`parseActorRef`, `actorRefsFromValue`, `MAX_ISSUE_PROPERTY_ACTOR_VALUES`) |
| API routes (`/api/properties`, PUT/DELETE `/api/issues/{id}/properties/{propertyId}`) | `server/cmd/server/router.go` |

## Verification command

### Shared configuration and Switch

| Contract | Source |
|---|---|
| Immutable proposed configuration, actual-owner consent and atomic held Switch; no pre-consent live mutation route | `server/internal/service/collaboration_switch.go`, `server/internal/handler/collaboration_switch.go`, `server/cmd/server/router.go` |
| Original-key authorized receipt recovery and owner-scoped committed grant IDs | `server/internal/service/collaboration_switch_view.go`; `collaboration_switch_test.go` |
| Real registered managed target retains waiting input and needs explicit Resume after Switch | `server/cmd/server/collaboration_switch_routes_test.go` |
| Shared skills propagate body/file/release invalidation through enabled agent bindings without changing channel control | `server/migrations/709_collaboration_skill_content_invalidation.up.sql`, `server/internal/service/collaboration_config_invalidation.go`, service/handler `collaboration_skill_invalidation_test.go` |

Re-derive any line above before depending on it:

```bash
cd server
grep -n 'pull-requests <id>'                 cmd/multica/cmd_issue.go
grep -n 'ListPullRequestsForIssue'           cmd/server/router.go internal/handler/github.go
grep -n 'func issuePullRequestRowToResponse\|type GitHubPullRequestResponse struct\|func derivePRState\|func extractIdentifiers\|func extractClosingIdentifiers\|closingIdentifierRe' internal/handler/github.go
grep -n 'extractIdentifiers(\|extractClosingIdentifiers(\|derivePRState(' internal/handler/github.go
grep -n 'qualifyingIdents\|reference_only\|ReferenceOnly' internal/handler/github.go pkg/db/queries/github.sql
grep -n 'prevIssue.Status == "backlog"\|func (h \*Handler) shouldEnqueueAgentTask' internal/handler/issue.go
grep -n 'func notifyParentOfChildDone'       internal/handler/issue_child_done.go
```

## Iteration compatibility and operation recovery

| Contract | Source |
|---|---|
| Omitted fields preserve membership; release-disabled assignment, client rollover and explicit batch/Plugin fields return 428 | `server/internal/handler/iteration_confirmation.go`, `issue.go` (`CreateIssue`, `UpdateIssue`, `BatchUpdateIssues`), `plugin_action.go` (`PatchPluginIssue`), `iteration_confirmation_test.go` |
| Confirmed create requires target expected_iteration_revision; done-to-active additionally requires allow_completed=true; commits participation with the issue, preserves duplicate409 and normal enqueue | `server/internal/handler/issue_iteration_assignment.go` (`parseIssueIterationCreate`, `issueIterationCreateHooks`), `server/internal/service/issue.go` (`AfterCreateInTx`), `issue_iteration_assignment_test.go` |
| Membership-only PUT requires issue expected_revision, reason for leave/switch, explicit allow_completed for active done joins; compound edits400; Observer denied; no execution | `server/internal/handler/issue_iteration_assignment.go` (`updateIssueIterationAssignment`, `updateIssueIterationInTx`), `issue.go` (`updateIssueAtomically`), `server/internal/iteration/membership.go`, `issue_iteration_assignment_test.go` |
| Detail/list/open/search/grouped/table issue DTOs and background broadcasts expose current_iteration_id and actual rollover count with identical JSON types | `server/internal/handler/issue.go` (three response builders and handwritten SELECT/Scan projections), `issue_table_rows.go`, `server/internal/service/task.go` (`IssueToMap`), `issue_payload_shape_test.go`, `issue_iteration_assignment_test.go` |
| An authorized actor reads their stored operation without a client hash, even after the entity was deleted | `server/internal/handler/iteration_operations.go`, `server/internal/iteration/operation.go` (`ReadOperation`), `server/cmd/server/router.go` |
| Membership is held current through read; revoked inbox content is removed atomically and suppressed outbox identities prevent reconstruction | `iteration_operations_test.go`, `iteration_revoke_test.go`, `workspace_revoke.go`, `server/pkg/db/queries/iteration.sql` (`DeleteIterationNotificationsForMember`) |
| Full route authentication and URL workspace isolation | `server/cmd/server/iteration_operations_route_test.go` |
| Settings/capability discovery defaults missing/new settings to enabled, preserving saved choices and human/admin enable/disable authority | `server/internal/handler/iteration_settings.go`, `server/internal/service/iteration.go`, `server/migrations/567_workspace_feature_defaults.up.sql` |
| Stable request identity, current authorization on retries, atomic result and mutation | `server/internal/iteration/operation_execute.go`, `server/internal/handler/iteration_settings_test.go` |
| Current human members maintain periods; complete preview/hash confirmation; no management execution side effects | `server/internal/handler/iteration_management.go`, `server/internal/service/iteration_lifecycle_preview.go`, `iteration_lifecycle_apply.go`, `iteration_lifecycle_test.go` |
| T1 optional iteration_assignment and final fields commit atomically; current invocation grants stay locked; plain accept never executes | `server/internal/handler/triage_iteration.go`, `triage_actions.go`, `triage_iteration_assignment_test.go`, `triage_iteration_authorization_test.go` |
| Coherent version-bound list/history pages; closed scope/statistics frozen, later metadata audit allowed | `server/internal/handler/iteration_list.go`, `iteration_history.go`, `server/internal/iteration/history.go`, `history_snapshot.go`, `server/cmd/server/iteration_lifecycle_routes_test.go` |
| End/active cancel/handoff/whole disable freeze one snapshot, release terminal tasks, roll remaining tasks exactly once, and preserve execution | `server/internal/service/iteration_closure.go`, `iteration_closure_test.go`, `server/internal/iteration/membership_batch.go`, `server/internal/handler/iteration_closure_writers_test.go` |
| Lost responses recover the original operation; complete sets and all writes share one transaction | `TestIterationClosureHTTPRecoversLostCommitResponse`, `TestIterationClosureBatchCASFailureRollsBackAllPhases`, `TestIterationClosureFinalSnapshotFailureRollsBackHandoff` |
| In-app outbox delivery reauthorizes and commits stable inbox ID with delivered status; overdue reminders use saved timezone and never advance periods | `server/internal/service/iteration_notifications.go`, `iteration_notifications_test.go`, `server/internal/handler/iteration_notifications.go`, `server/cmd/server/main.go` |
| Agent create provenance and assigned-agent rights are checked from locked references | `server/internal/handler/issue.go` (`prepareIssueCreationInTx`), `issue_create_authorization_test.go` |
