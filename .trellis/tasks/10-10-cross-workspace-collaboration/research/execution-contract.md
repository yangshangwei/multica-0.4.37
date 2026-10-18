# 共享执行的准入、启动和收尾契约

2026-10-17：M01–M11 的完整执行目标，承接[设计](../design.md)及[数据契约](data-contract.md)。已落实的目标/组/停止与拒绝恢复基础见[当前代码规范](../../../spec/server/collaboration-targets.md)，其按目标控制的写法将由 lane 与目标 hold 取代；真实调度、许可/proof、正向恢复和容量验证仍未交付。

## E01. 分层准入

拟用 CanAdmitCollaborationRun(tx, identities, expected) 统一验证；task 的空 issue_id 不代替来源准入，active 也不等于资源可运行。

| 边界 | 必须成立的事实 |
| --- | --- |
| 新评论及根 command | 普通评论保留当前有效人类来源成员或参与者的访问依据；只有有效参与者可产生根 command，且来源须非终态、typed target 属于本单并已完整授权。按 input_event/target 去重，固定目标/作者/快照；暂停、离线、忙碌或维护可记录 waiting，不假报开始 |
| R45 取消 waiting | 当前有效参与者且为作者/creator/handler，同单同目标、仍 waiting（含 hold 及 M11 已并入未读取者）、版本匹配；不依赖 grant、在线状态或执行 gate，不隐式升级成 Stop |
| promote 待执行批次 | lane 无未结 group；全局与 lane 均 active。按 FIFO 取合格 command 为一批，遇第二支合格小队截断；目标 hold、配置/grant 当前无效（同事务置其 hold）或已并入他组的 waiting 被跳过，不挡其他输入也不截断。合并运行依赖批次内全部 target 的 grant 集合，含 @S 时需 S 的完整名册授权。原子把入批 command 置 in_flight、设 group_id、占 group 位并建首个 run/派发意图；另一支同队长小队的 command 留到下一批（M06/M07）。其他 lane 不受限制 |
| M11 并入 | 仅 batch_role=squad 的 group：尚有存活或排队 run，expansion_sealed_at 与 settled_at 均空；全局、lane 与 gate active，作者为有效参与者，目标为 S 或其当前队长 A、无 hold 且配置/grant 合法，lane 无更早未并入的合格 waiting。锁 lane→group 后重验，原子把该 target 冻结配置/grant 追加进组集合，command 设 group_id 仍 waiting，并入（或新建）排队中的队长 continuation；该 run claim 时把输入写入其 prompt 快照并同事务置 in_flight，即为“已读取”，只有该 run 取得 never_launched_proven 才算未读取。运行环境固定用 S 的队长配置，已运行成员 token 不变；已读取输入依赖的 grant 失效按谓词 2 整组停止；只为未读取并入输入追加的 grant 失效时该条回 waiting（其目标 hold），该输入被取消/作者移除时该条 cancelled，两种情况都只摘它追加的 grant |
| claim/finalize/首次 permit | 完整运行谓词，run/组/目标身份一致，实际资源槽位已取得；既有 group 中的成员/continuation 不再争抢根 command 位置 |
| 组内派生 | 本组存活 run 的评论 @成员（M08 现状），同一未封闭 group、冻结名册内且授权有效，comment_id+成员去重。持久意图不等于已启动；不建普通 issue 子任务或其他目标根 command |
| 后续 provider attempt | 同 run 未 sealed、前次 attempt 已证明结束、完整运行谓词仍成立；独立 attempt nonce，不复用许可 |
| Resume（scope=target/issue） | target 须 E05 全部条件通过，否则零写入拒绝；issue 按 E05 逐 lane/hold 结算；gate FOR SHARE 至提交；恢复不占槽位，缺容量/离线只导致合法等待 |
| Switch | creator/handler；明确目标 paused（所在 lane 暂停或目标 hold）、其旧 group/run 全部结清、新配置与每项资源 consent 合法；同事务置目标 switch_hold，不换 typed target，不发 permit；换队长按 E07 转 lane |
| 业务 token 读/写 | 绑定当前 issue/group（含其 target/grant 集合）/run/实际 grant 的动态能力白名单；不能退回人类 read basis |
| Stop/强制失效/close | 验人类控制权或可信系统失效原因；不要求 grant、在线端或执行 gate 仍有效 |
| 终结 proof | 只依固定历史 run/installation/token/账本身份，不要求当前账号、grant、runtime/task 行存活 |

