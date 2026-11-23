# Research: F3 执行 / claim / 任务树来源证据

- Query: claim 与 finalize、task 生命周期 writer、ordinary issue 树 writer、无 `issue_id` 的执行、权限接缝、可复用并发原语；对照 `f3-minimum-budget-contract-20261113.md` §3/§6/§7。
- Scope: internal；只读源码与规格。唯一输出为本文件。未运行产品测试、DB 命令、迁移、Provider、`make` 或任何改变状态的 git 命令。
- Date: 2026-11-14（文件名沿用 20261113 链接）。
- HEAD: 开始时 `git log -1` 为 `feb87aee75fa6b9dd051a4e35bf7df8b101edb75`；结束时 `.git/HEAD` → `refs/heads/codex/cross-workspace-collaboration` = `fb9f4c07059445cea41ed19e956b3eb1861817cc`。研究期间并行会话新增 2 个提交（`b1d2e54ea`、`fb9f4c070`），`git diff --stat feb87aee7 HEAD -- server .trellis/spec/server` 只涉及 `server/internal/daemon/collaboration*` 4 个测试文件，本文读取的文件均未变化，`git status --short` 对这些路径无输出。
- Caveat: 行号属于本轮读取；"已读"指读过该行所在代码，"推断"指由已读代码推出但未追到底。列名是 `issue.parent_issue_id`（方案和总表写的 `parent_id` 不存在）。不重复 usage 分源内容。

## 1. 可直接用于方案的结论

1. claim 分两个独立事务：`claimTask`（状态 `queued→dispatched`、`claim_generation+1`）先提交，handler 组装 payload 后再开 `FinalizeTaskClaim` 事务写 `mat_` token 和评论回执。两者之间没有共用事务，预算预留只能加入后者。
2. finalize 失败和响应丢失都**不是**"按原 claim 查询"：前者把同一行退回 `queued`，后者由 reclaim 再次 `claim_generation+1` 并重新 finalize、重新签发 token。旧 token 不删除，源码注释明确承认原 daemon 可能已在启动。
3. `claim_generation` 只在 claim/reclaim/finalize/回执/claim 前拒绝路径上做 CAS。`StartAgentTask`、`CompleteAgentTask`、`FailAgentTask`（daemon 入口）、各取消与清扫 SQL 只按 `status` 过滤，不带 generation。
4. 所有终态 writer 只证明队列行状态变化。清扫类 SQL 的注释自己说明 daemon 进程可能仍在本机运行；没有任何一处证明 provider 调用已停止。
5. finalize 中对"当前权限"的复核，对普通/自定义 agent 只有：密码会话版本（仅 password 模式）、installation 绑定与 admission、task 行身份、issue admission。originator 成员资格和 invoke 权限只对受限模板角色（Reporter/Reviewer）复核。
6. 删除 issue 靠 `001_init` 的 FK 级联物理删除 `agent_task_queue`，再级联 `task_usage`、`task_token`、`task_message`。直接子任务被 `DetachDirectChildIssues` 置 `parent_issue_id=NULL`，不保留原父关系。
7. 改父的循环检查在事务外、用连接池查询、最多上溯 10 层；事务内对新父只有无锁的 admission 读取。创建子任务时父行也不加行锁。
8. quick-create task 在 claim 和运行期间 `issue_id IS NULL`，父任务只在 `context.parent_issue_id` 里；完成后才由 `LinkTaskToIssue` 回填。按 `task.issue_id` 归属会漏掉"agent 在父任务下创建子任务"这次执行。
9. 现成的 workspace 级串行点存在：`iteration:<ws>` advisory 锁被 issue 创建、更新、删除、`StartTask` 共同获取。claim/finalize 不取它。
10. task 与 issue 两族锁的顺序相反，靠 `NOWAIT` 调和：issue 写路径是 fence→issue `FOR UPDATE`→task `NOWAIT`；claim 路径是 task `FOR UPDATE`→issue `FOR KEY SHARE NOWAIT`。

## 2. 问题 1：claim 与 finalize

路径前缀：`service/`、`handler/` 在 `server/internal/` 下，`queries/` 在 `server/pkg/db/queries/` 下。不存在 `tasks.sql`，相关 SQL 全在 `agent.sql`、`managed_admission.sql`、`managed_runtime.sql`、`task_token.sql`。

