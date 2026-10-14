# Research: 58 项需求的执行、队列与机器隔离一致性复审

- Query: 复核 R08–R11、R13、R15–R17、R23–R28、R31、R35、R40、R43–R45、R53、R57，以及新增编辑/提及/阶段规则与执行协议的交界；确认旧重试、合并和恢复入口不能绕过共享队列。
- Scope: internal；独立技术一致性复审，只修改本研究文件。
- Date: 2026-10-13
- 基线: 阅读本任务当前 prd.md（R01–R58）、acceptance.md（AC01–AC89）、design.md、implement.md、research/execution-contract.md、research/data-contract.md、research/design-review-queue.md，并定点核对当前服务端、SQL、daemon、provider launch 和协议源码。以下行号对应本次读取快照，后续主会话修订可能移动。

## Findings

当前队列的数据库权威、不可变 command、每单唯一未结 run、停止与真实终结分离、撤权后窄终结凭据方向一致。旧 Q01–Q05 已在设计层闭合，本次没有重新打开这些旧问题。新增 R43/R44/R45/R53/R57 也有明确的领域边界。

本次发现 **1 项必须补实的证明契约 EX01**；其余列为实施验证风险和接线清单，不把“尚未实现”误报为已存在的产品漏洞。无需新增产品选择或放宽 R17。

### EX01 — 必须修订：Unix 进程组不能单独证明完整执行树结束

**覆盖：** R09、R16、R17、R25、R26、R31、R35；AC08、AC21–24、AC37–40、AC48–50、AC57–58、AC70；P4/P5。

**设计证据：**

- `design.md:118–120` 要求 run seal 和完整进程树结束证明，明确 PID 缺失、断线和超时不能放行。
- `research/execution-contract.md:83–89` 要求 fork 窗口保守 unknown、全部 attempt 终结、形成 proof 前 seal run，且无可证 containment 不启动。
- 但 `implement.md:114` 将 Linux/macOS process group 与 Windows Job 并列作为 supervisor 的进程树能力；`design.md:143` 的“不启用脱管后台 launch”尚未定义可执行的准入/禁止机制。实现者可能直接复用现有 PGID 检查，把该条件误当成已满足。

**源码与实际反例：**

- `server/pkg/agent/proc_other.go:27–41` 只为直接子进程设置 `Setpgid=true` 并 `cmd.Start()`。
- `server/pkg/agent/proc_other.go:62–70` 以 `kill(-leader_pid, 0) == ESRCH` 返回整组已消失；`server/internal/daemon/processtree/controller_unix.go:54–75` 同样只观察原组。
- descendant 调用 `setsid()` 或改变进程组后，不再属于该 PGID。原组消失时它仍可能存活；这是普通 detached 工具进程即可产生的情形，不需要恶意宿主伪造证明。
- 本次在当前宿主运行了只用 Python `fork/setsid` 的有界最小反例。输出为 `{'original_group_gone': True, 'detached_descendant_alive': True}`。子进程设置 5 秒 alarm，检查结束后显式 SIGKILL 清理；没有调用真实 agent/provider CLI，也没有修改产品代码。该反例证明现有观测原语不足，不声称尚未实现的共享 supervisor 已发生错误放行。

**最小可实现修订：**

1. 在 E04 和 P4 明确：PGID 消失仅证明该进程组为空，不能独立签发 `exited_proven`。终结能力须绑定确实覆盖该 run 全部执行尝试及可派生子进程的封闭执行范围。
2. 能力注册按 OS/provider/启动模式给出可检查的条件。凡 provider 的工具进程可脱离已监督范围而没有强制限制或可证明的独立监督路径，不能宣告共享运行能力。仅在提示词中禁止后台、用 PID/PPID 列表轮询、未观察到进程、CLI 正常退出，都不能替代此条件。
3. 如当前平台无法提供所需保证，已有设计允许显示不支持/需升级并拒绝共享启动；对已经存在但失去完整观测的 run 保持 `unknown` 和屏障。不要为了双端可用性把 R17 降为 best effort。具体可用 containment 由 P4 实现时选择并举证，本研究不要求引入新依赖或修改普通任务的兼容行为。
4. Windows 的共享分支仍须在 Job 绑定成功前禁止 resume：`server/pkg/agent/proc_windows.go:95–131` 的普通任务逻辑明确 fail-open，不能原样复用；这点已由 D07/P4 明确，本次只补上与 Unix 对称的硬准入。

**补充测试落点：**

