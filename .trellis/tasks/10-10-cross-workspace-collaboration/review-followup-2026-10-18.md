# 评审后的实现核对与推进计划

## 结论与依据

需要调整当前实现、集成验证和进度记录。沿用已评审的产品方向及 M09 单直接智能体切片，不重新开启产品选择。最新依据是 [讨论 §64–§66](discussion.md)、[单智能体范围](continuation-single-agent-2026-10-17.md) 和当前工作区；`review-2026-10-17.md`、`progress.md` 的旧结果只证明各自日期的范围。

用户本轮要求先说明结论和调整计划，再根据当前进度调整或继续开发。已恢复原 `in_progress` 任务，不新建任务；开发、检查授权沿用当前指令。开始时 HEAD 为 `d62d6169c`。既有未提交文件的完整内容与哈希保存在 `/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-review-followup-20261018-iluytkmz/baseline/` 和 `baseline.json`，用于区分本轮修改。

## 已有基础与调整项

| 范围 | 当前代码事实 | 本轮处理 |
| --- | --- | --- |
| 内容与通知 | 评论、逐目标回执、通知和输入同事务；编辑/删除不改输入快照，多作者保留各自身份 | 保留，进行相关回归；不宣称附件文件交付或实时客户端已经完成 |
| 实际 owner | 登记/确认要求合法参与关系，并绑定实际资源拥有者及当前身份代次 | 保留；完整第三账号加入/退出的业务验收仍单列 |
| M10 与批次 | 前向旧执行清理、根 agent 通道、等待批次、单 lane 恢复及 never-launched 结清已写入代码 | 验证现有实现；不重复开发已存在的模块 |
| 业务结果协议 | Runner/Client 使用 `succeeded/failed`，服务端接收 `success/failure` | 统一业务 outcome 为 `success/failure`；command/group 的终态继续使用 `succeeded/failed`，增加真实边界回归 |
| core 权限恢复 | 异常路径只检查账号会话，未检查请求捕获的对象访问代次 | 修复迟到拒绝覆盖已恢复权限的问题；现有定向测试已复现失败 |
| v3 读取投影 | 新测试/fixtures 已要求 channel/batch 身份，服务端投影和 core schema 仍为 v2 | 同步两端 v3 DTO、批次/别名关联与严格校验；不开放客户端执行能力 |
| 并发 FIFO | 批次输入查询先按事务起始 `created_at` 排序，再按 seq；真实重叠事务已复现 claim 颠倒输入与根作者选择 | 使用单直接目标的权威入队顺序，保留混合目标当前行为；跨目标 FIFO 仍留在后续阶段 |
| 原生闭环 | 已有真 DB/router/HTTP Runner/Linux namespace 测试，但还没有这版代码的非跳过通过证据 | 修复真实集成失败并执行现有测试；补 daemon managed 调度接线的有界回归 |
| 新评审后续项 | target hold、同队长多目标批次、M11 小队并入及多 lane 部分恢复尚未完整实现 | 保留在 M09 之后，避免按旧通道暂停模型继续扩建 |

## 本轮实施顺序与责任

1. **协议与运行接线**：在 `server/pkg/protocol/collaboration.go` 固定业务结果值，同步运行端、服务端及假程序；保留未知启动、nonce 墓碑、真实封闭证明和分离凭据的门禁。修改仅限该协议及其直接消费者、测试。
2. **core 迟到拒绝**：在 `packages/core/collaboration/access.ts` 的实际错误处理边界核验捕获 epoch；使用既有失败测试验证恢复后的新权限不被旧请求撤销。不开新 UI 工作。
   - 扩大验证发现的当前接线缺口一并修复：服务端 `collaboration_execution_view.go` 与 core `execution-schema.ts` 同步已有 v3 测试契约。两端直接消费者使用同一 channel/batch 关联，严格拒绝未知版本/缺失证明/跨通道冒用，并移除 M10 已废弃的 legacy 投影。已有 16 项 core 失败与类型错误作为反例，不降低断言以通过测试。
3. **单目标 FIFO**：在 `collaboration_batches.sql` 及服务端批次测试中复现事务开始时间与实际入队顺序相反的情形，修复后由主会话串行生成 sqlc。没有并发反例前不做顺带重构。
4. **真实集成**：使用本轮专用数据库与只含测试制品的 Linux 环境，运行评论→合批→claim/permit→监督启动→Stop→sealed proof→Resume→下一批和撤权后最小收尾。区分真实 DB/router/Runner 验证、daemon 主调度回归与普通模块测试。
5. **独立检查与记录**：运行相关 Go race/vet、core 测试、类型检查/lint、迁移与生成一致性检查；独立 Trellis 检查后更新当前进度、任务元数据和证据。完整任务保持 `in_progress`，不将单切片通过写成 97 项 AC 完成。

实施由独立 Trellis 代理按上述文件边界负责；主会话持有任务资料、共享生成文件和最终集成验证。所有人保留工作区已有修改，不提交或覆盖其他人的工作。测试仅使用自行创建的假可执行文件，不调用供应商 CLI。

