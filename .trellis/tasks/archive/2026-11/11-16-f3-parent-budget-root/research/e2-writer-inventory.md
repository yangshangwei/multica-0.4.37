# Research: E2 writer inventory — every path that creates an issue row or changes `issue.parent_issue_id`

- **Query**: F3 step 1 ("E2 清点"). List every code path that creates an `issue` row or changes `issue.parent_issue_id`, with transaction owner, `iteration:<ws>` fence, row locks and lock order; re-verify contract R3 / R4 / R5.
- **Scope**: internal (read-only source research)
- **Date**: 2026-11-16
- **Source tree read**: `/Volumes/artisan/code/2026/mc-f3-budget-root-20261116` at `86fea651cc5d09360327b6cf0f8357c1f360070d` (`git rev-parse HEAD`; `git status --porcelain` empty). No product code was read from the shared checkout.
- **Method**: static reading and `grep` only. No test, build or database command was run. Every concurrency statement below is derived from reading lock statements, not from an experiment. Statements marked **(inferred)** were not read directly.

All paths below are relative to `server/` in the clean worktree unless they start with `e2e/` or `scripts/`.

## Summary

| Kind | Count | Where |
|---|---|---|
| sqlc queries that `INSERT INTO issue` | 2 | `CreateIssue`, `CreateIssueWithOrigin` |
| Go call sites of those two queries | 5 | `internal/service/issue.go:532`, `:554`; `internal/service/autopilot.go:790`; `internal/handler/onboarding_shim.go:275`, `:446` |
| Creation entry paths (who reaches an INSERT) | 11 | C1–C11 below; 5 of them can produce a non-null parent |
| sqlc queries that write `issue.parent_issue_id` on an existing row | 4 | `UpdateIssue`, `DetachDirectChildIssues`, `UpdateCollaborationIssueStructure`, `PrepareWorkspaceDeletionLinks` |
| sqlc queries that `DELETE FROM issue` | 2 | `DeleteIssue`, `DeleteWorkspaceIssueRoots` |
| Database-level parent writer | 1 | FK `ON DELETE SET NULL` on `issue.parent_issue_id` |
| Product writers that do **not** take `iteration:<ws>` | 1 transaction (2 statements) | workspace teardown |

Every product Go path that inserts an issue, re-parents one, or deletes one takes the `iteration:<ws>` fence, **except workspace teardown**.

## Findings

### Files Found

| File Path | Description |
|---|---|
| `internal/iteration/transaction.go` | The fence: `WorkspaceFenceSQL` (:83), `LockWorkspace` (:89-95) |
| `pkg/db/queries/issue.sql` | `DetachDirectChildIssues` (:132), `CreateIssue` (:144), `UpdateIssue` (:159), `CreateIssueWithOrigin` (:280), `DeleteIssue` (:329) |
| `pkg/db/queries/collaboration_source_fields.sql` | `LockCollaborationSourceParent` (:18), `UpdateCollaborationIssueStructure` (:23) |
| `pkg/db/queries/workspace_delete.sql` | `PrepareWorkspaceDeletionLinks` (:238), `DeleteWorkspaceIssueRoots` (:556) |
| `pkg/db/queries/iteration.sql` | `LockIssueDeleteRows` (:97) |
| `pkg/db/queries/issue_delete.sql` | Delete-time task / token / autopilot-run locks |
| `pkg/db/queries/lifecycle.sql` | `LockLifecycleIssue` (:5-9) |
| `pkg/db/queries/agent.sql` | `CreateQuickCreateTask` (:459-489), `LinkTaskToIssue` (:530-544), `CreateManualQuickCreateRetryTask` (:662) |
| `internal/service/issue.go` | `Create` (:242), `createOnce` (:275), `CreateInTx` (:386) — the shared creation core |
| `internal/handler/issue.go` | HTTP create / quick-create / update / batch update / delete / batch delete |
| `internal/handler/issue_move.go` | `MoveIssue`, delegates to `UpdateIssue` |
| `internal/handler/issue_iteration_assignment.go` | `updateIssueIterationInTx` (:241) |
| `internal/handler/source_context.go` | Comment-to-child (`CreateCommentSubIssue` :464) |
| `internal/handler/lifecycle_handoff_transaction.go` | Handoff follow-up child (`writeLifecycleHandoffInTx` :247) |
| `internal/handler/triage.go`, `internal/handler/triage_import.go` | Triage intake and CSV import |
| `internal/handler/onboarding_shim.go` | Onboarding seed issues |
| `internal/handler/workspace.go` | `DeleteWorkspace` (:1081) |
| `internal/service/autopilot.go` | `dispatchCreateIssue` (:678) |
| `internal/service/collaboration.go` | `CollaborationService.Create` (:156), `CreateInTx` (:190), `createCollaborationIssueInTx` (:303) |
| `internal/service/collaboration_creation_fields.go` | `prepareCollaborationCreationFields` (:72) |
| `internal/service/collaboration_stage_actions.go` | `materializeStageChildren` (:567) |
| `internal/service/collaboration_edit.go` | `EditIssue` (:35) |
| `internal/service/collaboration_source_fields.go` | `validateCollaborationParent` (:144-166), `prepareSourceStructure` (:168-195) |
| `internal/service/collaboration_workflow.go` | Workflow transaction (`workflowInTx` :460) |
| `internal/service/collaboration_stage_admission.go` | `checkStageChildCreate` (:69), `CheckStageChildUpdateInTx` (:96) |
| `internal/service/task.go` | Quick-create enqueue (:1640-1800), `notifyQuickCreateCompleted` (:7568) |
| `internal/integrations/channel/engine/router.go` | Channel `createIssue` (:1052) |
| `internal/integrations/slack/slash_command.go` | Slack quick-create enqueue (:275) |
| `internal/admission/admission.go` | `Check` (:28-40) |
| `migrations/001_init.up.sql` | `parent_issue_id UUID REFERENCES issue(id) ON DELETE SET NULL` (:65) |