完整运行谓词在锁内重读：

1. 来源 issue/协作关系存在，admission_status=not_required/accepted，类别已识别且非 done/cancelled。新建为正式 not_required；blocked 不等于 triage 未接纳，旧 triage 不得改变协作 admission。
2. session 与所属 lane 均 active；group 仍是该 lane 当前根 group、未被停止/封闭。组内每条 command 作者的账号/参与/lifecycle 代次，以及各 target 的 target_epoch、config_version、grant/resource 代次均匹配；任一失效则整批停止（批次未开始或 M11 并入未读取时只移出该条）。重新加入、重开、禁用后恢复不能复活旧身份。
3. 该 run 的实际 agent/runtime 属于 execution workspace 且绑定一致，资源未归档/解绑；grant 属于本 group 冻结的 grant 集合并对应该资源。授权人是实际 runtime owner 并具备 agent 的既有 invoke 资格；公有资源仍验 namespace/invoke，私有 owner 一致约束不放宽。
4. managed installation/binding/owner/workspace/runtime/epoch 匹配且 admission=accepting；legacy 端须有 E07 验证过的独立协作 enrollment。NULL/0/binding 缺失不视为通过。
5. 部署 gate 开放，协议/provider/OS containment 合格；运行端在线健康，实际 agent、机器及排他工作目录容量允许。首次许可还绑定 run fence、claim_generation、launch/attempt nonce；成员/continuation 同样检查，不继承队长占用或宽权限。

容量按实际 daemon，不按其 runtime 分摊增额；agent 限额与机器/目录预约均须满足，先取槽再 claim，真实结束/清理后释放。DB waiting 不释放槽；A 缺容量不阻塞其他目标，等待者仍保序并获公平调度。

installation 停止/gate 冻结后仅观测/终结旧许可，旧恢复例外不允新 provider/成员/continuation。排他目录及私人供应商 session 不因目标类型不同而共享，见 E07。

## E02. 锁入口、幂等与控制范围

*Collaboration*InTx 借同一 pgx.Tx；网络准备在事务外，预读 ID 仅作锁提示，锁后重验。共同来源顺序：issue → session → lane → target（各自 UUID 有序）→ participant/grant → command/group → run/delegation。保留原目录/I1/账号/installation 前置 fence；提交串行不等于执行全单串行。

