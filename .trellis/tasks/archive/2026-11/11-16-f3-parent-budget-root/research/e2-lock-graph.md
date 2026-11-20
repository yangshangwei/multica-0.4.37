# Research: E2 lock graph for the F3 parent budget root

- **Query**: Step 1 of the F3 parent-task budget root ("E2 lock graph"). Map the claim path, launch payload, terminal writers, usage report, issue write path, quick-create and auth facts at current HEAD; draw one ordered lock graph with the proposed positions of the budget member read, the budget root lock and the attempt/reservation insert; say whether "root row lock always last" creates a cycle; give verdicts for contract items R1, R2, R6, R7, R8, R9, R10.
- **Scope**: internal (read-only source research)
- **Date**: 2026-11-16
- **Source tree read**: `/Volumes/artisan/code/2026/mc-f3-budget-root-20261116` at HEAD `86fea651c` (clean worktree). No product code was read from the shared checkout. All paths below are relative to `server/` in that worktree unless they start with `.trellis/`.
- **Method**: file reads and grep only. No tests, builds, database commands or git write operations were run. PostgreSQL behaviour that the repository does not state itself is marked **(inferred)**.

## Findings

### Files Found

| File Path | Description |
|---|---|
| `internal/service/task.go` | `claimTask` :3424–3574, `ClaimTaskForRuntime` :3584–3726, `FinalizeTaskClaim` :3734–3866, `RequeueTaskAfterClaimFailure` :3872–3891, `ClaimTasksForRuntimes` :3915–4125, `CompleteTask` :4416–4626, `failTask` :4841–, `HandleFailedTasks` :5904–, `runInTx` :6547–6567, `enqueueQuickCreateTask` :1648–1801, `notifyQuickCreateCompleted` :7568– |
| `internal/service/managed_runtime.go` | `LockManagedRuntime` :123–152, `lockManagedTaskSource` :174–191, `ValidateManagedTaskBinding` :193–202 |
| `internal/service/task_capability.go` | `finalizeTaskCapability` :17–58 (restricted-profile locks at finalize) |
| `internal/service/task_knowledge.go` | `finalizeTaskKnowledgeClaim` :629– (read to :674) |
| `internal/service/task_cancellation.go` | `lockClaimedTask` :26–41, `CancelTaskInTx` :67–134, `ApplyCancellationEffects` :137–151 |
| `internal/service/task_start.go` | StartTask transaction (98 lines, read fully) |
| `internal/service/issue_status_write.go` | `WriteIssueStatus` :22–62 |
| `internal/handler/daemon.go` | `ClaimTasksByRuntime` :1740–1945, `buildClaimedTaskResponse` :2192–3480, `ClaimTaskByRuntime` :3525–3706, `StartTask` :3889–3924, `CompleteTask` :4059–4166, `ReportTaskUsage` :4676–4748, `FailTask` :4797–4817, `failTask` :4824–4866, `requireDaemonTaskAccess` :139–209 |
| `internal/handler/daemon_rpc.go` | WS RPC `tasks.claim` :59, `rpcClaimTasks` :66–125 |
| `internal/handler/agent.go` | `AgentTaskResponse` :378–561 (launch payload shape) |
| `internal/handler/issue.go` | `issueWriteFenceSQL` :3860–3866, `issueReadLockSQL` :3903–3906, `updateIssueAtomicallyOnce` :3936–, `deleteIssuesAndCollectAttachmentURLs` :4843–5029, `authorizeIssueDeletion` :5031–5077 |
| `internal/handler/actor_guards.go` | `RequireHumanActor` :96–104, `isMachineCredentialActor` :110–120 |
| `internal/middleware/auth.go` | `Auth` :61–304 (token prefixes, `X-Actor-Source`) |
| `internal/middleware/password_auth.go` | `passwordAuthenticate` :24–105 |
| `internal/middleware/daemon_auth.go` | `DaemonAuth` :79–290 |
| `internal/auth/principal.go` | `CredentialKind` constants :13–16 |
| `internal/daemon/client.go` | daemon upstream calls: `ClaimTask` :233–253, `StartTask` :440–448, `CompleteTask` :526–548, `ReportTaskUsage` :550–557, `FailTask` :559–587, response decode :1214 |
| `internal/daemon/types.go` | `ExecutionFence` :84–87, `Task` :101–, `Task.UnmarshalJSON` :230–252 |
| `internal/daemon/knowledge_context.go` | `taskKnowledgeFence` :28–40, `addKnowledgeFence` :42–46 |
| `internal/daemon/daemon.go` | usage report call :5258, terminal reports :5745/:5747, start :7588 (file is 9691 lines; only the cited regions read) |
| `pkg/protocol/task_knowledge.go` | `TaskKnowledgeFence` :34–46 |
| `pkg/protocol/messages.go` | client capability constants :6–39 |
| `pkg/db/queries/agent.sql` | `CreateQuickCreateTask` :459–489, `LinkTaskToIssue` :530–544, bulk cancels :709–774, `ClaimAgentTask` :808–917, `SetTaskDeliveredCommentIDs` :919–944, `RequeueAgentTaskAfterClaimFailure` :946–963, `CompleteAgentTask` :1158–1180, `FailAgentTask` :1389–1418, `RecoverOrphanedTasksForRuntime` :1443–1458, `FailStaleTasks` :1460–1535, cancel statements :1702–1864, `GetAgentForClaimUpdate` :1908–1911, `LockIssueExecution` :2799–2800, `LockAgentTaskForStart` :2806–2809 |
| `pkg/db/queries/runtime.sql` | `FailTasksForOfflineRuntimes` :260–288 |
| `pkg/db/queries/task_usage.sql` | `UpsertTaskUsage` :1–18 |
| `pkg/db/queries/issue_delete.sql` | `LockIssueDeleteTasks`, `LockIssueDeleteTaskTokens`, quota/autopilot NOWAIT locks (read fully) |
| `pkg/db/queries/lifecycle.sql` | `LockLifecycleAgent` :11–12, `LockLifecycleOriginTask` :20–22 |
| `pkg/db/queries/managed_admission.sql` | `LockTaskForClaimFinalization` :4–5 |
| `pkg/db/queries/admin_operation.sql` | `GetCancellationTaskForUpdate` :8–9 |
| `pkg/db/queries/chat.sql` | `LockChatSessionForTask` :1513–1521 |
| `pkg/db/queries/task_token.sql` | restricted capability principal locks :16–35 |
| `migrations/001_init.up.sql` | `issue.parent_issue_id` :65 |
| `migrations/032_task_usage.up.sql` | `task_usage` table and FK :1–12 |
| `migrations/108_task_token.up.sql` | `task_token` FKs :21–24 |
| `migrations/284_task_owner_row_fence.up.sql` | `lock_task_owner_rows` :73–154; FK KEY SHARE statement :46–50 |
| `migrations/535_triage_execution_fence.up.sql` | first `lock_issue_execution` :12 |
| `migrations/650_collaboration_legacy_execution_fence.up.sql` | current `lock_issue_execution` :6–60 |
| `cmd/server/router.go` | daemon routes :1550–1609 (claim :1573, :1577–1578; usage :1596); `RequireHumanActor` mounts |
| `cmd/server/runtime_sweeper.go` | `FailTasksForOfflineRuntimes` :315, `FailStaleTasks` :611, `HandleFailedTasks` :632 |