| 接点 | file:line / symbol | 真实行为 | 证明 / 不证明 |
| --- | --- | --- | --- |
| HTTP 单 runtime | `handler/daemon.go:3525` `ClaimTaskByRuntime` | `requireDaemonRuntimeAccess` → `ClaimTaskForRuntime` → `buildClaimedTaskResponse` → `FinalizeTaskClaim` → 写响应 | 证明 finalize 在写任何响应字节之前；不证明 daemon 收到 |
| HTTP 批量 | `handler/daemon.go:1740` `ClaimTasksByRuntime` | 校验 `daemon_id`、逐 runtime 授权，`ClaimTasksForRuntimes`，逐 task 走同一 finalize；失败的 task 被 requeue 并从批次省略 | 证明两个 HTTP 入口共用 finalize |
| WS RPC | `handler/daemon_rpc.go:48,66,111` `DaemonRPCHandler` / `rpcClaimTasks` | 仅 `tasks.claim`；先 `AuthorizeDaemonConnection`，再构造进程内请求调用 `ClaimTasksByRuntime` | 证明 WS 没有独立 claim 实现；不存在第三个 claim 事务 |
| claim 事务 | `service/task.go:3425` `claimTask`，`runInTx`（`:6548`） | 顺序：`lockManagedTaskSource` → `LockManagedRuntime` → `GetAgentForClaimUpdate`（agent `FOR UPDATE`）→ `CountRunningTasks` → `ClaimAgentTask` → `CaptureTaskExecutionBinding` | 事务在返回 handler 前已提交 |
| claim SQL | `queries/agent.sql:808` `ClaimAgentTask` | `managed_admission` CTE `FOR SHARE OF i`；子查询 `FOR UPDATE SKIP LOCKED`；`lock_issue_execution`（issue `FOR KEY SHARE NOWAIT`）；写 `dispatched`、`claim_generation+1`、`dispatched_at=now()`、lease | 复核 agent 仍绑定该 runtime、私有 runtime owner 匹配、runtime online 且心跳新鲜、同 (issue,agent) 无在跑任务；不复核 invoke 权限、agent 是否归档 |
| reclaim | `service/task.go:3627-3668`、`:3995` 附近；`agent.sql:965,1037` | 已 dispatched、未 started、超过 90s 恢复窗且 lease 过期的行：`claim_generation+1`；legacy 刷新 `dispatched_at`，managed 保留原值 | 证明响应丢失的恢复是**重新投递并换代**；不证明原 daemon 未收到 |
| finalize | `service/task.go:3735` `FinalizeTaskClaim` | 见下方锁序；`:3807` 比较 `status=dispatched`、runtime、issue、context、`dispatched_at`、`claim_generation`、token 的 task/agent/workspace | 任一不等返回 `ErrManagedRuntimeSource`，整事务回滚 |
| finalize 失败 | `service/task.go:3873`；`agent.sql:946` `RequeueAgentTaskAfterClaimFailure` | CAS(`id, runtime_id, status=dispatched, started_at IS NULL, dispatched_at, claim_generation`) → `queued`；generation 不回退 | 证明旧 handler 不能回滚更新的 reclaim；requeue 自身失败只记日志，行留给 reclaim |
| 不重发 token 清理 | `handler/daemon.go:3682-3685` | 注释："a stale reclaim can race the original daemon starting"，因此 finalize 失败后不按 task 删 token | 证明同一 task 可同时存在多代有效 `mat_` |
| claim 前拒绝 | `service/task_cancellation.go:26` `lockClaimedTask` | `GetCancellationTaskForUpdate` 后比较 runtime、`dispatched_at`、generation，不符返回 `ErrClaimSuperseded` | 仅 `CancelClaimedTask*` / `FailClaimedTask` 使用 |

finalize 事务内的锁序（已读 `task.go:3763-3861`、`managed_runtime.go:123-152`、`task_capability.go:17-58`、`task_token.sql:16-35`）：

1. `runInTx` 的 `lockManagedTaskSource`：密码会话锁 + installation namespace advisory（仅绑定的 daemon token）。
2. password 模式：`auth.LockPasswordSession`，把 `session.Version` 写入 token 的 `auth_version`。非 password 模式这一步不存在。
3. `LockManagedRuntime`：namespace advisory → 重读 runtime → `managed_installation FOR SHARE`。
4. `LockTaskForClaimFinalization`：task 行 `FOR UPDATE`（阻塞）。随后做 `:3807` 的身份 CAS。
5. `LockIssueExecution`：task 自己的 issue `FOR KEY SHARE NOWAIT`；拿不到锁按"不允许"返回 `admission.Blocked`。
6. `finalizeTaskCapability`：`TaskCapabilityProfile==""` 时直接返回。受限角色才取 `LockTaskCapabilityPrincipals`（issue `FOR NO KEY UPDATE NOWAIT`，task/agent/runtime/两个 member/两个 user `FOR SHARE NOWAIT`）和 invocation targets `FOR SHARE NOWAIT`，再比较 `CurrentTaskCapability`。
7. `CreateTaskToken` →（可选）daemon token → `SetTaskDeliveredCommentIDs`（再次 CAS generation）。

