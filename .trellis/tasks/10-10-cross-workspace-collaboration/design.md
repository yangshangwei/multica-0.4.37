# 跨账号协作技术设计

- 日期：2026-10-14；状态：技术复审已收口，P1/P2/P4 分阶段实现中。
- 需求依据：[PRD](prd.md) 与[验收附表](acceptance.md)；执行顺序见[实施计划](implement.md)。
- 下列名称为拟实施契约。源码索引见[研究摘要](research/design-contracts.md)，准入/锁/启动/收尾细节见[执行协议](research/execution-contract.md) E01–E08。

## D01. 唯一任务与两种空间

保留来源工作空间的一条 `issue`，不复制影子任务，不将外部协作者补成整个空间的成员。`issue_collaboration` 的存在是启用协作契约的权威事实；API 的协作标记由此派生，客户端不能通过 PATCH 标记转换普通任务。

完整结构图见[研究索引](research/design-contracts.md#协作结构图)。

明确区分 `source_workspace_id`（业务、状态、内容归属）和 `execution_workspace_id`（agent、runtime、installation 归属）。来源 issue 的 UUID、编号、creator 和 workspace 不随转交、切换环境、结束或重开改变。

当前处理人复用 `issue.assignee_type='member'` 和 `assignee_id=user.id`。现有字段实际存全局用户 UUID，不能改用 `member.id`。协作分支验证有效账号及本单参与关系；普通任务仍执行原空间成员规则。AI 资源绑定放在 session/grant，不再通过把协作 issue 指派给 agent 启动执行。

采用显式协作执行关联：engine task 使用独立 `purpose=collaboration`，普通 `issue_id/chat_session_id` 留空，由 run 关联唯一来源 issue。解析 purpose 必须先于“无 issue/chat 则 quick-create”的旧回退。engine task 的 `context` 只存绑定 ID 和非敏感调度字段，不存来源正文。来源 transcript、结果、用量和附件独立持久化，不能仅引用会被执行空间清理的 task/message 行。

首版只由已认证人类通过明确入口新建。原子建立 issue、协作记录、发起人/处理人参与关系、session；默认处理人为创建人，选定其他账号时按 R46 同事务加入并派单，无待接受阶段。创建不触发普通任务自动执行；AI 输入只能来自后续显式 command。首版不开放协作单硬删除、跨空间移动、合并及存量转换；旧入口返回明确不支持，防止抹掉身份或 R37 的保留依据。

## D02. 持久数据与约束

完整字段、身份版本、唯一性、迁移和索引规则见[数据契约](research/data-contract.md)。全局 user UUID、协作版本字符串与旧 Issue 数字版本分离；来源单据和唯一共享队列仍为权威。

## D03. 授权与身份

新增事务可复用的 `ResolveIssueAccess`，明确读取两个独立依据：有效来源成员或有效本单参与者。`loadIssueForUser` 只负责解析，不能被误当成跨空间授权器。所有关联任务、文件、评论、run 必须再次核对确属本单。

| 动作 | 允许主体及额外条件 |
| --- | --- |
| 读取本单、历史、已记录执行及附件；普通评论 | 当前有效人类账号且来源成员或本单参与者；R53 的机器身份只得最小引用、写入拒绝 |
| 发 AI 指令、停止、明确恢复 | 当前有效参与者；提交/恢复/开始各自检查 D06 门禁。停止不要求 grant 仍有效 |
| 取消单条等待指令 | R45：当前有效参与者且为该指令作者、发起人或当前处理人；只接受 waiting |
| 邀请、普通转交、更换环境 | 发起人或当前处理人；目标/资源在写事务中重验 |
| 标题/正文、优先级/标签/属性值、开始/截止日期编辑；非终态状态切换 | 有效发起人或当前处理人；R41/R42/R48/R47/D08，留修改记录 |
| 看板列内排序（`position`） | R58：有看板编辑权的来源成员，沿用普通排序写入；不推进协作版本、不留记录、不通知 |
| 项目归属、父任务、迭代归属、父任务下阶段（`stage`） | R49/R50/R58：有效发起人或当前处理人且仍是来源成员；沿用来源引用校验及迭代 membership 协议，其他人只得最小引用 |
| 把来源普通任务挂到本单下/移出 | R51：来源成员按普通任务既有规则写子任务的 parent；不改本单协作状态，外部协作者只得最小子任务引用 |
| 解决/取消解决线程，添加/移除本人表情回应 | R57：当前有本单评论权限的人类；不改评论内容、AI 指令或执行，不进 R52 通知 |
| 编辑/删除人类普通评论 | R43：真实作者且仍有本单评论权限；不可操作 AI/系统记录 |
| 移除别人 | 发起人；不能移除自己；移除当前处理人同事务交回发起人 |
| 主动退出 | 非发起人、非当前处理人的本人 |
| 补充完成并交回、提交验收并转交 | 承担当前责任的人；请求/接收人还须有效 |
| 完成、取消、重开 | 普通入口仅发起人；管理员只走 D05 的独立代管入口 |
| 授予/撤回资源 | 真实资源 owner，且授权时已合法加入本单；不顺带取得处理责任 |

R36（2026-10-13 已确认）：发起人退出来源空间后，账号及本单参与关系仍有效时，保留完成、取消、重开权限；这些动作不额外要求来源成员身份。来源空间权限随成员身份失效，R49/R50/R58 的结构字段仍拒绝写入。退出来源空间不等于退出本单，也不构成 D05 的账号禁用代管条件。

来源 workspace revoke 只清该空间的成员/资源/来源订阅，不因此推进全账号协作 epoch、撤销 issue_participant 或清 creator/handler；若同时导致运行资源失效，仅走对应 grant 撤回与暂停。R53 的最小引用必须在查询匹配/计数/排序前执行：机器搜索协作单只能匹配允许的最小引用字段，不能利用正文/评论全文检索、snippet、命中数或高亮探测私有内容。HTTP/WS 与导出/搜索复用服务端身份投影。

人类确认入口使用可信实际登录主体，并拒绝任务凭据和 legacy agent actor 冒充。R53 的内容读取同样按白名单判人类：仅服务端标记的登录会话与个人 PAT；`task_token`、`cloud_pat`、plugin、IM 及未分类来源一律按机器，不沿用 `isMachineCredentialActor`（`server/internal/handler/actor_guards.go:110`）“未知视为人类”的默认。空间 admin/owner、资源 owner、历史作者都不是通用管理例外。退出后仍为来源成员则重新获得基础读/评论投影，不保留旧协作控制能力；两种依据都失效则详情、附件、实时和回执内容一致拒绝。

编辑按 R41/R42/R48/D08，值和前后值、操作者、时间及事件同事务保存，不改 command 或派发；R48 日期解析/清空沿用普通任务。R42 仅投影本单所需来源选项，写时重验定义，不开放配置管理。R43 保留评论 UUID、版本历史及删除墓碑，回复锚点不变；锁内重验作者/评论权限/版本，删除后不接受编辑，重试沿用 D08。旧 API/Plugin/空间 admin 无旁路；resolve/unresolve 与表情回应按 R57 记录真实操作者、保持每线程单一解决记录，旧回应通知对协作单跳过；已交付附件按 R44 留存，私有未绑定 draft 仅本人可清理。

评论持久版本/墓碑、collaboration_event 编辑历史及附件永久归属/多引用按数据契约实施。R57 解决/取消解决与评论编辑/删除共用 issue 锁，锁后重验权限再清旧解决并写目标；仅两条 UPDATE 放在事务里不能防止两个不同目标并发成为已解决。普通评论/回应复用现有表，协作分支禁止硬删除及自动触发。

执行记录保持 `originator_user_id=accountable_user_id=command.author_user_id`；另记不可变 creator、当前 handler、资源 owner 和供应商账号来源，不能为了通过私有 agent 校验将作者改成 owner。用量归属描述执行资源账号，不引入跨人自动结算或凭据共享。

精确查找使用 `kind=username|email|user_id` 加完整值及本单/合法创建上下文。复用账号归一化、UUID parser 和限流，最多返回一个供确认的账号摘要。无空值目录、姓名模糊查询或跨 Server 查找；查找没有写副作用，确认邀请/转交时重新校验。历史邮箱归一化若出现多匹配则拒绝歧义，不任选账号。

R46 在邀请/派单事务内重验当前 R12 角色和目标有效性，写参与关系、历史/outbox；派单另写 handler/待办。仅邀请只进入参与列表，离线或通知未投递不延迟生效；无需接收方的接受路由或待接受状态。服务端提交前不得乐观转出责任，幂等重放不重复加入/交接；原有资源 grant 不因新成员加入而切换，也不自动取得新成员的私人资源。

## D04. 人类流转与来源状态

来源状态目录是唯一解释依据；不用查看者当前空间的同名 key。详情只给本单状态及当前动作可用选项，不借访问本单开放整个状态配置目录。目标 key 必须存在、未归档且类别匹配；未知类别拒绝控制/生命周期写入，不使用显示层的 todo 兜底。

| 动作 | 状态映射 | 责任/交接效果 |
| --- | --- | --- |
| 新建 | 来源 todo 为默认，可显式选择有效非终态 | creator 不变，handler 为明确目标 |
| 普通转交/阶段交付 | 保留当前非终态 key | handler 改为目标，已有参与关系保留 |
| R47 手动改状态 | 锁内要求当前与目标来源类别均为非终态、session 未 closed，并校验 status_options_version | handler/待办不变；不建、不消费或作废补充请求，in_review 不算验收交接；done/cancelled→非终态只能走 R34/R38/R40 的明确重开 |
| 请求补充 | 当前已是 blocked 类别则保留，否则来源 canonical blocked | 保存前状态、真实请求者、目标及请求 ID |
| 补充完成并交回 | 恢复请求保存的有效非终态 key；失效则明确展示并选来源 canonical in_progress | 接收人只能是该次 requester，不能由客户端替换 |
| 提交验收并转交 | 保留有效 in_review 类别 key，否则来源 canonical in_review | handler 改为原 creator，不代表完成 |
| 完成/取消 | 来源 done/cancelled 类别的有效 key | 保留 handler 和历史；执行 D06 的 close |
| 重开 | 明确的来源非终态 key，默认 todo | 保留 creator；handler 仅按 D05/R40 例外变更；paused，旧屏障及指令终态不变 |

来源无有效目标时返回 `status_unavailable`。为每个动作返回 `status_options_version`：对来源空间、action、可选 key/category/archived_at/updated_at 和默认/回退输入按固定排序取摘要；它是新鲜度凭证，不是授权。提交带该值，服务端在既有目录 fence 内重算并校验目标，变化返回 `status_options_changed` 与当前选项，不能静默改选。旧目录没有通用 revision，不凭空复用一个字段。

2026-10-14 接线补齐：三种定向动作分别通过 `status-options?action=request_information|return_information|submit_acceptance` 读取唯一规定目标；提交保留原 `expected_status` 与 `status_options_version`。交回摘要还绑定具体请求、请求者/承接者/父请求及保存状态；同 key 的名称或类别变化也可能使确认失效。普通转交不接受这些状态字段；已拒绝的 key 不因目录恢复或后续选项变化而重新执行。

补充请求允许沿当前责任链逐层交回：当前有效请求以 parent_request_id 串联；承接者再请求别人补充时压入新请求，交回消费顶层并恢复仍有效的父请求。普通阶段转交或发起人另行改派则将不再匹配责任链的请求标为 superseded。过期/已消费请求、目标离开或禁用时拒绝定向交回，保留说明，由仍有转交权的人重新选择普通转交。不会自动重新邀请旧目标。提交验收目标 creator 已禁用时同样拒绝该定向交接；已有有效 handler 可继续正常流转，R38 独立生效。

这些状态变化及 R47 手工状态操作均不触发旧自动 AI 派发；R47 与 R41/R42 共用锁内角色/版本重验和修改记录。普通状态 PATCH、批量更新、快捷键、AI/自动化不得直接改协作单状态、assignee、creator 或来源；使用同一动作服务或明确拒绝。source iteration/status 审计通过已有借用事务入口写入，AI 完成不代替人类验收。

## D05. 生命周期、留存和有限代管

`DeleteWorkspace` 在原有删除行锁内、任何清理前检查所有协作记录，包括 done/cancelled；命中返回 `workspace_has_collaboration_records`。创建协作先持有来源空间的 KEY SHARE 存在性锁，与删除形成互斥边界。拒绝不做部分清理。

执行空间或 runtime 清理不能删来源 run、屏障、已记录内容或终结验证行。清理前撤销相关 grant、持久停止意图并暂停受影响会话；通用 engine task 可以按原策略清理，但来源关联保留其 ID/身份快照且不依赖该行继续存在。禁止删除未结记录来“修复卡住”。

账号禁用不移除历史，也不自动改变 handler/creator。提交、领取、开始均校验账号状态与 auth epoch；禁用事务记录持久失效事件，按作者/owner 找到受影响指令和 grant，取消该作者未启动指令，停止可能运行的指令，撤回失效 owner 的资源并暂停。跨单收敛采用有序处理和持久进度；在收敛完成前，账户 epoch 失效本身已阻止新许可。快速禁用后恢复也不能复活旧 command/grant。

2026-10-14 收敛实现：每个持久失效意图以 `last_issue_id` 保存已提交游标；单据效果、历史和游标同事务，同账号后续 epoch 不越过前序。单次消费有明确步数和重试上限，当前批次跳过失败账号后继续其他账号，不跳过失败单据或推进其游标。`last_attempted_at` 持久记录调度顺序：成功时随效果提交，失败回滚后只用独立短事务更新尝试时间；整批失败或进程重启也不反复堵在同一失败前缀。依据原失效 epoch 判断旧 command/grant，保留恢复后重新授权的工作。此消费不构成原生终结证明，也不开放正向 Resume。

代管沿用现有 password/admin 管理域：完整有效会话、当前 super_admin、管理功能启用、有效管理组织覆盖来源空间；PAT、task token、observer 和空间管理员不够。独立接口只接受 `complete|cancel|reopen`、非空原因和幂等/版本信息；锁内重验原 creator 仍 disabled。使用真实管理员写 `admin_operation/admin_audit_event` 和本单历史，creator 永不改写。仅披露办理代管所需的目标元数据，不因此开放本单全文和 AI 日志。

**R40（O11 已确认）：** 仅 `reopen` 且 creator/handler 均 disabled 时必填 `successor_user_id`，目标必须是本单已有、未撤销参与关系的有效账号；无人可接手或漏选则拒绝。其他动作/非双禁用场景携带该字段均拒绝，handler 有效时普通代管重开保留责任。候选查询与提交均复用上述管理域检查，锁内重验账号、参与关系及 revision；变化拒绝，不静默换人。

同事务重开、更新 handler/待办、终结不匹配的补充请求链、写前后处理人及原 creator 的审计/历史/outbox，幂等 payload 含接手人；失败全回滚。管理员不新增参与关系或取得普通转交权；新 handler 仅取得既有处理人权限。creator 不变、session paused、旧屏障/指令终态保留，不恢复账号、grant 或 AI。

## D06. 会话、队列与终结证明

新建 session 为 `paused`，原因 `runtime_unbound`。无资源时可保存显式指令但不派发；owner 授权、绑定资源之后仍需协作者明确恢复。`active` 只表示允许调度；是否空闲还必须看未结 run。`closed` 可同时存在仍待停止确认的旧进程。

| 事件 | 同事务结果 |
| --- | --- |
| 显式“发给 AI” | 验证参与/开放类别，分配 seq，固定作者及输入；active 时提示调度，paused 时只等待，closed 时拒绝 |
| 取消等待指令 | R45 查权后仅 waiting→cancelled，追加真实取消人/时间及事件；原文/作者/seq、active_run、queue_state 和 control_version 不变 |
| Stop | 先设 paused，再写当前 run 停止意图、撤业务能力和操作记录；未执行项保留 |
| 移除/退出作者 | 本人待执行项 cancelled；本人 run 若可能已启动则请求停止并 paused；其他作者队列保留 |
| 资源撤回或 owner 离开 | grant 失效，拒绝新许可，当前使用此资源的 run 停止，会话 paused；其他作者队列保留 |
| 普通转交/交回/提交验收 | 会话和执行不变 |
| 明确 Resume | 账号/参与/开放类别/grant/运行能力有效且没有未结 run，才改 active；空队列保持空闲 |
| Switch | 发起人或 handler；必须 paused、无未结 run、新 grant 有效，提升 resource_epoch，仍 paused |
| Complete/Cancel | 来源状态及生命周期版本、closed、全部未执行项 cancelled、当前 run 停止、审计一起提交 |
| Reopen | 非终态、提升 lifecycle_epoch、paused；不清旧 run，不复活旧指令、授权或失败的恢复请求 |
| 可信终结 proof | 只结算匹配 run、清它的屏障；paused/closed 保持原样 |

command 状态为 `waiting → in_flight → succeeded|failed|stopped`；未启动取消为 `cancelled`。run 的进度独立为 prepared/dispatched/preparing/launch_authorized/running/ending/settled；proof 为 never_launched_proven/may_be_live/exited_proven/unknown。已发许可但尚无 started 回报也必须视为可能存活。

完整准入使用[执行协议 E01](research/execution-contract.md)：显式验证来源正式 admission、source/execution 资源、账号/grant/绑定、installation accepting、部署 gate 及唯一 FIFO。promote 将队首设 in_flight；从未发过许可的 run 由服务器按 E03 原子撤派发并结算，不能等待一个从未领取的 daemon。claim/permit 不伪造实际 started。

R45 与 promote 共用 issue/session 锁，锁内重验参与、当前角色、同单 command 状态及版本。取消先提交则 promote 跳过该条；已 in_flight（包括尚未获许可的占位）或终态则返回 `command_not_waiting`，不自动调用 Stop 或改变后续队列。相同幂等键重放原结果，新的请求不得覆盖首个取消者/时间；取消不依赖资源授权、在线状态或执行 gate，正常 active 调度仍按原 FIFO 继续。

启动采用 E04 的跨进程锁和持久 nonce burn→进程记录→run seal→proof 状态机；burn/fork/PID 回写间崩溃为 unknown，不重复 fork。内部 attempt 也需前次证明和新许可；最终 proof 前 seal 整个 run，杜绝迟到 fallback。may_be_live 不换机器重派。

只接受服务器从未许可、daemon 持久永不启动墓碑或完整进程树结束证明；task 终态、超时、租约、断线、401、PID 缺失/not_observed 均不能放行。E05 分开 outcome/proof：结果先到仍占屏障，proof 先到但结果丢失以 outcome_unconfirmed 失败结算，不伪造成功或无限等日志。停止/取消优先，late success 不覆盖。正常失败且可信结束可继续 active FIFO；停止/取消不重试。撤权立即拒发新许可，旧许可 run 请求停止，不承诺零延迟物理停止。

停止请求带 `control_version`、观察到的 active_run/generation。普通评论、进度和新入队不改变控制版本。若 A 已结束且 B 已占位，针对 A 的旧 stop 返回 `stale_execution`，不误停 B；若尚未有 B，可接受暂停并返回 already_ended。Stop 先提交时，迟到成功不覆盖 stopped。未确认时 Resume 固定返回 `stop_unconfirmed`，不保存未来恢复意愿；proof 到达后相同 key 仍拒绝，必须新建明确恢复请求。

## D07. 凭据、运行能力与文件准备

| 能力 | 持有者 | 范围 |
| --- | --- | --- |
| 人类登录 | Web/Desktop 用户 | 仅该账号实际权限；body 的 actor 不可信 |
| daemon 资源身份 | daemon | 原有 execution namespace/owner/installation/binding 门禁，不转交给协作者或 provider |
| 协作执行业务凭据 | 当前 provider 子进程 | 显式 purpose，绑定本 issue/session/command/run/grant/epoch；默认拒绝所有未列入允许表的 API |
| 终结专用凭据 | supervisor 私有账本 | 只取本 run 停止意图、提交最小 fenced proof；不读内容、不写评论、不 claim/start/续租/resume/切资源 |

业务凭据只允许本单最小上下文和本次过程/成果；全局 purpose 白名单默认拒绝其他路由。每次调用 `AuthorizeCollaborationBusinessCapability` 比较当前 run/task/command、作者账号/参与 epoch、owner 账号、grant/resource/lifecycle epoch、安装绑定及未撤销能力，写操作在同事务重验。不得降级到 owner 的人类成员权、普通 mat_ 或 PAT/JWT/MDT fallback，不导出 connected-app 凭据。

认证兼容和版本来源固定在[执行协议 E07](research/execution-contract.md)：password 比较真实 session_version，中立协作账号 epoch 覆盖受支持的 legacy 人类登录。managed 复用合法安装绑定；legacy daemon 完成独立 challenge/key 的协作能力注册，不能放宽原 password-only enrollment，未注册不执行。grant 每次验证 agent/runtime 所属、关联、owner/私有约束、namespace/binding；变更组合即失效。

终结凭据在首次可能启动前预发，保留原部署/run/installation 的不可变验证快照和 token hash；撤销普通业务权限、密码轮换、owner 禁用或 runtime 删除不使其丧失仅收尾能力。只在本 run 确认后转为可重放的结果墓碑，不靠短 TTL 让长时间离线永远无法交证明；若因泄露显式吊销该能力，仍保留 unconfirmed。证明字段严格解码、receipt ID+digest 幂等，旧证明不能清新屏障。

业务能力撤回后不通过终结通道继续上传正文、附件、路径或任意结果。已入库过程/用量保留；未能入库的尾部标记记录不完整，不能为了补日志放宽撤权，也不能让日志失败阻塞合法的进程终结证明。R05 共享系统已记录内容，不承诺回收未上传数据。

Server 按协议/provider/OS 与实际 containment 准入；未满足显示升级/不支持。现有 ProcessExited 不统一，需要 supervisor；Windows Job 绑定失败拒绝启动，其他平台不能只查直接子进程或原进程组。PGID 消失不能证明调用 setsid/双 fork 的后代已退出；共享能力必须证明全部派生进程被封闭在可监督的执行范围内，提示词禁止后台、PID/PPID 轮询及未观察到进程不构成保证。无法提供此能力则拒绝共享启动；已失去完整观测的 run 保持 unknown。能力矩阵由 fake executable 和原生测试证明，详见执行复审 EX01。

协议信任绑定 daemon，不能证明恶意宿主行为；API ACL 不等于 OS 沙箱，不启用脱管后台 launch。代码通过已提交 Git/明确附件准备，不自动同步或启动。供应商 session 只在资源匹配、上次可信结束时复用，换环境重建上下文。

## D08. 事务、锁、幂等与旧入口

数据库是唯一顺序权威。准备网络资源在事务外；事务内所有授权 helper 接受同一 pgx.Tx，不借 pool 开旁路事务。预读 ID 仅为锁提示，取得锁后重读状态/身份，发现变化则整笔回滚。

锁序按[执行协议 E02](research/execution-contract.md) 的逐 writer 表：保留身份/namespace/目录前置锁，source/session 串行，跨旧资源顺序使用 NOWAIT；teardown 不能持 resource 阻塞等待 source。终结 proof 单独使用历史身份，不调用当前用户/runtime 授权 wrapper。新执行持部署 gate 的 FOR SHARE，冻结先短事务关 gate 再扫描。

遇到 NOWAIT、deadlock 或 serialization failure，回滚完整事务并有界抖动重试（最多三次），不重复外部副作用；仍冲突返回可重试 busy。不同 actor 的并发 stop 各留真实操作记录，可共用同一终结根。响应成功不得早于 commit；commit 结果不明只能按原 request ID 查询/重放。

人类请求统一 `Idempotency-Key`；正文包含对应业务 revision 或控制 version、目标及动作。授权先于回执读取；重复 key+相同 payload 返回原结果，变 payload 返回 `idempotency_conflict`。已退出者仅可在真实 actor 身份下查询自身操作的无内容收据，不能借旧回执重读本单。拒绝的 Resume、已完成的转交、旧 lifecycle epoch 下的请求也不因状态变好而重新执行。前端不能把旧请求自动配上新版本再提交。

旧入口逐项清单见[执行协议 E08](research/execution-contract.md)，覆盖 issue/comment/Plugin、所有执行 writer、运维取消和清理。service 与 SQL selector 都须分流；普通 issue 取消不停止 AI 的现有规则不变，R35 仅用于协作单。

## D09. API、实时与附件

账号对象接口位于 Auth 之后、普通 workspace membership middleware 之外，内部显式对象授权；清除 ambient `X-Workspace-ID/Slug`。创建仍验证明确来源空间的创建权限。

| 端点族 | 契约 |
| --- | --- |
| `GET /api/me/workbench?view=current|participating` | 同一服务端谓词分页/计数；当前仅聚合本功能协作单，不扩展普通任务范围 |
| `GET /api/me/collaboration-notifications`、`POST .../{id}/read|archive` | R52 账号级通知列表/未读数，按当前权限投影；已读/归档只改通知状态 |
| `POST /api/collaboration/issues` | 显式 source workspace、handler/说明，可选 R49 项目/父任务、R50 迭代（带 expected_iteration_revision）；原子新建，不提供 enable/convert |
| `PUT .../iteration` | R50 仅迭代归属：expected_revision，切换/移出带 iteration_reason，不混入其他字段 |
| `GET /api/collaboration/creations/{requestId}?source_workspace_id=...` | 仅真实 creator 查询创建幂等结果，无需预先知道 issue UUID |
| `POST /api/collaboration/recipients/resolve` | 精确账号查找，无写入 |
| `GET /api/collaboration/issues/{uuid}` 及 timeline/runs/messages | 最小来源摘要、creator/handler、参与/能力/版本、状态选项、队列及历史；R49–R51 的项目/父/子任务/迭代引用按查看者来源权限裁剪 |
| `PATCH /api/collaboration/issues/{uuid}` | R41/R42/R47–R49、R58 `stage` 字段白名单（`position` 走普通排序写入，不推进协作版本）；版本、幂等、历史及来源选项校验，status 仅非终态且带 status_options_version |
| `POST .../handoffs`、`information-returns`、`acceptance-submissions` | 明确业务意图、目标或 request ID、revision、幂等键 |
| `POST .../participants`、`participants/{user}/remove`、`leave` | 身份及指令/资源/责任副作用同事务 |
| `POST .../comments`、`commands` | 两个不同意图；command 响应含独立 ID、seq、真实等待状态 |
| `POST .../commands/{commandId}/cancel` | R45 仅 waiting，当前角色/同单归属/版本/幂等；状态变化返回 command_not_waiting |
| `PATCH/DELETE .../comments/{id}` | R43 仅作者；版本/幂等、历史及删除标记，禁用旧触发/清理副作用 |
| `POST .../comments/{id}/resolve`、`unresolve` | R57 当前本单评论权、同单评论、真实人类操作者；复用线程解决字段，不发送通知或执行 |
| `POST/DELETE .../comments/{id}/reactions/{emoji}`、`.../reactions/{emoji}` | R57 评论/单据回应，当前评论权；只添加或移除本人回应，复用既有记录，不通知、不执行 |
| `POST .../session/stop`、`resume` | control version/run identity；Resume 另带 expected_execution_epoch，防维护前意图复活 |
| `POST .../grants`、`grants/{id}/revoke`、`runtime-binding` | owner 确认与管理者绑定分开；切换后仍 paused |
| `POST .../lifecycle` | 发起人 complete/cancel/reopen，来源状态校验及收尾 |
| `GET .../operations/{requestId}` | 当前授权范围的回执；只有最小自操作收据可在失去单据权限后取回 |
| `GET /api/admin/collaboration/issues/{uuid}/reopen-candidates` | R38 管理域且单据已结束、双禁用；仅返回有效参与者的最小 AccountSummary |
| `POST /api/admin/collaboration/issues/{uuid}/lifecycle` | R38；仅 R40 分支必填 successor_user_id，其他场景拒绝该字段 |
| 专用 daemon collaboration claim/launch/event/proof 路由与等价 WS RPC | 完整 purpose、run/fence/nonce 检查；终结路径独立窄认证 |

错误码区分 forbidden/not_found、stale_revision、stale_execution、stop_unconfirmed、grant_required、runtime_upgrade_required、issue_closed、status_unavailable、idempotency_conflict、unsupported_collaboration_action。无访问依据的对象不泄漏存在性；关键协议损坏返回 unavailable，不能解释为空列表或已停止。

通知使用稳定 UUID；列表、未读数、read/archive 均按真实 recipient 限定，不以来源成员身份替代。R55 只解析当前参与者：新评论对每个接收人仅一条通知，提及只改变该条展示；评论/正文编辑按锁内前后有效提及差集，仅新增提及者收到 mention_added，不向全体广播编辑。操作者排除，@all 展开当前参与者，@AI/非法或非参与者提及不触发执行、订阅或新权限。投递身份在原事务固定，同一 event/recipient 幂等；重试不按后来成员名单重新投递。评论编辑新增提及沿用讨论 §49 的已定规则。

增加账号连接及显式 issue scope，不让外部参与者自动加入来源 workspace room。账号事件通知工作台/权限失效，并按 R52 向当前有效参与者（除操作者）写账号级通知，与 collaboration_event 同事务；旧 subscriber/inbox 监听对协作单的参与者跳过，不在来源空间写订阅或 inbox；旧 mention 监听对协作单整体跳过，R55 提及只在当前参与者内解析，非参与者与 @all 扩展不写任何 inbox；本单内容事件只给当时有 read basis 的订阅者。订阅、重连和受保护消息发送前检查权限，撤权丢弃尚未发送的旧 payload。事件带 issue、seq、revision 和授权代次；outbox、轮询及快照补偿断线，不借重连恢复队列。其他执行空间成员只能看到其运维权限允许的无正文运行摘要，通用 task list/snapshot/message/WS 不泄漏来源内容。

附件首次上传进入私有协作 draft，创建时验证 uploader/目标并一次绑定。元数据/字节/preview/range/绑定均查同单授权。fetch 可带 cookie/bearer；无 cookie 的 Web 原生图片/下载和 Desktop 原生链接均使用用户/对象/权限版本绑定、每次兑付重验的短能力，由服务器代理字节。不能跳转公网/S3 URL，禁止 `/uploads/*` 与旧 signed-download fallback；不把 bearer 登录强制改成 cookie。

R56：评论/选区建子任务的 manual 与 agent 模式对协作单在 capture 前拒绝，不走 source-context 复制，避免受控附件落入普通 `ObjectURL`。R44 区分评论当前展示引用和本单交付归属：删除评论或移除附件仅更新前者，保留来源 issue 关联、固定文件版本和历史引用。本单附件/历史入口继续按当前对象权限提供预览下载，其他评论/command 不改引用。旧 comment/file 清理及 GC 不得把无当前展示引用的已交付文件当作孤儿删除；未绑定私有 draft 的清理规则不变。

本单可见权只覆盖该附件，关联任务/私人 chat 引用不展开。HTML 预览保留现有 sandbox 与 renderer 文件访问防线。撤权阻止后续受控访问并清理缓存；已经下载或正在传输的字节不能远程收回，不作这种承诺。

## D10. Web / Desktop 工作台

统一地址 `/account?view=current|participating&issue=<uuid>#comment-<id>`；`account` 已是保留词且当前未用作 Web 页面。新增路径构造器/校验器，不占用可能已有的 workspace slug。分享、通知、跨账号流转默认使用此地址。

“当前处理”由有效参与、assignee=viewer 及来源非 done/cancelled 类别判定，blocked/in_review 不遗漏；“我参与的”包含有效参与关系下的终态和转出历史。通知已读/归档不改变任何责任。详情独立于列表谓词，转交后可继续看当前详情并切到参与视角。零空间账号也能进入；创建要求另选自己有创建权的来源空间。

共享页面放 `packages/views/collaboration/`，复用详情/评论/附件/transcript 的可参数化部分；业务查询、schema、动作及草稿放 core，UI 原子组件仍不依赖 core。Web/desktop app 只接认证、路由、原生外壳。来源空间旧详情识别协作模式后进入同一详情策略，不能由缺失字段降级为普通可执行页面。

工作台及有创建权的来源任务页提供“新建协作任务”：选来源、本人/精确接收人、正文/私有 draft 附件；一次原子提交后打开返回 UUID。未知结果查原创建 key，失败保留输入，无 enable/convert。已认证账号目的地不依赖 workspace list 成功；该列表 500 只影响空间选择，认证失败/强制改密仍阻断。

Desktop 为持久账号 TabGroup 增加 account/workspace/none scope，非一次性 WindowOverlay。仍由单个 tab Coordinator 投影路由；账号页进入前显式清 `setCurrentWorkspace(null, null)`。v1 一个工作台 tab，命中已有 tab 时也更新目标 `?issue=`；历史与 memento 包含 issue/view。持久格式迁移保留 workspace tabs、pin、MRU 和 history；离开某空间不删账号组。覆盖冷启动零空间、深链、pop-out 导航与跨 Server/账号清理。该持久全局目的地须在实现时同步 CLAUDE.md/Desktop spec 的路由分类，不复制一次性 overlay 规则。

Query key 含 server/viewer/issue/session；列表另含归一化 view/筛选/排序，cursor 按 infinite-query 管。原 workspace key 仍含 wsId。挂载、账号 WS 重新认证/重连、窗口聚焦都重验权限/控制快照，不能继承 Infinity 而一直信旧缓存。Query 管服务器数据；Zustand/StorageAdapter 仅持账号隔离的布局、草稿、未知请求 key。每个 composer 默认普通评论，AI 模式显式可见，不跨单暗继承。

来源 membership 变更亦触发账号/issue 投影失效：重验两种访问依据，保留有效参与者的协作历史及发起人生命周期能力，清除已失效来源目录和结构编辑缓存。workspace loss 不等同 participant removal，不删除账号组；现有 workspace WS 重定向不能替代这次快照重验。

API JSON 经 schema/parseWithFallback；关键 ID/版本/能力无效则 unavailable，可选显示才降级。动作结合显式 true/身份/状态，服务器重验。多实体动作 await，未知结果保留原 key。撤权先推进 access epoch，再 cancelQueries(revert:false)、清 Query/草稿/上传/预览/局部 transcript，阻止迟到响应；仍有来源成员权则重取基础投影。临时 500 可保留上次合法内容，权威拒绝必须清理。

协作内容、通知、受控附件/短能力及操作回执的 parseWithFallback 统一使用 redact:true；畸形响应日志只保留端点/结果码，不能写入 received、正文、token、能力 URL 或含值的 Zod 详情。未知能力或协作模式不得降级为普通可执行单据。

来源空间列表/看板/时间线按 R54 用服务端最小 AccountSummary 显示外部人员，不回退到成员表的 “Unknown”；协作卡的负责人列拖动改走转交入口，状态列拖动仅 R47 非终态，二者都不先乐观移卡。停止界面区分已受理/等待确认/已确认，结束单据仍展示未结 run。断线、加载、空列表及无权分别表达；双端覆盖键盘、焦点、窄屏和 200% 缩放。代管共享窄表单，R40 明确选人、空候选禁提交，失败保留输入并刷新候选，不自动改选。

## D11. 上线、回退与风险边界

1. 先部署加法 schema 和完整新旧 writer 防线，协作创建开关保持关闭；部署操作停止旧调度/后台 writer，确认所有可写节点支持同一协作协议再开放。不能混跑不识别 purpose 的旧服务。
2. 再交付 daemon/supervisor 能力矩阵、Web/Desktop；未兼容 runtime 不领取协作任务，旧客户端最多使用服务器允许的投影/动作，不能 fallback 普通执行。
3. 关闭创建仍保留读取/收尾。E06 持久 gate 先关再暂停所有 session；Resume 持 gate 共享锁至提交并比较 epoch。扫描完成才解除，旧意图失效，不自动恢复。
4. 已存在协作状态后仅可回退到理解这些数据和门禁的兼容版本；不支持直接回退到旧二进制或 drop 表。暂停后仍保留收尾服务及未结屏障，恢复版本也不自动恢复队列。
5. 观测等待长度/年龄、停止待确认时间、outbox 堆积、失效作者/资源、能力拒绝及重复 nonce；日志只记录 ID/结果码，不记录私有正文或凭据。

主要限制：账本丢失或旧进程无法证明结束时保持暂停，管理员/超时/换环境不能强制放行；跨空间锁、撤权和原子接手仍需 DB 并发验证，客户端兜底不能掩盖权限错误。
