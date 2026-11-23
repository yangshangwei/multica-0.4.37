# F3 单一父任务预算根：最小首包方案

日期：2026-11-13。状态：研究与设计提案；**未实现、未运行产品验收、未获 F3 实施授权**。本任务继续 `in_progress`。用户已确认 D1“长期绑定父事项”；该回答及“继续”只推进本轮设计。

阅读顺序：[source map](f3-source-map-20261113.md) → 本方案 → [验收矩阵](f3-acceptance-matrix-20261113.md) → [实施顺序](../implement.md)。M1-S1 和 F2 已交付记录保留；F4/F5、其他 M1 入口与独立真实运行不在本首包。

## 1. 用户结果与首包边界

让当前有权的人能回答：这个父任务及其已绑定子任务累计有哪些执行、哪些支出已有可信回传、哪些仍是估算或未知、哪些预留尚不能释放，以及原请求究竟是否已记账。答案来自可重放的最小账目，不依赖模型总结、当前树形结构或小时聚合。

| 切片 | 纳入 | 明确不承诺 |
| --- | --- | --- |
| F3-S1：observe 最小首包 | 单个普通父任务的持久根及成员绑定；从明确启用点向前记录；执行/调用身份；幂等用量事件与更正；影子预留、已知实际、估算、未知；当前人工管理/读取和原请求恢复；API/CLI 最小可检查结果 | 不以金额、未知费用或关闭状态拦截既有业务执行，不自动取消、不改变调度排序；不保证最终费用不超额 |
| F3-S2：未来 enforce | 对已证明支持的执行链，在最终领取事务预留并复核；每次新增实际调用受准入约束；取消/失联的可靠收敛；全入口无绕路证据 | 只有估算上限时，原子预留不能保证提供方最终收费不超额；不把 observe 开关改名当完成 enforce |
| 后包 | 项目/小队多根预算、跨根转移、独立 collaboration/外部执行、更多使用面 | 不嵌入 S1，不把 F4 调度和 F5 治理提前标完成 |

首包推荐只提供人工 API/CLI 管理和查看，无新 Web/Desktop UI；保留现有用量页面。只支持普通任务可证明的执行归属；无 `issue_id` 的 run_only autopilot、独立 collaboration/host run、插件外部调用不是自动纳入的“子执行”。已知排除的机制列为 `not_applicable`；同一纳入执行中无法辨认的调用是 `coverage_gap`，不能排除求完整。

S1 的预算错误与执行结果分开：预算金额不产生业务拒绝；错误身份、版本冲突、损坏记录及不能提交的事务仍须拒绝相关预算操作，不能伪装成功。领取内不可分的身份绑定写失败可使该领取失败，这是持久化失败，必须单列，不能记成“额度不足”。现有 task 完成/失败不会因费用未确认改判业务结果。

## 2. 当前事实与设计依据

- `autopilot_quota` 是 Cloud 决定的工作空间/时间段**次数**配额。create_issue 创建成功即消费一次，run_only 完成消费、失败释放；已消费次数不退款。它有同事务、CAS 与有界恢复模式，没有父任务金额账本。
- `task_usage` 以 `(task_id, provider, model)` 覆盖更新；主 writer 在 `handler/daemon.go`，`TaskService.CaptureTaskUsage` 只是指标转发。现有批次可能部分失败仍返回成功，没有逐事件确认。
- 回传只有计数和成本，缺原始调用、claim、计量范围、顺序、更正和完整性合同。同一个 task/claim 中还可能有多次实际调用；`claim_generation` 不能替代 invocation 身份。
- 当前 `cost_source=authoritative` 不能直接升级为 F3 actual：例如 Reasonix 的 `estimatedCost` 进入同一成本字段；零金额/缺金额、零 token 正费用、合并时部分未知也有信息损失。详细事实及静态测试出处见两个分源研究。
- archived M2 只提供“同根、整数、实报/估算/未知、原子预留、先观察后强制”的设计方向，不证明这些合同已实现。

以上是源码事实；下文名称、数据模型、端点能力和状态均是待实施提案。首包不修补旧统计以冒充新账本，不以历史测试数字作为 F3 验收。