## 后续顺序

M09 通过后，先实现目标级 hold 与跨 lane 部分恢复，再实现同队长 agent/squad 合批及换队长的等待输入迁移，随后按 M08 保留现有评论派发并实现 M11 并入/续作，最后接完整 Web/Desktop 流程与剩余 P1–P8 验收。

### 本轮途中 Claude 文档再次更新后的增量

用户要求重新读取文档，随后明确继续推进。已比较当前文件与本轮开始的副本，并保存最新文档哈希到证据目录的 `latest-claude-docs/manifest.json`。本次增量不改变 M09 顺序和当前四项修复；以下约束纳入后续实施与独立检查：

- 资源撤回、配置漂移及账号失效置目标 hold，并停止受影响组，**不暂停 lane**。新目标初始 `initial` hold；consent/重新授权不释放；promote 发现无效目标时同事务置 hold。组结算后同 lane 其他有效目标继续推进。
- 释放目标 hold 与解除 lane 暂停分别结算。session active 且本目标配置/授权有效时可先释放 hold；lane 证明不足仍 paused，回执必须明确两部分的结果。Resume All 分别返回 lane 与 hold 目标结果。
- M11 未读取输入不冒充已开始批次输入。取消、作者移除或仅为该条增加的 grant 失效，只摘该条与专用追加 grant；已读取输入失效才触发整组停止。组正常/异常结算及续作永久 prepare 失败时，未读取输入均回到 waiting、保留原顺序。
- M11/派发先取 gate FOR SHARE，再按共同顺序锁 lane→group；取消已并入 waiting 也锁 group。Switch 按 UUID 锁新旧 lane，冲突整笔重试，迁移 waiting 保持相对顺序并取新 lane 序号。
- M08 继续沿用评论派发；@队长只触发/合并续作，合法运行 @当前参与者按 R52 通知。briefing 的不支持子任务指派返回 `collaboration_subtask_unsupported`。

这些是完整目标模型的约束，当前单目标文本执行与 v3 读取通过不代表 target hold、跨目标 FIFO、M11 动态并入或上述全部控制回执已实现。

附件固定版本、Git/工作目录准备、通用供应商适配、legacy enrollment 启动、macOS/Windows 原生封闭证明及发布均不由文本 fake 单智能体闭环替代。当前运行能力只按实际验证的 OS/controller 声明。

## 本轮验证

本轮 M09 已通过实现、集成与独立复核，详见[机器可读验证汇总](verification-summary-followup-2026-10-18.json)及[独立报告](research/verification-followup-final-2026-10-18.md)。核心结论是单直接智能体文本执行闭环可验证，不等于完整产品已交付。

| 检查 | 结果 |
| --- | --- |
| Go 协作 service/handler/router/迁移 race | 300 顶层、517 子测试通过，0 失败 |
| Go daemon/运行监督器/CLI race | 1,995 顶层、1,987 子测试通过，0 失败 |
| core 全量 | 261 文件、3,484 测试通过 |
| 全仓 typecheck / lint | 9/9、6/6；既有非阻塞 lint 警告保留 |
| Linux 原生范围 | 真实 DB/router 4、daemon 2、运行端 18 个顶层用例通过；包含逃离进程组、双 fork、提前关闭 stdio、监督器崩溃、撤权与丢 ACK |
| 数据与静态 | 空库迁移至 697、0 invalid index；M10 保留回归；82 个索引恢复登记及真实重试；Go vet、gofmt、sqlc 一致性、58 R/97 AC 和上下文校验通过 |

宽回归之后，独立检查发现编辑测试的资源 fixture 会重加已退出来源的发起人。现改用第三个完整参与者作为独立 owner，新增编辑前身份断言，原断言保持；已复现失败并让三种角色的 race 回归通过。最新源码哈希见[源文件证据](verification-source-manifest-2026-10-18.json)。

宽回归收尾：真实 Linux 三组测试和 core 全量已通过，service/旧路由回归暴露批次切换后的测试前置条件不一致，以及共享 proof 结算遗漏孤儿派发检查。先按实际批次开始时点修正旧测试 setup、补齐路由 fixture 的 channel/batch 身份、同步故障注入的真实 SQL，再以原断言验证业务行为。生产修复限定 `collaboration_groups.go` 的既有共享完整性检查：所有保留派发（包括取消墓碑）必须有本组父 run，检查先于释放预约/结算；复用 `validateGroup`，保留 Stop 的原约束。实施代理连续速率限制后由主会话接管这组有界修复，独立复核保持分离。进入 hold、多通道或 M11 前须重新完整读取届时最新执行契约。

本轮代码提交：`188a7f4a5`（后端单智能体闭环与迁移），`6794f4e36`（core 协议与账号读取）。工作区承接的同任务基础和本轮修复一起形成可复验提交；完整任务继续进行。
