# F3-S1 技术设计

权威合同是父任务的 [F3 最小合同](../../../11-12-agent-squad-followup-roadmap/research/f3-minimum-budget-contract-20261113.md)。其 §11（R1–R10）是对照源码后的修订，与 §3/§5/§6/§7 冲突处以 §11 为准。本文只记录已定的设计骨架、接点和本次授权后固定下来的选择；不重复合同正文。

合同写于 HEAD `fb9f4c070`。到 2026-11-16 的 `86fea651c` 之间有 70 多个提交，`server/internal/service/task.go`、`server/internal/handler/daemon.go`、`server/cmd/server/router.go` 三个接点文件都有变动。合同里的行号一律视为过期，实施第 1 步重新核对；核对结果写入本任务 `research/`，与合同冲突时以新核对为准并在此追加修订。

## 1. 选择与边界

- 根绑定普通父任务 `(workspace_id, root_issue_id)`，终身一个，另发稳定 `budget_root_id`。
- 准入状态 `open / closed / tombstoned` 与结算状态 `unsettled / settled_as_of_revision` 是两条轴；业务状态变化不动预算（D2）。
- 首包只观察：记录影子预留、已知实际、估算、未知与 `would_refuse:<reason>`，不改变任何执行结果（D5）。
- `autopilot_quota`、`task_usage`、hourly 都不是预算账本；首包不读它们算金额，也不改它们。

## 2. 数据职责

五类，全部无 FK，保存 workspace/issue/task/runtime/agent 的 ID 副本，因此现有 issue 删除的级联不会带走预算行：

1. root/control：唯一业务锚点、启用点、policy 与 ledger revision、准入状态、净投影、最小 tombstone。
2. member binding：普通任务唯一 root、继承来源、原父 ID、绑定时间/版本。
3. attempt/invocation：task/claim/runtime 与调用/计量范围、原 reservation、未决状态。quick-create 的 attempt 没有 `issue_id`，按 `context.parent_issue_id` 归根（D6）。
4. immutable entries：原观察、更正、预留转移、释放证据与 source/schema/version；净投影由同一 reducer 重放得到。
5. operations/receipts：原 key、规范意图 digest、已提交结果。

表布局见 §8（六张表：五类职责各一张，另加只追加的观察表）。每个索引单独一条 CONCURRENTLY 迁移。

## 3. 计量

- `1 tick = 10^-10 USD`，Go/DB 用有界整数；对外金额和大计数用规范十进制字符串，拒绝小数、指数、溢出，不经 float64 或 JS number。
- `actual_known_ticks`、`estimated_ticks`、`reserved_ticks`、`unknown_hold_ticks` 分开；同一份额度不重复计入；从 reserved 转入 unknown_hold 不产生可用额度；没有上界证明时 `remaining_ticks = null`。
- 价格快照由服务端冻结（provider、model、adapter/normalizer 版本、price version/digest、各维度整数费率、舍入版本）；调用方不能自报价格；旧事件重放不用当前价格。
- 累计只追加差额；重复返回原收据；迟到旧值不倒扣；无依据的下降被隔离；显式 correction 可降低净值并保留原记录。

## 4. 接点

| 接点 | 位置 | 做什么 |
| --- | --- | --- |
| 成员绑定 | issue 创建/改父/删除路径，已持有 `iteration:<ws>` fence 处 | 同事务写成员行；启用根时用自己的递归查询（带环检测、不设 10 层上限） |
| 影子预留 | `FinalizeTaskClaim` 事务内，收据 UPDATE 之后、回调返回之前（V11） | 写 attempt、预留、收据，签发 `budget_attempt_id` 并随 launch payload 下发 |
| 用量事件 | 新的版本化上报入口；现有 `ReportTaskUsage` 保持不变 | 一批一个 attempt、整批原子、原 key 幂等、逐事件收据 |
| 管理与读取 | exact root 的人工 API 与 `issue budget` CLI | 允许名单的人工来源 + 当前 owner/admin + 根任务编辑权 |

- 重领产生新 attempt；前一代标 `superseded_unproven`，其预留转未知占额、不释放。`claim_generation` 不上行，不能当执行身份。
- 人工入口用允许名单识别来源，不复用 `RequireHumanActor`（它对未知来源放行）。
- 任何终态都不是释放依据（R10）。

## 5. 锁序

2026-11-16 按独立复核（§7 的 V11–V14）重写。原先的“根行锁在所有路径中排最后”对 issue 写路径并不成立，改成下面五条：

1. **issue 写路径不锁、不写根行。** 创建、改父、删除只在已持有的 `iteration:<ws>` fence 下写成员行和观察行，不锁、不更新任何根行。根任务被删除表现为它自己那条成员行的 tombstone，根的 `tombstoned` 状态由此推出，下一次持根锁的操作再落到根行上。2026-11-17 增补：允许对根行做不带锁的普通读取，用途只有读开关（§10.6），并且只在确实有观察要写的分支里读；没有根的任务树不会走到这次读取。
2. **取根行锁之后只写不带 FK 的预算表。** 不写 `task_usage`，不取 task、issue、user、agent、workspace、runtime 行锁，不取会话、namespace、installation 锁；这些如果需要，一律排在根锁之前。
3. **锁根的只有三类事务**：领取最终化（task → issue NOWAIT → … → 根，根是该事务最后一个取锁语句）、新用量入口（会话/namespace 前缀 → 根）、人工管理（权限复核 → 根）。启用根要写成员行，所以先取 fence，再插入根行。一个事务需要锁多个根时只能在 fence 下、按 id 升序。2026-11-17 增补第四类：生命周期对账（§10.3），只取根锁、不取任何其他锁。
4. **成员绑定一经写入不换根。** finalize 读成员行后再锁根不会落到错的根上；剩下的竞态只有“读时尚未绑定、随后被绑定”，由绑定方在同一事务里登记在途任务来覆盖（V13）。2026-11-17 更正：只靠扫描并不闭合，还要一把绑定锁，见 §10.10（V19）。
5. **到达预算接点的事务都是 READ COMMITTED**（2026-11-17 增补，V20）。fence 与绑定锁的论证都依赖一点：取到锁之后新发的语句能看到此前已提交的行。REPEATABLE READ 的事务在取锁之前就定了快照，等锁期间别人提交的成员行、新领取的 task 它都看不到，两把锁因此都不闭合。当前 HEAD 上创建、改父、删除、启用各路径用的是默认隔离级别或显式 READ COMMITTED（主会话 grep 核对，第 4 步独立复核再核一遍）。仓库里另有没有声明只读的 REPEATABLE READ 事务（项目写入的 `runProjectTransaction`、`iteration_history.go`、`iteration_lifecycle_preview.go`，以及第 4 步复核补上的第四个：`collaboration_stage_evidence_read.go` 里的 `readCollaborationStageEvidence`，N18），它们不得调用绑定函数。第 5b 步在绑定函数和 finalize 的预算步骤里加断言：读到成员行之后检查当前事务的隔离级别，不是 READ COMMITTED 就报错、让这次写入失败。断言只在有成员行的分支里做，没有根的任务树不多一次往返；从 REPEATABLE READ 调进来的代码在任何带预算的树上第一次运行就会失败，不会只在竞态窗口里悄悄漏记。

第 5 条的增补（2026-11-18，第 4 步复核 N1，报告第 12 节）：

- 复核逐个读了事务的开启处。到达预算接点或属于预算 service 的事务里，只有协作工作流（`workflowInTx`）自己声明了 READ COMMITTED；其余都是 `Pool.Begin` 发出的裸 `begin`，级别取自服务器的 `default_transaction_isolation`。四个没有声明只读的 REPEATABLE READ 事务都到不了接点（逐个阅读，加调用图）。
- 预算 service 自己的事务（`transact`：启用与四个管理操作）在第一条语句声明 READ COMMITTED，写法与 `workflowInTx` 相同，5b 做。启用是绑定方，它的在途扫描和重放判断都依赖这个级别；这是 F3 自己开的事务，代价只是管理操作多一次往返。
- 其余到达接点的事务（创建、改父、删除和它们的上层入口）与 finalize 不加声明，仍用上面的断言：读到成员行之后检查。F3 不改既有事务的开启方式。
- 部署前提：数据库的 `default_transaction_isolation` 是 `read committed`（PostgreSQL 的出厂值）。产品在别的级别下本来就不能正常工作：复核把默认值改成 `repeatable read` 做了实验，issue 编号分配在并发下报 40001；删除一个 issue 时，如果它的 iteration 归属在删除等 fence 期间被另一个事务改过，删除返回 500（`TestDeleteIssueIterationSeesJoinAfterFenceWait`，起因同样是 40001；复核去掉墓碑接点重跑结果相同，与第 4 步无关。同日更正：这里原先写成“删除 iteration 返回 500”，被删的是 issue）。第 8 步把这条前提写进 spec。
- 前提不成立时的残留：断言只在读到成员行时触发，所以每棵树上的第一个洞拦不住。两种情形：子任务创建与启用（或移入）竞态，子任务没有成员行也没有观察（复核已复现）；finalize 的快照早于绑定方提交、领取又晚于绑定方的在途扫描，这次执行既没有 attempt 也没有缺口。之后这棵树上凡是读到成员行的写入和领取都会失败（领取反复 requeue），问题是响亮的；已经漏掉的成员行用复核报告 5.4 的反连接查询找出来。
- 第三种残留（同日，复核在裁定之后补测，报告 12.3 的 `rre_rr`）：删除与启用竞态。删除在等 fence 期间启用绑定了这个 issue，删除的快照早于启用提交，墓碑那条 `UPDATE` 匹配不到行：删除照常返回 204，issue 没了，成员行留着且没有墓碑。墓碑是一条不先读取的更新，没有读到成员行，断言无从触发，之后对这一行也没有哪次写入会响（这棵树的其它成员下一次有接点读到成员行时照样会响，与前两种相同；一直不响的只是被删的这一行，复核报告 18.2 的 R3）。被删的若是根 issue，根永远不会被标成已删除，`Reopen` 不再以 `root_deleted` 拒绝（B13）。在墓碑语句之前检查级别能拦住，代价是每个删除事务多一次往返，不管有没有根，第 5 条有意不付这个代价；所以它与前两种一样留给部署前提，靠 5.4 的第二条查询（成员行没有墓碑、issue 已不存在）发现。
- 启动时检查 `SHOW default_transaction_isolation` 能把这条前提变成启动失败或告警。它管的是整个产品，不属于 F3，本任务不做，作为选项报给用户。

工作空间整体删除不取 fence、不写也不锁任何预算行（V3、V8）。完整锁图见 `research/e2-lock-graph.md`，其中的“BR lock (last)”以本节为准。

## 6. 兼容与回退

- 响应为新增字段与新增端点；客户端 schema 经 `parseWithFallback` 解析。
- 旧 daemon 不带 `budget_attempt_id` 照常执行，记为覆盖缺口，不是错误。
- 回退是停用观察开关：停止新观察，账目保留、只读仍可用；不做破坏性 down，不把金额降为 0。
- 未启用根的任务树零影响：没有成员行就不写任何预算行，finalize 多两条语句：一条按主键读 issue 的工作空间并取必然成功的 try 锁（§10.10、§10.2 的 2026-11-18 补充），一条成员索引查找。
- 混合版本（2026-11-18，第 4 步复核 N2）：没有预算接点的旧实例在滚动升级期间仍能创建或移动子任务，落在已启用根的树下时既没有成员行也没有观察。首包不为此加开关或兼容路径，规则是：所有实例都换成带接点的版本之后才启用预算根（启用入口到第 7 步才存在）。已经有根之后把服务端回退到旧版本，会重新打开这个窗口。升级或回退之后用复核报告 5.4 的两条只读查询各核对一次，结果都应为空：第一条列出“父任务是成员、子任务不是”的行（漏掉的绑定；6.5 万 issue、5.7 万成员行上 61–89 ms）；第二条列出“成员行没有墓碑、issue 已不存在”的行（漏掉的墓碑；同样数据上 60–71 ms）。第二条是 2026-11-18 复核在裁定之后补上的：旧实例没有墓碑接点，它删除成员 issue 必然留下这样的行，不是竞态；被删的是根 issue 时，根不会被标成已删除。两条都只是探测，不修复，也都只列出第一个洞（没绑上的子任务自己的子任务，父任务本来就不是成员，第一条不会列出）。第 8 步写进 spec。

## 7. 修订记录

（实施中发现与合同或本文不符的源码事实，在此追加，注明日期和证据文件。）

### 2026-11-16：E2 写入口清点（HEAD `86fea651c`）

证据：[research/e2-writer-inventory.md](research/e2-writer-inventory.md)。主会话对照源码抽查了插入调用点、裸 SQL、删除事务、工作空间删除和 `QuickCreateContext`，与清单一致。

| # | 源码事实 | 对设计的修订 |
| --- | --- | --- |
| V1 | 合同 R3 的“取消 task 在删除事务之外先提交”已不成立：`CancelAgentTasksByIssue` 与 `FailAutopilotRunsByIssue` 现在在删除事务内、NOWAIT 取锁之后执行（`handler/issue.go` 约 4951–4961）。FK 级联与 `DetachDirectChildIssues` 不变 | 删除路径的成员 tombstone 与取消在同一事务内，B56 中“取消先于删除事务提交这一顺序”的前提改为“取消与删除同事务”；预期结果（不释放）不变 |
| V2 | 创建 issue 只有 2 条 sqlc 查询、5 个调用点；11 条创建入口中 5 条可带父任务（HTTP 创建、评论转子任务、生命周期交接、协作创建、协作阶段子任务）。除工作空间整体删除外，所有创建/改父/删除都取 `iteration:<ws>` fence；合同 R5 中未证实的 autopilot、onboarding、协作、交接、分诊均已证实取 fence | 成员绑定挂在这 5 条可带父任务的入口和 4 处改父写入上即可；B60 可从“对账发现缺口”改为逐路径断言 |
| V3 | 工作空间整体删除（`PrepareWorkspaceDeletionLinks` 批量清父 + 批量删 issue）不取 fence，靠 workspace 行 `FOR UPDATE` 与其他 writer 的 `FOR KEY SHARE` 互斥 | 这是唯一的 fence 缺口。预算行无 FK，随工作空间删除保留为 tombstone（D2）；该路径不写成员行，只需保证删除后公共读与新上报被拒。具体处理在第 4 步定，写入 B13/B14 的相邻用例 |
| V4 | quick-create 没有自己的 issue writer：智能体经 HTTP 创建并带 `origin_type=quick_create`。`QuickCreateContext.ParentIssueID` 在入队、领取、finalize 都做准入检查和加锁，也参与身份 CAS，并随领取载荷下发；但创建 issue 时服务端不把请求里的父任务与它比对（`service/issue.go` 约 582–633） | D6 的归根拆成两半：那次执行的 attempt 按服务端保存的 `context.parent_issue_id` 归根；新建的子任务按它实际落到的父任务在创建事务内绑定成员。两者不一致时记一条差异观察，不假设相同，也不在首包新增拒绝（observe only）。B55 的 expected 相应补充 |
| V5 | 合同 R4 仍成立但行号已变（单条约 4338、批量约 5563）；另有协作 issue 自己的事务内、不设上限、带锁的遍历（`collaboration_source_fields.go` 约 144–166） | 启用根的递归遍历仍用自己的查询；协作结构 writer（`UpdateCollaborationIssueStructure`）是第 4 处改父写入，需同样接成员绑定 |

### 2026-11-16：E2 锁图（HEAD `86fea651c`）

证据：[research/e2-lock-graph.md](research/e2-lock-graph.md)。合同 R1、R2、R6–R10 在当前 HEAD 仍成立，行号已变。主会话抽查了 finalize 的 CAS 到发 token 一段、两个 finalize 调用点、`task_usage` 的外键和 `ReportTaskUsage` 全文，与锁图一致。

| # | 源码事实 | 对设计的修订 |
| --- | --- | --- |
| V6 | finalize 事务内的顺序是：`LockTaskForClaimFinalization`（task FOR UPDATE，约 `service/task.go:3802`）→ 身份 CAS（约 3806）→ 知识 claim → `LockIssueExecution`（issue KEY SHARE NOWAIT，约 3819）→ `finalizeTaskCapability`（约 3829）→ `CreateTaskToken`（约 3832）。只有两个调用点：`handler/daemon.go` 约 1911（批量，WS `tasks.claim` 也走它）和约 3663（单条） | 影子预留写在 `finalizeTaskCapability` 之后。相对 `CreateTaskToken` 的先后见 V8 |
| V7 | `task_usage.task_id` 带 `REFERENCES agent_task_queue(id) ON DELETE CASCADE`（`migrations/032_task_usage.up.sql:3`，之后没有迁移删除它）。写 `task_usage` 会对 task 行取阻塞的 KEY SHARE（推断）。finalize 持有该 task 行 FOR UPDATE 再等根行；若某个用量事务先锁根再写 `task_usage`，在“任务被重领并 finalize、旧一代 daemon 同时上报用量”时成环。现有 `ReportTaskUsage` 没有状态或 generation 检查，也没有事务，逐行 warn 后仍返回 200 | 新用量入口在持有根行锁期间**只写不带 FK 的预算表**，不写 `task_usage`、不锁 task 或 issue 行；归属从已保存的 attempt 行推出。现有 `ReportTaskUsage` 保持不变且不接触根行。若以后要在同一入口维护旧视图投影，必须在取根锁之前或另一个事务里完成。第 6 步加一条双连接回归：重领 finalize 与旧一代用量上报并发，不死锁 |
| V8 | 持根锁后 finalize 还会经 token 插入的外键等待 agent / user / workspace 行（推断）；工作空间整体删除持有 workspace 行 FOR UPDATE（V3） | 两条约束一起保证无环：工作空间删除路径**不锁预算根行**（它不写预算行，V3）；根行锁是 finalize 事务里最后一个取锁语句——第 5 步读完 `CreateTaskToken` 之后到提交之间的全部语句，把预留放到其后，除非那里有必须排在预留之后的取锁 |
| V9 | `claim_generation` 只在知识上下文任务的 `knowledge_fence` 里随 start/complete/fail 上行（`internal/daemon/knowledge_context.go` 约 28–46），不是 task payload 字段，也不在用量 body 里 | R2 的结论不变：执行身份用服务端签发的 `budget_attempt_id`，不借用 generation |
| V10 | quick-create 的 finalize 锁的是父/来源 issue 而不是 task 自己的 issue；受限模板角色额外取 issue NO KEY UPDATE NOWAIT；`LinkTaskToIssue`（约 `service/task.go:7625`）在完成事务提交后以 autocommit、尽力而为方式执行 | quick-create attempt 归根时用的成员查找针对 context 父任务（V4）；完成后的 link 不是可靠的绑定时机，子任务的成员行只在其创建事务内写 |

锁图未读的部分（`finalizeTaskKnowledgeClaim` 后半、`failTask` 后半、`updateIssueAtomicallyOnce` 后半、工作空间删除全文等）已由独立 check 补读，均不改变结论。

### 2026-11-16：第 1 步独立复核后的修正

证据：[research/e2-independent-check.md](research/e2-independent-check.md)（F1–F14，全部静态阅读，环路结论是推断）。结论：清点对产品代码完整，约 95 处引用无错行号，V1–V10 的决定都不作废；错的是从事实推出的“根锁最后”这条规则。主会话对照源码核实了 F1、F5、F6、F7 的事实后采纳如下。

| # | 复核发现 | 采纳的修订 |
| --- | --- | --- |
| V11 | F1：`task.go` 约 3831–3832 之间不是最后一个取锁位置，其后还有 `CreateTaskToken`（4 个 FK）、全局的 `DeleteExpiredDaemonTokens`、`CreateDaemonToken` 和收据 UPDATE（约 3848） | 预留写在收据 UPDATE 之后、回调返回之前；约 3845–3847 的提前返回改成条件块，两个分支都走到预算写入。回调只拿到 `*db.Queries`，预算写入用 sqlc 查询。`budget_attempt_id` 在事务外生成后传入，两个调用点（`daemon.go` 约 1911、3663）像 token 一样在事务后补进载荷。有成员行却没有事务（`TxStarter == nil`）时拒绝，不静默跳过 |
| V12 | F2：issue 写路径在任何绑定点之后都还有阻塞语句，“BR lock (last)” 是断言不是推导 | issue 写路径不锁根（§5 第 1 条）。成员写入已由 fence 按工作空间串行。连“父任务删除 → 根 tombstoned”也不在删除事务里写根行，改由根任务自己的成员行 tombstone 表达 |
| V13 | F3：读成员行再锁根之间有窗口 | 成员绑定不换根（B10、B11 本就要求），所以不会记到错的根上；不需要给成员行加锁。剩下的窗口是 finalize 读到“未绑定”后该 issue 被启用或改父绑定：绑定方在同一事务里扫描该范围内 queued 之外的在途 task 并登记缺口（B06 的 `in_flight_legacy`），所以不会出现“已是成员、那次执行既无 attempt 也无缺口”。第 5 步加双连接用例 |
| V14 | F4：V7 的例外比 `task_usage` 宽，根锁之后取 user 行、namespace advisory lock、`managed_installation` 都会成环 | 规则改为“根锁之后只写不带 FK 的预算表”（§5 第 2 条）；用量入口的会话、namespace、installation 锁走 `runInTx` 前缀，排在根锁之前 |
| V15 | F5：`TestWorkspaceDeletionManifestCoversPublicSchema`（`handler/workspace_delete_manifest_test.go` 约 397）要求每张 public 表都在拆除清单里分类；`retain_audit` 表必须带 `workspace_id`。该测试依赖数据库，没有库时静默跳过 | 所有预算表都带 `workspace_id NOT NULL`，在清单里登记为 `retain_audit`；第 3 步在克隆库上实际跑这条测试并证明没有跳过 |
| V16 | F6：`issue` 上有两个触发器（`trg_issue_delete_dirty_hourly`、`trg_issue_project_dirty_hourly`），都只 upsert `task_usage_hourly_dirty`，不写 `issue` | 清点里“无触发器”一句作废，结论不变。它们多出一族 dirty 桶行锁，只出现在 issue 删除和换项目里；issue 写路径不锁根（V12），所以与根无关 |
| V17 | F7：一个 quick-create task 可以建任意多个 issue。CLI 给该次运行里每个 `issue create` 都打同一个 origin；origin 索引不唯一；`origin_id` 不要求是调用者自己的 task；`GetIssueByOrigin` 是不带排序的 `LIMIT 1`；失败重试会换 task id 带同一 context | V4 的差异观察按“每个被创建的 issue”记，不按 attempt；schema 不对 attempt 的子任务数设唯一约束。归属依据用服务端从 `mat_` 凭据解析出的执行 task id，不用调用方自报的 origin。B55 覆盖 0、1、多个子任务 |
| V18 | F8、F9：删除 quick-create 的父任务不会取消也不会阻塞那条 task（它的 `issue_id` 为空），finalize 照常进行、载荷里父任务被丢掉；普通改父的查环在事务外、最多 10 层，数据里可以存在环 | attempt 仍按 context 父任务的成员行（含 tombstone）归根，并记 `would_refuse:root_deleted` 或成员 tombstone 观察；子任务落到无根位置时按 V17 记差异。第 4 步新增的每一处后代遍历都自带环检测，不只是启用根那一处 |

未采纳为改动、只记录的：F10（`issue.workspace_id` 的级联是第二个数据库级删除者，没有可删的行）、F12（finalize 与工作空间拆除、namespace 与 user 之间现有的两处逆序，与 F3 无关，根锁放最后之后根行不在这个窗口里）、F13（e2e 与 Go 测试夹具用裸 SQL 建 issue 和改父，B60 的用例要走 service 或自己写成员行）。

仍未证实：FK 取 KEY SHARE 的行为和线上库的实际约束（只读了迁移文件）；所有环路结论都是对取锁位置的推理，只有双连接测试能证明，第 5、6 步各自补。

## 8. 表布局（第 3 步）

六张表，全部无 FK、无级联，全部带 `workspace_id NOT NULL`，在工作空间拆除清单里登记为 `retain_audit`。第 3 步的迁移在最终提交前都可以原地修改（尚未提交、未部署）；提交之后的列变更只能追加迁移。