## 3. 预算根、成员和执行身份

### 3.1 已确认 D1：长期绑定父任务

根的业务锚点是启用时规范的 `(workspace_id, root_issue_id)`，其中 `root_issue_id` 是 ordinary/formal 普通父任务 UUID，不是人类编号、一次 task、provider session、autopilot_run、项目或小队。服务端另发稳定 `budget_root_id`。一个父任务终身至多一个根；关停、重开、删除、换项目均不释放这个唯一身份槽。

已绑定普通子孙任务持久保存唯一成员关系；它们的直接执行、分派给不同智能体的执行、服务端重试、新 task UUID、重领 generation、同 claim 的多次 provider 调用都回到原根。root 层汇总不是一次新的费用；父任务自己的独立调用可以计费，父任务转述子任务总数不能再次计费。

| 事件 | 归属与边界 |
| --- | --- |
| 启用根 | 当前人工明确操作；冻结 `activated_at`、范围/计量版本和成员集合。已有用量不自动导入为 actual；存在历史或在途旧协议执行则列出启用前/不可分基线缺口 |
| 新建普通子任务 | 在真正创建/绑定事务中继承已绑定父任务的根；嵌套后继续继承。不能等消费时才递归查询可变 `parent_id` |
| 重试、重领、重开 | 原根、原累计额度不变；新的真实调用有新 invocation，旧调用的 actual/unknown 不被覆盖；重开业务状态不自动调额 |
| 子任务移出、改父、清空父关系或换项目 | 已绑定关系保持，之后执行仍归原根；展示当前位置与原预算归属的差异；绝不退款。新父根不同则记录冲突，不能同时归两根或覆盖原根 |
| 把已有根或已绑定成员移入另一根 | 不自动合并/转账/创建嵌套额度。预算管理操作拒绝矛盾绑定；S1 执行记录 `would_refuse:root_conflict`，S2 必须拒绝新准入；解决跨根转移另案设计 |
| 普通子任务/task 删除 | 留最小成员/执行 tombstone 和既有事件，不能通过级联清掉消耗或释放未知占额；剩余子孙仍沿已保存的根 |
| 父任务删除 | 根变为 tombstoned，停止新增预算许可，保留原身份/义务；不能给同一已绑定成员补开新根。S1 若旧业务链仍运行，继续记录其原根及 `would_refuse:root_deleted` |
| 根/成员跨工作空间 | 不支持迁移同一根；旧事实仍属于原工作空间。新工作空间的 caller 不能凭当前 parent/project 搬走旧账目；已知迁移来源必须显式冲突/待处理 |
| 真正无关联的新普通任务 | 可由有权人另建独立根。首包不声称识别“复制描述再新建”的语义逃逸；已知 retry/child/source 身份不能伪装独立任务 |

首次成员收敛必须防止两个并行“父/子各建一根”、新子插入和改父的 phantom。建议预算成员操作复用工作空间级专用 advisory fence，按现有锁序在同事务检查 ordinary/formal 资格、遍历边界/循环与唯一根，再保存成员；所有相关创建/改父/删除 writer 必须参加。缺 writer 证据时 coverage 不完整，不能宣称具备 S2 防逃逸能力。旧 task 不能因客户端自报 `root_id=null` 逃过服务端成员查找。

`prospective_from_activation` 只表示首包精确归集的起点，不把整个父任务历史成本改写为 0。已知历史未纳入金额展示为 `pre_activation_unknown`；“启用后已知值”与“父任务终身总额”分开。后续 enforce 必须另解决基线义务，禁止删除根再启用来换一个起点。

### 3.2 何时结束：准入状态与费用结清是两条轴

建议根有 `open / closed / tombstoned` 准入状态，另有 `unsettled / settled_as_of_revision` 结算状态。业务 done/cancelled/in_review 不自动关闭或结清根。

