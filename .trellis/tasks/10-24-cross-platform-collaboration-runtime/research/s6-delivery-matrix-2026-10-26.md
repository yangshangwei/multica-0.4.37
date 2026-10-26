# S6 首批交付组合与当前门禁

记录时间：2026-10-26。此矩阵只列出有实际证据的组合；编译成功、Provider 名称或测试夹具通过都不能扩大准入范围。

| 组合 | OS / arch | Provider / 版本 | 模式 | 当前结论 | 证据 |
| --- | --- | --- | --- | --- | --- |
| A | macOS 26.2 arm64 | Claude Code 2.1.310 | `host_process_supervision_v1`, local | **真实 Provider smoke 通过**；真实安装版完整控制流程仍待夹具修复后重跑 | `server/internal/daemon/collaborationrun/host_agentintegration_test.go`; `claude-host-smoke.log` |
| B | macOS arm64 | test-owned Claude-protocol fixture `fixture-1` | `host_process_supervision_v1`, local | 原生生命周期、会话、停止、断网、重启 focused tests 通过 | `lifecycle-fixture.jsonl`; `verification.md` |
| C | Linux Debian 12 arm64 | bundled CLI / test-owned fixture | packaged `.deb`, host mode | **安装版完整 Main 控制通过**；真实 Provider 账号未使用 | `research/deb-install-acceptance-2026-10-26.json` |
| D | Linux arm64 directory package | bundled CLI / test-owned fixture | packaged directory, host mode | **通过**，作为 `.deb` 回归基线 | `research/deb-install-acceptance-2026-10-26.json` |
| E | Windows amd64/arm64 | 未定 Provider | host mode | **不可用 / 待原生 runner**；现有 cross-compile 不构成准入 | `verification.md`, native snapshot `92bb6d86` |
| F | macOS installer artifact | 真实安装版 Provider | packaged `.app` | `.app`/ZIP 已在本机构建；DMG 因 `dmg-builder` 网络下载失败；完整控制重跑在夹具 issue 创建 503 前置失败 | 本轮 local package output 与失败日志 |

## 首批可讨论的交付边界

首批候选是组合 A + C：macOS arm64 上的 Claude Code 2.1.310 本地 Provider，以及 Linux Debian 12 arm64 的 `.deb` 客户端控制同一 host-mode 协作语义。组合 C 仍使用 test-owned Provider runtime；组合 A 的真实账户 smoke 只证明本机 Provider 适配和一次受管理执行，不替代安装版 Web/API 全旅程。

当前不开放 E、F，也不把远端/gateway、其他 Provider 版本或其他架构隐式纳入 A。每个新增组合必须重新记录 Provider 版本、配置指纹、OS/arch、安装方式和 lifecycle evidence。

## 本轮新增证据

- Claude Code 2.1.310，macOS arm64，`TestHostProviderAccountSmoke`：43.129s，真实账户闭环只读取测试问题并返回 `HOST_SMOKE_OK`；无文件修改、评论、后台任务或凭据输出。
- focused lifecycle：`TestHostRunnerReusesOrdinaryProviderAndOriginalPermitAndProof`、`TestHostRunnerOfflineKeepsRunningAndOnlyReplaysReceipts`、`TestHostRunnerFreshRetryGetsNewTokenAndStopForbidsFallback`、`TestHostSessionLockKeepsUnknownOwnershipAfterSupervisorRestart` 及账本重启回放均通过。
- macOS `.app` 已使用 Electron 39.8.7 arm64、本地缓存 Electron ZIP、ad-hoc signature 构建；`app.asar`、Main、bundled CLI 和 ZIP 的 SHA-256 已记录在本轮验收报告中。
- packaged Main 的 acceptance home/userData 隔离代码和 11 个单元测试通过；普通安装路径不读取该 opt-in 环境变量。

## 尚未闭合的阻塞

本轮 macOS packaged full-control 运行在真实 `.app` 启动和隔离配置阶段之后，于 `seedHostRuntime` 的 issue 创建阶段收到 503。独立数据库的协作创建 gate 未在夹具启动前显式打开，另一轮还观察到 host daemon claim 500/注册时序竞态；这是验收环境准备/夹具门禁问题，不能归因于 packaged Main。API/Web 临时服务和数据库已停止/清理，原始日志保留。该失败没有证明 packaged Main 控制流程失败，也不能替代成功的完整控制证据；下一次应显式打开 `collaboration_execution_gate` 的创建开关，并修复/隔离 host fixture 的注册与 claim 时序，再重跑相同 `.app` 和临时 home。

Windows 原生 runner、其他安装格式、真实 Provider 账号的完整断网/重启闭环，以及发布授权仍未完成。生产准入和发布开关保持关闭。
