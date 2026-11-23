# Research: 共享任务的持久指令队列与执行控制

> 2026-10-17：本文是旧单会话研究，保留源码证据。M01–M03 已确认；当前逐目标 FIFO、执行组、两级控制和按资源授权见[调整清单](../mention-redesign-2026-10-17.md)及 data/execution-contract。旧唯一 FIFO 结论不覆盖新稿，原生证明约束仍有效。

- Query: 如何复用现有执行基础设施，实现每张业务任务唯一共享 AI 会话、严格逐条指令顺序、可信停止确认、撤权收尾和重启恢复，并防止所有旧入口绕过。
- Scope: internal；技术设计研究，不实施。范围以本任务 PRD 已确认的 R01–R38 为准，重点覆盖 R08–R17、R23–R28、R30–R35。
- Date: 2026-10-12。
- 方法: 静态阅读当前工作区代码、规范、PRD 和已有两份研究。没有运行测试、实际 AI CLI、服务或远程操作，没有据历史研究中的提交号声称当前工作区版本相同。
- 写入范围: 仅本文件。业务权限、最终表结构由权限研究及主会话设计定稿；下面区分已实现事实与建议契约。

## Findings

### 1. 结论与可行边界

建议复用 agent_task_queue、运行时容量调度、daemon 环境准备、执行器、消息/用量收集和通知机制，在其上增加**来源任务拥有的共享会话、逐条指令、执行关联和控制记录**。指令不是普通评论的别名，也不能直接采用当前“同 issue + agent 的 pending task 合并”逻辑。

共享会话持久保存两种互不替代的事实：

1. 队列是否允许领取：active / paused / closed。
2. 先前获准执行是否已证明结束：没有未结执行，或仍持有唯一 active_run 屏障。

task.status=cancelled、failed、completed、daemon 离线、过期 lease、连接关闭、恢复超时均不能单独清除第二种屏障。停止受理只保证暂停门禁已经提交；只有可信的未启动证明或进程退出证明才能确认停止。此结论也适用于自动重试、手动重跑、资源切换、整单结束后重开。

与权限同伴对齐的优选方向为**方案 B：执行关联**：业务 issue 留在来源工作空间；外空间 agent/runtime 的执行 task 通过明确的 collaboration purpose 和不可变关联连接到该 issue。不能把外空间 task 的旧 issue_id/chat_session_id 伪装成来源空间载体，再取消既有同空间校验。最终存储选择仍须由 design.md 定稿；本文件给出必须成立的接口，不私自扩大旧令牌权限。

可信执行收尾是交付前置条件。当前 admin_cancel_ack_v1 只能复用部分机制，不能直接承诺本功能全部停止保证；尤其需要补齐非托管环境、撤权后的窄回执通道，以及所支持 provider/OS 的进程树退出证据。

### 2. 已有文件与准确行为

下表中的行号为本轮静态读取位置，具体建议尚未实现。

| 文件:行 | 已实现行为 / 本次设计含义 |
| --- | --- |
| server/internal/handler/comment.go:1688、1719、1914、1955 | CreateComment 默认 type=comment，保存后仍调用触发解析；只有 /note 显式阻止旧触发路径。共享任务必须在服务端让普通评论、回复 AI、@AI 全部保持普通评论，不能只修改新客户端按钮。 |
| server/internal/handler/comment.go:2060、2132、2375、2413 | 待领取输入会合并到同 issue/agent 的 task；新评论替换 trigger，并整批重写 originator/accountable、MCP overlay 和 connected apps。不能用这张 task 行代表多人各自不可变的指令。 |
| server/pkg/db/queries/agent.sql:1947、1990 | coalesced_comment_ids 是去重后的数组计划，最新触发成为主输入；它不是有独立状态、作者及稳定顺序的命令队列。 |
| server/internal/handler/daemon.go:4060、4101、4107 | 完成时按 created_at 和 delivered_comment_ids 回放未送达评论，重新经过旧路由并合并为一条 follow-up；不是按每条输入依次执行。 |
| server/internal/handler/comment.go:2083 | 不同 head 的 queued blocker 场景仍有明示 best-effort 缺口，不能据“deferred”推断存在持久指令义务。 |
| server/migrations/257_agent_task_queue_channel_media_pending_unique_v2.up.sql:6 | pending 唯一键是 issue_id + agent_id，仅覆盖 queued/dispatched 和指定 media deferred；不保证每 issue 的全局单会话。 |
| server/pkg/db/queries/agent.sql:808、883、900 | claim 按 issue + agent（chat 则同 agent + chat）串行，明确允许不同 agent 同 issue 并行；跨任务按 priority、created_at、id 排序。新规则须按共享 session 独立锁定 FIFO 头。 |
| server/internal/service/task.go:3389、3414、3420、3445 | claim 先锁当前 managed runtime / binding，再锁 agent 并检查并发容量。共享会话屏障需要与这套调度协调，不能只在 API 入队时检查。 |
| server/pkg/db/queries/agent.sql:952、993；.trellis/spec/server/managed-installations.md:140 | 已派发未开始的丢响应恢复要求原 dispatch 超过 90 秒且 prepare lease 失效。托管回收保留原执行 fence、提升 claim_generation；时间窗口不是旧进程死亡证据。 |
| server/internal/service/task.go:3693、3757、3761、3793 | FinalizeTaskClaim 将任务令牌、可选 MCP daemon token、评论送达回执放进同一事务，比较 runtime/dispatched_at/claim_generation。可借鉴“构建完再原子登记送达身份”，但协作 purpose 要有自己的最小载荷和 token 校验。 |
| server/internal/handler/daemon.go:3766；server/pkg/db/queries/agent.sql:1118 | 现有 StartTask 请求依 task ID，SQL 只检查 dispatched/waiting 状态，未要求客户端提供 claim_generation 或单次 launch nonce。共享执行不能靠此接口防旧派发启动。 |
| server/internal/service/task_start.go:17、39、59、73 | start 有事务、工作空间/状态目录/I1/issue 锁及实际开始事实；应保留“真实 start 才记执行开始”，不能在提交指令、claim 或停止时伪造。 |
| server/internal/service/task_cancellation.go:65、106、137；server/pkg/db/queries/agent.sql:1689 | CancelTaskInTx 立即将运行行设 cancelled；ApplyCancellationEffects 随后广播和 NotifyTaskFinished。它没有等待物理退出，也没有共享队列暂停位。 |
| server/internal/service/task.go:6835 | NotifyTaskFinished 会清空 empty-claim 判断并唤醒运行时；新共享门禁必须在此类唤醒后仍有效，不能把 WS 唤醒当领取授权。 |
| server/internal/service/admin_operation.go:78、120、155、211 | 管理取消已有持久 operation、执行 fence、取消根/跟随者、审计及能力检查。queued 且未 dispatch 可确认未派发；已派发只在合适托管绑定上等待回执。 |
| server/internal/service/admin_operation.go:347、430、459、498 | 10 分钟 ACK deadline 到期只变 unconfirmed；metadata/ack 要求对应 daemon token 及 binding，ack 又调用 LockManagedRuntime。当前有效身份失效后不能直接照用。 |
| server/internal/handler/daemon.go:4875、150 | cancel-ack 和状态读取先 requireDaemonTaskAccess，再验证当前工作空间和运行时访问；撤销用户/运行时权限可能在收到回执前就拒绝请求。 |
| server/internal/daemon/cancel_receipts.go:181、228、294、339 | 本地持久取消 outbox 目前仅 managed 初始化；先落盘后投递，按原作用域重放，旧 boot 中仅观察过但未证明结束的记录改为 not_observed。可复用持久化模式，不可把 not_observed 当 stopped。 |
| server/internal/daemon/managed_transport.go:294 | 回执 transport 必须匹配当前 server/deployment/installation/user/auth_version/profile/binding/epoch；不匹配即隔离。不能把解除隔离当撤权后收尾方案。 |
| server/internal/daemon/daemon.go:5033、5069、5209、5259 | daemon 轮询终态、真实 404、401 时停止；网络/5xx 不停止。生产默认 5 秒轮询。401 可以触发本地 cancel，但没有 cancellation metadata 时无法形成相应停止证明。 |
| server/pkg/agent/agent.go:201；server/internal/daemon/daemon.go:8720；server/internal/daemon/cancel_receipts.go:400 | Result.ProcessExited 为明确 reap 证据；通道关闭、等待超时不会被当成退出。daemon 按每次 provider launch 计数，必须逐个得到退出证明。 |
| server/pkg/agent/codex.go:1369；server/pkg/agent/claude.go:277、353 | Codex 检查 Wait 返回、ProcessState 和进程组消失；Claude 当前只用直接子进程 ProcessState。检索其他 provider 未发现设置 ProcessExited 的路径；同名错误变量不等于退出证据。 |
| server/pkg/agent/proc_other.go:16、62；server/pkg/agent/proc_windows.go:80、110、142 | Unix 有进程组，Windows 有先挂起再加入 Job 的机制；Windows 绑定 Job 失败目前继续执行。新共享运行不能在无法确认 containment 时仍宣传可信停止能力。 |
| server/internal/service/task.go:4799、4937、5350、5653 | FailTask 内部自动 retry、孤儿失败后 MaybeRetryFailedTask、手动 RerunIssue 均能创建新 task；旧 rerun 还清除 pending slot。共享队列须统一接管，不能照用这种“清旧任务再插入”逻辑。 |
| server/pkg/db/queries/agent.sql:1430、1447 | daemon 启动 orphan recovery / stale sweeper 会将运行任务失败；其后可自动重试。并没有证据保证上一个 daemon 的 provider 子进程已经死亡。 |
| server/internal/handler/daemon.go:2020；server/internal/service/task.go:3761 | claim 要求来源上下文工作空间等于 runtime 工作空间；最终 token.WorkspaceID 同样绑定 runtime.WorkspaceID。明确的协作分支优于删除这些普通执行防线。 |
| server/pkg/db/queries/agent.sql:796；server/internal/service/task.go:7040 | task 没有独立 workspace_id；有些权限依据 agent.workspace_id，有些展示/事件依据 issue.workspace_id。直接跨空间复用旧关联会产生读写和广播身份分歧。 |
| server/internal/service/runtime_teardown.go:79、117、145 | runtime teardown 会取消、解除历史 runtime 引用及删除系统 agent；历史保护不能只依赖可被旧清理链删除的 task_id。 |
| server/internal/handler/issue.go:4096、4104 | 普通 issue 状态变化（含 cancelled）当前明确不停止执行；新 R35 应对协作 issue 增加原子收尾，不得把它描述为已有行为或无意改变普通 issue 契约。 |
| server/migrations/535_triage_execution_fence.up.sql:12 | lock_issue_execution 检查正式 admission 与评论 dispatch_eligible，用 KEY SHARE NOWAIT 保持现有单据存在；它不表示共享会话已恢复，也未代替本功能的单据完成/取消门禁。 |