预算预留可加入的位置：第 4 步身份 CAS 之后、第 7 步之前，使用同一个 `qtx`。约束：此时已持有 task 行锁，且 issue 只有 KEY SHARE。root 行锁必须排在最后；任何持有 root 锁的路径不能再阻塞等待 task 行或 issue 行，否则与 `StartTask`/issue 写路径（先 issue 后 task `NOWAIT`）形成反向等待。finalize 只锁 task 自己的 issue，不锁父或祖先行。

## 3. 问题 2：task 生命周期 writer

| writer | file:line / symbol | 迁移与 CAS | 对"provider 已停止"的证明力 |
| --- | --- | --- | --- |
| start | `service/task_start.go:18`；`agent.sql:1124` `StartAgentTask` | `dispatched|waiting_local_directory→running`；先锁 workspace/状态目录/`iteration:<ws>`/issue `FOR UPDATE`，再 `LockAgentTaskForStart`（`FOR UPDATE NOWAIT`）；无 generation 条件 | 只证明 daemon 调了 start；不能区分是哪一代 claim |
| complete | `service/task.go:4417`；`agent.sql:1158` | `WHERE status='running'`；0 行时读现状并按幂等成功返回（`:4504-4514`） | 是 daemon 的自报；已取消/失败后的晚到 complete 被吞掉 |
| fail（daemon） | `service/task.go:4842`；`agent.sql:1389` | `status IN (dispatched, running, waiting_local_directory)`；同事务创建 retry 子行 | 同上；handler 把上下文耗尽的 complete 改走 fail（`handler/daemon.go:4089`） |
| cancel | `service/task_cancellation.go:67`；`agent.sql:1702,1710,1810` | 5 个活动状态均可 → `cancelled`；0 行时返回现状、不报错 | 只写状态。`UpdateAgentTaskSession`（`agent.sql:1434`）允许 cancelled 行继续补 session，说明取消后 daemon 侧仍有动作 |
| 按 issue/agent/comment/chat 批量取消 | `agent.sql:709,743,754,765` | 纯状态过滤的 UPDATE | 无 |
| 超时清扫 | `cmd/server/runtime_sweeper.go:611`；`agent.sql:1460` `FailStaleTasks` | 按 `dispatched_at`/`started_at` 加 lease 或心跳缺席；→ `failed/timeout` | 注释：running 行在心跳缺席满 grace 前不杀，"healthy multi-hour work" 可存活；失败后进程是否退出无证据 |
| runtime 离线 | `runtime_sweeper.go:315`；`queries/runtime.sql:260` `FailTasksForOfflineRuntimes` | `FOR UPDATE OF task SKIP LOCKED` + 应用时重查状态 | 注释："must not terminate a daemon process that is still running locally" |
| queued 过期 | `runtime_sweeper.go:649`；`agent.sql:1537` | `SKIP LOCKED` + 重查 `status='queued'` | 该行从未 dispatched，但不含 reclaim 历史 |
| daemon 重启恢复 | `handler/task_lifecycle.go:28`；`agent.sql:1443` `RecoverOrphanedTasksForRuntime` | 单条 UPDATE，按 `runtime_id` + 活动状态；无 generation、无事务包裹后续 retry | 依据是"新 daemon 进程自己报告"，不证明旧进程的子进程已结束 |
| 失败后处理 | `service/task.go:5905` `HandleFailedTasks` | retry、委派失败恢复、issue `in_progress→todo` 回退、事件 | 不是事务，逐项 best-effort |

重试谱系（已读 `agent.sql:546-660`、`task.go:5407-5560`）：

- 自动重试是**新行**：`CreateRetryTask` 复制 `issue_id/chat_session_id/autopilot_run_id`，`attempt+1`，`parent_task_id = retry_of_task_id = 原行 id`，继承 originator/accountable/`delegated_from_task_id`/squad。`parent_task_id` 的 FK 是 `ON DELETE SET NULL`（`migrations/055:22`）。
- 两条创建路径：`failTask` 事务内（与失败同提交），以及清扫/恢复路径的 `MaybeRetryFailedTask`（独立事务，`:5530`）。
- `ON CONFLICT ... DO NOTHING`：(issue, agent) 的 queued/dispatched 槽被占时不建 retry，调用方把 `ErrNoRows` 当"未重试"。
- `retryEligible`（`:5407`）排除 autopilot task，要求有 issue、chat 或 source-context quick-create。
- 人工 rerun 是另一条线：`rerun_of_task_id`（`CreateAgentTask` 参数），不写 `retry_of_task_id`。委派失败恢复另建 coordinator task（`task.go:6427`）。
- 同一 task 行被 reclaim 时**不建新行**，只换 generation。