| writer | 前置锁 | 来源写入与交汇 |
| --- | --- | --- |
| 创建 | 账号 fence、source workspace KEY SHARE、创建操作键、draft | issue/参与/session/事件原子；与 LockWorkspaceForDelete 互斥 |
| edit/invite/transfer/remove/leave/close/reopen | 相关账号有序 fence、目录/I1、workspace | 共同聚合顺序，锁后验角色/版本/来源选项。R46 直接写参与/责任；执行副作用封闭相关 group 并写停止意图。R49/R50 另验来源成员，迭代用借用事务；R51 普通子任务仅把协作单当引用 |
| 新评论/根输入 | actor fence、workspace、目标资源身份前置锁 | 聚合锁内验访问/参与/名册，原子保存评论、逐目标 command/拒绝、通知/回执；M11 并入及组内派发先取 gate FOR SHARE，再按共同顺序锁 lane→group，锁后重验未封闭/未结算 |
| R43/R57 | actor fence、workspace、当前 read basis | 共用 issue 锁，再验同单/作者/版本；清旧解决+设新解决原子，不锁运行资源 |
| R45 waiting 取消 | actor fence、workspace | issue/session/lane/target/participant/command/group（M11 已并入时），与 promote 串行；不改控制版本或 active_group，不锁 grant/gate |
| promote/claim/finalize/permit/派生 | managed user→namespace/installation 或 enrollment；gate FOR SHARE；workspaces UUID 有序 | 共同聚合顺序，agent/runtime/task 后置锁 NOWAIT；空 task.issue_id 不代替 lock_issue_execution |
| Resume/任何 active 写入 | 账号/资源前置协议、gate FOR SHARE+epoch | 锁来源聚合及完整目标集合，重验 E05，gate 持至提交；集合变化重试或 stale_scope |
| target 组织/consent/revoke/switch | organizer/owner、namespace/installation、workspaces 有序 | 锁后验 creator/handler 与实际 owner，原子配置/grant/事件，后置资源 NOWAIT。新目标初始 hold（initial）；consent、重新授权与 config_version 变更同事务写入或保留 hold，均不释放；换队长按 UUID 序锁新旧 lane，新 lane 唯一冲突整笔重试，转入的 waiting 按原相对顺序取新 lane seq |
| R38/R40 admin | LockPlatformAdminMutation；admin/creator/handler/successor 有序锁及组织范围 | 目录/I1→来源聚合；双禁用/目标/版本再验，生命周期/责任/审计原子，不放宽资源权限 |
| account 撤销 | 原治理锁，同事务 epoch+持久失效事件 | 不持用户锁阻塞多个 session；issue UUID/NOWAIT 分批收敛，同事务置受影响 grant 的目标 hold，旧 epoch 已阻止新能力 |
| 资源 teardown | 原资源锁前枚举受影响目标；或反向 source 全部 NOWAIT | 整笔重试，不持 resource 等 source；保存撤 grant/目标 hold/停止/屏障后才清 engine |
| source workspace delete | LockWorkspaceForDelete | 清理前查全部协作记录，含已结束；存在即拒绝 |
| proof | 历史 token/安装身份，不调用当前授权 wrapper | 只锁来源 issue/session/lane/target/group/run 与 proof/operation；结算事务锁 lane→group，确认无存活/排队 run 及待派发后同事务写 expansion_sealed_at 再结算；不要求资源行存活，engine 投影 NOWAIT 或提交后幂等 |
| freeze/enable | 短事务独占 gate | 先关再按 E06 扫描，不持独占 gate 等来源对象 |

与旧 workspace→agent→issue→runtime 或 task→issue 锁序相交时后置 NOWAIT；同类对象 UUID 有序。NOWAIT/40P01/40001 整笔回滚，最多三次有界抖动重试，不重复外部副作用或部分提交停止。

Idempotency-Key 按操作 scope/actor/request 唯一，payload_hash 含 control scope=target/issue、业务/控制/配置版本、target_set_version 及观察到的 active_group/generation。授权先于回执；同 key 重放原成功/拒绝，改 payload 拒绝。未知 commit 只查原 key，不自动配新版本重试；退出者仅可查本人无内容收据。

Stop 比较 typed scope/control_version/target_set_version 与观察到的 active_group/generation：新根 group 或目标替换使请求失效，返回 stale_execution/stale_scope；原 group 已结束且无替代者可暂停并返回 already_ended。成员扩展与停止共锁，按当前账本纳入原 group 的全部后代；不能只停预览中已出现的 run。评论/进度/入队不改控制版本。多个 actor 分别留操作记录，保留首个强制处置根。

## E03. 根队列、小队交接与未许可收尾

command：waiting → in_flight → succeeded/failed/stopped，未执行取消为 cancelled；所在 run 证明 never_launched 时 in_flight 回 waiting（作者已移除/退出或 session closed 则 cancelled）。promote 把整批 command 绑定到一个新 group，一次运行处理全部输入；active/waiting_children/ending group 持有该 lane 位置，完整终态才释放 active_group_id，同批 command 随组一起结算，其他 lane 可独立推进。

小队沿用现状交接（M08，见[小队派发](squad-delegation-2026-10-17.md)），不新增命令：

