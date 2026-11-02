# 知识候选逐层沉淀：当前进度

当前新轮：共享 `make-check-v9` 已于 **2026-11-02 15:36:09 UTC** 启动，仍在运行。v6 的原失败和新专用库定向红绿分别留证；新 source/适配器/launcher 独立复核通过，见 [v9 启动绑定](../11-01-collaboration-knowledge-audit-fixes/research/continuation-20261102/shared-check-v9-start.json)。定向 7 项通过不等于整套通过；下面 v6 失败为上一轮实际结果。

上一轮实际结果：共享 `make-check-v6` 已于 **2026-11-02 15:02:20 UTC** 以 exit 2 结束；static15/TS13,082/脚本回归通过，Go `cmd/server` 两个顶级 skill catalog 场景 503，含子项 3 fail。剩余 Go/API/Web/PW 未执行，见 [实际结果](../11-01-collaboration-knowledge-audit-fixes/research/continuation-20261102/shared-check-v6-result.json)。当前等待唯一负责人完成 v9 的 outer-Go PUB_DIR 隔离、实际红绿与独立复核，再启动新整套；下列启动/running 描述保留为历史，不能计为当前通过。

历史启动记录（14:47:28 UTC）：主链路和 21 条审计处置已本地合入，RKI6 原 fake 证据独立复核通过；唯一负责人执行的第 4 轮完整检查当时启动，见 [同源运行绑定](../11-01-collaboration-knowledge-audit-fixes/research/continuation-20261102/shared-check-binding.json)。选定整合适配器的全局 `--` 问题已在新字节及两份独立复核中关闭，原冻结 v8 报告保留原范围。

真实知识框架的 [下一阶段设计](../11-01-collaboration-knowledge-audit-fixes/research/knowledge-real-model-preregistration-20261102/design.md) 已落盘：四次 K1、两次 K2、八个生产者/角色 K3 样本，共 14 次知识调用提案，普通 E0b 的一次另计。尚未实施或分配，不能从原剩余 14 或 runtime 上限 25 转移额度。确定性检查与真实模型/启用/发布分别核销，任务仍保持 in_progress。

## 2026-11-02 本轮推进

已按用户续作请求进入 [审计修复最终验收计划](../11-01-collaboration-knowledge-audit-fixes/advance-plan-2026-11-02.md)，第 4 候选 `64a9994ec` 的源码与构件绑定已核对。RKI6 原 fake 结果已独立 pass-with-notes、0 阻塞；v5 私有适配器两项隔离/路径问题及 v7 中发现的 compile-only 取值边界正在以红绿证据修复。完整第 4 轮由首轮闭环会话单执行，知识侧提供独立增量并复用同源结果；不重复启动完整检查。第三轮失败保留，不再将旧“检查运行中”作为当前状态。知识实际输入组合、纠错/撤回及部署开关两态的已记录证据保留原覆盖范围；知识 K1–K3 仍无预算，真实模型和发布门继续开放。

以下数字与里程碑是对应历史版本的交付记录，不代表本轮重新运行或最终检查已通过。

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
2. **新版本真实验收已登记为 RK1，尚不可运行。** [RK1 登记](../10-30-collaboration-context-handoff-runtime/research/real-model-rk1/README.md)（`d2fe54ab0`）固定了源码指纹、CLI 2.1.316、请求模型、网关、场景顺序和停止规则，调用上限为 0。开始前还需要：用户分配调用预算（参考值：最小集 14 次，不含 K3 的完整集 29 次）；构建授予 handoff/recording/knowledge 的真实 Provider 框架，并完成假 Provider 证明和独立复核；H5 需要第二个自控账号。现有 `TestHostProviderAccountSmoke` 的 claim 不授予 handoff 或知识。自 `96cedf485` 起 `Runner.Run` 给本机 Claude 的 claim 加了 `knowledge_version=1`，测试授权器一度不认这个参数，smoke 和 11 个 `TestContextExperiment*` 假测试在 `/claim` 收到 403；只改测试的修复已在 `ac2a24f8f` 合入，复核见 `1efa8d80f`。原系列使用冻结源码，不受影响。
3. **原系列已续跑一轮，再次停在不完整结果。** 2026-11-01 用户确认上游恢复，并指定会话 multica-0-4-37-54 执行。12:02 起串行运行 8 次：E2a trial 3、E2b 1–3、E3 baseline 1–3 共 7 个有效样本，全部符合预登记判据；E3 after trial 1 已写出 memory 之后，CLI 遇到 EPROTO TLS 握手失败（重试 1/10），约 55 秒无响应，到 5 分钟上限被终止，不是 429；按停止规则后续全部停止。**下一入口 E3 after trial 2，剩余上限 14 次**，其后 E4 → E5 → E6。该 trial 期间至少 4 个其他会话的请求在途，启动前的检查已显示有活动但未据此暂停；这支持环境类归因，不能确定原因。会话 -0b 已独立复核（`2fc5c962c`），同意全部分类和 7 个通过。见 [续作记录](../10-30-collaboration-fix-closure/verification-e2a-resume3-2026-11-01.md)。
4. **完成父任务发布验收。** Windows 原生、安装/升级组合、多账号及实际额度溯源仍开放，详见 [父任务验收矩阵](../10-30-collaboration-context-handoff/acceptance-2026-11-01.md)。当前 Web/macOS 和 fake 输入证据不替代这些门。

数据库测试隔离事件及追溯限制继续保留在集成报告中。整包数据库测试使用新建专用库；保留历史和失败现场用于复核。

本次仅更新进度记录和 RK1 登记，独立复核只在临时 worktree 运行单个 fake-executable 测试；未修改产品代码、重跑产品套件或消耗任一系列的实验槽位，任务不归档。