## 4. 问题 3：ordinary issue 树 writer

| 路径 | file:line / symbol | 父关系处理与锁 | 缺口 |
| --- | --- | --- | --- |
| HTTP 创建（人、agent 经 CLI/API） | `handler/issue.go:2995,3068-3095,3199`；`service/issue.go:382` `CreateInTx` | 事务内取 workspace `FOR KEY SHARE`、状态目录共享锁、`iteration.LockWorkspace`；父用 `GetIssueInWorkspace`（无行锁）校验同 workspace 与 `admission.Formal`，未指定项目时继承父项目 | 父行不加锁；无深度限制 |
| handoff / triage / source-context 创建 | `handler/lifecycle_handoff_transaction.go:398,405`；`handler/triage.go:609`；`handler/source_context.go:746-749` | 都经 `IssueService.Create`/`CreateInTx`；handoff 与 source-context 把源 issue 设为父 | triage 是否带父未读 |
| autopilot create_issue | `service/autopilot.go:790-806` | 直接 `CreateIssueWithOrigin`，`ParentIssueID` 为空 | 不经 `CreateInTx`；其事务取了哪些 fence 未读 |
| onboarding | `handler/onboarding_shim.go:275,446` | 直接 `CreateIssue`，父为空 | 同上 |
| 更新 / 改父 / 清父 | `handler/issue.go:4149,4301-4338`；事务 `:3936` `updateIssueAtomicallyOnce`；`queries/issue.sql:159` `UpdateIssue` | 事务外：禁止自指、父须同 workspace、沿父链最多 10 层查环（`h.Queries`，`:4322-4334`）。事务内：`lockIssueWriteFences` → issue `FOR UPDATE` → 新父仅 `admission.Check`（无锁读）→ `revision` CAS | 环检查与写入不在同一快照；超过 10 层不检查；并发双向改父可成环属推断，未见事务内复查 |
| 批量更新 | `handler/issue.go:5247,5500-5539` | 逐项同样的事务外检查，再走 `updateIssueAtomically` | 同上 |
| move | `handler/issue_move.go:33-149` | 允许字段含 `parent_issue_id`、`project_id`，解析相对位置后委托 `UpdateIssue` | 是 workspace 内移动 |
| 删除（单个 / 批量） | `handler/issue.go:4769,5703`；事务 `:4859-4985`；`issue.sql:132,329` | 事务前 `CancelTasksForIssue` 与 `FailAutopilotRunsByIssue`（各自提交）。事务内：fence → 迭代锁 → `LockIssueDeleteRows`（所选及直接子，按 ID 序）→ 授权 → 有协作子任务则拒绝 → `DetachDirectChildIssues`（子的 `parent_issue_id=NULL`、`stage=NULL`、`revision+1`）→ `DeleteIssue` | 取消与删除不在同一事务；孙辈保持指向已脱离的子 |
| workspace 拆除 | `queries/workspace_delete.sql:254` | 批量置 `parent_issue_id=NULL` | 拆除专用 |
| 协作 issue 结构 | `queries/collaboration_source_fields.sql:27`；`service/collaboration_edit.go:117` | 独立 writer 写 `parent_issue_id` | 未追踪 |

删除的级联（`migrations/001_init.up.sql:65,130`；`032:3`；`108:21`；`026:3`；`042:57-58`）：`issue.parent_issue_id … ON DELETE SET NULL`；`agent_task_queue.issue_id … ON DELETE CASCADE`（`033` 只去掉 NOT NULL）；`task_usage`/`task_token`/`task_message` 的 `task_id … ON DELETE CASCADE`；`autopilot_run.issue_id/task_id … ON DELETE SET NULL`。未搜到后续迁移删除 `agent_task_queue_issue_id_fkey` 或 `task_usage_task_id_fkey`。`handler/daemon.go:1976-1979` 的注释也把这些称为仍在生效的 legacy FK。结论：issue 删除后其 task 行和 usage 行物理消失，应用层不做保留。