1. 队长发 @成员 评论后结束本轮。仅本组存活 run 的评论、被 @ 者在冻结名册内且授权有效时记为派发（@队长 只触发或合并续作），comment_id+成员去重，编辑不追加；@本单当前参与者按 R52 通知（如队长 @发起人求助），其他机器评论的 @ 不执行。
2. 进程退出及本地清理后释放真实 agent/机器/排他目录槽，group 保持 waiting_children，不占运行槽。成员不等队长 proof，同机互斥由 E01 容量保证。
3. 成员只争实际资源，不进入任何 lane 的外部 FIFO；同一 agent 在本单同时至多一个运行，沿用现有 (issue, agent) 规则。成员可照旧 @名册内其他成员，不加层数/轮数上限。
4. 成员回帖、提问或 run 结束时，本组至多一个排队中的队长 continuation，已排队则合并；新 run/permit/nonce，原人类作者/参与/lifecycle 不变。无需处理时队长记 no_action 退出；已 seal 的 run 不重开，不另建外部输入。人类新 @S/@队长满足 E01“M11 并入”时归入本组并合并进该次续作，否则 waiting。
5. 组无存活/排队 run 且每个 run 有合法终结证据时结算，最终队长结果决定业务结果；失败成员照常唤醒队长，服务器不隐式重试。Stop/revoke/close 封闭扩展，迟到 @成员/continuation 拒绝，已记录未启动者按处置结算。组停止或结算（含续作永久 prepare 失败）时，M11 已并入但未被任何 run 读取的输入清 group_id 回到 waiting（原 seq；作者已移除/退出或 session closed 则 cancelled），只为它追加的 grant 一并移出组集合。

必须验证同机器 MaxConcurrentTasks=1。仅拆 FIFO、改 DB waiting 或忽略存活队长计数不能释放真实容量；不能用协作业务 token 评论并结束本轮的 provider 不声明共享小队能力。旧实现证据：daemon.go:4811–4816 先取机器槽再 claim，4905–4917 在 handleTask 返回后释放；agent.sql:1879–1885 明确等待也计容量（均位于 server/internal/daemon/、server/pkg/db/queries/）。

R45 与 promote 同锁：取消先胜则跳过；promote 先胜返回 command_not_waiting，即使未许可也不暗升为 Stop。组内工作不是 R45 外部 waiting command。

任一 run 若从未许可，服务端可原子撤派发/claim，确认 authorized_attempt_seq=0、无 started、无矛盾 running/success 证据且 prepare 状态已知后，写 never_launched_proven 并 settled。不能等待未领取的 daemon，也不能因一个成员未启动就结清整个 group。

| 事件 | 处置 |
| --- | --- |
| Stop/revoke | 未启动 run 安全结清，可能存活者保留屏障；根 group 标 stopped，Stop 暂停所属 lane，revoke 只置目标 hold，外部 waiting 保留 |
| remove/leave/close | 取消该作者未执行根输入；含其输入的已运行组整批停止。close 覆盖全部目标/队列，封闭扩展，保留首个 stopped/cancelled 原因 |
| 永久 prepare 错误 | run failed；只证明本次从未许可，资源/能力失效置目标 hold，组仍等待其他工作收尾 |
| 投递暂失败/结果不明 | 重放同一 step/run/nonce/claim，不新建重复执行 |

任意已发 permit、许可提交不明或矛盾证据均须原安装 E04 proof。迟到启动因 settled/封闭/失效代次拒绝。v1 无服务端隐式失败重试；合法 continuation、provider attempt 各自记账，不能借前者重做 stopped command。

## E04. 每个 run 的启动账本和真实容量

supervisor 使用跨 daemon 进程的 OS 互斥锁、私有文件及原子写+fsync；进程内 mutex 不够。每 run 固定 deployment/installation/批次角色 target/group/fence/launch nonce；阶段单向推进：

    registered
      → attempt_nonce_burned
      → process_identity_recorded
      → attempt_reaped
      → run_sealed
      → terminal_proof_persisted
      → proof_acknowledged

同一 launch 锁内先持久 burn 本次 attempt nonce，再允许 fork。burn 后/fork 前、fork 后至 PID/containment 回写间崩溃一律可能存活；无 PID、查不到进程、不再持有内存 slot 均不是未启动证明。重复请求只恢复原观测，不重复 fork；原 containment 无法恢复则 unknown，服务器保留该 run 及 group 的屏障。