| 表 | 职责 | 唯一性 |
| --- | --- | --- |
| `issue_budget_root` | 身份、准入状态（`open`/`closed`）、结算状态、观察开关、policy revision 与 epoch、ledger revision、额度、A/E/R/H 净投影与未知计数、启用点（tombstone 不存在根上，见 S3） | `(workspace_id, root_issue_id)` |
| `issue_budget_member` | 一个 issue 终身属于哪个根、继承来源（原父 ID）、绑定来源与版本、tombstone 时间。根任务自己也有一行 | `(workspace_id, issue_id)` |
| `issue_budget_attempt` | 一次领取：task、claim generation、runtime、agent、issue（quick-create 为空）、context 父任务、状态（含 `superseded_unproven`）、预留额、协议版本、判定所依据的 policy revision 与 epoch（`would_refuse` 只记观察行，见 S4、S5）。原有的价格快照两列 2026-11-17 挪到账目条目上（§11.7） | `(task_id, claim_generation)` |
| `issue_budget_entry` | 不可变账目：根内序号、条目身份、类型、attempt / invocation / scope、规范存储形式、digest、收据；服务端估算的用量条目另带冻结的价格快照与 digest（§11.7） | `(budget_root_id, seq)`、`(budget_root_id, entry_id)` |
| `issue_budget_operation` | 原 key、范围、规范意图 digest、操作者、已提交结果 | `(id)`、`(workspace_id, scope_kind, scope_id, operation_key)` |
| `issue_budget_observation` | 只追加的非金额观察：`would_refuse`（含不发生在 attempt 上的 `root_conflict`）、quick-create 归属差异、在途与旧协议缺口 | 无（V17：一次 attempt 可以有多条） |

说明：

- 没有单独的 invocation 表。invocation 与计量 scope 是账目条目上的列，纯计量包已经按它们归并；首包没有只属于 invocation 的可变状态。
- 观察单独成表而不是混进账目：差异和 `would_refuse` 不改变任何金额，放进账目会让重放结果依赖非金额行；它们也会发生在没有 attempt 的 issue 写路径上。
- 根的 `tombstoned` 不是根行上由 issue 写路径维护的列（V12）。读取时由根任务成员行的 tombstone 推出。
- 金额列一律 `BIGINT`（tick），带 `>= 0` 检查；状态列用 `TEXT` 加 `CHECK`，不建枚举类型。
- 迁移从 866 起：先是建表迁移，随后每个索引一条单语句 `CREATE [UNIQUE] INDEX CONCURRENTLY IF NOT EXISTS`，逐条登记到 `concurrentIndexCleanups`。

### 2026-11-16：第 2 步纯计量的取法（独立 check 已过，见下一小节）

证据：[research/step2-metering-implementation.md](research/step2-metering-implementation.md)。包在 worktree 的 `server/internal/budget/`，非测试代码只依赖标准库（最初取 `agent.CostUSDTicksPerUSD`，复核后改为本地常量，见下一小节 M4）。主会话复跑：54 PASS / 0 FAIL / 0 SKIP，race 通过，无浮点。

矩阵未写死、实现选定的读法（都不改变矩阵数字）：

| 行 | 选定读法 |
| --- | --- |
| B17 `rem` | 未知集合为空时 `rem = L − (A+R+H)`，下限 0；只要有任一未知项就不给数值 |
| B23 展示值 | 两个舍入版本：`half-up-v1` 显示 3333，`ceil-v1` 显示 3334；预留恒向上取整为 3334 |
| B24 自报价格版本 | 忽略并打标记，不报错；是否转成拒绝由第 7 步的 handler 决定 |
| B22 负值、B29 混入模式 | 整批拒绝且不留记录，不是隔离 |
| B26 迟到 v1 | 同身份重发返回原收据；首次到达且低于当前值则记为 stale，不倒扣 |

**S2 前置项（首包不解决）**：隔离与缺口产生的未知项目前没有任何条目类型可以消解，带这类未知的预留无法结清，其占额会一直保留。这符合合同 §6 与 D4（可能长时间显示未知），首包只观察、不拦截，所以不影响业务；但进入强制之前必须先有“消解未知”的条目类型和对应权限，否则一条被隔离的事件就能永久占住额度。并入合同 E5。

未纳入第 2 步：提供方原生 USD 十进制串转 tick 的版本化规则、投影里的 token 合计、limit/policy/`would_refuse`、持久化、B24 的 db 半行。前两项在第 6 步定，其余在第 4–5 步。

### 2026-11-17：第 2 步独立复核后的修正

证据：[research/step2-metering-check.md](research/step2-metering-check.md)。复核方按矩阵重算了全部 pure 行，做了 113 个变异（原 54 项测试杀掉 108 个中的 97 个；补测试后 113 个中杀掉 112 个，剩下 1 个是等价变异），并在包内先写测试再修了三处会算错账的问题。主会话复跑：`gofmt` 空、`go vet` 0、70 PASS / 0 FAIL / 0 SKIP、race 通过、`go build ./...` 0。

| # | 复核发现 | 处理 |
| --- | --- | --- |
| M1 | 累计与增量的更正可以经由另一个预留提交，结果把支出记到错的预留上（例：根显得比实际轻 20） | 已修：更正只能落在原条目所属的预留下，否则隔离为 `correction_unresolved`。这推翻了实现方有意选的“从提交更正的那个预留扣回”，依据是合同 §5 表“不得默认应用到另一个 attempt”。主会话确认保留新规则 |
| M2 | 标识符里的非法 UTF-8 让两条不同条目有相同的存储形式和 digest，重放结果与首次不一致 | 已修：条目与价格快照都拒绝非法 UTF-8 |
| M3 | 零值 `Ledger` 直接 `Apply` 会 panic | 已修 |
| M4 | 包为了一个常量引入 `server/pkg/agent`，闭包里带进 `os/exec`、`net` 共 144 个包，也挡住以后 `pkg/agent` 反向引用预算类型 | 主会话改为本地常量 `TicksPerUSD = 10_000_000_000`，另加外部测试包里的 `TestScaleMatchesAdapterCostUnit` 断言与 `agent.CostUSDTicksPerUSD` 相等。包的非测试依赖现在只有标准库。此前“取自 `pkg/agent`”的记录作废 |

留给后续步骤的条件（包未改）：

- **第 5 步，账目装载。** `Ledger` 没有快照/恢复，也没有按 ID 取条目或预留状态的访问器；每次写入要在根锁下按序重放该根全部条目。`Apply` 每次克隆整个账目，未知集合按事件增长（同一 scope 上 2,000 条无成本事件产生 2,000 个未知项，逐条应用耗时 0.40 秒）。第 5 步接入前先定：给纯包加可序列化的状态并存到根上，或接受重放并给出规模上限；未知项是否按 scope 与原因合并也在那时定。
- **第 6 步，入口解码。** 存储形式的解码是宽松的（缺字段当 0、重复键取后者、忽略未知字段）；对外入口必须另写严格解码，缺 `ticks` 不能变成“零成本”。
- **第 6 步，服务端推导字段。** `CorrectionAuthorized` 与三个释放证明目前由调用方填写；必须由服务端根据权限与证据推导。`CorrectionAuthorized` 参与条目身份，授权变化后的重发会是冲突而不是原收据，入口要按这个语义处理或把它移出身份。
- **第 6 步，信封字段。** 纯 `Entry` 不带 attempt ID（只有预留 ID）、producer epoch、payload digest、adapter/normalizer 版本、各时间戳、partial/final、批次 key；这些是 `issue_budget_entry` 上的列，不进 reducer。标识符可含 NUL 且无长度上限，PostgreSQL 存不了 NUL，入口拒绝并在表上加长度约束。
- **归属随到达顺序。** 同一 scope 上序号大的事件先到，序号小的随后会被记为 stale，根的总额正确但在两个预留之间的拆分不对。首包只展示根级数字，不按预留拆分对外承诺。
- **S2 前置项比原先记录的更宽**（仍并入合同 E5）：被隔离的歧义即使随后有正确基线的事件把总额补齐也不会消解；`[最终用量, 结算]` 同批提交在预留带任何用量未知时整批被拒，调用方要分两批；只有估算的用量让预留无法结算；被隔离或 stale 的条目占住自己的 ID 与序号，修正后重发要换新的。

**issue 写路径和启用产生的缺口都不进账目。** 按 §5 第 1 条，改父绑定时登记的在途缺口（V13）只能写观察行，不能追加账目条目（追加要序号，序号要根锁）。启用时的在途缺口同样写观察行：矩阵 B06、B50 要求“启用前未知”与启用后的账目未知分开显示，放进账目反而混在一起。因此读取时的未知是三部分：账目里的未知项、启用前缺口、绑定时在途缺口；后两者只要还有未被对应 attempt 覆盖的，`remaining` 就不给数值。细则见 §9。

未证实项（13 条，见清单末尾）中影响设计的：线上数据库是否仍保留这些 FK（只读了迁移文件）；带非空父任务的插入是否隐式对父行取 `FOR KEY SHARE`（由 FK 与代码注释推断）。两者都不改变“预算行不带 FK、保存 ID 副本”的选择。

### 第 3 步落定（2026-11-17）

实现报告：[research/step3-schema-implementation.md](research/step3-schema-implementation.md)。迁移 866（六张表）、867–879（十三个并发索引）、880（校验索引形状）；前两轮是 867–878 十二个索引加 879 校验，第三轮加入操作表的 `id` 唯一索引后顺延。主会话在新克隆上复跑通过后确认以下选择；独立 check 的结论追加在本节之后。

**2026-11-20 补充（编号）**：第 5 步又加了两个部分索引，各占一条迁移：880 `issue_budget_observation_gap`（`(budget_root_id, reason) WHERE kind = 'coverage_gap'`）、881 `issue_budget_attempt_open`（`(budget_root_id) WHERE state = 'open'`，给 10.3 的查找语句用）；校验迁移因此顺延到 882，校验十五个索引。任务工作树里现在是 866–882 共 17 条。共享分支已经提交到 868，所以第 8 步要把这 17 条整体改到 869–885；本文别处出现的 866–880 这些号都是当时的号，以迁移的名字为准。

- **down 迁移有保护**：任一预算表有行时拒绝回滚，空表可以干净回滚。这比派发说明里的直接 `DROP TABLE` 更严，符合 §6。
- **根**：结算轴是一个可空的 `settled_as_of_revision`；`mode` 只允许 `off` / `observe`，首包存不进 `enforce`；`limit_ticks` 可空表示未设额度；投影另存 `overrun_ticks`；不存账目的序列化状态（第 5 步定）；根行上没有 tombstone 列（第三轮删除，见下）。
- **成员**：没有独立 `id`，`(workspace_id, issue_id)` 即身份；`binding_version` 是绑定规则的版本，不是按根递增的计数（计数会迫使 issue 写路径更新根行）。
- **attempt**：`state` 一列容纳 open、五种占额原因和 settled；`issue_id` 与 `context_parent_issue_id` 至少一个非空。
- **账目**：规范存储形式用 `bytea` 存原字节（`jsonb` 会重排键，digest 就对不上），数据库用 CHECK 按纯计量包的 domain 前缀重算 digest。代价是改 domain 要配一条迁移。`attempt_id` 是 uuid，约定账目里的 `ReservationID` 一律用 attempt 的 UUID 字符串。索引列（`kind`、`attempt_id`、`scope_id`、`invocation_id`）与字节内容的一致性由写入方保证，数据库不校验。
- **账目的信封列**（第二轮，纯 `Entry` 不带这些，不进 reducer）：`source` 与 `schema_version` 必填；`adapter_version`、`normalizer_version`、`producer_epoch`、`payload_digest`、`observed_at`、`reported_at`、`received_at`、`completeness`（partial/final）可空，服务端自己写的条目留空；批次引用是一个可空的 `operation_id`，指向操作行的 `id`（第三轮改，见下）。
- **标识符长度**：所有调用方提供的字符串标识符统一 `octet_length` 在 1 到 200 之间。
- **账目装载**：`ListIssueBudgetEntries` 按 `seq` 读一个根的全部条目。实现方实测：不干预时规划器对小范围会走位图扫描再排序，不一定顺着 `(budget_root_id, seq)` 索引走；结果正确，只是多一次排序。
- **操作**：只存已提交的操作，`budget_root_id` 非空；查不到即“未提交”。
- **观察**：没有任何唯一索引（V17）；`budget_root_id` 非空，观察总是记在某个根上。
- **sqlc**：六张表的生成结构体一律 `json:"-"`，金额不可能经生成结构体以 JSON 数字流出，对外响应必须另建投影。

### 第 3 步独立复核后的修正（2026-11-17）

证据：[research/step3-schema-check.md](research/step3-schema-check.md)。结论：通过；迁移、查询、生成代码没有缺陷，硬性规则全部成立，B01–B60 每一行都能用这些表表达。复核方要求的五种变异全部被测试抓到；它另外发现 54 条 CHECK 里有 17 条去掉后测试仍然通过，已补 23 条拒绝用例（只改测试文件），现在 54 条全部被钉住。

复核核实、不需要改的三点：

- `(task_id, claim_generation)` 唯一键安全。没有产品路径能让同一代成功 finalize 两次：领取和两种重领都会递增 generation，requeue 保持不变而下一次领取递增，每个 handler 对每条领到的行只 finalize 一次，`runInTx` 不重试。这是调用方结构保证的，不是 `FinalizeTaskClaim` 内部强制的；第 5 步在插入 attempt 处写明这条不变量，撞上该唯一键按缺陷处理，不当作拒绝原因。
- 两个 digest 前缀与纯计量包的 domain 字符串一致，digest 就是 `sha256(domain || 序列化字节)`。保留数据库侧的重算。
- 操作表与观察表的 `budget_root_id` 非空不丢任何矩阵行需要的东西。查不到操作行的含义是“没有已提交的结果”，不是“确定未提交”（B33）。

主会话定下的五项表结构变更（第三轮，由实现方在未提交的迁移里原地修改）：

| # | 变更 | 理由 |
| --- | --- | --- |
| S1 | 观察表加可空的 `claim_generation` | §9.5 要拿它和 attempt 的 `(task_id, claim_generation)` 对上，不该从 `detail` 里转型取 |
| S2 | 操作表加 `id` 及其唯一索引；账目表的三列批次引用换成一个 `operation_id` | 三列副本把最长 200 字节的调用方字符串写进不可变账目的每一行，其中两列几乎总是重复 attempt；提交后再改要加列、回填、删列，是这几项里唯一事后昂贵的 |
| S3 | 删掉根行的 `tombstoned_at` | 它是成员行 tombstone 的滞后副本，可能永远为空；原规则还让一次被拒的 reopen 留下写入。改为读取时查根任务的成员行 |
| S4 | 删掉 attempt 的 `would_refuse_reason`，`would_refuse` 只记观察行 | B46 一次 attempt 可以有多个原因，一列放不下；两处存放会让 B50 的计数对不上 |
| S5 | attempt 加可空的 `policy_revision`、`policy_epoch`（同有同无） | 否则没有任何记录能说明一次“若强制将拒绝”是按哪一版 policy 判定的，B40 的 epoch 也没有使用方 |

另外给两条 digest 约束起显式名字，并在迁移里注明 `received_at` 是上报请求到达服务端的时间、`schema_version` 是存储形式的版本。

留给后续步骤的写入规则（不改表）：

- 条目与 scope 的身份在根内唯一，daemon 提供的事件 ID 要按 attempt 加前缀，加前缀后仍须在 200 字节内；含 NUL 的标识符在入口拒绝。
- `ReservationID` 用 attempt UUID 的规范小写字符串。
- `kind`、`attempt_id`、`scope_id`、`invocation_id` 这几列由同一个构造函数从 `Entry` 推出，并单测这个构造函数。
- 未定额的预留存 0，并伴随一条缺口。
- 缺口有两处存放（根锁下的账目缺口条目、无根锁的观察行），第 5 步写明 B50 的计数如何相加而不重复。
- 可以后补的：按原因分的未知计数（根上只有一个总数）、一次 attempt 用到多个模型时的价格快照存放、sweeper 写入的事实没有 actor 值、`ListIssueBudgetEntries` 是无界读取。

**账目装载的实测（第 5 步的输入）。** 复核方在本地容器上测的未强制计划：500 条的根走位图扫描加内存排序（509 个缓冲页），没有问题；20,000 条的根（占测试表 17%，按最坏情况看）走全表并行顺序扫描，排序溢出到磁盘 7.6 MB（9,307 个缓冲页）；按 `seq > 某值` 读尾部则直接顺着唯一键走，10 行只要 4 个缓冲页、无排序。结论：如果每次写入都在根锁下重放整本账目，大根每次写入都要付第二种代价。第 5 步要把“根上存序列化的账目状态”与“只读尾部的查询”一起定，`ListIssueBudgetEntries` 现在不动。那条计划测试只保留为“查询与索引相符”的契约，不代表线上实际选的计划。

**第三轮落定（2026-11-17）。** S1–S5 已由实现方在未提交的迁移里原地改完，主会话在两份新克隆上复跑通过（记录见 `implement.md`）。实现方在第三轮自己定、主会话确认的四处：

- 操作表的 `id` 唯一索引编号 877，紧挨操作表的另一个索引；其后的索引与校验迁移顺延一位。迁移都未提交，改号没有代价。
- attempt 的价格快照大小上限（2 到 65536 字节）是 `price_snapshot` 上的列级检查，digest 相等与“同有同无”各是一条具名约束。这样每条约束只管一件事，拒绝用例能逐条钉住。
- 账目表的 `operation_id` 不建索引，也不加“哪种条目必须带它”的检查。首包没有按操作反查条目的读路径；哪些条目带批次引用由第 6 步的入口决定，现在加检查会把未定的规则写死。
- 复核方补的 23 条拒绝用例保留 22 条，去掉的一条针对已删除的 `would_refuse_reason` 列。约束总数现在是 55 条，全部有拒绝用例。

## 9. 根与成员的规则（第 4 步）

代码放在 `server/internal/service/`，纯计算仍在 `server/internal/budget/`。绑定函数接收调用方已有的事务查询句柄，不自己开事务；管理操作各自开事务。

### 9.1 接入点

| 场景 | 位置 | 做什么 |
| --- | --- | --- |
| 创建 | `IssueService.CreateInTx` 插入 issue 之后（5 条可带父任务的入口都经过这里，V2） | 带父任务且父任务有成员行时，为新 issue 写成员行（`create`，记下父 ID） |
| 改父 | `handler/issue.go` 的 `UpdateIssue` 之后；`collaboration_edit.go` 的 `UpdateCollaborationIssueStructure` 之后 | 父任务确实变了才处理，规则见 9.3 |
| 删除 | 删除事务里 `DeleteIssue` 之前 | 把被删 issue 的成员行标 tombstone；不碰根行 |
| 清空子任务的父 | `DetachDirectChildIssues` | 什么都不做，成员行原样保留（B57） |
| 工作空间整体删除 | — | 什么都不做（V3、V8） |

没有根的任务树：创建和改父各多一到两次按 `(workspace_id, issue_id)` 的索引查找，删除多一条命中零行的 UPDATE，不写任何预算行（B47）。成员写入与业务写入同事务，写失败则该次业务写入失败；这是“不存在已是子任务却没有成员行”的代价，不另加吞错分支。

### 9.2 启用

事务顺序：工作空间行 `FOR KEY SHARE`（所有 fence 持有者的共同前置；工作空间不存在即 `not_found`）→ 取 `iteration:<ws>` fence → 查根的身份槽 → 读根任务 → 检查 → 遍历后代 → 插入根行、成员行 → 在途扫描与观察行 → 操作行。身份槽的检查排在读根任务之前，这样根任务被删除之后槽仍然占着、重放仍然有答案。第 5 步在全部拒绝检查通过之后、第一条预算写入之前加入绑定锁（§10.10）；被拒绝或重放的启用不取这把锁。

- 请求格式：原 key 为 1–200 字节、合法 UTF-8、不含 NUL；额度为空（不设额度）或不小于 0，负数拒绝。不合格一律 `invalid_request`，在任何语句之前判定。启用不带开关参数，新根一律是 `observe`；开关只由 9.4 的操作改变（它只接受 `off` 或 `observe`）。

- 资格：issue 存在于该工作空间，`admission.Formal(admission_status)` 为真（`not_required` 或 `accepted`），并且不是协作 issue（`IsCollaborationIssue` 为假）。这与 `delivery_completion.go` 里 `completionOrdinary` 是同一个定义。它只约束根任务本身：后代不论种类都写成员行（B60 的 C7、C8、S5 要求如此），成员身份不等于执行归属（§10.11）。待分诊的 issue 在被接受之前不能启用。
- 请求必须带原因（去掉首尾空白后 1–2000 字节；合同第 147 行要求显式 reason），原因计入意图摘要并存进操作结果。9.4 的四个操作相同。
- 该 issue 已是某个根的根任务：同一个原 key 且同一意图 → 返回原结果；同一个原 key 但意图不同 → `operation_conflict`；没有用过的 key → `already_enabled`。关闭过、删除过都一样，身份槽终身占用（B01、B03）。
- 该 issue 已是别的根的成员，或它的任一后代已有成员行：`root_conflict`，一行都不写（B02）。
- 后代遍历用自己的递归查询：不设层数上限，带环检测；发现环则拒绝 `parent_cycle`，一行都不写（B59）。环与后代的成员冲突同时存在时报 `parent_cycle`：遍历结果已经不是树，“某个后代已绑定”不再可靠。
- 成员行：根任务一行（`root`），当时的每个后代一行（`activation`，记下它当时的父 ID）。
- 在途执行：范围内 daemon 已经持有的 task（状态 `dispatched`、`waiting_local_directory`、`running`，与 `agent.sql` 现有的在途判定一致；包括 `issue_id` 为空、context 父任务在范围内的 quick-create task），每条写一条观察 `coverage_gap` / `in_flight_legacy`，带 task id 与 claim generation（B06）。

### 9.3 改父

记 M(x) 为 x 的成员行。只在父任务发生变化时处理：

1. x 已有 M(x)：永不改写。新父任务有成员行且属于另一个根时，在 x 的根上记一条 `would_refuse` / `root_conflict`（B11）；其余情况（清空父、移到无根任务下、换项目）不写任何东西（B10）。“当前位置与预算归属不同”在读取时由成员行里保存的原父 ID 与当前父 ID 比较得出。
2. x 没有 M(x)，新父任务 P 有 M(P)：把 x 绑定到 P 的根（`reparent`），并遍历 x 的后代（同样不设上限、带环检测；遇到环时不拒绝业务改父，只绑定已走到的节点并记一条 `coverage_gap` / `parent_cycle`）：没有成员行的后代一并绑定；已属于另一个根的后代保持不动，在 P 的根上各记一条 `root_conflict`。新绑定范围内的在途 task 各记一条 `coverage_gap` / `in_flight_at_bind`（V13）。
3. 两边都没有成员行：什么都不做。

改父从不拒绝业务操作（首包只观察）。根的开关为 `off` 时，规则 1 和规则 2 都只写成员行、不写任何观察；规则与它为什么不漏记见 §10.6，第 5 步实现（第 4 步的代码不读根行，三种观察在任何 mode 下都会写）。

### 9.4 关闭、重开、调额、开关

事务顺序：锁根行 → 检查 → 更新根行 → 写操作行。不取 fence（不写成员行），根锁之后只碰预算表。

