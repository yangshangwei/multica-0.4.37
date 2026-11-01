# 父任务当前验收矩阵

父任务保持 `in_progress`。runtime 产品 `0e1ee8e1e`、原集成证据 `2b861347e`、补充测试 `0bbc6a77a`/`84c4c2b3d`、最新原实验 `785b331e8` 分别绑定其验证范围。

| 验收项 | 当前结果 | 尚需完成/边界 |
| --- | --- | --- |
| AC0 交付提交链与已列全栈门禁 | 已通过（列明范围） | runtime 没有新增迁移，746 演练属于原修复版本；额外遗漏矩阵另列 |
| AC1 附件预算与小队异构 runtime | 确定性通过 | 四类实际成员 runtime 限额的可选选择/必需超限拒绝均通过；无真实 Provider 调用 |
| AC2 正式 E0 | 原基线通过／新 runtime 已登记未运行 | 原产品 4c833e737 正式 E0 已过；新 runtime/知识登记为 RK1（`d2fe54ab0`）；runtime 场景上限 19 次（`28c67c674`）。召回产品补丁已合入（`ac2a24f8f`/`c54a28ba6`），源码重新绑定为 `source-manifest-547760ea8.json`。还要等三件事：审计 9 项修复合入并重新绑定、知识修复合入、真实 Provider 框架（-35）通过复核。知识场景无预算；不能继承旧结论 |
| AC3 原预登记 E1–E6 | 部分完成／停在不完整结果 | E1 1 有效/2 无效；E2a 1 有效/2 无效；E2b 3/3、E3 baseline 3/3 有效；E3 after trial1 无效（模型请求 EPROTO 后超时，至少 4 个会话请求重叠，因果未定）；独立复核同意全部分类；E3 baseline 3/3 与预测相反，只能发现退化；下一 E3 after trial2，剩余14次；E4–E6 未运行 |
| AC4 中断与撤权后的记录保留 | 原确定性门通过／审计重开／真实模型待验收 | Stop/撤权/作者退出两种提交顺序已补齐并通过 race；真实 Provider 中断摄取未由 seeded UI 或服务 fixture 代替。合入后 runtime 审计确认 REC-1–REC-4 未修复：一次非确定性 checkpoint 失败后本轮停止上传、恢复约 1 秒、outcome 后剩余块被 409 拒收；fallback 后旧 attempt 的块使整条流关闭；seal 原因一律 `authority_lost`；瞬时失败补齐后仍永久 partial。会话 -a8 已在 `codex/runtime-audit-fixes-20261101` 提交这四项的修复 `4456cc65a`（截至 18:35 的 tip 为 `5b9f66292`，另含 ENV-2），没有复核，没有合入 |
| AC5 成员/队长/并行 memory 交接 | 原确定性门通过／审计重开／真实模型待验收 | 双成员与 claim 前后来源、memory 冲突后的 refresh/new-intent merge 及双事实/作者版本保留均通过；模型理解和真实隔离接力仍单列。审计确认 ENV-1/ENV-2/ENV-4 未修复：实时 handoff 把最新 memory 标成“Started from”；接近 64 KiB 且含凭据样文本的委派脱敏后被截断，成员 run 永久无法启动；continuation 的唤醒原因一律 unknown |
| AC6 有界评论召回与 revision | 原确定性门通过／审计重开 | 模型读取/理解仍属独立实验；普通评论是 live recall，不是启动预载。审计确认 RECALL-1/RECALL-2 未修复：runtime 评论召回没有作者显示名（父 PRD R4）；CLI spec 仍写读取只返回最新 200 条，召回契约未成文。召回修复 `c54a28ba6` 的独立复核（会话 [4a018c]）结论为 pass-with-notes，见 [复核](../10-30-collaboration-context-handoff-runtime/research/verification/2026-11-01-recall-surface-review/README.md)。另有三处开放：错误来源 UUID 返回 503 而不是 404；Claude CLI 默认只把 30000 字符输出交给模型，32 KiB 的来源块或 64 KiB 的评论页会让游标落在预览之外（未实测）；live `handoff show` 让 ENV-1 变得 Agent 可达 |
| AC7 非指令数据与凭据边界 | 确定性通过／真实模型待验收 | 八个新私有 handler 的40个凭据拒绝、7个正向对照及撤权后终止元数据通过；真实双账号 E5 未运行。RK1-H5 以 harness 数据库中测试创建的 owner/contributor 两个用户作为两个自控账号、使用一个 Provider 账号（会话 -87 记录的用户确认，`7e14213df`），未运行 |
| AC8 经人确认的知识沉淀 | 原确定性门通过／审计缺陷修复中／真实模型另验 | 产品 `96cedf485`、证据 `93093d2e9` 已以 `8cb04e329` 合入，生产开关仍关闭。2026-11-01 会话 -14 审计记录 21 项缺陷（高 3）。13:55 起由会话 -54 用 workflow 在四个分道分支修复，截至 18:22 已提交 7 项（H1、H2、L1、L2、M4、M5、L8），分组复核 pass_with_notes。这些修复还没整合到 `codex/collaboration-knowledge-fixes-20261101`，没有经过协调会话复核，也没有合入。本矩阵未独立核实这些缺陷 |
| AC9 可追溯证据与缺失范围 | 当前交付证据完成／外部门开放 | runtime 与知识源版本、完整测试、失败修正和独立审查已绑定；真实模型、Windows/安装组合/额度仍待验收。合入后 runtime 只读审计（会话 -ef 执行，会话 -a8 复核入账）确认 9 项（中 5、低 4）、驳回 2 项，未运行测试，见 [审计记录](../10-30-collaboration-context-handoff-runtime/research/post-merge-audit-2026-11-01/README.md)；用户决定在 RK1 前全部修复，由会话 -a8 执行，RK1 须在修复合入并重新绑定源码后启动。2026-11-01 用户决定把含私有指令的 Provider 会话记录移出本地历史：5 个提交改写为 `03469921c`…`b8fd547dc`，产品树不变，只保留 sha256 和来源，独立核查 pass-with-notes |