- 当前有权人以原 key、预期 policy revision 和理由显式 close；close 不撤销已有调用、不释放钱、不擦除账目。S1 仅产生“若强制将拒绝”的观察，S2 才停止新准入。
- ordinary 父任务仍存在时，可由同样有权的人明确 reopen **同一个**根，递增 policy revision，保留原 limit/actual/reserved/unknown。业务任务重开本身不做这次预算 reopen。
- 只有全部纳入调用有终结/最终用量证明、无未决预留与 unknown，才显示该账目 revision 已结清。根可以 closed 但未结清；新的可信更正仍追加到原根，不能把“已结清”当永不接收证据。
- 父任务删除的 tombstone 不提供自动恢复/换根；未来若要让其存活子任务继续在 enforce 下执行，需要另审承接原义务的转移方案。首包明确保留此限制。
- 推荐保留最小经济事实与 tombstone，不保存任务正文/模型输出；工作空间被删则拒绝公共读和新上报，保留 unresolved 事实，不记退款。具体留存/物理清除为 D2 待审，不能照搬 F2 owning-report 清理合同。

## 4. 计量、价格与未知

### 4.1 字段的含义

| 概念 | 定义 | 不能推出的结论 |
| --- | --- | --- |
| `actual_known_ticks` | 通过受信适配器、唯一计量范围和已接受版本证明的 USD 回传值；允许明确报告的 0 | 不代表账单已支付或最终对账，不包括只有估算的成本 |
| `estimated_ticks` | 由冻结价格/计量规则计算，或来源明确标为 estimate 的值；单独显示 provenance | 不能更名为 actual，也不证明是调用硬上限 |
| `reserved_ticks` | 尚未结清调用保留的**剩余**影子额度；已转 actual 的部分不再在 reserved 中重复出现 | S1 只是观察，不能宣传为实际阻断或已付费用 |
| `unknown_hold_ticks` | 由原 reservation 转来的、因结束/失联后证据不足仍需保留的数值占额；可能为 null | 是原预留残额，**不是未知实际费用的数值**，不能称未知成本等于此值 |
| `unknown` | 带原因、范围和完整性的集合；缺金额、来源不明、部分覆盖、计量重叠、序列缺口、执行结果不明分别表达 | 不能折成 0、排除项或一个假定事件数量 |

已知金额、估算金额和未知集合分别输出。`confirmed=0` 可以表示已证实部分为空，但必须同时标记总额 unknown；明确 0 费用需要 presence + 来源依据。金额缺失、币种不支持、解析失败和 0 是不同状态。

采用当前代码使用的 USD tick 单位 `1 tick = 10^-10 USD`（`10^10 ticks = 1 USD`），Go/DB 使用有界整数；对外预算金额和大计数采用规范十进制字符串，拒绝小数、指数、溢出/截断，不经 JS number 或 float64 往返。token 也是非负整数，并记录 input/output/cache-read/cache-write 的提供方口径，不能把输入已含 cache 的口径再加一次。

价格快照至少含 provider、原生/规范 model、adapter/normalizer 版本、价格来源、price version/digest、有效期、各 token 维度的整数费率/分母、币种、舍入版本。服务端决定版本；调用不能自报“更便宜的最新价格”。估算按冻结费率用安全整数/有理数运算，预留向上舍入；提供方原生十进制按版本化明确规则转换，保留原精度与舍入差。旧事件重放不用当前价格。实报值不需要借最新价格“修正”；改价只影响后来的估算。汇率转换、多币种和账单对账不纳入。

### 4.2 防重复的占额不变量

对一个最初预留 100 ticks 的调用，partial actual=30 时，保留 `A=30,R=70,U_hold=0`，不能显示 `A=30,R=100`。若随后失联，则同一事务转成 `A=30,R=0,U_hold=70`，费用总额仍 unknown，不能把 A+U_hold=100 当已知实际。

最终可信证据证明该调用总成本 80 且没有未报告调用时，追加 +50 actual，终结同一 70 hold，释放剩余 20；结果 `A=80,R=0,U_hold=0`。同一收据重放不再增加 50 或释放 20。若最终为 120，必须接收 +90、记 overrun=20；不能裁剪到预算上限或拒收使账面看似合规。

不变量：A/R/U_hold 同一份额度不得重复计入；从 R 移入 U_hold 不产生可用额度；A 和非负 hold 有溢出检查；可信 correction 可使净 A 降低但不能造成负的原生总值。没有上界证明时 `remaining_ticks=null`，不能用 `limit − known A` 冒充剩余额度。