### The fence

`internal/iteration/transaction.go:83`:

```go
const WorkspaceFenceSQL = `SELECT pg_advisory_xact_lock(hashtextextended('iteration:' || $1::uuid::text, 0))`
```

`LockWorkspace(ctx, tx, workspaceID)` (:89-95) runs it. It is a transaction-level, exclusive advisory lock. Its doc comment places it after the workspace `FOR KEY SHARE`, member fences and the status-catalogue fence, and before any iteration / attachment / issue row lock.

`lockIssueWriteFences` (`internal/handler/issue.go:3868`, statement list :3860-3866) sends one pipelined batch in this order: workspace `FOR KEY SHARE` → subscriber advisory lock `pg_advisory_xact_lock(hashtext(ws), hashtext(user))` → member `FOR SHARE` → status-catalogue shared advisory lock → `iteration.WorkspaceFenceSQL`.

### SQL-side enumeration (item 10)

Complete set of sqlc queries that insert an issue, write `parent_issue_id`, or delete an issue, with every Go caller found outside `_test.go` and generated code.

| # | Query | File:line | Statement | Go callers |
|---|---|---|---|---|
| S1 | `CreateIssue :one` | `pkg/db/queries/issue.sql:144-153` | `INSERT INTO issue (… parent_issue_id …)` (`$10`) | `internal/service/issue.go:554`; `internal/handler/onboarding_shim.go:275`, `:446` |
| S2 | `CreateIssueWithOrigin :one` | `pkg/db/queries/issue.sql:280-289` | Same plus `origin_type`, `origin_id` | `internal/service/issue.go:532`; `internal/service/autopilot.go:790` |
| S3 | `UpdateIssue :one` | `pkg/db/queries/issue.sql:159-254` | `parent_issue_id = changed.next_parent_issue_id` (:236); revision CAS (:204, :253); admission guard (:247-248) | `internal/handler/issue.go:4112` only |
| S4 | `DetachDirectChildIssues :many` | `pkg/db/queries/issue.sql:132-142` | `SET parent_issue_id = NULL, stage = NULL, revision = revision + 1 WHERE workspace_id AND parent_issue_id = $parent AND NOT id = ANY(excluded)` | `internal/handler/issue.go:4964` only |
| S5 | `UpdateCollaborationIssueStructure :one` | `pkg/db/queries/collaboration_source_fields.sql:23-31` | `UPDATE issue SET project_id, parent_issue_id = sqlc.narg(parent_issue_id)::uuid, stage WHERE id AND workspace_id AND EXISTS (issue_collaboration)` | `internal/service/collaboration_edit.go:116` only |
| S6 | `PrepareWorkspaceDeletionLinks :exec` | `pkg/db/queries/workspace_delete.sql:238-262` | CTE `detached_issues AS (UPDATE issue SET parent_issue_id = NULL WHERE issue.workspace_id = $1 AND parent_issue_id IS NOT NULL)` (:252-257) | `internal/handler/workspace.go:1204` only |
| S7 | `DeleteIssue :exec` | `pkg/db/queries/issue.sql:329-364` | `DELETE FROM issue WHERE issue.id IN (SELECT target.id FROM target)` with workspace guard; also deletes report snapshots, triage rows, VCS PR links | `internal/handler/issue.go:5016` only |
| S8 | `DeleteWorkspaceIssueRoots :exec` | `pkg/db/queries/workspace_delete.sql:556-582` | `DELETE FROM issue WHERE issue.workspace_id = $1` (:567) | `internal/handler/workspace.go:1300` only |

Searches that returned nothing:

- Raw `INSERT INTO issue` in non-test, non-generated Go: none.
- Raw `UPDATE issue` in Go: only `internal/handler/triage_actions.go:269, 479, 509, 518` and `internal/handler/triage.go:614`; none of them writes `parent_issue_id`.
- `SET workspace_id` on `issue` in queries or Go: none.
- `INSERT INTO issue` in `migrations/`: none.
- `CREATE TRIGGER … ON issue` in `migrations/`: none.
- `parent_issue_id` in `migrations/`: appears in 001 (DDL), 123, 204, 535, 650. 535 and 650 read `p_context->>'parent_issue_id'` inside `lock_issue_execution`; none writes `issue.parent_issue_id`.

All other `parent_issue_id` occurrences in `pkg/db/queries/*.sql` are reads. All other `UPDATE issue` queries (attachment, comments, collaboration edit / workflow / position, labels, properties, reactions, iteration membership, lifecycle, `DetachProjectIssues`, `TransferSquadAssignees`) leave `parent_issue_id` untouched.