- 都要带原 key 与 `expected_policy_revision`；revision 不符即冲突，不替调用者取最新值（B40）。同 key 同意图重放返回原结果，同 key 不同意图冲突。重放查找排在 revision 比较之前：重放带来的必然是旧 revision。
- 不改变任何东西的请求（关已关闭的根、开已打开的根、把额度或开关设成现值）拒绝 `state_conflict`，不写入。否则一次空操作会消耗一个 policy revision，让其他调用者手里的 `expected_policy_revision` 全部失效。
- close 只改准入状态并递增 policy revision，不动任何金额。reopen 是同一个根，policy revision 递增，额度与 A/R/H 原值保留（B03）。根任务的成员行已 tombstone 时 reopen 被拒 `root_deleted`，不留任何写入；这项检查排在“已打开”检查之前。service 层对已 tombstone 的根只拒绝 reopen，close、调额、开关仍可执行（例如把已删除根任务的根关掉开关以停止记录）；谁有权对它做这些操作由第 7 步的权限规则决定。
- 开关 `off` ↔ `observe`：每次从 `off` 回到 `observe` 都发新的 policy epoch，旧值不复用（B40）。
- `expected_policy_revision` 为负是格式错误，拒绝 `invalid_request`，在任何语句之前判定；0 仍按 revision 不符处理（2026-11-18，第 4 步复核 D1：此前负值会从意图摘要的编码里冒出一个普通错误，到 handler 就是 500）。
- 业务状态的 done、重开、再执行不经过这里，不产生任何预算写入（B09）。
- 权限复核、API 与 CLI 是第 7 步；第 4 步的 service 函数接收已解析的操作者，只负责事务内的状态规则。所有查询都带 `workspace_id`，别的工作空间拿根 ID 查不到（B14 的 service 半行）。

### 9.5 缺口观察何时算已覆盖

`in_flight_legacy` 与 `in_flight_at_bind` 观察带 task id 与 claim generation。之后若同一 `(task_id, claim_generation)` 出现了 attempt 行（finalize 在绑定提交之后才读到成员），该观察视为已覆盖，不再计入未知；这个判断在读取时做，观察行本身不改。

## 10. attempt 与影子预留的规则（第 5 步）

2026-11-17 主会话在派发第 5 步之前定下。依据：矩阵 B07、B08、B34–B38、B45–B47、B55，合同 §4.2、§6，纯计量包当前代码。实现方对照源码核实后如有出入，写进报告，不自行改规则。

### 10.1 预留额从哪里来

合同没有规定影子预留的数额，只规定了估算的算法（冻结费率、向上取整）。服务端现有的价格表（`internal/metrics/pricing.go`）是浮点、无版本的展示用表，领取时也不知道这次执行会用多少 token，所以首包不在 finalize 里估算。

- 根的 policy 加一项 `attempt_reserve_ticks`（可空，正整数）：管理者声明“每次执行按多少额度预留”。它和额度一起由 9.4 的调额操作设置，同样递增 policy revision。B45 的“各预留 10”就是这一项。
- 未设置时预留是**未定额**的：账目里的预留条目金额为 0 并带未定额标记，attempt 行的 `reserved_ticks` 为空（0 不能表示未定额）。
- 未定额不能写成“预留 0 加一条缺口条目”：纯计量包里挂在预留上的非生命周期未知会让该预留永远无法结算（`applySettle` 拒绝）。改为纯计量包原生支持：预留条目带未定额标记；该预留未结算期间贡献一个未知项 `reservation_unsized`，结算时随预留一起消除；未定额预留的实际支出不计入 `overrun`。§8“未定额的预留存 0，并伴随一条缺口”一句作废。
- 5a 的实现里设计没有写明的几处，主会话照实现确认（2026-11-17，5a 报告第 3 节）：未定额预留算一次纳入的执行；未定额标记配非零金额是 `ErrInvalidEntry`；金额为 0 的定额预留是另一个事实，同一身份换了标记是 `ErrConflict`；未定额预留上的 hold 照常接受，H 不变，`reservation_unsized` 这一项保留到结算；结算之后晚到的用量照样进 A，不算 overrun，也不把这一项带回来。所以一次结束的未定额执行在未知集合里占两项（未定额、hold 的原因）：`unbounded_unknown` 的 detail 和 `show` 按原因数的是未知项，不是执行数。
- 价格快照不放在 attempt 上：一次执行可以用多个模型，而一份快照只对应一个 provider/model。快照冻结在做估算的那条账目条目上（§11.7），attempt 表的两列删除（10.8）。
- 设置方式：调额操作每次同时给出额度与预留额两个值（各自可空），整体替换；两个值都进入意图摘要，都与现值相同才算空操作（`state_conflict`）。意图摘要因此变化，现在没有已部署的数据，直接改。
- 设了额度却没有声明预留额的根，每次执行都会得到一条 `would_refuse` / `unbounded_unknown`（10.2 第 9 步）：没有预留额就无法在准入时给这次执行定界，这正是将来 enforce 会拒绝的情形。没有设额度的根不产生这条观察。
- 启用时可以同时给出额度与预留额（各自可空），两者都进入启用的意图摘要（2026-11-17 补充）。不然设了额度的新根要再做一次调额才不会每次执行都带 `unbounded_unknown`。

这是首包里唯一新增的管理项。它是人声明的假设，不是费用上界；`show` 要把它标成“声明的预留”，不能当作估算或上限。

### 10.2 finalize 里做什么

位置按 V11：收据 UPDATE 之后、回调返回之前，用回调拿到的 `qtx`。现有代码在 `recordCommentReceipt` 为假时提前返回，预算步骤要放在两条分支都会经过的地方。

1. task 既没有 `issue_id`、也不是 quick-create（聊天、run_only autopilot 等）：直接返回，不取锁、不查询。
2. 取绑定锁的共享 try 锁（§10.10）。取不到就返回专门的错误，领取 requeue。
3. 找成员行：普通 task 用 `issue_id`；quick-create task 用服务端保存的 `context.parent_issue_id`（含已 tombstone 的成员行）。没有成员行就返回，不再取任何锁、不写任何行（B47）。
4. 锁根行。它是这个事务最后一个取锁语句；之后只读写预算表，对 task 等表只做不带锁的普通读取。
5. 根的 `mode` 为 `off`：什么都不写，返回。
6. 装载账目（10.4）。装载失败按 10.5 处理。
7. 同一 task 之前各代仍为 open 的 attempt：各追加一条 hold（`superseded_unproven`），状态改为 `superseded_unproven`，R 转 H，不释放（B35）。账目里没有对应预留的 attempt（10.5 留下的）只改状态、不追加条目。首包没有“未派发证明”的来源，所以 B08 里“有证明才释放”的分支只在纯计量层成立。
8. 追加本次的预留条目（预留 ID 就是 attempt id），插入 attempt 行（带当时的 policy revision 与 epoch）和操作行（B34 的原操作收据：scope 为 `attempt` / attempt id，key 为 `claim:<task_id>:<claim_generation>`，kind 为 `attempt_open`，操作者是 runtime；结果里存 attempt id、根 id、预留额或“未定额”、成立的 `would_refuse` 原因），更新根行的投影与 `ledger_revision`。
9. 判定 `would_refuse`，每个成立的原因各写一条观察（B46 一次可以有多个），原因字符串固定为：
   - `root_closed`：根的准入状态是 closed。
   - `root_deleted`：根任务的成员行已 tombstone（B13）。
   - `unbounded_unknown`：设了额度，且根的未知集合非空（定义见 10.12，含本次的未定额预留）；detail 里按原因列出数量。
   - `limit`：设了额度，且计入本次预留后的 A+R+H 超过额度（B45）。有未知时照样判，两条可以同时成立。
   没有设额度的根只可能出现前两种：没有额度，将来的 enforce 也没有金额条件可拒绝。
10. 返回 attempt id。调用方在事务提交后把 `budget_attempt_id` 补进载荷，没有写 attempt 就不带这个字段。

第 8 步的补充（2026-11-17，随 §11 一起定，属于 5b）：

- attempt 行记下领取方 daemon 声明的预算协议版本（§11.1），由 handler 从 `X-Client-Capabilities` 读出后传进 `FinalizeTaskClaim`；没有声明时该列为空，并在同一步写一条观察 `coverage_gap` / `legacy_protocol`（带 attempt、task、领取代数），永不算已覆盖。它在第 9 步之前写下，所以设了额度的根上，旧 daemon 的每次领取都带一条 `unbounded_unknown`。
- 账目条目的 `source`：finalize 写的是 `finalize`，对账写的是 `lifecycle`，用量入口写的是 `daemon_report`。与一条操作行同事务写下的条目都在 `operation_id` 上引用它（预留条目引用 `attempt_open` 那一行）；对账写的 hold 没有操作行，留空。
- 条目身份的格式各步统一，全部由服务端拼出：预留 `reserve/<attempt id>`；hold `hold/<attempt id>/<原因>`；用量 `usage/<attempt id>/<invocation id>/<model>`；账目内的缺口 `gap/<attempt id>/<invocation id>`。attempt id 与 invocation id 都是 36 字节的 UUID，model 至多 120 字节，最长恰好 200 字节，在条目表的上限之内。
- `would_refuse` / `limit` 的 detail 分开列出 A、R、H、额度与本次预留。首包里预留从不释放（§11.8），读的人要能看出占用里有多少是 hold。
- 条目行与账目状态的对应（2026-11-17，5a 报告第 7 条）：回执不是重放的条目，每条恰好插入一行，只留作记录的条目（`stale`、`quarantined`、`comparison`）也算；回执是重放的条目不插入行。写完之后新状态的 `Len()` 必须等于新的 `ledger_revision`，不相等就让事务失败，按缺陷处理。每条新条目先过 `EncodeEntry`（身份超过 200 字节或存储形式超过 65,536 字节会被拒绝），任何一条被拒绝，整批不写入、不发布。
- 服务端写的 hold，`detail` 一律为空（5a 报告第 6 条）。同一 attempt、同一原因的 hold 身份相同，纯计量包要求内容也完全相同，否则是 `ErrConflict`，不是重放。

第 2、3 步的补充（2026-11-18，第 4 步复核报告 N4）：预算步骤里的工作空间取自 issue 一侧，不取自 agent、runtime 或 task token。

- 依据：成员行的键是 `(workspace_id, issue_id)`，绑定锁按工作空间分，而 `FinalizeTaskClaim` 手里只有 `runtime.WorkspaceID`（并要求 token 的工作空间与它相等）。`lock_task_owner_rows`（迁移 284）分别收集 agent、issue、runtime 三者的工作空间，不要求相等。API 入口都在请求的工作空间里校验 agent（`validateAssigneePair`、评论触发用的 `GetAgentInWorkspace`、quick-create 的 handler），但服务层的入队函数用不带工作空间的 `GetAgent`，复核没能证明 `issue.assignee_id` 的每个写入方都守住同一工作空间。按 runtime 的工作空间去查，两者一旦不同就查不到成员行，这次执行按 B47 静默返回，等于把未知显示成 0。
- 普通 task：先按主键读 `issue.workspace_id`（不带锁的普通读取），用它算绑定锁的键、查成员行。读工作空间可以和 try 锁并进同一条语句。读成员行不行：READ COMMITTED 下一条语句只有一个快照，并进去读到的是取锁之前的状态，10.10 的闭合论证就不成立，它必须是拿到锁之后的另一条语句。这样没有根的树在 finalize 上仍是两条语句。
- quick-create task：用入队时服务端写进 context 的 `workspace_id`（与 `parent_issue_id` 出自同一次校验），不读 issue 行，因为父任务可能已经删除而成员行还在。`workspace_id` 解析不出来的按没有成员行处理，记一条 warn。
- 在途扫描（`ListIssueBudgetInFlightTasks`）：issue 分支的工作空间条件从 `agent.workspace_id` 改到 issue 行上（`JOIN issue`），范围内的 issue 上不管哪个工作空间的 agent 在执行都要扫到。quick-create 分支保留 agent 连接：去掉之后规划器只能按状态扫整个 `idx_agent_task_queue_agent`，代价随队列总量增长。它依赖的前提是“带父任务的 quick-create，agent 与父任务在同一工作空间”：能传父任务的入口有两个：`internal/handler/issue.go` 的 quick-create handler，和评论转子任务的 agent 模式（`internal/handler/source_context.go` 的 `createAgentCommentSubIssue`，把来源 issue 当父任务；第 4 步复核 N19 补上，主会话对照源码核实，2026-11-18）。两者都先过 `validateAssigneePair` 和 `GetAgentInWorkspace`，校验用的工作空间就是写进 context 的那个；Slack 入口不传父任务；手动重试照抄原 task。服务层的 `enqueueQuickCreateTask` 自己不做这项校验（它用不带工作空间的 `GetAgent`），所以以后新增能传父任务的入口要守住同一条校验，第 8 步写进 spec 时两个入口都点名。同一处一并改掉复核报告 N3：quick-create 分支的判定与 `parseQuickCreateContext` 一致（带聊天会话或 autopilot run 的 task 不算），两处共用一个定义。
- 计划（主会话在复核方留下的 6 万行库上实测，范围 3,000 个 issue，没有 8.4 的探测索引，三轮预热后）：issue 分支的通用计划改前 1.2–1.7 ms、改后 1.3–1.5 ms，走的是同一条路径（`idx_agent_task_queue_issue_id_keyset`，读范围内 issue 的全部 task 历史）。代价只发生在启用根时。
- 用例（5b）：成员 issue 上有一条 agent 与 runtime 都属于另一个工作空间的在途 task，启用根时被记为缺口；同样的 task 经 finalize 时写出 attempt。现有的 finalize 规则如果在预算步骤之前就拒绝这种 task，实现方报告拒绝发生在哪一步，改为钉住那次拒绝。

没有事务时的规则（2026-11-18，主会话读 `runInTx` 时发现设计没有写）：`TaskService` 的 `TxStarter` 为空时，`runInTx` 直接用 `s.Queries` 执行回调，不开事务。事务级的咨询锁和根行锁在各自的语句结束时就放掉，预算步骤的原子性也不存在。生产构造只有一处（`handler.New` 里的 `NewTaskService`），传的是连接池；只有测试里的字面量构造会没有 `TxStarter`。规则：

- 没有事务时，预算步骤不取任何锁、不写任何预算行，只按上面的规则确定工作空间并查成员行。
- 没有成员行：照旧返回。现有那些在没有 `TxStarter` 的 service 上调用 `FinalizeTaskClaim` 的用例不受影响。
- 有成员行（不管根的 `mode`）：finalize 失败，返回专门的错误，与同一函数里已有的 `task capability mint requires a transaction` 是同一类。不做没有事务的降级写入：那样写出的 attempt 没有锁保护，账目的连续性无从保证。
- 实现方报告有多少现有用例走这条路径，有没有哪一条因此要改。

任何一步的数据库写失败都会让整个 finalize 事务失败，领取按现有逻辑 requeue；这记为持久化失败，不是 `would_refuse`（B34）。`(task_id, claim_generation)` 唯一键撞上按缺陷处理（§8 复核结论）。

`ledger_revision` 定义为该根最后一条账目条目的 `seq`，没有条目时为 0；新条目的 `seq` 从它加 1 起连续分配。

第 3、4、8、9 步的补充（2026-11-19，5b 第二轮列出的“需要裁定”各项，主会话定；提交之前都可以推翻）：

- 找不到归属的领取照常继续（第 3、4 步）：普通 task 的 issue 行读不到，按没有成员行处理，记一条 warn；成员行指向的根行不存在（没有任何代码路径会写出这种数据），不写 attempt，记一条 error。两种情况都不再取锁、不写任何预算行，执行不因此被挡住。
- `root_deleted` 只看根任务的成员行（第 9 步的定义不变），对这个根上的每次领取都判，不分普通 task 与 quick-create。领取所依据的成员行已 tombstone、而它不是根任务的成员行时（首包里只有 quick-create 的 context 父任务会走到这里：普通 task 随自己的 issue 一起被删）：不写 `would_refuse`，将来的强制怎么对待这种领取是 F3-S2 的事；在同一步写一条观察 `attribution_difference` / `member_deleted`，带该成员 issue、task、领取代数与 attempt，detail 为空。这就是 V18 说的“成员 tombstone 观察”。依据的成员行就是根任务的成员行时只有 `root_deleted` 一条。
- 预留放不进账目时不让领取失败（第 8 步）：首包里预留从不释放，R 只增不减，根声明的预留额又没有上限，一条定额预留可能让 A+R+H 超出 int64，纯计量包对此报 `ErrOverflow`。这是重试治不好的失败：领取会被反复 requeue，一个只观察的根就挡住了执行。规则：`appendEntries` 在发出任何语句之前先让纯计量包接下整批条目；它报 `ErrOverflow` 而本次预留是定额的，就把本次预留改成未定额、整批重新追加一次（被取代 attempt 的 hold 照旧在同一批里）。attempt 行的预留为空，操作结果里是“未定额”，另写一条观察 `coverage_gap` / `reservation_overflow`（detail 带根当时声明的预留额 `attempt_reserve_ticks`），记一条 error 日志。`limit` 按不含本次金额的 A+R+H 判，`unbounded_unknown` 因未定额预留照常成立。管理 API 不为此设上限：任何上限乘以领取次数仍可能溢出，领取路径必须自己兜住。别的账目错误（`ErrConflict`、`ErrUnknownReservation`、`ErrInvalidEntry`、条目数与 revision 对不上）不降级，照旧让 finalize 失败、按缺陷处理。
- `attempt_open` 的意图摘要（第 8 步）：`sha256("budget-operation-intent-v1\n" + JSON)` 的十六进制，JSON 依次是 `kind`（`attempt_open`）、`workspace_id`、`scope_kind`（`attempt`）、`scope_id`（attempt id）、`task_id`、`claim_generation` 六个键，id 是小写带连字符的 UUID，代数与金额同一种写法（JSON 字符串里的十进制数，不是 JSON 数字）。它与管理操作的意图是两种结构（后者总带策略键），同一个域前缀下不会有相同的字节。用例里有一条不经被测代码算出的黄金向量。
- `attempt_open` 的操作结果（第 8 步）：`operation_id`、`operation_kind`、`budget_root_id`、`attempt_id`、`task_id`、`claim_generation`（十进制字符串）、`reservation`（`sized` / `unsized` / `none`，`none` 是 10.5）、`reserved_ticks`（定额时是十进制字符串，否则 `null`）、`ledger_unavailable`、`superseded_attempt_ids`（被取代的 attempt，旧的在前）、`would_refuse`（按判定顺序）。两种观察的 detail 键：`unbounded_unknown` 是 `ledger.<原因>` 与 `observation.<原因>`，值为十进制数量，10.5 时另有 `"ledger":"unavailable"`；`limit` 是 `actual_known_ticks`、`reserved_ticks`、`unknown_hold_ticks`、`limit_ticks`、`this_reservation`、`this_reservation_ticks`。
- 已接受的界限（10.10 的 try 锁与 9.5 的覆盖规则合起来的结果）：领取在第 g 代已提交、finalize 因绑定正在进行而把它退回队列，绑定方的在途扫描看到的正是这一代，记下 `in_flight_at_bind`；task 再次被领取是第 g+1 代，attempt 也在 g+1，那条缺口永不算已覆盖，根从此是未知，尽管第 g 代从未交给 daemon。不改覆盖规则：更高一代的 attempt 证明不了上一代没有执行过（失联回收同样会加代数）。触发条件是启用或改父的那一刻，范围内的 issue 上恰好有领取在 finalize；结果与 D4 一致（可能长期显示 unknown）。用例钉住这个结果。

### 10.3 生命周期事实（B36–B38）

取消、超时、进程退出、失联都发生在持有 task 或 issue 锁的路径里，按 §5 这些路径不能锁根。所以生命周期事实不在这些路径里同步写账，而是由一个对账函数事后补：

- 自己开事务：锁根 → 列出该根仍为 open 的 attempt → 不带锁读取对应 task 的当前状态 → 已到终态的各追加一条 hold（原因由 task 状态与失败原因映射；task 行已不存在时用断言最弱的 `lost_contact`）→ 更新 attempt 状态与根投影。开关为 `off` 的根跳过，回到 `observe` 后的下一轮补上；账目里没有对应预留的 attempt 只改状态。
- hold 原因的映射（2026-11-17）：正常完成与一般的失败是 `process_exit`（进程结束了，但用量是否完整没有证明）；取消是 `cancelled`；超时类的失败是 `timeout`；runtime 离线、心跳过期、被 sweeper 判定失联是 `lost_contact`。实现方把现有的 status 与 failure_reason 取值逐个对上，拿不准的归 `lost_contact`，整张表写进报告由主会话确认。
  - 2026-11-19 补充（主会话在派发 5c 之前清点终态写入点时定；提交前可推翻）：task 行的终态说的是它最后一次领取。task 行的 `claim_generation` 每次领取加一（`ClaimAgentTask` 与两条 `ReclaimStaleDispatched…`），`RequeueAgentTaskAfterClaimFailure` 不加。所以 open attempt 的代数小于 task 行当前代数时，说明之后有过一次已提交的领取、而那次领取没有写下自己的 attempt（当时根是 `off`，10.2 第 5 步在第 7 步之前返回；或那次领取的预算步骤所在事务失败、原行 requeue 之后没有再被领取就结束了）。这条 attempt 是被后来的领取顶替的，不是被终态所说的事件结束的：对账给它记 `superseded_unproven`，与 10.2 第 7 步在下一次领取时会记下的是同一个事实、同一个条目身份，不看终态与失败原因。代数等于 task 行当前代数的 attempt 才按映射表取原因；task 行不存在的仍是 `lost_contact`。触发查询不变（task 已终态或已消失）：代数落后但 task 还没结束的 open attempt 留给下一次领取的第 7 步或 task 结束后的对账，R 与 H 同样计入占用，金额上没有差别。
  - 2026-11-20 补充（主会话确认 5c 起草的原因表时定；提交前可推翻）。先看状态，`failed` 才看 `failure_reason`：
    - `completed` 记 `process_exit`。`cancelled` 不看行上的原因文本（那是调用方的自由文本），一律记 `cancelled`。
    - `failed` 且原因是 `cancelled` 的记 `cancelled`：daemon 把一次以取消告终的运行经 `/fail` 报上来时用的就是这个字面量。
    - daemon 自己观察到并上报的超时记 `timeout`：`agent_error.agent_timeout`、`runtime_cli_timeout`、`idle_watchdog`、`codex_semantic_inactivity`。
    - `runtime_offline`、`runtime_recovery`、`runtime_reconnect_timeout`、`queued_expired` 记 `lost_contact`。后两个只从 `deferred`、`queued` 写出；两个领取入口在 finalize 提交之后都不再把行放回队列，所以按今天的代码它们遇不到当前代的 open attempt，表里仍然给出最弱的原因。
    - `failed` 且原因是 `timeout` 的有两个写入方，写的是同一个字符串：sweeper 的 `FailStaleTasks`（daemon 证明不了自己活着，判定失联），以及 daemon 上报的 agent 超时与准备阶段超时。行上唯一能把两者分开的是 `error` 文本，sweeper 写的是常量 `task timed out`。裁定：`error` 恰好等于这个常量的记 `lost_contact`，其余记 `timeout`。理由：B36 要求超时“U 增加对应原因”，而 daemon 上报的 `timeout` 正是最常见的超时；一律记 `lost_contact`，首包的观察数据就会把普通的 agent 超时全部算成失联。错的方向有一边是安全的：daemon 恰好报了同样的文本时，记成较弱的 `lost_contact`。另一边（sweeper 改了文本而表没有跟上，失联被记成 `timeout`）由两条用例挡住：不连库的一条要求查询文件里 `FailStaleTasks` 那条语句写的就是这个常量，并且再没有别的语句写 `failure_reason = 'timeout'`；连库的一条真的执行 `FailStaleTasks` 再对账，要求得到 `lost_contact`。
    - 其余在 `taskfailure` 清单里的原因，以及 `agent_error`（`FailAgentTask` 的缺省值）、`local_directory_error`、`agent_fallback_message`、`codex_resume_oversized`，记 `process_exit`：是联系得上的一方报上来的一般失败。
    - 表里没有的字符串、NULL 与空串记 `lost_contact`。`FailAgentTask` 原样存下 daemon 给的文本，比服务端新的 daemon 可能带来表里没有的原因。这里的 `lost_contact` 是最弱的断言，不表示真的失联：能写下这种字符串的只有 `/fail`，上报方当时联系得上。
    - 两条守卫：`taskfailure.AllReasons()` 的每个原因都必须在表里有位置；查询文件里凡是把状态写成 `failed` 的语句，它写的 `failure_reason` 字面量都必须在表里。新增原因或新增写入方时用例先变红。daemon 里不在清单上的字面量没有守卫，新增的落到上一条。
    - 原因表原本要由 5c 与主会话各自独立得出再互相对照，这一点没有做到：5c 起草之前读到了主会话的清点（文件放在简报目录里，主会话的疏忽）。上面每一行由主会话对着代码逐行核过，第 5 步的独立复核再对着代码查一遍。