未来 S2 可在已知、可证明有界的状态检查 `A + R + U_hold + 新预留 <= limit`；所有量在同一根行锁下读改。存在无上界 unknown 或仅估算的“上界”时，推荐拒绝新准入并保留原因。即便数值式成立，也只证明准入时未超发；没有提供方/运行时硬封顶不能承诺最终 actual 永不超过 limit。

## 5. 原生事件、累计、增量、迟到与更正

### 5.1 三层身份

1. **业务归属**：服务端成员绑定决定 root；caller 不能从任意父任务借额度。
2. **执行**：task UUID + claim generation + server-issued budget attempt；重领不复用旧 attempt。`FinalizeTaskClaim` 是领取最终化，不是执行完成或费用结算。
3. **实际调用/计量范围**：每次 provider 启动/续调/失败后的新调用有稳定 invocation ID；重发同一上报保留原 ID。`meter_scope_id` 表示提供方真正计量范围（request/turn/session），与 provider/model 字符串分开。

同一 session 的累计用量可能跨多个 invocation，不能每个 invocation 都收取完整 session 累计。适配器必须证明基线和范围互不重叠；有证明时一个 meter scope 的累计差额只入账一次，并可关联多个调用。跨根 session、基线缺失或上游汇总包含下游而不能拆分时，记录 ambiguity，不同时给两根记 full amount，也不凭最大值猜账。

### 5.2 最小上报合同（提案）

版本化 envelope 保存原 batch/request key、event ID、预算 attempt/调用定位、producer epoch、meter scope、measurement mode、sequence/source revision、payload digest、adapter/normalizer 版本、observed/reported/received 时间、typed usage/cost presence 与 basis、partial/final、必要的 supersedes 和 final watermark。root、actor、workspace、claim 的有效绑定由服务端已保存事实推出。

生产者在实际调用前持久登记调用意图，回传前持久保存同一事件；收到已提交的逐批/逐事件收据才能清除待回传项。传输重试只重发原身份/内容，不发起新 provider 调用。原 key + 相同规范内容返回原收据；原 key + 不同内容返回 conflict。无法入账整批则不返回“全部成功”；首包推荐一批一个 attempt、全批原子接受，避免复制现有逐行 warn 后 200 的行为。

| 输入 | 接受规则 | 账目影响 |
| --- | --- | --- |
| `cumulative` 同 meter scope：v1=100、v2=150 | 版本/基线可证、范围相同 | 原始快照均保留；规范贡献从 100 变 150，只追加 +50 |
| 重复 v2=150 | 同身份且内容相同 | 原收据，不再追加贡献 |
| 迟到旧 v1=100 | 已接受旧版本或被新完整累计覆盖 | 保留来源/重复信息，不倒扣；不能按收到时间决定新旧 |
| 新版本 v3=130 且无更正依据 | 累计下降不是普通增量 | 拒绝/隔离，原 150 不变，增加安全 gap；不能取 max 后宣称全部可信 |
| 明确 correction v3=130 supersedes v2 | 来源有更正权且链/CAS 匹配 | 留原 100/150，追加 -20 的更正，净值 130 |
| `delta` d1=40、d2=60 | 每个原生事件唯一；模式在 scope 内固定 | 各一次共 100；重复 d1 不增加；更正须指向原事件 |
| 迟到更正缺前置/冲突分支 | 必需引用不可得 | 保留 pending/quarantine 与 unknown；不得覆盖 current 或默认应用到另一个 attempt |
| 父层 rollup=子调用之和 | 来源证明为汇总 | 作为只读对照，不产生收费事件；父自身调用是独立 scope 才可另计 |
| 同一 protocol 出现未知版本、币种或模型映射 | 无可信解释 | 明确 unsupported/gap，不降为 legacy 成功或 0 |

旧 `task_usage` 行、小时聚合和当前 API 只是诊断/对照来源。它们没有可恢复的旧版本与调用身份，不通过“当前值减上次值”伪造新事件账本。新事件为预算权威，现有聚合仍为用量视图；若未来在同一个 ingress 同时维护旧视图投影，须在明确事务内完成，不能产生两个互不一致的预算权威。

