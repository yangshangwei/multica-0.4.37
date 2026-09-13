# 合并与验证报告

验收完成。验证的生产代码提交为 `efb3c251c`，基线为 `8e123db48`。分支 `fix/upstream-041-043-selective` 保留在 `/Volumes/artisan/code/2026/multica-upstream-fixes-041-043`；没有合并、提交或推送到 main。

## 改动与业务影响

九个指定上游提交均以独立提交导入，并核对 stable patch-id 与源补丁一致。来源映射和逐项业务影响见 [business-impact.md](./business-impact.md)。最终代码范围为 **13 个生产文件、15 个测试文件**，另有任务、规则和验证记录。

额外发现并单独修复了一个基线已有的数据快照问题（`efb3c251c`）：复制 Git 索引时更新 mtime，可能漏掉同尺寸快速修改。修复保留私有索引时间戳，并在复制失败时从 HEAD 安全重建；不改用户索引、refs、工作内容或原冲突保护。确定性测试先失败后通过，原冲突保护再通过 20 次重复以及整个 execenv race 套件。

另一个额外提交 `d143ffdd2` 只修测试夹具：TestGitEnv 不再假定启动进程没有 Git 配置，并继续验证已有键和值不丢失。生产 Git 配置逻辑未修改。

没有新增依赖或数据库迁移。中文模板、审批／自治等级、项目默认小队、内网集成、自部署认证和更新日志保留。需要明确的预期变化是：Inbox 列表 body 使用摘要但详情和存储保留全文；Codex home 准备失败时改走新环境并放弃旧会话连续性；数据库暂时故障使用 500 而非误导性 404。

## 最终验证结果

| 检查 | 最终结果 | 证据 |
|---|---|---|
| 完整前端 TS 测试 | 687 文件、8,127 测试通过；强制绕过测试缓存 | [ts-full.log](/Volumes/artisan/code/2026/multica-upstream-fixes-041-043/.omx/reports/upstream-041-043/ts-full.log) |
| 移动端单元测试 | 20 文件、115 测试通过 | [mobile-unit.log](/Volumes/artisan/code/2026/multica-upstream-fixes-041-043/.omx/reports/upstream-041-043/mobile-unit.log) |
| 类型检查 | 根工作区 9 项通过；移动端另行通过 | [typecheck.log](/Volumes/artisan/code/2026/multica-upstream-fixes-041-043/.omx/reports/upstream-041-043/typecheck.log) / [mobile-typecheck.log](/Volumes/artisan/code/2026/multica-upstream-fixes-041-043/.omx/reports/upstream-041-043/mobile-typecheck.log) |
| Lint | 6 项通过，0 错误；保留既有 warnings | [lint.log](/Volumes/artisan/code/2026/multica-upstream-fixes-041-043/.omx/reports/upstream-041-043/lint.log) |
| 完整 Go race + coverage | 73 个有测试包通过；13,898 个结果通过（含子测试）；0 失败；9 个明确条件跳过 | [go-complete.log](/Volumes/artisan/code/2026/multica-upstream-fixes-041-043/.omx/reports/upstream-041-043/go-complete.log) |
| Go vet / build | 全包通过 | [go-vet-complete.log](/Volumes/artisan/code/2026/multica-upstream-fixes-041-043/.omx/reports/upstream-041-043/go-vet-complete.log) / [go-build-complete.log](/Volumes/artisan/code/2026/multica-upstream-fixes-041-043/.omx/reports/upstream-041-043/go-build-complete.log) |
| Web 生产构建 | 通过 | [web-build.log](/Volumes/artisan/code/2026/multica-upstream-fixes-041-043/.omx/reports/upstream-041-043/web-build.log) |
| Electron 构建 | main、preload、renderer 通过 | [desktop-build.log](/Volumes/artisan/code/2026/multica-upstream-fixes-041-043/.omx/reports/upstream-041-043/desktop-build.log) |
| UI 导出边界 | 通过 | [ui-exports.log](/Volumes/artisan/code/2026/multica-upstream-fixes-041-043/.omx/reports/upstream-041-043/ui-exports.log) |
| iOS 启动脚本回归 | 通过 | [mobile-ios-harness.log](/Volumes/artisan/code/2026/multica-upstream-fixes-041-043/.omx/reports/upstream-041-043/mobile-ios-harness.log) |
| **完整端到端** | **82/82 通过，0 失败、0 跳过、0 flaky** | [e2e-accepted.log](/Volumes/artisan/code/2026/multica-upstream-fixes-041-043/.omx/reports/upstream-041-043/e2e-accepted.log) |