### 3. 来源空间与执行空间的设计接口

必须分别命名，禁止从一个 workspace_id 猜出所有含义：

| 概念 | 权威来源与用途 |
| --- | --- |
| source_issue_id / source_workspace_id | 唯一业务任务、协作者身份、处理责任、状态目录、公开记录、持久队列及审计归属。通过 ResolveIssueAccess 取得。 |
| session_id / issue_epoch | 本单唯一共享控制域；转交、切换客户端、切换运行环境都不创建第二个控制域；关闭后重开提升 epoch。 |
| command_id / command_seq / author_user_id | 一条显式意图及真实作者，序号由服务端事务分配。永不借运行环境拥有者替换作者。 |
| run_id / task_id / attempt | command 的一次执行尝试与现有执行引擎的关联；两者可能分离存储，task 删除不能移除本单 run 历史或未结屏障。 |
| execution_workspace_id / runtime_id / agent_id | daemon 传输、资源所有权、容量及环境准备范围；由已验证的运行授权决定，不能来自客户端任意提交的 workspace header。 |
| grant_id / grant_epoch / resource_epoch | 当前本单资源授权及绑定版本；撤回、重新授权、切换都产生不可混淆的版本，旧授权永不复活。 |
| execution_fence / claim_generation / launch_nonce | 分别识别一次获准执行、一次派发响应、一次允许启动的本地 launch；不与 session_version、供应商 session_id 混用。 |

优选 B 的工作流：

1. 来源空间协作事务保存 command；授权接口返回允许的执行绑定快照。
2. 只为队首生成 collaboration purpose 的引擎 task 和 run 关联。关联插入、command 占位及 session.active_run 一次提交。
3. 新 claim 分支在来源空间读取经授权的 SourceIssue 投影，在执行空间读取明确授权的运行配置。投影只含本单内容，不把来源项目的其他任务、目录或外空间资源目录一并暴露。
4. 新用途的业务执行凭证只能访问本单允许的接口；普通 mat_ 并非单 issue capability，不能同时注入它作为“备用”。daemon 传输凭证、业务能力凭证、供应商环境账号、终结回执凭证是不同权限面。
5. 完成事件回到来源空间保存命令结果、公开执行消息/用量和交付附件引用，受本单访问权限控制。执行空间的普通成员或 runtime 管理列表不能因此取得来源任务内容。

反向隔离也要进入实现：generic task.context 只保存不可变关联 ID，不塞来源指令正文；server/internal/handler/agent.go:2647、2921 的任务列表/快照及普通 task WS 事件必须按 purpose 过滤或输出无内容元数据。不能因为 task 属于外空间 agent，就让外空间成员看到来源内容或取得本单取消权。

必须落实内容留存：来源空间要有独立持久的 run/message/result/usage/attachment-manifest 记录，或明确受保护且不会被外空间清理删除的等价存储；单纯保留 task_id 外键式逻辑引用不够。若采用异步投影，输入接收与“待复制”义务同事务，须在外空间删除前完成/保护已接收记录，并保留失败可重试身份；最好让协作回传入口直接写来源记录。任何方案都不得依靠两次无事务的双写兜底。

