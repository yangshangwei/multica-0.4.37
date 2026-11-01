# 父任务当前验收矩阵

2026-11-02 代码合流点为 `4085e5b38`，本轮证据以 `64a9994ec` 本地提交；origin 跟踪记录未 fetch。runtime、知识、RK1 框架、交付准备及后续 memory/Transfer 测试修复均已独立复核并本地合入。父任务保持 **in_progress，整体验收未通过**。[机器验收状态](acceptance-2026-11-01.json)与[执行归属](execution-resume-2026-11-01.json)持续更新。

被测产品固定为 `639bf6ef85ad894553b244873c2ed8d9f9dd29b2`，产品指纹 `9f72fc214b8e81f6fc3c1969be5e3304b7f158f60a6f7305f1531d55574d80ee`。后续 memory/Transfer 修改只涉及 E2E 与任务证据，产品根目录未改变；最终完整检查仍必须使用合流后的测试框架重新冻结并执行。

最新完整 `make check` 已结束，**exit 2，失败保留**：静态 15、TS 13,082、Go 20,852 个命名项通过，Go 22 skip、9 包仅编译；build/vet/sqlc 无漂移通过。浏览器 439 项为 **296 pass / 47 fail / 89 skip / 7 未运行**。独立归因为 42 项 PostgreSQL 连接容量、2 项私有 Go 适配器、2 项旧 Transfer 控件及 1 项原生焦点；历史焦点原因未追溯确认。见[实际结果](research/final-candidate-20261102/attempt-3-result.json)、[Go 全部包](research/final-candidate-20261102/attempt-3-Go-complete.json)和[逐项归因](research/final-candidate-20261102/attempt3-browser-triage.json)。此前中断和两轮失败现场、专用库均保留。

统筹在浏览器阶段额外启动自有 API，未计算共享 PG 总连接，导致容量不足。现已停止自有诊断 API/Web/renderer；后续全套 API 限制 max10/min1，串行启动，不并行增加 API。不修改共享上限、不终止其他会话连接。

| 合流范围 | 本地合入 | 当前证据与边界 |
| --- | --- | --- |
| runtime 9 项审计、5 项召回复核跟进 | `9e7f18785` | 产品 `7627916c8`、RV-1 `58d486656` 独立 pass-with-notes；最终候选与真实模型仍分开验收 |
| 知识 claim → launch → write → web | `6cb9fcb36` | 21 项缺陷与后续复核完成，旧 service 3319pass/3fail/18skip 原失败保留；639 完整 Go 无失败 |
| RK1 框架 | `30fa31eb7` | RKI-1–5 独立 pass-with-notes；RKI-6 静止环境 fake normal/race 各62通过，0fail/skip，结果独立复核待429恢复，付费入口仍关闭 |
| 交付准备及原生 origin 配置 | `c3db64b24` / `7435b04ec` | Windows 旅程框架、脚本、文档与 CI 准备已合入；外部 CI、原生 Windows 未运行 |
| E2E、bundle fixture 与知识实际输入/开关补验 | `7f8ca8725` / `231a08063` / `e6e7fe07d` / `639bf6ef8` | 均有独立复核；639 下知识普通27normal+27race、Host17normal+17race、部署ON/OFF各1通过，生产默认false |
| memory fixture 清理 | `020d18e62` | 原 red 泄漏真实复现；LIFO 修复后 memory→Leave 同库同API各通过，无门重置，失败OFF库冻结 |
| Transfer UUID picker | `4085e5b38` | 代码 `d39d54a07`，独立 `e98ee9569` pass-with-notes；生产Web2项通过。原焦点源码与断言完全保留 |

私有适配器 v4 的 AV4-1 changes-required 原报告保留。v5 关闭未标记 `go test` 的转发路径，递归测试需显式专用克隆及分配记录；11 项契约、最新去私有环境且禁网的 Go1.26.6 转发、原 Web/Desktop preparation 两项通过。[本地证据](research/final-candidate-20261102/adapter-v5-local-verification.json)不替代独立复核；代理两次遭遇429，复核仍开放。

