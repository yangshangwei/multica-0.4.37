# 迭代结转性能门：只读续查

**原性能门继续 FAIL，当前尚未确认产品根因。** 本片只读取冻结候选 `1a32ca36c2c4f4455a01bdd39d0e0ab4db20cccc` 的代码与本轮原始证据；未连接数据库、运行测试、构建、浏览器或 Provider，也未修改产品、SQL、阈值。分析输出只有本文件和同名 JSON。冻结源码根为 `/Volumes/artisan/code/2026/mc-core-e2e-1a32ca36c-20261104`，下列代码行号均指该副本。

## 当前样本支持什么

重新计算 `evidence/performance-1/samples.json`，去掉原定前 5 次预热后，30 次结转中 **7 次超过 2,000ms**；P95 仍为 **2,155.025ms**，中位数 **1,792.6445ms**。没有删除样本或另选分位算法。

原始样本按时间顺序呈明显分段变化，表中索引从 0 开始：

| 周期 | 结转中位数 ms | 详情中位数 ms | 预览中位数 ms |
| --- | ---: | ---: | ---: |
| 0–4，预热 | 2,125.136 | 68.763 | 353.699 |
| 5–10，正式前段 | 2,124.960 | 67.351 | 355.275 |
| 11–25，正式中段 | 1,802.250 | 57.419 | 325.445 |
| 26–34，正式后段 | 1,263.066 | 58.077 | 328.614 |

这些分段是描述现有数据，不是修改验收算法。结转下降幅度明显大于只读详情／预览，且历史行随周期积累；因此“任务越多线性变慢”不能单独解释这轮时间趋势。缓存、统计与计划改变、共享负载等仍需实测区分，不能直接认定自动 ANALYZE 已发生。

原始证据报告 `sql_statements=643369`，但它是测量 handler pool 的整段计数，包含初始 move、每周期 start、详情、预览、Apply，以及构造后续 draft 的 GetIssue 读取；**不能当作一次结转的查询数**。计数器只增加 Query/Batch.Len，未记录阶段耗时，见 `server/internal/handler/iteration_capacity_benchmark_test.go:121`。

Lock 观察器共 145,938 次采样、0 次命中，只查询 `wait_event_type='Lock'`，约每 1ms 一次；不覆盖短于间隔的锁、I/O、pool 等待、CPU 或 GC，也没有单请求归属。见同文件 `:159`。它使用单独连接，观察器查询不属于上述 measured-handler SQL 计数。现有开始／结束进程快照也不是全程 CPU 或数据库负载轨迹。

## 实际调用路径与可验证的工作量

`ApplyIterationOperation` (`server/internal/handler/iteration_management.go:159`) → `IterationService.Apply` (`server/internal/service/iteration_lifecycle_apply.go:31`) → `RunOperation`/`RunTransaction`：当前权限和有序锁、durable request 检索、真实 mutation、结果落库与提交。事务是 READ COMMITTED，最多三次指定数据库冲突重试；当前样本没有每请求 attempt 明细，不能从最终 200 证明每次都只尝试一次。见 `server/internal/iteration/operation_execute.go:26` 与 `transaction.go:23`。

本性能用例是 1,000 个内置 todo、active 源迭代 → 全新 planned 目标，operation=`end`，不是 handoff。start 在计时结转之前执行，因此 `captureStartedOriginals` 的工作不能误计为这次 closure。原测量边界见 `server/internal/handler/iteration_i1_p95_test.go:85`。

| 路径 | 此负载每次结转的源码推导量 | 证据 |
| --- | --- | --- |
| 活动 task 查询 | 1,000 次串行 `ListActiveTasksByIssue`，即使结果为空 | `service/iteration_lifecycle_preview.go:290` |
| source participation 读取并锁定 | 1,000 次 | `iteration/membership.go:53` |
| target participation 读取并锁定 | 1,000 次；全新目标允许 ErrNoRows | 同文件 `:63` |
| 状态类别查询 | 内置 todo 无额外 SQL；自定义状态才查询 | `iteration/record.go:51` |
| 成员写入 | 7 个有序 batch，各 1,000 条，共 7,000 条 SQL | `iteration/membership_batch.go:96` |
| 上述 batch 中最近事件子查询 | 约 4,000 次：2,000 append、1,000 source leave、1,000 target join | `pkg/db/queries/iteration_membership_batch.sql:4` |