Dynamic-table SQL: `internal/issueactivitybackfill/backfill.go:43-64` builds `UPDATE %s i SET last_activity_at = i.updated_at` with table default `"issue"`; it writes only `last_activity_at`. The other backfill packages (`attributionbackfill`, `chatoriginbackfill`, `taskusagebackfill`) have no `issue` table reference by grep.

### Shared creation core: `IssueService.CreateInTx`

Nine of the eleven creation paths end here, so its lock sequence is stated once.

`IssueService.Create` (`internal/service/issue.go:242`) → `RetryProjectAssociationTransaction` → `createOnce` (:275), which **opens the transaction** (:278) and takes: workspace `FOR KEY SHARE` via `LockWorkspaceForChatSessionCreate` (:284) → `LockSubscriberWrites` (:294) → `LockActiveMember` (:297) → when `opts.PrepareInTx != nil`: `LockIssueStatusCatalogShared` (:304) → `iteration.LockWorkspace` (:307) → `PrepareInTx` (:310) → `CreateInTx` (:315) → `AfterCreateInTx` → commit (:338).

`CreateInTx` (:386) **borrows the caller's transaction** and takes, in order:

1. Workspace `FOR KEY SHARE` (:389).
2. Status-catalogue shared advisory lock (:392).
3. **`iteration.LockWorkspace` (:395)** — the fence, unconditionally, for every caller.
4. When `SourceContext` is set: source issue `FOR UPDATE` via `LockIssueForDescriptionUpdate` (:400), then `LockCommentAncestorPath` (:408).
5. Parent read: `qtx.GetIssueInWorkspace` (:451) — **a plain read, no row lock** — then `admission.Formal(parent)` (:461) and project back-fill from the parent (:467).
6. Project `FOR SHARE NOWAIT` via `LockProjectForAssociationNowait` (:472).
7. `checkStageChildCreate` (:485) — takes no row lock (see below).
8. `CheckDeliveryCreateInTx` (:489), labels.
9. Duplicate advisory guard `issueguard.LockAndFindActiveDuplicate` (:502).
10. `AllocateIssueNumber` (:511) — `UPDATE workspace SET issue_counter …` (`internal/service/issue_limit.go:88`).
11. `NextTopPosition` (:525).
12. INSERT: `CreateIssueWithOrigin` (:532) when an origin is set, else `CreateIssue` (:554). Both pass `ParentIssueID: p.ParentIssueID`.

The parent row is never explicitly locked on creation. `pkg/db/queries/lifecycle.sql:7` carries the comment "NO KEY UPDATE still permits ordinary child creation's parent key-share lock"; with the foreign key in `001_init.up.sql:65` still present, the INSERT takes PostgreSQL's implicit referential `FOR KEY SHARE` on the parent row **(inferred from the FK plus that comment; not observed)**.

`checkStageChildCreate` (`internal/service/collaboration_stage_admission.go:69-88`) and `CheckStageChildUpdateInTx` (:96-116) resolve the governing collaboration root through `GetCollaborationStageGoverningRoot` (`pkg/db/queries/collaboration_stage.sql:190-203`), a recursive ancestor read bounded by `stageAncestorDepth = 64` (`collaboration_stage_admission.go:21`). The query has no `FOR` clause. `CheckStageChildUpdateInTx` calls `BumpCollaborationStageChildSemanticVersion` (:114) when title, description, project or parent changed.

### Creation writers (C1–C11)

"Fence" = whether the transaction takes `iteration:<ws>`, with the evidence line.

