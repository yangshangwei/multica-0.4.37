# P1 架构评审

日期：2026-10-05。结论：**ITERATE**。这是规划阶段的设计审查，不是实现、测试或产品验收通过。审查范围为父任务的 `prd.md`、`design.md`、`implement.md`、`test-spec.md`、两份 research map，与原 `projects-prd.md` 的全部 P1 要求及现有关键代码交叉核对。未修改产品代码、未运行产品测试。

## 总体判断

首选方向可行：复用 Project 和描述，以统一服务端正式集合计算健康，使用不可变进展修订与独立 outbox。五个子任务、共享文件单 owner、先 foundation 再 health/progress 的顺序合理；没有把 P2/P3 或真实 I1 实现伪装成 P1 的前置交付。当前需补齐三个执行契约并修正一处已由代码证实的旧端能力探测错误，再交 Critic；不需要另起架构或扩展产品范围。

## 必须修正

### AR-01（高）：发布事务的快照隔离尚未闭合

证据：`design.md:71` 为概览明确了 **REPEATABLE READ 只读事务**，但 `design.md:103` 的发布事务在证据检查后“采集统计”，`implement.md:52` 只要求“事务内系统快照函数”，没有约定该写事务的隔离级别或单 SQL 快照边界。现有 `server/internal/handler/handler.go:59` 的 `txStarter` 仅暴露 `Begin`；`server/internal/handler/triage.go:218` 的参考事务也没有设置隔离。

风险：项目行锁不会阻止已有任务改状态、日期、负责人；若实现者直接复用默认事务并分多次读取目录/任务/指派，已发布统计可能混合不同数据库时点。仅“处于同一事务”不足以满足原 PRD `projects-prd.md:213` 与 `:217` 的统一快照承诺。

修正建议：在设计中选定一个具体实现，优先让 preview、overview/drilldown 与 publish/correct 的健康采集共享可接收事务查询器的函数；发布为显式 REPEATABLE READ 写事务，在首次查询前设定隔离，并规定 serialization failure / lock conflict 的有界整事务重试和最终可恢复错误。继续在当前授权 fence 内验证提交，不把另开的只读事务结果直接贴到发布结果。若采用单 SQL 收集全部计算输入也可，但必须明确哪些输入包括在同一 statement snapshot、哪些检查仍在提交锁内。

完成证据：增加真实双连接屏障用例，在一次采集的两阶段之间变更任务状态/目录/有效指派，确认结果完全属于一个可解释版本或整事务重试；附快照的发布不能留下混合结果。把该证明落实到 health/progress 的交接函数和 P1-FR-05。

### AR-02（高）：outbox 领取算法与项目删除锁顺序需明确

证据：`design.md:107` 要求沿用 T1 通知模式并在投递时检查撤权及项目；`:141` 同时要求 workspace → member fence → project → 子记录，`:147` 删除在项目排他锁后删除 outbox。被引用的现有 T1 实际先在 `server/internal/handler/triage_notifications.go:164` 执行 `FOR UPDATE SKIP LOCKED` 领取队列行，再在 `:223` 用 **try advisory lock** 避免等候撤权 fence；它不是可以直接照搬的阻塞锁模板。

风险：P1 若先锁 outbox 再阻塞取得项目共享锁，删除已持 project 排他锁并等待相同 outbox 行时会形成环。通用顺序声明还没有告诉 progress 实现者如何在领取工作时遵循它。

修正建议：明确选择“无锁候选扫描 → 每条独立事务依序取得 workspace、recipient fence/member、project shared、outbox row → 锁内重查未投递/到期/授权 → inbox + delivered_at 同事务提交”的模式；或保留队列优先但所有逆序 fence/project 获取都必须 NOWAIT/try，失败释放整笔事务、延期重试。不要一边沿用 T1 的先领取模式，一边使用会阻塞的 project 锁。定义 worker 注册位置、轮询/退避和取消无效接收者的终态即可，无需引入通用消息框架。

完成证据：补投递与删除双顺序、投递与撤权、重复 worker 领取、崩溃后 inbox 已存在的屏障测试；除“不重复”外断言无锁等待环、删除后不能重建通知。对应 P1-FR-10/11/12 与第三项 pre-mortem。

### AR-03（中）：进展响应、证据版本与最新验收的跨域合同仍需固定

