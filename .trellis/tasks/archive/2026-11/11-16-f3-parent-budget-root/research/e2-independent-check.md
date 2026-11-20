# E2 independent check — F3 parent budget root, step 1

Reviewed: `e2-writer-inventory.md`, `e2-lock-graph.md`, and the V1–V10 revisions in `design.md` §7 that derive from them.

Source base: clean worktree `/Volumes/artisan/code/2026/mc-f3-budget-root-20261116`, HEAD `86fea651cc5d09360327b6cf0f8357c1f360070d`. Tracked files were clean for the whole review (an untracked `server/internal/budget/` appeared during it; it is step 2's package and was not read). All paths below are relative to `server/` unless they start with `e2e/` or `scripts/`.

Method: static reading, `grep` and `git` read commands only. Nothing was built, run or executed against a database. Each statement below is marked **read** (the cited lines were read and say this) or **inferred** (reasoned from what was read; not observed).

## Summary

| # | Severity | Area | One line |
|---|---|---|---|
| F1 | should fix before step 5 | lock graph §1, §8; V6, V8 | The proposed finalize position (between `task.go:3831` and `:3832`) is not the last lock-taking statement. The root lock can be last; it has to go after the receipt UPDATE, which means restructuring the early return at `:3845`. |
| F2 | should fix before step 4 | lock graph §8 P10; design §5 | "BR lock (last)" on the issue side is asserted, not derived. Every create path keeps executing blocking statements after any binding point inside `CreateInTx`. No cycle found, but the rule as written is not what the code allows. |
| F3 | should fix before step 5 | lock graph §8 | Member read then root lock has a window: an ancestor re-parent can move the task's issue to another root between the two. Finalize's issue lock covers only the task's own issue. |
| F4 | should fix (wording of V7/V8) | lock graph §8; V7 | The cycle exception is wider than `task_usage`. Any lock taken after the root lock that finalize takes before it closes the same cycle; the `"user"` row and the installation namespace lock are the concrete ones. |
| F5 | should fix before step 3 | neither document | A schema-coverage test requires every public table to be classified for workspace teardown; a retained table must carry `workspace_id`. Five new tables will fail it unless registered. |
| F6 | should fix (fact wrong, conclusion survives) | inventory line 95 | "`CREATE TRIGGER … ON issue`: none" is false. Two live triggers exist. Neither writes `issue`, but both take row locks the lock graph does not list. |
| F7 | should fix before step 3 (cardinality) | inventory §quick-create; V4, V10 | One quick-create task can create any number of issues, all with the same origin, and the completion link picks one arbitrarily. `origin_id` is not checked against the caller's own task. |
| F8 | note | V4, V10 | Deleting the quick-create parent cancels nothing and blocks nothing: the task still finalizes, and the payload silently drops the parent. |
| F9 | note | V5 | The ordinary cycle check runs before the write transaction, on the pool. Parent cycles can therefore exist in the data; every descendant walk must tolerate them, not only root enabling. |
| F10 | note | inventory P5 | A second database-level deleter is not listed: `issue.workspace_id … ON DELETE CASCADE`. |
| F11 | note | design V4 | "`ParentIssueID` 只用于提示词" contradicts the inventory (line 186) and V10. |
| F12 | note | lock graph §8 | Two inversions exist today without F3: finalize versus teardown (acknowledged in code), and namespace versus `"user"` between finalize and daemon registration (inferred, reachability not established). |
| F13 | note | inventory "searches that returned nothing" | Non-product writers bypass the service: e2e fixtures and the Go test fixture helper insert issues and set parents with raw SQL. |
| F14 | note | lock graph caveats | Three items the lock graph left unread are now read; none changes its conclusions. |

Nothing found blocks step 3 outright. F5 and F7 change what step 3 must produce; F1–F4 change the lock rule steps 4–6 are built on.

## Findings

### F1 — The finalize position is not last; it can be (should fix before step 5)

**Claim.** Lock graph §1: the position is "between `internal/service/task.go:3831` and `:3832`", and "Statements that still execute after that position … None of them waits on a task row other than the one already held". Design V8 defers the decision: put the reservation after everything between `CreateTaskToken` and commit "除非那里有必须排在预留之后的取锁".

**What the source says (read, `internal/service/task.go:3734–3866` in full).** After `:3831` the callback executes, in order:

| Line | Statement | Locks it takes |
|---|---|---|
| `:3832` | `CreateTaskToken` (`pkg/db/queries/task_token.sql:1–4`, plain INSERT) | FK checks on `agent_task_queue`, `agent`, `workspace`, `"user"` (`migrations/108_task_token.up.sql:21–24`) — blocking FOR KEY SHARE (**inferred**; standard FK behaviour, not observed) |
| `:3838` | `DeleteExpiredDaemonTokens` (`pkg/db/queries/daemon_token.sql:23–25`) | `DELETE FROM daemon_token WHERE expires_at <= now()` — global, not scoped to the workspace, unordered, blocking row locks. Only when one daemon token is passed |
| `:3841` | `CreateDaemonToken` (`daemon_token.sql:1–4`) | FK on `workspace` (`migrations/029_daemon_token.up.sql:4`) |
| `:3845–3847` | `if !recordCommentReceipt { return nil }` | — |
| `:3848` | `SetTaskDeliveredCommentIDs` (`pkg/db/queries/agent.sql:919–944`) | UPDATE on the held task row; WHERE calls `lock_issue_execution` → issue FOR KEY SHARE NOWAIT |

Then `runInTx` commits (`task.go:6547–6567`).

Nothing after `CreateTaskToken` forces the reservation to come first:

- No later statement reads anything the reservation writes (**read**).
- No deferred constraint exists anywhere: `grep -i deferrable migrations/*.up.sql` is empty (**read**).
- No trigger fires on the two task-row UPDATEs in this transaction. Triggers on `agent_task_queue` are column-scoped to `status`, `runtime_id`, `issue_id`, `dispatched_at` or are INSERT/DELETE triggers (`migrations/128_agent_task_queue_runtime_mcp_overlay.up.sql:50–51`, `243:7–11`, `494:12–13`, `500:23–24`); the finalize UPDATEs write `knowledge_context_version`, `knowledge_delivery` and `delivered_comment_ids` (**read**). There is no trigger on `task_token` or `daemon_token` (**read**, grep).

**Where the root lock can go to be last.** After the `SetTaskDeliveredCommentIDs` block, immediately before the callback returns. That requires turning the early `return nil` at `:3845–3847` into a conditional around `:3848–3859`, so both branches reach the budget write. At that point the transaction holds every lock it will ever hold, and the root lock is followed only by FK-free budget inserts and the commit.

**Why it matters.** With the root lock before `:3832`, finalize holds it while waiting on the agent row (held FOR UPDATE by `claimTask`), the `"user"` row, the workspace row (held FOR UPDATE by teardown for up to its 10 s `lock_timeout`, `internal/handler/workspace.go:792`) and other transactions' expired daemon-token rows. I could not build a cycle through those waits, so this is contention on the root row, not a deadlock — but it also puts the root row inside the existing finalize/teardown deadlock window (F12).

**Three practical constraints for step 5 (read).**

- The callback receives only `*db.Queries` (`task.go:6558–6563`), not the `pgx.Tx`. The budget write has to be sqlc queries on `qtx`, or `runInTx` has to change.
- `runInTx` runs the callback with no transaction when `s.TxStarter == nil` (`:6548–6553`). `FinalizeTaskClaim` already refuses that for capability tokens (`:3743–3745`); the budget write needs the same refusal.
- The launch payload is built before finalize, outside any transaction, and the handlers patch the token in afterwards (`internal/handler/daemon.go:1933`, `:3690`). `budget_attempt_id` has to travel the same way at both call sites (`:1911`, `:3663`): generated before the transaction and passed in, or returned from it.

### F2 — "BR last" on the issue side is asserted, not derived (should fix before step 4)

**Claim.** Lock graph §8: "P10 … P3 prefix … → I FOR UPDATE → BM write → BR lock (last)", and the cycle argument rests on "as the final lock in P10". Design §5: "根行锁在所有路径中排最后".

**What the source says (read).** P10 is described "as in the task brief"; no transaction was checked for what follows the binding point. In every create path the owning transaction continues after the issue row exists:

- Inside `CreateInTx` after the INSERT (`internal/service/issue.go:532`/`:554`): source-context persist or quick-create attach (`:578–633`), label attach (`:642`).
- `createOnce` after `CreateInTx` (`:315`): `AfterCreateInTx` hook (`:319`), deferred channel task insert (`:331`), commit `:338`.
- Handoff after `CreateInTx` (`internal/handler/lifecycle_handoff_transaction.go:405`): child metadata UPDATE (`:414`), source metadata UPDATE (`:453`), `CreateComment` (`:461`), commit `:476`.
- Stage children (`internal/service/collaboration_stage_actions.go:577`): a loop — the second child's `CreateInTx` re-runs the counter UPDATE on the workspace row, the project NOWAIT lock and the duplicate guard after the first child's binding. The whole loop sits in a savepoint that is rolled back on a business refusal (`:570`).
- Triage after `CreateInTx` (`internal/handler/triage.go:609`): raw `UPDATE issue SET admission_status='pending'` (`:614`) and the triage rows.
- Collaboration create after `CreateInTx` (`internal/service/collaboration.go:315`): `CreateIssueCollaboration` (`:322`) and the rest of the creation.

On the update path, after `UpdateIssue` (`internal/handler/issue.go:4112`): `LinkAttachmentsToIssue` `:4119`, `RecordIssueChange` `:4139`, `FinishDeliveryMutationInTx` `:4145` (project lock NOWAIT, `internal/service/delivery_completion.go:138`), `FinishStageMutationInTx` `:4151` (`issue_collaboration` FOR UPDATE; own issue FOR NO KEY UPDATE NOWAIT), commit `:4154`. On the delete path a tombstone placed after the NOWAIT block (`:4933–4947`) is followed by cancel, token delete, detach, attachment and source-context cleanup, `RecordIssueDeletion` `:5013` and `DeleteIssue` `:5016` with its cascades and the trigger in F6.

**What I tried.** For a cycle, a statement after the root lock must wait on something a root-waiter holds. Finalize, while waiting for the root, holds: `"user"` FOR UPDATE (password mode), the namespace advisory lock, `managed_installation` FOR SHARE, the task row FOR UPDATE, the issue row FOR KEY SHARE, and the restricted-profile NOWAIT locks. In the statements listed above I found no blocking wait on a task row, no FOR UPDATE on an issue row other than ones the transaction already holds or just created, and no write referencing `"user"` by FK (the FK list to `"user"` in `migrations/` ends at `109`; none of those tables is written by an issue transaction). So **no cycle found** (**inferred** from the statements read; the hooks `AfterCreateInTx` and `PrepareInTx` are caller-supplied and were read only for the HTTP create path).

**Correction.** The lock graph should state one of these, not "(last)":

1. The member write does not lock the root at all. Member rows are already serialized per workspace by `iteration:<ws>`, which every member writer holds (F-table in "Checked and correct"); the root row is then locked only by finalize, the usage entry and the admin API. This removes P10 from the root's lock graph entirely.
2. The member write locks the root mid-transaction, with the proven rule "after the root lock, no blocking wait on a task row and no FOR UPDATE on an issue row not already held", re-checked per path when step 4 picks the binding point.

Two related rules the lock graph does not state:

- A re-parent that moves an issue between two enabled roots touches two root rows. Only fenced paths can do that, and the fence is workspace-exclusive, so two such writers never interleave today (**inferred**). The rule "more than one root is locked only under the fence, in id order" should be written down so the admin API does not become the exception.
- The admin "enable root" path walks the tree and writes member rows; it has to take the same fence before the root, or it races every create and re-parent.

### F3 — Member read then root lock leaves a window (should fix before step 5)

**Claim.** Lock graph §8, proposed positions: "Member-row read … Root-row lock: immediately after the member read".

**What the source says.** Finalize holds FOR KEY SHARE on the task's own issue from `task.go:3819` (for quick-create, on the context parent — `migrations/650_collaboration_legacy_execution_fence.up.sql:20–44`). A re-parent of that issue needs it FOR UPDATE (`internal/handler/issue.go:3903–3906`), so the issue's own parent cannot change before finalize commits (**read**). An ancestor is not locked. Re-parenting an ancestor changes the root of every descendant, including the one finalize is working on, and that writer never touches the descendant's issue row (**inferred** — it depends on how step 4 rewrites descendant member rows).

**Consequence.** Finalize can read member → root R1, lose the race, lock R1 and write an attempt against a root the issue no longer belongs to.

**Correction.** Either take a share lock on the member row when reading it (order member → root in finalize; a re-parent holding the member row for write makes finalize wait before it holds any root, so no cycle), or re-read the member row after the root lock and retry on mismatch. The lock graph should name the member row as a lock, not a read.

### F4 — The cycle exception is wider than `task_usage` (should fix: wording of V7/V8)

**Claim.** Lock graph §8: "The one path read that would close a cycle" is the usage transaction locking the root and then writing `task_usage`. V7 turns that into "只写不带 FK 的预算表 … 不锁 task 或 issue 行".

**What the source says (read).** Before it reaches the root, finalize takes, in order: `"user"` FOR UPDATE in password mode (`task.go:3764`, and again via `LockManagedRuntime`, `internal/service/managed_runtime.go:125–129`); for a bound daemon-token session `runInTx` takes `"user"` FOR UPDATE and the namespace advisory lock before the callback (`task.go:6560`, `managed_runtime.go:174–191`); the namespace advisory lock (`managed_runtime.go:130–134`); `managed_installation` FOR SHARE (`:148`); the task row; the issue row.

So an edge from the root to any of those closes the same cycle as root → task. The concrete risk: a usage entry or admin handler that locks the root and then calls a helper that runs `auth.LockPasswordSession` or `LockManagedRuntime` — both are the normal way existing daemon-facing writers start.

**Correction.** State the rule as "nothing is acquired after the root lock except FK-free budget rows", and add: in the usage transaction, session, namespace and installation locks come first (use `runInTx`'s prefix), never after the root.

Two smaller points on the same claim:

- The FK lock on `task_usage` is taken on the insert path only. `UpsertTaskUsage` on an existing `(task_id, provider, model)` row updates without changing the FK column, so no FK check runs (**inferred**). The cycle is still reachable on a first report.
- `ReportTaskUsage` (`internal/handler/daemon.go:4676–4748`) is as the lock graph describes: no transaction, no status or generation check, per-row warn, 200 (**read**).

### F5 — Workspace-teardown manifest guard (should fix before step 3)

**Claim.** None. Neither document, nor `implement.md`'s step-3 conventions, mentions it.

**What the source says (read).** `internal/handler/workspace_delete_manifest_test.go:397` `TestWorkspaceDeletionManifestCoversPublicSchema` compares every public table against `workspaceDeletionManifest` (`:29–`) and fails on any unclassified table (`:425–440`). Actions are `delete`, `detach`, `keep`, `settle`, `retain_audit` (`:18–24`). A `retain_audit` table must have a `workspace_id` column and a `keep` table must not (`:481–491`); the one exception is hard-coded for `delivery_completion_terminal`.

**Consequence for the schema.** V3 says budget rows survive teardown as tombstones. That is the `retain_audit` class, and it requires `workspace_id` on each of the five tables — including the entries and receipts tables, where the design might otherwise reach the workspace only through the root. The manifest comment (`:30–31`) matches V3's read rule: "Public reads still require a live workspace and current authority; retention grants no access."

The test is DB-backed, so it skips silently without a database and fails only in `make check`.

### F6 — Triggers on `issue` exist (should fix: fact wrong, conclusion survives)

**Claim.** Inventory line 95: "`CREATE TRIGGER … ON issue` in `migrations/`: none."

**What the source says (read).** Two live triggers:

- `trg_issue_delete_dirty_hourly BEFORE DELETE ON issue FOR EACH ROW WHEN (current_setting('multica.workspace_teardown', true) IS DISTINCT FROM 'on')` — `migrations/243_workspace_teardown_dirty_trigger_guard.up.sql:13–18` (first created `102_task_usage_hourly_pipeline.up.sql:169–170`).
- `trg_issue_project_dirty_hourly BEFORE UPDATE OF project_id ON issue` — `102:226–227`.

The 084 dashboard triggers on `issue` were dropped by `103_drop_legacy_daily_rollups.up.sql:116–119`. The claim's grep was single-line; these statements span lines.

**What survives.** Neither function writes `issue` (`102:142–167`, `:176–224`), so "no trigger inserts an issue or writes a parent" holds.

**What changes.** Both functions run `INSERT INTO task_usage_hourly_dirty … ON CONFLICT … DO UPDATE` (`102:147–163`, `:183–219`), which takes blocking row locks on dirty-bucket rows inside issue delete and inside any project change — including the raw project UPDATEs in triage (`internal/handler/triage_actions.go:269`, `:509`). That is a lock family missing from the lock graph's P3 and P5. I found no cycle through it: the other holder is the hourly rollup under advisory lock 4246, which takes no issue, task or root lock (**inferred**; the rollup function was read only for its lock statements). Teardown switches the delete trigger off (`pkg/db/queries/workspace_delete.sql:5–8`).

### F7 — One quick-create task can create many issues (should fix before step 3: cardinality)

**Claim.** Inventory and V4: the quick-create issue goes through the ordinary HTTP create with `origin_type=quick_create`, and the server does not compare the request's parent with `QuickCreateContext.ParentIssueID`. V10: the completion link is best-effort.

**Confirmed (read).** `internal/handler/issue.go:3144–3164` accepts only `quick_create` as a caller-supplied origin; the create goes through `IssueService.Create` at `:3199` with the request's parent at `:3209`. `internal/service/issue.go:582–633` checks creator agent against the task's agent, context type, workspace and source-context attach; it never reads `ParentIssueID`.

**Not in either document (read).**

- The agent CLI stamps every `issue create` in the run: `cmd/multica/cmd_issue.go:1257–1258` sets `origin_type`/`origin_id` whenever `MULTICA_QUICK_CREATE_TASK_ID` is set, which the daemon does for the whole task (`internal/daemon/daemon.go:7656`).
- Nothing limits it to one. The origin index is non-unique (`migrations/042_autopilot.up.sql:77`), and the quick-create branch has no task-status or prior-issue check. With a captured source context the second create fails on the attach; without one there is no limit.
- `origin_id` is not required to be the caller's own task. The branch loads the task named in the request and checks only that its agent is the creator, so an agent can stamp any of its own quick-create tasks, including finished ones.
- An agent that omits the origin gets `agent_create` plus its actor task id stamped instead (`internal/handler/issue.go:3353–3360`), also unlimited.
- The completion link reads `GetIssueByOrigin` (`pkg/db/queries/issue.sql:532–542`): `LIMIT 1` with no `ORDER BY`. With several issues it links an arbitrary one.
- A failed quick-create task gets a retry task with a new id and the same context (`internal/service/task.go:5055–5062`), so one user request can span several tasks, each able to create issues.

**Consequence.** The discrepancy observation in V4 is per created issue, not per attempt; the schema must not put a unique key on (attempt) for it, and B55 should cover zero, one and several children. The actor's task id (`X-Task-ID`, set by the middleware for every `mat_` request) is available on every agent create and is a sounder attribution handle than the caller-supplied origin.

### F8 — Deleting the quick-create parent stops nothing (note)

**Read.** Issue delete locks and cancels tasks by `task.issue_id` (`pkg/db/queries/issue_delete.sql:1–5`; `agent.sql:715–718`). A quick-create task has `issue_id NULL`, so it is neither locked nor cancelled when its context parent is deleted. At finalize, `lock_issue_execution` skips a parent that no longer exists and returns true (`migrations/650:27–29`, `:41–44`). The claim payload then drops the parent (`internal/handler/daemon.go:3241–3243`), so the agent files a top-level issue.

**Consequence.** An attempt attributed to `context.parent_issue_id` can start after that issue's member row is a tombstone, and its child lands under no root. V4's "记一条差异观察" covers the mismatch; B55/B56 should include this ordering explicitly.

### F9 — Parent cycles can exist in the data (note)

**Read.** The ordinary cycle check walks at most 10 ancestors with `h.Queries` — the pool, before the write transaction begins — at `internal/handler/issue.go:4338` (single) and `:5563` (batch). The write transaction (`:3936–4156`) does not repeat it; it only runs `admission.Check` on the new parent (`:4040`). Two concurrent re-parents (A under B, B under A) each pass their check and then commit one after the other under the fence. Chains deeper than 10 are not checked at all. The collaboration writer does it properly, in-transaction and unbounded (`internal/service/collaboration_source_fields.go:145–166`).

**Consequence.** V5 gives the root-enabling walk its own cycle detection. The same is needed for any walk step 4 adds — in particular rewriting descendants' member rows on a re-parent.

### F10 — Unlisted database-level deleter (note)

**Read.** `migrations/001_init.up.sql:53`: `workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE`. The inventory lists one database-level writer (P5, the parent FK). There is exactly one `DELETE FROM workspace` (`pkg/db/queries/workspace.sql:244`), called at `internal/handler/workspace.go:1365` after `DeleteWorkspaceIssueRoots` (`:1300`), so the cascade is a backstop with nothing left to delete. No design impact; the inventory's count of database-level writers should be two.

### F11 — V4 wording (note)

Design V4 says `QuickCreateContext.ParentIssueID` "只用于提示词". The inventory itself (line 186) and V10 say otherwise, and the source agrees with them (**read**): it is admission-checked at enqueue (`internal/service/task.go:1649`), locked FOR KEY SHARE NOWAIT and admission-checked at enqueue, claim and finalize through `lock_issue_execution`, compared byte-wise in the identity CAS (`:3806`), and sent in the claim payload. The accurate statement is "not compared with the created issue's parent".

### F12 — Inversions that exist today without F3 (note)

- **Finalize versus teardown (read; acknowledged in code).** Teardown holds the workspace row FOR UPDATE (`internal/handler/workspace.go:1146`) and then takes every agent, issue and runtime row in the workspace FOR UPDATE (`pkg/db/queries/workspace_delete.sql:37`, `:40`, `:43`). Finalize holds the issue row FOR KEY SHARE and then waits on the workspace row at the token insert. The comment at `workspace.go:1064–1066` says so: "taking these locks can deadlock against a concurrent task update that locks in the opposite order … both outcomes surface as a retryable 503." F3 does not add an edge as long as teardown never locks a root (V3, V8 — confirmed: no fence, no budget table in the teardown step list `workspace.go:1195–1365`). With the root lock placed last (F1) the root row is never held inside this window.
- **Namespace versus `"user"` (inferred; reachability not established).** Outside password mode, finalize takes the namespace advisory lock (`managed_runtime.go:130–134`) and later a FK KEY SHARE on the token's user. Daemon registration takes `"user"` FOR UPDATE and then the same namespace lock (`managed_runtime.go:96–103`; callers `internal/handler/daemon.go:400`, `:438`). If the runtime owner is the token's user and both run at once, that is a cycle. I did not establish that the two can overlap in practice. Not caused by F3; reported because it was found.

### F13 — Non-product writers (note)

**Read.** Raw `INSERT INTO issue` in `e2e/fixtures.ts:292`; raw `UPDATE issue SET parent_issue_id` in `e2e/projects-p1-risk.spec.ts:23` and `e2e/fixtures/project-p1-health.ts:54`; the Go fixture helper inserts through a generic builder (`internal/testutil/db.go:52`, `:191`). The inventory's "none" is scoped to non-test Go and is correct for that scope. Tests for B60 that build a tree with these helpers will have children with no member row; they need to go through the service or write the member rows themselves.

### F14 — Items the lock graph left unread, now read (note)

- **`finalizeTaskKnowledgeClaim` remainder** (`internal/service/task_knowledge.go:629–684`, read in full). After `:674` there are two plain reads (`:658`, `:662`) and one write, `SetTaskKnowledgeDelivery` at `:682` (`pkg/db/queries/task_knowledge.sql:16–19`): an UPDATE on the task row already held FOR UPDATE, no function in its WHERE, no trigger fired. No new lock. The function's own comment (`:627–628`) says the same.
- **`updateIssueAtomicallyOnce` beyond `:4140`** (read to `:4160`). See F2: project NOWAIT, `issue_collaboration` FOR UPDATE, own issue NOWAIT, commit `:4154`, publish `:4158`. No task-row lock.
- **`failTask` after `:5114`** (read to `:5137`). `CreateRetryTask` `:5055`, source-context transfer, `CopyChannelTaskDelivery`, `ReleaseOnboardingKickoffFromTask` `:5122`, chat message insert. No root involvement by design (R10); no blocking wait on another task row.
- **Password-mode locking on issue write routes.** Every call site of `LockPasswordSession` / `LockPasswordUser` outside tests was listed by grep (22 matching lines in 12 files, definitions and generated code included); none is in the issue handlers, the issue service or the auth middleware. Issue writers take no `"user"` row lock.
- **Later redefinition of `lock_issue_execution`.** It is defined in migrations `535` and `650` only; `650` is the current definition. (`284` defines `lock_task_owner_rows`, a different function.)

## Checked and found correct

### Writer inventory — completeness (item 1)

Enumerated independently from the SQL outward: every statement in `pkg/db/queries/*.sql`, every function, trigger and rule in `migrations/*.up.sql`, and every Go string literal that can insert into `issue`, set `parent_issue_id` or delete from `issue`; then every caller.

- The eight queries S1–S8 are the complete set, at the cited lines (**read**).
- All Go call sites match: `CreateIssue` ×3, `CreateIssueWithOrigin` ×2, `UpdateIssue` ×1 (`internal/handler/issue.go:4112`), `DetachDirectChildIssues` ×1 (`:4964`), `UpdateCollaborationIssueStructure` ×1 (`internal/service/collaboration_edit.go:116`), `PrepareWorkspaceDeletionLinks` ×1, `DeleteIssue` ×1 (`:5016`), `DeleteWorkspaceIssueRoots` ×1 (**read**).
- No raw Go SQL inserts an issue or writes a parent; the raw `UPDATE issue` statements in triage write admission, status, project and assignee only (`internal/handler/triage.go:614`; `triage_actions.go:269`, `:479`, `:509`, `:518`) (**read**).
- No interface, wrapper or method value reaches the writers by another route; the seven `IssueCreateParams` constructions and all `CreateInTx` callers are as listed (**read**).
- Migration-level writes to `issue`: only the backfill at `020:24`; no rule; no function that inserts or re-parents (**read**).
- No migration drops the parent FK, the `task_usage` FK or the `task_token` FKs (**read**, sweep of `DROP CONSTRAINT`).
- `cmd/` backfills do not write parents (**read**, grep).

Missing from theirs: the two triggers (F6), the workspace cascade (F10), the non-product writers (F13). Found by me and not by them: nothing else. Something they found that I could not: nothing.

### Fence claims (item 2)

Every path below takes `iteration:<ws>` (`internal/iteration/transaction.go:83`, `:89–95`) on the same transaction that performs the write, before the write (**read** for each):

| Path | Transaction begins | Fence | Write |
|---|---|---|---|
| C1 HTTP create | `service/issue.go:278` | `:307` (when `PrepareInTx` is set) and `:395` (always) | `:532` / `:554` |
| C2 comment → sub-issue | via `IssueService.Create`, `handler/source_context.go:746` | `service/issue.go:395` | same; parent at `source_context.go:749` |
| C3 channel | via `IssueService.Create` | `service/issue.go:395` | no parent |
| C4 handoff | `handler/lifecycle_handoff_transaction.go:249` | `:275`, again `issue.go:395` | `:405`; parent at `:398` |
| C5/C6 triage, import | `handler/triage.go:219` | `:253` | `:609` (import: inside a savepoint of the same transaction, `triage_import.go:439–443`); no parent |
| C7 collaboration create | `service/collaboration.go:165` | `collaboration_creation_fields.go:79`, again `issue.go:395` | `collaboration.go:315`; parent at `:319` |
| C8 stage children | `service/collaboration_workflow.go:428` | `:513` | `collaboration_stage_actions.go:577`; parent at `:579` |
| C9 autopilot | `service/autopilot.go:681` | `:710` | `:790`; parent empty `:806` |
| C10/C11 onboarding | `handler/onboarding_shim.go:173`, `:393` | `:529` via `:180`, `:400` | `:275`, `:446`; parent empty `:286` |
| P1 update / move / batch | `handler/issue.go:3940` | `:3947` | `:4112` |
| P2 detach + D1 delete | `:4871` | `:4877` | `:4964`, `:5016` |
| P3 collaboration edit | `collaboration_workflow.go:428` | `:513` | `collaboration_edit.go:116` |

Workspace teardown takes no fence, as stated (**read**: no fence call in `workspace.go` or `workspace_delete.sql`; it relies on the workspace row FOR UPDATE at `:1146`). The iteration branch of the update transaction writes no parent (**read**, `handler/issue_iteration_assignment.go:241–300`).

### Lock graph (items 3, 4)

- `FinalizeTaskClaim` statement order and every line number in the §1 table (`:3764`, `:3794`, `:3798`, `:3802`, `:3806`, `:3810`, `:3819`, `:3826`, `:3829`, `:3832`, `:3838`, `:3841`, `:3845`, `:3848`) (**read**).
- `finalizeTaskCapability` (`internal/service/task_capability.go:17–58`, read in full): ordinary agents return at `:22–28` with no lock; restricted profiles take the NOWAIT locks at `:32` and `:35` (`task_token.sql:16–35`).
- Exactly two callers of `FinalizeTaskClaim`: `internal/handler/daemon.go:1911`, `:3663` (**read**).
- `lock_issue_execution` definition and lock statement, `migrations/650:6–11`, `:41–44`, `:57–58` (**read**).
- Opposite task/issue orders are both NOWAIT on the second lock: `agent.sql:2806–2809`, `issue_delete.sql:1–5`, `internal/handler/issue.go:4930–4933` (**read**).
- R3 has changed as V1 says: `CancelAgentTasksByIssue` `:4951`, `DeleteTaskTokensByTask` `:4957`, `FailAutopilotRunsByIssue` `:4961` are inside the delete transaction, after the NOWAIT locks, commit `:5020` (**read**).
- The P11 cycle is real as described (modulo F4).

### Citations spot-checked (item 5)

About 95 file:line citations were checked across the two documents, weighted to V1–V10. Mismatches: one factual claim (F6) and no wrong line number. One citation is ambiguous but resolves correctly: the lock graph's `migrations/029:4` — three files share prefix `029`; the intended one is `029_daemon_token.up.sql:4`.

By revision: V1 — 7 checked; V2 — 38 (table above plus the S1–S8 lines); V3 — 6 (`workspace.go:1146`, `:1204`, `:1300`, `:1365`; `workspace.sql:65–80`; `workspace_delete.sql:39–40`); V4 — 3; V5 — 3 (`issue.go:4338`, `:5563`; `collaboration_source_fields.go:144–166`); V6 — 8; V7 — 2 (`032_task_usage.up.sql:3`; `daemon.go:4676–4748`); V8 — 2 (`108_task_token.up.sql:21–24`; `029_daemon_token.up.sql:4`); V9 — 1 (`internal/daemon/knowledge_context.go:28–46`); V10 — 5 (`task.go:7568`, `:7579`, `:7625–7634`, `:4591–4593`; `task_token.sql:16–35`). Others: `actor_guards.go:96–104`, `:110–120`; `task_start.go:47`, `:66`; `task.go:1648–1649`, `:1688–1690`, `:6547–6567`; `managed_runtime.go:123–152`, `:174–191`; `managed_admission.sql:4–5`; `agent.sql:919–944`, `:2799–2800`; `handler/issue.go:3860–3866`, `:3903–3906`.

### Quick-create (item 6)

Both halves of the claim are confirmed; the multiplicity question is answered in F7 (yes, without limit when there is no captured source context).

## Could not verify

- **FK lock behaviour.** Every "FK → FOR KEY SHARE" statement here and in the lock graph is standard PostgreSQL behaviour, not observed. Nothing was executed.
- **The live schema.** Only migration files were read. Whether a deployed database still has these FKs and triggers was not checked.
- **Statement-internal lock order** inside `ClaimAgentTask`, `LinkTaskToIssue`, `CreateQuickCreateTask` and `SetTaskDeliveredCommentIDs` — still inferred, as the lock graph says.
- **Not read:** `CancelQueuedChatTasks`; `internal/service/task_knowledge_observe.go` beyond its three lock sites (`:40`, `:54`, `:133`); `MaybeRetryFailedTask` and `HandleFailedTasks`; `admission.Check`'s body; `lockStageChildRoot`; the admin cancellation path; caller-supplied `AfterCreateInTx` hooks other than the HTTP create's; the hourly rollup function beyond its lock statements.
- **Reachability** of the namespace/`"user"` inversion in F12.
- **The cycle statements** in F2 and F4 are reasoning over the lock sites read. A two-connection test is the only proof; V7 already plans one for P11, and F1–F3 each deserve one in step 5.

## Verdict

Step 1's output is sound enough to design the schema and the member binding on. The writer inventory is complete for product code: an independent enumeration from the SQL outward found the same eight queries and the same call sites, every fenced path takes the fence on the writing transaction before the write, and V1–V5 hold. The lock graph's facts hold too — about 95 citations checked, no wrong line number — and its one identified cycle is real. What it gets wrong is the rule drawn from those facts: "root last" is true of neither proposed position. Before step 3, add two inputs the documents do not have: the workspace-teardown manifest guard, which requires every budget table to be registered and to carry `workspace_id` (F5), and the fact that one quick-create task can create any number of issues with the same origin, so nothing in the schema may assume one child per attempt (F7). Before step 4, replace "BR lock (last)" on the issue side with either "member writes do not lock the root" or a per-path proof (F2), and correct the inventory's trigger claim so the lock graph gains the dirty-bucket family (F6). Before step 5, move the finalize position to after the receipt UPDATE so the root lock really is the last lock-taking statement (F1), make the member row a lock rather than a read (F3), and widen V7's rule from "no `task_usage`" to "nothing after the root lock" (F4). None of these invalidates a design decision already recorded in V1–V10.