备选 A 是给旧 task.issue_id 直接挂来源任务，同时增加明确 execution_workspace_id 与 purpose。其代价是必须重审所有旧 owner resolver、task read/WS、重试、issue 自动状态、删除与 token minting 路径，现有相等校验只能改为按 purpose 分支而非移除。B 不免除适配，但能更清楚隔离普通执行；最终表结构和清理范围由权限设计决定。

### 4. 建议状态模型

以下名称是设计术语，不代表已有数据库表或已添加枚举。

#### 4.1 会话

- queue_state: active、paused、closed。active 且没有 active_run 才是“空闲可领取”；paused 可以同时正在停止；closed 可以同时旧进程未确认结束。
- active_command_id / active_run_id: 同时至多一个。必须由锁定 session 的事务设置和清除。
- control_version: stop/resume/switch/close/reopen 等控制改变时递增；active_generation 标识队首执行变换。普通评论及新增排队指令不能无意义地使同一运行上的停止失效。
- event_seq: 全会话事件顺序，用于快照/增量重连；与 control_version 分开。
- issue_epoch、resource_epoch、next_command_seq、pause_reason、paused_by/control_operation_id 均持久化。
- 不保存 resume_requested、auto_resume_after_stop 等未来恢复意愿。关闭/撤权/重连/回执都不能隐式写 active。

| 事件 | 会话结果 |
| --- | --- |
| 显式发给 AI | 追加一条 command；active 时尝试唤醒调度，paused 时仅排队，closed 时拒绝。 |
| 有效 Stop | 在同一事务先设 paused，再记录当前 run 的终结要求；未执行队列保留。无当前 run 也可以暂停。 |
| 收到可信退出证明 | 只结算对应 run、清空它的屏障；paused / closed 原样保持。 |
| 明确 Resume | 当前协作权限、开放业务类别、有效资源授权、兼容运行环境及无未结旧 run 全部满足时才 active；空队列保持空闲。 |
| 撤回当前资源授权 | grant 失效、paused、当前使用该资源的 run 收到停止要求；其他作者指令保留。 |
| 普通转交 / 补充完成交回 / 提交验收 | 队列和 run 不变。 |
| 更换运行环境 | 必须 paused、无未结旧 run、新授权有效；提升 resource_epoch，仍 paused。 |
| 整单完成 / 取消 | closed；所有尚未执行 command 取消；当前 run 请求终止并保留屏障。 |
| 重开 | 提升 issue_epoch，恢复业务可提交状态但会话仍 paused；旧 run 屏障保留，旧 command 终态不变。之后重新明确恢复。 |

“重开后保持暂停”是满足 R10/R17/R35 的保守技术建议；PRD 未明确授权重开本身等价恢复。不能用新会话或新 epoch 绕过旧 run。

#### 4.2 指令

- waiting → in_flight → succeeded / failed / stopped。
- waiting → cancelled，用于作者退出/被移除、整单结束等；保留文本快照、author_user_id、授权身份 epoch、序号、原因及操作者。
- in_flight 包括派发/准备、运行、同指令 retry_wait、等待可信结束；界面通过 run 子状态区分，不把排队称为已送达。
- 自动技术重试若保留，始终属于同 command，增加 attempt，不改 author、不分配新 command_seq，不放行后续 command。
- 人类要求再做一次是新 command、新作者及新序号，可带 rerun_of_command_id；不能复活旧 cancelled/stopped 行。
- command 内容与附件版本在接收时固定。普通评论编辑/删除不能悄悄改写已经接受的指令；如未来提供指令编辑，应作为新的、独立授权且带版本的操作，不套用旧 comment merge。

“尚未执行”应按是否取得 launch 许可/已启动证据判断，不只看现有 task.status：被移除者的 waiting、retry_wait、已派发但确定尚未启动的输入均取消；若可能已启动，先取消它的继续执行资格并保留终结屏障，向该 run 请求停止。已获准的当前命令被用户 Stop 后，恢复不得自动再跑它，即便停止赶在实际 provider fork 之前。

#### 4.3 执行 run 与证据

建议保留进度状态与执行事实两条轴：

- 调度进度: prepared → dispatched → preparing → launch_authorized → running → ending → settled。
- proof_state: never_launched_proven / may_be_live / exited_proven / unknown。
- disposition: success / failure / stopped / cancelled_before_start。

prepared、dispatched 不是进程存在证明；launch_authorized 也不能保证进程已创建，但从这一刻必须按“可能存活”保留屏障。永不将 task 的终态直接映射成 exited_proven。started_at 只由实际 daemon start 协议提交；供应商成功结果和公开输出也不能替代进程收尾证明。

同 command 的任何后续 attempt（包括 daemon 内部 session-resume fallback、初始化重试）必须等待前一 provider launch 已确认退出，并重查停止要求。主 task ID 相同并不意味着内部可以同时启动多个 provider 进程。

#### 4.4 控制操作

每个操作者的 request_id 有自己的持久记录和审计；同一 run 的并发 stop 可引用同一 cancellation root，但不能合并掉真实操作人。区分 accepted、confirmed、unconfirmed、rejected；accepted 表示数据库门禁生效，confirmed 表示具备所需证明。

恢复请求在 stopping 未确认时返回 409 stop_unconfirmed，保存最终拒绝回执但不保存任何未来执行意图。之后即使收到退出证明，重放相同 key 仍返回原拒绝；用户须在当前快照上创建新的 request_id。同 key 改 payload 返回 409 idempotency_conflict。

### 5. 必须成立的不变量

1. 每 source_issue_id 仅一个持久共享 session；重开、转交、不同客户端、agent/rerun/CLI 入口不能另建并行域。
2. 每 session 最多一个未结 run；约束覆盖旧 task 已被标终态但进程未证明退出的情况。
3. FIFO 由数据库在 session 锁内分配的单调 command_seq 决定，不靠客户端时间、UUID 顺序或现有 priority。不同 issue 之间仍可使用现有容量调度。
4. 每条新指令有独立 author、内容快照及提交身份；后续其他人的评论和指令不改写它。
5. 入队、排队、claim、派发最终化、start、同指令 retry、resume、switch 各自重新验证当前协作/资源门禁。不可只检查第一次提交。
6. Stop 的 paused 写入与停止意图同事务；任何唤醒、完成、GC、超时、重连均不能取消暂停。
7. 被移除/退出作者的未执行指令取消；其他作者的未执行指令保留。若离开者拥有当前资源，则不论当前命令作者是谁都请求停止并暂停。
8. issue 的 completed/cancelled 类别及同类别自定义状态阻止新命令、claim、start、retry、resume；未知类别无法安全解析时拒绝执行。
9. 关闭事务取消所有未执行指令。重开提升 epoch，不重放旧请求、不复活旧命令、不清理未结旧进程。
10. 提交结果成功、消息已落盘、task 行终态、物理进程结束分别记录。不存在“过十分钟自动算停止成功”。
11. 恢复/切换要求当前有效权限；回执只证明当时某个 run 已结束，不给发回执者恢复、读内容或启动新工作权限。
12. 原资源环境的供应商账号不因作者/处理人变更而替换；业务 token 仍只代表真实 command 授权，禁止退回 owner PAT 或旧宽域 mat_。

