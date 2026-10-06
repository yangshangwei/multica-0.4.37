# working-on-issues source map

Evidence layer for `SKILL.md`. Every contract the skill states is traced to a
source file and symbol here. Historical PR/status rows retain their original
`feat/builtin-skills` line citations and drift notes; re-confirm an exact line
before using it. Triage references use symbols so integration edits do not turn
line movement into a misleading contract.

## Triage admission and human decisions

| Behavior | Source |
|---|---|
| Only `not_required` and `accepted` are formal; unknown values fail closed; execution checks read persisted state | `server/internal/admission/admission.go` (`Formal`, `Check`); `server/internal/service/issue_admission.go` (`checkIssueExecution`) |
| Explicit intake defaults to pending/backlog; candidates remain separate from formal project/assignee; agent intake retains Contributor authority | `server/internal/handler/triage.go` (`CreateTriageItem`, `createTriageItemInTx`) |
| Human review verifies credential class and resolved actor; settings require owner/admin; workspace membership rechecked under write fences | `server/internal/handler/triage.go` (`triageHumanActor`, `beginTriageWrite`, `UpdateTriageSettings`) |
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
| Omitted iteration fields preserve membership; explicit generic writes return 428, including null | `server/internal/handler/iteration_confirmation.go`, `issue.go` (`CreateIssue`, `UpdateIssue`, `BatchUpdateIssues`), `plugin_action.go` (`PatchPluginIssue`), `iteration_confirmation_test.go` |
| An authorized actor reads their stored operation without a client hash, even after the entity was deleted | `server/internal/handler/iteration_operations.go`, `server/internal/iteration/operation.go` (`ReadOperation`), `server/cmd/server/router.go` |
| Membership is held current through read; revoked notifications are removed atomically | `iteration_operations_test.go`, `iteration_revoke_test.go`, `workspace_revoke.go`, `server/pkg/db/queries/iteration.sql` (`DeleteIterationNotificationsForMember`) |
| Full route authentication and URL workspace isolation | `server/cmd/server/iteration_operations_route_test.go` |
| Settings/capability discovery and human/admin enable, with rollout closed by default | `server/internal/handler/iteration_settings.go`, `server/internal/service/iteration.go`, `server/internal/featureflags/keys.go` |
| Stable request identity, current authorization on retries, atomic result and mutation | `server/internal/iteration/operation_execute.go`, `server/internal/handler/iteration_settings_test.go` |
| Agent create provenance and assigned-agent rights are checked from locked references | `server/internal/handler/issue.go` (`prepareIssueCreationInTx`), `issue_create_authorization_test.go` |