provider fallback 仍属同一 run，分配单调 attempt_seq/独立 nonce，前次证明退出后重新取得完整 E01 许可。最终 proof 或 never_started 墓碑前，在同一跨进程锁内永久 seal 整个 run，禁止迟到 fallback，再持久化终结记录。proof 携最后授权/观察 attempt 边界；单次 reap 不等于整个 run sealed。组内 continuation 是新 run，不重开旧账本。

Windows Job 绑定/确认失败拒绝启动。Unix 原 PGID 消失不能证明 setsid、换组或双 fork 后代已退出；能力须强制封闭全部派生进程并按 OS/provider/启动模式举证。PID/PPID 轮询、提示词禁后台、stdio 关闭或直接子进程退出不足；运行后观测丢失保留 unknown。daemon 重启、另一实例、HTTP/WS 切换与旧 outbox 均服从同一 seal/nonce。

一个 run 的 OS 进程后代与平台小队成员 run 是两种关系：前者由本 run containment 证明，后者在各自安装、实际资源和独立账本证明。队长退出/机器槽释放不能证明远端成员已结束；group 终结需覆盖两层。

termination-purpose 能力在首次可能启动前预发，仅 supervisor 私有持有；记录固定安装验证身份和 token hash。撤销业务权限、owner 禁用/密码轮换、runtime/task 清理不取消窄收尾用途；只查本 run 停止意图和交最小 fenced proof，不能读正文、claim/start/续租/恢复。确认后留幂等墓碑，不靠短 TTL 使离线运行永远无法交证明；若因泄露显式吊销则保留 unconfirmed。

## E05. 结果、两级停止和按 lane/hold 恢复

run 独立保存 business_outcome、forced_disposition、proof_state、sealed_attempt_seq、recording_incomplete。进度为 prepared/dispatched/preparing/launch_authorized/running/ending/settled，证明为 never_launched_proven/may_be_live/exited_proven/unknown。已发 permit 未报 started 仍可能存活；task 终态/租约/心跳/401/超时/PID 缺失不解屏障。

| 到达顺序 | 处理 |
| --- | --- |
| outcome 无 proof | 留结果，run/根 group 不结清；无关目标可继续 |
| outcome+proof | 结清该 run；强制处置优先，否则 success/failure 或持久交接结果；组终态另查全依赖账本 |
| proof 无 outcome/交接结果 | failed/outcome_unconfirmed，标记录不完整；exit code=0 不算业务成功，不无限等日志 |
| Stop/close/revoke 在先 | 固定首个强制根，late success 不覆盖，后来原因追加 |
| proof 后 Stop | 不改历史成功，按当前范围版本暂停，不套到新 group |
| 重放/乱序 | event/receipt key+digest 幂等，不同 digest 拒绝，已结结果不被晚到正文改写 |

group 终态要求扩展已封闭（正常收口由结算事务封闭，见 E02）、全部成员/continuation 意图有确定处置、每个 run 有合法终结证据；按强制根或最终业务结果结算，原子推进 command 并清匹配 active_group_id。父 proof 不清子屏障，证明到达不解除暂停/closed/维护。

| 动作 | 原子效果 |
| --- | --- |
| Stop target | 所属 lane paused（同 lane 目标一起），封闭当前 group，撤新业务/启动能力并保存全后代停止意图；waiting 保留，其他 lane/全局不变 |
| Stop issue | session paused，暂停全部目标并封闭/停止其组；新增目标初始 hold 且受全局约束 |
| remove/leave 作者 | 跨 lane 把其 waiting（含已并入未读取者）移出并取消；含其已读取输入的组整批停止（同批他人输入随之结束），暂停该 lane；其他作者 waiting 与无关 lane 保留 |
| revoke/owner 离开 | 撤 grant，相关目标置 hold（grant_required），停止使用它或受派生依赖影响的组；lane 不暂停，组结算后照常推进同 lane 其他目标。无关目标不变，waiting 保留 |
| 普通转交/交回/验收交接 | 执行状态不变 |
| Complete/Cancel | 来源终态/lifecycle、session closed、全部 target paused、waiting cancelled、封闭/停止全部组及审计同事务 |
| Reopen | 新非终态/lifecycle、session paused；不清屏障或复活旧 command/group/grant |

