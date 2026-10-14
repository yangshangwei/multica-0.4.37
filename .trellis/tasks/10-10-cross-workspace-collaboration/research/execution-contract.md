# 共享执行的准入、启动和收尾契约

- 日期：2026-10-12；[design.md](../design.md) D06–D08、D11 的技术补充，仍属规划。
- 解决静态审阅 Q01–Q05 的具体协议问题；不替代产品需求或实际并发/原生测试。

## E01. 唯一准入谓词

拟新增 `CanAdmitCollaborationRun(tx, identities, expected)`；不是只看 session.active，也不依赖 engine task 的空 issue_id 触发旧检查。

| 边界 | 必须成立的当前事实 |
| --- | --- |
| 人类保存 command | 真实有效参与者、来源单有效非终态、参与及账号代次有效；无资源或维护期间可以保存 waiting，不能派发 |
| R45 取消 waiting | 当前有效参与者且为原作者/creator/handler，同单 command 仍 waiting、版本匹配；无需 grant、在线 runtime 或开放执行 gate |
| promote/claim/finalize/首次 permit | 下列完整谓词，且最早有效 command/唯一未结 run 与请求身份匹配 |
| 内部下一次 provider 启动许可 | 同 run 未 sealed、前一次 attempt 已证明结束、完整谓词仍成立，禁止复用上次 attempt 的许可 |
| Resume | 身份/资源/来源合法、全局 gate 开放且 expected_execution_epoch 匹配、无未结 run；持 gate FOR SHARE 至 active 提交 |
| Switch | 发起人或 handler、paused、无未结 run、新 grant 关系合法；不生成 permit，不恢复队列 |
| 正常业务 token 读/写 | design D07 的动态能力谓词；机器不能降级使用人类 read basis |
| Stop/强制失效/close | 不要求执行准入开放或 grant 仍有效；分别验证人类控制权或可信系统失效原因 |
| 终结 proof | 只依固定历史 run/installation/凭据及本地证明身份，不要求当前账号、grant、runtime 仍有效 |

完整新执行谓词：

1. 来源 issue 及协作关系存在，`admission_status` 为 `not_required|accepted`，来源类别可识别且非 done/cancelled。新建初始化为正式 `not_required`，旧 triage 不得改变协作 admission。`blocked` 是正式业务类别，不等于 triage 未接纳。
2. 当前协作账号代次、有效作者/参与 epoch、owner、grant、lifecycle/resource epoch 都匹配。账号禁用后恢复、撤权后重加不能使旧身份重新有效。
3. grant 的 agent/runtime 归属 execution workspace，agent.runtime_id 匹配，资源未归档/解绑。授权人是 runtime owner 且具备选定 agent 的既有调用资格；私有 runtime/agent 原有 owner 一致约束不放宽。public 组合也必须通过既有 invoke 与 namespace 规则，不能借 runtime grant 使用他人的私有 agent。
4. managed 分支当前 installation、binding、owner、workspace、runtime、binding epoch 匹配，installation admission=`accepting`。非 managed 分支必须完成 design D07 的协作能力注册，具有当前有效的稳定安装/绑定身份；NULL binding 不能表示准入通过。
5. 运行端在线/健康、协议/provider/OS 能力合格、容量允许，部署的持久执行 gate 开放；session.queue_state=`active`。首次 permit 还验证 claim generation、run fence、launch/attempt nonce。

installation 停止或全局 gate 冻结后，可以继续观测和终结以前已许可的 run，不能发新 permit。旧“恢复已接纳运行”的例外不是新 provider 启动许可。上述事实在来源与执行资源的事务锁内重读；仅加锁不等于执行了谓词。

## E02. 锁入口和交汇点

以下 `*Collaboration*InTx` 为拟新增借用事务 primitive。既有函数名称只表示接入位置，不能嵌套调用会重复获取前置锁的 pool wrapper。所有 ID 预读仅作 hint，锁后重验，变化则完整回滚。