- 触发点：周期性地跑一轮（先用一条查询找出“有 open attempt 且其 task 已终态或已消失”的根，没有就不取任何锁）。用量入口不调它（§11.4；2026-11-17 上午“入口在写入前对自己的根调用一次”的写法作废）。原来写的是“现有的周期性 sweeper 里加一步”，由下面的补充改成独立循环。
  - 2026-11-20 补充（主会话在 5c 动手建这一步之前定；提交前可推翻）：
    - **独立循环，不是 30 秒存活检测那一轮里的一个阶段。** `runPeriodicSweep` 在一轮没跑完时会丢掉后面的 tick，而那一轮管的是离线判定与任务回收；现有代码已经因此把逐行开事务的工作挪了出去（runtime GC、source context 清理各有自己的循环）。对账对每个根取一把根锁、开一个事务，放在同一轮里，一把被占住的根锁就能拖慢存活检测。所以它是一个同类的循环，同样 30 秒一轮，与 GC 循环一起启动。它没有健康观察者：这一步失败不影响 sweeper 的健康状态（首包是观察，不能让观察拖累执行）。阶段指标照常上报。
    - **查找语句排除 `off` 的根**（连上根行，普通读，不取锁）。`off` 的根本来就会在根锁下被跳过；不排除的话，一个长期 `off` 的根只要有一条结束了的 open attempt，每一轮都会白拿一次根锁。根锁下的检查保留，因为查找与取锁之间开关可能变。回到 `observe` 后的下一轮照常被找到。
    - **每轮有界**：查找语句按根 id 排序并带上限（500）；每个根的对账有自己的时限（5 秒），查找语句同样；某个根失败或超时，记一条带根 id 的 warn 后继续下一个。不做游标，也不给整轮设时间预算。
    - **账目装载失败的根**（下一条）每轮都会被重新找到：每轮至多“上限”条 error 日志与同样多次根锁下的完整重放，直到修好。已知局限：排在前面的不可推进的根达到上限时，后面的根要等它们修好才轮得到。期间金额不丢：attempt 保持 open，open attempt 的预留与它将来的 hold 一样计入占用。
    - **两处“已结束”必须是同一个集合**：查找语句的条件，与对账函数判断一条 attempt 是否结束的条件（三个终态，或 task 行不存在）。两者不同的话，某个根每轮都被找到却什么都关不掉。由用例钉住。
    - **建成之后量出来的局限（2026-11-20 晚，5c 第一轮报告“Known limits (stage B)”，主会话裁定首包都接受）**：
      - 每个根的 5 秒管的是整个对账（等根锁、重放、写入）。账本装载不了或重到进不了进程缓存的根每次都整本重放，重放本身超过 5 秒时这个根永远轮不到对账：每轮一条 warn，它结束了的 attempt 一直 open，预留照样计入占用，只是不写 hold、不计入原因计数。实测 4 万条重放最慢 2.65 秒（模拟 amd64 的库，偏慢）。这样的根在认领路径上同样每次整本重放，问题先出在认领上。**留给 F3-S2**：与账本快照一起定，候选是只给等锁设上限（`SET LOCAL lock_timeout`）而让重放跑完，或让时限随账本大小变。
      - 等锁等到超时的根，pgx 会关掉那条连接、连接池再开一条；最坏一轮 500 次。换成 `lock_timeout` 会连带消掉这一条，同上留给 F3-S2。
      - 最坏一轮约 42 分钟（500 个被占住的根各 5 秒），只推迟这个循环自己的下一轮；日志最坏每轮 500 行。
      - 一轮里跳过与失败的根数不进指标（阶段指标只有“找到的根数”和“关掉的 attempt 数”）：找到数长期大于零而关掉数为零，加上逐根的日志，是现有的信号。首包不加新指标。
      - 每个服务进程都跑这个循环；两个进程找到同一个根时由根锁排序，后到的在锁下发现没有可关的，什么都不写。
- 对账时账目装载失败（10.5）：跳过这个根并记 error，不改任何 attempt 的状态，下一轮再试。有没有对应预留要靠装载出来的账目回答，装载不了就不能判断。
- 这是第四类锁根事务，只取根锁，所以不引入新的锁序。§5 第 3 条相应增补。
- 只凭生命周期事实从不结算、不释放（B37）。首包没有结算的生产路径（§11.8；“结算在第 6 步收到带三项证明的最终用量时发生”的写法作废），所以一次结束的执行留下的 hold 不会消除。hold 与 settle 的先后由根锁排序，重复执行结果相同（B38，由 reducer 保证）。

### 10.4 账目装载

重放是定义，缓存只是加速。每个进程一份“已提交前缀”缓存：

- 键是根 id，值是（revision，不可变的账目状态）。缓存里的状态只含已提交的条目，发布时机有两个：刚取到根锁并补完尾部之后（此时 `seq <= 根行 ledger_revision` 的条目必然都已提交，写入方持根锁直到提交）；本事务提交成功确认之后，把含有自己条目的新状态按新 revision 发布。提交失败或结果未知时不发布，回滚因此不会污染缓存。只有新 revision 大于缓存里的 revision 才替换，晚到的发布不会盖掉更新的状态。
- 装载：缓存的 revision 小于根行的就读尾部（`seq > 缓存 revision`，顺着唯一键走，复核实测 10 行 4 个缓冲页）并应用；没有缓存就全量重放一次；缓存的 revision 大于根行的（库被还原）就丢弃缓存重来。
- 校验：尾部 `seq` 必须从缓存 revision 加 1 起连续到根行的 `ledger_revision`；装载出的状态（全量重放，或缓存加尾部）的投影必须与根行各投影列相等，两条路径都比。任何一项不成立都按 10.5 处理。
- 纯计量包加一个不修改接收者的应用方法（返回新状态），缓存里的状态永不被原地修改。5a 第二轮把原地修改的 `Apply` 改成包内私有：包外只能用 `With` 得到新状态，`*Ledger` 的导出方法全部只读，多个 goroutine 共享同一个缓存状态不用加锁。这一条由类型保证，不靠约定（2026-11-17，5a 报告问题 4）。
- 条目的存储形式归纯计量包管（5a 第二轮）：写入方用 `EncodeEntry` 得到字节与摘要，装载用 `DecodeEntry(行上的 schema_version, 字节)`。`DecodeEntry` 只认当前版本，并要求字节恰好是解出的条目重新编码的结果：多出的键、缺键（例如第 2 步没有 `unsized` 的旧形式）、换了顺序、结尾多余数据都拒绝。摘要由条目表的约束保证，装载时不重算。
- 缓存按条目总数设上限并淘汰最久未用的根；上限是包内常量，实现方给出每条目内存的实测值。5a 的实测（2026-11-17）：装载后每条目占 0.8–1.3 KB 堆内存，存储形式每条约 0.7 KB；已有缓存的根上追加一次是一次整体克隆，1 万条目 1–2 ms、分配 7–10 MB，10 万条目 15–31 ms。上限由 5b 按这组数字取，并写明对应的内存上界。
- 上限的落点（2026-11-17，5a 报告第 5 条）：常量放在缓存所在的包（service），不放在纯计量包。主会话定的初值是全进程 5 万条目，按上面的实测约 40–65 MB；5b 认为该取别的数，就在报告里给出依据。单个根的条目数超过上限时不缓存：每次装载全量重放，并记一条 warn（带根 id 与条目数）。内存上界因此是严格的，代价落在这一个根自己的领取上，并随它的条目数线性增长；首包接受，快照留给以后。
- 上限按根的权重计（2026-11-18 更正，5a 补充报告 P.9 的小根实测，主会话在 Go 1.26.9 上复测得到相同数字）：上一条的“约 40–65 MB”只对条目很多的根成立，按条目数计的上限并不给出这个内存上界。根本身有固定开销：只有一条预留的根占 2.8 KB，两条目的根每条目 1.7–2.2 KB，5 条以上才落回 1.0–1.4 KB。5 万条目如果全在单条目的根里要占 138 MB，全在两条目的根里 83–111 MB；而“一个父任务一个根、每个根只有几次执行”正是首包最常见的形状。改法：缓存里每个根的权重是它的条目数加 2，上限 5 万指权重之和；淘汰、以及“单个根超过上限就不缓存”，都按权重判。两张实测表的每一格都不超过（条目数 + 2）× 1,400 字节，所以账目状态本身的上界约 70 MB。这个数字是用 36 字节的身份测的，10.2 的身份更长（用量条目的身份可到 200 字节），也不含缓存自己为每个根付出的开销：5b 用 10.2 的身份格式在 Go 1.26.x 上实测每单位权重的字节数，把上界写进报告，本条届时换成实测值。
- 重放的调用方式（2026-11-18，5a 补充报告 H.10 第 2 条）：`With` 每调用一次就整体克隆一次状态，所以全量重放要把读到的全部条目交给一次 `With` 调用，尾部同理。逐行调用是平方的：5a 实测 1 万条目逐条调用 4.26 s、分配 39 GB，一次调用 3.3 ms。
- 线性的依据（2026-11-18）：hold 的查重改成常数时间之后，首包形状（每次执行一条未定额预留加一条 hold，从不结算）的全量重放是线性的。主会话在 `86fea651c` 的树上独立复测，重放 1 千 / 1 万 / 10 万条目是 0.39 ms / 3.4 ms / 45 ms（两条 Go 发行线相同）；改之前重放是 0.82 ms / 49 ms / 5.5 s。超过缓存上限的根每次装载按条目付费，连同严格解码（`DecodeEntry` 要重新编码比对）：仓库实际使用的 Go 1.26.x 上每条目 5.4–6.2 µs，5 万条目约 0.3 s、10 万条目约 0.6 s；本机默认的 Go 1.27.0 上每条目 3.9–4.6 µs，10 万条目约 0.4 s（2026-11-18 主会话在 1.26.9 与 1.27.0 上各测一遍，5a 补充报告 P.9 的数字相同）。这里原先写的“每条目 2.3–2.5 µs、5 万条目约 0.12 s”是 1.27.0 加宽松解码测的，作废：差别一半来自严格解码，一半来自 1.26 的 JSON 解码比 1.27 慢一倍。规则不变，代价是线性的，缓存上限决定多久付一次。结算、累计用量、更正三处对未知列表的扫描没有动，首包的生产路径到不了；S2 让结算或累计用量进入带估算的根之前，要先把这三处也改掉（5a 报告 5.6 第 1 条的其余部分）。
- 存储形式与 Go 发行线（2026-11-18，5a 补充报告 P.10 第 2 条，主会话接受）：`DecodeEntry` 靠“重新编码必须与原字节相同”判定，所以一行能不能读，取决于编码器还写不写当初那些字节；存储形式就是 `encoding/json` 的输出。5a 实测 Go 1.26.9 与 1.27.0 对全部 1,112,064 个 Unicode 标量值和往返用例的 16,400 条条目写出相同字节，只在非法 UTF-8 上不同，而条目不会带非法 UTF-8。首包的防线是三条钉住字节的用例（固定形式及其摘要、未定额预留与定额孪生的两个摘要、`TestStoredFormSpellsTextTheSameInEveryToolchain` 钉住 39 个有转义写法的字符）：换 Go 版本后哪一条变红，就不能发布，要先给纯计量包写自己的编码器（写出与今天相同的字节），不能改期望值。若放过去，后果是所有含受影响条目的根都按 10.5 变成账目不可用。首包不预先写这个编码器。第 8 步写进 spec。
- 整体赋值（同上第 3 条）：包外代码拿着 `*Ledger` 仍可以写 `*a = *b` 把一个状态整个盖掉，类型挡不住，包内的用例也看不见。规则：缓存只存指针、只换指针，任何地方都不经指针赋值；5b 的复核按这一条查 service 包。
- 两个上限只有 `EncodeEntry` 知道（同上第 4 条）：`With` 和 `DecodeEntry` 不拒绝超长的身份或形式，靠 10.2“每条新条目先过 `EncodeEntry`”和条目表自己的检查挡住，不在 `With` 里重复。
- 多进程各有各的缓存，正确性只靠根锁下的 revision 比较。
- 装载函数本身不做隔离级别断言，也不假设调用方持有根锁：第 7 步的 `show` 在 REPEATABLE READ 的只读事务里调用它（§12.4）。“缓存的 revision 大于根行的就丢弃缓存”只适用于持根锁的调用方。不持锁的读取遇到这种情况，是别的写入方在它的快照之后提交并发布了：它不动缓存，自己按快照全量重放，结果不发布（2026-11-17 补充）。
- “账目不可用”只由全量重放判定（2026-11-18，主会话审 5b 的装载代码时定）：从缓存状态加尾部装载时任何一项校验不成立，先丢弃该根的缓存状态，在同一次装载里从头全量重放一遍，全量重放仍不成立才按 10.5 处理。理由：重放是定义，缓存只是加速；缓存与库对不上（库被还原、运维按 10.5 修过行而别的进程还留着修之前的前缀）不是账目坏了。不这样做的话，这种情况下第一次装载会误报不可用，finalize 随之写下一条永不算已覆盖的 `ledger_unreadable`，一个刚修好的根就此永远是未知。代价只落在失败路径上：多一次全量读取。丢弃缓存状态时记一条 warn（根 id、缓存的 revision、根行的 revision、不成立的是哪一项校验），这是运维能看到“缓存与库对不上”的唯一痕迹；持根锁的调用方因为缓存的 revision 大于根行而丢弃缓存（上面“装载”一条）时记同一条 warn，原因单列，不持锁的读取遇到这种情况不丢也不记；丢弃按持锁与否分别处理：持根锁的丢掉缓存里现有的状态，不持锁的只丢自己用过、而且此刻仍在缓存里的那一个（5b 报告 R1，2026-11-19 接受）。还原数据库之后仍然要重启全部服务端进程：缓存的 revision 不大于根行、内容不同而投影恰好相同的状态，这条规则查不出来；第 8 步写进 spec。
- 单条条目的字节上限（2026-11-19，5b 报告 R4，主会话定）：缓存的权重数的是条目，不是字节，而存储形式允许到 65,536 字节，全是这种条目时 5 万权重约 3 GiB，上面“内存上界因此是严格的”并不成立。改法不动权重：所有服务端写入都经过的 `appendEntries` 拒绝存储形式超过 2,048 字节的条目（常量 `issueBudgetLedgerEntryMaxBytes`，放在缓存上限旁边），在发出任何语句之前拒绝，错误包着 `budget.ErrInvalidEntry`；这是一条被拒的条目，不是账目不可用。装载不设这道上限：别的版本或运维写下的行照读，`budget.MaxStoredEntryBytes` 与条目表的检查不变。取 2,048 的依据（主会话在 Go 1.26.9 与 1.27.0 上实测，数字相同）：首包各种条目取各自最长的值时，定额预留 625 字节、未定额 609、hold 643、带 `outcome_unknown:<id>` 的缺口 703、用量条目 938（无成本）/ 964（实报）/ 1,108（估算，64 字节的价格版本，四个 token 数与金额都取 int64 最大值）。上报方唯一能自己定的文本是模型名（1–120 字节，11.5），`encoding/json` 把其中每个 `<`、`>`、`&` 写成 6 字节，120 个 `<` 的模型名给出 1,708 字节；这是一批合法的 v1 上报能产生的最大条目，上限之上还有 340 字节。一条不连库的用例钉住“最大的合法用量条目不超过上限”：以后改存储形式或放宽 11.5 的长度，把合法条目推过上限时它先变红。内存的两个数字（5b 第二轮实测，报告 6.1；取两条 Go 发行线里较高的 1.26.9，1.27.0 低约 5%–9%）：(a) 服务端实际会写的形状（条目身份按 10.2 的格式，不带自由文本）：每单位权重实测最高 1,838 字节，按 2,000 计，缓存到上限时约 100 MB；(b) 凡是能写到上限的条目都写到 2,048 字节：实测最高 2,960 字节，按 3,200 计，约 160 MB。(b) 不是“每条都在上限”的那两格（排除条目与不指名 attempt 的缺口，2,384 与 2,464 字节），而是挂在一个预留后面、价格版本拉长到上限的估算用量条目：用量条目在文本之外还留一个计量范围，估算值还留在未知列表里。两个数字都是测过的形状里的最高值，不是对纯计量包接受的每一种条目的证明：累计式观察、更正、结算首包不写，没有测，哪一步开始写就由那一步把自己的形状加进基准（`BenchmarkIssueBudgetLedgerCacheMemory`）；3,200 相对 2,960 的 8% 余量用来盖住“自由文本分散在几个字段、各自按分配档位取整”，这一点没有实测。上界只对经过 `appendEntries` 写下的行成立：别的版本或运维写下的行照 65,536 字节装载，仍按权重缓存，每单位约 64,900 字节。

第 3 步复核提出的两个选项里不采用“根上存序列化状态”：账目状态里的去重表和序号集合随条目数增长，快照和重放一样大，每次写入还要重写一个大字段。

### 10.5 账目不可用（B47 的“配置损坏”）

装载时条目解不出（`DecodeEntry` 拒绝，含不认识的版本，10.4）、重放报错、序号不连续或投影对不上，都算该根的账目不可用。此时不追加任何账目条目、不动根投影；仍然插入 attempt 行（预留为空）并签发 `budget_attempt_id`，另写一条观察 `coverage_gap` / `ledger_unreadable`，记 error 日志；执行照常继续。不把它当成 `off`，`show`（第 7 步）要明确显示这个状态。

- 这样的 attempt 在账目里没有预留（纯计量包的 `HasReservation(attempt id)` 为假）。之后要给它追加 hold 的地方（10.2 第 7 步、10.3）都先问这一句，为假就只改 attempt 的状态。它的 `ledger_unreadable` 观察永不算已覆盖，根因此一直是未知（10.12）：那次执行的预留从未入账。
- 操作行照写（B34 的原操作收据不依赖账目），结果里注明账目不可用。
- `would_refuse`：`root_closed`、`root_deleted` 照常判定；`limit` 不判，没有可信的金额；设了额度时写 `unbounded_unknown`，`ledger_unreadable` 本身就是一项未知。
- 该根在缓存里的状态丢弃，下一次重新全量重放。修复数据不在首包里；修好之后新的 finalize 自然回到正常路径，已有的无预留 attempt 保持原样。
- 账目不可用时，同一 task 之前各代仍为 open 的 attempt 原样留着，不改状态（2026-11-19，5b 第二轮，主会话接受）：它在账目里有没有预留此刻问不出来；只改状态而不追加 hold，账目修好之后那笔预留就一直留在 R 里，再没有人把它转成 H。留着的 attempt 由下一次能读到账目的 finalize（10.2 第 7 步）或对账（10.3 的代数规则）关闭。

### 10.6 开关为 `off` 期间（B52）

`off` 的含义是这个根停止产生新的预算记录，成员关系除外：

- finalize 什么都不写（10.2 第 5 步）；对账跳过（10.3）；用量入口对 `off` 的根不写任何行，应答 `not_recorded`（§11.4 第 5 步）。
- issue 写路径照常写成员行（成员关系是终身绑定，不是观察），但不写任何观察：改父的 `root_conflict`、`parent_cycle`、`in_flight_at_bind`，创建时的 `attribution_difference`（10.7）都不写。绑定方为此对根行做一次不带锁的读取（§5 第 1 条）；改父规则 2 的这次读取排在取到绑定锁之后。
- 从 `off` 回到 `observe`：发新的 policy epoch（9.4），并在同一事务里给这个根写一条观察 `coverage_gap` / `observation_off`，detail 带新的 epoch。它的意思是“这个 epoch 开始之前领取的执行可能没有记录”，永远不算已覆盖。
- 写不写 `observation_off` 不看期间是否真的有过执行：task 行会随 issue 删除而消失，事后证明不了“没有”。一个根只有经过开关操作才会是 `off`，启用时一律是 `observe`（9.2）。
- 绑定方读开关必须是取到绑定锁之后新发的一条语句，并且事务隔离级别是 READ COMMITTED（§5 第 5 条）；用事务开始时的快照读，下面的论证不成立。

为什么用一条时间段缺口，而不是回到 `observe` 时扫描在途 task（2026-11-17 上午的写法，作废）：

- 整个发生在 `off` 期间的执行不留任何痕迹，回来之后根的合计里没有它。不标出来就是把未知显示成 0。逐条扫描只能点出回来那一刻还在途的 task，补不了已经结束的。
- 有了时间段缺口，扫描就是多余的。它还带来一个漏洞（第 4 步实现方指出）：改父事务读到 `off`、不记缺口，随后调开关的事务提交（它的扫描看不到尚未提交的成员行），改父再提交，被移入的 issue 上的在途 task 就既无 attempt 也无缺口。
- 不变式因此写成：属于某个根的 issue 上每一次已提交的领取，根上要么有它的 attempt，要么有一条覆盖它的缺口观察——点名这次领取的 `in_flight_legacy` / `in_flight_at_bind`，或一条在它之后写入的 `observation_off`。上面那个交错里，finalize 读到“未绑定”必然发生在改父事务取到绑定锁之前，也就早于它读开关、早于开关提交，所以开关提交时写下的那条 `observation_off` 覆盖它。
- `observation_off` 不点名任何执行，也不按时间戳比较：只要这个根有一条，根就是未知（10.12）。它的 `created_at` 是开关事务的开始时间，而它代表的时间段到该事务提交才结束；以后若要“确认某段缺口”，不能拿 `created_at` 当边界。

代价：一个根只要关过一次观察，它的未知集合（10.12）就永远非空，`remaining` 一直为空；之后如果设了额度，每次执行都会有 `would_refuse` / `unbounded_unknown`。这与 D4（只保证可证范围、保留历史缺口、可能长期显示 unknown）一致。由人确认某段缺口的操作不在首包里。

### 10.7 quick-create（B55）

- attempt 按 10.2 第 3 步归到 context 父任务的根。context 父任务在 finalize 之前被删除时仍归它的根：它就是根任务时带 `would_refuse` / `root_deleted`，不是根任务时带一条 `attribution_difference` / `member_deleted`（V18；10.2 的 2026-11-19 补充。原先这里只写了前一种，与第 9 步对 `root_deleted` 的定义不一致）。
- 该次执行创建 issue 时（创建接点，issue 写路径，只写观察、不锁根）：执行身份用服务端从 `mat_` 凭据解析出的 task id，不用调用方自报的 origin；创建入口把它传进 `CreateInTx`。只处理 quick-create 的 attempt（attempt 行的 `issue_id` 为空）：取该 task 最新一代的 attempt，新 issue 实际绑定的根与 attempt 所属根不同（包括新 issue 没有父任务、不属于任何根）时，在 attempt 的根上按每个被创建的 issue 记一条 `attribution_difference`；根的开关为 `off` 时不记（10.6）。支出仍只进 context 父任务的根。0、1、多个 issue 三种都要有用例。
- 普通 task（有自己的 `issue_id`）创建的 issue 不记差异：它的支出归自己 issue 的根，没有歧义。
- 代价：带 task 凭据的创建多一次按 task id 的 attempt 索引查找（没有根的工作空间里查不到，不写任何行）；人工创建不查。

### 10.8 第 5 步要改的表结构（迁移尚未提交，原地改）

