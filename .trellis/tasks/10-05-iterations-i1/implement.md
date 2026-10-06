# I1 实施次序与门槛

2026-10-06：FG 已通过；LG/HG 并行实现和集成验收已完成，提交 `1247d2728`、`4c9be7652`。当前证据见 [LG/HG 验收记录](lg-hg-verification.md)，下一业务阶段为 CG。保持发布开关关闭，foundation 保留至 FCG。下方顺序检查单保留为阶段合同。

## 交付阶段

| 子任务 | 写入所有权（实施时） | 启动前置 | 完成证据 |
| --- | --- | --- | --- |
| foundation | schema/migrations/sqlc、共享时区、server iteration 事务框架与持久 operation/hash/result 基础、最小事实 recorder/事件持久化、writer inventory 与接线整合、共享 router/main | 已评审工程方案及实施指令 | FG：schema/锁序/全部相关 writer 事实采集、持久操作/授权、显式清理、单 active、旧字段保留、性能基线与迁移测试；所有后续 shared SQL 最终整合后才归档 |
| lifecycle | iteration handler/service 创建/开始/编辑/归属/取消空计划、T1接受集成 | FG | LG：完整真实API与create/edit/start/move重放、批量原子/权限矩阵通过 |
| history | 持久事实到统计投影、图表、原始承诺规则/冻结 payload DTO | FG；可与 lifecycle 的独立文件并行 | HG：真实事实投影与算例、时区、冻结 payload 测试通过；最小 recorder/writer 接线归 FG，真实关闭后不漂移归 CG/VG |
| closure | end/handoff/disable 的操作编排、outbox、逾期提醒 | LG+HG | CG：真实数据库竞态/故障注入/去重/整体禁用通过 |
| clients | core API/query + views + Web/Desktop wiring；移动兼容必要改动 | FG 后实现 DTO fixtures/core，随 LG/HG API 分段联调；完整联调需 CG | UG：两端完整闭环、旧服务/客户端/移动兼容和无障碍完成 |
| verification | e2e、性能/迁移演练与验证记录 | 可提前写矩阵；最终执行需 FG+LG+HG+CG+UG | VG：29项 I1 验收与跨模块场景逐项证据、正式 CI 和关闭开关发布准备 |

树结构不是依赖调度器；task.json meta gates 与本表相互核对。FG 只是 foundation 内部里程碑（含持久幂等底座），不能伪装任务整体完成来解锁其余任务。foundation 最终完成门槛 FCG=LG+HG+CG+UG 后全部 shared SQL/router/sqlc 接线整合与回归通过；FCG不是其余任务的启动前置。基础阶段不得启用功能。

## 顺序检查单

1. 完成 handoff S0：以 c96b63c09 刷新全部 writer 和 P1 事务增量；P1 时区与 550–566 迁移已有基线，后续新增 schema 才继续分配编号。在 research/writer-inventory.md 登记 path/symbol/事务/锁序/测试。冻结合同 fixture、PRD 默认建议，测普通任务写吞吐基线。
2. 完成 S1 的最小 recorder + W01 垂直切片，再在 S2 扩展 W02–W17、操作对账/授权和清理，补真实失败回归。保留已有单 active/迁移证据，SQL 改动后再生成 sqlc。只有 handoff §6 全部 FG 检查通过才放行 LG/HG。
3. 完成 lifecycle 与 history 的独立实现；同一 SQL/router/generated 文件统一交 foundation 整合。记录 source/target fence、scope_revision、原始事实与所有状态写入。LG/HG 不能只靠 mocked 测试。
4. 完成 closure：预览全量、持久幂等、单事务 end/handoff/disable、outbox 重试与撤权。故障注入每一个中间写点；证明回滚无部分计数/通知。
5. 完成双端共享页、任务创建/详情/批量和项目过滤、设置、图表表格、失败保留输入。能力探测和 Mobile 兼容可用真实旧服务器 fixture 验证。禁止无确认端直接写 closure。
6. verification 对 test-spec 逐项记录 commit、命令、结果、实际截图路径和限制。先窄检查，合并后 `make sqlc`（产物无漂移）、`make test`、`pnpm typecheck`、`pnpm lint`、`pnpm test`、`pnpm exec playwright test` 与 `make check` 按仓库实际脚本执行；测试环境须归属当前 checkout。Go vet/静态分析使用 make check 定义。只文档规划不运行产品套件。
7. additive schema→服务端→能力→客户端→试点显式启用。先对 1,000 任务 detail/preview/closure 和禁用多计划集合测 P95/锁等待；预发布记录阈值和上限（PRD 2s 为建议），不将建议当通过。CI 绿及版本发布按 CLAUDE.md 与现有 release workflow 单独执行，不在本规划任务推标签。

## 交接与回退

每个 gate 在该子任务 verification.md 写真实证据，不默认创建 PASS 文件。当前分支已包含 P1 实现，联合删除场景必须按真实 P1 路径补跑，未验证的场景继续标 pending。P1 已拥有共享时区迁移和配置 API，I1 直接复用。

建议后续一名 executor 先完成 S1，再拥有 foundation/共享改动，lifecycle 和 history 两个有边界的 executor 在 FG 后并行，closure 接续，clients 可用 fixture 提前准备，verifier 独立验收。只有运行环境支持 OMX 且用户明确要求团队模式才启动 OMX；普通 App 使用原生代理或单人顺序，禁止把运行时口令当实现前提。

任何失败保持功能关闭并前向修复；数据产生后不能退回不懂迭代事件的旧 server。禁止为赶发布删历史、拆分结束事务或忽略未覆盖 writer。