## 6. 预留、消费、释放与原请求恢复

| 阶段/故障 | 必需事实与处理 |
| --- | --- |
| 候选/readiness/准备 | 只建议执行，不产生永久 reservation；已有权限/容量检查原样保留 |
| 领取最终化 | 在当前授权、锁定 task/generation/runtime 与 root/policy 下保存 attempt + 影子 reservation + 原操作收据，再返回 launch payload/token；失败/响应丢失按原 claim 查询，不能补造下一次 attempt |
| 实际调用开始 | 调用意图先落本地 durable journal，服务端持有精确调用关联；同 claim 的 fallback/补问也有新调用身份。S2 还需每次真正调用之前的可验证许可/上限，S1 没有硬拦截承诺 |
| partial usage | 追加可证贡献；按同 reservation 转移 A/R；仍缺的部分保留 gap。partial 不能标最终结清 |
| 已证从未发出调用 | 必须有防止旧 launch 后来执行的 fence 与原未派发证明，才能释放原 reservation；只有队列行消失、未查到结果、HTTP abort 均不够 |
| 取消/超时/进程退出 | 只证明该生命周期事实；有可信最终用量、scope 完整和不可再执行证明才结算释放。否则转 unknown/保留占额 |
| 失联/daemon 重启/回调未 ACK | 保存原调用/事件和占额；重传原证据；不因 TTL、当前运行数为 0 或重新 claim 而清零旧成本 |
| 最终化/消费与取消竞争 | 同一原 reservation/attempt 在同事务排序；只产生一份终结贡献和收据，取消不覆盖已消费事实 |
| 响应丢失或 commit 未知 | 返回 unknown，使用原 key/ID 只读查询或精确原上报恢复；查无、issued、pending 不证明未提交，不另开根/调用/key |
| 已关闭/已删除源收到晚报 | 能证明原 scope 且当前 telemetry 身份合格时记入原根，不需要重新打开执行权限；无法证明则保留 gap/占额，不自动释放 |
| 明确更正已结清记录 | 对原 scope 追加 correction，更新净投影和 revision，保留旧收据；已释放额度不被当成新“退款操作”重放 |

允许有界重试的仅是**已证整笔回滚**、请求仍 live 的数据库冲突；commit 未知、取消的 HTTP 请求和已发出的 provider 工作不能套自动重试。原请求恢复与一次新执行分别命名、分别授权。

## 7. 当前权限、事务和版本

### 7.1 最小权限提案

| 主体/动作 | 建议合同 |
| --- | --- |
| 建根/调额/close/reopen | 当前 human + 当前工作空间 owner/admin + 当前普通根任务编辑权限；显式 reason、原 key、expected policy revision；不把任务发起人/智能体 owner 的 ID 当人工许可 |
| 人工读根/原收据 | 同一当前工作空间管理权限；源存在时还须当前根读取权限。源删除后只给最小经济 tombstone，不复活正文/目录身份；新工作空间/其他用户的原 key 不通用 |
| runtime/daemon 上报 | 当前有效的 daemon/安装身份 + 服务端持有的原 attempt/调用证明；仅写原 scope 的账务证据，不能改 root/limit/mode、开新 attempt 或借此获得任务 API 权限 |
| 已撤 invoke/已终结旧 task | 禁止新执行/新预留；已发生调用的窄证据可在仍有效的 telemetry 身份下收敛。旧 mat_ 无复活特权；daemon 也被撤权则拒绝，费用保持 unknown，后续恢复须另有可信来源 |
| Reporter/Reviewer/custom task/mcn_ | 不新增预算管理、根列表或跨任务读取能力；不修改 F2 exact GET/人工采集/终止撤回合同 |

读取 no-store，正文在同次授权读取/事务成功后才释放；鉴权/404/冲突错误不泄漏隐藏根、其他任务、provider 会话或金额。hash 是完整性材料，不是来源真实性证明或权限凭据。新的机器写入口不能通过普通 task header 冒充 daemon。

### 7.2 接点与锁序

复用已有 task claim/finalize 的真实 q/tx、workspace/user credential/version、runtime/agent/principal/invoke 复核及 generation CAS。名单、候选和 Auth 首次检查不替代最终事务。尤其不能只在 `CaptureTaskUsage` 指标方法或候选查询上加预算判断。