### 1. Claim path

The claim is two separate database transactions with payload construction between them, outside any transaction.

**Transaction A — dispatch (`claimTask`, `internal/service/task.go:3424–3574`).** One `s.runInTx` at :3434–3544. `runInTx` (:6547–6567) begins a transaction with no explicit isolation statement and no retry loop, calls `lockManagedTaskSource` first (`managed_runtime.go:174–191`; it only acts for a password session that has a `BindingID` and kind `daemon_token`: `LockPasswordSession` on the `"user"` row FOR UPDATE, then the installation namespace advisory lock, then `ValidateManagedNamespace`), then the callback, then commit.

Locks in order inside the callback:

| # | Site | Lock | Mode |
|---|---|---|---|
| 1 | `task.go:3449` `LockManagedRuntime` (`managed_runtime.go:123–152`) | installation namespace advisory lock; `managed_installation` row | advisory exclusive, blocking; FOR SHARE |
| 2 | `task.go:3455` `GetAgentForClaimUpdate` (`agent.sql:1908–1911`) | `agent` row | FOR UPDATE, blocking |
| 3 | `task.go:3494` `ClaimAgentTask` (`agent.sql:808–917`) | `issue` row via `lock_issue_execution` (:842) and `FOR SHARE OF i` (:829); candidate `agent_task_queue` row | KEY SHARE NOWAIT (function) / SHARE; FOR UPDATE SKIP LOCKED (:913–915) |

`ClaimAgentTask` sets status `dispatched` and `claim_generation + 1`. Then `ValidateManagedTaskBinding` :3511, `CaptureTaskExecutionBinding` :3515, `ReanchorClaimedDirectChatInput` :3530. The relative evaluation order of the issue lock and the task SKIP LOCKED lock inside the single `ClaimAgentTask` statement is **(inferred)**, not read from a plan. Neither waits: one is NOWAIT, the other SKIP LOCKED.

Callers of `claimTask`: `ClaimTask` :3415, `ClaimTaskForRuntime` :3708 (after a separate reclaim transaction :3629–3639 and an autocommit candidate list :3683), `ClaimTasksForRuntimes` :4098 (after reclaim transaction :3994–4009 and candidates :4052).

**Between the transactions — launch payload.** `buildClaimedTaskResponse` (`internal/handler/daemon.go:2192–3480`) runs on the pool, outside any transaction. The handlers then generate the task token string in memory (`auth.GenerateAgentTaskToken`, `daemon.go:1886` batch, `:3645` single) and only its hash is passed into finalize.

**Transaction B — finalize (`FinalizeTaskClaim`, `internal/service/task.go:3734–3866`).** One `s.runInTx` at :3762–3861.

| # | Site | What | Lock / mode |
|---|---|---|---|
| 1 | `:3764` `auth.LockPasswordSession` (password mode only) | session re-check; `:3768` `session.UserID != token.UserID` → `ErrPasswordSession` | `"user"` row FOR UPDATE, blocking |
| 2 | `:3794` `runtimeLookup.Get`, `:3798` `LockManagedRuntime` | managed binding | namespace advisory (blocking), `managed_installation` FOR SHARE |
| 3 | `:3802` `qtx.LockTaskForClaimFinalization` (`managed_admission.sql:4–5`) | re-read of the task | `agent_task_queue` row FOR UPDATE, blocking |
| 4 | `:3806` | **identity CAS** (no lock) | — |
| 5 | `:3810` `finalizeTaskKnowledgeClaim` | knowledge descriptor check | no explicit lock in the part read (`task_knowledge.go:629–674`); remainder unread |
| 6 | `:3819` `qtx.LockIssueExecution` (`agent.sql:2799–2800` → migration 650 :41–44) | admission re-check; `:3823–3824` not allowed → `&admission.Blocked{}` | `issue` row(s) FOR KEY SHARE NOWAIT |
| 7 | `:3826` `ValidateManagedTaskBinding` | binding compare | — |
| 8 | `:3829–3831` `finalizeTaskCapability` (`task_capability.go:17–58`) | restricted profiles only | issue FOR NO KEY UPDATE NOWAIT; task/agent/runtime/member/user and invocation targets FOR SHARE NOWAIT (`task_token.sql:16–35`) |
| 9 | `:3832` `qtx.CreateTaskToken` | **token issued** | INSERT into `task_token`; FK references to `agent_task_queue`, `agent`, `workspace`, `"user"` (`migrations/108_task_token.up.sql:21–24`) → FOR KEY SHARE on those rows, blocking **(inferred from the FK statement at `migrations/284:46–50`)** |
| 10 | `:3838` `DeleteExpiredDaemonTokens`, `:3841` `CreateDaemonToken` (only when exactly one daemon token is passed) | Remote MCP daemon token | DELETE row locks on expired `daemon_token` rows; INSERT with FK to `workspace` (`migrations/029:4`) |
| 11 | `:3845` | `if !recordCommentReceipt return nil` | — |
| 12 | `:3848–3855` `SetTaskDeliveredCommentIDs` (`agent.sql:919–944`) | delivery receipt | UPDATE on the already-held task row; its WHERE calls `lock_issue_execution` again → issue KEY SHARE NOWAIT |

The identity CAS at `task.go:3806`:

```go
if current.Status != "dispatched" || current.RuntimeID != task.RuntimeID || current.IssueID != task.IssueID || !bytes.Equal(current.Context, task.Context) || !current.DispatchedAt.Time.Equal(task.DispatchedAt.Time) || current.ClaimGeneration != task.ClaimGeneration || token.TaskID != current.ID || token.AgentID != current.AgentID || token.WorkspaceID != runtime.WorkspaceID {
    return ErrManagedRuntimeSource
}
```

`task.RuntimeID.Valid` is always true at :3793 because :3750 rejects an invalid runtime id, so steps 2–8 always run.