| ID | Entry point (route) | Function, file:line | sqlc INSERT | Transaction owner | Fence | Row locks, in order | Parent can be non-null |
|---|---|---|---|---|---|---|---|
| C1 | `POST /api/issues` (`cmd/server/router.go:2266`) — members, agent CLI, quick-create agents | `CreateIssue` `internal/handler/issue.go:2995`; calls `IssueService.Create` :3199 with `ParentIssueID` :3209 | S1 or S2 via `CreateInTx` | `createOnce` `internal/service/issue.go:278` | **Yes** — `internal/service/issue.go:307` (handler passes `PrepareInTx`) and again :395 | Core sequence; `prepareIssueCreationInTx` (`internal/handler/issue.go:3325`) adds agent `FOR SHARE NOWAIT` and origin task `FOR SHARE OF t NOWAIT` | **Yes** — from request body |
| C2 | `POST …/comments/{id}/sub-issues` (`router.go:2516`, `RequireHumanActor`), manual branch | `createManualCommentSubIssue` `internal/handler/source_context.go:667`; `IssueService.Create` :746, `ParentIssueID: capture.SourceIssueID` :749 | S1 or S2 via `CreateInTx` | `createOnce` | **Yes** — `internal/service/issue.go:395` | Core sequence; source (= parent) issue `FOR UPDATE` at `issue.go:400`, comment ancestor path :408 | **Always** — parent is the comment's issue |
| C3 | Channel `/issue` command | `createIssue` `internal/integrations/channel/engine/router.go:1052`; `r.issues.Create` :1085 | S2 via `CreateInTx` | `createOnce` | **Yes** — `internal/service/issue.go:395` | Core sequence | **No** — params built at :1055 carry no `ParentIssueID` |
| C4 | Lifecycle handoff follow-up | `writeLifecycleHandoffInTx` `internal/handler/lifecycle_handoff_transaction.go:247`; `IssueService.CreateInTx` :405, `ParentIssueID: source.ID` :398 | S1 or S2 via `CreateInTx` | The handoff handler (transaction begun before :255) | **Yes** — `lifecycle_handoff_transaction.go:275`, and again `issue.go:395` | Workspace `FOR KEY SHARE` :256 → subscriber :263 → member :266 → status catalogue :272 → fence :275 → duplicate advisory guard :282 → agent `FOR SHARE NOWAIT` :302 → source issue (and existing target, in id order) `FOR NO KEY UPDATE NOWAIT` via `LockLifecycleIssue` :320 (`pkg/db/queries/lifecycle.sql:5-9`) → origin task :370 → core sequence | **Always** — parent is the handoff source |
| C5 | `POST …/triage/items` (`router.go:2235`) | `CreateTriageItem` `internal/handler/triage.go:659`; `createTriageItemInTx` :523; `IssueService.CreateInTx` :609 | S1 via `CreateInTx` | `beginTriageWrite` `internal/handler/triage.go:207` | **Yes** — `internal/handler/triage.go:253`, and again `issue.go:395` | `workspace_triage_settings FOR UPDATE` :235 → subscriber → member → status catalogue → fence → core sequence; then raw `UPDATE issue SET admission_status='pending'` :614 | **No** — params at :609 carry no `ParentIssueID` |
| C6 | Triage CSV import | `commitTriageImportRow` `internal/handler/triage_import.go:379`; `createTriageItemInTx` :443 inside a nested transaction (savepoint) | S1 via `CreateInTx` | `beginTriageWrite` (:381) | **Yes** — same as C5 | As C5, plus batch row and import row `FOR UPDATE` (:387, :394) and an external-id advisory lock (:416) before the savepoint | **No** |
| C7 | `POST /api/collaboration/issues` (`router.go:1677`) | `CollaborationService.Create` `internal/service/collaboration.go:156` → `CreateInTx` :190 → `createCollaborationIssueInTx` :303; `Issues.CreateInTx` :315, `ParentIssueID: p.ParentIssueID` :319 | S1 via `CreateInTx` | `CollaborationService.Create` (:165) | **Yes** — `internal/service/collaboration_creation_fields.go:79`, and again `issue.go:395` | User rows (`LockCollaborationUsers` :218) → collaboration gate shared :240 → workspace `FOR KEY SHARE` :244 → source member :250 → savepoint :277 → account fences :307 → status catalogue + fence (`collaboration_creation_fields.go:72-79`) → core sequence | **Yes** — optional, from request body (`internal/handler/collaboration_creation.go:92`) |
| C8 | Stage-proposal acceptance inside a collaboration workflow | `materializeStageChildren` `internal/service/collaboration_stage_actions.go:567` (called at :514); `issues.CreateInTx` :577, `ParentIssueID: root.ID` :579 | S1 via `CreateInTx` | Collaboration workflow transaction (`internal/service/collaboration_workflow.go:427-457`) | **Yes** — `internal/service/collaboration_workflow.go:513`, and again `issue.go:395` | Workflow sequence (users → workspace `FOR KEY SHARE` :486 → source member → status catalogue :505 → fence :513 → issue iteration → collaboration root `FOR NO KEY UPDATE` :526 → collaboration / session / channel / target rows) → savepoint `stage_materialization` → core sequence per child | **Always** — parent is the collaboration root |
| C9 | Autopilot `create_issue` dispatch | `dispatchCreateIssue` `internal/service/autopilot.go:678` | S2 directly at :790 (does not use `CreateInTx`) | `dispatchCreateIssue` (:681) | **Yes** — `internal/service/autopilot.go:710` | Workspace `FOR KEY SHARE` :688 → subscriber :698 → member :701 → status catalogue :707 → fence :710 → project `FOR SHARE` + autopilot `FOR UPDATE` :717 → squad / leader agent `FOR SHARE NOWAIT` → autopilot duplicate guard :765 → issue counter :773 → position :785 → INSERT | **No** — `ParentIssueID: pgtype.UUID{}` :806 |
| C10 | Onboarding bootstrap with runtime | `BootstrapOnboardingRuntime` `internal/handler/onboarding_shim.go:138` | S1 directly at :275 | The handler (:173; commit :320) | **Yes** — `internal/handler/onboarding_shim.go:529` via `lockOnboardingIssueCreate` (:180) | Workspace `FOR KEY SHARE` → subscriber → member → status catalogue → fence (:515-530) → issue counter :263 → INSERT | **No** — `ParentIssueID: emptyUUID` :286 |
| C11 | Onboarding bootstrap without runtime | `BootstrapOnboardingNoRuntime` `internal/handler/onboarding_shim.go:371` | S1 directly at :446 | The handler (:393; commit :481) | **Yes** — same helper (:400 → :529) | As C10 | **No** — `ParentIssueID: emptyUUID` :457 |

### Parent-changing writers on existing rows (P1–P5)