| writer | 前置锁/实际接入 | 来源聚合与后置资源 |
| --- | --- | --- |
| 创建/精确目标确认 | 人类认证/账号 fence；来源 `workspace` KEY SHARE；创建操作唯一键；私有 draft 行 | 新 issue/session/participant 同事务；创建先占创建操作，重复 key 不再创建。与 `LockWorkspaceForDelete` 互斥 |
| 人类 edit/invite/transfer/remove/leave/close/reopen | actor/目标等相关账号 fence；原有来源目录/I1 前置 fence；workspace 存在性锁 | issue→session→participant/grant→command/run；R41/R42/R47–R50 重验 handler/版本/来源选项或 status_options_version（R49/R50 另验来源成员，R50 走迭代 membership 借用事务；R51 普通子任务写 parent 时协作单只作引用，不进入本表锁链），不改 command；R46 重验 R12 角色与目标后直接写参与/handler，无待接受态；后置 engine task/runtime 锁 NOWAIT |
| R43 评论 edit/delete | actor 账号 fence、workspace 存在性与当前访问依据 | issue→participant（如适用）→comment；重验真实作者/权限/版本，历史/墓碑/outbox 同事务，不取执行资源锁 |
| R57 resolve/unresolve/reaction | actor 账号 fence、workspace 存在性与当前访问依据 | 与 R43 共用稳定 issue 锁，锁后验同单 comment/真实 actor，再清旧解决+设新解决；回应只改本人记录。不同目标并发解决也串行，不通知、不执行 |
| R45 取消 waiting command | actor 账号 fence、workspace 存在性 | issue→session→participant→command；与 promote 串行，重验当前角色/状态/版本，cancelled 与事件同事务；不取 grant/runtime/gate 锁或更改控制版本 |
| promote/claim/finalize/permit | 原 managed user→namespace/installation admission，或协作 enrollment 对应锁；全局 gate FOR SHARE；source/execution workspace 按 UUID 排序 | source admission fence、issue→session；agent 容量/运行资源及 task 后置锁 NOWAIT。不得假定空 task.issue_id 已经过 `lock_issue_execution` |
| Resume/任何 session.active 写入 | 相同身份/资源前置协议、全局 gate FOR SHARE（含 epoch 校验） | source/session 锁内重验，gate 锁持有至 active 提交；不能用无锁预读 gate |
| grant consent/revoke/switch | owner/actor 账号、namespace/installation；两个 workspace 按 UUID 排序 | issue→session→grant/participant；agent/runtime 当前关系重验，后置资源 NOWAIT；网络准备在事务外 |
| R38/R40 admin | `LockPlatformAdminMutation` advisory；admin/creator/handler/所选 successor 去重有序用户锁，组织/来源范围再验 | 来源目录/I1→issue→session→participant；重读双禁用、目标有效及 revision，重开/责任/审计/outbox 原子写入；后置资源 NOWAIT，不放宽 runtime 权限 |
| account 禁用/credential 全撤 | 原账号治理锁；同事务推进中立账号 epoch 和持久失效事件 | 不持用户锁等多个源 session；分批按 issue UUID NOWAIT 收敛。epoch 已使全入口拒旧能力，持久 fanout 不可丢 |
| runtime/agent/execution-space teardown | 在原 resource 锁前枚举源 session；或保留原顺序但对每个反向 source 锁使用 NOWAIT | 必须整笔回滚重试，不能持 resource 等待 source；撤 grant/stop/source barrier 保全后才执行旧 engine 清理 |
| source workspace delete | 既有 `LockWorkspaceForDelete` | 清理前查全部协作记录；存在则拒绝，不继续获取清理锁 |
| termination proof | 历史 token hash/固定安装身份，禁止调用当前 `LockManagedRuntime` 或人类权限 wrapper | 只锁 source issue/session/run 和 proof/operation；不要求 engine task/user/runtime 行尚存，optional engine 更新用 NOWAIT 或提交后幂等投影 |
| 部署 freeze/enable | 独占全局 gate 行，单独短事务，不持 gate 独占锁扫描全部 session | 关闭 gate 后逐源 session 暂停；见 E06，不能反向同时等待 identity/resource 锁 |

跨多个同类对象按稳定 UUID 顺序。凡与旧 `lock_task_owner_rows` 的 workspace→agent→issue→runtime 或 task→issue 阻塞次序交叉，新路径的后置对象使用 NOWAIT，不新增反向等待。锁冲突/40P01/40001 整个事务回滚，最多三次有界抖动重试；不重做外部准备副作用，不在未知 commit 后换 request ID。实现必须用真实并发测试证明每个 writer 接入了该表，不以本文替代证明。

## E03. promote 后但从未许可的 run

promote 成功即把队首 command 从 waiting 改为 in_flight，原子创建唯一 run/engine 关联。其后没有 launch permit 仍是当前 command，不会退回队列或新建另一个 run。

R45 仅处理 promote 前的 waiting；取消先胜则该条不再参与调度，promote 先胜则取消返回 `command_not_waiting`。即使还未获得许可也不得将 R45 请求暗中升级为本节 Stop；需要新的人类停止意图。