### 6. 事务、锁顺序与幂等

#### 6.1 推荐锁协议

数据库为唯一调度权威；不用进程内 mutex、Redis 或 WS 单独保证串行。所有共享 writer 都要走同一事务内 helper，不在事后 listener 中修队列。

新共享路径建议采用如下顺序，最终必须与权限报告的锁协议合并，并以真实并发测试验证：

1. 预读不可变 ID 和准备外部资源；网络、供应商查询、MCP overlay 构建放在事务外。预读结果只是 hint。
2. 获取现有身份/资源前置锁：用户凭据版本；涉及 managed runtime 时保留 user → namespace → installation admission 的顺序（managed_runtime.go:121）。跨多个运行时按规范的 namespace 键排序。
3. 来源/执行 workspace 的存在性 KEY SHARE 按 UUID 排序；进入既有目录/成员/状态/I1 写路径时沿用各自前置 fence。与旧 writer 的顺序冲突点必须用 NOWAIT 并整体回滚重试，不增加相反方向的等待。
4. 来源 issue → session 行锁。协作关闭/重开、参与者变更、资源授权、调度都在同一 session 上串行；跨 session 操作按 issue UUID 排序。
5. 本单 participant/grant 行 → agent 容量行 → command/run → engine task，稳定 ID 排序。agent/task 与现有 writer 交汇时使用 NOWAIT，避免旧 claim 的 agent→issue 路径和新 session→agent 路径形成环。
6. 控制回执/消息/业务事实/outbox，同事务提交。网络通知只在提交后发送；通知丢失可由轮询和 outbox 修复。

授权 helper 必须接收当前 pgx.Tx 的 queries，不能在持锁时用 pool 再查一次授权或再开事务。不能在已持有 session/task 锁之后调用会重新获取 user/namespace 的普通 wrapper；应拆出 borrowed-transaction primitive。实际行锁取得后再次检查身份、grant/session epoch、actor eligibility、issue 状态和选中的运行配置；发现预读实体变了就回滚重新读取。

这不是对全仓库已有锁序的断言：当前 start、claim、W01、teardown 有不同锁入口，本轮只核查了上述关键路径。实现前需要逐 writer 的锁序表，不能把新增“总序”写进文档就当已消除死锁。固定次数仅重试已回滚的 55P03/40001 等可重试冲突，不重试未知 commit 结果。

#### 6.2 原子操作边界

| 事务 | 必须一次生效的写入 |
| --- | --- |
| Enqueue | 当前权限/开放类别校验、幂等记录、command_seq 分配、不可变 command、公开输入记录、event/outbox。 |
| Promote / materialize | 选择最早有效 waiting command、设置 active_command/run、创建唯一 engine task/关联、绑定当前 grant/resource epoch、outbox。不能先插 task 再补 session 屏障。 |
| Claim / final payload | 校验唯一 active run、门禁、绑定、当前 author 权限、claim_generation、构建快照版本；提交本次 payload/token 的完整身份后才返回响应。 |
| Start | 比较 run + task + execution fence + claim_generation + launch_nonce，检查 gate/epoch，将“本次允许启动”事实与真实 start 记账绑定。重复响应只对同一本地 launch 生效。 |
| Stop | 先暂停会话、记录 root/actor 操作、给当前 run 写停止原因/控制版本、撤销其业务执行能力；保留后续队列。 |
| Remove / leave | 撤销 participant 身份、取消该 author 未执行指令；若其命令正在执行或其拥有当前资源，撤回相应 grant、暂停并请求停止；处理人回交发起人按 R32 同事务。 |
| Withdraw grant | grant 失效/epoch、暂停、当前使用该 grant 的 run 停止、业务 token 失效和审计。不能因网络停机失败回滚已生效撤权。 |
| Complete / cancel issue | 来源业务状态/修订/I1 事实、closed、所有未执行 command 取消、当前 run 停止、审计及通知身份。 |
| Accept terminal proof | 严格核对原 run，写唯一证明、结算它的 command/attempt、更新安全 resume pointer、清掉该 run 屏障；不改 paused/closed，不执行 rejected Resume。 |
| Switch / reopen | 比较当前控制版本/资源版本/业务 epoch；切换或重开不会复活终态 command，也不自动恢复。 |

“移除作者正在运行”和“关闭单据”可先将业务 task 标 cancelled 以沿用 daemon 停止信号，但 source run 的 may_be_live 屏障仍保留。推荐优先由专用 control_state 传输停止，避免依赖 task.status 的二义性；若首版桥接旧取消状态，也必须用 purpose 隔离其唤醒/重试副作用。

#### 6.3 幂等与版本

- 人类请求 key 建议以 session + actor_user_id + operation_kind + request_id 唯一；记录规范化 payload hash、已接受/已拒绝结果及对应 command/run/version。
- 授权先行：已被移除的人不能借幂等查询读回本来无权查看的内容。有效调用者重放成功操作只读取旧结果，不能再次切换、再次恢复或把处理责任拉回。
- command_seq 与事件序号使用 BIGINT；API 以十进制字符串传输，避免 JavaScript 安全整数边界。DB 时间用于审计，不能承担指令序列或唯一 launch 身份。
- claim_generation 是一次送达尝试，执行 fence 标识同一次引擎执行；增加稳定 run/launch nonce，不能把重新派发当新 command。
- 可建立唯一约束：每 issue 一 session；每 session + seq 一 command；每 command + attempt 一 run；每 session 至多一未结 run；每 run 一有效 task 关联；每 producer + event/receipt key 一条已接受记录。
- 新索引遵守 CREATE UNIQUE INDEX CONCURRENTLY 单语句迁移；不新增 FOREIGN KEY/CASCADE。先审计重复/历史数据再建约束，不在触发器中偷偷补外部权限。
- commit 结果未知时按原 request/run ID 查询并重放原结果，不另外生成 UUID 再发。返回 HTTP 成功不得早于持久化。

### 7. 领取、停止、完成的竞争结果

停止请求应带 expected active_run/active_generation 和 control_version，而不是只传 task ID。进度更新不改变这两个控制身份。下面是推荐的可解释结果：