**The point "after identity CAS, before token issued".** The window is `task.go:3806–3809` (CAS, then `currentTask = current`) to `task.go:3832` (`CreateTaskToken`). Three existing lock acquisitions sit inside that window: :3810, :3819, :3829–3831. The last one ends at :3831. The position that is after the identity CAS, after every existing pre-token lock, and before the token insert is therefore **between `internal/service/task.go:3831` and `:3832`**.

Statements that still execute after that position in the same transaction: :3832, :3838, :3841, :3848 (rows 9–12 above). None of them waits on a task row other than the one already held, and the issue lock in :3848 is NOWAIT.

**Failure after dispatch.** `RequeueTaskAfterClaimFailure` (`task.go:3872–3891`) runs the autocommit CAS `RequeueAgentTaskAfterClaimFailure` (`agent.sql:946–963`). The handler comment at `daemon.go:3682–3685` states that the finalize transaction rolls the token back with the receipt and that the handler never deletes tokens by task on this path.

**Every finalize caller.** A grep for `FinalizeTaskClaim(` outside tests finds exactly two call sites:

- `internal/handler/daemon.go:1911` in `ClaimTasksByRuntime` (:1740–1945, batch; token generated :1886; `resp.AuthToken = tokenStr` :1933; requeue :1927; response `{"tasks": out}` :1944).
- `internal/handler/daemon.go:3663` in `ClaimTaskByRuntime` (:3525–3706; `buildClaimedTaskResponse` :3604; `resp.AuthToken = tokenStr` :3690; response `{"task": resp}` :3705).

The WS RPC `tasks.claim` (`internal/handler/daemon_rpc.go:59` → `rpcClaimTasks` :66–125) builds an HTTP request and calls `h.ClaimTasksByRuntime(rec, req)` at :119, so it reaches the batch call site. Routes: `cmd/server/router.go:1573` (`/runtimes/{runtimeId}/tasks/claim`), :1577–1578 (`/tasks/claim`, `/claim`).

### 2. Launch payload and daemon echo (R2)

**Payload.** `AgentTaskResponse` (`internal/handler/agent.go:378–561`) carries, among others: `execution_fence` (:380; `{runtime_id, dispatched_at}`, set at `daemon.go:2200–2202` only when `ExecutionBindingID.Valid`), `id`, `agent_id`, `runtime_id`, `issue_id`, `workspace_id`, `dispatched_at` (:421), `attempt` / `max_attempts`, `parent_task_id`, `knowledge_context` (:441), `delivered_comment_ids` (:477), `chat_session_id`, `autopilot_run_id`, `quick_create_*`, `parent_issue_id` (:508), `auth_token,omitempty` (:553–561). `taskCapability` (:379) is server-only. A grep of this file finds no `claim_generation` JSON field on the task payload.

**Daemon decode.** `Task.UnmarshalJSON` (`internal/daemon/types.go:230–252`) uses plain `json.Unmarshal` for the task object, so unknown fields are ignored. `knowledge_context` is decoded separately and strictly. The HTTP response decode is `json.NewDecoder(respReader).Decode(respBody)` (`client.go:1214`) and the WS response decode is `json.Unmarshal(resp.Body, respBody)` (`wsrpc.go:261`); neither sets `DisallowUnknownFields`. `DisallowUnknownFields` was found only in `managed_handoff.go:57`, `collaboration.go:47`, `collaborationrun/ledger.go:489` and the protocol collaboration files, none of which decode the claim response.

**Upstream identity.** For start, complete, fail and usage the daemon identifies the task by the URL `{taskId}` plus its daemon bearer token (`client.go:1194–1196`).

| Call | Body (daemon side) | Server decoder |
|---|---|---|
| `StartTask` `client.go:440–448` | `{}` plus optional `knowledge_fence` | `daemon.go:3889–3924`, plain decoder |
| `CompleteTask` `client.go:526–548` | `output`, optional `knowledge_fence`, `branch_name`, `session_id`, `work_dir`, `durable_work_dir`, `session_rollout_missing`, `retired_session_id`; retried with `defaultTerminalRetrySchedule` | `TaskCompleteRequest` `daemon.go:4008–4030`, plain decoder |
| `FailTask` `client.go:559–587` | failure fields plus optional `knowledge_fence` | `TaskFailRequest` `daemon.go:4775–4795`, plain decoder |
| `ReportTaskUsage` `client.go:550–557` | only `{"usage": usage}`; single `postJSON`, no retry | `daemon.go:4676–4748` |
| `AckTaskCancelled` `client.go:498–500` | `TaskCancelAck` :466–486: `operation_id`, `binding_epoch`, `execution_fence`, `outcome` | not read |

`claim_generation` goes upstream only inside `knowledge_fence`: `taskKnowledgeFence` (`internal/daemon/knowledge_context.go:28–40`) returns `TaskKnowledgeFence{ClaimGeneration, DispatchedAt}` (`pkg/protocol/task_knowledge.go:34–46`) only when the task carries a `KnowledgeContext` or a recovered fence, and `addKnowledgeFence` (:42–46) places it in `body["knowledge_fence"]`. It is never in the usage body and never a general task field. The usage report is sent at `internal/daemon/daemon.go:5258` after the runner returns; a failure is only logged (:5259).

**Where a server-issued `budget_attempt_id` could ride.** The facts that bear on this, without a recommendation:

- Downstream: the claim response is decoded tolerantly on both transports (`types.go:230–252`, `client.go:1214`, `wsrpc.go:261`), so an added top-level field in `AgentTaskResponse` is ignored by a daemon that does not know it. A field added inside `knowledge_context` would hit the strict decoder.
- Upstream: the start, complete, fail and usage request bodies are decoded with plain `json.NewDecoder` on the server, so an added body field is ignored by a server that does not know it, and a daemon that does not send it leaves it absent.
- Capability signalling that exists today: the `X-Client-Capabilities` header (`client.go:177–188`, `daemonClientCapabilities` :206–217) with the constants at `pkg/protocol/messages.go:6–39`.

### 3. Terminal writers (R10)