- `issue_budget_root` 加 `attempt_reserve_ticks`（可空，大于 0）。
- `issue_budget_attempt.reserved_ticks` 改为可空，空表示未定额。
- `issue_budget_attempt` 删去 `price_snapshot`、`price_digest` 两列和它们的两条约束；`issue_budget_entry` 加同名两列（2026-11-17，§11.7）：快照 2 到 65536 字节，同有同无，digest 等于域前缀 `budget-price-snapshot-v1\n` 加快照的 SHA-256，只有 `usage` 条目可以带。三条规则各是一条具名约束，拒绝用例逐条钉住；第 3 步里针对 attempt 这两列的用例随之挪到条目表。条目里 `cost.price_digest` 与这一列相等由写入方保证，并有用例。
- `pkg/protocol` 加能力常量 `budget-usage-v1`（§11.1）。
- `issue_budget_entry.schema_version` 写纯计量包的常量 `EntryFormVersion`，现在是 1（2026-11-17，5a 报告问题 2）。5a 给条目加了 `unsized` 键，存储形式变了，但版本不升，摘要的域仍是 `budget-entry-v1`：迁移没有提交过，测试库之外不存在旧形式的行，第一次提交的形式就是版本 1。迁移合入之后再改存储形式，必须同时升版本和摘要的域。
- 其余不动。需要新索引（例如 sweeper 的那条查询）时，实现方先给出查询计划再加，一个索引一条并发迁移并登记。

### 10.9 必须有的双连接用例

- B45：两个并发 finalize 在根锁上排队，两者都记录、都继续，恰一个 `would_refuse` / `limit`。
- V13 窗口（§10.10）：绑定事务未提交期间到来的 finalize 取不到绑定锁、失败并 requeue，重领后有 attempt；先拿到共享锁的 finalize 让绑定方等到它提交，随后被扫描记为缺口。任何交错下，一次已发出的执行要么有 attempt，要么有缺口观察。
- finalize 与人工调额并发不死锁；finalize 与对账函数并发，同一 attempt 只有一条 hold。
- 缓存：追加后回滚的事务不影响下一个写入方的结果；两个各带缓存的写入方交替写入，投影与全量重放一致。
- 关闭观察期间改父与调开关的交错（10.6）：改父读到 `off` 之后开关提交、改父再提交，结果是根的未知集合非空（有 `observation_off`），不是“完整”。
- 绑定锁的排队行为（10.10 依赖它保证绑定方不被饿死）：一个连接持有共享锁、第二个连接在等排他锁时，第三个连接的共享 try 返回失败。这是 PostgreSQL 锁管理器的行为，用例把它钉住，升级数据库版本时能发现变化。

### 10.10 绑定锁（V19，修正 V13）

V13 认为“绑定方在同一事务里扫描在途 task”就能覆盖 finalize 读到“未绑定”的窗口。主会话审第 4 步代码时发现这不成立：扫描看到的是该语句开始时已提交的状态，绑定事务的提交在它之后。一个在扫描之后才提交领取、又在绑定提交之前读完成员行的 finalize，既没有 attempt 也没有缺口。这正是合同禁止的“未知显示成 0”。

改法：每个工作空间一把绑定锁（事务级 advisory lock，键与 `iteration:<ws>` fence 不同）。

- **绑定方**：启用根、改父规则 2（把没有成员行的 issue 绑进某个根）取排他锁（阻塞），位置在 fence 之后。启用根在全部拒绝检查通过之后、插入根行之前取，被拒绝或重放的启用不取；改父只在规则 2 里取，位置在后代遍历之后、读开关（10.6）和写成员行之前。创建子任务不取：新 issue 的 task 在创建事务提交前对 daemon 不可见，提交时成员行已经在了。
- **finalize**：预算步骤先取共享的 try 锁，之后才读成员行（10.2 第 2、3 步）。取不到说明有绑定事务正在进行或正在排队，返回一个专门的错误，领取按现有逻辑 requeue（`RequeueTaskAfterClaimFailure`，随即唤醒 daemon 重领）。不阻塞等待：改父的事务在接点之后还有别的语句，可能反过来等这个 finalize 持有的 task 行，阻塞就会成环。try 锁也不能提前到 finalize 事务的开头：finalize 持着共享锁去等 task 行，同样成环。
- **为什么闭合**：finalize 读成员行时持有共享锁。它要么先于绑定方拿到锁（绑定方等它提交之后才写成员行、扫描，那条 task 此时已是 dispatched，被记为缺口），要么取不到锁（不执行；重领时成员行已提交），要么在绑定方提交之后拿到（读到成员行，写 attempt）。
- **锁序**：fence → 绑定锁 → 根。finalize 拿到共享锁之后只可能等根锁；持根锁的事务都不等 fence 或绑定锁，所以绑定方等 finalize 不成环。PostgreSQL 的锁管理器让新的共享请求排在已在等待的排他请求之后，绑定方不会被持续到来的 finalize 饿死。
- **代价**：绑定事务进行期间（启用根、把任务移入预算树），同一工作空间里恰好在做最终化的领取会失败并立即重领。这是首包对既有执行路径唯一的可见影响，性质与现有的 `LockIssueExecution` NOWAIT 冲突相同。从未启用过根的工作空间里没有人取排他锁，try 锁必然成功。专门的错误在两个调用点按 info 记录，不按 error；单任务领取端点此时按“没有任务”应答（HTTP 200、task 为空），不回 500，批量端点照旧略过这条 task。实现方要核实 daemon 收到空任务之后会被 requeue 的通知唤醒重领，empty-claim 缓存不会因此记成“没有任务”。主会话 2026-11-17 读源码的结果（`86fea651c`），5b 用一条用例钉住即可：`RequeueTaskAfterClaimFailure`（`internal/service/task.go:3872`）在放回队列之后调 `notifyTaskAvailable`，后者先 `EmptyClaim.Bump`（版本加一，此前取样的“空”结论随之失效）再发唤醒；“空”结论只在候选查询一条都没查到时写（`task.go:3692`、`:4081`），这条路径上查到了 task，不会写。daemon 一侧唤醒先进容量 256 的通道（`internal/daemon/daemon.go:2064`），再合并进容量 1 的信号（`:4781`），所以领取请求还在途时到达的唤醒不会丢，请求一返回就再领一轮。
- **误报**：扫描时处于 dispatched、随后 finalize 失败并换代重领的 task，会留下一条不会被覆盖的缺口（那一代其实没有执行）。偏保守，首包接受。
- **启用的顺序与激活时刻**（第 4 步实现方提出）：持有绑定锁期间，锁之前已提交的领取只会从在途走向终态，所以“从这一刻起归属”的时刻不能早于在途扫描。第 4 步的代码先取 `activated_at`、最后扫描，在激活时刻之后、扫描之前结束的 task 既无 attempt 也无缺口（窗口是三条语句）。5b 把启用改成：绑定锁 → 在途扫描 → 取激活时刻（语句时钟，不是事务开始时间）→ 插入根行、成员行、观察行。这样误差偏保守：扫描与取时刻之间结束的 task 多得一条用不着的缺口。根 id 在 Go 里生成、各表没有 FK，顺序可以自由调整。
- 成员行的 `bound_at` 是事务开始时间（改父路径上早于取 fence），不是归属边界；第 7 步不要把它显示成“从此刻起计入”。

调开关不取这把锁，理由见 10.6。

绑定方一侧（`Enable` 与 `BindIssueBudgetOnReparent`）是第 4 步的代码，由第 5 步连同 finalize 一侧一起改，并用双连接用例证明。

**2026-11-20 补充：绑定期间的重领次数实测，首包不加延迟。** 上文“代价”一条只说“失败并立即重领”，没有说会重领多少次。5e 测了（报告 `research/step5e-claim-endpoints.md` 第 6 节），主会话据此裁定。

- 测法与数字，是上界：在进程内直接调领取端点，一被答复就再领，没有网络、没有 daemon；另一条连接持有绑定锁 10 ms、100 ms、1 s。go1.27.0，共享的仿真 PostgreSQL 容器，系统负载约 5。一“圈”是一次以“没有任务”作答的请求。五次的中位数：单 runtime 端点 1、9、91 圈（队列里一条 task 与两条 task 相同）；批量端点、两条 task 是 0、4、43 圈，即 0、8、86 次被放回的领取。一圈约 11 ms，负载约 10 时 13–14 ms。
- 一圈对每条 task 是：一次让 `claim_generation` 加一的领取、一个回滚的 finalize 事务（task token、评论回执、attempt 都不留）、一次 requeue、一次 agent 状态对账、一条 `task:queued` 广播、一次唤醒、两条 info 日志（handler 的 `budget binding in progress; requeueing claim` 与 requeue 自己的 `task requeued after claim finalization failure`）。持锁一秒，一条 task 的代数前进约 90。代数是 bigint，没有任何地方按代数或放回次数设上限，task 不会因此失败。
- 这个上界对内网部署是现实的：daemon 走批量路径（`ClaimTasksWSFirst`），没领满就进 `sleepWithContextOrWakeup`，而 requeue 发出的唤醒已经在等，调用立即返回，循环里没有最小间隔。真实的一圈比进程内多一次往返和一次完整的应答构造，量级相同。
- **裁定：首包不加延迟。**
  1. 持锁多久由绑定方决定。正常情况是一个启用或改父事务里取锁之后的那几条语句，10–100 ms，对应 0–9 圈。到秒级需要绑定事务在取锁之后被别的行锁挡住。
  2. 既有路径同样没有下限：finalize 的任何其他错误都是 requeue 加唤醒（单任务端点回 500，批量端点略过这条 task），`LockIssueExecution` 的 NOWAIT 冲突也是。绑定这一支没有带来新的形状。
  3. 什么都不丢：每一圈都能从日志里数出来，task 最终被领走并在绑定提交之后拿到 attempt。
  4. 加下限并不便宜。批量应答里同时带着别的 task，在 handler 里等一下会把它们一起拖住；给 task 记“不早于”时刻要加列、改领取查询。为只观察的首包都不值得。
- 圈数会让上文“误报”那一条更容易出现：一圈里 task 大部分时间处于 dispatched，绑定方最后那次在途扫描正好落在其中时，会给这一代记一条 `in_flight_at_bind`，而这一代随后被放回，attempt 出现在更晚的一代，这条缺口不会被覆盖。上限是每个绑定方、每条被绑定的 task 一条（扫描只做一次），前提是领取恰好落在绑定窗口里。首包照旧接受。可以收窄的办法（扫描只记“这一代的 finalize 已经提交”的 dispatched task）要先找到可靠的标记，与“让根重新变成已知”一起留给 F3-S2。
- 留给 F3-S2：拒绝成为真实结果之后，被拒的领取不能再靠立即重领收敛，届时一并设计下限，倾向于 task 行上的“不早于”时刻，而不是在请求里等。
- 规格里写明首包没有下限，以及上面的数字。排查“绑定期间日志刷屏”先看绑定事务在等什么。
- 钉住这些行为的用例在 `internal/handler/daemon_claim_budget_test.go`：`TestClaimBudgetSingleRuntimeBindingInProgress`（200、task 为空、代数加一、放回队列）；`TestClaimBudgetBindingInProgressInvalidatesEmptyVerdict`（放回之后缓存的“没有任务”失效。handler 的测试集没有接 Redis，这条用内存里的替身装上真实的 `EmptyClaimCache`，是默认测试集里唯一覆盖这一点的用例，保留；缓存本身的行为仍以 service 里需要 `REDIS_TEST_URL` 的用例为准）；`TestClaimBudgetBatchBindingInProgress`（批量略过；持锁期间成员 issue 与非成员 issue 的 task 都被拒，因为 try 锁先于读成员行）；`TestClaimBudgetFinalizeFailure`（其他错误仍是 500）；`TestClaimBudgetBindingInProgressLogsAtInfo`。上文引用的 `RequeueTaskAfterClaimFailure` 在 5d 交付的树里位于 `task.go:3891`。

### 10.11 归属按机制（B48）

- 写不写 attempt 只看两件事：这次领取是否经过 `FinalizeTaskClaim`；它的 task 是否有 `issue_id`，或者是带 context 父任务的 quick-create。满足之后，只要对应的 issue 有成员行就写，不看 issue 的种类。协作 issue、待分诊的 issue 只要在成员范围内，经这条路径的执行一样记。9.2 的资格只约束根任务本身。
- 不满足的执行不产生 attempt，也不产生缺口观察。它们是机制级排除，读取侧（第 7 步）按名称列为 `not_applicable`，不进覆盖率分母，也不显示成 0 成本。按矩阵 B48，首包已知三类：没有 issue 的 task（run_only autopilot、聊天）；协作的 run，host 模式与 contained 模式都是（走 `ClaimCollaborationRuntimeRun`，不经过 `FinalizeTaskClaim`；daemon 一侧没有 task、不经 `runTask`，用量随 run 的 outcome 记录上报，调研 4.3）；插件的外部调用（没有 task）。
- 5b 的实现方要清点服务端所有把一次执行交给 daemon 或 runtime 的入口，逐个标明属于哪一类，写进报告。发现名单之外的入口先报告，不自行决定归属。

**2026-11-20 补充：入口清点的结果与名单之外各项的裁定。** 清点由 5e 完成（报告 `research/step5e-claim-endpoints.md` 第 4 节，含文件与行号），只读代码，没有改动。

- 经过 `FinalizeTaskClaim` 的入口有三个，调用点两处（`internal/handler/daemon.go:1921`、`:3689`）：单 runtime 领取 `POST /api/daemon/runtimes/{runtimeId}/tasks/claim`；批量领取 `POST /api/daemon/tasks/claim` 及其别名 `/api/daemon/claim`；WebSocket RPC `tasks.claim`，它走批量的那一处。一条 task 属于哪一类只在 `claimSubject` 一处判定。
- 应答丢失之后的重领由同样的服务调用再发一次，代数加一，同一条 task 因此第二次经过预算步骤。这由 10.2 的取代规则处理，5d 用三条用例钉住：`TestIssueBudgetFinalizeSupersedesTheEarlierClaimOfATask`、`…SupersedesAnEarlierClaimThatReservedNoAmount`、`…KeepsTheHoldWhenTheReserveOfAReclaimDoesNotFit`。
- 已知的三类机制级排除核对无误：没有 issue 的 task；协作 run 的领取与随后的 permit（host 与 contained 共用一条路由，模式是请求里的字段）；插件的外部调用（四个调用方：人、agent 在 task 中的工具调用、调度器、事件分发）。
- 名单之外 5e 报告了五项，主会话裁定如下。
  - 心跳应答里带给 daemon 的工作（CLI 更新、模型列表、本地技能列表与导入），以及转发给云端 runtime 服务的节点命令（`POST /api/cloud-runtime/nodes/exec`）：不是执行。没有 task，没有 agent 运行，没有用量回报。不属于预算的对象，也不进 `not_applicable`。
  - 服务端自己发起的模型调用（创建人推荐、描述辅助、聊天标题、协作知识自动化的模型调用，后者在 `internal/service/collaboration_knowledge_automation_model.go`）：是真实的模型支出，但没有 task、daemon、runtime，也没有一个确定的根可归（发生在 issue 创建之前、聊天会话上，或由后台 worker 发起）。首包的对象是根的任务树上的 agent 执行，这些调用不在其中。**不加进 `not_applicable`**：那张表列的是 agent 执行绕过 `FinalizeTaskClaim` 的机制，v1 应答已冻结（§12.9），加第四个名字要改五个 fixture，7b 的 CLI 与 Core 解析已按它们写完。规格里写明“根的合计不含服务端自己的模型调用”。这些调用该不该受某种预算约束是产品问题，收尾时向用户提出，首包不定。
  - `GET /api/daemon/runtimes/{runtimeId}/tasks/pending`：只读，不领取，不发 token。`TaskService.ClaimTask`：测试之外没有调用方。两者都不处理。规格里加一条规则：新增的领取路径必须经过 `FinalizeTaskClaim`，做不到的先进这张清单再合入。
- 5e 没有逐个走三条 issue task 插入查询（`CreateAgentTask`、`CreateDeferredChannelIssueTask`、`CreateDeferredAgentTask`）的调用方；传空 `issue_id` 的调用方会落进“没有 issue 的 task”。不补这一步：归属按机制判定，不按调用方，哪一类都由 `claimSubject` 在领取时决定。
- 能力标记只读第一行 `X-Client-Capabilities`（`requestHasClientCapability` 用 `r.Header.Get`；WebSocket 一侧在连接时存一个字符串）。daemon 只发一行。把标记放在第二行同名头里的客户端会被当作没有声明协议，记 `legacy_protocol` 缺口：偏保守，不会把未知显示成 0。写进规格，不改。

### 10.12 根的未知集合

“未知”有两个来源，读取时合并，都不回写：

- 账目里的未知项，由纯计量包的投影给出：未结算的未定额预留、各种 hold、账目内的缺口条目。
- 观察表里没有被覆盖的 `coverage_gap`。`in_flight_legacy` 与 `in_flight_at_bind` 在同一 task、同一代出现 attempt 之后算已覆盖（9.5）；其余原因（`observation_off`、`ledger_unreadable`、`parent_cycle`）永不覆盖。规则写成“没有被覆盖的 `coverage_gap` 一律算未知”，以后新增的缺口原因默认也算，不维护名单。`parent_cycle` 并不漏记支出（成员行照样逐个写入），算进来是偏保守：父链成环之后，“这个根的范围”已经不能从结构上说清。

用到它的地方有两处：finalize 第 9 步判 `unbounded_unknown`，第 7 步的 `show` 判 `remaining` 是否为空。两处用同一个查询。没有设额度的根在 finalize 里不做这次查询。finalize 读到的是当时已提交的观察；issue 写路径不持根锁，晚到的观察影响的是下一次执行，对观察性的记录够用。这个查询只应扫到缺口观察：实现方给出查询计划，现有索引会扫过该根的全部观察时，加一条只含 `coverage_gap` 的部分索引（一个索引一条并发迁移）。

B06 的对应关系：矩阵写的 `pre_activation_unknown`（原因 in_flight_legacy）在表里是 kind 为 `coverage_gap`、reason 为 `in_flight_legacy` 的观察。启用时已经在执行的 task 不会再有 attempt，这条观察永不覆盖，根的终身总额因此一直标未知，正是该行的期望。

代价：启用根的时候只要范围内有 agent 正在执行，这个根从第一天起就是未知；之后设了额度，每次执行都多一条 `unbounded_unknown` 观察。首包只观察，这些记录的用处是让人看到 enforce 会在哪里拒绝。让一个根重新变成“已知”需要由人确认缺口的操作，属于 F3-S2。

**2026-11-19 补充：查询计划实测与部分索引。** 上文“实现方给出查询计划”一句，由主会话测了（5b 拆成 5d、5e 之后这件事没有归属）。环境：测试容器（PostgreSQL 17.11，amd64 仿真，`shared_buffers` 128MB，`jit` 开），造数库 3,000 个根、约 50 万条 attempt、约 42 万条观察（其中约 21 万条缺口）。数字是热缓存下的执行时间和读过的缓冲页，仿真容器偏慢，只用来比形状，不是生产延迟。

- 现有索引 `issue_budget_observation_root_idx (budget_root_id)` 确实扫过该根的全部观察。一个只有 3 条缺口、却有 20 万条 `would_refuse` 的根：一次查询读 4,613 页、丢掉 200,001 行，generic plan 约 29 ms。这不是造出来的极端：设了额度又有未覆盖缺口的根，每次领取都多写一条 `unbounded_unknown`（见上“代价”），下一次领取的查询就多扫一行。上文的条件成立。
- 加一条部分索引 `issue_budget_observation_gap_idx`：`(budget_root_id, reason) WHERE kind = 'coverage_gap'`。同一个根变成 11 页、0.02–0.14 ms。键里带 `reason`，是因为查询按原因分组并排序，索引的顺序直接可用；只含 `budget_root_id` 的写法在 20 万条缺口时仍要落盘排序（约 76 ms，对带 `reason` 的 45–48 ms）。
- 加了索引之后，代价与该根**未覆盖缺口的行数**成正比，这一点首包不改：5,003 条缺口 1.2–1.3 ms（129 页），20 万条 45–48 ms（约 4,620 页）。缺口会随领取逐次增长的来源有三个：旧 daemon 的每次领取（`legacy_protocol`）、账目不可读期间的每次领取（`ledger_unreadable`）、预留溢出的每次领取（`reservation_overflow`）。一个根是一棵父任务树，要在同一棵树上这样执行上千次才到毫秒级，首包接受，不做合并、不加计数列。混合版本长期存在而某个根的缺口到了万级时，补救是给根记一个“永不覆盖的缺口数”，与“让根重新变成已知”的操作一起留给 F3-S2。
- 观察很多的根（规划器按它在表里的占比估出上万行）在一条连接的头几次执行里会触发 JIT：仿真容器上光编译就要 60–115 ms，`jit = off` 时同一查询 0.03–0.08 ms。仓库里没有任何地方设置 `jit`，这次也不加；写进规格：这条查询变慢时先看 `EXPLAIN (ANALYZE)` 的 JIT 一行。
- finalize 热路径上另外三条查询已经各走一条唯一索引，不需要新索引：成员查找（`issue_budget_member_issue_key`，4 页）、某 task 较早各代的未结束 attempt（`issue_budget_attempt_claim_key`，4 页）、关闭 attempt（`issue_budget_attempt_id_key`）。
- 迁移：这条索引自己占一条并发迁移，排在就绪检查之前，就绪检查保持最后一条；就绪检查对它要求“恰好是这个谓词”，对其余各条仍要求没有谓词，谓词的比较不依赖某个 PostgreSQL 大版本怎样写括号和类型转换。由 5c 一并做（它本来就可能为“某个根的未结束 attempt”加索引），DDL 与数字由主会话给。

### 10.13 第 5 步的拆分

- **5a 纯计量包**（只动 `server/internal/budget/`，无 DB）：未定额预留（10.1）；不修改接收者的应用方法（10.4）；`Len` 与 `HasReservation` 两个读取方法；克隆加应用在 1 千、1 万、10 万条目下的基准和每条目内存。1 万条目的单次应用超过约 5 ms 时只提出更省的结构，不在这一步做。第二轮（2026-11-17，主会话裁定 5a 报告之后，仍在临时 worktree 里做，拷回之前完成）：重复 hold 的判定改成常数时间（首包从不结算，全量重放原先是平方的）；`Apply` 改为包内私有（10.4）；存储形式的 `EntryFormVersion`、`EncodeEntry`、`DecodeEntry` 与两个长度常量（10.4、10.8）；按预留读取的 `Reservation`（§12.5）。
- **5b finalize 接入**：10.8 的表结构、查询、启用与调额带预留额；账目装载与缓存（10.4、10.5）；finalize 的十个步骤与补充（10.2：协议版本、`legacy_protocol`、条目身份格式、`limit` 的 detail）；绑定锁两侧（10.10）；隔离级别断言（§5 第 5 条）；两个调用点读能力标记、下发 `budget_attempt_id` 并处理专门的错误；10.11 的入口清点。出口：B07、B08、B34、B35、B45、B46、B47、B55 的 finalize 部分，B53 的 finalize 半边（没有声明协议的领取记 `legacy_protocol`），以及 10.9 里的 B45 并发、V13 窗口、finalize 与人工调额并发、缓存、绑定锁排队五组用例。
- **5c 事后与周边**：生命周期对账与 sweeper 接入（10.3，B36–B38）；`off` 期间的规则与 `observation_off`（10.6）；quick-create 的 `attribution_difference`（10.7，B55 的其余部分）；10.9 余下的两组用例（finalize 与对账并发、关闭观察期间的交错）。第 4 步的两条用例随之调整：`TestIssueBudgetBindingNeverTouchesARootRow` 收窄为“从不锁、不更新根行”（现在允许读开关）；`TestIssueBudgetTransactionsFollowTheLockOrder` 补一条 `off` → `observe`。
- B35a（finalize 事务失败后原行 requeue：attempt 与预留随同一事务回滚，不留孤儿，之后的领取是第一个 attempt）此前没有归到任何一步，归 5b；绑定锁取不到的那条路径同样要断言不留任何预算行。
- 顺序：5a 不依赖第 4 步的任何文件，和第 4 步的独立复核并行；为了不让半成品影响复核方的编译，它在单独的临时 worktree 里做，完成后把包拷回。5b 要改第 4 步的文件，等第 4 步复核通过再开始；5c 在 5b 之后。三段做完再做整步的独立复核。

第 4 步复核留给后面各步的事（2026-11-18，报告第 13 节；这些用例和代码是第 4 步交付的，后面的规则会让它们变）：

