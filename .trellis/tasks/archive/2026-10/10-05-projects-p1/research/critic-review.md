# P1 Critic 评审

日期：2026-10-05。结论：**ITERATE**。总体方案与范围成立，但尚不能把当前文档作为无核心猜测的并行实施合同。此次只评审规划，不要求产品代码、运行产品测试或生产部署作为规划通过条件。

## 判定与检查范围

已读取 Architect 评审、父任务四份规划文件、五个子任务的 PRD/design/implement/task 元数据及上下文 manifests、两份研究映射、原 projects-prd 的全部 P1 与共同 README。复核了真实项目路由、UUID 错误路径、项目 SQL/正式准入、T1 outbox 锁、现有自动化状态、前端能力/编辑器/风险上下文研究，以及 package.json、移动 package.json、check.sh 和 Playwright/Electron 执行入口。没有执行产品测试或迁移。

| 维度 | 判定 |
| --- | --- |
| 清晰度 | 业务行为和 ownership 清楚；发布隔离、队列锁及跨域 DTO 仍有核心缺口 |
| 可验证性 | 14 个功能要求、27 个 P1 AC、18 个补充边界均已映射；需补发布/投递屏障与移动专属验证 |
| 完整性 | P1 范围没有被缩减；不存在用 P2/I1 新实现阻塞本轮规划的问题 |
| 整体方向 | 单 Project、单描述、实时健康、不可变进展与人工验收符合目标 |
| 原则与方案一致 | 通过。A 遵守服务端全量统计和事实/判断分离；旧描述写 428 的兼容代价已明确 |
| 备选方案公平性 | 通过。B 的稳定历史成员与低成本分页有实际优势；A 的刷新成本没有被隐藏；C 被明确的正确性要求排除 |
| 风险与验证强度 | 待修正。现有测试分层和三个 pre-mortem 有效，但不足以自行填补 AR-01/02/03/04 和 CR-02 |
| 可直接分工 | 待修正。字段合同与调度依赖仍可能使 health/progress/ui 各自作不同决定 |

## 必须合并修正的清单

### 1. AR-01：发布统计必须固定真实快照边界（高，确定缺失）

`design.md` §5 明确只读 REPEATABLE READ；§6 的发布写事务仅写“采集统计”。项目锁不能防止另一个事务修改现有任务状态、目录或指派，因此“事务内函数”本身不能证明同版本统计。接受 Architect 建议：明确写事务隔离、在首次查询前设置，以及 serialization/锁冲突的有界整笔重试与最终错误；统一 health 的事务查询器接口，不能另开只读事务再粘贴结果。

将真实双连接屏障放到 P1-FR-05：采集的不同阶段间变更状态/目录/有效指派，断言快照来自一个可解释版本或整事务重试，不能混合。此次修正交付的是合同与测试设计，无需现在实现或运行。

### 2. AR-02：outbox 领取与删除必须有可执行锁算法（高，确定缺失）

`design.md` §6 “沿用 T1 模式”与 §8 通用锁序不足以决定 worker 实现。真实 `triage_notifications.go:164` 先锁队列，`:223` 再以 try advisory lock 避免反向等待；直接复制后再阻塞取 project shared，会与已持 project 排他锁且等待 outbox 的删除事务形成环。

选择一种具体算法并贯穿 schema/worker/删除/测试：无锁扫描候选后，每条独立事务按 workspace → recipient fence/member → project shared → outbox 顺序领取和重查；或保留队列优先，但全部逆序锁使用 NOWAIT/try，失败释放整笔事务再退避。明确 worker 注册位置、轮询/退避和无效接收者终态。补投递/删除双顺序、投递/撤权、重复 worker 和 crash 重投的屏障与去重断言。

同一删除协议修订中，将“自动化 disabled”固定为现有字段语义：`status='paused'`、可解释 `pause_reason`、trigger `enabled=false`，保留已运行执行与历史。源码 `autopilot.sql:73`、`:89` 已有该模式；不要创造未经定义的 disabled 状态。