| 验收项 | 当前结果与下一入口 |
| --- | --- |
| AC0 交付链与全栈门禁 | 最新完整检查失败。待 v5 独立复核后冻结合流框架完整重验；原生焦点原断言已通过1项；不拼接各分支或定向结果为整套通过 |
| AC1 附件预算与异构 runtime | 历史及当前确定性有界证据通过；真实 Provider 未验 |
| AC2 正式 E0 | 原基线通过；新 RK1 真实调用未开始。final639 fake normal/race各62通过，结果独立复核待429恢复；最终付费源码ID为空、runnable=false |
| AC3 原 E1–E6 | E1及E2a各1有效/2无效，E2b和E3baseline各3/3有效；E3aftertrial1无效（EPROTO后超时，因果未定）。下一E3aftertrial2，再E4→E5→E6，剩余14次；保持原冻结源码与编号 |
| AC4 中断/撤权保留 | REC审计修复及当前Go有界证据通过；真实Provider中断与摄取仍未验 |
| AC5 成员/换Agent/并行memory | ENV修复及实际有界旅程通过；memory fixture泄漏已单独修复复验。真实接力及整套候选验收仍开放 |
| AC6 长评论/大结果/分页 | 作者显示、召回契约、游标及5项跟进已修复。H2不覆盖超过32项结果/原因分页；H2b另测大结果来源，真实模型可见性需实际tool_result |
| AC7 非指令数据/凭据 | 确定性凭据与canary边界有证据；真实E5/H5未验。H5允许两个自控Multica用户共用Provider账号，不能据此关闭AC43 |
| AC8 人工知识确认/纠错/撤回 | 整批审计修复、实际输入纠错链、当前开关ON/OFF及四个隔离库normal/race通过；K1–K3真实验收另需预算，生产默认false |
| AC9 可追溯证据 | 分别记录源码、框架、CLI、配置、构件哈希与命令/退出码。完整候选、缺失平台/账号及默认库事故恢复边界均未关闭 |

| 关联交付 | 当前范围与缺口 |
| --- | --- |
| production Web + 原生 macOS Electron | 639 定向fixture旅程已实际执行；整套最新检查失败。原生焦点已按原源码/原断言重验1pass，0fail/skip/retry；历史失败保留 |
| macOS arm64 `.app`/ZIP/DMG | [同源639构件与独立复核](research/local-packaging-639bf6ef8-20261102/summary.json)已完成，ad-hoc签名与离线collector5文件通过，Mac.app原Main旅程1pass/3fake启动；安装升级/真实账号/正式发布未验 |
| Linux arm64 Host | [实际230命名项与11必需检查](research/final-candidate-20261102/Linux-native-Host-matrix.json)通过，0fail/skip，缓存容器禁网且保留 |
| Linux arm64 namespace | [6项路由/CLI/daemon与1项batchpoller](research/final-candidate-20261102/Linux-native-namespace-results.json)通过，专用新库，0fail/skip；未改全局sysctl/PG配置 |
| Linux arm64 目录包完整Main | [实际1项](research/final-candidate-20261102/Linux-directory-installed-Main-result.json)通过，ASAR/exe/CLI启动哈希匹配；Linux客户端控制独立Mac fake Host，不代表Linux daemon/deb/安装升级/真实账号 |
| Debian/RPM/服务端离线升级 | 最终deb/RPM工具及三个离线镜像基底缺失，未下载/拉取。指南迁移上限763、保持认证模式与知识false；最终包、演练及双份指南一致仍待验 |
| Windows amd64/arm64 | 现有框架/CI/脚本准备已合流。当前639 amd64/arm64 CLI、四测试包编译及vet共12条命令通过（禁网、无TestMain/DB）；原生PowerShell/ACL/Job/fault/安装升级/账号均未验 |
| 多账号额度 AC43 | 两个执行环境、不同Provider账号，涵盖多目标/小队/普通转交/usage与授权来源；环境、账号、额度未分配 |
| 外部操作 | 用户只授权本地准备；没有公开推送、外部CI、部署、发布或真实Provider调用 |

预算分账：RK1 runtime上限25、实际0，含H2b及H6每轮两次；原冻结实验剩余14；知识K1–K3、普通E0b、安装真实账号与AC43另行登记。即使fake通过，也须先完成前置门与最终源码/框架/CLI/配置/构件绑定，具备授权后才可独占共享网关从新正式E0开始；首个不完整即停，保留原样本。

默认库事故仍未解决：[审计](default-db-incident-audit-2026-11-01.json)记录未限定数据库的迁移使本地 `multica` 前向迁移，新增272条账目（456–763）。缺迁移前快照，未回退或恢复；此后仅只读留证，不将其作为验收库。失败现场和其他会话未提交文件继续保留。

[发布门清单](release-gates-runbook-2026-11-01.md)保留历史步骤；以本页及机器台账为当前入口，旧矩阵保留于Git历史。

下一完整检查已在干净 detached 候选 `64a9994ec` 的独立 worktree 中完成[本地准备](research/final-candidate-20261102/next-complete-check-preparation.json)：API池10/1、Git树外临时目录、逐包新库、v5适配器与JSON报告均已固定，未带入临时Mac诊断spec。运行尚未启动；独立v5/RKI6结果复核受429阻断，启动器保留对应的复核放行记录要求。所有自有证明API/Web均已停止、数据库与日志保留。