Resume target 带所属 lane 的 control_version/group_generation、资源配置版本与 expected_execution_epoch，全部条件先验、拒绝零写入：session paused/closed 优先拒绝；本目标配置/grant 无效为 grant_required/config_invalid；lane 暂停且 scope 内未结 group 或 run/指针矛盾为 stop_unconfirmed。lane 当前未结组若因 revoke/漂移被强制停止且缺 proof，同样 stop_unconfirmed。通过后释放本目标 hold，暂停的 lane active（同 lane 目标一起）；lane 忙碌/active 时只释放 hold，不改其他目标；无 hold 且 lane 已 active 回执 already_active。全局 active 时无关目标的未知 proof 不阻止本目标恢复；拒绝不保存未来意愿。

Resume All 按 lane 与目标 hold 结算（M04）：

1. scope=issue 请求绑定 target_set_version、全局控制、gate/epoch 版本；锁内重枚举，集合漂移返回 stale_scope。终态、gate/协议无效或请求者无权时整笔拒绝，不改状态。
2. 否则同一事务 session=active，再逐个检查：暂停中的 lane 范围内全部应收尾 run 有证明者 active，否则保持暂停（stop_unconfirmed）；带 hold 的目标当前配置/grant/完整名册有效且所在 lane 无被强制停止而缺 proof 的组者释放，否则保持 hold（grant_required/config_invalid/stop_unconfirmed）。回执逐 lane 与逐 hold 目标给出结果，不保存延后恢复意愿；无效授权只挡其目标自身输入，不挡同 lane 其他目标。
3. 已移除目标的遗留屏障不阻止 session=active，但其可能存活的 run 继续占用实际 agent/机器/排他目录，直到取得证明。全局已 active 时未被停止的健康运行保持原样，不重建已有 group；临时离线/缺容量只导致合法等待。无目标时仅解除全局暂停，新目标仍初始 hold。

原 key 重放原回执；保持暂停的 lane 之后只能经新的 Resume target 恢复。完整结束的普通失败组可继续 active lane 下一批，stopped/cancelled 不重试。另建目标、重开、换资源、管理员声明或超时不绕过该 lane 的未知证明。

proof 严格解码，只含固定身份、sealed attempt 边界、未启动/退出枚举和有限退出信息，无正文/附件/路径。撤业务能力后不借终结通道补私有日志；来源已入库过程/用量保留，缺尾部标不完整，合法收尾不等日志。

## E06. 部署冻结与恢复

每部署持久 collaboration_execution_gate：enabled、epoch、freeze_operation_id/progress，独立于 creation_enabled。promote/claim/finalize/permit、派生/continuation 准入及所有 session/target active 写入持 FOR SHARE 至提交；冻结短事务 FOR UPDATE 关 gate、升 epoch，不持独占锁扫描来源对象。

先拒新许可，再按稳定顺序持久执行每单 Stop issue 并记游标，覆盖所有目标/远端成员，不能只查旧 active_run。重启继续原 operation。评论、合法 waiting、人类流转、Stop/proof/历史仍可用，界面不能把未扫描的 active 误报恢复。

Resume 比较 expected_execution_epoch；冻结和 enable 都提升 epoch，旧意图延迟到重开仍 stale_admission。扫描完成、无遗漏活跃队列且全部 writer 兼容才可 enable；enable/proof 不自动写 active。

关闭创建不关闭历史/收尾。旧 writer 须先停止，gate 无法约束不理解协议的旧代码；带协作数据不得回退旧二进制。数据迁移按数据契约，不删未结记录解除屏障；唯一例外是 M10 删除未发布分支上的旧单会话执行数据。

## E07. 目标授权、认证及运行身份