| Writer | Transaction | Locks in order | Guard |
|---|---|---|---|
| `CompleteTask` `task.go:4416–4626` | `runInTx` :4422–4502 | `lockChatSessionForTaskWrite` :4423 (chat_session FOR UPDATE) → only with knowledge fences `LockTaskForClaimFinalization` :4427 (task FOR UPDATE) → `CompleteAgentTask` :4438 (UPDATE row lock) | `agent.sql:1158–1180` `WHERE id = $1 AND status = 'running'`. On `pgx.ErrNoRows` the existing row is returned as idempotent success (:4503–4515) |
| `failTask` `task.go:4841–` | transaction from :4921 | `lockChatSessionForTaskWrite` :4922 → (fences) `LockTaskForClaimFinalization` :4926 → (expected claim) `lockClaimedTask` :4941 → `FailAgentTask` :4945 → `CreateRetryTask` :5055 in the same transaction | `agent.sql:1389–1418` `WHERE id = $1 AND status IN ('dispatched','running','waiting_local_directory')` |
| `lockClaimedTask` `task_cancellation.go:26–41` | caller's | chat_session → `GetCancellationTaskForUpdate` (`admin_operation.sql:8–9`, task FOR UPDATE, blocking) → CAS :37 on runtime / dispatched_at / claim_generation / status `dispatched` / `started_at IS NULL` → `ErrClaimSuperseded` | — |
| `CancelTaskInTx` `task_cancellation.go:67–134` (via `CancelTaskWithResult` `task.go:2911`) | `runInTx` | queued-only path: `LockChatSessionForTask` → `CancelQueuedAgentTask`; otherwise `lockChatSessionForTaskWrite` :103 → `CancelAgentTaskByUser` / `CancelAgentTaskWithReason` / `CancelAgentTask` | status-guarded UPDATEs `agent.sql:1702–1864`; `ErrNoRows` returns the existing row unchanged |
| `CancelQueuedChatTasks` `task.go:2926–2965` | transaction | chat_session → `agent` FOR UPDATE → `CancelQueuedAgentTasksForSession` | status-guarded |
| Bulk cancels `agent.sql:709–774` | per caller | plain `UPDATE … RETURNING` | status-guarded. Callers: `handler/comment.go:3468`, `handler/agent.go:2544`, `handler/chat.go:615/:720`, `handler/workspace_revoke.go:112`, `handler/issue.go:4951`, `auth/password_revoke.go:61`, `service/task.go:2735/:2756/:5766`, `service/runtime_teardown.go:150` |
| `FailStaleTasks` `agent.sql:1460–1535` | autocommit from `cmd/server/runtime_sweeper.go:611`, then `HandleFailedTasks` :632 | SKIP LOCKED row locks | lease/timeout predicates |
| `FailTasksForOfflineRuntimes` `runtime.sql:260–288` | autocommit from `runtime_sweeper.go:315` | `FOR UPDATE OF task SKIP LOCKED` :279 | runtime offline predicate |
| `RecoverOrphanedTasksForRuntime` `agent.sql:1443–1458` | autocommit from `handler/task_lifecycle.go:34` | UPDATE row locks | runtime + status |
| `ExpireStaleQueuedTasks` `agent.sql:1537`, `FailExpiredRuntimeReconnectRetries` :1651 | autocommit (`runtime_sweeper.go:649`, :344) | UPDATE row locks | status + age |
| `RequeueAgentTaskAfterClaimFailure` `agent.sql:946–963` | autocommit | UPDATE row lock | CAS on the dispatched claim |
| `HandleFailedTasks` `task.go:5904–` (read to :5959) | post-failure, own transactions | `MaybeRetryFailedTask`, `recoverDelegatedTaskFailure`, `WriteIssueStatus(… "todo", "failed_task_reset" …)` :5956 | — |
| Issue-delete cancel `handler/issue.go:4843–5029` | **single transaction**, commit :5020 | `lockIssueWriteFences` :4877 → `LockIterations` :4884 → `LockIssueDeleteRows` :4891 (issues + direct children `ORDER BY id FOR UPDATE`) → `authorizeIssueDeletion` :4904 (agent actor: `LockLifecycleAgent` FOR SHARE NOWAIT, `LockLifecycleOriginTask` FOR SHARE OF t NOWAIT) → `LockIssueDeleteTasks` :4933 (`ORDER BY task.id FOR UPDATE OF task NOWAIT`, active statuses) → `LockIssueDeleteTaskTokens` :4936 (FOR UPDATE NOWAIT) → autopilot/quota NOWAIT :4939–4945 → `CancelAgentTasksByIssue` :4951 → `DeleteTaskTokensByTask` :4957 → `FailAutopilotRunsByIssue` :4961 → `DetachDirectChildIssues` :4964 → `RecordIssueDeletion` :5013 → `DeleteIssue` :5016 | post-commit `PublishIssueDeletionTaskCancellations` (:4813 single, :5822 batch) |

None of the terminal writers above takes an `issue` row lock, except the issue-delete transaction, which holds the issue rows FOR UPDATE before it touches tasks.

Handler-side token revoke after a terminal write is autocommit and best-effort: `DeleteTaskTokensByTask` at `daemon.go:4160` (complete) and :4853 (fail).

**R10 evidence.** Every terminal writer is a status-guarded UPDATE on the task row. None of them reads or proves that the daemon-side process has exited. The repository states the gap itself:

- `agent.sql:1469–1470`: "An expired lease may fail immediately on a healthy runtime".
- `agent.sql:1478`: "This keeps healthy multi-hour work alive through a network partition".
- `runtime.sql:262–264`: "A short or medium network partition must not terminate a daemon process that is still running locally".
- `agent.sql:1443–1458` comment: "the tasks the previous daemon process was running when it died".
- `task.go:4503–4515`: completion of an already-terminal row is returned as success ("already finalized").

### 4. `ReportTaskUsage` end to end