| 事件发生在“从未发过 permit”时 | 同一源事务的结果 |
| --- | --- |
| 用户 Stop/资源撤回 | 撤销所有派发/claim 身份，写 never_launched_proven，当前 command=stopped，run settled，清该屏障；其余 waiting 保留，session paused |
| 作者移除/退出、整单 close | 同样证明未启动，command=cancelled，保留相应原因；按动作暂停/关闭，其他作者是否取消依 R15/R35 |
| 确定性的永久 prepare/构建错误 | command=failed，记录错误，证明从未许可后结算；资源失效/能力不符另使 session paused，正常已结束失败可让 active FIFO 继续 |
| 暂时投递失败/响应不明 | 保留同一 run/nonce/claim 身份查询或重放，不产生新 command；仅在数据库确定没有 permit 时才能走未启动结算 |

该快捷结算不等待一个从未领取任务的 daemon。任何已发许可或许可提交结果不明都不能使用它；必须等待 E04 的永不启动/退出证明。延迟 claim/finalize/start 会因 run settled、代次失效而拒绝。

## E04. 不可重用的启动账本

supervisor 使用跨 daemon 进程的 OS 互斥锁和私有、原子写+fsync 的本地账本。仅进程内 mutex 不够。账本按固定 deployment/installation/run/fence/launch nonce 标识，状态单向推进：

```text
registered
  -> attempt_nonce_burned (durable; may have forked)
  -> process_identity_recorded
  -> attempt_reaped
  -> run_sealed
  -> terminal_proof_persisted
  -> proof_acknowledged
```

在同一 launch 锁内，先持久 burn 本次 attempt nonce，再允许 fork；重复请求只恢复原观测，不重用 nonce。burn 后、fork 前、fork 后至 PID/containment 回写之间崩溃都保守视为可能存活。没有 PID 或查不到进程不能据此再 fork；若无法从原 containment 证明收尾，则 unknown，保持服务器屏障。

内部 fallback 也分配单调 attempt_seq/独立 nonce；必须上报前次 attempt 已退出并取得同 run 的下一许可，完整重验 E01。它不是另一个 command，原 run 屏障始终保留。无法服从该协议的 wrapper 不声明共享执行能力。

形成任何最终 proof 或 never_started 墓碑前，在同一个跨进程锁内先永久 seal 整个 run，禁止所有后续内部启动，再把终结记录可靠落盘并发送。proof 携带最后授权/观察 attempt 边界，服务器核对全部许可；某一个 attempt 的 reap 不能证明整个 run sealed。never_started 要有已 seal 的账本，不能用未观察到运行替代。

Windows Job/Unix containment 建立或确认失败不能声明 proof；无可证的范围不启动共享执行。daemon 重启、另一实例和旧 outbox 重放仍遵守同一个 seal/nonce，不能因切换 HTTP/WS 或新 boot 绕过。

EX01 修订：Unix PGID 消失仅证明原进程组为空，不能证明 setsid、改变进程组或双 fork 后代已结束。共享 capability 必须绑定能强制覆盖全部派生进程的封闭执行范围，并对 OS/provider/启动模式举证；PID/PPID 轮询、提示词禁止后台、直接子进程退出均不足。无法保证时不得宣告共享执行能力或发放许可；已运行后完整观测丢失则保留 unknown/屏障。原生负例须覆盖 detached descendant、提前关闭 stdio、daemon 崩溃及 PID/PGID 重用，不以旧 ProcessExited 或进程组空替代 proof。

## E05. outcome 与 proof 的归并

run 分开存 `business_outcome`（success/failure 及固定结果引用）、`forced_disposition`（停止/未启动取消及原因）、`proof_state`、`sealed_attempt_seq`、`recording_incomplete`。业务失败可先显示，但 settled_at/屏障只由合法终结证明推进。

| 到达顺序 | 确定结果 |
| --- | --- |
| outcome 先到，proof 未到 | 保存/显示业务结果，command 仍 in_flight、屏障保留；不启动下一条 |
| outcome 后有合法 proof | 无强制处置则按已记录 outcome 结算 succeeded/failed；有强制处置优先结算 stopped/cancelled |
| proof 先到，outcome 丢失或已被撤权拒绝 | 以 failed、reason=`outcome_unconfirmed` 结算并清合法屏障，独立标记记录不完整；不把 exit code=0 自动算业务成功，不无限等正文 |
| Stop/close/revoke 先于 proof | forced disposition 优先；late success 不能覆盖。后续终结动作附加原因，不反转首个有效终结根 |
| 正常 proof 已结算，随后 Stop | 暂停会话；不把已成功 command 改写 stopped。若后来已有新 run，旧 fence 请求按 D06 拒绝 |
| 重复/乱序回传 | 相同 event/receipt key+digest 幂等；不同 digest 拒绝；已结算 outcome 不被后来的内容改写 |