这是**当前源码证明的机制计数，尚不是本轮测得的阶段归因**；另有少量权限／设置／scope／history／snapshot／notification／commit 工作。7,000 条写 SQL 已使用 pgx batch，不等于 7,000 次独立应用网络往返。每一 phase 消费并关闭结果，再进入下一 phase，必须保留 source leave 在 target join 之前、CAS 及 rollback 语义。

Apply 在 preparation 末尾为涉及的迭代读 history（`service/iteration_lifecycle_preview.go:481`），freeze 又读取源 events/history 并构造不可变 snapshot（`service/iteration_closure.go:125`），全部 membership 写之后才设置 processed time 并持久化 snapshot（同文件 `:254`）。这些看似重复读取具有事务时点与历史一致性约束，未测量前不能直接删掉或缓存。

## 已有计划证据的边界

原测试在周期开始前打印了三条 `EXPLAIN (ANALYZE, BUFFERS)` 的只读查询：

- events：估计 1 行、实际 1,001 行，Bitmap Index/Heap Scan 后 Sort，执行 2.185ms。
- members：估计 1 行、实际 1,000 行，使用 `iteration_issue_current`，执行 0.701ms。
- labels：Nested Loop，3,000 行，执行 3.996ms。

计划估计偏差是检查统计信息的理由；它们**不是** membership 的 `ORDER BY sequence DESC LIMIT 1` 规范 SQL 实际缓存计划，更不是正式样本 6 或 26 时刻的计划。不能据此断言每个 leave 都扫描／排序全部事件。现有 `iteration_event_sequence(iteration_id,sequence)` 索引已在原 read plan 中出现，声明见 `server/migrations/559_iteration_event_sequence.up.sql:1`；目前没有依据新增索引。完整解析结果在同名 JSON。

11-02 旧候选 `7af9ab...` 的阶段诊断（串行读 731ms、批写 1,279ms）、prepared-plan/ANALYZE 对照可指导设计，但不是本候选 1a32 的时延证据，不能直接解释当前尾部样本或复用旧许可／数据库。

## 三个排序后的可证伪假设

1. **最近事件子查询在批写阶段受统计／实际缓存计划或缓冲状态变化影响。** 它最能解释结转特有的阶梯下降。若成立，源 leave、目标 join 或事件 append 的完整 batch 时长应占主要部分，实际规范计划／访问行数／buffer 指标应随慢段与快段变化；若批写耗时很小且计划、访问量稳定，则不能解释主要超标。当前只有源码与前置读计划，未证实。
2. **3,000 个串行读取造成可优化的固定传输／解码成本。** 若成立，仅 Apply 请求内的 pgx trace 应显示 1,000 active-task + 2,000 participation 及显著累计时长；若该部分很小，则不是主要瓶颈。这个恒定数量本身不能解释后半段阶梯下降。只有确定占比后，才考虑维持规范 SQL／typed scanners／锁顺序的传输对照。
3. **观察器或共享执行环境影响耗时。** 1ms observer、共享主机和 x86_64 PostgreSQL 镜像在 arm64 主机运行均是潜在因素；目前没有因果证据。若成立，pool acquisition、query/commit、GC 或数据库等待分类可看到不由行数解释的开销；严格匹配的 observer ON/OFF 诊断可进一步区分。单凭架构、一次进程快照或 0 个 Lock 样本，不能把失败改判为环境通过。

## 下一步：一次有归属的诊断，而非重新抽 P95

建议下一明确资源窗口运行一个新的 `TestIterationClosureAttributionDiagnostic`，放在 task-private Go overlay 的新增 `_test.go`；不替换产品、SQL 或原冻结测试。root 分配一个全新、已迁移、空业务数据的专库，HOME/TMP 必须在所有 Git 工作树外，保留 CLI guard、固定工具和源码哈希，no-race/count1/parallel1。

该单次诊断保留原 1,000 todo／10 项目／owner／5 优先级／每任务 3/6 标签负载，只完成一次实际 end closure；初始 move、start 和 preview 使用真实入口，**不计入 closure span**。建议保留原 Lock observer 以明确测量条件，但全部 tracing 都由 Apply 的 context marker 限定：