M03 菜单只列本单完整授权 typed targets；busy/paused/offline 单独显示，不等于 revoked。creator/handler 组织目标配置，实际 owner 逐项 consent；组织权不代替 invoke/owner 权。owner 须已合法参与本单，否则先明确邀请，@或 grant 不自动加人。

配置固定队长/成员角色、实际 agent/runtime/installation 与 grant；授权完整才供提及。成员、机器、owner 或执行权限规则变化需新 config_version，逐项取得真实 owner 对新版本的 consent，不把旧版本或名称相同视为授权。group 在 promote 冻结批次内各 target 的配置/名册/grant 及各 command 作者代次，每次派生/permit/业务调用重验。漂移置目标 hold（config_invalid）并停止受影响组，保留原 run/proof 身份，不向旧组加入新人或复活旧组；waiting 原目标保留，明确恢复后用合法新配置建下一组。Switch 同事务置目标 switch_hold；小队换队长时其 waiting 同事务转入新队长 lane 末尾，之后到达的 @S 也进新 lane。hold 期间 promote 跳过且不挡他人，可按 R45/R15 取消；Resume target S 只验 S 的新配置/grant 并释放 hold，新 lane 忙碌或 active 时只释放；暂停时按 E05 证明齐才一起恢复，否则整笔拒绝 stop_unconfirmed。旧 lane 状态不变。纯显示名变化不换执行身份。

每条 command 的 originator=accountable=其 author_user_id；合并批次的 run 记录全部入批 command 作者，续作 run 另记其读取的 M11 并入输入作者，业务 token 绑定 group 与其冻结 grant 集合。另记实际 agent、parent/delegator、资源 owner、供应商账号。派生不是机器新根输入，不冒充 owner。供应商恢复指针绑定 provider/agent/target/group/角色/资源代次，仅前次可信结束且匹配时使用；不同目标/角色不共享私人 session，换环境重建，文件只从已提交 Git/明确附件准备。

中立 collaboration_account_fence(user_id, epoch) 首次参与/授权原子建立；password 另验真实 session_version。disable/全局撤认证/安全重置/身份迁移同事务升 epoch+持久失效事件；正常刷新/JWT 到期/登录退出不撤持久输入，单安装解绑只撤相关代次/grant。跨单按 issue 有序游标、有限重试收敛，快速禁用恢复不复活旧身份。

| 模式 | 人类检查 | 运行身份 |
| --- | --- | --- |
| password+managed | 有效账号/密码会话/中立 epoch；admin 仍限管理域 | 真实 installation/binding/namespace 版本及 supervisor |
| password+legacy runtime | 原登录不变 | 合法 managed 或独立协作 enrollment，不放宽 password-only 注册 |
| legacy JWT/PAT 人类+legacy runtime | 可信人类/有效账号/中立 epoch，agent actor 不冒充 | 持久 key challenge enrollment，不借管理注册/宽 token |
| 未兼容/未注册端 | 人类可查看/评论/流转 | 不发 permit；已授权目标可存 waiting，显示升级/注册状态 |

collaboration_runtime_enrollment 固定 deployment/owner/execution workspace/runtime/installation UUID/public key/binding epoch/revoked_at。实际 owner 用现有人类凭据发起，本机持久私钥完成一次性 challenge（绑定 ID/key/到期时间）。重注册/换 key/迁移升 epoch、撤旧 grant，不换未结 run 的历史 proof 身份。managed/enrollment 显式 tagged union，不拼可空字段或用 NULL/0/daemon_id 假装版本。无新登录类型或新增管理员权。

业务 token 绑定 issue/session/group（含其 command/target/grant 集合）/run/实际 grant/各代次，每次动态重验，写事务再验；默认拒绝未列 API，仅本单最小上下文、本次过程/结果、E03 派发评论及 squad activity 记录。不得 fallback 普通 mat_/PAT/JWT/MDT、owner 成员权、connected-app/remote-MCP 宽 broker。其他机器只得 R53 最小引用；provider 不持 supervisor/注册凭据。终结能力按 E04 独立保留，API ACL 不等于 OS 沙箱，也不保证恶意宿主。

## E08. 入口适配与验证

