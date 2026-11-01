# 知识候选逐层沉淀：当前进度

状态：**工程已合入，确定性验收通过，真实模型与发布验收仍开放**。任务保持 `in_progress`，知识注入开关默认 `false`，未部署或发布。

本次核对基线为 `ba6f3cd50`。产品提交 `96cedf485`、证据提交 `93093d2e9`，已通过 `8cb04e329` 合入 `codex/cross-workspace-collaboration`。完整范围见 [集成验收报告](verification-integration-2026-11-01.md)。

## 已完成

| 范围 | 当前结果 |
| --- | --- |
| 人工知识候选与治理 | 提案、精确版本确认、来源权限、纠错/派生、过期、撤回和后续注入过滤已实现；技能层继续后置 |
| runtime 与共享页面接线 | 普通执行及协作运行的协议、路由、客户端、启动授权、会话历史保护和 Web/Desktop 页面已集成 |
| 数据与失效 | 迁移 747–760、独立目标修订、持久失效通知和客户端可见期限完成；sqlc 零漂移 |
| Go 验收 | 82 包：73 个含测试包通过、9 个无测试包；10,284 项顶层测试通过，21 个既有跳过逐项记录 |
| 前端与平台 | 前端 13,060 项测试及 lint/typecheck 通过；知识流程 7 项 Web、1 项原生 macOS Electron 用例通过 |
| 实际输入及独立审查 | 普通/Host 的 fake-executable 输入组合、审批并发、原键恢复、来源删除和清理回归通过；知识确定性终审通过 |

上述数字属于各自已记录的版本和覆盖范围，不作为每次更新都重新执行的测试结果。

## 上次交付后的新增进展

- `65cd0599b` 补强了启动注入与 checkpoint 联合故障场景的原 permit、attempt/nonce 和终止事实断言。联合 race 40 项、tagged fake 启动 80 项、补强后 race 6 项及 vet 通过；各套件有重叠，不累计为独立用例。见 [联合补验记录](../10-30-collaboration-context-handoff-runtime/research/verification/2026-11-01-joint-current/README.md)。这批新增断言已于 2026-11-01 补做独立复核，结论 **pass-with-notes**：期望值与生产路径一致，7 个测试期望变异和 1 个生产变异均使新断言失败（生产变异下旧断言仍通过），`-race -count=10` 30 次子测试通过且无数据竞争。注意事项不是缺陷：spec 中“fallback refusal”的措辞比测试只覆盖的 snapshot drift 更宽。见 [独立复核](../10-30-collaboration-context-handoff-runtime/research/verification/2026-11-01-joint-review/README.md)。原产品独立审查结论保留原范围。
- `629f0a079` 将知识目标选项的同步查询改为等待选项出现，修正测试时序。同一 17 项套件串行 3 轮、并行 4 轮均通过；原偶发失败未能稳定复现，因此记录为复跑未再现。
- `ba6f3cd50` 已将联合补验与开放门同步到 runtime 任务和会话日志。以上新增改动未改变产品执行逻辑，也未启动真实 Provider 实验。

## 未完成与下一步

1. ~~补查新增联合故障断言。~~ 已完成，pass-with-notes，见上节。
2. **新版本真实验收已登记为 RK1，尚不可运行。** [RK1 登记](../10-30-collaboration-context-handoff-runtime/research/real-model-rk1/README.md)（`d2fe54ab0`）固定了源码指纹、CLI 2.1.316、请求模型、网关、场景顺序和停止规则，调用上限为 0。开始前还需要：用户分配调用预算（参考值：最小集 14 次，不含 K3 的完整集 29 次）；构建授予 handoff/recording/knowledge 的真实 Provider 框架，并完成假 Provider 证明和独立复核；H5 需要第二个自控账号。现有 `TestHostProviderAccountSmoke` 的 claim 不授予 handoff 或知识，只能证明 CLI/生命周期。
3. **在有上游恢复依据后独立续跑。** 原系列下一入口仍为 **E2a trial 3，剩余上限 22 次**；最近 trial 2 是上游 502 无效样本。保持原场景、模型、编号和停止规则，新系列与原系列分开记录。
   2026-11-01 11:03 用户确认上游已恢复，要求续跑。trial 3 已登记在 [`experiments-2026-11-01-e2a-resume3/prerun.json`](../10-30-collaboration-fix-closure/research/experiments-2026-11-01-e2a-resume3/prerun.json)，预检通过：两份冻结 HEAD、两侧三个框架文件哈希、CLI 2.1.316 均与登记一致。由于同一账号上另有 4 个活跃会话，其中两个拿到同一接续任务、正在准备真实运行，独占串行窗口不成立，**尚未启动，未消耗槽位**。需要先指定唯一执行会话并让其余会话在运行期间停止调用。
4. **完成父任务发布验收。** Windows 原生、安装/升级组合、多账号及实际额度溯源仍开放，详见 [父任务验收矩阵](../10-30-collaboration-context-handoff/acceptance-2026-11-01.md)。当前 Web/macOS 和 fake 输入证据不替代这些门。

数据库测试隔离事件及追溯限制继续保留在集成报告中。整包数据库测试使用新建专用库；保留历史和失败现场用于复核。

本次仅更新进度记录和 RK1 登记，独立复核只在临时 worktree 运行单个 fake-executable 测试；未修改产品代码、重跑产品套件或消耗任一系列的实验槽位，任务不归档。
