# 共享执行的准入、启动和收尾契约

2026-10-17：M01–M03 的完整执行目标，承接[设计](../design.md)及[数据契约](data-contract.md)。已落实的目标/组/停止与拒绝恢复基础见[当前代码规范](../../../spec/server/collaboration-targets.md)；真实调度、许可/proof、正向恢复和容量验证仍未交付。

## E01. 分层准入

拟用 CanAdmitCollaborationRun(tx, identities, expected) 统一验证；task 的空 issue_id 不代替来源准入，active 也不等于资源可运行。

| 边界 | 必须成立的事实 |
| --- | --- |
| 新评论及根 command | 普通评论保留当前有效人类来源成员或参与者的访问依据；只有有效参与者可产生根 command，且来源须非终态、typed target 属于本单并已完整授权。按 input_event/target 去重，固定目标/作者/快照；暂停、离线、忙碌或维护可记录 waiting，不假报开始 |
| R45 取消 waiting | 当前有效参与者且为作者/creator/handler，同单同目标、仍 waiting、版本匹配；不依赖 grant、在线状态或执行 gate，不隐式升级成 Stop |
| promote 根 command | 本目标最早有效 waiting，target 无未结 group；全局与目标均 active，目标配置/grant 合法。原子 command=in_flight、group 占位及首个 run/派发意图；其他目标不受该 FIFO 占位限制 |
| claim/finalize/首次 permit | 完整运行谓词，run/组/目标身份一致，实际资源槽位已取得；既有 group 中的成员/continuation 不再争抢根 command 位置 |
| 组内派生 | 当前获授权 squad run、同一未封闭 group、名册允许的成员/角色及明确依赖，幂等键有效。持久意图不等于已启动；不建普通 issue 子任务或其他目标根 command |
| 后续 provider attempt | 同 run 未 sealed、前次 attempt 已证明结束、完整运行谓词仍成立；独立 attempt nonce，不复用许可 |
| Resume（scope=target/issue） | E05 的证明/授权/版本全通过，gate FOR SHARE 至提交；恢复不占槽位，缺容量/离线只导致合法等待 |
| Switch | creator/handler；明确目标 paused、其旧 group/run 全部结清、新配置与每项资源 consent 合法；不换 typed target，不发 permit，仍 paused |
| 业务 token 读/写 | 绑定当前 issue/target/group/run/实际 grant 的动态能力白名单；不能退回人类 read basis |
| Stop/强制失效/close | 验人类控制权或可信系统失效原因；不要求 grant、在线端或执行 gate 仍有效 |
| 终结 proof | 只依固定历史 run/installation/token/账本身份，不要求当前账号、grant、runtime/task 行存活 |

完整运行谓词在锁内重读：

1. 来源 issue/协作关系存在，admission_status=not_required/accepted，类别已识别且非 done/cancelled。新建为正式 not_required；blocked 不等于 triage 未接纳，旧 triage 不得改变协作 admission。
2. session 与 target 均 active；group 仍是该目标当前根 group、未被停止/封闭。根作者账号/参与/lifecycle 代次，以及 run 的 target_epoch、config_version、grant/resource 代次均匹配。重新加入、重开、禁用后恢复不能复活旧身份。
3. 该 run 的实际 agent/runtime 属于 execution workspace 且绑定一致，资源未归档/解绑；grant 对应本 target/group 冻结名册中的该资源。授权人是实际 runtime owner 并具备 agent 的既有 invoke 资格；公有资源仍验 namespace/invoke，私有 owner 一致约束不放宽。
4. managed installation/binding/owner/workspace/runtime/epoch 匹配且 admission=accepting；legacy 端须有 E07 验证过的独立协作 enrollment。NULL/0/binding 缺失不视为通过。
5. 部署 gate 开放，协议/provider/OS containment 合格；运行端在线健康，实际 agent、机器及排他工作目录容量允许。首次许可还绑定 run fence、claim_generation、launch/attempt nonce；成员/continuation 同样检查，不继承队长占用或宽权限。

容量按实际 daemon，不按其 runtime 分摊增额；agent 限额与机器/目录预约均须满足，先取槽再 claim，真实结束/清理后释放。DB waiting 不释放槽；A 缺容量不阻塞其他目标，等待者仍保序并获公平调度。

installation 停止/gate 冻结后仅观测/终结旧许可，旧恢复例外不允新 provider/成员/continuation。排他目录及私人供应商 session 不因目标类型不同而共享，见 E07。

## E02. 锁入口、幂等与控制范围