证据：`design.md:43` 使用未定义类型的 `observed_revision`，`:86` 只有未展开的 `latest_acceptance`，`:97` 给出列表/历史路径但无响应 DTO；`:99` 描述 `preview_hash` 却未固定构成字段和变更比较规则。与此相对，`implement.md:12` 允许 UI 按 mock 契约开发，`:29` 要求多个 owner 交接。实际实体版本不同：`server/pkg/db/generated/models.go:976` 是 issue.revision，执行的 `AgentTaskQueue` 在 `:302` 是 state_version，并不是统一 revision 字段。

风险：progress、health 与 UI 会分别猜测 response shape、验收选择顺序和证据变化含义。现有设计也把 `statistics_snapshot` 定义为“同 §5 DTO”（`design.md:45`），而 §5 带 `latest_acceptance`；若不分出非递归统计 DTO，历史引用边界会变得含混。

修正建议：加一张明确合同表或代码块，至少固定：

- `ProjectUpdate`、revision 列表、create/correct/replay、preview 的返回身份、nullable 字段、稳定排序及 cursor；每个响应包含 workspace/project/update 身份，成功不能通过 fallback 伪造。
- 独立非递归的 `ProjectStatisticsSnapshot`，只含计数/健康计算事实与时间/时区/版本；`latest_acceptance` 只回摘要，不把进展统计再嵌回统计。定义多条验收的选择顺序，以及“较新旧描述验收”和“较早当前描述验收”并存时的显示规则。
- 证据各 kind 的版本来源、授权 loader 和比较字段；issue 可用 revision，execution 明确使用 state_version 是否还需纳入可变结果指纹，url 为无服务端验证版本。定义 preview hash 覆盖的规范化业务字段、证据版本、描述版本、接收者，以及是否覆盖统计版本，防止 UI 与服务端对“需要重新预览”理解不同。
- 作者/修订者离开后的署名读取路径。可复用 `server/pkg/db/queries/user.sql:9` 的批量历史署名查询，响应仅保留必要展示字段；不必新增重复用户资料快照，但需定义找不到用户时的稳定占位。

完成证据：补 DTO examples 和畸形响应 fixture、不同证据 kind 的复核测试、多个验收及旧版更正的选择测试；让 health/progress/ui 三个子任务引用同一个合同，避免把核心决定留到实现阶段。

### AR-04（高）：旧后端能力探测会返回 400，当前 404 降级条件失效

此项由后端研究代理补充，并经本评审读取源码复核。`design.md:63` 拟定 `GET /api/projects/capabilities`，假设旧服务端为 404；但现有 `server/cmd/server/router.go:2136` 的 `/{id}` 会接收 capabilities，`server/internal/handler/project.go:231` 调用 UUID 校验，`server/internal/handler/handler.go:768` 明确返回 **400 invalid project id**。只在新端把静态路由放在前面，不能改变已安装的旧服务端行为。

修正建议：优先改为 `GET /api/workspaces/{validWorkspaceId}/project-capabilities`，新端放入 URL 工作空间的成员授权组，旧端无此子路由时可得到 404；保留 401/403/network/schema error 与 unsupported 的区别，不把任意 400 放宽为旧端能力缺失。也可用现有 `/api/config` 的构建能力字段缺失作旧端探测，但它不承担租户授权。同步全部 design/test/任务引用。

完成证据：新客户端对真实旧 router 的请求应明确走 unsupported 分支；错误矩阵覆盖 400、401、403、404、网络与畸形响应，不能只 mock 一个旧端不会返回的 404。该项关联 P1-FR-13。

## 已有设计符合的关键边界