原始证据路径见 [机器可读矩阵](acceptance-2026-11-01.json)。runtime 已列 Go/TS/静态/浏览器门禁及审计补充矩阵全部通过；保留原始失败和复验，不把容器测试重复累计。

| 关联交付 | 当前边界 |
| --- | --- |
| production Web + native macOS Electron | runtime原6项、知识8项各自绑定版本；不等于安装版或真实模型验收 |
| macOS arm64 .app/ZIP/DMG | 旧源 61ff581d 本地 ad-hoc 构建；非0e1ee8e1e，无真实安装账号/公证发布证明 |
| Debian12 arm64 .deb | dpkg离线安装和控制macOS fixture通过；Linux真实Provider、原地升级、自动更新未证实 |
| Linux arm64 runtime | 真实Provider账号和完整安装组合未证实 |
| Linux amd64/macOS amd64 | 前者交叉编译，后者原生待执行；不能代替原生/安装/账号证据 |
| Windows amd64/arm64 | 只有交叉编译和本地验证快照；原生ACL/Job/fault/lifecycle、安装、Provider账号、CI仍缺 |
| 多账号/额度 AC43 | 需至少两个执行环境及不同Provider账号，涵盖多目标/小队/普通转交/实际usage与授权来源 |
| 发布/升级 | 未推送/部署/发布；creation迁移744已默认开，AI execution/host准入仍默认关；混合writer滚动升级另需核对。之前因为一份私有会话记录而禁止推送，这份记录已从所有分支移除；推送本身仍需用户授权（仓库公开） |

知识已完成确定性验收并整合；原系列 2026-11-01 续跑 8 次后停在 E3 after trial 1 超时，下一入口 E3 after trial 2/14 次。新 runtime 模型记录另行登记。Windows、安装组合、多账号额度和发布验收分别保留。

上表各发布门的可执行步骤、证据要求和待用户决定事项见 [发布门清单](release-gates-runbook-2026-11-01.md)。该清单只做了只读核对，没有推送、触发 CI、构建安装包或调用 Provider。

知识最终证据见 [集成报告](../10-30-collaboration-knowledge-promotion/verification-integration-2026-11-01.md)，包含测试隔离事件的事实与追溯限制。