- 每类规范 SQL 的调用数和 Query/Scan/Rows.Err/Close 完整耗时；7 个完整 batch span 只加一次，逐 statement callback 仅作计数。
- Begin/acquire、各事务尝试与 Commit 耗时；提交错误、SQLSTATE 及 retry 次数。
- 请求前后 pool 计数、进程级 GC／allocation 增量；不把 residual 自动命名为 CPU 时间。
- 当前实际后端 PID、索引/统计元数据与 prepared statement 身份，作为之后选择性计划捕获的基线；第一步不 ANALYZE、不改 plan_cache_mode、不切 planner/index 开关。
- 实际仍核对 1,000 项只结转一次、所有 snapshot 保留 priority 与 3 个 labels、source closed/target planned、指针/历史一致、运行中 task 无新增执行等产品不变量。

**决策规则：**先看哪个阶段占主要时间。若 event-last 批写最重，再在新隔离诊断后端捕获规范 `SendBatch` SQL 的实际 prepared plan；普通 EXPLAIN 前缀计划不足以代表该路径。若串行读最重，再做 typed transport 的等价性探针；不能将权限检查移到锁前，不能将等待后新 RC snapshot 变为旧 CTE snapshot，不能吞 ErrNoRows/Rows.Err/Close 错误。只有这些证据到位才提交产品优化。

这一 closure 能证明当前机制的时间构成，**不能**证明 P95 修复，也无法单独还原原35周期第11/26段切换原因。若要调查切换，必须另有明确、预先固定的阶段观测方案；不能反复跑完整门挑绿色结果。

## 单次探针的具体落点与回滚合同

- 实体文件建议：`.omx/private/core-e2e-20261104/conditional/iteration-attribution-1/iteration_attribution_test.go`。
- overlay 虚拟目标：冻结副本 `server/internal/handler/iteration_attribution_diagnostic_test.go`，目标文件不落到产品树。
- 用例名／筛选：`TestIterationClosureAttributionDiagnostic` / `^TestIterationClosureAttributionDiagnostic$`；一个新编号执行、一个 root 明确分配的新库。
- 第一探针走真实 `Apply` 并提交一次，以保留真实 Commit 时长及提交后的1,000项不变量。不要把 Commit 偷换成 Rollback 后仍声称真实完整闭环通过；证据库保留，后续清理由root负责。
- 准备、CAS、snapshot 或 operation 写失败仍由原 `RunTransaction` 整体回滚。若另外增加具名故障对照，需要核对 membership/event/snapshot/operation 无部分提交，不能混入性能值。
- 若 session 已支持 `auto_explain`，可在这个任务专用后端用 session-only 配置与 `OnNotice` 捕获规范批写的真实计划，绑定 PID、prepared identity、SQL及参数人口；不修改全局设置。初始保持现有计划／索引选项。若工具不可用，报告缺口，不用 EXPLAIN 前缀冒充实际缓存计划。
- 后续若单独 EXPLAIN ANALYZE 写 SQL，必须另用新诊断事务并显式回滚，比较 source/target/issue/participation/event/snapshot 的逻辑指纹；不声称回滚恢复了 WAL、buffer、tuple version 或统计元数据。

这份计划尚未生成或执行探针代码；当前只交付可审阅的落点和合同，避免在H1窗口内运行。

## 后续改动应保留的回归入口

- `iteration/membership_batch_test.go:12`：规范单条与 batch SQL 等价。
- `service/iteration_closure_test.go:497`、`:517`、`:561`：多目标、交叉交换来源、任意 CAS 失败整事务回滚。
- 同文件 `:125`、`:443`、`:465`、`:853`：request replay、snapshot 写失败回滚、逻辑时钟与结转事件计数。
- `handler/iteration_closure_writers_test.go:133`、`:255`、`:325`：真实任务／项目写、成员撤权与状态目录竞争。
- `service/iteration_closure_test.go:712`、`:785`：daemon 完成屏障及在途执行不被迭代管理重启。

验证限度：只对原 JSON 样本重新计算、解析原计划 JSON、核对源码引用与哈希。未运行 lint、TypeCheck、测试、数据库或模型；这是按 H1 活动窗口要求的只读分析，不是新的性能通过证据。
