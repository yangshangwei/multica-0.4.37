# M09 之后：目标 hold 与分范围恢复

## 本轮依据与顺序

用户在 M09 已验证并提交后要求继续推进。沿用既有任务和实施授权，不新建任务、不重开产品选择。起点为 `5a0817b8a`，工作区干净；已完整重读当前 [执行契约](research/execution-contract.md)、[数据契约](research/data-contract.md)、[PRD](prd.md)、[设计](design.md) 与 [实施计划](implement.md)，并核对讨论 §66 的最后两轮修正。

本轮先落实目标 hold 与 M04 控制边界，再推进同队长多目标 FIFO、切换迁移及 M08/M11。旧 [M09 记录](review-followup-2026-10-18.md) 是历史证据，其“可以先释放 hold、lane 继续暂停”的中途描述已被最新 E05 取代：**Resume target 全有或全无，Resume All 才按 lane 与目标 hold 分别结算。**

## 行为差距和修改边界

当前目标的暂停状态仍镜像通道，资源撤回会通过 `stopTargets` 暂停整条通道，恢复也把通道内全部目标授权绑在一起。目标自身可执行性与人类明确暂停必须分开持久化；否则同队长目标共用通道后，一个失效目标会阻塞其他合法输入。

1. **目标 hold。** `initial`、`grant_required`、`config_invalid`、`switch_hold` 只阻止本目标输入。登记、确认/重新授权、配置变更、撤权、账号失效、资源 teardown 及 promote 发现失效均在原事务写入或保留；仅明确 Resume 可清除。资源失效停止依赖它的组并保留证明屏障，不暂停通道。作者移除/退出仍暂停已读取该作者输入的组所属通道。
   - 目标新增独立 `hold_version` 十进制版本；实际 hold 变更、新 consent/重新授权和配置变更推进该版本，重复原操作不推进。Resume 请求绑定 `expected_hold_version`，阻止撤权再授权后旧请求释放新 hold；lane control_version 不因授权改变而推进。
2. **恢复结果。** Resume target 先验证整单、当前目标配置/授权和所属通道证明；任一失败不写控制状态或 hold。Resume All 先验整单权限、生命周期、gate/epoch、范围版本；通过后同事务解除整单暂停，分别处理 lane 证明与目标 hold，返回两组明确结果。无效授权只保留本目标 hold。被强制停止且仍缺证明的当前组阻止其通道上的 hold 释放。原幂等键始终重放原结果。
3. **真实依赖。** 停止/失效按批次全部输入作者和冻结资源依赖收敛，不能只扫描根 run 的首位作者。保留当前封存证明、未知启动和原始 author/target/batch 身份；所有 helper 借同一事务，按最新 E02 锁序。
4. **读取和 core。** 在服务端投影与 core 边界同步明确的 hold/恢复字段，版本仍用十进制字符串；未知枚举、重复或跨通道身份、缺必要字段均拒绝。执行能力开关按实际交付范围保留，不用新字段暗示完整客户端已可执行。

回执保留当前 `channel_id` 命名，分别返回去重 `channels` 与 `target_holds`；目标读取带 `hold_reason`（可空四值）及 `hold_version`，后端与 core 一起固定此契约。控制回执另带 `issue_id/request_id/action`，通过专用 `GET .../control/operations/{requestId}` 返回原始完整回执；既有人类流转的 operation 查询保持其原契约。

预期文件集中在 `server/internal/service/collaboration_{targets,control,invalidation,batches,groups,execution_view}.go`、原资源 teardown/账号失效入口、对应 SQL 与测试，以及 `packages/core/collaboration/` 和直接 API 消费者。新增迁移从当前最大版本之后分配，已提交迁移不改；主会话串行生成 sqlc 并验证恢复登记。实际资源入口补充 `runtime_teardown.go`、`managed_installation.go` 与直接调用的 runtime/profile/旧 daemon 合并/GC 路径：它们此前只拒新能力，没有在资源删除或重绑事务中保存协作停止。复用同一借用事务失效 helper；资源锁在先的路径用来源 NOWAIT，最多三次完整事务冲突重试，未知 commit 不重试。以现有普通资源测试及新增真实调用者回归约束局部事务提取，不改变普通资源删除语义。

保持既有 service/SQL/权限 helper，移除错误的共享暂停耦合；不引入依赖或并行控制框架。正文/附件交付、通用供应商适配、Web/Desktop 完整交互及 M08/M11 属于后续顺序，不以本轮控制测试勾选完整 AC95 或全部 97 项 AC。

## 验证与责任

