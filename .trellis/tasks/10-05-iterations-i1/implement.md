# I1 实施次序与门槛

2026-10-06 用户明确授权实施。沿用已评审方案，父任务进入 in_progress，按 FG→LG/HG→CG→UG→VG 门槛推进。当前沙箱限制 .git 写入与网络；不绕过限制，验证与提交状态分别记录。

## 交付阶段

| 子任务 | 写入所有权（实施时） | 启动前置 | 完成证据 |
| --- | --- | --- | --- |
| foundation | schema/migrations/sqlc、共享时区、server iteration 事务框架与持久 operation/hash/result 基础、writer inventory 与接线整合、共享 router/main | 已评审工程方案及实施指令 | FG：schema/锁序/状态 writer 覆盖、单 active、未知字段保留、迁移测试；所有后续 shared SQL 最终整合后才归档 |
| lifecycle | iteration handler/service 创建/开始/编辑/归属/取消空计划、T1接受集成 | FG | LG：完整真实API与create/edit/start/move重放、批量原子/权限矩阵通过 |
| history | iteration 纯统计、事件采集、图表、原始承诺/快照 DTO | FG；可与 lifecycle 的独立文件并行 | HG：全部 writer events 与算例、时区、删除历史保护通过；接线由 foundation 统一 |
| closure | end/handoff/disable 的操作编排、outbox、逾期提醒 | LG+HG | CG：真实数据库竞态/故障注入/去重/整体禁用通过 |
| clients | core API/query + views + Web/Desktop wiring；移动兼容必要改动 | FG 后可准备 DTO fixtures，真实集成需 LG+HG+CG | UG：两端完整闭环、旧服务/客户端/移动兼容和无障碍完成 |
| verification | e2e、性能/迁移演练与验证记录 | 可提前写矩阵；最终执行需 FG+LG+HG+CG+UG | VG：29项 I1 验收与跨模块场景逐项证据、正式 CI 和关闭开关发布准备 |

树结构不是依赖调度器；task.json meta gates 与本表相互核对。FG 只是 foundation 内部里程碑（含持久幂等底座），不能伪装任务整体完成来解锁其余任务。foundation 最终完成门槛 FCG=LG+HG+CG+UG 后全部 shared SQL/router/sqlc 接线整合与回归通过；FCG不是其余任务的启动前置。基础阶段不得启用功能。

## 顺序检查单

1. 刷新 base commit、扫描全部 issue/status writer、确认 P1 时区迁移所有者；在 research/writer-inventory.md 登记 path/symbol/事务/锁序/测试。冻结合同 fixture、PRD 默认建议，测普通任务写吞吐基线。
2. 先为单 active、计划终态、正式准入、旧字段保留及 schema guard 写失败测试。逐个迁移文件创建索引，无 FK；生成 sqlc。接入 workspace fence，证明既有 T1/P1 锁序无环；尚未启用时普通任务回归。
3. 完成 lifecycle 与 history 的独立实现；同一 SQL/router/generated 文件统一交 foundation 整合。记录 source/target fence、scope_revision、原始事实与所有状态写入。LG/HG 不能只靠 mocked 测试。
4. 完成 closure：预览全量、持久幂等、单事务 end/handoff/disable、outbox 重试与撤权。故障注入每一个中间写点；证明回滚无部分计数/通知。
5. 完成双端共享页、任务创建/详情/批量和项目过滤、设置、图表表格、失败保留输入。能力探测和 Mobile 兼容可用真实旧服务器 fixture 验证。禁止无确认端直接写 closure。
6. verification 对 test-spec 逐项记录 commit、命令、结果、实际截图路径和限制。先窄检查，合并后 `make sqlc`（产物无漂移）、`make test`、`pnpm typecheck`、`pnpm lint`、`pnpm test`、`pnpm exec playwright test` 与 `make check` 按仓库实际脚本执行；测试环境须归属当前 checkout。Go vet/静态分析使用 make check 定义。只文档规划不运行产品套件。
7. additive schema→服务端→能力→客户端→试点显式启用。先对 1,000 任务 detail/preview/closure 和禁用多计划集合测 P95/锁等待；预发布记录阈值和上限（PRD 2s 为建议），不将建议当通过。CI 绿及版本发布按 CLAUDE.md 与现有 release workflow 单独执行，不在本规划任务推标签。

## 交接与回退

每个 gate 在该子任务 verification.md 写真实证据，不默认创建 PASS 文件。当前分支已包含 P1 实现，联合删除场景必须按真实 P1 路径补跑，未验证的场景继续标 pending。P1 已拥有共享时区迁移和配置 API，I1 直接复用。

建议后续一名 executor 拥有 foundation/共享改动，lifecycle 和 history 两个有边界的 executor 在 FG 后并行，closure 接续，clients 可用 fixture 提前准备，verifier 独立验收。只有运行环境支持 OMX 且用户明确要求团队模式才启动 OMX；普通 App 使用原生代理或单人顺序，禁止把运行时口令当实现前提。

任何失败保持功能关闭并前向修复；数据产生后不能退回不懂迭代事件的旧 server。禁止为赶发布删历史、拆分结束事务或忽略未覆盖 writer。