跨 workspace：`parent` 校验一律 `GetIssueInWorkspace`；`DeleteIssue`/`UpdateIssueStatus` 带 `workspace_id` 谓词；对 `queries/*.sql` 的有限 grep 未发现 `UPDATE issue … SET workspace_id`。未发现 issue 跨 workspace 移动的 writer（grep 范围有限，见 Caveats）。未发现 duplicate/clone issue 的 handler；`FindActiveDuplicateIssue` 是创建前的重复拦截，不是复制。

## 5. 问题 4：哪些执行有 `issue_id`

| 执行来源 | 创建 SQL | `issue_id` | 归属线索 |
| --- | --- | --- | --- |
| 指派触发 / 评论触发 | `agent.sql:337` `CreateAgentTask`（`service/task.go:1233`） | 有 | 单语句内 `lock_task_owner_rows` + `lock_issue_execution`，不开事务 |
| mention / thread parent / squad leader / 委派 | 同一 SQL（`task.go:1397-1460`） | 有，同一 issue | `is_leader_task`、`squad_id`、`delegated_from_task_id`；每个 (issue, agent) 一个 queued/dispatched 槽（`idx_one_pending_task_per_issue_agent_v2`），不同 agent 可并行 |
| 延迟升级 / channel issue | `agent.sql:392,491` | 有 | 先 `deferred` 后提升 |
| 自动重试 | `agent.sql:546` | 继承原行 | 见 §3 |
| quick-create | `agent.sql:459`；回填 `:530` `LinkTaskToIssue` | 创建到完成期间为 NULL | `context.parent_issue_id`（`task.go:1689`）；`lock_issue_execution` 会把它当作 admission 所属 issue |
| chat | `queries/chat.sql:1071` `CreateChatTask` | NULL | `chat_session_id` |
| autopilot run_only | `queries/autopilot.sql:592`（`service/autopilot.go:1177`） | 字面量 NULL | `autopilot_run_id` |
| autopilot create_issue | 先建 issue（父为空），再走指派触发 | 有 | 新 issue 是顶层 |
| 协作 / host run | `collaboration_run`（`migrations/571:82`，`task_id` 可空 `594`） | 未追踪 | `lock_issue_execution`（`migrations/650`）对存在 `issue_collaboration` 的 issue 返回 false，普通队列拒绝这类 issue |

子任务上的执行：未见"为子 issue 派发"的专用路径；子 issue 被创建并指派后走普通的 `maybeEnqueueOnAssign`（`service/issue.go:864`）。task 行不保存父或根，只有 `issue_id`。

## 6. 问题 5：权限接缝

| 接缝 | 出处 | 当前事实 |
| --- | --- | --- |
| 人工专属 | `handler/actor_guards.go:96,110` `RequireHumanActor` | 只拒绝 `X-Actor-Source` 为 `task_token`、`cloud_pat`；未知来源按人放行（注释称有意如此）。`mdt_` 不在此名单 |
| `mat_` 认证 | `internal/middleware/auth.go:114-148` | 按 hash 查未过期 token，用 token 绑定值**覆盖** `X-User-ID`(=runtime owner)、`X-Agent-ID`、`X-Task-ID`、`X-Workspace-ID`。`X-User-ID` 是 owner，不是人工操作证据 |
| 受限角色 ceiling | `.trellis/spec/server/task-api-capabilities.md:15-25` | Reporter/Reviewer 每次请求复核 originator 成员 + invoke、runtime owner 成员、generation、终态；其他角色与自定义 agent 保持 legacy 权限 |
| daemon 的 task 接口 | `handler/daemon.go:151` `requireDaemonTaskAccessWithWorkspace` | workspace 归属（`mdt_` 比较 workspace，PAT/JWT 查成员）+ `ValidateManagedTaskBinding` + runtime 访问。complete/fail/usage 都只过这一关 |
| claim 时的拒绝 | `handler/daemon.go:2236-2316,2372` | agent 不存在、agent 已换 runtime、私有 runtime owner 不匹配、受限角色缺 issue/originator 或带外部工具 → `failClaimedTaskBeforeLaunch` |
| 成员移除 | `handler/workspace_revoke.go:102-134,156` | 同事务：按 runtime 和已归档 agent 取消 task、runtime 强制离线、删 daemon token、删该成员的 invocation target。注释：`ClaimAgentTask` 不按 `agent.archived_at` 把关 |
| 终态后 token | `handler/daemon.go:4155,4848` | complete/fail 后 best-effort 删 `mat_`；否则 24h 过期 |