| 核对项 | 判断与证据 |
| --- | --- |
| T1 正式准入 | 正确。`prd.md:27`、`design.md:69` 使用 `not_required/accepted`；源码 `server/pkg/db/queries/project.sql:69` 已使用相同白名单和租户条件。待分拣开关不参与公式，候选项目不当实际关联。 |
| 完成/取消兼容 | 正确。`design.md:65` 保留旧 done_count=F+C，分项另加；源码 `project.go:80` 通过 terminal keys 算旧数。未知状态保留 N/U 且完整性失败，测试覆盖自定义 archived 目录和全取消。 |
| 单描述与 CAS | 方向正确且必要。现有 `project.go:564` 会携带预读的 nullable 值，SQL `project.sql:46` 无 CAS。设计先锁后按字段合并，独立 description_revision 保证图标等变更不使验收失效。 |
| 人工身份 | 充分规划。`design.md:97` 同查机器凭据、resolveActor 和有效成员，符合 `actor_guards.go:110` 与 `triage.go:175`；不会把机器 token 的 owning user 当人工最终发布者。 |
| 幂等与不可变修订 | 模型合理。`design.md:36`—`:45` 及 `:105` 将稳定发布身份、修订、请求身份分开，同 payload 重放返回原 revision；更正不能把旧验收绑定到新描述。通知稳定身份与原作者/首发时间不随更正改变。 |
| 关联/删除与历史 | 范围完整。`design.md:143` 纳入任务、T1候选/接受、资源、聊天、小队、自动化与新进展；保留候选 ID 为失效来源而非活跃关联可行；`triage_actions.go:46` 的现有 NOWAIT 明确保留。专属进展随项目删除符合源 PRD:265，执行/评论不删。自动化停用应落到现有 status=paused、pause_reason 与 trigger.enabled 语义（源码 `autopilot.sql:73`、`:89`、`:225`），不另造 disabled 枚举。 |
| 快照与下钻 | 选实时重算及显式刷新符合源 PRD:217。`design.md:91` 明确 cursor 绑定版本且变更回第一页；`:93` 隔离 activeView/隐藏状态/子任务偏好，不伪装成历史冻结列表。 |
| 包边界与客户端 | `implement.md:23`—`:29` 给出单 owner，core 不持 UI、views 用适配器，移动独立；`:70` 修触及路径的现有乐观删除。撤权先清缓存再导航、晚响应不能重填写入测试，符合服务端授权与共享状态边界。 |
| I1 边界 | `test-spec.md:99`—`:105` 明确真实迭代场景待 I1，P1 只证明当前字段与历史不受误写；没有以 mock 迭代表假报验收。 |

## 首选方案的最强反方论证、真实权衡与综合路径

**反方论证（支持 B）**：A 每次概览和风险翻页都重读全量任务，并将有效指派/执行环境状态纳入指纹。一个活跃的万任务项目可能在每次翻页间变化，使用户反复被送回第一页；即使 SQL 正确，风险定位仍可能不可用。持久化带 TTL 的成员快照能稳定分页、复现点击时事实、减少重复聚合，是有实际价值的 B，而非只多几张表。来源：`design.md:13`—`:17`、`:89`—`:91`。

**权衡**：稳定复现旧集合与展示当前事实无法同时无成本获得。B 仍需逐次授权、删除/撤权处理、过期提示与回收；A 少存储且读到当前集合，但必须接受刷新和较高重算成本。另一项真实权衡是旧客户端无 token 描述写与防丢写：`design.md:55`、`:163` 选择 428 是可解释的 API 限制，不能同时宣称旧描述编辑完全无感兼容；其余旧字段写继续支持。无需为了“兼容”绕开 CAS。

**综合路径**：保留 A，先修 AR-01/02/03，并在现有性能验证中增加持续变更时的分页稳定性观测（刷新频率、达到第二页的成功率/耗时），不只量 P95。如真实负载造成不可接受的反复重置，再以同样 DTO/授权边界评审短期快照 B；目前不提前实现双统计来源。CAS 限制在支持矩阵、CLI/内置 skill、发布说明中明示，旧端请求失败时确保保留输入；回退保持具备 CAS 与历史只读能力的服务端。

## Pre-mortem 与验证计划审查

三个 pre-mortem 都对应真实失败模式，原 `design.md:169`—`:173` 与 `test-spec.md:107`—`:121` 具有 unit、integration、E2E、observability 分层：

1. 统计改义/漏项：unit 的 N/F/C/U 与 category 矩阵、DB 全量/JOIN、E2E 偏好隔离、complete/unknown 观测可证明；补 AR-01 发布同快照屏障与活跃分页观测。
2. 验收错绑/撤权：CAS、真实 member actor、证据提交复核、缓存清理及晚响应测试齐全；补 AR-03 的证据版本算法与最新验收选择 fixture。
3. 孤儿/通知重复：全 writer 双顺序和故障注入、outbox 重启与稳定 inbox id 已列；补 AR-02 的领取/项目锁竞争，避免只测幂等而漏死锁。

性能预算当前明确为建议、基线后固定，没有将 2 秒/5 秒或四周成效指标写成已通过。无新依赖、无新 FK/级联、独立并发索引迁移和保留数据的回退边界符合仓库约束。剩余不确定性来自未来实现和运行环境，不要求本轮规划运行产品测试或认证生产部署。

## 交接

请父任务修订设计、开发任务与相应测试合同以闭合 AR-01—04，再进行架构复核并顺序交 Critic。此次 ITERATE 不否定 P1 范围与五子任务拆分；仅阻止将尚未固定的并发/DTO 契约和不成立的旧端探测标为“可直接并行实施”。