| ID | Entry point (route) | Function, file:line | sqlc query | Transaction owner | Fence | Row locks, in order | Effect on parent |
|---|---|---|---|---|---|---|---|
| P1 | `PUT /api/issues/{id}` (`router.go:2275`); `POST …/move` (`router.go:2287`, `internal/handler/issue_move.go:149` delegates to `UpdateIssue`, `parent_issue_id` allowed at :24); `POST /api/issues/batch-update` (`router.go:2271`) | `UpdateIssue` `internal/handler/issue.go:4163`; `BatchUpdateIssues` :5292 (per-item `updateIssueAtomically` :5644); `updateIssueAtomically` :3720 → `updateIssueAtomicallyOnce` :3936 | S3 at :4112 | `updateIssueAtomicallyOnce` (`h.TxStarter.Begin` :3940; commit :4154). Batch = one transaction **per item** | **Yes** — `internal/handler/issue.go:3947` (`lockIssueWriteFences`) | Fences → issue's iteration row `FOR UPDATE` then issue row `FOR UPDATE` (`lockIssueReadRows` :3971, or the attachment variant) → project `FOR SHARE NOWAIT` :4032 → new parent: `admission.Check` only :4040 (`internal/admission/admission.go:28-40`, plain `GetIssue`, **no lock**) → agent / task `FOR SHARE NOWAIT` (authorize) → `CheckStageChildUpdateInTx` :4057 (no row lock) → `UpdateIssue`. Neither the old nor the new parent row is explicitly locked | Set, replace, or clear (explicit `null`, :4351) |
| P2 | Issue delete (see D1) | Inside `deleteIssuesAndCollectAttachmentURLs` `internal/handler/issue.go:4843` | S4 at :4964 | Same transaction as D1 | **Yes** — :4877 | Children were locked `FOR UPDATE` by `LockIssueDeleteRows` :4891 before the detach | Clears parent and stage of every direct child that is not itself selected for deletion |
| P3 | `PATCH /api/collaboration/issues/{id}` (`router.go:1793`; body parsed at `internal/handler/collaboration_edit.go:63`) | `EditIssue` `internal/service/collaboration_edit.go:35` (operation kind `edit_source_structure` :38) | S5 at :116 | Collaboration workflow transaction (`collaboration_workflow.go:427-457`) | **Yes** — `internal/service/collaboration_workflow.go:513` | Workflow sequence (see C8), then `validateCollaborationParent` (`collaboration_source_fields.go:144-166`): each ancestor `FOR SHARE NOWAIT` via `LockCollaborationSourceParent` (`collaboration_source_fields.sql:18`), walking the whole chain | Set, replace, or clear — collaboration issues only (query requires an `issue_collaboration` row) |
| P4 | `DELETE /api/workspaces/{id}` | `DeleteWorkspace` `internal/handler/workspace.go:1081`, step at :1204 | S6 | `DeleteWorkspace` (`h.TxStarter.Begin` :1125) | **No** — no `iteration.LockWorkspace` and no `'iteration:'` string in `workspace.go` or `workspace_delete.sql` | Workspace row `FOR UPDATE` via `LockWorkspaceForDelete` :1146 (`pkg/db/queries/workspace.sql:65-80`) → every issue in the workspace `FOR UPDATE` via `LockWorkspaceTaskOwnerIssues` :1072 (`workspace_delete.sql:39-40`) | Clears every parent in the workspace in one statement |
| P5 | Database | FK `parent_issue_id UUID REFERENCES issue(id) ON DELETE SET NULL` (`migrations/001_init.up.sql:65`) | — | Whatever transaction deletes the parent | Not applicable | Implicit | Clears children's parent when a parent row is deleted while children still point at it |

Notes on P1:

- `updateIssueAtomically` has three callers: `UpdateIssue` (:4416), `BatchUpdateIssues` (:5644) and `internal/handler/issue_iteration_assignment.go:225`.
- When the request carries `current_iteration_id`, the transaction takes the `updateIssueIterationInTx` branch (`internal/handler/issue.go:3950-3958`; body `issue_iteration_assignment.go:241-300`) and commits there. That branch does not call `UpdateIssue`; it goes through `iteration.PrepareMembershipChange` / `CommitMembershipChange`. No parent write is reachable from it: S3 has a single caller at :4112 and no raw SQL writes `parent_issue_id`.
- `refreshUntouchedNullableIssueParams` (:4028) re-reads the locked row so an untouched parent keeps its current value.
- After commit, `PublishCollaborationChildrenChanged` runs (:4157-4158).

Notes on P4: the workspace row `FOR UPDATE` conflicts with the `FOR KEY SHARE` that every creation / update / delete writer above takes first (`pkg/db/queries/workspace.sql:82-91`). Teardown is therefore serialized against those writers by the workspace row, not by the fence. `DeleteWorkspace` refuses when `WorkspaceHasCollaborationRecords` is true.

### Delete writers (D1–D2)

| ID | Entry point | Function, file:line | sqlc query | Transaction owner | Fence | Row locks, in order |
|---|---|---|---|---|---|---|
| D1 | `DELETE /api/issues/{id}` (`router.go:2288`); `POST /api/issues/batch-delete` (`router.go:2272`) | `DeleteIssue` `internal/handler/issue.go:4783` → `deleteIssueAndCollectAttachmentURLs` :4839; `BatchDeleteIssues` :5748 (call at :5809) → `deleteIssuesAndCollectAttachmentURLs` :4843 | S7 at :5016 (plus S4 at :4964) | `deleteIssuesAndCollectAttachmentURLs` (`h.TxStarter.Begin` :4871; commit :5020). Batch delete = **one transaction for the whole batch** | **Yes** — :4877 | Fences → iterations `FOR UPDATE ORDER BY id` (:4884) → selected issues **and their direct children** `FOR UPDATE ORDER BY id` (`LockIssueDeleteRows` :4891; `pkg/db/queries/iteration.sql:97-101`) → authorization :4904 → collaboration-children refusal :4917-4925 → active tasks `FOR UPDATE OF task NOWAIT` (`LockIssueDeleteTasks` :4933) → task tokens NOWAIT → autopilot runs NOWAIT → quota rows → per issue: cancel tasks :4951, delete task tokens :4957, fail autopilot runs :4961, detach children :4964, attachment / source-context cleanup, `RecordIssueDeletion` :5013, `DeleteIssue` :5016 |
| D2 | `DELETE /api/workspaces/{id}` | `DeleteWorkspace` `internal/handler/workspace.go:1081`, step at :1300 | S8 | `DeleteWorkspace` (:1125) | **No** | As P4 |