在途 task 被撤权后的行为：已读代码中，invoke 权限只在入队（未读细节）、受限角色的 finalize 与每次 `mat_` 请求处复核。普通 agent 的在途 task 在 complete/usage 上报时不复核 invoke。单独修改 invocation target 是否取消在途 task：未找到。成员移除会取消 task 并删除 daemon token，此后该 daemon 的 usage 上报应被拒（由 token 删除推断，未追 `mdt_` 中间件）。

## 7. 问题 6：已有并发原语

| 原语 | 出处 | 说明 |
| --- | --- | --- |
| `iteration:<ws>` advisory（事务级、排他） | `internal/iteration/transaction.go:83`；`handler/issue.go:3860-3866`；`service/issue.go:391`；`service/task_start.go:46` | issue 创建/更新/删除与 StartTask 共用的 workspace 级 fence |
| `<ws>:issue_status` 排他/共享 | `queries/issue_status.sql:122,132` | 归档取排他，issue 写取共享 |
| `(hashtext(ws), hashtext(user))` | `queries/subscriber.sql:57`；`handler/issue.go:3862` | 每 workspace-用户串行 |
| `hashtextextended(key, 81271465)` | `queries/managed_installation.sql:17`；try 版 `collaboration_authority.sql:75`、`task_knowledge.sql:41` | installation namespace 族 |
| `hashtextextended(key, 0)` 文本键 | `queries/issue.sql:292`（重复键）、`plugin.sql:13`、`handler/property.go:775`、`squad_template.go:370`、`agent_template.go:365`、`triage_import.go:416` | 通用按键串行 |
| 常量键 | `4246`（usage rollup、`workspace_delete.sql:17`）；`70856394211471801`（`admin.sql:13`、`delivery_completion_collaboration.go:103`）；seed `728401`（`admin_alert.sql:2`） | 新键需避开 |
| 会话级 advisory | `internal/seatcapacity/workspace_locker.go:91`；`cmd/migrate`、各 backfill | 非事务级 |
| 行锁约定 | `managed_admission.sql:2,5`；`agent.sql:1908,2801-2808`；`issue.sql:92-99,123-130`；`task_token.sql:16-35`；`delivery_policy.sql:32` | task/agent/issue `FOR UPDATE`；跨族一律 `NOWAIT` 或 `SKIP LOCKED` |
| revision / CAS 列 | `issue.revision`（`issue.sql:204,253` 双重检查）；`claim_generation` + `dispatched_at`；`execution_binding_epoch`、`execution_admission_version`；task `status` | — |
| 幂等键 | `.trellis/spec/server/delivery-policy-evaluations.md:34-41,70-78` | 命名空间 = workspace + 已验证人工 evaluator + request key；规范请求 digest；同键同请求恢复、不同请求冲突；commit 不明返回非成功；不建 FK |
| 部分唯一索引 + `ON CONFLICT DO NOTHING` | `migrations/257`；`agent.sql:657` | 每 (issue, agent) 一个待执行槽 |

已成文的锁序声明：

- `service/managed_runtime.go:121-122,146`：`LockManagedRuntime` 须在 agent/task 锁之前；admission 与绑定变更先于 agent/task 锁串行。
- `agent.sql:849`：claim 调用方在 agent 锁之前持有 namespace fence。
- `agent.sql:2806-2808`：start 先锁 workspace/issue，task 用 `NOWAIT`，"avoids reversing another task-owner/issue writer's lock order"。
- `task_token.sql:17`：capability 锁用 `NOWAIT`，不向 task/issue 路径引入等待。
- `handler/issue.go:3898-3900,3962,4029,4877,4988`：iteration 先于 issue；状态目录锁先于附件与 issue 行锁；持 issue 锁时项目用 `NOWAIT`；删除按全局 ID 序锁 issue；agent/task 引用锁用 `NOWAIT`。
- `service/task_cancellation.go:65`：chat-session → task。
- `service/issue.go:379-381`：借用事务的调用方须先取目录/迭代 fence 再锁源 issue。
- `migrations/284`：`lock_task_owner_rows` 顺序为 workspace（按 id）→ agent → issue → runtime，均 `FOR KEY SHARE`。

未找到一份覆盖全部路径的统一锁图文档。

## 8. 源码与方案 §3/§6/§7 的矛盾或缺乏支撑之处