- **Route and middleware**: `cmd/server/router.go:1596`, inside the `/api/daemon` group (:1550–1609) with `handler.PlatformAdminNoStore` and `middleware.DaemonAuth(...)`.
- **Access check**: `requireDaemonTaskAccess` (`daemon.go:139`; `…WithWorkspace` :151–209): `GetAgentTask` → `ResolveTaskWorkspaceIDChecked` → workspace access → `ValidateManagedTaskBinding` → runtime access.
- **Identity trusted**: the URL `taskId` and the daemon credential's workspace/runtime access. No task status check, no claim generation or fence check, no attempt identity.
- **Transaction**: none. A loop of autocommit `h.Queries.UpsertTaskUsage` calls (:4713–4722), one per usage entry.
- **Partial failure**: on an upsert error the handler logs `slog.Warn` and continues (:4723–4725), then always returns 200 `{"status":"ok"}` (:4747). The daemon cannot observe a partial write. The daemon does not retry the call (`client.go:550–557`) and only logs a transport failure (`daemon/daemon.go:5259`).
- **Payload**: `TaskUsagePayload` :4651–4663; `authoritativeCostTicks` :4669–4674; an empty provider is backfilled from the task's runtime (:4701–4711). Metrics via `CaptureTaskUsage` :4726 (`service/task.go:878–884`).
- **SQL**: `pkg/db/queries/task_usage.sql:1–18` — `INSERT … ON CONFLICT (task_id, provider, model) DO UPDATE SET` every counter and `cost_usd_ticks` to the `EXCLUDED` value (overwrite, not accumulate), `updated_at = now()`.
- **Tables**: `task_usage` (`migrations/032_task_usage.up.sql:1–12`; `task_id UUID NOT NULL REFERENCES agent_task_queue(id) ON DELETE CASCADE` :3; `UNIQUE (task_id, provider, model)`). `task_usage_hourly` is fed from the `updated_at` watermark by the scheduled `rollup_task_usage_hourly` (`cmd/server/main.go:814–827`). The only trigger on `task_usage` is `trg_tu_dirty_hourly BEFORE DELETE` (`migrations/243:20–25`; function `migrations/102:233–259`, comment :232 "INSERT/UPDATE are covered by the watermark window"). No migration was found that drops the `task_usage` FK by constraint name.
- **Lock taken today**: the INSERT's FK check takes FOR KEY SHARE on the referenced `agent_task_queue` row, blocking **(inferred from `migrations/284:46–50`)**.

### 5. Issue write path order versus finalize order (R6)

**Issue write order.** `issueWriteFenceSQL` (`internal/handler/issue.go:3860–3866`): workspace FOR KEY SHARE → subscriber advisory → member FOR SHARE → issue-status catalog shared advisory → iteration fence (`pg_advisory_xact_lock(hashtextextended('iteration:' || $1::uuid::text, 0))`, `internal/iteration/transaction.go:83`). `issueReadLockSQL` (:3903–3906): iteration `FOR UPDATE OF i`, then issue FOR UPDATE.

- `updateIssueAtomicallyOnce` (:3936–): fences :3947 → issue FOR UPDATE :3971/:3988 → `LockProjectForAssociationNowait` :4032 → `admission.Check` :4040 → `UpdateIssue` :4112. No task lock in :4029–4140; the rest of the function was not read.
- `WriteIssueStatus` (`internal/service/issue_status_write.go:22–62`): own READ COMMITTED transaction; workspace KEY SHARE :46 → catalog :49 → fence :52 → iteration :55 → issue FOR UPDATE :59. Comment :23–24: "FOR UPDATE is intentional: execution admission holds KEY SHARE, which would not conflict with NO KEY UPDATE."
- StartTask (`internal/service/task_start.go`, read fully): READ COMMITTED; `GetIssue` :37 → workspace KEY SHARE :41 → status catalog shared :44 → `iteration.LockWorkspace` :47 → `LockIssueIteration` :50 → `lockStageChildRoot` :56 → `LockIssueForDescriptionUpdate` :59 (issue FOR UPDATE) → `LockAgentTaskForStart` :66 (task **FOR UPDATE NOWAIT**, `agent.sql:2806–2809`, comment "NOWAIT avoids reversing another task-owner/issue writer's lock order.") → fences :70 → `StartAgentTask` :87 → commit :94. `LockAgentTaskForStart` has exactly one caller (:66).
- Issue delete: issue FOR UPDATE (`issue.go:4891`) → task `FOR UPDATE OF task NOWAIT` (`issue_delete.sql`, `LockIssueDeleteTasks`, called at :4933). Comment at `issue.go:4930–4932`: "Completion, credential minting and quota writers may hold their rows before an issue row. Never wait on those intersections while retaining the deletion's issue locks."

**Finalize order.** Task FOR UPDATE, blocking (`task.go:3802`) → issue FOR KEY SHARE NOWAIT (`task.go:3819`).

**The migration that defines the finalize-side issue lock.** `migrations/650_collaboration_legacy_execution_fence.up.sql` is the current definition of `lock_issue_execution(p_issue_id, p_trigger_id DEFAULT NULL, p_comment_ids DEFAULT '{}', p_context DEFAULT NULL)` (:6–11). The lock statement is :41–44, `SELECT admission_status … FROM issue WHERE id = owner_id FOR KEY SHARE NOWAIT`, looped in id order. `EXCEPTION WHEN lock_not_available THEN RETURN false` at :57–58. The function was introduced in `migrations/535_triage_execution_fence.up.sql:12`, with the comment at :11 "NOWAIT avoids a lock-order inversion with deletion; contention yields no run."

**Result.** The two paths take the task row and the issue row in opposite orders: issue writers hold issue FOR UPDATE and then take the task NOWAIT; finalize holds the task FOR UPDATE and then takes the issue KEY SHARE NOWAIT. In both directions the second acquisition is NOWAIT, so neither side waits for the other's row. Two points differ in detail from the contract wording:

- For a quick-create task finalize locks `context.parent_issue_id` and the captured source issue, not "its own issue" (migration 650 :20–40), because the task's `issue_id` is NULL.
- A restricted-profile finalize additionally takes the issue FOR NO KEY UPDATE NOWAIT (`task_token.sql:16–35`, via `task.go:3829`).

`lock_task_owner_rows` (`migrations/284:73–154`) is a separate blocking fence: workspace FOR KEY SHARE (:124), then agent :138, issue :143, runtime :148, all FOR KEY SHARE without NOWAIT. It is called from task-ownership writers (enqueue, `LinkTaskToIssue`), not from finalize.

### 6. Quick-create (R7)