- `server/pkg/agent/` 与新 supervisor 原生测试：fake provider 派生孙进程后 `setsid`/改变 group、双 fork、提前关闭 stdio，直接 provider 退出后 descendant 仍存活。断言不会生成完整 proof，不会放行下一 command；如果系统明确禁止脱离，则断言启动失败/脱离失败。
- daemon 崩溃重启与 PID/PGID 重用：历史 run 不能把当前无该 PGID 当作退出证明，也不能误杀碰巧复用 PID 的无关进程。
- Windows 原生 Job 绑定失败、accounting 查询失败、重复 daemon/旧 outbox proof；mock `ProcessExited=true` 不算该矩阵证据。
- Server 对上述 unknown 保留屏障，Stop 后 Resume 返回 `stop_unconfirmed`，换 runtime 和 R40 重开不能绕过。

### 已闭合的旧 Q01–Q05：不需要重新要求用户决策

| 旧发现 | 本次复核依据 | 实施仍需证明 |
| --- | --- | --- |
| Q01 无许可占位无法结算 | `execution-contract.md:54–67` 明确 promote 即 in_flight；从未许可时撤派发后由 Server 结算，晚到 claim/finalize 拒绝 | promote 后 permit 前 Stop/remove/close；不等待不存在的 daemon |
| Q02 fork 窗口与迟到 fallback | `execution-contract.md:69–89` 明确跨进程锁、durable burn、单调 attempt、最终 seal；EX01 是对完整观测范围的补充 | burn/fork/PID/reap/seal 故障注入；一个 nonce 不能重用 |
| Q03 来源 admission 与安装 admission | `execution-contract.md:22–30`、`:36–52` 显式来源正式状态、当前资源、installation accepting 和逐 writer 锁表 | 空 engine issue_id 仍检查来源；stopped installation 不发新 permit；teardown 逆锁 NOWAIT |
| Q04 维护扫描与 Resume 竞争 | `execution-contract.md:106–116`：gate 共享锁至 active 提交、freeze/enable 都升 epoch | freeze 前的未送达 Resume 在 enable 后仍拒绝；恢复部署不自动 active |
| Q05 outcome/proof 乱序 | `execution-contract.md:91–104` 固定 reducer：结果先到留屏障、proof 先到无结果记 outcome_unconfirmed、forced disposition 优先 | late success 不覆盖停止；日志不完整不阻塞合法 proof、不扩大终结 token |

旧 `research/design-review-queue.md:81–87` 的“已关闭”仅为设计层结论；本次继续保持这个证据等级，不把它变成 AC 通过。

## 需求与验收覆盖

| 需求 | 主要 AC | 当前设计一致性与唯一主验证阶段 |
| --- | --- | --- |
| R08、R23 | 07、33、34、36 | D06 与数据契约每 session seq/唯一未结 run 支持 FIFO；不同作者不合并；P5 |
| R09、R10、R17 | 08–12、22–24、32、34 | Stop 先 paused，拒绝的 Resume 持久重放拒绝；proof 不自动 active；EX01 补强真实证明，P4/P5 |
| R11 | 11、14、26、30、40 | 普通转交/交回/提交验收不改 session、grant 或执行身份；P2，P5 联验 |
| R13 | 15、16、37、43、44 | owner 按 issue 授权完整资源组合；后加入合法参与者可用；author/accountable 不改成 owner；P4 |
| R15、R16 | 19–24、48、49 | 等待项取消与可能运行项 stop 区分，其他作者等待项留存；P5 |
| R24 | 35、36、60、72、74、78、79、86、88 | 只有人类显式 command 才能产生本单共享指令；普通编辑/提及/回应不派发、不改旧输入；P3/P5 |
| R25、R31 | 37、38、49、50 | grant 立即失效、可能运行 run 请求停止、owner 重加不恢复 grant/queue；P4/P5 |
| R26、R27 | 39–42 | Switch 要 paused、无未结 run、新 grant；waiting 不预绑旧资源，Git/明确附件无自动目录迁移；P4 |
| R28 | 43、44 | 所选环境供应商账号与平台 command author 分离；provider 只能拿窄业务 token；P4 |
| R35 | 57、58、70 | complete/cancel 原子 closed+取消 waiting+停止当前；reopen paused，旧 proof 屏障保留；P5/P7 |
| R40 | 69、70 | 双禁用 reopen+接手不复活账号/grant/command，不绕过未结 run；P7，P5 联验 |
| R43、R44 | 74、75 | 作者当前权限、评论墓碑/历史；移除展示不删除已交付文件或改变 command；P3 |
| R45 | 76 | 与 promote 同锁，只 waiting 可取消；资源失效/维护中仍可取消；不会自动升级 Stop；P5 |
| R53 | 84、87、88、89 | 机器默认最小引用、共享业务 token 是明确窄例外；事件/claim/prompt/插件需一致；P1/P3/P4 |
| R57 | 88 | 评论权限人类可 resolve/reaction；不改内容/command，不创建 R52 或旧 inbox；P3 |