The source comment at `internal/handler/issue.go:4907-4910` states the intent of the fence in D1: "The workspace iteration fence taken by lockIssueWriteFences is the one collaboration structure and creation writers also take, so no collaboration child can be attached to these parents until this transaction ends."

### Coverage statements, items 1–10

**1. HTTP create, update (reparent incl. batch), delete, batch delete — FOUND.** C1, P1, P2, D1.

**2. `service/issue.go` create paths and comment-to-child — FOUND.** One creation core (`Create` → `createOnce` → `CreateInTx`); callers of `IssueService.Create`: `internal/handler/issue.go:3199`, `internal/handler/source_context.go:746`, `internal/integrations/channel/engine/router.go:1085`. Callers of `IssueService.CreateInTx`: `internal/service/issue.go:315`, `internal/handler/lifecycle_handoff_transaction.go:405`, `internal/handler/triage.go:609`, `internal/service/collaboration.go:315`, `internal/service/collaboration_stage_actions.go:577`. Comment-to-child has two branches: manual (C2, creates the issue) and agent (`createAgentCommentSubIssue` `internal/handler/source_context.go:879` → `EnqueueQuickCreateTaskWithSourceContext` :887, which creates a task, not an issue).

**3. Autopilot `create_issue` — FOUND.** C9. It inserts directly and never sets a parent.

**4. Quick-create — FOUND; no dedicated issue writer.**

