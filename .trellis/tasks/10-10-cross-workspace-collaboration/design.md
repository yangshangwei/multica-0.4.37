# 跨账号协作技术设计

- 日期：2026-10-17；状态：M01–M11 产品选择及修订方案实施均已确认。当前实施范围见[通道 FIFO 与配置失效续作](continuation-fifo-2026-10-18.md)。
- 需求依据：[PRD](prd.md) 与[验收附表](acceptance.md)；执行顺序见[实施计划](implement.md)。
- 当前调整依据：[需求设计调整](mention-redesign-2026-10-17.md)及讨论 §57–§66。本文描述完整目标模型；已落实的多目标存储、控制与读取契约见[当前代码规范](../../spec/server/collaboration-targets.md)，其按目标控制将由 lane 与目标 hold 取代；真实运行协议与完整界面仍待完成。
- 具体关系与签名见[数据契约](research/data-contract.md)、[执行协议](research/execution-contract.md)；源码锚点见[研究摘要](research/design-contracts.md)。新能力保持未启用，普通任务行为不随本修订改变。

## D01. 唯一任务与两种空间

一张单保留一份共享历史及一个 issue 级 session，只负责整单暂停/关闭；执行通道（lane）按实际根智能体划分，各有有序输入、控制版本和当前执行组。lane 间可并行，小队成员/续作属于其组内工作。

保留来源工作空间的一条 `issue`，不复制影子任务，不将外部协作者补成整个空间的成员。`issue_collaboration` 的存在是启用协作契约的权威事实；API 的协作标记由此派生，客户端不能通过 PATCH 标记转换普通任务。