| 谁先取得 session 门禁并提交 | 必须观察到的结果 |
| --- | --- |
| Stop 先于下一条 claim | paused 已提交；claim/finalize/start 全部拒绝下一条，任何 stale WS hint 无效。 |
| claim 已提交，Stop 后提交 | 该 run 成为唯一被停止对象；禁止后续领取。若从未发出 launch 许可，可事务性撤销未启动 run；若可能收到许可，保留屏障直到可信 not_started/exited 证明。 |
| payload finalization 前 Stop | finalization 的 gate/epoch CAS 拒绝发凭证和可执行载荷；已建立 run 留有取消/未启动记录，不能变成 orphan 后被旧 reclaim 启动。 |
| start 先提交、Stop 后提交 | 启动许可已属于旧 run，Stop 请求中止它；在它退出前没有第二个 run。不能把停止受理文案写成进程已经退出。 |
| Stop 先提交、延迟 start 后到达 | 相同 run 的旧 generation/nonce 被停止 gate 拒绝，daemon 必须确认未启动，不得按“任务已有派发记录”继续。 |
| 正常完成和 Stop 竞争 | 仍需退出证明。Stop 先则 command 的 disposition=stopped，迟到 success 不改成可重试；完成先且下一条尚未占位，可接受 pause 并返回 already_ended；两者都不自动恢复。 |
| A 完成、B 已领取，客户端仍用 A 的 fence Stop | 返回 409 stale_execution 和当前可见快照，不把原本针对 A 的旧请求套到 B；B 要用新的明确停止意图。保证针对“已受理 Stop”的线性化边界，不能假称请求到达前没有竞争。 |
| Resume 在退出证明前到达 | 固定拒绝 stop_unconfirmed；之后回执不触发恢复。 |
| Resume 和资源撤回竞争 | 撤回先则 Resume 拒绝；Resume 先则撤回随后把 session 重新暂停并拒绝新 claim/start。 |
| 作者移除和 claim/start 竞争 | 移除先则其未执行 command 取消；start 先则指令可能已执行，暂停并终止对应 run。不能由别的 author 接管该旧 command。 |
| 关闭和新 command 入队竞争 | 关闭先则入队拒绝；入队先则关闭将尚未执行的新 command 一并取消。重开之后重放旧 key 仍是旧结果。 |

远程进程 fork 与数据库提交无法做同一原子事务。因此“撤回立即生效”应明确为服务器自撤回提交起不再授予新的执行许可；此前已经拿到许可的旧 run 是停止对象。daemon 须在本地 launch 锁内重查已收到的 stop/fence，再 fork；如果网络恰在最后一次检查后分区，不能承诺零毫秒物理停止。排他屏障负责保证这种不确定性不会放行下一条。

### 8. 可信退出与撤权后的回执

#### 8.1 哪些证据有效

允许释放共享屏障的证据只应有：

1. **服务器证明从未授权启动**：对应 run 从未发放 launch permit；同事务撤销全部待派发/可启动身份，并确认没有旧版本协议能直接执行它。
2. **daemon 证明永不启动**：可能送达的 run 已在本地持久启动账本中写终结 tombstone，所有重复派发/nonce 重放将拒绝 fork；与“本次没在进程表找到”不同。
3. **daemon 证明已结束**：匹配 run/launch/fence 的每次 provider process 均已 reap，受控进程组/Job 已结束，终结记录先可靠落盘再发送。

不得采信：简单 HTTP 200、旧空 cancel-ack、连接关闭、任务终态、消息 drain 完成、无心跳、lease/deadline 过期、进程路径或 PID 单独不存在、not_observed、新 daemon boot、token 401。正常完成和失败也需要对应进程收尾事实；正常 success output 不应成为 bypass。

当前 ProcessExited 仅够作为现有“明确 reap”接口雏形。Codex 的进程组确认可复用；Claude 与其他 provider 的证明要补齐。Windows Job 取得失败必须对共享运行拒绝启动或返回不支持，而不是沿用当前 fail-open 后声称可确认停止。能力矩阵至少按 daemon 协议、provider wrapper、OS/进程 containment 实际结果判断。

推荐让统一 run/process-tree supervisor 负责启动身份、所有内部尝试的回收及 proof 生成，provider wrapper 提供自己的完成信号；这样避免每个 wrapper 各发一张语义不同的“已结束”。这是一项待实施能力，不表示已经确认永久排除某个 provider 或 OS。可以先记录业务失败和错误文案；在证明缺失时保留执行屏障及 unconfirmed，不能为了结束失败状态而解锁下一条。

这里信任的是已绑定 daemon 的正确实现，不能对一个主动伪造退出事实的恶意主机给出物理证明。签名和 capability 防止其他账号/运行伪造回执，不证明操作系统本身可信。PRD 的“本单共享会话”也不自动涵盖 AI 在远端另外创建的云端后台任务；首版执行器应禁用脱管后台 launch，不能宣传能停止不受 daemon 管理的工作。

#### 8.2 当前撤权后确认为什么会卡住

现有 managed 回执有三道当前权限门：HTTP requireDaemonTaskAccess、service LockManagedRuntime、daemon cancellationReceiptTransport 的当前 user/auth_version/binding 比对。账号禁用、密码版本变更、binding 撤销后，这些门可能一起拒绝原回执。daemon 看到 401 会取消本地执行，但观察函数只有成功读到带 metadata 的 cancelled 状态才记录 operation/fence。

所以以下方案不成立：先撤掉全部权限，然后等旧 /cancel-ack；把 401 当已停；回执不通就重新授权 owner；改用 owner PAT；把旧 binding 当当前；在普通 middleware 对所有终态任务放宽访问。

仅撤回“本单资源 grant”而未变更 user/binding 时旧传输未必立即失效，但仍不能依赖此偶然条件；R31、账号禁用、外空间生命周期均要求收尾独立于当前业务权。

#### 8.3 推荐窄终结协议

在本次执行仍获授权、claim/launch 身份最终化时，预先建立 **termination purpose** 的能力；必须在可能启动 provider 之前持久化到 daemon 私有启动账本。建议用已有随机令牌/哈希校验机制，或已安装身份的签名能力，不引入依赖：

- 绑定 deployment + session + run + command + task + execution fence + launch nonce + 原 installation/key/namespace；在服务器保留不可变、最小化的验证记录，不依赖已删除的运行时/用户行重新推断身份。
- 仅允许读取该 run 的停止要求及提交该 run 的 not_started / exited / not_observed 证据和 receipt ID；不返回 issue 内容、评论、附件、环境凭据、其他任务列表。
- 不能领取、start、延长 prepare/运行授权、恢复队列、切换资源或修改业务状态；也不能作为业务 actor 或签发其他 token。
- 仅 daemon/supervisor 持有该凭证；不得注入 provider 子进程环境、prompt、浏览器 DTO、任务公开 context 或普通日志。业务执行器持有的单据能力不能自己伪造物理退出证明。
- 不因协作者退出、资源 grant 撤回、owner 账号禁用/密码轮换而获得新能力，但其原有**收尾能力**可继续使用。若原凭证因密钥泄露需显式吊销该窄能力，必须保持 unconfirmed，不能自动放行。
- 建议专用路由和 middleware，不能套旧 RequireHuman/WorkspaceMembership 后开一条特例，更不能让旧 mat_/MDT 普遍绕过 revoked 检查。
- 终结回执字段严格解码，拒绝未知/重复/大小写混淆的关键字段；核对不可变 fence 和 launch 身份，原 receipt ID + digest 幂等；旧 run 回执不能触及新 run。
- 首次终结 proof 与历史记录同事务提交；确认后保留可重复查询的窄 tombstone 和相同回执结果。不能刚确认就删除验证行，使 daemon 丢响应后永远无法清 outbox。
- ACK 超时只改变可见 unconfirmed 状态，不改变安全性，也不自动恢复。若能力设置有限有效期，需有同等窄范围续交/保留策略；否则到期后只能停留未确认。不能用延长业务登录代替。