- Enqueue: `POST /api/issues/quick-create` (`router.go:2267`) → `QuickCreateIssue` (`internal/handler/issue.go:2618`; parent parsed :2797-2798; enqueue :2813) → `enqueueQuickCreateTask` (`internal/service/task.go:1648`). It runs `admission.Check` on the parent from the pool (:1649), stores `payload.ParentIssueID` (:1688-1690) and inserts the task with `CreateQuickCreateTask` (:1742 or `qtx.` :1772; `pkg/db/queries/agent.sql:459-489`, `issue_id` is the literal `NULL`). No issue row is written and the fence is not taken.
- Other enqueue callers: `internal/handler/source_context.go:887` (parent = comment's issue) and `internal/integrations/slack/slash_command.go:275` (passes `pgtype.UUID{}` for parent, :284).
- Context shape: `QuickCreateContext` (`internal/service/task.go:1571-1592`), field `ParentIssueID string` with JSON key `parent_issue_id`.
- Readers of `context.parent_issue_id`: `internal/handler/daemon.go:3142` (parse) and :3217-3243 (claim response: resolves the identifier, sets `resp.ParentIssueID`, drops it when the source-context parent is gone); `internal/daemon/prompt.go:367-368` (prompt tells the agent to pass `--parent`); `lock_issue_execution` in `migrations/535_triage_execution_fence.up.sql:31` and `migrations/650_collaboration_legacy_execution_fence.up.sql:24` (`owner_id := NULLIF(p_context->>'parent_issue_id', '')::uuid`); `internal/service/task.go:1792` (log line); `internal/service/task_knowledge.go:251` and :378 (parse the context for type / workspace / project only, in the lines read).
- Issue creation: the agent calls the CLI (`cmd/multica/cmd_issue.go:1216` sends `parent_issue_id`), which reaches C1 with `origin_type=quick_create` and `origin_id=<task id>` (`internal/handler/issue.go:3151-3153` allows only `quick_create` from a request). So the fence is taken at `internal/service/issue.go:395`.
- In `CreateInTx`'s quick-create branch (`internal/service/issue.go:582-633`) the origin task, creator agent, context type and workspace are checked. In the lines read there is **no comparison between the request's `ParentIssueID` and `QuickCreateContext.ParentIssueID`**: the new issue's parent is whatever the HTTP body carries.
- Link on completion: `notifyQuickCreateCompleted` (`internal/service/task.go:7568`, called at :4592 and :5241) → `GetIssueByOrigin` (:7579) → `LinkTaskToIssue` (:7625; `pkg/db/queries/agent.sql:530-544`). It runs on the pool with no transaction and no fence, and writes `agent_task_queue.issue_id`, not the issue row.
- Manual retry: `RetrySourceContextQuickCreate` (`internal/service/task.go:1812`) creates a successor task with `CreateManualQuickCreateRetryTask` (`agent.sql:662`), which copies `p.context` unchanged. No issue row is written.

**5. Onboarding / seed / template / import / clone / duplicate.**

- Onboarding: FOUND — C10, C11.
- Import: FOUND — triage CSV import, C6. No other importer calls S1 or S2.
- Clone / duplicate: PROVED NONEXISTENT in product Go — S1 and S2 have exactly the five call sites listed; `FindActiveDuplicateIssue` is a guard read.
- Template: PROVED NONEXISTENT as an issue writer, by the same caller enumeration.
- Seed: no product seed inserts issues. Non-product direct SQL exists: `scripts/seed-rich-test-data.mjs` (generic `INSERT INTO ${table}` at :458, `parent_issue_id` at :193; own transaction with `LOCK TABLE … IN SHARE ROW EXCLUSIVE MODE NOWAIT` :425 and workspace `FOR UPDATE` :408; no advisory fence); `e2e/fixtures.ts:292`; `scripts/seed-rich-test-data.test.mjs:99, 152`; Go test fixtures under `internal/testutil`.

**6. Collaboration structure writers that create child issues — FOUND.** C7 (collaboration issue, optionally under a parent), C8 (ordinary children materialized under a collaboration root), P3 (re-parent of a collaboration issue).

**7. Squad / delegation sub-issue writers — no dedicated writer.** Squad code writes issues only through `TransferSquadAssignees` (`pkg/db/queries/squad.sql:167`, assignee columns). Sub-issues from agents or squads arrive through C1 (agent CLI `--parent`) or C4 (handoff follow-up).

**8. Workspace teardown and direct SQL that nulls or rewrites parent — FOUND.** P4, D2, P5. The FK in `001_init.up.sql:65` is still declared; no migration drops it (grep for `DROP CONSTRAINT … fkey` matched only unrelated constraints in migrations 018, 096, 118, 152, 173, 186, 193).

**9. Cross-workspace move — PROVED NONEXISTENT.** No query or Go statement sets `issue.workspace_id` on an existing row. `MoveIssue` is a same-workspace field update (P1).

**10. SQL-side enumeration — done.** Table S1–S8 above.

Plugin public API: `PatchPluginIssue` (`internal/handler/plugin_action.go`, around :434) calls `IssueService.UpdateContent`, which reaches `UpdateIssueContentOnly` (title / description). `ParentIssueID` appears in plugin code only in response mapping (:601). It is not a parent writer.

### Contract re-verification

**R3 (delete cascades) — CHANGED in one respect.**

- Still true: the FK cascades are still declared. `issue.parent_issue_id … ON DELETE SET NULL` (`001_init.up.sql:65`); `agent_task_queue.issue_id … ON DELETE CASCADE` (`001_init.up.sql:130`); `task_usage.task_id`, `task_message.task_id`, `task_token.task_id … ON DELETE CASCADE` (`032_task_usage.up.sql:3`, `026_task_messages.up.sql:3`, `108_task_token.up.sql:21`). No dropping migration found.
- Still true: direct children are detached explicitly by `DetachDirectChildIssues` (`pkg/db/queries/issue.sql:132`) before the parent is deleted, and selected issues plus direct children are locked first.
- Changed: task cancellation and autopilot-run failure are **no longer committed outside and before the delete transaction**. They now run inside it: `qtx.CancelAgentTasksByIssue` (`internal/handler/issue.go:4951`) and `qtx.FailAutopilotRunsByIssue` (:4961), preceded by NOWAIT locks on tasks, task tokens and autopilot runs (:4933-4947, `pkg/db/queries/issue_delete.sql`). At `fb9f4c070` the calls were `h.TaskService.CancelTasksForIssue(…)` (`handler/issue.go:4786`) and `h.AutopilotService.FailAutopilotRunsByIssue(…)` (:4788; batch :5763-5764), outside the transaction. The change arrived with commit `5aee2f203`. The contract's `handler/issue.go:4786` reference no longer matches the code.

**R4 (cycle check outside the transaction, 10-level cap) — STILL TRUE for ordinary issues; line numbers moved.**

- Single update: pre-transaction checks at `internal/handler/issue.go:4315-4353`; self-parent rejection :4324; parent existence by pool read :4329; ancestor walk `for depth := 0; depth < 10; depth++` at :4338 (block :4336-4348, was :4322-4334), each step a pool `h.Queries.GetIssue`.
- Batch update: same checks per item at :5540-5586; loop at :5563 (block :5560-5577, was :5515-5531).
- Inside the transaction the only new-parent check is `admission.Check` (:4040), a lock-free read. The parent row is not locked and the cycle walk is not repeated.
- Creation reads the parent without a row lock at `internal/service/issue.go:451` (was :445).
- Addition not in the contract: collaboration issues have a separate walk, `validateCollaborationParent` (`internal/service/collaboration_source_fields.go:144-166`). It runs inside the workflow transaction, under the fence, is unbounded, uses a `seen` map for cycles and locks each ancestor `FOR SHARE NOWAIT`. It applies only to P3.

**R5 (which writers take the fence) — STILL TRUE; every previously unverified item is now resolved.**

- Fence definition unchanged at `internal/iteration/transaction.go:83`.
- HTTP create (`internal/service/issue.go:307`, :395), update (`internal/handler/issue.go:3947`), delete (:4877) and `StartTask` (`internal/service/task_start.go:47`) take it.
- `internal/service/task.go` contains no `iteration.LockWorkspace` call (grep), so claim, completion, failure and quick-create link do not take it.
- Autopilot `create_issue`: **takes it** (`internal/service/autopilot.go:710`).
- Onboarding: **takes it** (`internal/handler/onboarding_shim.go:529`).
- Collaboration creation, structure edit and stage-child materialization: **take it** (`internal/service/collaboration_creation_fields.go:79`; `internal/service/collaboration_workflow.go:513`; and `internal/service/issue.go:395` for the two that create).
- Lifecycle handoff and triage: **take it** (`internal/handler/lifecycle_handoff_transaction.go:275`; `internal/handler/triage.go:253`).
- Workspace teardown: **does not take it**.

Other transactions that take the fence but neither create an issue nor write a parent: `internal/handler/label.go:748`, `internal/handler/project.go:909`, `internal/handler/squad.go:620`, `internal/service/issue_public.go:66`, `internal/service/issue_status_write.go:52`, `internal/service/collaboration_stage_execution.go:150`, `internal/service/admin_collaboration.go:363`.

### Related Specs

- `.trellis/tasks/11-12-agent-squad-followup-roadmap/research/f3-minimum-budget-contract-20261113.md` — contract; §11 R3 / R4 / R5 re-verified above.
- `.trellis/tasks/11-12-agent-squad-followup-roadmap/research/f3-execution-source-map-20261113.md` — earlier source map at `fb9f4c070`; its line numbers are superseded by this file for the writers listed here.
- `.trellis/tasks/11-16-f3-parent-budget-root/{prd.md,design.md,implement.md}` — the task this inventory feeds.

## Writers that do NOT take the `iteration:<ws>` fence

Issue-row writers:

1. **Workspace teardown, bulk parent clear** — `PrepareWorkspaceDeletionLinks`, `internal/handler/workspace.go:1204`. Serialized against other issue writers by the workspace row `FOR UPDATE` (:1146) and an all-issues `FOR UPDATE` (:1072), not by the fence.
2. **Workspace teardown, bulk issue delete** — `DeleteWorkspaceIssueRoots`, `internal/handler/workspace.go:1300`. Same transaction as item 1.
3. **Foreign key `ON DELETE SET NULL`** on `issue.parent_issue_id` (`migrations/001_init.up.sql:65`). A database action, outside application code. In the ordinary delete path direct children are already detached by `DetachDirectChildIssues` before the parent row is deleted, inside a fenced transaction; in workspace teardown parents are cleared by item 1 first.
4. **Non-product direct SQL** — `scripts/seed-rich-test-data.mjs`, `e2e/fixtures.ts:292`, `scripts/seed-rich-test-data.test.mjs`, Go fixtures under `internal/testutil`.

Not issue-row writers, but they carry or bind the parent relation without the fence:

5. **Quick-create enqueue** — `internal/service/task.go:1648` records `context.parent_issue_id` on a task row; the parent is checked by a pool-level `admission.Check` (:1649).
6. **Quick-create manual retry** — `internal/service/task.go:1812` copies the same context to a successor task.
7. **`LinkTaskToIssue`** — `internal/service/task.go:7625`, pool-level, no transaction, best-effort; writes `agent_task_queue.issue_id`.
8. **Pre-transaction reparent reads** — the parent-existence read and the 10-level cycle walk (`internal/handler/issue.go:4329`, :4338, :5563) run on the pool before the fenced transaction opens.

## Caveats / Not Verified

1. **Deployed schema.** Only migration files were read. Whether a given database still has the foreign keys is not proved by the files (a conditionally skipped migration is still recorded in the ledger).
2. **Implicit FK row locks.** That inserting or updating a non-null `parent_issue_id` takes `FOR KEY SHARE` on the parent row is inferred from the declared FK and the comment at `pkg/db/queries/lifecycle.sql:7`; it was not observed.
3. **All concurrency behaviour.** Nothing was executed. Lock order and conflict statements come from reading the statements in source order.
4. **`parseIssueIterationUpdateFields`** was not read. Whether a request that carries both `current_iteration_id` and `parent_issue_id` is rejected or has the parent ignored is unverified. What is verified is that the iteration branch cannot write a parent.
5. **`iteration.PrepareMembershipChange` / `CommitMembershipChange`, `loadStageRoot`, `BumpCollaborationStageChildSemanticVersion`** bodies were not read. Their not writing `parent_issue_id` rests on the SQL enumeration (four queries write it, each with a single caller).
6. **`internal/handler/lifecycle_handoff_transaction.go:247-254`** (where the transaction is begun) and everything after :410 were not read.
7. **`internal/handler/collaboration_creation.go`** was read only around :92-109; **`internal/handler/collaboration_edit.go`** only around :63.
8. **`PatchPluginIssue`** was checked by grep on its body (single `IssueService.UpdateContent` call), not read line by line.
9. **`internal/service/task_knowledge.go`** was read only at :245-256 and :372-382.
10. **`attributionbackfill`, `chatoriginbackfill`, `taskusagebackfill`** were checked by grep for the `issue` table name only.
11. **Database functions.** `lock_issue_execution` and `lock_task_owner_rows` were read only where they mention `parent_issue_id`; a full read of every plpgsql function body in `migrations/` for writes to `issue` was not done. The grep for `parent_issue_id` across `migrations/` found no write.
12. **Channel router** (`internal/integrations/channel/engine/router.go`) was read at :1052-1085 only; that no other channel path sets a parent rests on the caller enumeration of `IssueService.Create`.
13. **Daemon, CLI and MCP clients** were treated as HTTP clients of the routes above. `cmd/multica/cmd_issue.go` was read at the three lines cited only.