| 入口 | 契约 |
| --- | --- |
| comments/trigger-preview、comments | 预览无副作用，comments 唯一产根输入/通知；旧 comment/@all/@squad 监听跳过协作单，取消/未授权目标不执行 |
| edit/delete/reply/resolve/reaction | 不重触发或取消旧 command；新增有效人员才通知，保留墓碑/附件。仅回复/无目标不执行，R57 无执行/通知 |
| squad 派生/普通子任务 | 仅本组存活 run 的 @名册成员评论计为 E03 派发；briefing 的 `--status todo` 子任务指派对协作单返回 collaboration_subtask_unsupported，队长记 failed；R51 普通子任务隔离，R56 preview/manual/agent 仍在 capture/复制前拒绝 |
| targets/consents、control、grant revoke、target runtime-binding | 当前 scope/版本与 E01/E05/E07；不暴露匿名 commands 或旧 session/stop 旁路 |
| issue/batch/status/assignee/triage/autopilot | 人类动作共用服务，其余拒绝；无自动根输入/AI 验收，不改 admission/责任绕过 |
| enqueue/claim/start/finalize/retry/orphan/GC | generic selectors 排除协作 purpose，专用完整 E01；旧完成/cancel/reconcile 不清组屏障，手工重做需新人类 command |
| Plugin/CLI/bridge/HTTP/WS | 同一用途/范围；机器 comment 不派新根，plugin_event_bridge/IM 不带私有正文/评论/附件，工具不继承宽 broker |
| 整 agent 停止/运维/资源清理 | 反查全部受影响目标/组并保存屏障，来源记录独立于 engine task，不伪造 proof |

[研究索引](design-contracts.md)保留旧源码清单。notification_listeners、comment source-context、quick-create/purpose fallback、普通 squad 子任务唤醒均需逐入口隔离，不能只改 UI；普通任务行为不变。

未来验证至少包含：

- Q01：根/成员 permit 前 Stop/revoke/close，无 daemon 只结清已证未启动者，不提前清全组。
- Q02/EX01：burn/fork/PID/reap/seal 崩溃、另一 daemon、迟到 fallback；setsid/双 fork/stdio 先关/PID 复用/Windows Job 失败，不重复 fork/伪造 proof。
- Q03/A04/A05：admission、installation stopped、managed/legacy 组合、challenge 重放、缺版本、私有 owner 不一致、rebind/teardown/proof 竞争。
- Q04：Resume All/target→commit 与 freeze 竞争；旧 scope/group/config 延迟到新工作/enable。整单级失败与 scope=target 拒绝不改状态；Resume All 中 lane 级失败只让该 lane 保持暂停，原 key 重放原回执。
- Q05：成员 outcome/proof 乱序/丢失/撤权/重放，父 proof 不清子屏障，收尾不等私有尾部。
- M01：双调度 worker、跨 lane 并行/同 lane 批次顺序/typed 去重；机器容量=1 的交接/成员/continuation、两队共享成员/排他目录、评论派发去重与续作合并，超额槽位/恢复指针串用。
- M02：目标 Stop 不干扰无关组，issue Stop 覆盖新增目标/迟到派生；Resume All 时未确认成员只让所属 lane 保持暂停、无效 grant 只让该目标保持 hold，其他 lane 与整单照常恢复；已移除目标的遗留屏障不阻止解除整单暂停；全局 active 时无关目标可独立恢复。M06/M07：@A+@S 合并、忙碌时批次合并、单条移出与整批停止。
- M03：跨 owner 完整 consent、配置变化不扩旧权、根作者/owner 失效跨组扇出；机器旧凭据/评论不派新根。
- 数据/内容：事务失败/未知 commit 不重复根或通知，编辑不改快照；附件受控，执行空间清理保留来源历史，并发索引不清未知屏障，M10 只删旧单会话数据。

归属见[实施计划](../implement.md) P4/P5/P8；用确定性 DB 同步和测试创建的 fake 原生进程，不靠 sleep/单个 ProcessExited mock。默认不查找或执行用户 AI CLI、不访问付费账号。本文没有运行上述测试，不表示实现授权或验收通过。