- **Enqueue**: `enqueueQuickCreateTask` (`task.go:1648–1801`). `admission.Check` on the parent :1649 (no lock). The `QuickCreateContext` with `ParentIssueID` (:1688–1690) is marshalled into the task `Context` (:1702/:1731). Insert is `CreateQuickCreateTask`, autocommit at :1742, or inside a transaction with source-context capture (:1744–1779: `LockIssueForDescriptionUpdate` on the source issue :1750 → `LockCommentAncestorPath` :1754 → insert :1772).
- **SQL**: `CreateQuickCreateTask` (`agent.sql:459–489`) writes `issue_id NULL`, `context = $4`, with `WHERE lock_task_owner_rows($1, NULL, $2) AND lock_issue_execution(NULL, NULL, '{}', $4::jsonb)`.
- **Where `context.parent_issue_id` is stored**: only inside the `agent_task_queue.context` JSON. No separate column was found.
- **How it is compared in the identity CAS**: `task.go:3806` compares `current.IssueID != task.IssueID` (both NULL for quick-create) and the whole context byte-wise, `!bytes.Equal(current.Context, task.Context)`. `parent_issue_id` is covered as part of those bytes; it is not compared as a separate field.
- **Finalize issue lock**: `task.go:3819` passes `task.Context` to `LockIssueExecution`, which locks the parent issue and the captured source issue FOR KEY SHARE NOWAIT (migration 650 :24–39, :41–44).
- **Payload**: `handler/daemon.go:3140–3284`, `resp.ParentIssueID` :3242. `parseQuickCreateContext` (`task.go:7488–7503`) returns false if `IssueID`, `ChatSessionID` or `AutopilotRunID` is valid.
- **Link on completion**: `notifyQuickCreateCompleted` (`task.go:7568–`) is called at :4591–4593, after the `CompleteTask` transaction has committed. It finds the created issue with `GetIssueByOrigin` (:7579; origin type `quick_create`, origin id = task id) and calls `s.Queries.LinkTaskToIssue` at :7625–7634 — autocommit, best-effort, failure only logged.
- **Link SQL and locks**: `LinkTaskToIssue` (`agent.sql:530–544`): `UPDATE agent_task_queue SET issue_id = $2 WHERE id = $1 AND issue_id IS NULL AND lock_task_owner_rows(NULL, $2, NULL) AND lock_issue_execution($2);` → workspace and issue FOR KEY SHARE (blocking), issue FOR KEY SHARE NOWAIT, plus the UPDATE's own lock on the task row. Trigger `trg_atq_dirty_hourly BEFORE UPDATE OF runtime_id, issue_id` fires (`migrations/243:6–11`).

A quick-create task therefore has `issue_id IS NULL` for its entire execution, including claim, finalize, start, usage and the terminal write, and stays NULL if the post-commit link fails.

### 7. Auth facts (R8, R9)

**R8 — what finalize re-checks for an ordinary agent.** `finalizeTaskCapability` (`internal/service/task_capability.go:17–58`) returns at :22–28 when the profile is empty (nil, or `ErrTaskCapability` if the token carries an `ApiCapability`). The lock and compare steps (`LockTaskCapabilityPrincipals`, `LockTaskCapabilityInvocationTargets`) run only for restricted profiles. For an ordinary agent the finalize transaction re-checks: password session version (`task.go:3764`, password mode only), managed installation binding (:3798, :3826), task row identity (:3806), issue admission (:3819). No agent, runtime owner or member re-check was found on this path.

**R9 — `RequireHumanActor`.** `internal/handler/actor_guards.go:96–104`; the decision is `isMachineCredentialActor` :110–120:

```go
switch r.Header.Get("X-Actor-Source") {
case "task_token", "cloud_pat":
    return true
default:
    return false
}
```

The comments at :91–95 and :108–109 state that unknown actor sources intentionally remain human-equivalent. It is a deny-list of two values of a server-set header.

Mount points of the guard: `cmd/server/router.go:1632`, :1792, :1867, :1870, :2147, :2159–2162, :2190, :2210, :2277–2286, :2323, :2378–2384, :2437, :2515–2516. Inline uses of `isMachineCredentialActor`: `iteration_settings.go:61/:75`, `personal_access_token.go:68`, `handler/auth.go:700`, `agent.go:1939`, `triage.go:183`, `cloud_billing.go:85`.

**Token kinds and sources.**

| Credential | Middleware site | `X-Actor-Source` | `auth.CredentialKind` / session kind | Notes |
|---|---|---|---|---|
| Client-supplied `X-Actor-Source` | `middleware/auth.go:71`, `daemon_auth.go:90` | deleted | — | header is server-set only |
| `mat_` task token | `auth.go:114–148` | `task_token` (:143) | `CredentialTask` (`principal.go:13–16`) | sets `X-User-ID` to the token's bound user (:134), `X-Agent-ID`, `X-Task-ID`, `X-Workspace-ID` |
| `mat_` in password mode | `password_auth.go:24–105` | `task_token` (:61) | session kind `task_token` (`internal/auth/managed_binding.go:87–88`) | — |
| `mcn_` cloud PAT | `auth.go:170–209`; `daemon_auth.go:176–210` | `cloud_pat` (:205 / :206) | `CredentialCloud` | rejected in password mode (`password_auth.go:40–42`) |
| `mul_` PAT | `auth.go:212–263`; `daemon_auth.go:213–256` | none | `CredentialPAT`; password session kind `pat` (`password_auth.go:49`) | — |
| JWT | `auth.go:265–304`; `daemon_auth.go:259–287` | none | `CredentialJWT`; password session kind `jwt` (`password_session.go:111`) | — |
| `mdt_` daemon token | `daemon_auth.go:127–166` | none | daemon auth path `daemon_token` (:27–30); password session kind `daemon_token` (`password_token.go:36/:45`) | sets daemon workspace / daemon id in context; sets no `X-User-ID`. No branch in `Auth`; it falls through to the JWT parse **(inferred to fail as an invalid token)** |
| `mpi_` / `mpc_` plugin tokens | `middleware/plugin_auth.go:46` on the Plugin API origin (`router.go:1612`) | not read | not read | rejected by `passwordAuthenticate` (:40–42) and `CheckPasswordToken` (:24) |

`withAuthenticatedRequestPrincipal` (`auth.go:35–42`) sets `principal.AgentActor` when `X-Agent-ID` or `X-Task-ID` is non-empty. `auth.CredentialKind` values: `jwt`, `pat`, `task_token`, `cloud_pat` (`internal/auth/principal.go:13–16`). Daemon auth-path constants: `daemon_token`, `pat`, `cloud_pat`, `jwt` (`daemon_auth.go:27–30`).

### 8. The lock graph

Legend: `→` = acquired after, in the same transaction. `[W]` = may wait. `[NW]` = NOWAIT. `[SL]` = SKIP LOCKED. `(inf)` = inferred FK lock.

