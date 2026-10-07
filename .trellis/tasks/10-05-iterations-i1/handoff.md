> **本地提交已完成（2026-10-07）：** 已验收源码以五个本地提交 `823a61774`…`e516b15c2` 落在 `codex/projects-p1`，见[提交与源码对应](../10-05-iterations-i1-verification/verification.md#本地提交与已验证源码的对应关系)。下一步只剩单独授权后的推送与正式远端CI；不推送／合并／部署，发布开关仍关闭。下方“HEAD仍为115b4cd28／大量未提交”是提交前的历史状态。
>
> 此前的接续入口：用户已指定新会话完成“核对提交范围、完成本地提交”，见[本地提交handoff](../10-05-iterations-i1-verification/commit-handoff.md)。下文“本轮不提交”指此前验收会话；新会话已获本地提交目标授权，仍不推送／合并／部署。

# I1 最终验收 Handoff（2026-10-07）

**接续位置：本地验收已收尾，UG/FCG 已关单；VG 的正式远端 CI 尚未执行。不要重新开始 CG 或客户端开发。** 下方历史接手记录中的“下一步 CG／客户端不存在”已被本节替代。

- 目录 `/Volumes/artisan/code/2026/multica-0.4.37`；分支 `codex/projects-p1`；HEAD 仍为 `115b4cd28277941a8d9ad8fda5acecf99b999152`。
- 原开发会话 `01a1119c-6652-7230-87ec-e5e6945ee021` 在11:12中断；接手先核对进程与两次源码SHA，未发现继续写入。已有171项dirty入口和全部CG／客户端成果均保留。仍有大量未提交内容，不能把HEAD单独当作最终代码身份。
- FG/LG/HG/CG/UG/FCG 通过；foundation、clients子任务已完成。父任务与verification仍为 `in_progress`，原因只有原计划正式远端CI门槛未执行，不是本地实现或验收仍缺失。
- [最终验收](../10-05-iterations-i1-verification/verification.md)、[29项矩阵](../10-05-iterations-i1-verification/acceptance-matrix.md)、[原PRD界面审查](../10-05-iterations-i1-verification/ui-final-audit.md)、[源码指纹与运行证据](../10-05-iterations-i1-verification/source-provenance.json)是当前事实来源。

## 已完成与证据

完整基线 `full-check-accepted.log`：10,215 TS tests、15/15静态任务、全Go race/vet、生产构建、249 E2E通过／49跳过。接手验证基线快照的五项差异均在10:38重跑前完成，其他6,232项一致。

最终只补缺失证据／界面遗漏：真实迁移INVALID恢复及部分回退、真实F移出后外部完成、真实父子计数；手动模式说明、终态可读选择、冻结任务事件身份、保存时区预览；移动端迭代通知的Web/Desktop提示。无新依赖或生命周期协议变化。

增量验证：views100 tests、移动196 tests+iOS脚本及各自typecheck/lint；迁移race2顶层/4子项、History15顶层/21pass事件；最终生产Web与原生Electron四项I1 E2E全部通过，0skip/0retry，24.2s；视觉94/pass，12张截图带SHA。6,696个运行／测试文件与最终冻结输入一致，87个sqlc生成文件无漂移。不要把重叠测试数相加，也不要称最终小修后重跑了249项全套。

性能原阈值复算仍通过：1,000项detail/preview/closure warm P95为57.992/324.154/1,099.838ms；1,530项disable preview/apply为459.041/957.511ms；普通四写者关联任务426.3/s、P95 12.10ms。仅代表记录的本机warm负载。

## 后续边界

1. 本轮不提交、不推送、不合并、不部署；`iterations_i1`默认false。完整VG所要求的正式远端CI仍待未来单独授权的提交／发布阶段，不能填写假的CI链接或标无条件passed。
2. 以后要提交时先重新核对并发会话与当前dirty文件；接手前已有的`.agents/`、`.codex/`、Trellis模板等变动不能顺手混入I1提交。保留Lore提交协议。不要自动归档现有任务树。
3. 旧客户端兼容由wire/schema／真实旧请求／mobile序列化证明，未运行历史发行二进制；mobile完整编辑和真机视觉仍未交付，点击通知已有明确支持版本提示。完整辅助技术、其他OS／浏览器不冒称覆盖。
4. [49项跳过说明](../10-05-iterations-i1-verification/skipped-tests.md)全部为独立fixture，I1没有跳过；这些场景仍未运行。部分旧`/tmp`原日志已过期，保留的完整race和内嵌性能样本证据边界见source-provenance。
5. 新迁移测试库已确认所有权后删除；本轮启动的API/Web已停止；原I1 agent测试环境`check-20261006151843-35615`数据库和profile保留。恢复该环境时用其自身ENV_FILE，核对实际API端口与生产rewrites；不要混用另一个check环境的构建配置。

---

# 历史接手记录（截至2026-10-06；仅保留设计／过程依据）

以下状态和下一步描述是当时记录，不是当前任务指令；当前结论以上文及task.json为准。

# I1 后续开发计划与 Handoff

> **2026-10-06 22:20 接续入口：** [CG 最新交接与新会话引导语](handoff-2026-10-06-cg.md)。已核对 FG/LG/HG 通过；下一步 CG，UG 产品实现仍待开发，之后完成 FCG/VG。本文下方原始设计和历史检查单继续保留。

日期：2026-10-06。原规划基线为 `c96b63c09`；本轮从 `da6c0fc65` 实施 LG/HG。当前以 [LG/HG 集成验收记录](lg-hg-verification.md) 和 `task.json` 为准。原规划的[独立审查](research/handoff-review.md)继续有效。

> **当前进度：FG、LG、HG 已通过，代码提交 `1247d2728`、`4c9be7652`。** 发布开关保持关闭；下一业务门槛为 CG，foundation 保留至最终 FCG。

已完成的 S0–S2 不重做；历史 writer 和性能证据见 [S2 记录](../10-05-iterations-i1-foundation/s2-verification.md)、[FG 验收记录](../10-05-iterations-i1-foundation/fg-verification.md)。下方 S1/FG 原始检查内容保留为实现约束。

## 1. 接手位置与当前结论

- 开发分支：`codex/projects-p1`。
- 工作目录：`/Volumes/artisan/code/2026/multica-0.4.37`。目录名不代表当前分支；先检查 `git status -sb`。
- 活动任务：`.trellis/tasks/10-05-iterations-i1`；父任务和 foundation 为 `in_progress`，其他子任务按现有状态推进。
- I1 原始工作区成果已经提交并整合到 P1；`main` 保持 `eb71e299e`，无需再次搬运或回滚。
- `/Volumes/artisan/code/2026/multica-projects-p1` 是保留的 detached checkout，不要在那里继续提交 I1。
- **LG/HG 已验收，下一阶段为 CG；I1 发布开关保持关闭，foundation 等待 FCG。**

事实依据：[分支整合记录](branch-integration.md)、`task.json`、`server/internal/iteration/transaction.go:23`、`server/pkg/db/queries/iteration.sql:1`。

## 2. 已有成果与仍缺的能力

| 领域 | 当前可复用成果 | 后续缺口 |
| --- | --- | --- |
| Schema | P1 536–549；I1 550–566；单 active、参与、事件、快照、operation、outbox 表和独立并发索引 | 后续有 schema 变化从实际最大编号继续分配，不重写已整合历史；不新增 FK |
| 共享时区 | P1 配置复用；I1 创建确认并冻结时区，已覆盖日历/午夜规则 | 客户端按周期时区展示，UG 验收 |
| 历史与统计 | `history*.go` 投影持久事实，复用统计/图表/日历；真实 A–J 和冻结 payload 已验证 | 实际 end/handoff 持久化后不漂移归 CG/VG |
| 事务与操作 | settings/create/edit/start/move/cancel/delete 已接入 `RunOperation`；重授权、重放、操作查询已有真实测试 | CG 复用同一持久协议，不另建事务/重试所有者 |
| SQL | iteration/lifecycle/history/list queries 已整合，84 个 sqlc 产物稳定 | CG 快照和 outbox 写入仍需整合 |
| 生命周期 | 创建/编辑/列表/详情、完整预览、开始/移动、计划取消/删除、普通创建与 T1 归属已实现 | end/active cancel/handoff/disable 和通知属于 CG |
| 客户端 | P1 提供可参考的 schema/query/draft/access/realtime 模式 | I1 core/views/Web/Desktop 产品实现不存在；Mobile 完整编辑和新 CLI 命令不在首期承诺内 |

上述 Go 文件位于 `server/internal/iteration/`。迁移与时区依据：`server/migrations/550_iteration_tables.up.sql:1`、`server/internal/handler/project_timezone.go:77`、`server/pkg/db/queries/workspace.sql:246`。客户端参考：`packages/core/api/client.ts:4483`、`packages/core/projects/progress-draft-store.ts:10`。

上次整合的可信证据：空库全量迁移至 566、真实 P1→I1 升级/仅 I1 回滚、16 个 I1 索引 catalog 核验、81 个 sqlc 文件二次生成稳定、Go build/vet、886 项 Go 检查通过（含子用例）、10,054 项 TS 测试低并发通过、typecheck/lint。既有真实 pg_bigm 检查跳过；完整 Go 全仓、浏览器/Electron E2E、远端 CI 未执行。**这是基线证据，不是以后改动的验收结果，也不是 FG 通过证明。** 详见 [branch-integration.md](branch-integration.md)。

## 3. P1 落地后必须遵守的增量合同

旧 [writer inventory](research/writer-inventory.md) 是旧 main 的检索线索；接手时逐项刷新 W01–W17，不照抄旧事务描述。

| 当前事实 | 实施约束与入口 |
| --- | --- |
| W01 已有 P1 事务重试与关联保护 | `server/internal/handler/issue.go:3256`、`:3278`、`:3336`：保留 workspace KEY SHARE、字段 CAS、附件事务和 project NOWAIT；I1 fence 必须在附件/issue 行锁之前 |
| 项目删除已显式清引用 | `server/internal/handler/project.go:881`、`:885`、`:905`；`server/pkg/db/queries/project.sql:115`：在 project 锁之前取得 I1 fence，逐项记录 detach 事实；保留 current_iteration_id、累计结转及冻结历史 |
| P1 的隔离级别有业务理由 | `server/internal/handler/project_write_fence.go:74` / `:83`：普通 P1 写 RR，项目删除 RC；不能为统一 helper 把两者都改成 I1 的 RC |
| 既有重试预算不同 | `server/internal/service/issue.go:235` 与 `server/internal/iteration/transaction.go:23`：由最外层 owning transaction 统一预算，禁止嵌套重试相乘；保留已发生的错误分类 |
| 创建可能已经持有锁 | `server/internal/service/issue.go:338` / `:374`；`server/internal/handler/lifecycle_handoff_transaction.go:247`：在最外层调整 catalog/I1 fence 顺序，不能到 CreateInTx 才晚补锁 |
| T1 的顺序不可反转 | `server/internal/handler/triage.go:227` / `:233` / `:238` / `:242`：workspace KEY SHARE → T1 settings → member fence，再衔接 catalog/I1 fence |
| 非 HTTP writer 仍需接入 | `server/internal/service/issue_public.go:42`、`server/internal/handler/github.go:1897`、`server/internal/service/task.go:5865`、`server/cmd/server/runtime_sweeper.go:733`；保留各入口原有 CAS、失败重置条件与 commit 后广播 |
| 执行开始是单独事实 | `server/internal/service/task.go:4154` / `:4162`：实际 StartAgentTask 成功才记 started；enqueue/claim/完成、无 issue 的 task 不得冒充开始；管理动作不得重启或停止执行 |
| 批量与清理需要专门覆盖 | `server/internal/handler/squad.go:563`、`server/pkg/db/queries/squad.sql:139`、`server/internal/handler/workspace.go:1264`、`server/internal/handler/workspace_revoke.go:60` / `:219`：租户范围、全事务、锁序、显式 I1 清理及撤权后防重生 |

原始源码行号以 `c96b63c09` 为基准，代码演进后以符号定位。这些是原始定位；当前 writer 接线以 FG/LG/HG 验收记录为准。

## 4. 开发次序与阶段门槛

保留已有六个子任务，不再另建一套任务树。以下切片是可独立提交的工作单元，不是六个子任务的替代品。

| 切片 | 所有者 / 依赖 | 要交付什么 | 可检查的退出条件 |
| --- | --- | --- | --- |
| S0：刷新实施基线 | foundation；已完成 | 确认分支与隔离环境；把 W01–W17 标成真实路径/明确豁免；记录 P1 锁序和普通写性能基线 | 每项包含 symbol、事务所有者、锁顺序、事实字段、回归测试、当前状态；没有未解释入口；不把旧 sandbox 当当前阻断 |
| S1：一个真实 writer 闭环 | foundation；S0 | 最小 recorder + W01 UpdateIssue 接线；详见下一节 | 真实 DB 证明 issue/facts/event/scope_revision 同成败，no-op 不造事件；旧请求字段保留、P1 CAS/NOWAIT/附件回归通过；此时仍不能宣告 FG |
| S2：完成写入底座 | foundation；S1 | 扩展 W02–W17；操作重放/查询、读取授权、settings/capability、空间删除/撤权；补齐所有相关锁协议 | 满足下方 FG 检查单；保留关闭状态，记录普通写吞吐及锁等待；不重新创建时区能力 |
| S3-L：手动生命周期 | lifecycle；FG | enable/create/edit/list/detail、完整 preview、start/move、planned cancel/delete、T1 accept 联合事务 | LG：真 API 的授权、单 active、日期/终态选择、单归属、批量整体成功、同 request 重放全部通过；T1 目标冲突保持 pending、显式执行只一次 |
| S3-H：历史和统计读取 | history；FG，可与 S3-L 独立文件并行 | 读取持久事实、O 基线规则、当前统计/日末图表、历史 DTO、冻结 payload 构造 | HG：真实 DB 事实得到 A–J 的 9/8/5/4、62.5%/50%；固定 O、再入计数、started、时区图表正确；fixture 构造的冻结内容不会依赖实时 JOIN 漂移 |
| S4：原子关闭和通知 | closure；LG+HG | end/active cancel/handoff/disable；快照、去向、累计结转、operation/outbox 同事务；逾期提醒与授权投递 | CG：预览后任一版本/权限/目标变化整体回滚；响应丢失不重复结转/快照/通知；多目标仅一个下一 active；整空间 disable 不分页截断；实际关闭后的历史不漂移 |
| S5：双端及兼容完成 | clients；FG 后开发，按已就绪 API 逐段联调，最终需 CG | core schema/query/mutation、共享页面和操作预览、任务/T1/项目入口、Web/Desktop 路由、Mobile/旧端兼容 | UG：两端真实创建→安排→开始→完成一项→预览结转→历史；403/409/未知结果恢复、键盘、长文本、图表表格、旧字段保留通过 |
| S6：联合验收与交付 | verification + foundation；LG+HG+CG+UG | 完成共享 SQL/router/sqlc 整合 FCG；29 项验收和跨模块矩阵、性能、故障恢复、完整检查与 CI | VG：每项有 commit/fixture/命令/结果；迁移及 P1/T1 不回归；开关仍默认关，发布与试点由明确的发布指令推进 |

### 依赖边界，避免互相等待

- **FG** 拥有最小 `RecordIssueChange`、事件持久化、sequence/scope_revision、writer 接线及一致性。这些不能等 FG 后的 history 来实现。
- **HG** 拥有统计投影、原始承诺规则、图表及冻结 payload 构造。可用真实 DB fixture 验证，不要求 closure API 先完成；真实 end/handoff 冻结及其后续漂移测试属于 CG/VG。
- **LG/HG** 通过测试 fixture 和稳定接口各自推进，联合 start/O 基线在汇合时补验，不以另一子任务“全部完成”作为隐含启动门槛。
- **FCG** 是 foundation 的最终整合完成点，不是 LG/HG 的启动前置；FG 通过不能归档 foundation。
- 现在可以并行写验收矩阵、交互方案和 DTO 示例文档；客户端 fixture/代码实施仍按原计划在 FG 后启动。mock 结果不能通过 UG。

## 5. 历史 S1 交付说明（已完成）

目标：让一次真实 `UpdateIssue` 在原有事务内产生正确的 I1 事实，失败时 issue 和 I1 事实一起回滚。

**写入范围：** 新增 `server/internal/iteration/record*.go` 与同层测试；必要的 `server/pkg/db/queries/iteration.sql`、生成代码；`server/internal/handler/issue.go` 中 `updateIssueAtomicallyOnce` 及对应 handler 测试。新增文件名是实施建议，不是现有实现。共享 SQL/sqlc 由同一负责人修改。

1. 完成 S0 中 W01 的精确锁序核对，明确 actor、operation identity、before/after 事实输入。recorder 接收调用者已有的 `pgx.Tx`，不自行开事务、提交、广播或调执行。
2. 用真实 DB fixture 构造 active iteration/current participation。先让下列回归失败，再补最小 SQL 与实现；不要求先有用户可调用的 start API。
3. 目录锁后、附件/issue 行锁前取得 I1 fence；在锁内读取 before、校验当前授权与引用，调用原 UpdateIssue，再在同一事务记录 after/event/version。保留 P1 关联事务和 project NOWAIT。
4. 事实覆盖：状态类别、标题、项目、负责人及参与内 started；仅描述、附件、评论或 no-op 不制造范围事件。普通 issue revision 引起的保守预览失效与统计事件分开处理。
5. 必须通过：单次事实改变只产生契约所需的一组事件；原始 O 不缩小；no-op 无事件；附件或 recorder 任一失败全部回滚；并发 revision 单赢家；缺省迭代字段仍保留归属和计数；未关联任务同时加入时不漏事实。
6. 回归参考 `server/internal/handler/issue_revision_test.go:454` / `:563` / `:681`、`server/internal/handler/project_association_concurrency_test.go`、`server/internal/handler/issue_reassign_no_cancel_test.go`。新增关联竞争可先由测试事务驱动 join，不提前开放生命周期功能。
7. 在 foundation 的验证记录追加本轮 commit、红绿证据、实际路径和剩余 W 条目；独立提交后继续 S2。S1 完成不宣告 FG，也不启用功能。

## 6. FG 完成检查单

本轮验收通过；每项证据及 LG/HG/CG/VG 责任边界见 [FG 验收记录](../10-05-iterations-i1-foundation/fg-verification.md)。

- [x] W01–W17 每项有“接入”或有依据的“豁免”；每条真实生产路径至少有一个持久事实/明确无事件断言，不能仅测 recorder mock。
- [x] 创建、HTTP/批量/move、Plugin、GitHub、失败重置、T1、源上下文/lifecycle、squad 转移、项目/任务删除、真实执行开始覆盖；当前不存在的状态重分类/跨空间移动不发明新产品功能。
- [x] 两连接 barrier 证明 fence 先于 issue/attachment；55P03/40P01/40001 重试不重复事实；外层预算受限，新 I1 自有操作超限 503，借用既有 writer 保留其错误合同；不嵌套开启新事务。
- [x] same request/same payload 重放；不同 payload 409；未知 commit 用旧 ID 对账；删除后本人可查结果；撤权后不能读或重放保护内容。
- [x] `GET iteration-operations/{request_id}` 有独立的当前授权读取路径：不能要求 GET 客户端提供原 payload hash，也不能把 `LoadOperation` 当鉴权入口（`operation.go:62`、`api-contract.md:23`）。
- [x] workspace 删除同事务清理所有 I1 表；普通 issue/project 删除保留历史；成员撤权清理保护通知并保留 recipient fence 合同；后续投递器在 CG 验证撤权后不重建。
- [x] 旧 HTTP/Plugin/CLI/Mobile 写缺字段保留归属；未确认的显式迭代写按合同 428；pending/rejected/duplicate/未知准入不能进入迭代。
- [x] 功能关闭下普通 issue/T1/P1 回归通过；更新 SQL 后 sqlc 稳定、迁移/索引/共享时区联合检查通过。
- [x] 普通写基线与接线后吞吐、P95、锁等待和查询成本已记录；测试前冻结工程接受阈值，超线修复或记录阻断，不以“默认关闭”豁免回归。

## 7. 客户端实施定位与约束

| 交付 | 已存在的参考入口 | I1 必须增加的检查 |
| --- | --- | --- |
| Issue 字段/schema | `packages/core/types/issue.ts:161`、`packages/core/types/api.ts:6`、`packages/core/api/schemas.ts:1258` | optional 字段缺省表示未知，不伪造 rollover=0；通用更新不得清字段 |
| 能力与写响应 | `packages/core/api/client.ts:4472` / `:4483`、`packages/core/api/project-p1-schemas.ts` | 仅合法空间专用 404 为旧端不支持；401/403/network/malformed 不能降级成成功 |
| 共享时区 | `packages/core/api/client.ts:4498`、`packages/core/projects/p1-queries.ts:10`、`packages/core/projects/p1-mutations.ts:20` | 复用同一 API，修改后同时失效 P1/I1 相关投影；旧 iteration.timezone 不变 |
| 草稿/未知结果 | `packages/core/projects/progress-draft-store.ts:10`、`packages/core/projects/access.ts:66` | 按连接/空间/实体隔离；网络未知复用 request_id+payload；GET404 不生成新 ID；成功不清除更新的草稿；撤权拒绝迟到响应 |
| Realtime/通知 | `packages/core/realtime/use-realtime-sync.ts:992`、`packages/core/types/inbox.ts:5`、`packages/core/inbox/queries.ts:91` | 明确 iteration 与 issue/status/task/project/member 事件的失效表；WS 更新 Query，不复制服务端事实到 Zustand；不按流式消息重算统计 |
| 页面/路由 | 新 `packages/views/iterations/`；`packages/views/layout/app-sidebar.tsx:162`；`packages/core/paths/paths.ts:41`、`packages/core/paths/tab-subject.ts:83`、`packages/core/paths/tab-presentation.ts:169`、`packages/core/paths/route-icons.ts:103` | 共享业务 UI，Web/桌面只做薄接线；Desktop 用 workspace session route；完整 tab identity/标题/图标 |
| 创建/批量/T1 | `packages/views/modals/create-issue.tsx`、`packages/views/issues/components/issue-detail.tsx`、`packages/views/triage/triage-fields.tsx:146`；`packages/core/types/triage.ts:72` | 批量安排走全量 preview；T1 沿用 request_id，能力未确认不发送新字段；不拼通用 PUT 绕过确认 |
| Mobile/CLI | `apps/mobile/data/api.ts:617` / `:816`、`apps/mobile/data/mutations/issues.ts:511`；`server/cmd/multica/cmd_issue.go:1149` / `:1326` | 移动端仅复用 types/pure helpers，验证现有编辑和 WS 不丢字段；CLI 先做兼容/限制，不擅自扩大为完整 iteration CLI |

新增包遵守 `core` 无平台 API、`ui` 无 core、`views` 无路由框架依赖。具体模式见 `CLAUDE.md`。

当 API/CLI 或产品行为影响内置指导，同步 `server/internal/service/builtin_skills/multica-working-on-issues/SKILL.md` 及 source map，P1 联动同步 `multica-projects-and-resources/`。这些是待更新的产品指导，不是本轮执行的外部发信/发布技能。

## 8. 验证、环境与收尾

### 当前环境事实

本机 PostgreSQL 与 Git 写入在整合时可用；当时的隔离验证数据库已经删除，不能复用其名称或把历史失败当当前环境状态。当前目录保留原 `.env`，不能仅因已切换分支就认定它指向隔离测试库。首先 `make status` / `make list` 核对服务与数据库归属；需要开发环境时使用现有 `make up C=api,web ARGS=--ephemeral`，只在确认的本地任务环境运行 DB 测试。`--ephemeral` 只设置 owner/TTL，仍可能复用现有 `.env`/manifest 的数据库，不自动隔离；需要全套隔离验证时使用 `make check` 的专用环境机制。

`make check` 会创建独立 API/Go 验证数据库及专用端口（`scripts/check.sh:33` / `:94`），但需要 `psql` 等已安装工具在 PATH；先检测已安装工具路径，不临时增加项目依赖。原 `/tmp` sqlc 二进制是便利缓存，不是交接依赖；标准入口为 `make sqlc`（`Makefile:402`）。

### 按阶段运行

```sh
# 只读接手检查
git status --short --branch
git log -1 --oneline
python3 .trellis/scripts/task.py current --source
python3 .trellis/scripts/task.py validate 10-05-iterations-i1

# SQL 有改动时生成并检查漂移
make sqlc
git diff --check

# DATABASE_URL 已指向本地任务测试库且所需迁移已应用后；默认测试禁止真实 agent CLI
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -p 2 -parallel 2 ./internal/iteration ./internal/migrations ./cmd/migrate -count=1
# handler/service 的每个切片另加真实入口定向用例；不得只运行上面三包就通过 FG

# 完整汇合使用仓库规定的有限并发，避免首轮整合曾出现的并行资源超时
pnpm exec turbo run lint typecheck --filter='!@multica/mobile' --concurrency=1 --force
pnpm exec turbo run test --filter='!@multica/mobile' --concurrency=1 --force -- --maxWorkers=2

# VG 阶段：隔离环境中的全 Go / Web build / E2E 等完整链
make check
```

Web 与 Electron 使用独立 I1 suite，参考 `e2e/fixtures/project-p1.ts:21` 和 `e2e/fixtures/project-p1-desktop.ts:9`；不能只跑 Web 就声称双端通过。Mobile 兼容需要其本包检查，根脚本排除了 Mobile。纯统计边界留在 Go canonical 层，UI 测试覆盖接线、错误恢复和可访问性。

`test-spec.md` 的 29 项 ITR-AC 及 15 组工程场景是追踪清单。每项记录测试名、fixture、运行命令、commit、结果、实际截图/日志及限制；SKIP、mock、旧提交结果分别标记。1,000 任务详情/预览 P95≤2 秒仍是 PRD 建议（`docs/plans/2026-10-04-work-management-prds/iterations-prd.md:389`），先记录硬件、DB、冷热/分布并冻结验收线；更大集合必须全量成功或 413 零变化，不截断。

## 9. 人员与提交安排

推荐一个集成负责人顺序拥有 foundation、SQL/sqlc、共享 handler/router/main 和锁序变更。S1 先由一个 executor 完成，避免在公共写入口尚未稳定时分散修改。

FG 后可并行：lifecycle 一人、history 一人；共同 SQL 需求交负责人整合。client lane 在稳定 API 上逐段接入，verification lane 可提前维护用例矩阵与环境。closure 等 LG/HG 汇合后执行。只有职责和写文件范围独立才并行，不为凑人数拆分共享事务。

App 会话可使用原生 subagents；不以 OMX CLI/team 为前置条件，不硬编码本地不支持的模型。每块可回滚交付按 Lore 格式独立提交，记录 Tested/Not-tested；合并前另一个只读 reviewer 检查锁序、授权和历史保留。

## 10. 当前继续位置

LG/HG 已通过；在 `codex/projects-p1` 推进 **closure / CG**。先读取 [LG/HG 集成验收](lg-hg-verification.md)、[closure 实施计划](../10-05-iterations-i1-closure/implement.md)和父 API/test-spec；不要重做 S0–S3，也不要打开 `iterations_i1`。

1. 复用 `RunOperation` 和完整预览，实施 end、active cancel、handoff、整空间 disable；全部去向、快照、参与释放和计数必须同一事务。
2. 用真实两连接和中间写点故障注入证明整体回滚；确认后的目标/权限/成员变化拒绝整批，丢失响应重试不能再次结转。
3. 接入持久 outbox、逾期提醒和当前接收人授权，验证撤权防重生、崩溃恢复和可见通知去重。
4. 共享 SQL/sqlc/router 由一名整合负责人维护；UG 完成双端与兼容，随后 FCG/VG 执行完整验收和远端 CI。

LG 的 1,000 项容量证据是单次本地完整性/计时检查，不是 P95 或关闭容量验收。既有 FG 四并发 P95 余量仅 0.037ms，保留原始记录。完整 Go 全仓、Web/Electron E2E、远端 CI 和发布尚未完成。