*Collaboration*InTx 借同一 pgx.Tx；网络准备在事务外，预读 ID 仅作锁提示，锁后重验。共同来源顺序：issue → session → target（UUID 有序）→ participant/grant → command/group → run/delegation。保留原目录/I1/账号/installation 前置 fence；提交串行不等于执行全单串行。

| writer | 前置锁 | 来源写入与交汇 |
| --- | --- | --- |
| 创建 | 账号 fence、source workspace KEY SHARE、创建操作键、draft | issue/参与/session/事件原子；与 LockWorkspaceForDelete 互斥 |
| edit/invite/transfer/remove/leave/close/reopen | 相关账号有序 fence、目录/I1、workspace | 共同聚合顺序，锁后验角色/版本/来源选项。R46 直接写参与/责任；执行副作用封闭相关 group 并写停止意图。R49/R50 另验来源成员，迭代用借用事务；R51 普通子任务仅把协作单当引用 |
| 新评论/根输入 | actor fence、workspace、目标资源身份前置锁 | 聚合锁内验访问/参与/名册，原子保存评论、逐目标 command/拒绝、通知/回执 |
| R43/R57 | actor fence、workspace、当前 read basis | 共用 issue 锁，再验同单/作者/版本；清旧解决+设新解决原子，不锁运行资源 |
| R45 waiting 取消 | actor fence、workspace | issue/session/target/participant/command，与 promote 串行；不改控制版本或 active_group，不锁 grant/gate |
| promote/claim/finalize/permit/派生 | managed user→namespace/installation 或 enrollment；gate FOR SHARE；workspaces UUID 有序 | 共同聚合顺序，agent/runtime/task 后置锁 NOWAIT；空 task.issue_id 不代替 lock_issue_execution |
| Resume/任何 active 写入 | 账号/资源前置协议、gate FOR SHARE+epoch | 锁来源聚合及完整目标集合，重验 E05，gate 持至提交；集合变化重试或 stale_scope |
| target 组织/consent/revoke/switch | organizer/owner、namespace/installation、workspaces 有序 | 锁后验 creator/handler 与实际 owner，原子配置/grant/事件，后置资源 NOWAIT |
| R38/R40 admin | LockPlatformAdminMutation；admin/creator/handler/successor 有序锁及组织范围 | 目录/I1→来源聚合；双禁用/目标/版本再验，生命周期/责任/审计原子，不放宽资源权限 |
| account 撤销 | 原治理锁，同事务 epoch+持久失效事件 | 不持用户锁阻塞多个 session；issue UUID/NOWAIT 分批收敛，旧 epoch 已阻止新能力 |
| 资源 teardown | 原资源锁前枚举受影响目标；或反向 source 全部 NOWAIT | 整笔重试，不持 resource 等 source；保存撤 grant/停止/屏障后才清 engine |
| source workspace delete | LockWorkspaceForDelete | 清理前查全部协作记录，含已结束；存在即拒绝 |
| proof | 历史 token/安装身份，不调用当前授权 wrapper | 只锁来源 issue/session/target/group/run 与 proof/operation；不要求资源行存活，engine 投影 NOWAIT 或提交后幂等 |
| freeze/enable | 短事务独占 gate | 先关再按 E06 扫描，不持独占 gate 等来源对象 |

与旧 workspace→agent→issue→runtime 或 task→issue 锁序相交时后置 NOWAIT；同类对象 UUID 有序。NOWAIT/40P01/40001 整笔回滚，最多三次有界抖动重试，不重复外部副作用或部分提交停止。

Idempotency-Key 按操作 scope/actor/request 唯一，payload_hash 含 control scope=target/issue、业务/控制/配置版本、target_set_version 及观察到的 active_group/generation。授权先于回执；同 key 重放原成功/拒绝，改 payload 拒绝。未知 commit 只查原 key，不自动配新版本重试；退出者仅可查本人无内容收据。

Stop 比较 typed scope/control_version/target_set_version 与观察到的 active_group/generation：新根 group 或目标替换使请求失效，返回 stale_execution/stale_scope；原 group 已结束且无替代者可暂停并返回 already_ended。成员扩展与停止共锁，按当前账本纳入原 group 的全部后代；不能只停预览中已出现的 run。评论/进度/入队不改控制版本。多个 actor 分别留操作记录，保留首个强制处置根。

## E03. 根队列、小队交接与未许可收尾

command：waiting → in_flight → succeeded/failed/stopped，未执行取消为 cancelled。promote 原子创建唯一 group；active/waiting_children/ending group 持有该目标 FIFO 位置，完整终态才释放 active_group_id，其他目标可独立推进。

小队使用非阻塞的持久交接：