- **5b**：`transact` 声明级别（§5 的增补），`…TransactionsFollowTheLockOrder` 的五份语句清单各多一行；启用换序（10.10）之后 `Enable` 那份清单重写，为此把 `recordIssueBudgetInFlightTasks` 拆成扫描与写入两步。清单按 §5 和 10.10 写出来，不从运行输出粘贴。调额带预留额之后 `…SetLimitToTheCurrentLimitIsRefused` 与 `…OriginalKeyStandsForTheWholeIntent` 跟着改，并给意图摘要钉一条固定向量（N6：现在没有，摘要的输入变了不会有用例变红）。
- **5b，领取被拒之后**（报告 13.3）：重领不带延迟，绑定方持有或排队等排他锁期间，daemon 会反复领取、被拒。5b 实测排队情形下一条 task 要转几圈，结果交主会话决定要不要加短延迟，实现方不自行加。requeue 的 CAS 没有匹配到行时不发唤醒，所以用例断言 task 行和下一次领取的结果，不断言唤醒。`RequeueTaskAfterClaimFailure` 在 `internal/handler/daemon.go` 有九个调用点，逐个对到两个 finalize 调用点上。
- **5c**：10.7 要求执行身份到达创建接点，而 `BindIssueBudgetOnCreate` 现在只看得到 issue 行和调用方自报的 origin；`IssueCreateParams` 加一个由服务端从凭据解析出的 task id 字段，接点加参数。改父在根为 `off` 时跳过在途扫描（不写观察，扫描没有用处）。压力用例 `…ConcurrentWritersKeepEveryTreeBound` 一边调开关一边断言“0 条观察”，5c 之后会有 `observation_off`，断言改成按原因计数。
- **第 7 步**：见 12.7 的同日补充。
- **任何一步**：静态清点守卫 `…WriterInventoryIsComplete`（复核方新增，主会话读过后保留）不连数据库，钉住 8 条写 issue 的查询、它们的 12 个调用点和各自的接点；新增 issue 写入口、去掉接点、在 Go 里手写改 issue 的 SQL 都会让它变红。改接点签名或增删写入口时显式更新它的清单。
- 在途扫描的专用索引（报告 8.4）首包不加：现有路径在 6 万行队列上各种形态都在 8 ms 以内，30 万行时 16–30 ms（冷启动 52 ms），只发生在启用和绑定型改父里，这时持有 fence。单个工作空间的队列到百万行量级时再加，届时是两条各自独立的并发迁移；其中 quick-create 那条索引要求查询保持 `lower(context->>'parent_issue_id')` 这个表达式和三个条件原样。

## 11. 用量入口与 daemon 回传的规则（第 6 步）

2026-11-17 主会话在第 5 步进行期间定下。依据：矩阵 B16、B24、B31–B33、B35、B36、B42、B49、B53，合同 §4–§7 与 §11（R2、R10），用量来源图 §3–§7，§8 里第 2 步复核留给第 6 步的三条，纯计量包当前代码，866 的条目表与操作表。实现方对照源码核实后如有出入，写进报告，不自行改规则。

### 11.1 协议声明与旧 daemon（B53）

- 能力标记 `budget-usage-v1`，是 `pkg/protocol` 里与现有 `DaemonCapability*` 并列的常量，名字定为 `DaemonCapabilityBudgetUsageV1`（2026-11-18 定名：5b 与 6b 分头实现，各自加的必须是同一行）。daemon 用现有的 `X-Client-Capabilities` 头声明；WS RPC 的领取靠 `daemon_rpc.go` 重放这个头，实现方核实两条路径都能读到。
- 两个领取调用点用 `requestHasClientCapability` 读出，换成协议版本 1 传进 `FinalizeTaskClaim`；attempt 行的 `protocol_version` 存 1，没有声明时为空（10.2 的补充）。这半边属于 5b，免得 5b 的用例到第 6 步返工：5b 里 B45 等用例用声明了协议的领取，另有一条用例覆盖没有声明的领取。
- 没有声明：执行照常，`budget_attempt_id` 照样下发（daemon 用不带 `DisallowUnknownFields` 的解码读领取响应，`internal/daemon/client.go:1214`，旧 daemon 会忽略这个字段），那次执行记 `coverage_gap` / `legacy_protocol`。这条观察永不覆盖，根因此是未知。旧的 `ReportTaskUsage`、`task_usage`、hourly 与现有用量页面不改。新 daemon 连到没有这条路由的旧服务端时，领取响应里没有 `budget_attempt_id`，daemon 不建记录、不调入口。
- daemon 一侧到第 6 步才开始声明，并且只在本地回传日志目录可用时声明（先例：`admin_cancel_ack_v1` 只在回执日志就绪时声明）。日志不可用的 daemon 等同旧 daemon，执行不受影响。

### 11.2 B35 的 `attempt_unbound` 不实现

矩阵 B35 末句是“回调不带该 ID 时记 `coverage_gap:attempt_unbound`”。本设计里没有会产生它的地方：新入口的路径里就带 attempt id，不带就到不了这个入口；旧的 `ReportTaskUsage` 预算从不读（B16）。不带 ID 的回调只可能来自没有预算协议的 daemon，它在 finalize 时已经按执行记了 `legacy_protocol`。矩阵该行加带日期的修订说明。

### 11.3 入口与鉴权（B42 的晚到用量）

- 路由放在 `/api/daemon` 组里（`DaemonAuth` 之后）：`POST /api/daemon/budget/attempts/{attemptId}/usage`；查询是同一路径后加 `/{batchKey}` 的 GET（11.9）。2026-11-18：路径就按这里写的定死，不再由实现方调整，因为 6b 先于 6a 实现，两边只在这份路径和 11.5 的格式上相接。
- attempt id 用 `parseUUIDOrBadRequest`。之后不带锁读 attempt 行，先按 attempt 行上的 `workspace_id` 调 `requireDaemonWorkspaceAccess`，再按 `runtime_id` 调 `requireDaemonRuntimeAccess`。不经过 task 行：issue 删除之后 task 行可能已经不在，而晚到的用量仍然要收。attempt 不存在与调用方进不了它的工作空间，应答完全相同（404，正文也相同），不泄漏 attempt 是否存在；先查工作空间再查 runtime 就是为了让“runtime 已删除”这种不同的应答只有进得了工作空间的调用方才看得到。
- 只有查询确实完成而且没有找到，才答 404。读 attempt 或 runtime 时的数据库错误答 5xx，daemon 重试。现有的 daemon helper 已经是这个规矩（它们的 404 会让 daemon 做清理）；这里同理，把一次抖动答成 404 会让 daemon 丢掉回传记录。
- task 凭据（`mat_`）不能调用这个入口。实现方核实 `DaemonAuth` 对它的处理并写一条反向用例（合同 §7：新的机器写入口不能用普通 task 头冒充 daemon）。
- 根已关闭、根任务已删除、attempt 已被换代或已转 hold，都照收：晚到的用量记入原根，不需要执行权限（合同 §6）。runtime 的访问被撤销之后拒收，那次执行的金额保持未知（它的预留由对账转成 hold）。
- attempt 的 `protocol_version` 为空（旧 daemon 领取的）或与批里声明的版本不同：拒收（`protocol_mismatch`，永久错误）。这一列写入后不再变，所以检查排在请求体解码（11.4 第 1 步）之后、开事务之前，不需要锁。批里的 `protocol_version` 不是 1 属于解码失败（400），到不了这一步。完整顺序：解析 attempt id（400）→ 读 attempt（404）→ 工作空间（404）→ runtime → 解码（400）→ 协议（409）→ 事务。
- B42 的另一半（invoke 被撤后新的预留请求被拒绝）留在第 7 步，读作“被现有 finalize 检查拒绝的领取不写 attempt”（R8）。

### 11.4 一批的事务

入口自己开事务，显式 READ COMMITTED（§5 第 5 条）。不取 fence，不取绑定锁：attempt 已经存在，归属早已定下。它是 §5 第 3 条里的用量入口那一类，只取根锁；事务里不写 `task_usage`（V7），旧的上报仍是另一个请求。

1. 请求体严格解码，做完全部不依赖数据库的校验（11.5）。失败直接 400，不开事务。
2. 开事务，锁根行。
3. 按（attempt，批 key）查操作行。找到且摘要相同：返回当初的结果并标 `replayed`。找到而摘要不同：冲突（409 `operation_conflict`）。这一步排在读开关之前：已经提交过的批，开关后来变成 `off` 也要答原结果（B32）。
4. 该 attempt 已经有另一个 key 的用量批：冲突（409 `batch_conflict`）。协议 v1 里一个 attempt 至多一批（11.5）。查法是按 `(workspace_id, 'attempt', attempt id)` 走操作表唯一键的前缀。这个前缀下还有 finalize 写的 `attempt_open` 那一行（key 是 `claim:…`），只有 kind 为 `usage_batch` 的行才算用量批，所以至多读到两行。
5. 根的 `mode` 为 `off`：什么都不写，操作行也不写，应答 200、`not_recorded` / `observation_off`。daemon 收到后丢弃这条记录；回到 `observe` 时写下的 `observation_off` 已经说明这段时间可能有执行没有记录（10.6）。
6. 装载账目（10.4）。账目不可用（10.5）：不写任何行，应答 503 `ledger_unavailable`，daemon 保留记录、之后重发。修好之前 `show` 把这个根显示为账目不可用，不会显示成 0。
7. 归一化成账目条目（11.6、11.7），整批一次应用。任何一条被纯计量包拒绝，整批不提交，应答 400（B31）。
8. 插入条目行与操作行（scope 为 `attempt` / attempt id，key 为批 key，kind 为 `usage_batch`，操作者是 runtime），更新根行投影与 `ledger_revision`，提交；提交确认之后发布缓存。

摘要：`payload_digest` 是请求体原始字节的 SHA-256，写在这批的每个条目行上；操作行的意图摘要沿用第 4 步的做法，对（kind、工作空间、scope、`payload_digest`）取摘要。“内容相同”因此就是字节相同，daemon 重发的是落盘的原字节（11.10）。

2026-11-19 补充（6a 派发前主会话定）：用量批的意图摘要是 SHA-256（域前缀 `budget-operation-intent-v1\n`，后接一个 JSON 对象），小写十六进制。对象恰有五个键，顺序固定：`kind`（`usage_batch`）、`workspace_id`、`scope_kind`（`attempt`）、`scope_id`（attempt id）、`payload_digest`；两个 id 是小写规范 UUID 文本，没有空白。它用自己的结构体，不借第 4 步管理操作那一个（那个结构体恒带 `expected_policy_revision` 等键、没有 `payload_digest`，两种形式的字节不会相同，共用一个域前缀没有歧义）。五个值都不含需要转义的字符，所以摘要可以在 Go 之外手工算出，6a 用这样算出的定值钉住它。

入口不调对账函数，10.3 原来那一句作废。首包没有结算（11.8），用量与 hold 谁先入账，投影都一样：预留 10、用量 3，先用量后 hold 是 A=3、H=7，先 hold 后用量也是 A=3、H=7。入口也不改 attempt 行。

### 11.5 线上格式（协议 v1）

```json
{
  "protocol_version": 1,
  "batch_key": "0195f3a2-...",
  "producer_epoch": "0195f39e-...",
  "adapter": "claude",
  "adapter_version": "2.1.34",
  "normalizer_version": "1",
  "reported_at": "2026-11-17T08:00:00Z",
  "invocations": [
    {
      "invocation_id": "0195f3a1-...",
      "resumed": false,
      "ended": "returned",
      "observed_at": "2026-11-17T07:59:58Z",
      "usage": [
        {"model": "claude-fable-5", "tokens": {"input": "1200", "output": "340", "cache_read": "0", "cache_write": "0"}, "cost_ticks": "81000000"}
      ]
    }
  ]
}
```

- **invocation** 是 daemon 发起的一次 provider 进程调用（一次 `executeAndDrain`）。`invocation_id` 是 daemon 在启动它之前生成的 UUID。`resumed` 为 false 表示 daemon 能确定这次调用用的是新会话，其余一律为 true（判定见 11.10，2026-11-18 改）。
- `ended`：`returned`（调用返回，daemon 拿到了 adapter 的最终用量）、`interrupted`（因取消、超时或被杀而结束，用量是到那时为止看到的）、`unknown`（daemon 只有启动意图、没有结局，例如进程崩溃后重启；此时 `usage` 必须为空）。
- `usage` 按模型列出这次调用自己的用量。四个 token 数必填；`cost_ticks` 可缺，出现时必须大于 0。现有的 `agent.TokenUsage` 分不清“没报”和“报了 0”，所以 0 一律不发；缺就是没有数字，不是零成本。四个 token 全 0 又没有成本的项是非法的。同一次调用里同一模型只能出现一项。
- 金额与 token 都是规范十进制字符串（`budget.ParseDecimal`），不接受 JSON 数字。线上只有 tick，不接受提供方原生的十进制美元串：换算由 adapter 做，规则的版本就是 `normalizer_version`（§8 留给第 6 步的这一项如此定）。
- 线上没有这些字段：计量模式、序号、基线、重叠、`supersedes`、货币、零成本依据、价格版本、任何释放证明。v1 只有“每次调用自己的增量”这一种形状，其余由服务端推导（11.6）。带了这些字段的请求按未知字段拒绝，自报价格版本也在其中（B24）。
- `batch_key` 是 daemon 在为 attempt 建回传记录时生成的，`^[A-Za-z0-9_-]{16,64}$`，所以不会与 `claim:<task>:<gen>` 那类 key 相撞。一个 attempt 只有一个 key：v1 里一个 attempt 至多一批，执行结束后一次发完。以后要分批上报是新的协议版本。
- `invocations` 可以为空：attempt 领到了但没有启动任何 provider 调用。这样的批只写操作行。
- `producer_epoch` 是组这一批的 daemon 进程的启动标识；`observed_at`、`reported_at` 是 daemon 的时钟。这些只进信封列，不决定新旧，也不拿来和服务端时钟比较。

严格解码（§8 第 2 步复核的要求）：拒绝未知字段、重复键（`encoding/json` 默认取最后一个，要先按 token 流查一遍）、缺字段、类型不符、结尾多余数据；字符串必须是合法 UTF-8 且不含控制字符。长度：`invocation_id` 是小写规范 UUID；`model` 1–120 字节；`adapter` 匹配 `^[a-z][a-z0-9_-]{0,62}$`；`producer_epoch`、`normalizer_version` 1–64 字节的 `[A-Za-z0-9._-]`；`adapter_version` 可缺，1–120 字节。上限：请求体 256 KiB，一批至多 32 次调用，全批用量项至多 256 条。超限是永久错误，daemon 丢弃记录，那次执行靠 hold 保持未知。

2026-11-18 补充（6a 派发前主会话定；都是对上面规则的细化，线上格式不变。daemon 组批时不替服务端做这些校验：模型名、调用数、用量项数不合规的批会被整批拒收，那次执行靠 hold 保持未知）：

- 同一批里 `invocation_id` 不得重复，重复按解码失败拒绝（400）。条目身份由 attempt、调用、模型拼出（11.6）：两次调用用了同一个标识，内容相同的两项会被纯计量包当成同一条目的重放而只记一次，内容不同的是冲突，哪一种都不是上报方的本意。
- `reported_at`、`observed_at` 以 `time.Parse(time.RFC3339, …)` 接受的写法为准（小数秒、时区偏移都收），其余拒绝。daemon 发的是 UTC 整秒。
- “控制字符”指 Unicode 的 Cc 类（`unicode.IsControl`），按解码之后的字符串判，所以转义写出的控制字符同样拒绝。请求体原始字节先整体验 UTF-8：`encoding/json` 会把非法字节悄悄换成 U+FFFD，不先验就查不出来。
- 键名必须逐字节相同：`encoding/json` 匹配键名不分大小写，`Batch_Key` 这样的键按未知字段拒绝。必填字段给 `null` 按类型不符拒绝，数组字段同样。
- 超限的应答：请求体超过 256 KiB 答 413（code `request_too_large`），读到上限即停，不解码；调用数、用量项数超限是内容校验，答 400 `invalid_request`。两者对 daemon 都是永久错误（11.10）。

### 11.6 归一化

每个用量项变成一条 `usage` 条目，每次没有可用用量的调用变成一条 `gap` 条目。条目身份全部由服务端按 10.2 补充里的格式拼出，daemon 不能指定。

- 用量条目：计量模式是增量，序号为 0；meter scope 是 `<attempt id>/<invocation id>`（scope 在账目里是全根共用的，带上 attempt id，daemon 就指不到别的 attempt 的 scope 上）；条目行的 `attempt_id`、`invocation_id`、`scope_id` 三列照填。
- 预留：attempt 在账目里有预留（`HasReservation`）时，条目挂在该预留上；没有（10.5 留下的 attempt）时预留为空，金额进“无预留的实际支出”，条目行的 `attempt_id` 仍然填。
- 重叠由服务端按证据表定，不由 daemon 报：
  - 证据表（`internal/service` 里的一张常量表）以（`adapter`，`normalizer_version`）为键，给出三项：新会话的用量是否只含本次调用；续用会话时是否只含本次调用；成本是提供方的实报还是 CLI 自己的估算。
  - 一个 adapter 只凭证据进表：仓库里 adapter 的代码与夹具，或厂商官方文档（注明出处与日期）。表里每行的注释写明证据。没有列出的组合三项都按最保守处理。
  - 对应项没有证明的用量，重叠记为“未证明”，纯计量包把它隔离成 `scope_ambiguity`：留记录，不贡献任何金额，根为未知。不拿它猜账（合同 §5）。
  - 初始内容由一次专门的调研给出（`research/step6-adapter-usage-semantics.md`），主会话逐行裁定后写进这里。
  - **初始内容（2026-11-18 裁定，依据 `research/step6-adapter-usage-semantics.md`）**：表里只有一行。

    | `adapter` | `normalizer_version` | 新会话只含本次调用 | 续用会话只含本次调用 | 成本 |
    |---|---|---|---|---|
    | `claude` | `1` | 已证明 | 未证明 | 未证明实报（adapter 目前从不给成本） |

    - 新会话的证据是厂商文档（`https://code.claude.com/docs/en/agent-sdk/cost-tracking.md`，2026-11-17 取，主会话 2026-11-18 重取核对，26,965 字节）：L222 “Independent calls, with no `resume` or `continue` option: each result covers only its own call”；L61 说 `modelUsage` 只在续用会话时才带上恢复的历史用量；L60 说顶层 `usage` 只含本轮。adapter 的三条取数路径（`result.modelUsage`、`result.usage`、逐条 assistant 用量求和）范围都在本次调用之内。
    - 续用会话不进表：同一页 L223 与 L61 说 Claude Code 2.1.277 起，续用会话的 `modelUsage` 带着整个会话的累计，adapter 原样取、不减基线；2.1.277 之前才是只含本次。表的键里没有 CLI 版本，daemon 报的 `adapter_version` 是探测值、可空，不当证据用，所以续用一律按未证明处理。
    - 成本：`agent/claude.go` 没有任何给 `CostUSDTicks` 赋值的地方。行里这一项按“没有实报证明”填，将来 adapter 开始带成本时落到估算一侧。
    - 已知的不精确（记录，不挡入表）：没有可用 `result` 事件时（进程崩溃、被杀）adapter 用的是逐条 assistant 用量求和，不按消息 id 去重，厂商说并行工具调用时多条消息共用同一 id 和同一份用量（L144），输出 token 是占位值（L148）。这种调用的 token 数可能偏高或偏低。它只进估算（E），从不进 A，`limit` 判定也不用 E。S2 让任何 claude 用量进 A 之前，adapter 要先记下用量出自哪条路径。
    - 证据没有覆盖到的地方（调研 5.1，记录）：引用的句子出自 Agent SDK 的成本页，点名了 `claude -p`，没有逐字点名 `--output-format stream-json`，adapter 读的字段名与该页一致；页面不分版本，adapter 接受 ≥ 2.0.0，而 changelog 记着两处旧缺陷（2.1.214 修了流上多次累计 `message_delta` 造成的成本与 token 重复计数，2.1.285 修了 `modelUsage` 把一轮记到错误模型名下），没有说修之前影响哪些字段。这一行不设版本下限：这些缺陷让数字偏高或挂错模型，不会把别的调用的用量带进来。
    - 其余 23 个实现都不进表（调研第 1 节的逐行结论，共 24 个实现、26 个 provider 键）。`antigravity` 不返回用量；另外 22 个在“新会话”“续用会话”两项上都没有够格的证据：只有结构、仓库注释、手写用例，或者分不出“只含本次”与“会话累计”的单次抓取（cursor、openclaw、grok 各一份）。成本一项，24 个实现里 13 个从不给成本（含 claude），`reasonix` 的一个来源是 CLI 自己的估算，它的另一个来源和其余 10 个 ACP 实现只是透传，没有证据说明数字是提供方算的还是 CLI 算的。没有一个 adapter 的成本被证明是提供方实报。
    - qwen 离入表最近，首包不收：夹具 `agent/testdata/qwen-code-0.20.0-stream-json.jsonl` 里一次真实运行的 result 用量恰好等于该次两次模型调用之和，这次运行无论是新会话还是续用，都支持“新会话只含本次”。但它只有一次运行、一个版本（恰是 adapter 接受的最低版本），没有厂商说明，夹具也没有记录会话状态。有授权抓取时先补它。
    - codex：仓库里没有真实抓取，厂商文档站拒绝了请求。调研另外读了厂商源码仓库某一次提交（`openai/codex` `8b5223dd`）。裁定：不采信为入表证据。它对不上 adapter 接受的版本范围（≥ 0.100.0），在那次提交里 `turn/completed` 不带用量、adapter 没有处理真正带用量的通知，rollout 来源的累计值还可能被上下文窗口填充重置；`agent.Result` 也不说明用量出自通知还是 rollout。
    - 后果，要向用户说明：首包里 A 恒为 0；只有“claude、新会话、没有用户自定启动参数”的调用贡献估算金额；其余调用的用量都被隔离成 `scope_ambiguity`，根为未知。`would_refuse` / `limit` 实际只由预留和 hold 驱动。要让 codex 和续用会话进表，需要真实 CLI 运行的抓取（会消耗额度，本任务没有授权），或者改 adapter 让它自己给出只含本次的数字。
    - 改动 adapter 推导用量的代码时要升它的 `normalizer_version`，升了之后表里没有对应的行，按最保守处理，直到重新裁定。第 8 步写进 spec，并在 `claudeResultUsage` 处加注释指到这张表。
    - 表可注入，做法同 11.7 的价格源：生产用上面这张常量表；B16、B24、B32 这些要看 A 的用例注入一张带实报行的表。另有一条用例钉住生产表本身：只有这一行、没有任何行声明成本是实报。往生产表里加行或改项，要同时改这条用例和 spec。
- 成本：
  - 带 `cost_ticks`、证据表说是实报：记为实报，货币 USD。
  - 带 `cost_ticks`、没有实报证明：记为估算，不带价格快照（它是 adapter 自己的估算）。进 E，不进 A，并带 `estimate_only`。
  - 不带 `cost_ticks`：服务端估算（11.7）；估不了就是 `cost_absent`。
  - adapter 给了数字的，服务端从不重估。
  - 2026-11-18 补充：重叠与成本是两条互不相干的轴。被隔离成 `scope_ambiguity` 的用量条目照样按上面四条定成本（含服务端估算与快照），只是不贡献任何金额；这样条目里留着“当时按什么价估过”，归一化也只有一条路径。估算算不出来（价格行换不成整数费率，或金额溢出 int64）与查不到价格同样处理：`cost_absent`，不带快照。