最小 proof 仅含固定身份、sealed attempt 边界、未启动/已退出枚举和有限退出信息，不包含正文/附件/路径。合法 proof 不等待日志全部上传；其到达后 run 业务能力关闭，未入库尾部保持不完整。active 且其余门禁有效才可推进下一条，paused/closed/maintenance 均不自动解除。

## E06. 部署冻结与恢复

新增每部署一行持久 `collaboration_execution_gate`：enabled、epoch、freeze_operation_id、freeze_progress。它与创建开关分开，所有 writer 使用同一行。promote/claim/finalize/permit、Resume 及任何 session.active 写入均以 FOR SHARE 持有 gate 到事务提交；冻结用 FOR UPDATE 改 false 并提升 epoch。冻结提交后拒新 permit，之前已许可 run 按停止流程处理。

会话控制快照返回 execution_gate_epoch，Resume 必传 expected_execution_epoch 并计入幂等 payload。冻结及重新 enable 都推进 epoch；冻结前意图即使延迟到重新 enable 才首次到达，也返回 stale_admission，不恢复 session。旧 key 重放只有原收据，不能再写 active。扫描按普通控制协议提升修改会话的 control_version，旧已暂停会话的意图由 gate epoch 同样失效。

冻结顺序：先关闭执行 gate，再按稳定顺序持久暂停所有未关闭 session、停止当前 run，记录扫描进度。Resume/promote/claim/finalize/permit 在 gate 关闭时拒绝；评论、持久 waiting 输入、正常人类流转、Stop/proof/历史/审计继续可用。界面显示维护阻止执行，不能将扫描前的 active 标志显示为已恢复。

解除冻结前验证暂停扫描完成、没有遗漏的 active session，并确认全部 writer 是兼容版本；新的 session/reopen 本就 paused。enable 只开放新的显式 Resume 及许可校验，不自动写任何 session.active。扫描失败可继续原 freeze operation，不靠隐藏入口宣称已冻结。关闭创建开关本身不影响证明接收或已存在单据的保护。

已存在共享数据后禁止回退不识别 purpose/gate 的二进制；部署必须停止所有旧 writer 后启用新创建。前述 gate 不能阻止一个完全不实现协议的旧二进制，所以兼容版本控制是独立的发布前置条件。

## E07. 认证与运行身份兼容

现有 User 没有通用 auth epoch；password 的 session_version 位于密码凭据。无 managed 历史的 daemon 可以合法返回空 binding，但这不是共享运行的安装证明。不得用 NULL/0、daemon_id 或缺失字段的默认值假装完成身份校验。

新增 `collaboration_account_fence(user_id, epoch)` 作为中立协作失效代次；用户首次参与/授权时原子建立。账号禁用、全局认证撤销、密码安全重置以及身份迁移的写事务同时推进它并记录失效事件，具体写入点在 P1/P4 登记；password 分支还比较真实 session_version。普通浏览器刷新、JWT 正常到期或一次登录/退出不自动撤销持久派单意图。单个安装解绑则推进安装代次并撤相关 grant，不把所有账号派单一并取消。

| 认证/运行模式 | 人类入口与当前账号检查 | 共享执行身份 |
| --- | --- | --- |
| password + managed runtime | 既有完整人类认证、有效密码会话/账号及中立 epoch；R38 仍限完整管理会话 | 复用现有已验证 installation/binding/namespace 和真实 credential/binding 版本，补 supervisor 能力 |
| password + legacy runtime | 维持现有登录；原 password-only managed enrollment 规则不放宽 | 可走既有合法 managed 接入，或下面的独立协作 enrollment；未完成时拒绝共享运行 |
| legacy JWT/PAT 人类 + legacy runtime | 既有身份验证、可信实际 user、有效账号和中立 epoch；agent actor 不能冒充确认 | 新的协作 capability enrollment 提供稳定安装/绑定身份，不调用 password-only 管理注册作为兼容捷径 |
| 任何人类模式 + 不兼容/未注册 daemon | 人类仍可查看、评论、流转和保存 waiting | 明确需升级/注册，不派发、不补宽 token |