### 3. AR-03：固定进展、证据、快照及验收摘要合同（中，确定缺失）

`design.md` §3 的 `observed_revision`、§5 的 `latest_acceptance`、§6 的 preview/list/history 返回结构尚未展开，§3 统计快照又指向含验收的 §5 DTO。UI 被允许按 mock 合同先行时，这些字段不能留待各 owner 猜测。

按 Architect 清单补统一 DTO：响应 identity/nullability/排序/cursor，非递归 `ProjectStatisticsSnapshot`，最新验收的选择顺序和描述适用性，issue revision / execution state_version 与必要结果指纹 / url 无验证版本的区别，preview hash 的规范化输入及变更比较规则，历史作者离开后的署名读取/占位。补最小请求响应示例、畸形返回 fixture、多验收与旧版更正选择测试。所有受影响子任务指向该单一合同。

### 4. AR-04：旧端能力检测必须匹配真实路由行为（高，已复核源码）

旧 `router.go:2136` 将 `/api/projects/capabilities` 匹配到 `/{id}`；`project.go:231` 和 `handler.go:768` 因 UUID 无效返回 **400**，不是当前设计与 P1-FR-13 假定的 404。仅在新 router 把静态路由前置不能修复旧端。

采用 Architect 提出的 `/api/workspaces/{validWorkspaceId}/project-capabilities` 或其他已证明旧端返回 404 的独立子资源；保持工作空间成员授权，不把任意 400 当 unsupported。同步所有文档、mock 和迁移/版本矩阵；明确 400/401/403/404/network/schema 错误分支。用旧 router 行为设计测试，不能只编一个期望的 404 mock。

### 5. CR-01/02：调度元数据与平台验证需闭合（中，确定不一致/缺失）

**CR-01 调度：** 父 `implement.md` §1 允许 progress 在 foundation 后与 health 并行做无统计发布；progress 的 task.json `meta.depends_on` 却包含 health，子 PRD/implement 又直接重复整任务依赖。verification 在第一波建立 fixtures/基线，但其 depends_on 包含前四项。UI 的 mock 准备与最终集成同样不同。虽然“并行条件见父计划”可由人解释，自动调度仍无法区分开始与完成。

明确 `depends_on` 的唯一含义，并另列 preparation/start/integration gates（或同等具体字段）；同步父表、子文档与 task.json。foundation 首轮 schema/锁交付和之后持续负责 SQL/router/sqlc 整合也要区分：health/progress 不能等待“foundation 完全结束”，foundation 也不能等待它们而形成隐式循环。不要求再拆新子任务。

**CR-02 移动验证：** 计划明确修改 `apps/mobile/data/` 并交付读取/能力限制，但测试层表没有移动专属文件；§8 列的根 `pnpm typecheck/lint/test` 和 `make check` 都排除 mobile。证据：根 package.json 的三个脚本及 `scripts/check.sh:117/121` 使用 `--filter=!@multica/mobile`。因此当前命令全部通过仍不能证明移动兼容。

在 ui/verification 中指定移动实际 API/parser、项目 Query/realtime 和原属性写入兼容 fixture 的测试落点；至少补 `pnpm -C apps/mobile typecheck`、`pnpm -C apps/mobile lint`、`pnpm -C apps/mobile test`（移动 package.json 均已有）。标清旧字段/未知能力/畸形响应/撤权与新字段保留的证据；只验证本次承诺的兼容范围，不要求完整新增移动编辑或本轮执行这些命令。

## 原需求到规划的覆盖审计

“覆盖”表示可观察要求已进入设计和测试计划，不表示产品测试通过；“待闭合”表示相关核心合同仍受上述问题阻挡。

