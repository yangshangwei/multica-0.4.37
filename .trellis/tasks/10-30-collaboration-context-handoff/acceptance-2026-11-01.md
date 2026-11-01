# 父任务当前验收矩阵

2026-11-02 本地续作快照：集成分支 `codex/cross-workspace-collaboration` 为 `c6289db93`，领先本地 origin 跟踪记录 194 个提交，未 fetch。runtime、知识、RK1 框架、交付准备及原生 renderer origin 准备修复均已独立复核并本地合入。父任务保持 `in_progress`，最终候选尚未验收通过。逐项机器状态见 [验收 JSON](acceptance-2026-11-01.json)，人员与工作树见 [执行台账](execution-resume-2026-11-01.json)。

| 已合流范围 | 本地合入 | 证据与边界 |
| --- | --- | --- |
| runtime 9 项审计、5 项召回复核跟进 | `9e7f18785` | 产品 `7627916c8`，RV-1 `58d486656`；独立 pass-with-notes，RV-1 另经统筹复验；分支最终协作 service 1668 通过，0 失败/跳过 |
| 知识 claim → launch → write → web，吸收 runtime | `6cb9fcb36` | [整批交接](../11-01-collaboration-knowledge-audit-fixes/research/combined-ready-for-main-integration.json)及独立复核；`0702d5039` 修复 skill 入口和迁移 fixture，`742abb217` 修正优化器选路断言，均有独立复验 |
| RK1 框架及审查修正 | `30fa31eb7` | [独立复核](../10-30-collaboration-context-handoff-runtime/research/real-model-rk1/independent-review/recheck-review.json)对 RKI-1–5 结论 pass-with-notes；RKI-6、最终绑定和真实调用仍开放 |
| Windows 安装旅程、CI、脚本及交付文档准备 | `c3db64b24` | [本地准备](research/delivery-preparation/ready-for-integration.json)；WD-1/WD-2 修复并复核；未运行原生 Windows/安装升级/真实账号旅程 |
| 完整检查的原生 renderer origin 配置 | `7435b04ec` | [独立复核](research/final-check-native-origin/independent-review/review.json) pass-with-notes；下一轮须在启动前显式配置 renderer URL，实际 WebSocket/完整检查仍待运行 |

C3 候选 `c3db64b24` 的首轮完整 `make check` 在确认原生 origin 准备缺口后由统筹中断，退出码 `-2`。此前 Core 4340、Views 7118、Docs 62、Web 290、Desktop 1272，共 **13,082 项 TS 测试通过**；已分配的 6 个 Go 包数据库和中断现场均保留。这是阶段证据，不能标成完整检查通过。修复已合入，仍须冻结新候选并完整重验。命令、源版本、退出码及日志哈希见 [中断检查登记](research/acceptance-ledger-refresh-20261102/interrupted-check-summary.json)。

| 验收项 | 当前结果 | 下一入口与边界 |
| --- | --- | --- |
| AC0 交付提交链与已列全栈门禁 | 历史范围通过；修复已合流；最终候选开放 | 新候选完整 `make check`、sqlc 无漂移、迁移/Go/前端及门控 Web/Electron/RK1 fake 必须实际通过；不能拼接各分支历史结果 |
| AC1 附件预算与小队异构 runtime | 历史确定性范围通过 | 四类实际成员 runtime 限额的可选选择/必需超限拒绝有证据；真实 Provider 仍未运行，最终候选交互待重验 |
| AC2 正式 E0 | 原基线通过；新 RK1 未运行 | runtime 上限 **25 次、消耗 0**，含 H2b、H6 每轮两次启动。RKI-1–5 修正已复核合入；RKI-6 同候选全场景 fake/race 仍开放。保留 `runnable=false` 和空最终 `source_id`；完成绑定与独占窗口后先运行新正式 E0 |
| AC3 原预登记 E1–E6 | 部分完成，停在不完整结果 | E1 1 有效/2 无效；E2a 1 有效/2 无效；E2b 3/3、E3 baseline 3/3 有效。E3 after trial 1 因 EPROTO 后超时无效，因果未定。下一 **E3 after trial 2，剩余 14 次**，之后 E4→E5→E6；保留冻结源码与原编号，新产品不得回填旧槽位 |
| AC4 中断与撤权后的记录保留 | REC 审计修复已复核合入；最终候选与真实模型待验 | 既有 stop/撤权/作者退出双提交顺序及 race 证据保留；新候选须验证实际记录保留，真实 Provider 摄取不能由 seeded UI/fixture 代替 |
| AC5 成员/队长/并行 memory 交接 | ENV 修复及 RV-1 已复核合入；最终候选待验 | `822c15785` 历史 ENV-1 失败保留；须在新候选重放基线/实时 handoff 的正向归因、换 Agent、并行 memory 冲突与中断接力 |
| AC6 有界评论召回与 revision | 作者显示、召回契约与预览游标等修复已合入 | `7627916c8`、5 项跟进 `c9b285552` 有独立复核；最终长评论/大结果/分页及模型可见性待验。H2 不覆盖超过 32 项的结果/原因分页；H2b 单列大结果来源读取，必须以实际 tool_result 证明暴露 |
| AC7 非指令数据与凭据边界 | 历史确定性范围通过；真实模型待验 | 40 个凭据拒绝、7 个正向对照等已有证据。RK1-H5 可用两个自控 Multica 用户和同一 Provider 账号；原 E5 未运行；该安排不关闭要求不同 Provider 账号的 AC43 |
| AC8 经人确认的知识沉淀 | 21 项审计已处置、整批及后续复核通过并合入；最终候选待验 | `45efa093c` 的完整 service **3319 pass/3 fail/18 skip、exit 1** 保留；窄范围修正与复验（迁移 536/536、skill 10/10、索引计划契约）不改判原失败。最终完整 service/check、知识开关开/关、人工确认与纠错/撤回后的注入仍待验；生产开关保持 `false` |
| AC9 可追溯证据与缺失范围 | 合流与历史结果可追溯；最终绑定开放 | 最终源码/框架/CLI/配置/构件指纹待固定；历史 E2E 归因不是候选通过，平台/账号/升级证据仍缺；默认库迁移事故恢复边界仍开放 |