1. 当前队长 run 提交具幂等键的成员意图与 continuation 检查点；服务端验同组/名册/角色/根作者代次/无环依赖。返回“已记录”，不等待成员；父 run 的阶段固定完整子工作集合，不从未上传日志重建。
2. 队长结束此阶段；只有交接结果已持久且其 E04 sealed proof 已验证，子工作才可启动。进程退出及本地清理后释放真实 agent/机器/排他目录槽，group 保持 waiting_children，不占运行槽。
3. 成员只争实际资源，不进入其他目标的外部 FIFO。再次派生也先持久意图、结束自身 run 并证明，再运行后代；依赖仅在同组，不等待另一外部根队列。
4. 阶段成员结果及证明齐备后，账本至多创建一次 continuation，step_key=group/父 run/阶段；新 run/permit/nonce，原 human author/参与/lifecycle 不变。已 seal 的 run 不重开，也不另建外部输入。
5. 最终队长结果与全部成员结清决定组终态。失败成员可交合法 continuation 汇总，服务器不隐式重试。Stop/revoke/close 封闭扩展，迟到成员/continuation 拒绝，已记录未启动者按处置结算。

必须验证同机器 MaxConcurrentTasks=1。仅拆 FIFO、改 DB waiting 或忽略存活队长计数不能释放真实容量；不支持此交接的 provider 不声明共享小队能力。旧实现证据：daemon.go:4811–4816 先取机器槽再 claim，4905–4917 在 handleTask 返回后释放；agent.sql:1879–1885 明确等待也计容量（均位于 server/internal/daemon/、server/pkg/db/queries/）。

R45 与 promote 同锁：取消先胜则跳过；promote 先胜返回 command_not_waiting，即使未许可也不暗升为 Stop。组内工作不是 R45 外部 waiting command。

任一 run 若从未许可，服务端可原子撤派发/claim，确认 authorized_attempt_seq=0、无 started、无矛盾 running/success 证据且 prepare 状态已知后，写 never_launched_proven 并 settled。不能等待未领取的 daemon，也不能因一个成员未启动就结清整个 group。

| 事件 | 处置 |
| --- | --- |
| Stop/revoke | 未启动 run 安全结清，可能存活者保留屏障；根 group 标 stopped，目标 paused，外部 waiting 保留 |
| remove/leave/close | 取消该作者未执行根输入；已运行组停止。close 覆盖全部目标/队列，封闭扩展，保留首个 stopped/cancelled 原因 |
| 永久 prepare 错误 | run failed；只证明本次从未许可，资源/能力失效暂停目标，组仍等待其他工作收尾 |
| 投递暂失败/结果不明 | 重放同一 step/run/nonce/claim，不新建重复执行 |

任意已发 permit、许可提交不明或矛盾证据均须原安装 E04 proof。迟到启动因 settled/封闭/失效代次拒绝。v1 无服务端隐式失败重试；合法 continuation、provider attempt 各自记账，不能借前者重做 stopped command。

## E04. 每个 run 的启动账本和真实容量

supervisor 使用跨 daemon 进程的 OS 互斥锁、私有文件及原子写+fsync；进程内 mutex 不够。每 run 固定 deployment/installation/target/group/fence/launch nonce；阶段单向推进：

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

## E05. 结果、两级停止和原子恢复

run 独立保存 business_outcome、forced_disposition、proof_state、sealed_attempt_seq、recording_incomplete。进度为 prepared/dispatched/preparing/launch_authorized/running/ending/settled，证明为 never_launched_proven/may_be_live/exited_proven/unknown。已发 permit 未报 started 仍可能存活；task 终态/租约/心跳/401/超时/PID 缺失不解屏障。

| 到达顺序 | 处理 |
| --- | --- |
| outcome 无 proof | 留结果，run/根 group 不结清；无关目标可继续 |
| outcome+proof | 结清该 run；强制处置优先，否则 success/failure 或持久交接结果；组终态另查全依赖账本 |
| proof 无 outcome/交接结果 | failed/outcome_unconfirmed，标记录不完整；exit code=0 不算业务成功，不无限等日志 |
| Stop/close/revoke 在先 | 固定首个强制根，late success 不覆盖，后来原因追加 |
| proof 后 Stop | 不改历史成功，按当前范围版本暂停，不套到新 group |
| 重放/乱序 | event/receipt key+digest 幂等，不同 digest 拒绝，已结结果不被晚到正文改写 |

group 终态要求扩展已封闭、全部成员/continuation 意图有确定处置、每个 run 有合法终结证据；按强制根或最终业务结果结算，原子推进 command 并清匹配 active_group_id。父 proof 不清子屏障，证明到达不解除暂停/closed/维护。