资源/用户撤权事务同时撤销普通业务执行能力并写 stop intention。daemon 用窄通道获取 stop，并在本地停止后用原 scope 提交证明。该通道无内容权限，即使仍可用也不恢复被撤销的协作身份。签名备选方案同样必须固定历史 run 的 key/purpose，不要求用户重新登录，也不得通过签名请求重绑定运行时。

停止后的晚到公开内容/用量是否仍接收应走独立、有限的“已发生执行尾部回传”能力和序号/额度边界，由权限设计明确；不要顺手给 termination token 添加任意 messages、附件、路径或结果写入权限。现有 cancel-ack 携带 branch/workdir 字段只能在相应受限回传协议中按原 run 校验。

### 9. 重连、重启、重试与环境切换

#### 9.1 重新连接与服务重启

- Web/Desktop 刷新只读 session/command/control 快照，按 event_seq 补历史；任何“自动恢复本地 optimistic 状态”都不能发送 Resume。
- daemon 重连只恢复原 run 的观测、回传和停止状态；server persisted paused/closed 为权威。当前 WS RPC sent/unsent 及 generation 机制可以保留，但未知送达结果不能立即走 HTTP 再发一份。
- server 重启依赖 DB 的 session.active_run 和 outbox 继续协调；不能从内存 active task map 是否为空推断安全。
- daemon 本地账本记录 run/fence/launch_nonce、启动阶段、受控进程身份、终结 proof、未确认 outbox；文件权限和原子写入可复用 managed 文件协议。
- daemon 崩溃后未知旧子进程应进入 unknown/unconfirmed；普通 RecoverOrphanedTasksForRuntime 可以标注引擎故障，但共享目的任务不能自动进入旧 retry 路径。只有原 containment 恢复后确认已结束/已禁止启动，才可清屏障。
- 本地账本丢失、旧主机长期离线、权限不足无法验证进程等情况保持暂停。人工点击“我确认已停”、管理员代管、换 runtime、重建 session、等待超时均不是 R17 允许的替代证据。若要支持机器重启证明或管理员外部终止证明，需要另行定义可验证协议，当前代码未提供。

#### 9.2 重试、follow-up 和 rerun

区分三类行为：

1. **网络请求重放**：同 operation/command/run/receipt，绝不增加指令或重复启动。启动响应丢失只能由持久 launch 身份恢复，不能把“server 已 running”误当本地从未启动。
2. **同指令的技术重试**：只有前一 attempt 已可信结束、command 非 stopped/cancelled、issue 开放、会话 active、原作者和 grant 仍有效才可创建。retry_wait 仍占 FIFO 队首，后续指令不能因 retry backoff 越过。保留现有失败分类与预算有复用价值，但不能直接使用旧 FailTask/MaybeRetryFailedTask 的自动插入路径。
3. **新的业务要求或手动重做**：由当时有权限的人显式创建新 command，追加队尾。停止/移除/关闭留下的旧条目绝不转成它的自动 retry。

普通评论回放、@mention 路由、squad leader fallback、autopilot、webhook、生命周期 follow-up、delegated failure recovery、旧 rerun、chat quick action 都必须对共享 issue 执行以下一致策略：没有明确获授权的 command 身份就拒绝/不入队；有显式意图的已适配入口只调用同一 enqueue 服务。agent 内部“给自己再派一单”也不能绕过本单会话；建立不相关私人会话不得取得本单执行关联或授权。

当前自动技术重试会改变用户感知的“一轮结束”。建议首版保留有界技术重试作为同 command 的 attempt，且展示重试等待；是否所有失败均暂停整条队列不是已确认需求，应由主设计明确，不能偷偷沿用旧策略。最低安全要求是不确定物理结束时必暂停，业务上明确失败且已结束时不能把下一条并发启动。

#### 9.3 更换资源与供应商会话

- 切换仅由发起人/当前处理人执行；后台 rebind、daemon reregister、运行时删除也必须受同一源 session 门禁。
- 新 grant 已由拥有者确认；旧 active_run 为空或有可信结束证明；queue_state=paused；比较 expected resource_epoch/control_version，整个切换原子提交。
- 保持同 session 与 command_seq。等待命令不固定到旧 runtime；真正成为队首时快照新的 grant/resource_epoch。已停止/取消条目不会随切换重入队。
- 供应商 native session_id/workdir 只在同 runtime/provider/有效资源代次下恢复。切换时清当前可恢复指针，保留历史；旧 run 的晚到 session 更新不能覆盖新代次。
- 不能用旧 GetLastTaskSession 的 agent + issue 搜索跨环境恢复，也不能把旧 task/workdir 当新授权范围。现有 resume-unsafe / retired_session_id 的思想可以复用，但共享 pointer 必须有 resource_epoch。
- 文件仅通过 R27 的 Git 已提交内容或显式交付附件准备；环境准备成功本身不启动执行。新环境不自动获得旧未提交工作目录、私人 provider session 或个人凭据。

### 10. 兼容门禁与需要适配的 API / RPC

| 入口 | 建议行为 |
| --- | --- |
| 共享评论 POST | 默认纯评论；旧客户端缺少新字段仍为纯评论。禁止回复/@mention 自动执行。已有一般评论历史保留。 |
| 显式 command POST / 列表 GET | 新用途接口；返回稳定 command/session/seq、真实作者、入队状态、当前控制版本，不以普通 comment 201 冒充任务已执行。 |
| Stop / Resume / Switch | 单会话操作、request_id、expected control version/active run/resource epoch；Resume 的拒绝结果可按原 key 查询。 |
| 旧 issue cancel-task / generic task cancel | 对共享 task 验证本单 AI 控制权并路由到 session Stop，或返回明确需升级冲突；来源空间成员身份不能直接取消。 |
| 旧 issue rerun / direct agent assignment / quick action | 对共享 issue 不再用旧 pending-slot 清理/普通 enqueue；必须转成明确 command 或拒绝。不能因调用入口旧而默认获授权。 |
| claim 单条 / batch / WS RPC / HTTP fallback | 全部识别 collaboration purpose 与源 session 屏障；相同服务 primitive、能力门禁和 generation 检查。旧 daemon 不能领到自己会当 quick-create 或普通 issue 执行的载荷。 |
| start / prepare lease / progress / session update / complete / fail / messages / usage | 专用 purpose 校验 run/task/fence/generation；业务内容回传检查来源授权及留存策略。start 必须有稳定 launch nonce，终态 callback 携带明确证明或等待独立 proof。 |
| termination status / receipt | 独立窄凭证，撤权后可收尾；不通过普通内容/管理接口。 |
| operation status / 快照与事件 | 权限化查询自身/可见控制记录与最新实际状态；与旧 workspace 广播分开，避免只订阅外 runtime 工作空间才能看到进度。 |