```
Family index (position in the common prefix, where a path uses it)
  U   "user" row FOR UPDATE                     (password mode: LockPasswordSession)
  N   installation namespace advisory, exclusive
  MI  managed_installation FOR SHARE
  CS  chat_session FOR UPDATE                   ("chat_session -> agent_task_queue is the repo-wide order", task.go:2967-2991)
  A   agent FOR UPDATE                          (GetAgentForClaimUpdate, CancelQueuedChatTasks)
  W   workspace FOR KEY SHARE
  SUB subscriber advisory
  M   member FOR SHARE
  SC  issue-status catalog shared advisory
  IF  iteration fence advisory
  IT  iteration row FOR UPDATE
  I   issue row   (FOR UPDATE | KEY SHARE | NO KEY UPDATE)
  T   agent_task_queue row (FOR UPDATE | SHARE | implicit UPDATE)
  TT  task_token row
  BM  budget member row      (proposed, read)
  BR  budget root row        (proposed, lock)
  BA  attempt + reservation  (proposed, insert)

P1  claimTask tx (task.go:3434-3544)
    [U[W]] → N[W] → MI → A[W] → { I KEY SHARE[NW] , T FOR UPDATE[SL] }        (order inside the statement inferred)

P2  FinalizeTaskClaim tx (task.go:3762-3861)
    [U[W]] → N[W] → MI → T FOR UPDATE[W] (:3802) → identity CAS (:3806)
           → I KEY SHARE[NW] (:3819)
           → [restricted only: I NO KEY UPDATE[NW], T/A/runtime/M/U SHARE[NW] (:3829)]
           → <<< proposed: BM read → BR lock → BA insert  (between :3831 and :3832) >>>
           → task_token INSERT (:3832): KEY SHARE(inf) on T (already held), A, W, U
           → [daemon_token DELETE/INSERT (:3838/:3841): KEY SHARE(inf) on W]
           → T UPDATE receipt (:3848) with I KEY SHARE[NW]

P3  issue update / WriteIssueStatus (issue.go:3936-, issue_status_write.go:22-62)
    W → SUB → M → SC → IF[W] → IT[W] → I FOR UPDATE[W]

P4  StartTask (task_start.go)
    W → SC → IF[W] → IT[W] → I FOR UPDATE[W] → T FOR UPDATE[NW]

P5  issue delete (issue.go:4843-5029)
    W → SUB → M → SC → IF[W] → IT[W] → I FOR UPDATE[W] (issues + children, id order)
      → [A SHARE[NW], T SHARE[NW] for an agent actor]
      → T FOR UPDATE[NW] → TT FOR UPDATE[NW] → autopilot/quota[NW] → T UPDATE (cancel) → TT DELETE → I DELETE

P6  complete / fail / cancel (task.go:4422-, :4921-, task_cancellation.go)
    [U[W]] → [N → MI] → CS[W] → [T FOR UPDATE[W] when a fence or expected claim is present] → T UPDATE[W]
    (failTask may then INSERT the retry task in the same tx)

P7  sweepers / orphan recovery / requeue (autocommit)
    T UPDATE, candidates taken [SL] where the statement has a locking subquery

P8  quick-create link (autocommit, after P6 commit)
    W KEY SHARE[W] → I KEY SHARE[W] → I KEY SHARE[NW] → T UPDATE[W]

P9  usage report today (autocommit per entry)
    task_usage INSERT/UPDATE: KEY SHARE(inf) on T [W]

P10 proposed issue-side member binding, as described in the task brief
    P3 prefix … → I FOR UPDATE → BM write → BR lock (last)

P11 proposed versioned usage report, as described in the task brief
    … → BR lock → (usage write)
```

**Existing opposite orders and why they do not wait on each other today.** P2 takes T then I; P4 and P5 take I then T. In P2 the I acquisition is NOWAIT (migration 650 :42) and in P4/P5 the T acquisition is NOWAIT (`agent.sql:2806–2809`, `issue_delete.sql`). P8 takes I (blocking KEY SHARE) and then T, the same direction as P4/P5; KEY SHARE on I conflicts only with I FOR UPDATE holders (P3, P4, P5), which do not wait on T.

**Proposed positions.**

- Member-row read: in P2 after the last existing pre-token lock, i.e. after `task.go:3831`.
- Root-row lock: immediately after the member read, still before `task.go:3832`.
- Attempt + reservation insert: after the root lock, before `CreateTaskToken` at `task.go:3832`.

At that position P2 already holds: U (password mode), N, MI, T FOR UPDATE, I KEY SHARE (and for restricted profiles the NOWAIT locks from :3829).

**Does "root last" create a cycle?** With BR placed at that position in P2 and as the final lock in P10, every edge involving BR points into BR from locks already held: `T → BR` and `I(KEY SHARE) → BR` in P2, `I(FOR UPDATE) → BR` in P10. P10's I FOR UPDATE and P2's I KEY SHARE conflict with each other, but P2 takes its I lock NOWAIT, so P2 never waits for I while P10 holds it; a transaction that fails that NOWAIT has not reached BR. On the paths read, no existing transaction takes a blocking T or I lock after the proposed BR position in P2: the statements after it are the token insert, the daemon-token delete/insert and the receipt UPDATE (own T row already held; I lock NOWAIT). So within P1–P8 and P10, BR is a sink and the rule is cycle-free with respect to task and issue rows.

**The one path read that would close a cycle.** P11, if the usage transaction locks BR and then executes `UpsertTaskUsage` while still holding it. `task_usage.task_id` references `agent_task_queue(id)` (`migrations/032_task_usage.up.sql:3`), so the insert takes a blocking FOR KEY SHARE on the task row **(inferred)**. That is an edge `BR → T`. The counter-edge `T → BR` is P2 holding T FOR UPDATE from `task.go:3802` and waiting for BR. The interleaving is reachable when a task that was re-claimed is being finalized while the previous generation's daemon reports usage for the same task id (the usage handler has no status or generation check, section 4). Other holders of T FOR UPDATE that conflict with the FK KEY SHARE and would make the usage transaction wait while it holds BR: `GetCancellationTaskForUpdate` (`admin_operation.sql:8–9`), `LockTaskForClaimFinalization` in complete/fail with fences (`task.go:4427`, :4926), StartTask's lock (held until commit, `task_start.go:66–94`), the delete transaction's task lock (`issue.go:4933`), and the knowledge observe path (not read). Of these, only P2 was read as also waiting for BR.

**Waits that remain possible while P2 holds BR at the proposed position (not on task or issue rows).**

- FK KEY SHARE on the `agent` row from the token insert (:3832) **(inferred)**: conflicts with agent FOR UPDATE held by P1 (`GetAgentForClaimUpdate`) and by `CancelQueuedChatTasks`. Neither of those takes BR on the paths read.
- FK KEY SHARE on the `"user"` row **(inferred)**: conflicts with user FOR UPDATE held by `LockPasswordSession`. In password mode P2 already holds that same row FOR UPDATE from :3764, before BR.
- FK KEY SHARE on the `workspace` row **(inferred)**: conflicts with workspace teardown's `LockWorkspaceForDelete` (named in `migrations/284:32–33`; the teardown path itself was not read).
- `DeleteExpiredDaemonTokens` (:3838) row locks on expired `daemon_token` rows, only when a Remote MCP daemon token is being minted.
- Issue delete takes its agent and origin-task reference locks as FOR SHARE NOWAIT (`lifecycle.sql:11–12`, :20–22). SHARE does not conflict with KEY SHARE, and both are NOWAIT.