| 动作 | 原子效果 |
| --- | --- |
| Stop target | 目标 paused，封闭当前 group，撤新业务/启动能力并保存全后代停止意图；waiting 保留，其他目标/全局不变 |
| Stop issue | session paused，暂停全部目标并封闭/停止其组；新增目标初始 paused 且受全局约束 |
| remove/leave 作者 | 跨目标取消其 waiting，按根 human author 停止相关组并暂停目标；其他作者 waiting 与无关目标保留 |
| revoke/owner 离开 | 撤 grant，暂停/停止所有使用它或受派生依赖影响的目标/组；无关目标不变，waiting 保留 |
| 普通转交/交回/验收交接 | 执行状态不变 |
| Complete/Cancel | 来源终态/lifecycle、session closed、全部 target paused、waiting cancelled、封闭/停止全部组及审计同事务 |
| Reopen | 新非终态/lifecycle、session paused；不清屏障或复活旧 command/group/grant |

Resume target 带目标 control_version/group_generation、资源配置版本与 expected_execution_epoch。只验该范围证明/授权，但 session paused/closed 优先拒绝；scope 内未结 group 或 run/指针矛盾为 stop_unconfirmed。全局 active 时无关目标的未知 proof 不阻止本目标恢复；拒绝不保存未来意愿。

Resume All 严格原子：

1. scope=issue 请求绑定完整目标集合、target_set_version、全局/目标控制、active_group/generation、配置/gate 版本；锁内重枚举，漂移拒绝，不缩成合格子集。
2. 验本单全部应收尾组/run（含已撤目标遗留屏障）和所有当前目标/grant/完整名册。全局暂停后的所有可能运行者须先有证明；全局已 active 时，未被停止的健康运行保持原样，不重启。
3. 任一 stop_unconfirmed、授权/配置/协议无效或其他验证失败，session/所有 target 均不变；持久拒绝及逐目标原因，无部分/延迟恢复。
4. 全部通过才同事务 session=active、覆盖目标 active；不重建已有 group。临时离线/缺容量只导致合法队列等待。无目标时仅解除全局暂停，新目标仍初始 paused。

旧拒绝 key 在 proof/重授权/移除目标后仍拒绝，须新的人类明确请求。完整结束的普通失败组可继续 active 目标下一条，stopped/cancelled 不重试。另建目标、重开、换资源、管理员声明或超时不绕过未知证明。

proof 严格解码，只含固定身份、sealed attempt 边界、未启动/退出枚举和有限退出信息，无正文/附件/路径。撤业务能力后不借终结通道补私有日志；来源已入库过程/用量保留，缺尾部标不完整，合法收尾不等日志。

## E06. 部署冻结与恢复

每部署持久 collaboration_execution_gate：enabled、epoch、freeze_operation_id/progress，独立于 creation_enabled。promote/claim/finalize/permit、派生/continuation 准入及所有 session/target active 写入持 FOR SHARE 至提交；冻结短事务 FOR UPDATE 关 gate、升 epoch，不持独占锁扫描来源对象。

先拒新许可，再按稳定顺序持久执行每单 Stop issue 并记游标，覆盖所有目标/远端成员，不能只查旧 active_run。重启继续原 operation。评论、合法 waiting、人类流转、Stop/proof/历史仍可用，界面不能把未扫描的 active 误报恢复。

Resume 比较 expected_execution_epoch；冻结和 enable 都提升 epoch，旧意图延迟到重开仍 stale_admission。扫描完成、无遗漏活跃队列且全部 writer 兼容才可 enable；enable/proof 不自动写 active。

关闭创建不关闭历史/收尾。旧 writer 须先停止，gate 无法约束不理解协议的旧代码；带协作数据不得回退旧二进制。数据迁移按数据契约，不删未结记录解除屏障。

## E07. 目标授权、认证及运行身份

M03 菜单只列本单完整授权 typed targets；busy/paused/offline 单独显示，不等于 revoked。creator/handler 组织目标配置，实际 owner 逐项 consent；组织权不代替 invoke/owner 权。owner 须已合法参与本单，否则先明确邀请，@或 grant 不自动加人。

配置固定队长/成员角色、实际 agent/runtime/installation 与 grant；授权完整才供提及。成员、机器、owner 或执行权限规则变化需新 config_version，逐项取得真实 owner 对新版本的 consent，不把旧版本或名称相同视为授权。group 在 promote 冻结配置，每次派生/permit/业务调用重验 current target/config/resource/根作者代次。漂移暂停/停止受影响组，保留原 run/proof 身份，不向旧组加入新人或复活旧组；waiting 原目标保留，明确恢复后用合法新配置建下一组。纯显示名变化不换执行身份。