- Trellis 实施代理负责明确分配的生产代码与对应测试；Trellis 检查代理独立核对最终差异。所有代理共享工作区，不覆盖其他人的修改、不递归派生。
- 主会话负责任务资料、接口协调、迁移号与 sqlc、隔离环境、集成验证、规范及提交。
- 先用真实 PostgreSQL 回归复现：撤权不暂停 lane；同 lane 有效目标继续；单目标拒绝零写入；整单部分恢复和逐项回执；证明后仍须新键明确恢复；重新 consent 不释放 hold；非根作者失效；teardown 与证明竞争。
- core 增加真正的响应边界负例，覆盖 hold 枚举、缺失/未知版本、关联和回执重放，不用 DOM 重复纯状态矩阵。
- 迭代通过后执行受影响 Go race/vet、core 测试、lint/typecheck、迁移/生成一致性及独立检查；改变运行链路时重跑真实 Linux 测试程序闭环。默认测试不查找或调用供应商 CLI。
- 原始文档快照与基线验证存于本轮独立证据目录，路径见 `research/hold-evidence-path-2026-10-18.txt`。不得以旧 M09 日志代替本轮新增行为证据。

完整任务保持 `in_progress`，每项结果按实际验证范围记录。


## 本轮结果

目标 hold 与 M04 控制基础、当前直接智能体运行准入、真实资源删除/重绑收尾及完整控制回执恢复已实现并通过独立复核。目标 hold 不再借用通道暂停；复用 `stopGroup/validateGroup`，资源入口统一复用完整事务重试，保留原终结证明与真实容量预约。

新增 `hold_version/expected_hold_version`，防止撤权再授权后的旧请求释放新 hold。单目标恢复失败时仅保存原拒绝回执，不改业务状态；整单恢复分别返回唯一 `channels` 与 `target_holds`。core 用独立 schema、原键草稿和访问代次保护提交/GET 恢复；控制回执用于历史反馈，当前状态通过重取获得。

检查收口修复了三项具体问题：promote 的拒绝原先会回滚新 hold；旧 Reconfigure 留下 target-only 控制镜像；旧人类路由夹具分别用了不同暂停默认值。现已分别用提交明确拒绝、698 窄范围修复、真实绑定查询解决；严格读取与原业务断言保持。

| 验证 | 结果 |
| --- | --- |
| 最终 Go race 宽回归 | service/handler/router/migrations：594 顶层、687 子测试通过，0 失败 |
| Redis 补充回归 | 9 个先前 opt-in 跳过项在专用 Redis 下通过 race，0 跳过 |
| Linux 实际运行 | ARM64 测试程序与 ARM64 Linux 内核匹配，5 顶层用例通过，0 跳过；包含双通道真实并行、延迟 A 证明、B 继续下一批、原回执恢复 |
| core 全量 | 264 文件、3,509 测试通过 |
| 全仓静态检查 | typecheck 9/9、lint 6/6；既有非阻塞警告保留，相关 Go vet/gofmt 通过 |
| 数据与生成 | 698 升级、连续旧配置修复、负例及受保护回滚通过；0 invalid index；103 个 sqlc 文件再生成无差异 |
| 独立复核 | H01/H02/H03 全部关闭，本轮切片无未解决阻塞 |

Go 宽回归的 12 个条件跳过项均已补齐：9 个 Redis 用例单独运行，3 个 Linux 用例在实际 Linux 内核运行。最初 AMD64 模拟架构结果另存，不作为 ARM64 原生运行证据。临时 PostgreSQL、Redis、Linux 测试容器及专用网络已清理，原始日志与源码哈希留存。

完整文件清单与哈希见[源文件记录](verification-source-manifest-hold-2026-10-18.json)，命令、分组数量及日志位置见[验证汇总](verification-summary-hold-2026-10-18.json)，独立结论见[复核报告](research/verification-hold-review-2026-10-18.md)。本轮包含 66 个生产/测试/SQL/内置说明文件，另更新对应规范和任务资料；未新增依赖。

## 明确保留的后续工作

本轮不是完整目标调度或全部产品验收。混合 A/S 的 waiting 批次仍需按真实通道 FIFO 重组：当前 held S 仍可能挡住同一预合批中的有效 A，不能据本轮控制恢复宣布该场景已交付。下一步须实现通道统一序号、合格输入筛选和只允许未开始输入的安全批次/通道迁移，并核对 E03 的 never-launched 回队规则。

其后仍有完整冻结资源集合、真正 Switch、外部 agent/squad 配置写入的持久失效收敛、M08/M11 及容量 1 的小队证明。附件/Git、通用供应商适配、legacy enrollment 启动、非 Linux 封闭能力及完整 Web/Desktop/P1–P8 仍按原计划推进。创建及客户端执行能力继续关闭，全部 97 项完整 AC 尚未验收；任务保持 `in_progress`。

本轮实现与规范提交：`a389ef859`。测试使用的 66 个源文件与该提交逐一核对一致。