表中的附带 AC 是交界覆盖，不改变 `implement.md:189–198` 既定主验证归属。

## 实施验证风险与最小接线清单

### EV01 — 评论变化必须在旧副作用之前分流

**不是新设计缺口。** `implement.md:93–100` 和 E08 已明确屏蔽 cancel/retrigger；实现必须覆盖以下实际入口，而非只给新评论端点换 UI。

- `server/internal/handler/comment.go:3332–3336`：旧客户端无 revision 时先取消任务再更新评论。
- 同文件 `:3378–3379`：严格 revision 分支在事务内调用 `CancelAgentTasksByTriggerComment`。
- 同文件 `:3521–3564`：删除先取消 task、硬删记录和对象存储，再重派 surviving comments；删除失败还可能重派完整输入。
- `server/pkg/db/queries/agent.sql:1990–2013`：普通评论 merge 会改 trigger、originator、accountable、connected-app overlay；`:2037–2080` 的 planned-input 注册会在 completion 时补派。这些均不适用于 R23/R24 的逐指令共享 FIFO。

**测试：** 同单分别处于 idle、active、paused、stop-unconfirmed、closed 时执行普通评论 create/reply/@AI/edit/delete/resolve/reaction。断言 command 数量、原文/作者/seq、run、queue_state/control_version 不因普通事件改变；评论自己的历史、墓碑、事件可以正常变化。直接覆盖旧 handler 的带/不带 revision 两种请求，以及 SQL/service 调用和删除失败回滚，不能只测试协作新 route。

### EV02 — 所有旧调度器必须显式识别 purpose

**已设计，待实现。** `design.md:17`、E08 和 P5 已要求 purpose 优先于形状推断。

- `server/pkg/db/queries/agent.sql:808–817` 的旧串行键是 `(issue, agent)`、chat 或全空链接 quick-create；`:883–902` 的 active 判断及 priority 排序不是 session FIFO。只增加 service 分支，仍让旧 SQL 领取共享 engine task，会破坏唯一会话和序号语义。
- `server/internal/handler/daemon.go:3071` 的 quick-create 判定依赖无 issue/chat/autopilot；共享 carrier 具有相同空链接形状，必须先 purpose 分流，包含构建失败与 finalize 路径。
- `server/internal/service/task.go:5360–5389`、`:5806–5828` 是失败重试/委派恢复；`:6240–6303` 是绕过普通 mention 的 recovery merge/enqueue。当前 `retryEligible` 对无 issue/chat 的某些形状可能本已拒绝，但不能把这种偶然形状当共享协议保护。
- `server/internal/handler/daemon.go:4172–4199` 在任务完成时重算评论触发，不能恢复 R24 普通评论的执行意图。

**测试：** 同一个带 purpose 的持久 run 逐个经过 enqueue/merge/planned registration/claim/start/finalize/fail/retry/rerun/orphan/reconcile/GC 入口。旧分支必须拒绝或投影到同一领域 primitive；不能新增 run/command、改 author 或绕过 paused。设一组普通 issue/chat/quick-create 控制样本，保留既有行为。

### EV03 — author、owner、执行工作空间三者不得为复用旧校验而合并

**已设计，待实现。** D03/D07、E01/E07 对 grant 关系、账号/参与 epoch 和业务凭据已有明确规定。

- `server/pkg/db/queries/agent.sql:819–827` 的 managed admission 依赖执行 workspace、runtime owner、密码版本及 membership；`:858–877` 的 private runtime 要求 agent/runtime owner 兼容。新跨空间功能不能把来源外部作者伪造成 runtime owner 来通过这些条件。
- `server/internal/handler/daemon.go:2391–2423` 目前同时组装 owner profile 和 initiator；共享 builder 要明确两种身份，而不是复用 owner 的“Requesting User”成为 author。
- `server/internal/handler/actor_guards.go:110–119` 未识别 actor 来源默认人类；R53 必须使用已规划的协作白名单，不能直接复用旧 `RequireHumanActor` 得出内容权限。

**测试：** 两空间三账号，owner 提供私有 runtime+合法 agent，另两人排队；执行 author/accountable 保留真实 command author，供应商账号保持 owner 环境。第三方私有 agent、不匹配 namespace、旧 binding、账号快速禁用后恢复、重新加入参与关系，都不能令旧 token/grant 复活。个人 PAT 与机器 token/未知来源按各自明确契约断言。