| 原要求 | 规划状态 | AC 与说明 |
| --- | --- | --- |
| PRJ-001 | 覆盖 | AC-01：同一身份、属性、五状态、五种原视图、原入口保留 |
| PRJ-002 | 覆盖 | AC-02/03：单描述、模板预览/选择/追加、上下文同源、不覆盖 |
| PRJ-003 | 待闭合 AR-03 | AC-04/05/06：验收独立、描述版本失效、依据必填；补 latest 选择合同 |
| PRJ-004 | 覆盖 | AC-07/08/09：N/F/C/U、空集合、全取消；done_count 不改义 |
| PRJ-005 | 覆盖 | AC-10/11：T1 not_required/accepted、实际关联、接受仅准入且去重 |
| PRJ-006 | 待闭合 AR-01/03 | AC-12/13/14/15：日期、风险去重、离线和有效引用、自定义类别均已计划；发布同快照补齐 |
| PRJ-007 | 覆盖，补 AR-03 DTO | AC-16/17：全量风险下钻绕过个人过滤、版本变更刷新、未知不绿色 |
| PRJ-008 | 待闭合 AR-01/02/03 | AC-18/27：人工发布、系统快照、成员通知、agent 零执行均未删减 |
| PRJ-009 | 待闭合 AR-02/03 | AC-19/20：请求幂等、不可变更正、历史追溯，需 DTO/投递算法支持 |
| PRJ-010 | 覆盖，补 AR-03 验收摘要 | AC-21：剩余/取消/验收提示、允许完成、原因或未填写审计 |
| PRJ-011 | 覆盖 | AC-22：状态不改任务/执行；真实迭代分支明确交接 I1 |
| PRJ-012 | 待闭合 AR-02 | AC-23/24：管理员删除、原子清理/并发 writer/历史保留；I1 部分单列 |
| PRJ-013 | 覆盖，补 AR-03 历史署名 | AC-25：描述冲突可恢复；资源/lead/统计失效在补充 FR 追踪 |
| PRJ-014 | 待闭合 AR-04/CR-02 | AC-26：租户/证据/下载/缓存权限覆盖；真实旧端探测与移动验证补齐 |

27 项 AC 均有 fixture、动作、断言、测试层和 owner，未发现遗漏 P1 AC 或将其换成更弱目标。P1-FR-01—18 也逐项进入测试表：01 字段、02 日期、03 时区七日、04 属性并发、05 快照、06 证据复核、07 发布竞争、08 删除撤权编辑、09 不完整/历史署名、10 全 writer 删除、11 故障删除、12 通知恢复、13 混合版本、14 回退、15 WS/日界、16 真实身份、17 安全呈现、18 零执行副作用。需加强 05/06/09/10/11/12/13/15 的合同或平台证据，分别对应上面 AR/CR；不新增产品范围。

## 实施模拟

1. **目标模板 + 两人描述编辑：** ui 复用编辑器追加/flush，core 持 adopted description revision；foundation 锁内按实际字段合并并 CAS，409 返回授权后的最新正文。原描述进入执行上下文，图标变更不让验收失效。路径完整，可实现，不需要新目标实体。
2. **人工验收 + 统计 + 丢响应重试：** progress 收 preview 后在成员 fence/project 锁内查证据和描述，调用 health 采集，写 revision/request/outbox。到“调用 health”无法判定默认写事务是否同快照；到“返回 latest/历史”仍需猜 DTO/选择顺序；到投递可能与删除反锁。AR-01/02/03 必须在实现前固定。
3. **新桌面接旧服务端 + 移动兼容：** 依当前设计调用 capabilities 会得到旧 GetProject 的 400，不能进入预期降级；之后即使根检查全绿，移动 API/realtime 改动仍未被执行。AR-04/CR-02 是明确的可复现规划缺口。

## 修订后准出

由 Planner 一次性修订父设计/实施/测试规格及五子任务的受影响依赖/上下文，保留原范围和未实施状态；然后 Architect → Critic 顺序复审。复审要求看到六个问题的具体文档修正与对应测试设计，不要求实现结果。A/B 的权衡可保留现有选择；补活跃大项目翻页频繁刷新观测即可，无需现在引入持久健康快照、通用消息框架或 I1。

本轮没有发现其他需要扩大 P1 或重新访谈的问题。仅写入本评审文件，未修改计划或产品代码。