1. **§6"领取最终化：失败/响应丢失按原 claim 查询，不能补造下一次 attempt"**——现状相反。失败走 requeue，丢响应走 reclaim 换代并重新 finalize、重新发 token（§2）。把预留写进 finalize，同一 task 会在每次重投时产生新一代预留，而前一代是否已启动无从证明。
2. **§5.1/§7.2 把 `task UUID + claim generation` 当执行身份**——generation 不随 start/complete/fail/usage 上行（§3），服务端无法把 daemon 的后续回调绑定到某一代。`StartAgentTask` 无 generation 条件。
3. **§7.2"复用已有 … invoke 复核"**——finalize 中的 invoke/originator 复核只对受限模板角色存在（`task_capability.go:22-28`）。普通 agent 没有可复用的此项复核；非 password 模式也没有凭据版本锁。
4. **§3.1"普通子任务/task 删除：留 tombstone，不能通过级联清掉消耗"**——当前删除 issue 会经 FK 级联物理删除 task 与 usage 行（§4）。预算记录若要保留，必须不依赖这些行存活；启用前的历史在删除后不可回查。
5. **§3.1"父任务删除 … 剩余子孙仍沿已保存的根"**——当前删除父任务会把直接子任务的 `parent_issue_id` 清空，源码里没有任何原父记录可供事后恢复。
6. **§3.1"按现有锁序在同事务检查 … 遍历边界/循环"**——不存在可复用的事务内遍历。现有查环在事务外、10 层封顶；父行在创建和改父时都不加锁（§4）。现存数据可能深于 10 层，是否已有环未验证。
7. **§3.1"所有相关创建/改父/删除 writer 必须参加 fence"**——`iteration:<ws>` 已覆盖 HTTP 创建/更新/删除/StartTask，但 autopilot create_issue、onboarding、协作结构 writer、workspace 拆除是否取它未证实；claim/finalize 确定不取。
8. **§1"只支持普通任务可证明的执行归属"按 `issue_id` 判定**——quick-create 在父任务下创建子任务的那次执行 claim 时无 `issue_id`（§5），会落到"无 issue 执行"一侧。
9. **§6"取消/超时/进程退出：只证明该生命周期事实"**——源码支持，且更强：complete 在行已终态时按幂等成功返回，cancelled 行仍接受 session 回填。
10. **§3.1"根/成员跨工作空间"**——未发现 issue 跨 workspace 的 writer；该行是防御性条款，源码既不支持也不反驳。
11. **§7.1"daemon 也被撤权则拒绝，费用保持 unknown"**——与成员移除路径一致（task 被取消、daemon token 被删）。但"已撤 invoke 即禁止新预留"在 claim 路径上没有现成检查点可挂（见第 3 点）。

## 9. Caveats / Not Found

- 未读：`buildClaimedTaskResponse` 全文（只读了权限相关片段）、`ClaimTasksForRuntimes` 全文（只确认了事务与调用点）、`finalizeTaskKnowledgeClaim`、`auth.CurrentTaskCapability` 内部、`mdt_` 中间件、daemon 侧如何得知取消并停止进程、`LockIssueDeleteRows` 的锁模式、`iteration.LockWorkspace` 函数体（由 `WorkspaceFenceSQL` 推断）。
- `security-boundaries.md` 只做了关键字 grep，未通读；`delivery-policy-evaluations.md` 读了 §3，`task-api-capabilities.md` 读了 §1-§4。
- 协作/host run 的执行与用量链未追踪；`collaboration_run` 与 `agent_task_queue` 的关系只看到表和索引定义。
- "无跨 workspace writer""无 clone handler""FK 未被后续迁移删除"均基于有限 grep，不是穷尽证明；也不证明生产库的迁移状态。
- 并发成环、成员移除后 usage 被拒，是由已读代码作出的推断，未运行验证。
- invoke 授权（`agent_invocation_target`）变更对在途 task 的影响：未找到处理代码。
- 入队时的 invoke 检查位置（`canInvokeAgent`，`handler/agent_access.go:50`）只定位未阅读。

## 10. SHA-256：本轮读取的关键文件