预算 root 行必须实际递增 ledger/control revision；reservation/事件/投影/原收据在同事务提交。并行建根还需要成员范围 fence；同根两次预留对最后额度串行排序。计划中推荐在**既有授权/生命周期锁族之后**获取预算专用成员 fence/root/reservation/event 锁；所有预算路径遵守同序，不能拿着 root 锁再反向申请 task/成员锁。准确的全路径锁图是实施前 E2 证据门，不能把这段顺序当未经核对的现成 helper。

新准入拒绝旧 root policy revision、过期/已撤权限、改变的 task generation/runtime/绑定关系、缺失/损坏 schema 或未知协议；不取最新值替调用者续权。已提交原操作的 lookup/replay 在当前读取/上报权限后返回原结果，标示当前 stale，不因后来 policy 变更抹掉旧提交。policy epoch 即使 mode 关闭再开启也不复用，防止 ABA。

S1 不读 Cloud 商业次数额度来设置父任务金额，不写共享项目策略；配置损坏不是自动 off。S2 必须证明全部 provider 启动入口和所有 claim 入口经过约束；旧 daemon 不支持预算协议时不能发 enforce launch。现有 quota 的 fail-open、released key 可复用和按状态/年龄释放都不能原样复制。

## 8. 最小持久化与使用面（候选，未写迁移/路由）

数据职责可收敛为五类，不建立新的工作流引擎：

1. root/control：唯一业务锚点、启用/范围、policy 与 ledger revision、准入状态、净余额/quality、最小 tombstone。
2. member binding：普通任务唯一 root、继承来源、绑定时间/版本；删除不丢归属。
3. attempt/invocation：真实 task/claim/runtime 与调用/计量范围、原 reservation 和持久未决状态。
4. immutable entries：原观察、更正、预留转移、释放证据与 source/schema/version；可由同一 reducer 重放净投影。
5. operations/receipts：当前主体范围下的原 key、规范意图 digest、已提交结果；可以与上述记录共表，具体布局在实施前按读写形状收敛。

所有 money/token 正值、root/scope/事件身份、唯一键、引用链和账目 revision 要在事务内验证；关系不新增数据库 FK/cascade。每个索引（包括唯一索引）必须单独单语句 CONCURRENTLY 迁移，确切迁移编号到获准实施时再分配。账目存在时不做破坏性 down；回退停止新观察/准入，保留账目与窄原结果读取，不降级成金额 0 或改共享策略。

人工使用面建议集中在普通 `issue budget` CLI：创建/调整/close/reopen 需要明确确认、版本、理由及原意图文件；show/by-key 只读，响应丢失只显示 unknown 和原查询方式。HTTP 端对应 exact root 的 create/config/operation/read 能力，不新增 workspace 全量列表或任务角色通配权限。精确命令/DTO 在 D3/E3 最终审阅后冻结；本轮不注册任何路由。

## 9. 已决与剩余决策

| ID | 状态 | 建议与取舍 |
| --- | --- | --- |
| D1 根绑定 | **用户已确认“长期绑定父事项”** | 同根累计不重置；不同阶段支出显式调额。无需再向用户确认这一方向 |
| D2 关闭与留存 | 待审阅具体方案 | close/reopen 同根、结清另轴；源删除留最小经济 tombstone，父删除不自动承接新执行；默认不自动清除 unresolved。保住追溯，但需要后续物理保留/删除政策；不借 F2 的报告撤回或清理权处理预算 |
| D3 最小使用面与权限 | 待审阅 | 当前 human owner/admin 管理/查看、API/CLI 先行，Reporter 不扩权；可审计且面小，但普通成员暂不能自助调额，UI 后置 |
| D4 计价与不确定性 | 待审阅承诺边界 | USD 整数，提供方回传/估算/未知分开；新根只保证启用后的可证范围，保留历史 gap；未来有无上界 unknown 时拒绝准入。可能长时间显示 unknown，不承诺账单或硬封顶 |
| D6 quick-create | 新增，待审阅 | 见 §11：建议纳入；不纳入则列为 not_applicable |
| D5 S1→S2 | 已有先 observe 后 enforce 方向，具体启用条件待证据 | S1 不阻断金额，S2 另行授权且必须有完整 writer/调用入口、版本协商和可靠释放证明；不能提前顺带接 F4 |