能力建议分开表达协议与实测运行能力，例如 shared_issue_queue_v1、execution_fence_v1、termination_receipt_v1；最终命名不是本研究定案。不能仅信任浏览器自己宣称 capability，服务器必须在 runtime 注册和每次 claim/launch 校验 daemon 能力及 provider/OS 支持情况。

旧 daemon 不支持时，本单指令保留并显示 runtime_upgrade_required / stop_confirmation_unsupported，不回退普通 task、私人 chat 或另一会话。已经可能运行的旧执行保持 unknown/unconfirmed，不能用“不支持确认”当“无需确认”。能力丢失、runtime 降级或协议滚动部署必须同样阻止 fresh claim。

启用功能时服务器所有 writer 都必须已识别共享 purpose；仅在数据库有新列、UI 有新按钮时打开功能会让旧服务节点绕过门禁。滚动部署需 feature gate + 节点版本一致性检查；关闭新建入口不能停掉已有 cancellation reconciliation。回滚到不认识共享目的的服务版本前，须阻止派发并保留数据/控制收尾能力。

#### 新建路径与存量转换需分开验收

首版是否只允许新建协作任务正在由主会话向用户确认，本研究不替用户决定。

- **仅新建**：来源 issue、协作标记/session 与初始参与关系须原子创建，并从第一次 assignee/status 事件开始阻止普通自动 enqueue。不能先创建普通 issue 触发一个旧 run，随后才加协作标记。普通评论历史不回放为 command；若创建时需要 AI 工作，必须有独立、明确的“发给 AI”意图和 command 身份。
- **允许存量转换**：转换前先持久冻结这张 issue 的所有旧 enqueue/claim/reclaim/start/自动 follow-up 路径，再清点已经派发、可能启动及实际运行的旧任务。不能只查看 ListActiveTasks 的当前 status，因为旧 cancelled/failed/completed 也未必有进程退出证据。转换提交前需要各旧执行的可信 quiesce 证据或受控 supervisor 的全量启动身份清点；取不到证据时不能激活共享新队列。
- 待执行的旧 coalesced 输入不能直接展开并声称恢复了原作者、原顺序和原意图。转换的保留/取消/重新提交方案及历史共享范围需要产品规则；不偷偷把过去所有普通评论变成显式 AI 命令。
- 转换自身用 request_id/版本/CAS 记录，崩溃后恢复同一次操作；冻结后未转换完成仍不能重新开放旧派发。转换成功也不能继续旧 completion reconcile 或新建并行 session。
- 旧服务节点不识别冻结/协作 purpose 时，任何“仅新 UI 允许转换”的措施都不够；需要部署兼容门禁。新建-only 可以减少存量迁移工作，但不免除所有旧执行入口对新协作 issue 的服务端拒绝。

### 11. 涉及模块和规范

| 位置 | 实施时需要核对的职责 |
| --- | --- |
| server/internal/service/task.go、task_cancellation.go、task_start.go | 受控 materialization/claim/start/complete/fail；共享分支不能自动重试、清 pending slot 或按旧 task 终态释放源会话。 |
| server/internal/service/issue_task_transaction.go、issue.go；handler/issue.go、task_lifecycle.go、comment.go | 统一 admission、旧派发入口拦截、评论不触发、整单结束/重开与队列原子性。 |
| server/internal/service/admin_operation.go、managed_runtime.go、runtime_teardown.go、platform_account.go | 借鉴持久控制和执行 fence；资源/账号撤权与最小收尾凭证；清理操作不得丢未结屏障或已形成历史。 |
| server/pkg/db/queries/agent.sql、task_token.sql、admin_operation.sql；server/migrations/ | 新命令/会话/执行关联查询与约束、旧 generic selectors 排除、并发索引、无新 FK。 |
| server/internal/handler/daemon.go、daemon_rpc.go；server/cmd/server/router.go；server/pkg/protocol/ | payload 分支、专用 token purpose、完整 HTTP/WS 一致性及能力门禁。 |
| server/internal/daemon/daemon.go、client.go、types.go、wsrpc.go、cancel_receipts.go、managed_transport.go | 本地启动账本、重连/重放、取消观察、窄回执、同 command 内部多次 provider 尝试与停止检查。 |
| server/pkg/agent/agent.go、codex.go、claude.go、其他 provider wrapper、proc_other.go、proc_windows.go | 明确 ProcessExited/进程树证据能力与不可证明时的拒绝策略。 |
| server/internal/issuestatus/；server/internal/iteration/ | 来源空间自定义结束类别与真实执行事实，保持现有目录/I1 事务契约。 |
| packages/core 的 API schema/types/queries；packages/views 的共享任务页面；Web/Desktop 平台接线 | 客户端同伴负责：显示 command/queue/control/proof 各状态、原 key 查询、明确新 Resume、升级门禁。 |
| server/internal/service/builtin_skills/ 相关 SKILL.md 与 source maps；CLI 命令 | 新旧命令行为、仅评论与发给 AI 的差异、不能绕过共享队列；更新文档与代码同批交付。 |

已读相关规范：CLAUDE.md（权限边界、无 FK、并发索引、真实 CLI 测试限制）；.trellis/workflow.md（当前只规划）；.trellis/spec/server/security-boundaries.md（真实发起人、事务/归因）；managed-installations.md（绑定、generation、回执）；triage.md（所有 writer admission）；iterations.md（借用事务与锁序）；platform-observability.md（证据、租约与审计）；.trellis/spec/guides/cross-layer-thinking-guide.md（载荷边界）。

既有任务研究：research/runtime-identity-and-usage.md、research/status-and-access-contracts.md。本文不覆盖完整账号/附件/工作台权限模型；来源与执行数据归属须与权限同伴最终设计一致。

### 12. 有意义的验证场景（计划，尚未执行）

测试应以真实 DB 两连接/事务 barrier 和 fake provider 进程观察为主，避免只测试一个状态转换 helper。