originator=accountable=根 command.author_user_id；另记实际 agent、parent/delegator、资源 owner、供应商账号。派生不是机器新根输入，不冒充 owner。供应商恢复指针绑定 provider/agent/target/group/角色/资源代次，仅前次可信结束且匹配时使用；不同目标/角色不共享私人 session，换环境重建，文件只从已提交 Git/明确附件准备。

中立 collaboration_account_fence(user_id, epoch) 首次参与/授权原子建立；password 另验真实 session_version。disable/全局撤认证/安全重置/身份迁移同事务升 epoch+持久失效事件；正常刷新/JWT 到期/登录退出不撤持久输入，单安装解绑只撤相关代次/grant。跨单按 issue 有序游标、有限重试收敛，快速禁用恢复不复活旧身份。

| 模式 | 人类检查 | 运行身份 |
| --- | --- | --- |
| password+managed | 有效账号/密码会话/中立 epoch；admin 仍限管理域 | 真实 installation/binding/namespace 版本及 supervisor |
| password+legacy runtime | 原登录不变 | 合法 managed 或独立协作 enrollment，不放宽 password-only 注册 |
| legacy JWT/PAT 人类+legacy runtime | 可信人类/有效账号/中立 epoch，agent actor 不冒充 | 持久 key challenge enrollment，不借管理注册/宽 token |
| 未兼容/未注册端 | 人类可查看/评论/流转 | 不发 permit；已授权目标可存 waiting，显示升级/注册状态 |

collaboration_runtime_enrollment 固定 deployment/owner/execution workspace/runtime/installation UUID/public key/binding epoch/revoked_at。实际 owner 用现有人类凭据发起，本机持久私钥完成一次性 challenge（绑定 ID/key/到期时间）。重注册/换 key/迁移升 epoch、撤旧 grant，不换未结 run 的历史 proof 身份。managed/enrollment 显式 tagged union，不拼可空字段或用 NULL/0/daemon_id 假装版本。无新登录类型或新增管理员权。

业务 token 绑定 issue/session/target/group/run/实际 grant/各代次，每次动态重验，写事务再验；默认拒绝未列 API，仅本单最小上下文、本次过程/结果及 E03 派生。不得 fallback 普通 mat_/PAT/JWT/MDT、owner 成员权、connected-app/remote-MCP 宽 broker。其他机器只得 R53 最小引用；provider 不持 supervisor/注册凭据。终结能力按 E04 独立保留，API ACL 不等于 OS 沙箱，也不保证恶意宿主。

## E08. 入口适配与验证

| 入口 | 契约 |
| --- | --- |
| comments/trigger-preview、comments | 预览无副作用，comments 唯一产根输入/通知；旧 comment/@all/@squad 监听跳过协作单，取消/未授权目标不执行 |
| edit/delete/reply/resolve/reaction | 不重触发或取消旧 command；新增有效人员才通知，保留墓碑/附件。仅回复/无目标不执行，R57 无执行/通知 |
| squad 派生/普通子任务 | 仅当前组合法 run 可提交 E03 名册内工作；R51 普通子任务隔离，R56 preview/manual/agent 仍在 capture/复制前拒绝 |
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
- Q04：Resume All/target→commit 与 freeze 竞争；旧 scope/group/config 延迟到新工作/enable。失败不部分改状态，原 key 永不自动成功。
- Q05：成员 outcome/proof 乱序/丢失/撤权/重放，父 proof 不清子屏障，收尾不等私有尾部。
- M01：双调度 worker、跨目标并行/同目标 FIFO/typed 去重；机器容量=1 的交接/成员/continuation、两队共享成员/排他目录，无环/超额槽位/恢复指针串用。
- M02：目标 Stop 不干扰无关组，issue Stop 覆盖新增目标/迟到派生；任一未确认成员或无效 grant 阻止 Resume All，所有 gate 不变；全局 active 时无关目标可独立恢复，旧拒绝不随资源修复变成功。
- M03：跨 owner 完整 consent、配置变化不扩旧权、根作者/owner 失效跨组扇出；机器旧凭据/评论不派新根。
- 数据/内容：事务失败/未知 commit 不重复根或通知，编辑不改快照；附件受控，执行空间清理保留来源历史，并发索引和升级映射不清未知屏障。

归属见[实施计划](../implement.md) P4/P5/P8；用确定性 DB 同步和测试创建的 fake 原生进程，不靠 sleep/单个 ProcessExited mock。默认不查找或执行用户 AI CLI、不访问付费账号。本文没有运行上述测试，不表示实现授权或验收通过。