产品方案可审阅不等于“所有实现前决策已解决”。最终 F3 实施必须针对本方案及其实际审阅后的版本获得独立明确授权；D2–D4 的不同选择可能改变实现范围。

## 10. 尚缺的技术/验收证据

| 门 | 需要取得的证据 | 当前状态 |
| --- | --- | --- |
| E1 生产者语义 | 各 adapter 的 source kind、cumulative/delta、缓存口径、session baseline、原始费用是否估算、final completeness；无法证明的明确不支持 | 已有源码分源清单；没有真实 Provider 证明，不能推广到全部版本 |
| E2 归属与并发 | 全部 ordinary child/create/reparent/delete/clone/import writer 清单及统一锁图；两种 claim API、相同 claim 多调用、新 task 重试、唯一根与成员 phantom 反例 | 接点已研究；未写实现/测试，S2 尚不具备防绕过证据 |
| E3 持久协议 | 原事件与 batch ACK、daemon durable journal、能力协商/旧版本拒绝、最小 DTO/错误/原 key 查询、schema/唯一索引和留存恢复 | 本文为合同候选；现有 usage wire 不足 |
| E4 手算与实际验证 | [F3 验收矩阵](f3-acceptance-matrix-20261113.md) 的具名 pure/DB/Auth/claim/daemon/CLI 结果，normal/race 分开，新建任务自有环境 | 仅 expected 和计划，未运行产品测试 |
| E5 enforce 有界证明 | 每次调用准入与可执行成本上限、旧进程不可复活 fence、失联后终结证明、启用前义务处置 | 后续 S2 门；估算或本地 fake 无法证明真实提供方收费上限 |

不得调用真实 Provider、操作 POLA-1、调整共享项目策略、恢复账单对账、发布/部署/Hosted CI 或启动独立真实运行任务来填这些缺口。本轮只核对文档、源码身份、需求/验收映射和任务状态。

## 11. 2026-11-14 源码复核修订（优先于上文冲突处）

依据 [执行/claim 分源](f3-execution-source-map-20261113.md)。复核时 HEAD 为 `fb9f4c070`（较 `feb87aee7` 多两个并行提交，只动 collaboration 测试文件；`fafb27b1d` 仍是祖先）。以下各条在与 §3/§5/§6/§7 冲突时为准；仍是提案，未实现。路径省略 `server/` 前缀。