| 路径 | SHA-256 |
| --- | --- |
| `server/internal/service/task.go` | `8ad09048e05873b9723045682352bcb85d309be90d8584631d9d2b83fe817481` |
| `server/internal/service/task_capability.go` | `6aff9cd37776e74a6e46e26d69e2c59b99b813df70d099cc6027b16679159b23` |
| `server/internal/service/task_cancellation.go` | `f161458f5384d5e7d18809a6769b9e98417fb2eee6523a06c693a26f32b00d0f` |
| `server/internal/service/task_start.go` | `f63236fbff502d5a23699b3bff357e009110c6073a287c2e4499e7b248a55979` |
| `server/internal/service/managed_runtime.go` | `768925aac9df0a05bd21f6d8c33918d7f238a037d3305b2ff0d1460dc1c0a785` |
| `server/internal/service/issue.go` | `d1f7c8f5aa6e621e117cf97e1f4f2da496ccaeba9dabb7f3fdef12e97cff8ba2` |
| `server/internal/handler/daemon.go` | `c2773379f601e54fd71a66ccd1d3f5df63b6aa7506f8e11003fb7cfdbe34e157` |
| `server/internal/handler/daemon_rpc.go` | `5fb862e72fff347a5fa15b0c71954608b8467bb49404c4abd5c7796fa967da55` |
| `server/internal/handler/issue.go` | `417744af42e0becbb07ff83116a61fc55d4ba067f0041aaa5fa02faacb22563b` |
| `server/internal/handler/issue_move.go` | `d24a2721b68acc381d80f7389aa786ad71c13dbec2e9adb9df79af69b227de45` |
| `server/internal/handler/task_lifecycle.go` | `4f55f59bcdd3d23a28df0fc3d9cdd8d017be9797aadac6e117f106ff9e6d5e0c` |
| `server/internal/handler/actor_guards.go` | `d7444bb08db5505abcef91e6c4361acef38208ad6eb2569bb26f1f027df1a77a` |
| `server/internal/handler/workspace_revoke.go` | `e66a530adc17bd8414f2ef323a0f80d4b698c1a35a4b5a99e99704ec1368f72d` |
| `server/internal/middleware/auth.go` | `18392905c77f59897c63b4bc2490b20c95389bf49c88748e934643ca9b6e5dfe` |
| `server/internal/admission/admission.go` | `089fd33d6e37161dc62581896139797032f0863f3f68670d709272cc3eb03614` |
| `server/cmd/server/runtime_sweeper.go` | `88c059cb71140a9a8bf30ffc0a0a5ab1e1ed5d4d6e00e441f022321c21e46862` |
| `server/pkg/db/queries/agent.sql` | `4d900ec193fb94e9b3c8f4835224ab64035872a85788bb2b42526cd0ea3999a1` |
| `server/pkg/db/queries/issue.sql` | `1a83e00afbfab08d8d75a1cf9c1d9269a209216f9fa24c1e2cc6a2980d98583e` |
| `server/pkg/db/queries/runtime.sql` | `1f7e9ff10b5086c1e38d34f08c7fe7d1bb80656251bbaed3f2daf24c6f52e5b8` |
| `server/pkg/db/queries/task_token.sql` | `7ce1e5654ded18ec5e2734b776f49a5d4fb2f9539abf97e3c24c5f1f4102b0aa` |
| `server/pkg/db/queries/managed_admission.sql` | `6e444d1b5a231ed1d6ffd728875cd8ecc9a807dd6b38acae6295925427d3f977` |
| `server/pkg/db/queries/managed_runtime.sql` | `ce4f8440d1898f33158c68a426e155ab7baffbe429c0375c36b32790a96eccd8` |
| `server/pkg/db/queries/autopilot.sql` | `024687e22fc734941f93bd85eda60420f9959435489a29e40a03560c0db2a135` |
| `server/pkg/db/queries/chat.sql` | `c06f67e13cbd982e2f3544e123b1cce8aa13e9b362ef1010361800619512807d` |
| `server/migrations/001_init.up.sql` | `88bbaf79a586d195cc5d9c770f6b0aaa53cbc5a663672b594c759ede3d38784e` |
| `server/migrations/032_task_usage.up.sql` | `bf72146798f8f9dc6cfaab987b6ed745cea4ce84ca7f725fbc18718e7efc2b5d` |
| `server/migrations/284_task_owner_row_fence.up.sql` | `d84ea77232e6c71b50db4ee9a86267b6b1953305963216e12762d95b04a6286a` |
| `server/migrations/650_collaboration_legacy_execution_fence.up.sql` | `867c1f2ba62c90c820b5075615e5b69f2db3e905f4edac0f7862083446de5df3` |
| `.trellis/spec/server/task-api-capabilities.md` | `7daa1ea5610b6d3917b8e4816d2f89d09f4dae97b166bdd8e0f273ef5b9f4a07` |
| `.trellis/spec/server/delivery-policy-evaluations.md` | `874258b2c662cee13022ac5c20ba5b2ddb276c28bdadfb5dbd16a93b14816fa8` |
| `.trellis/spec/server/security-boundaries.md` | `eafe18806e77423c13062317ab9168019f0148131107179bfdd162876957e2cf` |

`task.go`、`agent.sql`、`handler/daemon.go`、`daemon_rpc.go`、`task-api-capabilities.md` 的摘要与 usage 分源文件记录的一致。