完整 E2E 使用生产 Web、真实隔离 API／数据库，并实际启用 Electron 更新日志和动态 feed 发布两个条件测试。最终 API `/health` 已核对为 `efb3c251c`。结果明细见 [Playwright JSON](/Volumes/artisan/code/2026/multica-upstream-fixes-041-043/.omx/reports/upstream-041-043/e2e-results-accepted.json)，截图／trace 位于 `/Volumes/artisan/code/2026/multica-upstream-fixes-041-043/.omx/reports/upstream-041-043/e2e-accepted-artifacts/`。

Go 全仓语句覆盖率为 **69.8%**。UTF-8 预览、Codex compaction 识别／注释以及 Classify 均为 100%；seedSnapshotIndex 为 87%。较大函数仍有未执行分支，例如 Reuse 58%、captureUserSnapshot 60%、ResolveTaskWorkspaceIDChecked 55.6%；不能把全套通过当作全仓 100% 覆盖。[完整函数覆盖率](/Volumes/artisan/code/2026/multica-upstream-fixes-041-043/.omx/reports/upstream-041-043/go-coverage-functions-complete.txt)。前端未引入新的覆盖率依赖，以完整套件和业务路径矩阵证明覆盖。

## 失败如何处理

1. 基线相关前端 98 项通过；只导入新测试时复现 7 个预期失败，应用补丁后通过，证明测试可检出旧行为。
2. 新 E2E 第一轮的三处问题来自测试假设：URL 尚未提交就刷新、通知行选择器匹配详情标题、checkbox NodeView 的 DOM 属性假设不符。浏览器 trace 和保存的 Markdown 证明产品行为正确；修正后 5/5 专项、82/82 全套通过。
3. 首次全套浏览器运行碰到开发服务器与生产构建并行时的导航超时，未用其作为验收证据；改用完成构建的生产服务器，从头跑完整套件并通过。
4. 完整 Go 测试发现 TestGitEnv 的宿主环境假设，已修夹具。随后发现真正的基线快照遗漏，以确定性回归证明机制并单独修复。
5. 一次 Go 运行因测试 Redis 收到 SIGTERM 退出，产生额外依赖跳过；该运行没有用作最终验收。恢复任务专用容器、固定 IPv4 并重新跑整个 Go 套件。**最终日志无 Redis 不可达，也无数据库 TestMain 静默跳过。**

## Go 的 9 个保留跳过项

这些为既有环境／平台条件或明确保留的测试限制，并非这次为了通过而新增跳过。

| 测试 | 原因 |
|---|---|
| `TestCommentContentBigramRequirementMatchesRealPGBigm` | migrate_comment_search_index_strategy_test.go:156: Postgres does not provide pg_bigm; install it to run the real-opclass integration test |
| `TestCLIConfig_UnknownFieldsArePreserved` | config_test.go:265: documenting known limitation: encoding/json drops unknown fields on round-trip; future PR can switch to a preserving encoder |
| `TestIsDriveRoot` | local_directory_test.go:383: windows-only behaviour |
| `TestWebhookHandler_DBErrorOnTokenLookupReturns500` | autopilot_webhook_handler_test.go:971: 500-branch requires injecting a stub Queries; left as a code-review-protected invariant |
| `TestFetchFromSkillsSh_AnthropicPptxIntegration` | skill_test.go:989: set MULTICA_RUN_SKILLS_SH_INTEGRATION=1 to run live GitHub integration test |
| `TestACPManagedTerminalPreservesWindowsUnicodeShellOutput` | acp_terminal_test.go:89: Windows cmd.exe code-page behavior only applies on Windows |
| `TestCodexExecuteRetriesAfterSignaledProcessIsReaped` | codex_test.go:3195: Linux signal/reap semantics regression |
| `TestPiExecuteAttachesStdinPipe` | pi_test.go:226: stdin fd inspection relies on /proc/self/fd/0 |
| `TestClaudeRootSudoPreflight/root_bypass_without_sandbox_errors` | claude_test.go:755: root-only preflight assertion |

## 隔离与限制

本机为 macOS；未执行 Windows/Linux 原生运行，也未连接真实模型账号。默认 Go 测试均经过 CLI guard，新 Codex 流程使用测试创建的假 app-server。文件系统 Chtimes 错误未作故障注入；失败回退通过缺失／不可用索引场景验证。

PostgreSQL 端点在建库前与现有容器身份一致性校验通过；应用/E2E 与 Go 使用两个独立数据库，Redis 使用任务专用容器。未向用户正常数据库运行测试。测试结束后仅停止本次 API/Web/renderer/Redis 服务，保留 worktree、两个测试库、依赖及构建/验证产物，供复查。

所有代码、测试和记录均位于新分支。main 仍是 `8e123db4801a9867a1f81e68cc87410e040aa1b6`；最终 Git 状态凭据保存在 `.omx/reports/upstream-041-043/final-git-state.json`。测试支持“在已覆盖场景中未发现回归”，不作跨所有环境的绝对保证。