完整结构图见[研究索引](research/design-contracts.md#当前协作结构图拟实施)。

明确区分 `source_workspace_id`（业务、状态、内容归属）和 `execution_workspace_id`（agent、runtime、installation 归属）。来源 issue 的 UUID、编号、creator 和 workspace 不随转交、切换环境、结束或重开改变。

当前处理人复用 `issue.assignee_type='member'` 和 `assignee_id=user.id`。现有字段实际存全局用户 UUID，不能改用 `member.id`。协作分支验证有效账号及本单参与关系；普通任务仍执行原空间成员规则。AI 资源通过 target/grant/授权清单绑定，不改写 issue 负责人。target 是有类型的 agent 或 squad；重复同一 target 去重，@A 与 @S（队长 A）同 lane 同批合并为一次队长运行（M06），不按显示名合并，共享实际资源仍竞争同一容量。

采用显式协作执行关联：engine task 使用独立 `purpose=collaboration`，普通 `issue_id/chat_session_id` 留空，由 run→execution_group→command/target 关联唯一来源 issue。实际执行者和原人类授权来源分开记录，派生不能改记为环境拥有者。解析 purpose 必须先于“无 issue/chat 则 quick-create”的旧回退。engine task 的 `context` 只存绑定 ID 和非敏感调度字段，不存来源正文。来源 transcript、结果、用量和附件独立持久化，不能仅引用会被执行空间清理的 task/message 行。

首版只由已认证人类通过明确入口新建。原子建立 issue、协作记录、发起人/处理人参与关系、session；默认处理人为创建人，选定其他账号时按 R46 同事务加入并派单，无待接受阶段。创建不触发普通任务自动执行；人类新评论的有效 @生成根 command，小队受控派发只产生原组内工作；无 @评论、回复、编辑和状态变化不产生新根执行。首版不开放协作单硬删除、跨空间移动、合并及存量转换；旧入口返回明确不支持，防止抹掉身份或 R37 的保留依据。

## D02. 持久数据与约束

数据附表定义 issue 控制器、target、command、执行组和 run。每 issue 一个 session 保留；session 单 grant/run 权威和全局未结 run 唯一性由逐 lane/组约束替代。迁移须保留历史与未结证明（M10 删除旧单会话数据除外），不能只删旧索引放开并发。

完整字段、身份版本、唯一性、迁移和索引规则见[数据契约](research/data-contract.md)。全局 user UUID、协作版本字符串与旧 Issue 数字版本分离；来源单据、逐 lane 输入顺序与组内派生记录由服务端持久化为权威。

## D03. 授权与身份

人类角色保留。机器仅以绑定本单、目标、执行组、实际资源与授权代次的协作身份参与；普通凭据、@文本或队长身份不能代替授权。

新增事务可复用的 `ResolveIssueAccess`，明确读取两个独立依据：有效来源成员或有效本单参与者。`loadIssueForUser` 只负责解析，不能被误当成跨空间授权器。所有关联任务、文件、评论、run 必须再次核对确属本单。

| 动作 | 允许主体及额外条件 |
| --- | --- |
| 读取本单、历史、已记录执行及附件；普通评论 | 当前有效人类账号且来源成员或本单参与者；未获本单执行授权的机器只得最小引用/写入拒绝；合法协作能力仅按 D07 的窄用途访问 |
| 发 AI 指令、两级停止/恢复 | 当前有效参与者；提交/恢复/开始各自检查 D06 门禁。停止不要求 grant 仍有效 |
| 组织新增 AI 目标 | 发起人或当前处理人；仅写待授权目标，不自动授权资源或邀请人类 |
| 确认/撤回目标资源授权 | 真实 owner 只确认其合法资源组合；授权时须已按既有规则合法加入本单，组织者不能替其确认或自动加人；退出按 R31 自动撤回 |
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

R36（2026-10-13 已确认）：发起人退出来源空间后，账号及本单参与关系仍有效时，保留完成、取消、重开权限；这些动作不额外要求来源成员身份。来源空间权限随成员身份失效，R49/R50/R58 的结构字段仍拒绝写入。退出来源空间不等于退出本单，也不构成 D05 的账号禁用代管条件。

来源 workspace revoke 只清该空间的成员/资源/来源订阅，不因此推进全账号协作 epoch、撤销 issue_participant 或清 creator/handler；若同时导致运行资源失效，仅走对应 grant 撤回与暂停。R53 的最小引用必须在查询匹配/计数/排序前执行：机器搜索协作单只能匹配允许的最小引用字段，不能利用正文/评论全文检索、snippet、命中数或高亮探测私有内容。HTTP/WS 与导出/搜索复用服务端身份投影。

人类确认入口使用可信实际登录主体，并拒绝任务凭据和 legacy agent actor 冒充。R53 的内容读取同样按白名单判人类：仅服务端标记的登录会话与个人 PAT；`task_token`、`cloud_pat`、plugin、IM 及未分类来源一律按机器，不沿用 `isMachineCredentialActor`（`server/internal/handler/actor_guards.go:110`）“未知视为人类”的默认。空间 admin/owner、资源 owner、历史作者都不是通用管理例外。退出后仍为来源成员则重新获得基础读/评论投影，不保留旧协作控制能力；两种依据都失效则详情、附件、实时和回执内容一致拒绝。

编辑按 R41/R42/R48/D08，值和前后值、操作者、时间及事件同事务保存，不改 command 或派发；R48 日期解析/清空沿用普通任务。R42 仅投影本单所需来源选项，写时重验定义，不开放配置管理。R43 保留评论 UUID、版本历史及删除墓碑，回复锚点不变；锁内重验作者/评论权限/版本，删除后不接受编辑，重试沿用 D08。旧 API/Plugin/空间 admin 无旁路；resolve/unresolve 与表情回应按 R57 记录真实操作者、保持每线程单一解决记录，旧回应通知对协作单跳过；已交付附件按 R44 留存，私有未绑定 draft 仅本人可清理。

R42 接线约定（2026-10-14）：PATCH 的 `priority`、`label_ids`、`properties` 缺省或外层 null 均保留旧值；空标签数组替换为空集合，属性对象按定义 UUID 局部修改，键值 null 清除该值（含已归档定义）。标签 ID 归一化去重，归一化后冲突的属性键拒绝。复用普通字段值校验和 16 KiB 存储限制，数据库 JSONB 相等视为无变化。字段选项只给当前发起人/处理人，详情只带所选标签和值引用的最小定义；详情授权、字段与版本取自同一数据库快照。新增内部摘要字段在缺省时省略，以保留原有标题/日期请求的幂等回执恢复能力。

评论持久版本/墓碑、collaboration_event 编辑历史及附件永久归属/多引用按数据契约实施。R57 解决/取消解决与评论编辑/删除共用 issue 锁，锁后重验权限再清旧解决并写目标；仅两条 UPDATE 放在事务里不能防止两个不同目标并发成为已解决。普通评论/回应复用现有表，协作分支禁止硬删除和旧隐式触发；R24 的新评论 @执行只走专用入口，编辑/回应不产生根执行。

每条 command 保持 `originator_user_id=accountable_user_id=command.author_user_id`；合并批次的 run 记录全部作者，engine task 的 originator 取批次首条 command 作者，仅作展示/用量归属，授权以业务 token 为准；另记不可变 creator、当前 handler、资源 owner 和供应商账号来源，不能为了通过私有 agent 校验将作者改成 owner。用量归属描述执行资源账号，不引入跨人自动结算或凭据共享。

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

执行空间或 runtime 清理不能删来源 run、屏障、已记录内容或终结验证行。清理前撤销相关 grant、持久停止意图并暂停受影响目标/组，无关有效目标保持状态；通用 engine task 可以按原策略清理，但来源关联保留其 ID/身份快照且不依赖该行继续存在。禁止删除未结记录来“修复卡住”。

账号禁用不移除历史，也不自动改变 handler/creator。提交、领取、开始均校验账号状态与 auth epoch；禁用事务记录持久失效事件，按作者/owner 找到受影响指令和 grant，取消该作者未启动指令，停止可能运行的指令，撤回失效 owner 的资源并暂停。跨单收敛采用有序处理和持久进度；在收敛完成前，账户 epoch 失效本身已阻止新许可。快速禁用后恢复也不能复活旧 command/grant。

2026-10-14 收敛实现：每个持久失效意图以 `last_issue_id` 保存已提交游标；单据效果、历史和游标同事务，同账号后续 epoch 不越过前序。单次消费有明确步数和重试上限，当前批次跳过失败账号后继续其他账号，不跳过失败单据或推进其游标。`last_attempted_at` 持久记录调度顺序：成功时随效果提交，失败回滚后只用独立短事务更新尝试时间；整批失败或进程重启也不反复堵在同一失败前缀。依据原失效 epoch 判断旧 command/grant，保留恢复后重新授权的工作。此消费不构成原生终结证明，也不开放正向 Resume。

代管沿用现有 password/admin 管理域：完整有效会话、当前 super_admin、管理功能启用、有效管理组织覆盖来源空间；PAT、task token、observer 和空间管理员不够。独立接口只接受 `complete|cancel|reopen`、非空原因和幂等/版本信息；锁内重验原 creator 仍 disabled。使用真实管理员写 `admin_operation/admin_audit_event` 和本单历史，creator 永不改写。仅披露办理代管所需的目标元数据，不因此开放本单全文和 AI 日志。

**R40（O11 已确认）：** 仅 `reopen` 且 creator/handler 均 disabled 时必填 `successor_user_id`，目标必须是本单已有、未撤销参与关系的有效账号；无人可接手或漏选则拒绝。其他动作/非双禁用场景携带该字段均拒绝，handler 有效时普通代管重开保留责任。候选查询与提交均复用上述管理域检查，锁内重验账号、参与关系及 revision；变化拒绝，不静默换人。

同事务重开、更新 handler/待办、终结不匹配的补充请求链、写前后处理人及原 creator 的审计/历史/outbox，幂等 payload 含接手人；失败全回滚。管理员不新增参与关系或取得普通转交权；新 handler 仅取得既有处理人权限。creator 不变、session paused、旧屏障/指令终态保留，不恢复账号、grant 或 AI。

## D06. 多目标调度、控制与终结证明

session 的 `queue_state=active|paused|closed` 是整单控制。执行通道（lane）按实际根智能体划分，@A 与以 A 为队长的小队共用通道 A（M06）；lane 保存控制状态、授权配置代次、单调输入序号和至多一个当前根执行组。不同 lane 按实际 agent/runtime/独占工作目录容量并行。

一条评论可生成多个目标 command，各自进入所属 lane。lane 忙碌或暂停时新 command 为 waiting；promote 按 FIFO 取合格输入成为一个 execution_group、一次运行（M07），同批 @S 使其以小队身份运行，遇另一支合格同队长小队截断，hold 或授权/配置失效者被跳过、不挡他人也不截断。每条 command 仍保留作者、快照、目标和附件；未开始时单条取消或作者移除只移出该条，已开始的组整批停止。组内 run 分别是智能体、队长、成员或续作；组保持 lane 位置直到全部内部工作结算并封闭派生。成员不进入另一个目标的外部 FIFO；一个 run 完成不代表小队完成。command 的目标、输入/附件快照和真实人类作者不可变，不能在执行时取全局当前 grant 把 @A 交给 B。

**小队接续（M08 保持现状，见[小队派发](research/squad-delegation-2026-10-17.md)）：** 队长照旧发 @成员 评论后结束当前 run，进程退出后释放实际容量；组进入 waiting_children 不占运行位。仅本组存活 run 发出、被 @ 者在冻结名册内且授权有效时，评论 @成员 才算组内派发，comment_id+成员去重。成员回帖或结束时在同组合并生成队长续作 run（新 run/fence/nonce，继承原人类授权），不依赖“负责人是小队”。小队仍在执行时人类再 @S 或 @队长，满足 E01“M11 并入”则并入下一次队长续作，组停止或结算时未被任何 run 读取的并入输入回到 waiting（M11）。容量为 1、崩溃重放和目录竞争必须实际验证。

| 动作 | 持久结果 |
| --- | --- |
| 新评论 @有效目标 | 保存输入、各目标 command/回执；忙碌、暂停或离线时 waiting，不自动恢复；无效目标有独立拒绝结果 |
| 取消 waiting command | 只把选中输入移出待执行批次，不删除同评论其他目标，不影响当前组或控制状态 |
| Stop target | 暂停该 target 所在 lane（同 lane 目标一起暂停），封闭当前组新派生，停止其已许可 run，取消尚未启动的派生；waiting 保留，其他 lane 不变 |
| Stop issue | 先暂停整单，再处理全部目标/组；新增或重新授权目标也受整单暂停约束 |
| Resume target | 整单 active、本目标配置/授权有效，且 lane 无证明缺口（暂停 lane 的未结组，或被强制停止缺 proof 的当前组），才释放本目标 hold 并解除 lane 暂停（同 lane 目标一起恢复）；任一不满足整笔拒绝、零写入；不改变其他 lane |
| Resume issue | 整单级条件（终态、gate/epoch、协议、权限、目标集合版本）不满足则整笔拒绝。否则同一事务解除整单暂停，证明齐的 lane 恢复、授权有效且无强制停止缺 proof 组的 hold 目标释放，其余保持暂停/hold 并返回原因；已移除目标的遗留屏障不阻止解除整单暂停，但继续占用实际资源（M04） |
| 作者移除/退出 | 跨 lane 移出并取消其未开始输入及未读取的并入输入；含其已读取输入的组整批停止（同批他人输入随之结束）并暂停该 lane；其他作者等待输入保留 |
| 资源撤回/owner 离开 | 撤对应本单 grants，失效目标置 hold，停止受影响组，lane 不暂停；无关有效目标不变，owner 离开覆盖其本单全部资源 |
| Switch target resource | creator/handler 组织，目标 paused、旧组可信结束、新组合获实际 owner 授权；保留目标身份和输入，切换后目标 switch_hold 仍不执行；换队长时 waiting 转入新队长 lane 末尾（E07） |
| Complete/Cancel | 关闭整单、封闭全部派生、停止所有 run、取消全部等待输入，保留历史 |
| Reopen | 整单/目标保持 paused，不复活旧组、停止/取消输入或旧授权 |
| 正常组完成 | 全部 run/proof/派发均结算后才释放 lane；两个控制层均 active 且授权/容量允许才开始下一根输入 |

首次 session 默认为 paused，新目标初始 hold；授权、重新授权及切资源不解除暂停或 hold。忙碌/暂停/离线不是撤权，有效目标仍可选并显示等待原因；失效目标退出可选 @清单，历史和合法停止入口保留。

控制请求显式 `scope=target|issue`、相应控制版本及观察到的 active group/代次；Resume 还捕获每个目标的 `expected_hold_version`，与 lane 控制版本分开，撤权或新 consent 后旧请求不能释放新 hold；全单操作还绑定目标集合版本。旧 stop 指向已经被新根组替换的目标时返回 stale_execution，不能误停后来工作；同组新成员仍在其控制闭包内。普通评论和进度不推进控制版本。

每个 run 分开持久化 outcome、forced_disposition、proof 和 settled_at。从未发过许可的 run 可由服务器可信结算；发过许可或不确定时只接受原 supervisor 的 sealed proof。组结算要求所有实际 run 和可启动派发记录收口；队长证明不能覆盖其他机器的成员。拒绝/未知 Resume 沿原 key 重放原结果，不保存延后恢复意图；proof 只收尾，不恢复 gate。nonce burn、原生 containment、late outcome 和无许可结算详见 E03–E05。

## D07. 凭据、运行能力与文件准备

| 能力 | 持有者 | 范围 |
| --- | --- | --- |
| 人类登录 | Web/Desktop 用户 | 仅该账号实际权限；body 的 actor 不可信 |
| daemon 资源身份 | daemon | 原有 execution namespace/owner/installation/binding 门禁，不转交给协作者或 provider |
| 协作执行业务凭据 | 当前 provider 子进程 | 显式 purpose，绑定本 issue/session/group（含其 command/target/grant 集合）/run/epoch；默认拒绝所有未列入允许表的 API |
| 终结专用凭据 | supervisor 私有账本 | 只取本 run 停止意图、提交最小 fenced proof；不读内容、不写评论、不 claim/start/续租/resume/切资源 |

业务凭据只允许本单最小上下文、本次过程/成果及服务器核验的组内派发。成员须在该组已授权资源清单内；服务器从父 run 固定组/目标/原人类身份，不能接受机器自行指定另一单或新根作者。普通输出 @不开放任意机器新根执行；全局 purpose 白名单默认拒绝其他路由。每次调用 `AuthorizeCollaborationBusinessCapability` 比较当前 run/task/group 及其 target/command 集合、原人类账号/参与 epoch、实际 owner 账号、grant/target/resource/lifecycle epoch、安装绑定及未撤销能力，写操作在同事务重验。不得降级到 owner 的人类成员权、普通 mat_ 或 PAT/JWT/MDT fallback，不导出 connected-app 凭据。

认证兼容和版本来源固定在[执行协议 E07](research/execution-contract.md)：password 比较真实 session_version，中立协作账号 epoch 覆盖受支持的 legacy 人类登录。managed 复用合法安装绑定；legacy daemon 完成独立 challenge/key 的协作能力注册，不能放宽原 password-only enrollment，未注册不执行。grant 每次验证 agent/runtime 所属、关联、owner/私有约束、namespace/binding；变更组合即失效。

终结凭据在首次可能启动前预发，保留原部署/run/installation 的不可变验证快照和 token hash；撤销普通业务权限、密码轮换、owner 禁用或 runtime 删除不使其丧失仅收尾能力。只在本 run 确认后转为可重放的结果墓碑，不靠短 TTL 让长时间离线永远无法交证明；若因泄露显式吊销该能力，仍保留 unconfirmed。证明字段严格解码、receipt ID+digest 幂等，旧证明不能清新屏障。

业务能力撤回后不通过终结通道继续上传正文、附件、路径或任意结果。已入库过程/用量保留；未能入库的尾部标记记录不完整，不能为了补日志放宽撤权，也不能让日志失败阻塞合法的进程终结证明。R05 共享系统已记录内容，不承诺回收未上传数据。

Server 按协议/provider/OS 与实际 containment 准入；未满足显示升级/不支持。现有 ProcessExited 不统一，需要 supervisor；Windows Job 绑定失败拒绝启动，其他平台不能只查直接子进程或原进程组。PGID 消失不能证明调用 setsid/双 fork 的后代已退出；共享能力必须证明全部派生进程被封闭在可监督的执行范围内，提示词禁止后台、PID/PPID 轮询及未观察到进程不构成保证。无法提供此能力则拒绝共享启动；已失去完整观测的 run 保持 unknown。能力矩阵由 fake executable 和原生测试证明，详见执行复审 EX01。

协议信任绑定 daemon，不能证明恶意宿主行为；API ACL 不等于 OS 沙箱，不启用脱管后台 launch。代码通过已提交 Git/明确附件准备，不自动同步或启动。供应商 session 只在资源匹配、上次可信结束时复用，换环境重建上下文。

## D08. 事务、锁、幂等与旧入口

数据库是唯一顺序权威。准备网络资源在事务外；事务内所有授权 helper 接受同一 pgx.Tx，不借 pool 开旁路事务。预读 ID 仅为锁提示，取得锁后重读状态/身份，发现变化则整笔回滚。

锁序严格按[执行协议 E02](research/execution-contract.md) 的逐 writer 表，本文不另定义聚合锁顺序。保留身份/namespace/目录前置锁及 source 短事务锁，跨旧资源顺序使用 NOWAIT；teardown 不能持 resource 阻塞等待 source。终结 proof 单独使用历史身份，不调用当前用户/runtime 授权 wrapper。新执行持部署 gate 的 FOR SHARE，冻结先短事务关 gate 再扫描。

遇到 NOWAIT、deadlock 或 serialization failure，回滚完整事务并有界抖动重试（最多三次），不重复外部副作用；仍冲突返回可重试 busy。不同 actor 的并发 stop 各留真实操作记录，可共用终结意图；控制锁不跨实际执行时间持有，不阻止不同目标的运行并发。响应成功不得早于 commit；commit 结果不明只能按原 request ID 查询/重放。

人类请求统一 `Idempotency-Key`；正文包含对应业务 revision 或控制 version、目标及动作。授权先于回执读取；重复 key+相同 payload 返回原结果，变 payload 返回 `idempotency_conflict`。已退出者仅可在真实 actor 身份下查询自身操作的无内容收据，不能借旧回执重读本单。拒绝的 Resume、已完成的转交、旧 lifecycle epoch 下的请求也不因状态变好而重新执行。前端不能把旧请求自动配上新版本再提交。

旧入口逐项清单见[执行协议 E08](research/execution-contract.md)，覆盖 issue/comment/Plugin、所有执行 writer、运维取消和清理。service 与 SQL selector 都须分流；普通 issue 取消不停止 AI 的现有规则不变，R35 仅用于协作单。

## D09. API、实时与附件

账号对象接口位于 Auth 之后、普通 workspace membership middleware 之外，内部显式对象授权；清除 ambient `X-Workspace-ID/Slug`。创建仍验证明确来源空间的创建权限。

| 端点族 | 契约 |
| --- | --- |
| `GET /api/me/workbench?view=current\|participating` | 同一服务端谓词分页/计数；当前仅聚合本功能协作单，不扩展普通任务范围 |
| `GET /api/me/collaboration-notifications`、`POST .../{id}/read\|archive` | R52 账号级通知列表/未读数，按当前权限投影；已读/归档只改通知状态 |
| `POST /api/collaboration/issues` | 显式 source workspace、handler/说明，可选 R49 项目/父任务、R50 迭代（带 expected_iteration_revision）；原子新建，不提供 enable/convert |
| `PUT .../iteration` | R50 仅迭代归属：expected_revision，切换/移出带 iteration_reason，不混入其他字段 |
| `GET /api/collaboration/creations/{requestId}?source_workspace_id=...` | 仅真实 creator 查询创建幂等结果，无需预先知道 issue UUID |
| `POST /api/collaboration/recipients/resolve` | 精确账号查找，无写入 |
| `GET /api/collaboration/issues/{uuid}/field-options` | R42：当前发起人/处理人的本单编辑选项；返回本单/来源 ID、协作版本、issue 标签和有效字段定义，不返回账号目录、使用统计或配置管理数据 |
| `GET /api/collaboration/issues/{uuid}` 及 timeline/runs/messages | 最小来源摘要、角色/能力/版本、整单控制、targets/队列/执行组及历史；R49–R51 引用按来源权限裁剪 |
| `GET .../mention-targets` | 本单人员及完整有效授权的 typed AI targets，附控制/等待能力，不读其他资源目录 |
| `POST .../targets`、`POST .../targets/{target}/consents` | creator/handler 组织待授权目标，各实际 owner 只确认自身资源；未齐备不进入可选清单 |
| `POST .../comments/trigger-preview`、`POST .../comments` | 同一目标解析器，预览无副作用；发送保存不可变输入、逐目标回执和去重通知 |
| `POST .../control` | scope=target/issue，stop/resume、版本、观察到的 group/目标集合；整单恢复按 lane 与目标 hold 结算（M04），单目标恢复全有或全无 |
| 组内派发/续作 | 协作入口把本组存活 run 评论中的 @成员 识别为派发，校验原组、冻结名册与授权；不从普通监听创建新根 |
| `PATCH /api/collaboration/issues/{uuid}` | R41/R42/R47–R49、R58 `stage` 字段白名单（`position` 走普通排序写入，不推进协作版本）；版本、幂等、历史及来源选项校验，status 仅非终态且带 status_options_version |
| `POST .../handoffs`、`information-returns`、`acceptance-submissions` | 明确业务意图、目标或 request ID、revision、幂等键 |
| `POST .../participants`、`participants/{user}/remove`、`leave` | 身份及指令/资源/责任副作用同事务 |
| `POST .../commands/{commandId}/cancel` | R45 仅 waiting，当前角色/同单归属/版本/幂等；状态变化返回 command_not_waiting |
| `PATCH/DELETE .../comments/{id}` | R43 仅作者；版本/幂等、历史及删除标记，禁用旧触发/清理副作用 |
| `POST .../comments/{id}/resolve`、`unresolve` | R57 当前本单评论权、同单评论、真实人类操作者；复用线程解决字段，不发送通知或执行 |
| `POST/DELETE .../comments/{id}/reactions/{emoji}`、`.../reactions/{emoji}` | R57 评论/单据回应，当前评论权；只添加或移除本人回应，复用既有记录，不通知、不执行 |
| `POST .../grants/{id}/revoke`、`targets/{target}/runtime-binding` | 实际 owner 撤权与 creator/handler 组织绑定分开；明确目标，切换后仍 paused；授权通过上述 consents 入口 |
| `POST .../lifecycle` | 发起人 complete/cancel/reopen，来源状态校验及收尾 |
| `GET .../operations/{requestId}` | 当前授权范围的人类流转回执；只有最小自操作收据可在失去单据权限后取回 |
| `GET .../control/operations/{requestId}` | 重新验证当前账号/对象授权后，返回本人原控制请求的完整成功/拒绝回执，保留 issue/request/action、逐 channel 与 target_holds 结果；不更新版本或重新执行 |
| `GET /api/admin/collaboration/issues/{uuid}/reopen-candidates` | R38 管理域且单据已结束、双禁用；仅返回有效参与者的最小 AccountSummary |
| `POST /api/admin/collaboration/issues/{uuid}/lifecycle` | R38；仅 R40 分支必填 successor_user_id，其他场景拒绝该字段 |
| 专用 daemon collaboration claim/launch/event/proof 路由与等价 WS RPC | 完整 purpose、run/fence/nonce 检查；终结路径独立窄认证 |

错误码区分 forbidden/not_found、stale_revision、stale_execution、stop_unconfirmed、grant_required、runtime_upgrade_required、issue_closed、status_unavailable、idempotency_conflict、unsupported_collaboration_action。无访问依据的对象不泄漏存在性；关键协议损坏返回 unavailable，不能解释为空列表或已停止。

R29 请求明确携带 `kind`、完整 `value` 以及恰好一个非 null 的 `issue_id` 或 `source_workspace_id`；不接受重复/未知字段。返回 `{issue_id, source_workspace_id, recipient}`，创建上下文中的 issue_id 为 null，recipient 沿用最小 AccountSummary。现有单据在查询前后均重验邀请/转交能力；创建上下文先查来源成员和创建开关，再查目标。查询没有参与、责任或授权写入，关闭创建不影响已有单据内的合法查找。复用有界认证限流器的独立 actor/IP 命名空间，后端故障拒绝；精确匹配无可用账号为 `recipient_not_found`，历史归一化歧义为 `recipient_ambiguous`。core 核对上下文和账号 ID 查询的目标身份，保存原输入，确认结果不代替后续写事务的目标校验。

通知使用稳定 UUID；列表、未读数、read/archive 均按真实 recipient 限定，不以来源成员身份替代。R55 只解析当前参与者：新评论对每个接收人仅一条通知，提及只改变该条展示；评论/正文编辑按锁内前后有效提及差集，仅新增提及者收到 mention_added，不向全体广播编辑。操作者排除，@all 只展开当前参与者；未解析为具体目标的 @AI、非法/非参与者人员提及不产生执行、订阅或权限。有效 agent/squad 另按协作入口生成 command，不能被混合人员提及吞掉。投递身份在原事务固定，同一 event/recipient 幂等；重试不按后来成员名单重新投递。评论编辑新增提及沿用讨论 §49 的已定规则。

增加账号连接及显式 issue scope，不让外部参与者自动加入来源 workspace room。账号事件通知工作台/权限失效，并按 R52 向当前有效参与者（除操作者）写账号级通知，与 collaboration_event 同事务；旧 subscriber/inbox 监听对协作单的参与者跳过，不在来源空间写订阅或 inbox；旧 mention 监听对协作单整体跳过；新入口分别解析当前参与者和已授权 AI 目标，非参与者不写 inbox；本单内容事件只给当时有 read basis 的订阅者。订阅、重连和受保护消息发送前检查权限，撤权丢弃尚未发送的旧 payload。事件带 issue、seq、revision 和授权代次；outbox、轮询及快照补偿断线，不借重连恢复队列。其他执行空间成员只能看到其运维权限允许的无正文运行摘要，通用 task list/snapshot/message/WS 不泄漏来源内容。

新评论以一次持久提交保存 comment/input、每个 AI target 的 accepted/blocked 结果、有效 command 和人员通知意图。单个目标无权/失效不吞掉其他合法目标或通知；数据库失败全部回滚。同 request ID 重放原回执，不重复插入评论、执行或通知；同 typed target 去重；@A 与 @S（队长 A）进入同一 lane 批次，合并为一次队长运行（M06）。预览与发送共用解析器但后者重验；取消本次目标只抑制执行，不删文字。

目标登记由 creator/handler 组织，资源 owner 在合法权限内确认自身组合；小队所需资源未齐备不进入候选，不因小队授权把成员自动开放为独立 direct-agent 目标。配置/资源变化不得扩大旧授权；失效移除候选但保留历史，忙碌/暂停/离线仅显示等待。该清单不暴露其他空间机器目录或凭据。

多目标控制 DTO 使用显式协议版本和 targets/groups 集合。旧单会话协议已删除（M10），不识别该版本的客户端须显示不支持/只读并拒绝执行控制，不能把未知协议当作空闲或 fallback 普通执行；旧数字 Issue.revision 不变。

附件首次上传进入私有协作 draft，创建时验证 uploader/目标并一次绑定。元数据/字节/preview/range/绑定均查同单授权。fetch 可带 cookie/bearer；无 cookie 的 Web 原生图片/下载和 Desktop 原生链接均使用用户/对象/权限版本绑定、每次兑付重验的短能力，由服务器代理字节。不能跳转公网/S3 URL，禁止 `/uploads/*` 与旧 signed-download fallback；不把 bearer 登录强制改成 cookie。

R56：评论/选区建子任务的 manual 与 agent 模式对协作单在 capture 前拒绝，不走 source-context 复制，避免受控附件落入普通 `ObjectURL`。R44 区分评论当前展示引用和本单交付归属：删除评论或移除附件仅更新前者，保留来源 issue 关联、固定文件版本和历史引用。本单附件/历史入口继续按当前对象权限提供预览下载，其他评论/command 不改引用。旧 comment/file 清理及 GC 不得把无当前展示引用的已交付文件当作孤儿删除；未绑定私有 draft 的清理规则不变。

本单可见权只覆盖该附件，关联任务/私人 chat 引用不展开。HTML 预览保留现有 sandbox 与 renderer 文件访问防线。撤权阻止后续受控访问并清理缓存；已经下载或正在传输的字节不能远程收回，不作这种承诺。

## D10. Web / Desktop 工作台

统一地址 `/account?view=current|participating&issue=<uuid>#comment-<id>`；`account` 已是保留词且当前未用作 Web 页面。新增路径构造器/校验器，不占用可能已有的 workspace slug。分享、通知、跨账号流转默认使用此地址。

“当前处理”由有效参与、assignee=viewer 及来源非 done/cancelled 类别判定，blocked/in_review 不遗漏；“我参与的”包含有效参与关系下的终态和转出历史。通知已读/归档不改变任何责任。详情独立于列表谓词，转交后可继续看当前详情并切到参与视角。零空间账号也能进入；创建要求另选自己有创建权的来源空间。

共享页面放 `packages/views/collaboration/`，复用详情/评论/附件/transcript 的可参数化部分；业务查询、schema、动作及草稿放 core，UI 原子组件仍不依赖 core。Web/desktop app 只接认证、路由、原生外壳。来源空间旧详情识别协作模式后进入同一详情策略，不能由缺失字段降级为普通可执行页面。

工作台及有创建权的来源任务页提供“新建协作任务”：选来源、本人/精确接收人、正文/私有 draft 附件；一次原子提交后打开返回 UUID。未知结果查原创建 key，失败保留输入，无 enable/convert。已认证账号目的地不依赖 workspace list 成功；该列表 500 只影响空间选择，认证失败/强制改密仍阻断。

Desktop 为持久账号 TabGroup 增加 account/workspace/none scope，非一次性 WindowOverlay。仍由单个 tab Coordinator 投影路由；账号页进入前显式清 `setCurrentWorkspace(null, null)`。v1 一个工作台 tab，命中已有 tab 时也更新目标 `?issue=`；历史与 memento 包含 issue/view。持久格式迁移保留 workspace tabs、pin、MRU 和 history；离开某空间不删账号组。覆盖冷启动零空间、深链、pop-out 导航与跨 Server/账号清理。该持久全局目的地须在实现时同步 CLAUDE.md/Desktop spec 的路由分类，不复制一次性 overlay 规则。

Query key 含 server/viewer/issue/session；列表另含归一化 view/筛选/排序，cursor 按 infinite-query 管。原 workspace key 仍含 wsId。挂载、账号 WS 重新认证/重连、窗口聚焦都重验权限/控制快照，不能继承 Infinity 而一直信旧缓存。Query 管服务器数据；Zustand/StorageAdapter 仅持账号隔离的布局、草稿、未知请求 key。composer 没有单独的 AI 模式：发送前的触发预览列出本次将执行的 AI 目标及合并情况，可逐个取消，草稿不跨单继承。

来源 membership 变更亦触发账号/issue 投影失效：重验两种访问依据，保留有效参与者的协作历史及发起人生命周期能力，清除已失效来源目录和结构编辑缓存。workspace loss 不等同 participant removal，不删除账号组；现有 workspace WS 重定向不能替代这次快照重验。

API JSON 经 schema/parseWithFallback；关键 ID/版本/能力无效则 unavailable，可选显示才降级。动作结合显式 true/身份/状态，服务器重验。多实体动作 await，未知结果保留原 key。撤权先推进 access epoch，再 cancelQueries(revert:false)、清 Query/草稿/上传/预览/局部 transcript，阻止迟到响应；仍有来源成员权则重取基础投影。临时 500 可保留上次合法内容，权威拒绝必须清理。

协作内容、通知、受控附件/短能力及操作回执的 parseWithFallback 统一使用 redact:true；畸形响应日志只保留端点/结果码，不能写入 received、正文、token、能力 URL 或含值的 Zod 详情。未知能力或协作模式不得降级为普通可执行单据。

来源空间列表/看板/时间线按 R54 用服务端最小 AccountSummary 显示外部人员，不回退到成员表的 “Unknown”；协作卡的负责人列拖动改走转交入口，状态列拖动仅 R47 非终态，二者都不先乐观移卡。停止界面提供目标/小队与整单两级动作，区分已受理/等待确认/已确认，成员过程归所属组，结束单据仍展示未结 run。整单恢复逐 lane 显示结果（“已恢复 N 个，M 个仍暂停”及原因），整单级拒绝时保持原暂停。断线、加载、空列表及无权分别表达；双端覆盖键盘、焦点、窄屏和 200% 缩放。代管共享窄表单，R40 明确选人、空候选禁提交，失败保留输入并刷新候选，不自动改选。

## D11. 上线、回退与风险边界

1. 先部署相对 main 为加法的 schema（M10 删除只针对未发布分支数据，执行后不可回退）和完整新旧 writer 防线，协作创建开关保持关闭；部署操作停止旧调度/后台 writer，确认所有可写节点支持同一协作协议再开放。不能混跑不识别 purpose 的旧服务。
2. 再交付 daemon/supervisor 能力矩阵、Web/Desktop；未兼容 runtime 不领取协作任务，旧客户端最多使用服务器允许的投影/动作，不能 fallback 普通执行。
3. 关闭创建仍保留读取/收尾。E06 持久 gate 先关再暂停所有 session；Resume 持 gate 共享锁至提交并比较 epoch。扫描完成才解除，旧意图失效，不自动恢复。
4. 已存在协作状态后仅可回退到理解这些数据和门禁的兼容版本；不支持直接回退到旧二进制或 drop 表。暂停后仍保留收尾服务及未结屏障，恢复版本也不自动恢复队列。
5. 观测等待长度/年龄、停止待确认时间、outbox 堆积、失效作者/资源、能力拒绝及重复 nonce；日志只记录 ID/结果码，不记录私有正文或凭据。

主要限制：账本丢失或旧进程无法证明结束时保持暂停，管理员/超时/换环境不能强制放行；跨空间锁、撤权和原子接手仍需 DB 并发验证，客户端兜底不能掩盖权限错误。

## D12. 修订状态与评审边界

M01–M03 及修订方案实施已确认，当前稿采用整单控制器、按根智能体分 lane 的批次 FIFO、组内派生及逐资源授权（M04–M11 见讨论 §64–§66）。M09 已验证单直接智能体运行端许可/proof、单 lane 正向恢复、评论文本合批及 v3 读取，见[代码规范](../../spec/server/collaboration-targets.md)和[2026-10-18 验证](verification-summary-followup-2026-10-18.json)。目标 hold/M04 控制与资源边界基础及双直接智能体通道已经[本轮验证](verification-summary-hold-2026-10-18.json)；混合 FIFO、完整 Switch/配置收敛、M08/M11 及完整双端仍按[当前续作](continuation-hold-2026-10-18.md)和实施计划逐项验证。文档复审见 [review-2026-10-17.md](review-2026-10-17.md)，实施续作见讨论 §62；旧日期测试不替代本轮验证。