| 场景 | 必须证明的结果 |
| --- | --- |
| 两账号多客户端同时发命令 | 每个请求独立作者和 seq；同请求重发仅一个 command；同单最大 provider 活动数为 1，不同 issue 可并发。 |
| 普通评论、reply、@AI、旧客户端 | 不创建 command/task；source 普通成员只能评论；显式 AI 才入队。 |
| 合并/评论回放/旧 rerun/squad/autopilot/quick action | 所有后门被共享 purpose 门禁覆盖；不改旧 command 的作者或内容，不清除其他人的等待项。 |
| claim / finalize / start 与 Stop 双连接竞争 | 按第 7 节矩阵线性化；Stop 提交后不产生新的 launch 许可；已有许可保留屏障至结束。 |
| 取消广播马上触发 NotifyTaskFinished | 即使旧系统立即唤醒，下一条仍不能 claim。 |
| Stop 与 success/fail 及内部 provider retry | 同 run 终态不反转；停止后没有初始化/恢复 fallback 再 launch；每个尝试必须有退出证明。 |
| 停止时 daemon 断网 | 受理可见但未确认；Resume 被固定拒绝；收到退出证明后仍 paused。 |
| 重放此前拒绝的 Resume | 返回原拒绝，不在条件后来满足时执行；新 request_id 的明确恢复才可继续。 |
| 移除/退出作者 | waiting/retry_wait/未启动项取消并保留历史；其当前 run 停止；其他作者项保留。 |
| 当前资源 owner 退出但正在执行者为他人 | grant 原子撤回、会话暂停、当前 run 停止、只有 owner 的待执行 command 取消。 |
| grant 重新授权 / owner 重新加入 | 旧 grant/command 不复活，恢复仍需新显式操作；原 stop proof 不能指向新代次。 |
| 撤 grant、禁用 owner、换密码/解绑后回执 | 普通 token 已拒绝；窄 termination 凭证仍只能读 stop/回传同 run proof；访问内容/领取/start/恢复均拒绝。 |
| 伪造/过期/重复/错误 scope 的 proof | 不解屏障；缺字段/重复字段/not_observed/旧 task fence 不确认当前 run；同 receipt 重发返回同结果。 |
| provider result 通道提前关闭、退出超时、仅父进程退出、Windows Job 失败 | 不形成 exited_proven，不启动下一条；native 子进程/孙进程测试验证实际 containment。 |
| server 崩溃在 command/任务/operation/outbox 每个提交边界 | 事务要么全有要么全无；未知 commit 按原 key 对账；不产生重复 command/run。 |
| daemon 崩溃在 launch 前/后、proof 落盘前/后 | 原 launch nonce 不重复启动；有 durable proof 可重放，只有旧观察则 unconfirmed；孤儿回收不自动 retry。 |
| claim 响应丢失及 WS→HTTP fallback | 原 run 恢复，不重复 launch；过 90 秒只改变派发恢复资格，不证明旧进程结束。 |
| 资源切换 / 原环境删除 | 旧未结时拒绝；有效切换保留队列且 paused；旧 session/workdir 不注入新环境；源历史不被外空间删除。 |
| 完成/取消/自定义结束类别与入队/claim 竞争 | 所有未执行项取消、当前 run 收尾；所有入口拒绝新执行和恢复。 |
| 重开及管理员 R38 代管 | 提升业务 epoch，不复活旧命令；旧进程未确认时不能启动；管理员不能替代资源 grant 或 force-resume。 |
| runtime 能力缺失、旧服务节点、客户端降级 | 不掉回普通执行/另一会话；只阻塞本单，其他不受影响的任务继续。 |
| 新建协作单 / 存量转换 | 新建时没有先行旧自动 run；若允许转换，旧 writer 已冻结且所有可能存活进程有 quiesce 证明，旧合并输入不伪造为新命令，转换崩溃不恢复旧派发。 |
| 源/执行空间权限与留存 | source 专有内容不向 execution workspace 普通成员泄露；账号/资源退出或清理仍保留已接收的本单消息、用量和附件。 |

可以借鉴但不能冒充新功能验证的现有测试文件/案例：

- server/internal/service/admin_operation_test.go:74、171、303、332、467、516、658：取消原子性、proof、截止期/旧绑定、未知 commit、完成竞态、ABA 和旧 claim 拒绝。
- server/internal/daemon/cancel_receipts_test.go:57、176、214、273、309、367：先证明再落盘、重启重放、scope 隔离、不从关闭推断退出、认证失败及真实 runner 证据。
- server/internal/handler/managed_claim_recovery_test.go:12：已派发未启动丢响应恢复。
- server/internal/service/task_start_iteration_test.go、retry_deferred_test.go、duplicate_pending_task_test.go、rerun_workdir_reuse_test.go：现有 start、retry 和 pending 语义参考，需要补 shared-purpose 对照。
- server/pkg/agent/codex_cleanup_unix_test.go、claude_cancel_unix_test.go、proc_windows_test.go：进程树/取消 fake executable 测试参考；静态读取和交叉编译不等于各平台实际运行验收。

未来验证命令由实现计划列出；默认 Go 测试只使用测试创建的 fake executable。不得运行用户已安装的 AI CLI 或把带真实账户的 agentintegration 测试当本轮已授权行为。

### 13. 外部参考与版本

- 本轮未调用网络文档或远程服务，结论以仓库源码为依据，没有外部性能、供应商停止 API 或跨平台测试结果。
- server/go.mod:3 声明 Go 1.26.6；server/go.mod:18 使用 pgx/v5 v5.9.2。
- CLAUDE.md:108 的 CI 数据库基线为 PostgreSQL 17（pgvector/pgvector:pg17）。行锁/NOWAIT、事务隔离及并发索引建议针对该基线；没有据此假定当前运行实例已经验证。
- 新目的 token、launch ledger、source 持久投影、shared queue capability 均是待设计/实现的扩展，不是声称已有可调用 API。

## Caveats / Not Found

1. 尚无每业务 issue 的共享会话/逐条命令持久队列；现有 pending-task 合并与评论 reconcile 不能作为完成 R23/R24 的证据。
2. 当前停止回执不能普遍跨 user/binding 撤权继续上报；不补窄收尾协议就只能保持 unconfirmed。不能降低 R17 换取表面可恢复。
3. ProcessExited 未覆盖所有 provider，Claude 的直接 reap 和 Windows 当前降级路径也不足以直接宣称完整进程树停止。支持矩阵与拒绝启动策略必须进入首版验收。
4. 当前普通 orphan/stale retry 及 issue/rerun/评论触发需要逐 writer 接入门禁；只修改 UI 或 HTTP Stop 不够。purpose 没有完整排除旧 quick-create/重试/广播路径前不能开放功能。
5. 最终存储模型、令牌字段、外空间删除与源历史持久化策略由权限研究与 design.md 定稿。本文提出的名字不是 schema 定案，不应按文字直接造平行权限系统。
6. 真实产品待定点应单列，不能隐含在技术代码中：是否仅新建还是支持存量转换（主会话正在询问）、正常失败是否暂停队列、是否保留同 command 自动技术重试、重开后的首个明确恢复交互、完全无法取得旧主机退出证据时的恢复产品边界、账号禁用但未移除协作者时等待 command 的保留/取消策略。既有 R17 禁止不确认就启动，以上任何选择不能反过来绕过它。
7. 本轮未运行测试、编译、lint、数据库迁移或真实进程实验；本文件是有源码依据的研究与验证计划，不是实现完成或验收报告。
