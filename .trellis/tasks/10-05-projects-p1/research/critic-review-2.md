# P1 Critic 最终复审

日期：2026-10-05。结论：**APPROVE（规划可交付）**。未发现需要再次修订的实质问题。原 [critic-review.md](critic-review.md) 保留为首轮记录；此结论只证明实施方案就绪，不证明产品已实现、测试已通过或已经部署。

## 复审范围与证据

按顺序读取 [architect-review-2.md](architect-review-2.md)、[planner-revision.md](planner-revision.md)，再核查当前父 [design.md](../design.md)、[api-contract.md](../api-contract.md)、[implement.md](../implement.md)、[test-spec.md](../test-spec.md)。逐个读取五个子任务的实际 task.json、PRD/实施清单依赖段、设计合同入口及 implement/check manifests。未以修订记录的声明代替当前文件内容，也未运行产品测试。

| 首轮问题 | 当前可执行合同与验证设计 | 结论 |
| --- | --- | --- |
| AR-01 发布统计快照 | design §5/6 固定 Begin 后第一条 SQL 设置 RR READ WRITE、同一 qtx 贯穿授权/证据/统计/写入、授权 locking read 和最多三次整事务重试；test-spec P1-FR-05/06 包含双连接阶段变更与旧授权等待屏障 | 闭合 |
| AR-02 outbox 与删除 | design §6 固定无锁候选扫描后逐条 RC、workspace→recipient→project→outbox 正序锁、inbox/outbox 同 commit；含轮询/退避/dead-letter/ctx 生命周期；§8 使用真实 paused/pause_reason/trigger.enabled；FR-10/11/12 覆盖删除/撤权/双 worker/崩溃竞争 | 闭合 |
| AR-03 DTO/证据/验收 | api-contract 为唯一线协议，分别定义非递归统计、进展/历史/写结果/preview、issue revision、execution state_version+结果 digest、URL 未验证、canonical hash、历史署名；两个验收摘要有稳定选择规则；AC-05/18、FR-06/09/13 和合同 §6 提供 fixture 要求 | 闭合 |
| AR-04 真实旧端探测 | 专用合法 workspace UUID 子资源取代旧 projects/capabilities；旧错误路径 400 与专用路径 404 均明确，404 还需确认工作空间可读；400/401/403/network/schema 分支独立，FR-13 要求真实旧 router 验证 | 闭合 |
| CR-01 调度与共享整合 | 父表与五份 task.json/子文档均区分 preparation/start/integration gates；depends_on 一致为 [] 且明示不等于忽略里程碑；FG/HG/PG/UG 需实际 commit/命令/结果；foundation 在 FG 后仍持有 SQL/router/main/sqlc 整合职责 | 闭合 |
| CR-02 移动验证 | test-spec 新增 MO 的四个 mobile/data API/Query/realtime/access 测试落点，FR-13/15 明确旧字段、能力、缓存和撤权；implement §8 独立列出 mobile typecheck/lint/test，并声明根检查排除 mobile | 闭合 |

## 父子任务一致性

实际五子任务均保持 `status=planning`、`parent=10-05-projects-p1`，未把方案评审或阶段解锁写成产品完成。各自依赖为：

| 子任务 | 启动 | 完成集成 |
| --- | --- | --- |
| foundation | 实施授权 | FG + 所有下游共享改动整合及回归；FG 不归档 |
| health | FG | HG + foundation 已整合对应生成文件 |
| progress | FG，可先做无统计路径 | HG + PG + 统计发布及 worker/router 整合 |
| ui | FG，可在 API 开发期间实现 | HG + PG + UG + 真实平台验证 |
| verification | FG 后真实服务集成，准备/基线可先行 | FG/HG/PG/UG + 前四任务完成 + 最终证据 |

父表、子 PRD、子 implement 与 meta gates 语义一致，没有整任务互等循环。十份子任务 implement/check manifests 各包含一次唯一 API 合同，引用目标均存在；五份子 design 也指向该合同。不支持里程碑的调度器由父任务核对证据再启动，避免把空 depends_on 当无条件实施许可。

## 范围、原则与验证完整性

当前 test-spec 仍完整保留 PRJ-001—014、PRJ-AC-01—27 和 P1-FR-01—18。目标模板、完成/取消拆分、健康概览、手动进展以及支撑的验收/权限/并发/删除/混合版本契约均未缩减。T1 正式集合仍为实际关联且 `admission_status IN ('not_required','accepted')`，不依赖分拣开关；旧 done_count 继续表示完成加取消。

选择 A 与原则一致。B 的稳定分页优势被公平保留，修订增加活跃变更下的刷新率、第二页到达率/耗时和连续重置观测，足以在实施时检验 A 的代价；没有提前引入第二套统计存储。描述 CAS 对旧无 token 写的 428 限制仍明确，没有虚构完全无感兼容。三个 pre-mortem 与 unit/integration/E2E/observability 分层保留，并新增针对首轮问题的屏障与协议测试。

迁移仍不新增 FK/级联或依赖，每个新索引单语句 CONCURRENTLY；回退有保留数据 guard、隔离演练和 writer 停止要求。I1 真实迭代历史分支仍明确交接后续实现，既不省略保护契约，也不要求本轮建设 I1。

## 重新模拟实施

1. **验收发布、附统计、响应丢失：** progress 可按唯一 Draft/Preview/Write DTO，在固定 RR/qtx 合同内检查证据、描述与统计版本；同 request/hash 的回放先返回原 result_revision，不因后续新 preview 重新发布。health/progress/ui 不再各自猜测快照或响应形状。
2. **投递与项目删除竞争：** 两者都先 project 后 outbox；worker 在任何 outbox 锁前处理 workspace/recipient fence，删除先提交时不重建通知。失败释放事务、退避和最终状态都有明确实现边界。
3. **分工及混合版本：** FG 后 health/progress/ui 可按不同阶段并行，foundation 继续整合共享文件；新桌面用真实旧端路径降级；移动有独立数据层 fixture 和执行命令，不会再借根检查代验。

三个模拟均能从当前文档取得核心决定，无需执行者另行定义业务或协议。剩余工作是未来产品实施与真实验证，不是本轮规划缺项。

父任务可完成最终链接/JSON/文档校验并交付规划。本报告只新增此评审文件，未修改产品代码或其他代理的文件。