| # | 源码事实 | 对方案的修订 |
| --- | --- | --- |
| R1 | 列名是 `issue.parent_issue_id`（`migrations/001_init.up.sql:65`，`ON DELETE SET NULL`）；没有 `tasks.sql`，claim SQL 在 `pkg/db/queries/agent.sql` | 上文与总表中的 `parent_id`、`queries/tasks.sql` 均指这两处 |
| R2 | claim 是两个事务：`claimTask`（`internal/service/task.go:3425`）先提交，再 `FinalizeTaskClaim`（`:3735`）。finalize 失败则同一行 requeue（`agent.sql:946`）；响应丢失则重领换代并重新 finalize、重新发 token。不存在“按原 claim 查询”。`claim_generation` 不随 start/complete/fail/usage 上行 | §6“按原 claim 查询、不补造下一次 attempt”作废。预留写在 finalize 事务内、`:3807` 身份 CAS 之后、发 token 之前。每次成功 finalize 签发一个服务端 `budget_attempt_id`，随 launch payload 下发，由 daemon 在后续上报中带回；generation 不能作执行身份。重领产生新 attempt，前一代标 `superseded_unproven`，其预留转未知占额、不释放。代价：响应丢失较多时影子预留会偏高，但不低估 |
| R3 | 删除 issue 经 FK 级联物理删除 `agent_task_queue`（`001_init:130`）及其 `task_usage`（`032:3`）、token、message；直接子任务被清空父指针（`queries/issue.sql:132`）；取消 task 在删除事务之外先提交（`handler/issue.go:4786`）。按约束名未找到后续删除这些 FK 的迁移 | 预算表不带 FK，保存 workspace/issue/task/runtime/agent 的 ID 副本，因此不随级联消失；成员行保存原父 ID。“tombstone”指预算自己的行，不要求改动现有级联。首包不修改现有删除行为 |
| R4 | 改父查环在事务外、走连接池、封顶 10 层（`handler/issue.go:4322-4334`，批量 `:5515-5531`）；事务内对新父只有无锁检查；创建时父行不加锁（`service/issue.go:445`） | 没有可复用的事务内遍历。启用根时用自己的递归查询（带环检测、不设 10 层上限），发现环即拒绝。并发改父成环属推断，未证实 |
| R5 | 已有工作空间级 advisory fence `iteration:<ws>`（`internal/iteration/transaction.go:83`），issue 创建/更新/删除与 StartTask 都取；claim/finalize 不取。autopilot create_issue（`service/autopilot.go:790`）、onboarding、协作结构 writer、工作空间拆除是否取该 fence 未证实 | 成员绑定建议复用这条既有 fence，而不是新增第二条（少一把锁、少一种锁序）；取舍是预算与 iteration 共享争用点。未证实的 writer 列入 E2，清点前按 B60 记缺口 |
| R6 | 两族锁序相反、靠 NOWAIT 调和：issue 写路径是 fence → issue FOR UPDATE → task NOWAIT（`agent.sql:2806-2808`）；finalize 是 task FOR UPDATE → issue FOR KEY SHARE NOWAIT（`migrations/650`）。finalize 只锁 task 自己的 issue | 根行锁在两族中都排最后；持根锁时不得再阻塞等待 task 或 issue 行。finalize 通过已保存的成员行找根，不遍历也不锁祖先 |
| R7 | quick-create task 在领取和运行期 `issue_id` 为 NULL，父任务只在 `context.parent_issue_id`，完成后才关联（`agent.sql:459,530`）。chat 与 run_only 也无 `issue_id` | §1“按 issue_id 判定普通任务执行”会漏掉在父任务下建子任务的那次执行。新增决策 D6（见下） |
| R8 | finalize 的 invoke/originator 复核只对受限模板角色存在（`internal/service/task_capability.go:22-28`）；普通 agent 只复核密码会话版本（仅 password 模式）、安装绑定、task 行身份、issue 准入 | §7.2“复用已有 invoke 复核”对普通 agent 不成立。S1 只观察，不需要它；“撤 invoke 即拒绝新预留”是 S2 要新增的检查点，不是复用 |
| R9 | `RequireHumanActor` 只拒绝 `task_token` 与 `cloud_pat`，未知来源按人放行，`mdt_` 不在名单（`handler/actor_guards.go:110-119`）；`mat_` 请求的 `X-User-ID` 被覆盖为 runtime owner | 预算管理入口改用允许名单（只接受明确识别的人工来源），不直接复用该 guard |
| R10 | 所有终态 writer 只证明行状态；`FailStaleTasks`、`FailTasksForOfflineRuntimes` 的注释明说 daemon 进程可能仍在运行；complete 在行已终态时按幂等成功返回 | 与 §6 一致且更强：任何终态都不是释放依据 |

### 新增决策 D6：quick-create 执行是否纳入首包

- 建议：**纳入**。finalize 的身份 CAS 已比较服务端保存的 `context`，可据其中的父任务归根；否则“让智能体在父任务下快速建子任务”是一条现成的不计入路径。
- 取舍：纳入要多处理一种无 `issue_id` 的 attempt，以及完成后关联时的成员绑定；不纳入则实现更小，但须在读数里把它列为 `not_applicable:quick_create`。
- chat、run_only autopilot、独立 collaboration/host run 仍不纳入。

### 修订后的未读与推断

未读：`buildClaimedTaskResponse` 全文、协作/host run 链、daemon 如何得知取消、调用目标变更对在途 task 的影响、`mdt_` 中间件。推断：并发双向改父可成环；成员移除后用量上报被拒；不存在跨工作空间移动 issue 的 writer、不存在 clone handler（基于有限搜索）。这些都并入 E2/E3，不作为已证事实。