- `gap` 条目（原因 `coverage_gap`）：调用返回或被打断但 `usage` 为空，detail 是 `no_usage:<invocation id>`（B49）；`ended` 为 `unknown`，detail 是 `outcome_unknown:<invocation id>`。条目表要求非用量条目的 `invocation_id`、`scope_id` 两列为空，所以调用标识只在 detail 和条目身份里。预留的取法与用量条目相同（attempt 有预留就挂在它上面），条目行的 `attempt_id` 照填。挂在预留上的缺口在纯计量包里会挡住该预留的结算，这正是它的含义：这次执行的范围不完整。
- `completeness`：`returned` 的用量条目是 `final`，`interrupted` 的是 `partial`。被打断的调用不另加缺口：它所在的 attempt 必然以取消、超时或失败收场，对账写下的 hold 已经让根是未知。
- 信封列：`source` 为 `daemon_report`，`schema_version`、`adapter_version`、`normalizer_version`、`producer_epoch`、`payload_digest`、`observed_at`、`reported_at`、`received_at`（请求到达时刻）、`operation_id`（这批的操作行）。
- 条目表没有单独的 adapter 列（2026-11-18）：`adapter_version` 列存 `<adapter>/<adapter_version>`，没报版本时存 `<adapter>/unknown`，与 11.7 快照里的写法相同；最长 63 + 1 + 120 字节，在该列 200 字节的上限之内。批的原始字节不落库，所以操作行的 `result` 里另外按调用记下 `invocation_id`、`resumed`、`ended`，以及这一批套用的证据表结论（命中了哪一行或没有命中、三项各取了什么），事后才解释得了某条用量为什么被隔离。
- `CorrectionAuthorized` 恒为假，三项释放证明恒不出现（11.8）。

首包不在根投影里加 token 合计（§8 留给第 6 步的另一项）：token 留在条目里，`show` 不汇总。

### 11.7 服务端估算与价格快照（B24 的 db 半行）

- 只在用量项没有 `cost_ticks` 时估算。价格来自一个可注入的价格源，默认实现读现有的 `internal/metrics/pricing.go`（`PriceForModelAlias`）；查不到的模型不估，保持 `cost_absent`。
- 那张表是浮点的。换成整数费率在 service 层做，不进 `internal/budget`：每个浮点价取最短往返十进制表示（`strconv.FormatFloat(v, 'f', -1, 64)`），按十进制精确解析成分数，再用 `budget.USDRate` 得到“每百万 token 多少 tick”。除了这一次格式化，全程没有浮点运算。
- 每条估算各自冻结一份快照：provider 与 model 取价格表命中的那一行，`native_model` 是上报的模型串，adapter 版本写成 `<adapter>/<adapter_version>`（没报版本写 `unknown`），normalizer 版本取批里的，舍入 `half-up-v1`，货币 USD。`price_version` 由四个整数费率的规范形式取摘要得出（`metrics:<摘要前 16 位>`），价格表一改它自然就变，不靠人记得升版本号。
- 条目的成本记为估算，带 tick、`price_version` 与快照 digest；快照的原始字节存在该条目行新增的两列上（10.8）。重放只读条目里存下的数字，从不重算：价格表升版之后旧条目的 E 逐位不变，实报的 A 与价格表无关。
- 估算结果是 0（用量极小或费率为 0）仍然是估算，不是已知的零成本。
- 用例里的“价格表升版”通过注入第二个价格源实现，不改全局表。

### 11.8 首包没有更正，也没有结算

- 更正：v1 线上没有 `supersedes`，`CorrectionAuthorized` 恒为假。B28 只在纯计量层成立。一个 attempt 只有一批，批提交之后那次执行的数字不再变。
- 结算：入口从不写 `settle`。三项证明里，“scope 完整”（没有未上报的调用）没有证据来源：adapter 的用量是 CLI 说了多少算多少，子进程、CLI 内部的重试、后台调用是否都在里面，没有任何一方能证明；“不可再执行”同样没有（R10：任何终态都不是释放依据，daemon 自己也证明不了 CLI 没有留下后台进程）。合同把可靠的释放证明列为 S2 的前置条件（D5），首包不假装有。
- 后果一：首包里预留从不释放。每次结束的执行，声明的预留额减去已知实际支出的那部分一直留在 H 里。`would_refuse` / `limit` 按 A+R+H 判，所以会随执行次数累积而提前出现：额度 50、每次预留 10、每次实际只花 1，第 6 次执行起就会记 `limit`。这是 enforce 在没有释放证明时的真实行为，观察记录如实反映它；10.2 的补充要求 detail 里分开列出 A、R、H。
- 后果二：一个根只要有过一次结束的执行，未知集合就非空（那条 hold），`remaining` 为空；设了额度的根此后每次执行都带 `unbounded_unknown`。
- 后果三（2026-11-18，证据表裁定之后）：生产的证据表里没有实报成本的行（11.6），首包里 A 恒为 0，金额只有服务端估算（E），而且只来自 claude 的新会话调用。同一个 issue 上的后续执行通常续用上一次的会话，它们的用量被隔离。`show` 因此会是“预留与 hold 有数、实际支出为 0、估算偏少、根未知”。这如实反映了现状：没有一个 adapter 的数字能被证明是这次调用自己的实报成本。要改变它，得有真实运行的抓取、adapter 自己给出只含本次的数字，或者账单对账，三者都不在首包授权之内。
- B36 后半句（附带三项证明时才结算并释放）在纯计量层成立；数据库层加一条用例，经内部的追加函数写入一条带齐证明的 `settle`，证明存储、重放与投影都支持它。没有任何生产路径调用它。

### 11.9 应答与按原 key 查询（B32、B33）

- 200 `recorded`：`replayed` 加逐条回执（条目身份、类型、处置、原因）。不带金额，不带根 id、根的合计或 `ledger_revision`：daemon 只需要知道记没记上。操作行的 `result` 里存完整信息，供第 7 步的读取侧用。
- 200 `not_recorded`：原因 `observation_off`。
- 400 `invalid_request`：指出第几次调用、第几项。永久错误。
- 404：见 11.3。409：`operation_conflict`、`batch_conflict`、`protocol_mismatch`。永久错误。
- 503 `ledger_unavailable`：可重试。其余 5xx 与网络错误同样重试。
- 查询（GET，鉴权与入口相同，不取任何锁、不写任何行）只有两种答案：操作行已提交，返回与当初相同的结果；否则 `unknown`，并给出再次查询的路径。没有“不存在，可以重新开始”这种答案：操作行只在提交之后才存在，查不到可能是正在提交。矩阵 B33 里的 issued / pending 在服务端没有对应的状态，都落在 `unknown` 里。daemon 的恢复方式是原样重发，不靠查询；查询是给人排查用的。

### 11.10 daemon 一侧

- 回传记录：daemon 状态目录下每个 attempt 一个文件，原子写入并 fsync，做法照 `cancel_receipts.go`。领取响应带 `budget_attempt_id` 且本进程声明了协议，就在 `runner.run` 之前建记录，此时生成批 key，记下本进程的启动标识。
- 每次 `executeAndDrain` 之前追加一条启动意图（`invocation_id`、是否续用会话），返回之后追加结局与该次调用自己的用量。`runTask` 里首次运行和换新会话重试是两次调用；实现方清点 daemon 里所有启动 provider 进程的位置，逐个接上，写进报告。
- 启动点（2026-11-18，调研第 4 节核实）：普通领取路径上 `backend.Execute` 只经 `executeAndDrain` 一处调用，`runTask` 里调它两次：首次运行（`daemon/daemon.go:7949`）和换新会话重试（`:8005`）。两次的结果在 `reconcileFreshRetryResult`（`:8020`）里被合并成一张表，要在合并之前各自取走。issue、聊天、autopilot、quick-create、带知识上下文的 task 都走这一条路径。
- `resumed` 的判定（2026-11-18）：这一次 `executeAndDrain` 收到的 `ExecOptions` 里 `ResumeSessionID` 为空，而且 `ExtraArgs`（daemon 级的 `MULTICA_<PROVIDER>_ARGS`，`defaultArgsForProvider`）与 `CustomArgs`（agent 的 `custom_args`）都为空，才是 false；其余为 true。按传入的原始切片判，不看 adapter 过滤之后还剩什么。环境变量不在判定之内。理由：claude 的屏蔽表不拦 `--resume`、`--continue`（`agent/claude.go:777-790`），厂商 CLI 能选中或恢复会话的参数很多而且在变（`--continue`、`--resume`、`--fork-session`、`--from-pr`、`--cloud` 等），黑名单证明不了“一定是新会话”。带自定参数的 agent 因此得不到新会话的证明，它的用量按未证明处理。
- `adapter` 的取值（2026-11-18）：task 用的是 provider 自己的可执行文件时写 provider 的键（`runTask` 里的 `provider`，例如 `claude`、`omp`）；用的是自定义 runtime profile 的命令时（`usesCustomProfileCommand`，`daemon/daemon.go:6917-6931`，它同时带来 launch prefix）写 `custom-<provider 的键>`。证据表里的证明是针对厂商 CLI 本身的，说同一种协议的别的命令背后跑的是什么没有证据；`custom-` 开头的值永远不进证据表。界线沿用代码里已有的那一条（`agent.Config.BuiltinRuntime`）。运维把内置 provider 的可执行路径指到包装脚本时 daemon 分不出来，记为已知局限。
- `ended` 的取法（2026-11-18，对应调研 3.1）：
  - `executeAndDrain` 交回 backend 自己的结果，状态是 `completed` 或 `failed`：`returned`，用量取这份结果的。
  - backend 自己的结果，状态是别的（`aborted`、`timeout`、`cancelled`，以及被 `executeAndDrain` 改标成 `idle_watchdog` 的）：`interrupted`，用量取这份结果的。
  - `executeAndDrain` 自己合成的结果（`drainCtx` 先到之后的四处：没有知识上下文时的 `idle_watchdog`、`cancelled`、`timeout`，`daemon/daemon.go:8919-8937`，这时 backend 的结果即使随后到了也被丢掉；带知识上下文时等了 10 秒仍没有结果而合成的失败，`:8905`）：`interrupted`，用量为空。首包不改这段代码去捞那份结果。
  - `backend.Execute` 报错（`executeAndDrain` 因此返回 error，或者在错误属于知识检查失败时交回一份失败结果，`:8502-8512`）：`returned`，用量为空。启动意图在调用之前已经写下，所以这也是一次调用；没有逐个 adapter 证明 `Execute` 报错时进程一定没有起来，所以不当作什么都没发生。
  - 用量为空的两种在服务端各得一条 `no_usage` 缺口。`completeness` 只是信封列，纯计量包不读它，首包里没有任何判定依赖 `returned` 与 `interrupted` 的区别。
  - 2026-11-18 更正（第一条 6b 车道对照源码发现，主会话读 `:8486-8940` 核实）：上面第三条原先把 `:8893-8939` 整段都算作合成，第四条原先写的是“或返回知识检查失败的结果”。实际有三处要分开。其一，带知识上下文的 task 在 `drainCtx` 先到之后还会再等 backend 的结果 10 秒（`:8897-8901`），等到了就交回这一份；它是 backend 自己的结果，按前两条处理，用量取它的（调研 3.1 第 3 项与 4.1 的表原本就是这样写的）。其二，`:8905` 那份合成的知识失败属于第三条：进程起来过，结果没有等到，不能报成“返回了而且没有用量”。其三，`settleKnowledge` 会把 backend 的结果改成 `failed`（`:8838-8871`）；状态按交回调用方时的算，所以是 `returned`，用量取那份结果的。结果通道被关闭而没有给出结果的，算合成。分辨“backend 自己的”与“合成的”靠 `executeAndDrain` 里的标记，不靠结果的内容；它的签名、返回值和取消行为不变。
- 不算调用的进程（2026-11-18，调研 4.4）：`runTask` 里的模型目录发现（`listModels`）、版本探测、openclaw 的配置命令也会启动 provider 的可执行文件，但不发用户消息、不产生 `agent.Result`，不建启动意图。codex adapter 在一次 `Execute` 里可能起两个 app-server 进程（内部重试），仍算一次调用。
- 每次调用自己的用量取 `executeAndDrain` 那一次返回的结果，不取 `mergeUsage` 合并之后的。四个 token 全 0 但成本大于 0 的行要发（现有 `task_usage` 组装会丢掉这种行，旧路径不改）。
- `runner.run` 返回之后组批、把批的字节落盘、发送。发送不阻塞、不影响任务的完成或失败上报；失败留给后台重试。旧的 `ReportTaskUsage` 调用原样保留。
- 字段来源（2026-11-18）：`adapter_version` 取这次启动解析到的 CLI 版本（`agent.Config.CLIVersion`），为空就不带这个字段；`normalizer_version` 取 `pkg/agent` 里按 provider 给出的常量（新增一个函数，首包所有 provider 都是 `1`），改动某个 adapter 推导用量的代码时升它的值；`producer_epoch` 是 daemon 进程启动时生成一次的标识。四个 token 全 0 而且没有成本的行不发；其余的行原样发，daemon 不替服务端做校验。
- 收到 200（`recorded` 或 `not_recorded`）或永久错误（400、404、409）就删记录；5xx 与网络错误保留，原字节原 key 重发。2026-11-18 补全其余状态码：任何 2xx 都算送达；413、422 与 400 同样是永久错误；其余一切（401、403、408、425、429、5xx、网络错误、读不完的应答）都保留重发，由 7 天上限兜底。永久错误删记录时记 warn，带状态码与错误代码，不带请求体。任何情况下都不换 key、不新建 attempt 或 invocation（B33）。这个入口的 404 只意味着删掉这一条记录：现有代码里 runtime 路由的 404 会让 daemon 丢弃 runtime 并重新注册、task 路由的 404 会打断正在运行的 agent，实现方核实新的客户端方法不会走进这两种处理。
- 重启：已有批字节的记录直接重发；没有批字节、而且是上一个进程建的记录，没有结局的调用按 `unknown` 组批再发。
- 2026-11-18 补充（重派 6b 之前定下，前四条来自第一条 6b 车道的清点）：
  - 不发批的两处：`handleTask` 在 `runner.run` 之前有两处提前返回，runtime 已不在本机（`daemon/daemon.go:5135-5147`）与本地目录锁中止（`:5184-5198`）。这两处不建记录、不发批。前一处 daemon 连 provider 都不知道，填不出 `adapter`；两处在首包里发不发批金额都一样，那次 attempt 的预留由对账转成 hold，根未知。记为已知局限，S2 需要“什么都没有启动”的证明时再补。`runTask` 里面没有启动 provider 就返回的情形（环境准备失败、知识检查拦下、只重新生成快捷操作）在记录建立之后，照 11.5 发一个 `invocations` 为空的批。
  - `unknown` 调用的 `observed_at` 取它的启动意图时间：线上格式要求这个字段必填，而 daemon 对这次调用只知道这一个时间。`reported_at` 是组批的时间。
  - 错误代码的来源：永久错误的 warn 里的代码取应答 JSON 的 `code` 字段（仓库里 `writeErrorCode` 的形状，6a 的错误应答都用它）；没有 `code` 时取 `error`，而且只在它形如 `^[a-z][a-z0-9_]{0,63}$` 时才记，其余不记。应答正文不进日志。
  - 重定向不跟随：3xx 归入“其余一切”，保留重发。默认的 HTTP 客户端会把 POST 的 301/302/303 改成 GET 再发，中间设备的登录页会因此被读成 200、记录被当作送达而删掉。
  - 记录只发回建它的服务端：记录里记下建立时的服务端地址，与当前地址不同的记录不发送，留到 7 天上限。理由：一个 profile 被改指到另一个服务端之后，上一个服务端的 task 的模型名与 token 数不应该发过去。托管安装下还按记录里的工作空间选凭据，当前没有那个工作空间的有效绑定就留着不发。
  - 恢复的前提是本进程是这个 profile 唯一的 daemon：`Run` 先绑定健康端口（`listenHealth`，端口被占就退出，`daemon/daemon.go:1967`），回传日志的初始化与启动时那一轮恢复都排在它之后。两个 daemon 用不同的健康端口共用一个 profile 目录属于配置错误：后启动的会把先启动的在途记录按 `unknown` 报掉，先启动的那一批随后得到 `batch_conflict`。`cancel_receipts.go` 有同样的前提。记为已知局限。
  - 目录：回传日志放在 daemon 的 profile 状态目录（`cli.ProfileDir`）下自己的子目录里。`cancel_receipts.go` 的目录只在托管安装下存在，照搬的是它的写法（临时文件、fsync、改名、目录 fsync），不是位置。
  - 2026-11-19 补充（主会话审 6b 第一轮时定）：`adapter_version` 是信封里的备注，不参与任何判定，来源是可执行文件 `--version` 的输出，自定义 profile 的命令输出什么都有可能。daemon 记下它之前去掉首尾空白；剩下的不是 1 到 120 字节的合法 UTF-8、或者含控制字符（`unicode.IsControl`），就当作没有版本，不带这个字段（服务端记 `<adapter>/unknown`）。上面“daemon 不替服务端做校验”说的是用量行；一条备注不该让整批用量被拒收。
  - 2026-11-19 补充（主会话审 6b 第一轮时定，对应 6b 报告第 9 节；提交前可推翻）：记录文件不能在“启动过几次”上落后于进程。下面“日志写不进去不能挡执行”一条只管还活着的进程（它从内存里报），没有说下一个进程能从文件里读出什么。每次改动都整份重写，所以文件只在一次失败的写入到下一次成功的写入之间落后；进程死在这个窗口里，下一个进程手里只有文件。
    - 少的是结局或批字节（`endInvocation`、`seal` 的写入失败）：启动意图在文件里，下一个进程按 `unknown` 报或者重新组批，方向是保守的，不用处理。
    - 少的是启动意图（`beginInvocation` 的写入失败）：尽力删掉这个 attempt 的记录文件，删不掉记 warn；执行照常，内存里的记录与在途表都不动，此后任何一次整份重写成功，文件就又是完整的。删掉之后进程若死在窗口里，下一个进程什么都不报，那次 attempt 靠 hold 保持未知。
    - 恢复（启动时那一轮与后台重发）不上报“没有批字节、也没有任何启动意图”的记录，不论是哪个进程建的，删掉并记 warn。这样的文件分不清三种情况：还没启动 provider 就死了；首次启动意图没写进去而文件又没删掉；建记录失败、文件没删掉、那次执行按没有协议跑了（6b 报告偏离 7）。后两种里 provider 起来过，报成空批就是说了假话（11.5：空批的意思是“领取了，没有启动任何 provider 调用”）。“什么都没有启动”的证明因此只来自建记录的进程自己封的空批。首包不从空批推出任何结论（11.8），所以金额上没有差别。
    - 没有批字节、是本进程建的、而且不在本进程在途表里的记录，同样删掉并记 warn，不上报。在途表里还有的记录，恢复不碰。
    - 已知局限：目录从第 N 次启动（N > 1）起变得不可写，删除也失败，进程又在送达之前死掉，下一个进程会报出前 N−1 次调用而没有第 N 次。三件事同时发生才会出现；首包不结算（11.8），那次 attempt 的 hold 仍在，金额不受影响。S2 要从一批里推出“调用完整”之前必须先补上这一条（例如只认建记录的进程自己封的批）。
    - 目录里不是记录的名字（6b 报告 10.2 第 4 项）：原子写入留下的临时文件（`.installation-*.tmp`）与开目录时的探针（`.probe-*.tmp`）在进程死于创建与改名之间、或删除失败时会留下。恢复只认 `<规范 UUID>.json`，别的名字一律跳过、不读、不报；7 天上限那一轮顺手删掉以 `.` 开头、以 `.tmp` 结尾、修改时间早于上限的普通文件，别的名字不动。
- 保留上限：记录最多留 7 天，过期丢弃并记 warn。那次执行在服务端靠 hold 保持未知。
- 日志写不进去不能挡执行：建记录失败就按没有预算协议执行这一次（不发批）；中途追加失败时调用仍在内存里，随批上报。
- 默认测试不解析、不执行本机安装的 agent CLI，用测试自建的假可执行文件。

### 11.11 必须有的用例

- B16：同一消费经旧路径和新入口各报一次，随后改写、删除 `task_usage` 行与 hourly 桶，根的 A 不变。
- B24：注入的价格源换版之后重放旧条目，E 逐位相同；带 `price_version` 的请求被拒；实报的 A 不随价格变。
- B31：三次调用的批里第二个非法，整批不提交、应答不是成功；同一用例里对照旧入口“逐行 warn 后 200”的行为没有变。
- B32：提交后以原 key 重发相同字节得到原结果，A 不变；原 key 不同内容是冲突，A 不变。
- B33：另一个连接上未提交的批，查询答 `unknown`；提交之后答结果；查询不写任何行。daemon 重发的是落盘的原字节。
- B42：runtime 访问有效时，已关闭的根、已 tombstone 的根、已换代的 attempt 都收晚到用量；访问被撤后 404，根保持未知。
- B49：没有用量的调用、结局不明的调用各得一条缺口，根为未知。
- B53：没有声明协议的领取，执行照常、有 `legacy_protocol`、旧上报路径行为不变；对它的 attempt 上报被拒。
- 证据表：没有证明的续用会话用量被隔离、不贡献金额；有证明的计入。生产表（2026-11-18）：`claude` 新会话的用量得到估算，`claude` 续用会话、没有列出的 adapter、`custom-claude`、`normalizer_version` 不是 `1` 的 `claude` 都被隔离；生产表只有一行且没有实报行；`adapter_version` 列与操作行 `result` 的内容符合 11.6。
- 双连接：同一批并发重发，恰一次写入，后到的应答是标了 `replayed` 的原结果；批与 finalize 落在同一根上不死锁，投影与重放一致；批与调开关并发，结果要么已记录、要么 `not_recorded`，不出现半批。
- daemon：崩溃后重启按原 key 重发；意图已写、结局未写的调用报 `unknown`；日志目录不可用时不声明协议。
- daemon（2026-11-18）：`resumed` 的四种输入（有续用标识、有 daemon 级参数、有 agent 自定参数、三者皆无）；换新会话重试产生两次调用，各带自己的用量，不是合并后的；自定义 runtime profile 的 `adapter` 是 `custom-` 开头；`ended` 的四种来源；只有成本的行会发；模型目录发现与版本探测不产生调用。

### 11.12 第 6 步的拆分

- **6a 服务端**：证据表；入口、查询、严格解码、归一化、估算与价格源；11.11 里除 daemon 之外的用例。
- **6b daemon**：能力声明、回传记录、组批与发送、重启恢复；11.11 的 daemon 用例。
- 线上格式由本节定死，两段只在这份格式上相接。6a 在 5c 之后；6b 不依赖第 5 步的服务端代码。两段做完再做整步的独立复核。


## 12. 权限、API 与 CLI 的规则（第 7 步）

2026-11-17 主会话在第 5 步进行期间定下。依据：矩阵 B14、B39–B44、B48、B50、B52、B58，合同 §7、§8，认证中间件与现有 handler 的写法。实现方对照源码核实后如有出入，写进报告，不自行改规则。

### 12.1 谁能用

读和写用同一条规则，三项都满足才放行：

1. **人工凭据（允许名单，B58）**。认证中间件把服务端核实过的凭据种类放在请求上下文里（`auth.AuthenticatedPrincipal`）。只接受种类为 `jwt` 或 `pat`、`AgentActor` 与 `Restricted` 都为假、请求不带 `X-Agent-ID` / `X-Task-ID`、`X-Actor-Source` 为空的请求；拿不到 principal 一律拒绝。写法照 `handler/comment_delegation.go` 里现有的人工判定，不复用 `RequireHumanActor`（它是拒绝名单，未知来源放行）。以后新增的凭据种类不改这里就进不来。
2. **当前工作空间的 owner 或 admin**。
3. **根任务还在时，能过 `UpdateIssue` 用的那道任务加载检查**（`loadIssueForUser`）。根任务已删除时没有这一项。实现方核实 `UpdateIssue` 在加载之外还有没有别的编辑权检查，有就一并套用，写进报告。

拒绝的应答只取决于调用方自己，不取决于根：第 1、2 项不满足时在查任何预算表之前就答 403、正文只有 `budget_forbidden`，有没有根应答都一样。`mat_`、`mcn_`、带 agent 提示头的人工凭据、普通成员都落在这里；`mdt_` 不被普通认证接受，是 401。Reporter、Reviewer、custom task 都是 task 凭据，同样被第 1 项挡住（B43），F2 的既有用例不受影响。

别的工作空间的根 id、issue id、原 key：按当前工作空间查不到，答 404、正文只有 `not_found`，与不存在的 id 完全相同（B14）。