### Contract verdicts at HEAD 86fea651c

| Item | Verdict | Evidence |
|---|---|---|
| R1 | Still true | `migrations/001_init.up.sql:65` `parent_issue_id UUID REFERENCES issue(id) ON DELETE SET NULL`. `pkg/db/queries/` has `task_knowledge.sql`, `task_message.sql`, `task_token.sql`, `task_usage.sql` and no `tasks.sql`; claim SQL lives in `agent.sql`. |
| R2 | Still true in structure, line numbers moved, one nuance | Dispatch `task.go:3424`, finalize `:3734`, identity CAS `:3806`, requeue SQL `agent.sql:946`. `claim_generation` is not a task payload field (`handler/agent.go:378–561`) and is not in the usage body (`client.go:550–557`). Nuance: it does travel upstream inside `knowledge_fence` on start/complete/fail for tasks whose claim carried a `knowledge_context` (`daemon/knowledge_context.go:28–46`, `pkg/protocol/task_knowledge.go:34–46`). |
| R6 | Still true in shape, with nuances | Issue writers: issue FOR UPDATE then task NOWAIT (`task_start.go:59` → `:66`, `agent.sql:2806–2809`; `issue.go:4891` → `:4933`). Finalize: task FOR UPDATE `task.go:3802` → issue KEY SHARE NOWAIT `task.go:3819`, defined at `migrations/650_collaboration_legacy_execution_fence.up.sql:41–44` (introduced `migrations/535:12`). Nuances: quick-create finalize locks the parent/source issue, not the task's own; restricted-profile finalize adds issue NO KEY UPDATE NOWAIT; issue delete is a second issue-then-task NOWAIT site. |
| R7 | Still true | `agent.sql:459–489` (insert with `issue_id NULL`), context compare at `task.go:3806`, link at `task.go:7625–7634` via `agent.sql:530–544`, executed after the completion transaction commits (`task.go:4591–4593`), best-effort. |
| R8 | Still true | `task_capability.go:22–28` returns early for an empty profile; ordinary-agent finalize re-checks only `task.go:3764`, `:3798`, `:3806`, `:3819`, `:3826`. |
| R9 | Still true | `actor_guards.go:110–120`: deny-list of `task_token` and `cloud_pat`; unknown sources are human-equivalent (:91–95, :108–109). `mdt_` produces no actor source (`daemon_auth.go:127–166`). |
| R10 | Still true | Status-guarded UPDATEs only (`agent.sql:1158–1180`, :1389–1418, :1460–1535, :1702–1864; `runtime.sql:260–288`); comments at `agent.sql:1469–1470`, :1478 and `runtime.sql:262–264`; idempotent completion at `task.go:4503–4515`. |

Observed outside the requested list: contract item R3's wording that task cancellation commits outside the issue-delete transaction does not match HEAD. `LockIssueDeleteTasks`, `CancelAgentTasksByIssue` and `DeleteTaskTokensByTask` run inside the delete transaction (`handler/issue.go:4933–4957`, commit :5020); only the cancellation publish is post-commit.

### External References

None. No external source was consulted.

### Related Specs

- `.trellis/tasks/11-16-f3-parent-budget-root/prd.md`, `design.md`, `implement.md` — the task this research serves.
- `.trellis/tasks/11-12-agent-squad-followup-roadmap/research/f3-minimum-budget-contract-20261113.md` — contract; §6, §7 and §11 R1–R10 (§11 wins).
- `.trellis/tasks/11-12-agent-squad-followup-roadmap/research/f3-execution-source-map-20261113.md`, `f3-usage-source-map-20261113.md` — earlier source maps written at `fb9f4c070`; their line numbers are stale against HEAD.
- `.trellis/tasks/11-16-f3-parent-budget-root/research/e2-writer-inventory.md` — another agent's writer inventory in the same directory; not read for this document.

## Caveats / Not Found

**Inferred, not read or executed**

- FK-induced FOR KEY SHARE locks on INSERT (task_token, daemon_token, task_usage). The repository states the general rule at `migrations/284_task_owner_row_fence.up.sql:46–50`; the specific lock on each referenced row was not observed.
- Evaluation order of the issue lock and the task SKIP LOCKED lock inside the single `ClaimAgentTask` statement, and of the WHERE-clause function calls in `LinkTaskToIssue`, `CreateQuickCreateTask` and `SetTaskDeliveredCommentIDs`.
- The row-lock mode of the status-guarded task UPDATEs (FOR NO KEY UPDATE versus FOR UPDATE).
- That an `mdt_` token presented to the member `Auth` middleware fails as an invalid JWT.
- The cycle statement in section 8 is reasoning over the lock sites read. Nothing was executed against a database.

**Unread or partially read files that matter**

- `internal/service/task_knowledge.go` after :674 (`finalizeTaskKnowledgeClaim` remainder) — sits inside the CAS-to-token window; no explicit lock was seen in the part read.
- `internal/service/task_knowledge_observe.go` — another holder of the task row.
- `internal/service/task.go` `failTask` after :5114; `HandleFailedTasks` after :5959; `MaybeRetryFailedTask`.
- `internal/handler/issue.go` `updateIssueAtomicallyOnce` beyond :4140.
- `internal/service/issue.go` `CreateInTx` :395–450 and `admission.Check`.
- `lockStageChildRoot` (called at `task_start.go:56`).
- `adminOperationService().AcknowledgeCancellation` and `admin_operation.go:292`/`:486`; the server handler for `AckTaskCancelled`.
- `internal/daemon/daemon.go` outside :5258–5259, :5745–5747, :7588, :8042–8047 (the file is 9691 lines).
- Workspace teardown (`LockWorkspaceForDelete`) and the legacy runtime merge (`LockRuntimesForMerge`).
- Other holders of `agent` FOR UPDATE and `"user"` FOR UPDATE beyond those listed.
- Password-mode request middleware locking on the issue write routes.
- Fence users at `autopilot.go:710` and `onboarding_shim.go:529`.
- `middleware/plugin_auth.go` beyond the token-prefix entry at :46.
- Down migrations and any migration after 650 that might redefine `lock_issue_execution` were not swept exhaustively; 650 is the latest definition found by grep.

**Not found**

- A `claim_generation` field on the task launch payload.
- A separate column holding the quick-create parent issue id on `agent_task_queue`.
- Any status, generation or attempt check in `ReportTaskUsage`.
- A migration dropping the `task_usage → agent_task_queue` foreign key.
- Any terminal writer that verifies daemon-side process exit.