独立 `collaboration_runtime_enrollment` 记录 deployment、owner、execution workspace、runtime、installation UUID、public key、binding epoch、revoked_at。注册由当前资源 owner 的现有人类凭据发起，只针对其可管理资源；daemon 用本机私有持久密钥完成服务器一次性 challenge，challenge 绑定这些 ID、key 和到期时间。成功后固定安装/launch 身份和撤销代次，再允许按 issue 授权。重注册、换 key 或绑定迁移推进 epoch、撤旧 grant；不能在已未结 run 上替换其历史 proof 身份。

这不是新登录类型，也不给 legacy 用户平台管理员权限。新路由与原 password-only `installation` 管理入口隔离，不向 provider 提供 enrollment 认证凭据。managed 与协作 enrollment 用显式 tagged union，分别验证真实版本，绝不把两者拼成一组可选空字段。终结记录保存注册时已验证的最小历史身份，在当前 enrollment 被撤销/清理后仍能按本 run 的窄用途验证。

grant 统一调用 E01 的资源关系谓词，授权、绑定、更换、领取及业务 token 请求全部重验；agent/runtime owner、archive、rebind 或 namespace 变化使旧组合失效并进入暂停/停止收敛。对于公有 agent，owner 对资源的既有 invoke 权限是授权前提；任何涉及私有资源的额外 owner 约束均原样保留。

## E08. 旧入口适配清单

| 入口 | 协作契约 |
| --- | --- |
| issue PATCH/batch/move/delete、状态快捷操作 | R41/R42 字段及其他已确认动作共用领域服务；其余拒绝，不借混合 PATCH 改身份/状态/来源定义或触发执行 |
| comment 创建/编辑/删除/resolve、mention/reply、preview | R43 普通评论 edit/delete 保历史/墓碑，不取消/重派/修改 command；R44 移除展示不删已交付附件，未授权拒绝；R57 resolve/unresolve/表情回应对有评论权限的人类开放，旧回应通知（`server/cmd/server/notification_listeners.go:912`、`:947`）跳过协作单；R55 提及只解析当前参与者，旧 mention/@all/@squad 监听跳过协作单；R56 评论子任务 preview/create（`server/cmd/server/router.go:2283–2284`，manual/agent）对协作单拒绝且无 source-context 副作用 |
| rerun/quick-action、status/assignee 自动触发、squad/autopilot、triage | 不生成旁路执行；不得借显式 command 名义让机器自行派新指令 |
| enqueue/claim/start/finalize/retry/orphan/GC | purpose 分流；generic selectors 排除协作执行，专用分支完整 E01 校验 |
| task 完成/失败/cancel、后台状态 writer | 记录本 run outcome/proof，不直接放行队列或代替人验收 |
| Plugin、CLI、browser bridge、HTTP/WS daemon RPC | 同一动作/用途契约；旧 route 不保留绕过权，包括正文 PATCH/附件删除。R53：非本单共享会话的机器读取只得最小引用；出站 plugin 事件桥（`server/internal/service/plugin_event_bridge.go:54`）与 IM inbox 推送对协作单不带正文/评论/附件。共享 claim 默认不继承普通 PluginHookTools/connected-app/remote-MCP 宽域 broker；协作专用工具也须逐调用验证本 run/issue/grant/epoch 允许表 |
| 全 agent 停止、运维取消、runtime/workspace teardown | 停止资源同时持久暂停/终结屏障，不伪造整单结束或退出证明 |

资源清理后的来源 transcript/附件/run/proof 不依赖 engine task 存活。无支持意图的旧入口应明确拒绝，而不是猜一条新的 AI 指令。普通任务继续既有行为。

## 验证对应

- Q01：promote 后、permit 前 Stop/撤权/close，无 daemon 也能安全结算；迟到领取拒绝。
- Q02：burn/fork/PID 回写/最后 reap/seal 前后逐点崩溃，另一 daemon/重复派发/迟到 fallback 不重复 fork。
- Q03：source admission 非正式、installation stopped、resource rebind 与 permit/teardown/proof 竞争；旧恢复路径不能新发许可。
- Q04：用 barrier 卡住 Resume 的 gate 读取到 active 提交窗口，同时发 freeze；验证共享锁阻止越过扫描。冻结前未送达意图在重新 enable 后因旧 epoch 拒绝；重复 key 和解除 gate 不自动恢复。
- Q05：proof/outcome 乱序、丢失、重复、撤权夹在两者之间；失败已上报但 proof 永不到时保持屏障。
- A04/A05：password/legacy 人类与 managed/legacy runtime 组合、缺失 epoch/installation、challenge 重放、换 key/rebind、私有 owner 不一致；只认当前合法资源组合。

这些均列入[实施计划](../implement.md) P4/P5/P8，以数据库事务同步和 fake 原生子进程验证。本文没有运行这些测试。