### EV04 — 终结通道不能接在会先拒绝已撤销身份的旧 RPC 外层

**已设计，待实现。** E02 termination proof 和 D07 的历史窄凭据足以闭合；需要保护实际 route 接线。

- `server/internal/handler/daemon_rpc.go:48–60` 在 method 分派前调用 `AuthorizeDaemonConnection`；简单添加 `collaboration.proof` case 会先依赖当前 daemon/runtime authority。
- `server/internal/daemon/cancel_receipts.go:228–247` 的现有取消收据依赖当前 managed/user/version/binding；`:301` 用进程内 attempts==exited 聚合，不能当持久 run seal。
- `server/pkg/protocol/messages.go:34–39` 当前 managed/admin-cancel capability 不能代替新共享 supervisor capability。

**测试：** owner 禁用、密码安全重置、解绑、runtime/engine task 删除后，业务 token、claim/launch/renew 都拒绝；历史 proof 凭据仍只允许 stop intent+该 run 的最小 proof。用真实 HTTP/WS 路由测试，不能只直接调用证明 service。若采用独立 proof HTTP 通道，明确旧普通 WS 失败不触发换用宽域 PAT/MDT。

### EV05 — teardown 与 account fanout 不得以旧 task 终态清空新屏障

**已设计，待实现。** D05、E02 已规定来源事实独立和 NOWAIT 交汇。

- `server/internal/service/runtime_teardown.go:62–83` 先持 runtime/agent 锁再取消 task；`:90–125` 依赖旧 nonterminal count 后解绑历史。新增 source/session 不能在持旧资源锁时阻塞倒序获取。
- `execution-contract.md:46–49` 要求账号 epoch 先失效并可靠 fanout，资源反向 source 锁 NOWAIT/整笔回滚，终结 proof 不要求这些当前资源仍存在。

**测试：** revoke/remove/owner leave/disable 与 promote/permit 并发；runtime teardown 与 source Stop/proof 并发；执行 workspace 被清理但来源单仍存在。断言没有部分撤权、孤立 active run、丢失终结验证快照或未结屏障被 task GC 清掉。有限重试用同步屏障，不用 sleep 证明事务顺序。

### EV06 — R53 还需覆盖 claim 内注入的插件工具，而不只审查事件桥

**已有 D07 默认拒绝原则，列为接线风险。**

- `server/internal/service/plugin_event_bridge.go:45–73` 只识别 payload `suppress_execution`，将事件原 payload 交 dispatcher；R53 要以来源协作事实识别，不信任缺失/遗漏字段。
- `server/internal/handler/daemon.go:2160–2176` 自动注入执行空间的 `PluginHookTools` 与 `AgentMCPConnections`。
- `server/internal/daemon/daemon.go:7052–7071` 以 `RemoteMCPDaemonToken` 为插件工具建立本地 MCP 通道。即使 provider 环境中看不到 MDT 明文，宽域 daemon broker 仍可能代它行权。

**最小实施约束：** 新 purpose builder 默认不继承普通 plugin/connected-app/remote-MCP 工具通道；若提供任何协作专用工具，其调用也必须进入 D07 的同一 run/issue/grant/epoch 允许表，不以执行空间资源 owner 权限代理。人类打开本单不授权任何插件读取全文。

**测试：** 订阅插件与 IM 只收最小引用或不收；共享 claim 不包含旧插件 hook/宽域连接凭据；机器借 browser bridge、daemon plugin hook、直接 CLI/API 或未知来源都不能读写本单内容。共享专用业务 token 的最小上下文读取作为正例。

### EV07 — 新 R58 的父子方向性不能被“所有自动触发一律禁用”抹掉

**与 R53/普通事件规则的交界说明，无新增产品选择。**

- `acceptance.md:96` / R58 要求协作单作为子项完成后仍按阶段唤醒其父任务。`implement.md:66` 已保留这一点。
- `server/internal/handler/issue_child_done.go:115–116` 当前跳过 member-assigned parent；共享 issue 的 handler 使用 member，因此普通子任务完成时当前不会唤醒共享父单。但应补显式协作类型守卫，不能长期依赖 assignee 字段的偶然形状。
- 相反，协作子单→普通 agent/squad 父单的 stage wake 是普通父单自己的执行；应保留 `issue_child_done.go:620–665` 的合法外部任务语义。不能因子项是协作单而全部禁用，也不能把它加入子单共享 queue。
- `issue_child_done.go:272–308` 会向普通父任务复制最小子项身份/标题/阶段摘要，随后 agent 自行读取子项时仍须服从 R53；不得扩展为复制协作正文、评论、附件或执行结果。最小引用中的链接不能带来机器读取权限。