旧 E2E 报告绑定 `822c15785`，438 项计划中 421 pass/15 fail/2 blocked。剔除已另行处置的 ENV-1 等边界后，存量 14 项失败已完成 [源码归因](research/final-e2e-triage/findings.json)：9 项旧收件人交互、3 项包含读时评估时间戳的比较、2 项跨入独立 handoff 区域的记录分页断言。C3 的三类代表重放首次受 CORS 服务配置阻断，尚未到达目标契约；保留失败现场，环境修正后再复现和修复，不推断新候选结论。

RK1 保留 `4b508179f` 的 [中间版本 race 失败](../10-30-collaboration-context-handoff-runtime/research/real-model-rk1/fake-proof/runtime-fixed-race.json)：E0a 缺 recording seal 且超时，虽然业务 outcome/proof 已完成，样本仍不完整。RKI-1–5 的定向通过不能关闭 RKI-6；待重型测试静止后在同一最终候选、新建专用数据库执行全场景 normal/race，不放宽断言、延长超时或静默补跑。

| 关联交付 | 当前边界 |
| --- | --- |
| production Web + native macOS Electron | 原 runtime 6 项、知识 8 项分别绑定历史版本；最终合流候选的两条认证通道、原生交互与开关开/关仍须重验 |
| macOS arm64 .app/ZIP/DMG | 旧源 `61ff581d` 本地 ad-hoc 构建保留；C3 renderer 本地构建不能代替最终安装包、账号或公证证据 |
| Debian 12 arm64 .deb / Linux runtime | 旧 dpkg/fixture 与 runtime 证据保留；最终同源安装/原地升级/真实 Provider 组合待验 |
| Linux amd64 / macOS amd64 | 交叉编译与本机可执行范围须分别登记，不能代替缺失的原生/安装/账号证据 |
| Windows amd64/arm64 | 安装版协作旅程框架已补齐并合入；Node/TS/YAML/交叉编译只是准备。原生 PowerShell、ACL/Job/fault/lifecycle、安装升级、账号与 arm64 未验；未触发外部 CI |
| 多账号/额度 AC43 | 需两个执行环境和不同 Provider 账号，记录多目标/小队/普通转交及实际 usage/授权来源；账号、环境和额度未分配 |
| 服务端离线升级/交付 | 指南已更新迁移前缀上限 **763**、认证方式保留与知识开关默认 `false`；最终同源升级构件、演练、`README.md`/`操作文档.md` 字节一致与哈希待验 |
| 外部发布 | 用户决定仅继续本地准备；未授权公开推送、外部 CI、部署或发布，未执行这些操作 |

预算分账不变：RK1 runtime 25 次已批、0 消耗；原冻结实验剩余 14 次；知识 K1–K3、普通 E0b、安装版真实账号旅程及 AC43 另需预算与账号/环境，不能借用前两账。真实窗口须暂停共享网关其他编码/审查和本机重型测试，正式 E0 起按登记顺序串行；首个环境、框架或 runtime 不完整结果即停止后续场景，保留原失败。

默认库事件继续披露：[事故审计](default-db-incident-audit-2026-11-01.json)记录 2026-11-01 23:14 左右，未限定数据库的迁移使本地 `multica` 前向迁移，新增 272 条账目（456–763）。迁移前快照缺失，之后仅保留只读 schema/账目证据；未回退、清理或恢复到原状态。这是未解决的恢复边界，不是验收证据。后续数据库创建/迁移由统筹限定专用库并核对身份。

发布门的执行清单仍见 [runbook](release-gates-runbook-2026-11-01.md)；其 2026-11-01 只读快照中的版本、未合流状态与待定授权以本次台账为准。旧验收矩阵和原始执行快照保留在 `c6289db93` 及此前 Git 历史；本次仅校正当前入口，不改变历史失败分类。变更来源与哈希见 [本次核对记录](research/acceptance-ledger-refresh-20261102/audit.json)。
