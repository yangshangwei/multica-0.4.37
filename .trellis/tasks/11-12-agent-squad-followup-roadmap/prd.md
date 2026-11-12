# 智能体与 AI 小队后续能力建设

状态：in_progress。M1-S1 已按 task.json 中 2026-11-12 的明确批准启动；本 PRD 继续约束首包范围和后续路线，不代表其余路线获批或完成。

## 来源与目标

用户于 2026-11-12 在审阅六项任务的归档评估后确认“归档已交付阶段”。本任务承接原研发体系父任务尚未实现的功能，原任务的 M0、已验收 M1 增量和完成事务门禁 v1 均保持已交付，不重做。

原始需求、设计、子任务及失败证据保存在 [阶段归档](../archive/2026-11/11-05-agent-squad-rnd-improvements/stage-closure-2026-11-12.md)。真实供应商行为与可靠性验证另由 [真实运行验收](../11-12-collaboration-real-runtime-acceptance/prd.md) 承接。

## 剩余需求

| ID | 范围 | 可观察的完成条件 |
| --- | --- | --- |
| F1 | M1 剩余交付交互与授权流程 | 按当前产品合同逐项收口普通事项完成授权、提示消费与其他入口的交互范围；提示确认不冒充事务授权；对已实现后端门禁不重复开发。拖拽、批量、创建即完成、混合更新、协作/R38是否补提示分别设计，不把本期未覆盖入口自动视作缺陷。 |
| F2 | M2 确定性报告快照 | 冻结统计结果、来源、水位和缺口，包含范围、时区、时间窗、覆盖率；重读原快照可重现，不能只存查询参数。新增接口同步 Reporter 最小权限。 |
| F3 | M2 父级预算 | 先建立单一父任务预算根，归集子执行与重试，区分预留、实际和未知；父子不重复扣减，故障可恢复。先观察后强制准入，再扩展项目和小队。 |
| F4 | M2 能力与负载调度 | 复用 readiness 和领取机制；实际领取复核权限、容量和预算，取消、失败、资源变化可解释；不因空闲静默切换运行时身份。 |
| F5 | M3 持续治理 | SLO/恢复演练、版本化模型评测、制品供应链、模板升级、知识持续复核与失效治理五包独立验收；复用已有知识版本、审批、撤回、到期能力。 |

## 边界

- 原完成门禁 v1 已完成；当前不可豁免检查与 `skip_supported=false` 是已验收限制，不是待补缺陷。
- 11 月 12 日 UI/CLI 单事项 status-only 提示增量及旧阶段同期回归修复由旧阶段结项清单记录；本任务只承接该包以外的剩余范围。
- 不自动开放知识注入、真实 Provider、发布、Hosted CI 或部署；不恢复用户已排除的用量/账单对账和已收窄的 Windows 范围。
- 本任务不把历史失败计为通过；真实验收预算与执行条件在另一个待办内按原系列保留。

## 推进顺序与执行状态

F1 的首包为 M1-S1：普通单事项“查看当前交付证据 → 明确授权 → 提交完成 → 查询实际结果”。用户价值是让当前有权的人看清自己授权的任务版本、目标状态和制品，并在超时或拒绝后辨认实际结果。

- 首包需求与逐项验收：[requirements.md](requirements.md)。覆盖 Web/Desktop 共享的普通单事项显式 status-only 入口，以及 CLI 合同；其他入口的现状与后续顺序单列。
- 当前事实及缺口：[后端研究](research/m1-backend-current-contract.md)、[客户端研究](research/m1-clients-current-contract.md)。历史测试不代表本轮测试。
- 技术设计：[design.md](design.md)；执行顺序、测试归属和门禁：[implement.md](implement.md)。二者仅详设 M1-S1。
- 后续顺序保持 M1-S1 → F2 报告快照 → F3 单一父任务预算 → F4 能力/负载调度。F1 其他入口在各自设计收口后安排，不作为启动 F2 的隐含全量前置。
- M3 五包的独立结果与依赖见 requirements；本轮不建立新的父子任务、不改变真实运行验收授权。

当前实现源自已批准的 M1-S1 规划。当前共享工作区有其他并行改动，不能用其源码/测试状态替代 M1-S1 自有证据；各执行 lane 的文件边界和新建测试环境记录在 implement.md 与 research/。后续 M1 入口、F2–F5 和独立真实运行验收仍分别保留原审批、预算与准入边界。

### M1-S1 当前状态

后端只读候选/预览、Core 原操作协调器、共享 Web/Desktop modal/gate 及 CLI 首包均已实现。后端 handler/router normal/race、Core 全量、M1/surface 定向测试与 CLI 全量定向验证见 `research/m1-backend-resume.md`、`research/m1-clients-resume.md`、`research/m1-cli-resume.md` 与 `research/m1-final-check.md`。最终复核修复了 CLI receipt/applicability/expiry 的 fail-closed 缺口。

尚未完成的验收：当前本地 Desktop Test 项目不在 enforce，未验证真实 enforcing modal 的 renderer/键盘/窄屏视觉路径；Views 全量套件未在定向修复后重跑。POLA-1 在只读 UI 检查误操作期间曾 `todo → done → todo`，当前状态已恢复，但历史保留两条状态事件。该事项不再用于额外 UI 检查。本任务保持 `in_progress`，直到能在隔离并明确允许的 enforcing 测试环境中完成用户流程验收。