**测试：** 普通 child→共享 parent 不产生共享 command/run；共享 child→普通 parent 的阶段满足后仅启动父任务自己的普通执行，shared session 不变；普通 parent 的 agent 读取共享 child 仍只得到最小引用。R58 的 stage 编辑本身不执行。

## Files Found / Code Patterns

| 文件 | 本次用途 |
| --- | --- |
| `prd.md`、`acceptance.md`、`design.md`、`implement.md` | 当前 58 项需求、89 项 AC、领域契约和分阶段验证归属 |
| `research/data-contract.md:12–26` | 永久 session、不可变 command、唯一未结 run、独立历史、无隐式重试 |
| `research/execution-contract.md` | E01–E08 当前准入、锁、许可、证明、冻结及旧入口清单 |
| `research/design-review-queue.md` | 旧 Q01–Q05 原发现及关闭记录，仅设计证据 |
| `server/pkg/db/queries/agent.sql` | 旧 claim/空链接串行键、评论 merge/re-attribution、planned-input/恢复 SQL |
| `server/internal/handler/comment.go` | 评论 edit/delete 的取消、硬删、重派和旧客户端分支 |
| `server/internal/service/task.go` | 自动失败重试、委派失败合并、completion 后恢复 |
| `server/internal/handler/daemon.go`、`daemon_rpc.go` | claim builder、空链接 fallback、plugin 注入、当前身份先于 RPC 分派 |
| `server/internal/daemon/cancel_receipts.go` | 旧 managed 收据身份、内存 attempt 计数及 proof 持久化接入 |
| `server/pkg/agent/proc_other.go`、`proc_windows.go` | Unix PGID 观测和 Windows Job fail-open 现状 |
| `server/internal/daemon/processtree/controller_unix.go`、`run.go` | 有界辅助进程同样只观察原 process group |
| `server/pkg/protocol/messages.go` | capability 常量、共享 HTTP/WS 协议接入点 |
| `server/internal/service/runtime_teardown.go` | 旧资源锁序、task cancel/解绑和历史清理 |
| `server/internal/service/plugin_event_bridge.go`、`server/internal/handler/actor_guards.go` | 出站原 payload 和未知 actor 默认人类的既有边界 |
| `server/internal/handler/issue_child_done.go` | R58 普通父任务唤醒与 R53 最小引用交界 |

## Related Specs

- `CLAUDE.md`：Go 检查、API 兼容、无需新依赖、无 FK/并发索引、测试层归属、真实 AI 隔离。
- `.trellis/workflow.md`：research 产物持久化；本次不修改任务状态或规划主文件。
- `.trellis/spec/server/index.md`：服务端规范索引。
- `.trellis/spec/server/security-boundaries.md`：凭据不能洗成个人登录；自动化授权 principal 与 actor 分离；借用同一事务、提交后事件；密码失效及资源边界。
- `.trellis/spec/server/managed-installations.md:48–75`：namespace/owner/绑定不降级与锁序；`:98–126` 区分 admission、delivery generation 和真实停止收据；`:140–146` 丢 claim 响应及原生 HTTP/WS 验证边界。
- `.trellis/spec/server/triage.md:86–96`：每一个 enqueue、merge、planned recovery、retry、claim/start 都查持久 admission；临时事件 flag 不能代替数据库 fence。

## External References

未访问网络、未引入第三方依赖。Unix 原语的边界使用当前仓库实现和本机 Python `os.fork/os.setsid/os.killpg` 反例核查；不是 provider 兼容认证。Windows 原生 Job 行为尚未在本机运行，不能从源码或 Unix 结果宣称 Windows 通过。

## Caveats / Not Found

- 没有实施任何产品功能，也未运行 DB migration、Go/TS 产品套件、真实 daemon/provider、Web/Desktop E2E。唯一执行验证是 EX01 的有界临时 Python 子进程反例。
- 没有修改其他研究、需求、设计、规范、代码或任务状态；未进行任何 git 操作。
- 未发现需要重新询问产品权限的情况。应先把 EX01 的可证明执行范围写实，再按 P4/P5 的原生与 DB 竞争测试验收；其他风险均可由实现和 review 按现有已确认规则解决。
- 行号是读取时快照。旧审阅中的 O11 未决说明属于历史，当前 R40 已确认，本次按当前 R40 审查。
- 此报告只负责分配的执行/队列/机器边界切片；完整 58 项的授权、客户端和全局一致性结论由主会话汇总其他独立报告。