**事务内复核（B41）**。第 4 步的 service 不解析权限。第 7 步在 service 的每个写操作里加一步：事务内、第一条写语句之前，重新读操作者在该工作空间的成员行，不是 owner 或 admin 就拒绝，什么都不写；应答与 handler 的拒绝相同（403 `budget_forbidden`）。它排在原 key 重放的查找之前：被降级的操作者重发自己当初的请求，得到的是拒绝，不是原结果。这是 READ COMMITTED 下的一次普通读取，不锁成员行（锁它会给 E2 锁图添一条新边）；复核之后、提交之前才提交的降级，等价于操作排在降级之前。凭据种类在一次请求内不变，只在 handler 判。

**2026-11-19 补充**（主会话写 7a 说明书时对照源码定，提交前都可以推翻）：

- 判定的顺序：第 1、2 项 → 路径里的 id 与原 key 的写法 → 请求正文的严格解码 → 第 3 项与各种查找 → service。第 1、2 项排在正文之前：不够格的调用方发来畸形正文，得到的仍是 403，应答只取决于调用方自己。正文排在查找之前：畸形请求不读任何预算表。
- 第 3 项的落法：`loadIssueForUser` 自己往应答里写句子（“issue not found”等），还会经 `rejectLegacyCollaborationAccess` 写协作边界的拒绝。预算路由拿一个丢弃正文、只记状态码的 `ResponseWriter` 去调它，再按状态码换成自己的正文：404 → 404 `not_found`；5xx → 500 `internal_error`；其余拒绝，启用路由答 422 `not_eligible`（落到这里的只有协作 issue，service 对它本来也是这个答案），别的路由答 404 `not_found`。不拆 `loadIssueForUser`：它是全仓库 issue 路由共用的入口，共享分支还在动它。
- issue 一侧的三条路由始终先过 `loadIssueForUser`。根任务删除之后它们答 404 `not_found`，包括按原 key 查启用操作那一条；读 tombstone 的入口只有按根 id 寻址的路由（12.2）。由此有一个接受下来的边角：启用已提交、应答丢了、根任务又在查询之前被删，调用方按原 key 查不到，重发也是 404。12.3 的“只有两种答案”要防的是换一个 key 重复提交，这里发生不了：issue 已经不在，换什么 key 都启用不了；根与操作行都还在库里。
- 按根 id 寻址的路由：先按工作空间读根（查不到 404），根任务的成员行没有 `tombstoned_at` 时再对根任务的 id 过一次 `loadIssueForUser`。读根与过加载之间根任务被删，这次请求答 404，重试即见 tombstone；不为此加锁。

### 12.2 路由

都在需要工作空间成员身份的那一组里（`X-Workspace-ID` 选工作空间）。没有列出全部根的路由（合同 §8：不新增工作空间全量列表）。

| 路由 | 作用 |
| --- | --- |
| `POST /api/issues/{id}/budget` | 启用。`{id}` 经 `loadIssueForUser`（UUID 或人类编号） |
| `GET /api/issues/{id}/budget` | 这个 issue 属于哪个根：根 id、它是不是根任务、绑定来源。不属于任何根时答 404 `not_found` |
| `GET /api/issues/{id}/budget/operations/{key}` | 按原 key 查启用操作 |
| `GET /api/issue-budgets/{rootId}` | `show`（12.4） |
| `POST /api/issue-budgets/{rootId}/operations` | 关闭、重开、调额、调开关；正文里的 `kind` 区分 |
| `GET /api/issue-budgets/{rootId}/operations/{key}` | 按原 key 查上面这些操作 |
| `GET /api/issue-budgets/{rootId}/observations` | 观察行，按时间倒序分页 |
| `GET /api/issue-budgets/{rootId}/attempts` | 执行列表，分页（12.5） |

- `rootId` 只接受 UUID。2026-11-19 更正：这里原先括注“`parseUUIDOrBadRequest`”，不成立，那个函数出错时写的是句子；预算 handler 自己做带检查的解析（`util.ParseUUID`），失败答 400 `invalid_request`，见 12.9 的同日更正。按根 id 寻址的路由在根任务删除之后照常可用，这是读“最小经济 tombstone”的入口。
- 原 key 是调用方给的规范小写 UUID（与 `issue report-snapshot` 的 `--request-key` 同一约定）。HTTP 入口只接受这种形状，所以人工路由查不到 `claim:…` 和用量批的操作行（它们的 scope 是 attempt）。
- 写请求的正文严格解码（未知字段、缺字段、类型不符都拒绝）。金额是 tick 的规范十进制字符串；`limit_ticks` 与 `attempt_reserve_ticks` 必须显式给出，值或 `null`。每个写请求都带 `reason`，除启用外都带 `expected_policy_revision`。
- 分页是 keyset（创建时间加 id），每页至多 100 条；`ListIssueBudgetEntries` 那样的无界读取不暴露给 HTTP。

### 12.3 应答

- 所有预算路由的应答，包括 401、403、404、409，都带 `Cache-Control: no-store`（B44）。401 由认证中间件在路由组之前发出，所以这个头要在它之前按路径前缀加上，做法照 `PlatformAdminNoStore`；`/api/daemon/budget/` 同样加。
- 错误正文只有一个代码，不带根 id、金额、成员、别的任务或 provider 会话：`invalid_request` 400；`budget_forbidden` 403；`not_found` 404；`not_eligible` 422；`already_enabled`、`root_conflict`、`parent_cycle`、`operation_conflict`、`policy_revision_conflict`、`root_deleted`、`state_conflict` 409。`policy_revision_conflict` 不附带当前的 revision，调用方自己去 `show`（B40：不替调用方取最新值）。
- 写成功：201 加操作结果；按原 key 重放：200 加同一份结果并标 `replayed`。
- 按原 key 查询只有两种答案（与 §11.9 同一原则，B33）：已提交，返回原结果，外加 `stale`（根当前的 policy revision 比这次操作产生的新，B41）；否则 `unknown` 加再次查询的路径。没有“不存在，可以换个 key 重来”这种答案。恢复方式是用原 key、原参数重发。

### 12.4 `show` 的内容（B48、B50、B52）

在一个只读事务（REPEATABLE READ、READ ONLY，不取任何锁）里读根行、装载账目、读缺口观察，三者出自同一个快照。应答里有：

- 身份与状态：根 id、根任务 id、准入状态（`open` / `closed` / `tombstoned`）、开关、policy revision 与 epoch、`ledger_revision`、启用时刻、结算轴（首包恒为空）。根任务已删除时只有这些 id 与数字，不带标题、编号或正文（预算表里本来就没有，读取侧不去别的表补）。
- 管理项：`limit_ticks`、`attempt_reserve_ticks`。后者标明是声明的预留，不是估算或上限（10.1）。
- 金额分列：A（已知实际）、E（估算）、R（预留）、H（未知占用）、overrun。E 不进 A，也不进占用。
- 未知：按原因计数，合并两个来源（10.12）。启用之前就在执行的那部分（`in_flight_legacy`，矩阵里的 `pre_activation_unknown`）单列一个数，不和启用之后的原因混在一起。
- `remaining_ticks`：没有设额度，或未知集合非空时为 `null`；从不用“额度减 A”代替。另给 `total_known`。
- 覆盖：纳入的执行数；`not_applicable` 是固定的机制名单（10.11）：`task_without_issue`、`collaboration_host_run`、`plugin_external_call`。它们不进分母，也不显示成 0 成本。名单是读取侧的常量，账目里不为它们写条目。
- 账目不可用（10.5）：`ledger_state` 为 `unavailable`，全部金额与 `remaining_ticks` 为 `null`。根行上存的投影此时无法验证，不拿出来显示。
- 开关为 `off` 的根照常可读，金额与关闭前相同，不归零；按原 key 查询照常可用（B52）。

读取可以把装载出的状态发布到 10.4 的缓存：`seq` 不超过快照里根行 `ledger_revision` 的条目都已提交。快照比缓存旧时不发布，也不丢弃缓存（10.4）。

2026-11-19 补充（5b 报告 R2，主会话定）：首包里读取侧不发布，上一段的“可以”不落实。`show` 与执行列表用 5b 的不持锁装载，装载出的状态用完即弃。这两条路由是人手动调用的，没有缓存时多付的只是一次全量重放（10.4 的实测：5 万条目约 0.3 s），而读取侧发布要多出一条“快照不比缓存旧”的判定和它的并发用例。写入方（领取、用量）照 10.4 发布，热的根上读取用到的仍是缓存加尾部。

### 12.5 执行列表

每行：attempt id、task id、领取代数、issue id（quick-create 为空）与 context 父任务、agent、runtime、状态、预留额（或未定额）、该预留名下的已知实际支出、协议版本（空表示旧 daemon）、创建时刻。合同的目标问题里第一项就是“有哪些执行、哪些预留还不能释放”，只有合计答不了它。纯计量包为此加一个按预留读取的方法（已知实际、未花部分、状态），不改 reducer；它在 5a 第二轮就加上（`Reservation`，10.13），第 7 步直接用。账目不可用时金额列为 `null`。

### 12.6 CLI

`multica issue budget` 下：`enable <issue>`、`show`、`set-limit`、`set-mode`、`close`、`reopen`、`operation`、`observations`、`attempts`。除 `enable` 外用 `--root <id>` 寻址；`show` 也接受 `<issue>`，先查它属于哪个根。

- 写命令必须带 `--request-key`（规范 UUID，由使用者提供并保存）、`--reason`、`--yes`；除 `enable` 外必须带 `--expect-revision`。CLI 不替人生成 key、不自动重试。结果不确定（网络错误、5xx、应答对不上）时只说“结果未知”，给出按原 key 查询的命令，并提醒保留原 key 与原参数；不建议换 key。做法照 `issue report-snapshot`（包括本地的人工凭据检查 `requireHumanLocalCommand` 和对 `mat_` / `mdt_` / `mcn_` 的拒绝）。
- 金额在命令行上是十进制美元（最多 10 位小数），按十进制精确换成 tick，不经过浮点；`none` 表示不设。输出同时给美元与 tick。`null` 显示成“未知”或“未设”，从不显示成 0。
- `show` 的文本输出把 A、E、R、H 分行，未知逐原因列出，启用前的未知单列；`--output json` 原样给出应答。
- 2026-11-19 补充（主会话，随 12.9 一起定）：每个子命令都有 `--output table|json`，缺省 `table`（这组命令是给人用的），`json` 逐字节打印收到的正文。`enable` 与 `set-limit` 必须同时给出 `--limit` 和 `--attempt-reserve`（值或 `none`），不靠省略来设或清一个金额。`operation` 用 `--root <id>` 或 `--issue <issue>` 寻址，后者查启用操作。本地的人工凭据检查对读命令同样生效。写命令的结局只有三种：已提交（201，或 200 且 `replayed`，并且结果对得上本次请求）、被拒绝（401，或正文是 12.9 错误形状且代码在 12.3 名单里的 4xx）、未知（其余一切，包括 5xx、网络错误、读不懂的 2xx、没有已知代码的 4xx）。读应答时不认识的字段忽略、不认识的枚举值原样显示；缺字段或类型不符算读不懂，不拿 0、`open` 或 `available` 顶替。

### 12.7 其余

- Core 加 `show` 应答的 zod schema、API 方法和畸形响应用例（仓库的 API 兼容规则）。首包没有 Web / Desktop 界面，不加页面、不加 hook。
- 内置 skill 文档与 source map：`issue budget` 是只给人用的命令，文档写明 agent 凭据会被拒绝，不要在任务里调用。
- 启用可以同时给额度与预留额（10.1 的补充）。
- `IssueBudgetService` 用连接池构造，不拿请求里已有的事务当 `TxStarter`（2026-11-18，第 4 步复核 12.3）。`pgx.Tx` 也满足这个接口，拿它构造的话 `transact` 就成了外层事务里的保存点：外层已经是 READ COMMITTED 时级别声明不起作用，外层是别的级别时被数据库拒绝（主会话在 PostgreSQL 17.11 上实测，5b 用用例钉住），根锁也会一直持到外层事务结束。事务内的成员资格复核是 `transact` 里多出的一条读取，`…TransactionsFollowTheLockOrder` 的五份语句清单各多一行。

### 12.8 必须有的用例

- B39 / B58：owner、admin 成功；普通成员、`mat_`、`mcn_`、`mdt_`、带 agent 提示头的 JWT 与 PAT、没有 principal 的请求，读写全部拒绝，拒绝前后各预算表行数不变。
- B14：另一工作空间的 admin 用根 id、issue id、原 key 读写，都是 404 `not_found`，正文与不存在的 id 逐字节相同。
- B40：过期的 `expected_policy_revision` 被拒且不写；开关关再开，epoch 是新的。
- B41：操作者被降级后，service 层的事务内复核拒绝；他先前提交的操作，另一位 admin 按原 key 读得到，并带 `stale`。
- B43：F2 的具名用例结果不变（与 B54 一起跑）。
- B44：401、403、404、409 四种应答都有 `no-store`，正文只有代码；根任务删除后 `show` 只有 id 与数字。
- B48 / B50 / B52：`show` 的各字段；有未知时 `remaining_ticks` 为 `null`；`off` 之后金额不变；整个用例前后共享项目策略表零写入（实现方指明是哪些表）。
- B42 余下的一半：被现有 finalize 检查拒绝的领取不写 attempt。
- CLI：结果未知时不换 key；金额换算的边界（10 位小数、超出 int64、负数、`none`）。

### 12.9 线上形状（HTTP v1）

2026-11-19 主会话冻结，让第 7 步的两段（12.10）可以各自动手。字段名、顺序、可空性和数字的写法以这一节为准。操作结果就是第 4 步已经存进操作行的那份 JSON（`IssueBudgetOperationResult`），金额与按预留读出的字段名沿用纯计量包已有的 JSON 名。实现方对照源码发现写不出来的地方，先报告，不自行改形状。

**通则**

- 请求与应答都是 UTF-8 的 JSON 对象。服务端按下面列出的字段顺序输出，紧凑格式，末尾一个换行（`writeJSON`）。读应答的一方不依赖顺序，遇到不认识的字段忽略（仓库的 API 兼容规则）。服务端读请求则严格：未知字段、缺字段、类型不符、重复的键、正文之后还有内容，都是 400 `invalid_request`。
- 金额（`*_ticks`）、`policy_revision`、`policy_epoch`、`ledger_revision`、`expected_policy_revision`、`settled_as_of_revision`、`claim_generation` 是规范十进制字符串（`budget.Ticks` / `budget.Count` 的写法：没有符号，没有前导零）。个数（`count`、`included_attempts`、`pre_activation`、`member_count`、`in_flight_gap_count`）与 `protocol_version` 是 JSON 数字。
- id 是规范小写 UUID 文本。时刻是 UTC 的 RFC 3339，以 `Z` 结尾，小数秒可有可无。
- 标了“可空”的字段总是出现，值或 `null`，不省略。例外只有操作结果里属于启用的三个字段（见下）。
- 预算 handler 发出的错误，正文恰好是 `{"error":"<code>","code":"<code>"}`：两个字段都是 12.3 的代码，没有句子。服务端自身出错是 500 `internal_error`，同一个形状。401 由认证中间件发出，正文是它自己的。2026-11-19 更正（主会话写 7a 说明书时对照源码）：这里原先括注“`writeErrorCode(w, status, code, code)`”，不成立。`writeErrorCode` 把一个 map 交给 `json.Marshal`，键按字母序输出，`code` 会排在 `error` 前面；`parseUUIDOrBadRequest`、`loadIssueForUser`、`requireWorkspaceRole` 出错时写的又是句子。预算 handler 因此不经这几个函数出错误正文：用一个字段顺序为 `error`、`code` 的结构体经 `writeJSON` 输出，路径里的 id 自己做带检查的解析，`loadIssueForUser` 的判定照用、它写的正文不外传（12.1 的同日补充）。fixture 与 7b 的读取不受影响：fixture 本来就是 `error` 在前，读的一方不依赖顺序。

**写请求**

`POST /api/issues/{id}/budget`（启用），四个字段都必须出现：

```json
{"request_key":"3f0e6f0a-6f1c-4c55-9d1e-0f2b7c1a9e10","reason":"Q4 launch","limit_ticks":"250000000000","attempt_reserve_ticks":null}
```

`POST /api/issue-budgets/{rootId}/operations`，`kind` 决定还要哪些字段，多给少给都是 400：

```json
{"kind":"set_limit","request_key":"…","reason":"…","expected_policy_revision":"3","limit_ticks":null,"attempt_reserve_ticks":"5000000000"}
{"kind":"set_mode","request_key":"…","reason":"…","expected_policy_revision":"3","mode":"off"}
{"kind":"close","request_key":"…","reason":"…","expected_policy_revision":"3"}
{"kind":"reopen","request_key":"…","reason":"…","expected_policy_revision":"3"}
```

`request_key` 是规范小写、非全零的 UUID。`reason` 与金额的取值范围由 service 判（第 4 步已有），handler 不另立规则。

**写的应答**

201 `{"replayed":false,"result":{…}}`。同一个 key、同一份意图再来是 200 `{"replayed":true,"result":{…}}`，`result` 与第一次逐字节相同。`result` 是操作行里存的那份：

```json
{"operation_id":"…","operation_kind":"enable","budget_root_id":"…","root_issue_id":"…","admission_state":"open","mode":"observe","policy_revision":"1","policy_epoch":"1","limit_ticks":"250000000000","attempt_reserve_ticks":null,"reason":"Q4 launch","activated_at":"2026-11-19T01:02:03.456789Z","member_count":4,"in_flight_gap_count":1}
```

`operation_kind` 是 `enable`、`close`、`reopen`、`set_limit`、`set_mode`。`limit_ticks`、`attempt_reserve_ticks` 可空。`activated_at`、`member_count`、`in_flight_gap_count` 只在启用的结果里出现，后两个为 0 时也不出现（存储形式如此，读的一方按 0 处理）。`admission_state` 在这里只会是 `open` 或 `closed`。

**按原 key 查询**（两条路由同形，都是 200）

```json
{"state":"committed","stale":false,"result":{…}}
{"state":"unknown","retry_path":"/api/issue-budgets/7d0c…/operations/3f0e…"}
```

`stale` 为真表示根当前的 policy revision 大于 `result.policy_revision`。`retry_path` 就是本次请求的路径，issue 一侧用 issue 的 UUID，不用人类编号。key 不是规范小写 UUID 时 400。

**issue 属于哪个根**（`GET /api/issues/{id}/budget`）

```json
{"issue_id":"…","budget_root_id":"…","is_root_issue":false,"bind_source":"create","bound_at":"2026-11-19T01:02:03.456789Z"}
```

`bind_source` 是成员行里存的值：`root`、`activation`、`create`、`reparent`。

**`show`**（`GET /api/issue-budgets/{rootId}`）

```json
{"budget_root_id":"…","root_issue_id":"…","admission_state":"open","mode":"observe","policy_revision":"3","policy_epoch":"1","ledger_revision":"42","activated_at":"2026-11-19T01:02:03.456789Z","settled_as_of_revision":null,"limit_ticks":"250000000000","attempt_reserve_ticks":"5000000000","ledger_state":"available","amounts":{"actual_known_ticks":"73500000000","estimated_ticks":"1200000000","reserved_ticks":"10000000000","unknown_hold_ticks":"5000000000","overrun_ticks":"0"},"unknown":{"by_reason":[{"reason":"lost_contact","count":1},{"reason":"observation_off","count":2}],"pre_activation":1},"total_known":false,"remaining_ticks":null,"coverage":{"included_attempts":12,"not_applicable":["task_without_issue","collaboration_host_run","plugin_external_call"]}}
```

- `admission_state`：`open`、`closed`、`tombstoned`。根任务已删除时是 `tombstoned`，其余字段照旧，`root_issue_id` 仍是那个 id。
- `limit_ticks`、`attempt_reserve_ticks`、`settled_as_of_revision` 可空；最后一个在首包恒为 `null`。
- `ledger_state`：`available` 或 `unavailable`。`unavailable` 时 `amounts` 为 `null`，`remaining_ticks` 为 `null`，`total_known` 为假，`coverage.included_attempts` 为 `null`，`unknown.by_reason` 只剩观察表给出的原因。
- `unknown.by_reason`：10.12 的两个来源按原因名合并后的个数，按原因名升序，个数为 0 的不列。启用之前就在执行的那部分不在里面，单列为 `pre_activation`：没有被覆盖的 `in_flight_legacy` 观察数，加上账目里原因为 `pre_activation_unknown` 的未知项数（首包没有写入方，后者恒为 0）。
- `total_known`：账目可用、`by_reason` 为空、`pre_activation` 为 0，三者同时成立才为真。
- `remaining_ticks`：设了额度并且 `total_known` 为真时是 `max(0, 额度 − (A + R + H))`，否则 `null`。
- `coverage.included_attempts`：账目里有预留的执行数。`not_applicable` 是固定名单，顺序如上。

**观察行与执行列表**

两条路由都接受查询参数 `limit`（1 到 100，缺省 50）和 `cursor`（上一页的 `next_cursor`，对调用方不透明）。新的在前。没有下一页时 `next_cursor` 为 `null`。参数不合法是 400。

```json
{"observations":[{"id":"…","kind":"coverage_gap","reason":"in_flight_legacy","issue_id":"…","task_id":"…","claim_generation":"2","attempt_id":null,"covered":false,"detail":{"task_status":"running"},"created_at":"2026-11-19T01:02:03.456789Z"}],"next_cursor":null}
```

`kind` 是 `would_refuse`、`attribution_difference`、`coverage_gap`。`issue_id`、`task_id`、`claim_generation`、`attempt_id` 可空。`detail` 是一个 JSON 对象，可以是空对象；读的一方原样显示，不解释里面的键。`covered` 只对在途缺口有意义（9.5），其余恒为假。

```json
{"ledger_state":"available","attempts":[{"attempt_id":"…","task_id":"…","claim_generation":"1","issue_id":"…","context_parent_issue_id":null,"agent_id":"…","runtime_id":"…","state":"open","reserved_ticks":"5000000000","protocol_version":1,"created_at":"2026-11-19T01:02:03.456789Z","reservation":{"status":"open","unsized":false,"reserved_ticks":"5000000000","actual_known_ticks":"1200000000","unspent_ticks":"3800000000","overrun_ticks":"0"}}],"next_cursor":"…"}
```

`state` 是 attempt 行的状态：`open`、`cancelled`、`timeout`、`process_exit`、`lost_contact`、`superseded_unproven`、`settled`。`reserved_ticks` 是行上的预留额，未定额时为 `null`（`"0"` 是定额为零，不是未定额）。`issue_id`、`context_parent_issue_id`、`protocol_version` 可空，最后一个为空表示旧 daemon。`reservation` 是纯计量包按预留读出的那份（`ReservationProjection` 的 JSON，`status` 为 `open`、`held`、`settled`）；账目不可用，或账目里没有这次执行的预留时为 `null`。`ledger_state` 与 `show` 的含义相同。

### 12.10 第 7 步的拆分

- **7a 服务端**：事务内复核（12.1），读取侧（`show`、执行列表、观察行、issue 所属、按原 key 查询）与它们的分页查询，八条路由，`no-store` 前缀中间件，12.8 里除 CLI 一行之外的全部用例。它要用 5b 的装载、5c 的开关读取和第 6 步的操作行，所以排在第 5、6 步的独立复核之后。
- **7b 调用方**：CLI（12.6），Core 的 `show` schema 与 API 方法（12.7），内置 skill 文档与 source map。它不依赖服务端的实现，只依赖 12.9，在自己的工作树里做，可以和第 5、6 步并行。
- 两段共用一个 fixture 目录 `server/internal/handler/testdata/issue_budget_http_v1/`。每个文件是一条紧凑 JSON 加一个换行，内容照 12.9。7b 先写出来，拿它们当假服务端的应答。7a 的用例把每个文件严格解码进自己的应答结构再编码，必须得到相同的字节（字段名、顺序、可空性、数字的写法由此对齐），并断言真实应答的键集合与对应的文件相同。fixture 与 12.9 不一致时以 12.9 为准，先报告再改。
