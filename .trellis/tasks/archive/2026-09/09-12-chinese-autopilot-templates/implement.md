# 内置自动化模板中文化实施计划

目标：完成全部内置执行内容的业务化中文编写，并证明预览、保存、编辑、派发和既有规则正常。

架构：沿用 Go 嵌入注册表与既有自动化创建/执行链路；Web/Desktop 继续共享同一预览页面。技术栈：Go、PostgreSQL、React、Vitest、Playwright。

## 1. 研究和计划

- [x] 阅读 CLAUDE.md、Trellis 流程、中文规范、智能体/skill 先例及全部 9 份原文。
- [x] 创建 PRD 和设计；明确采用中文正文唯一来源及缺陷 priority 修正。
- [x] 保存数据流研究与上下文清单，独立检查设计后 `task.py start`。

## 2. 先建立验收约束

- [x] 修改 `server/internal/service/builtin_autopilot_templates_test.go`：中文正文、巡检去重、汇总评论、日期标题与合法 priority。
- [x] 运行 `go test ./internal/service -run 'TestAutopilotTemplate' -count=1`，记录对当前英文内容的预期失败。
- [x] 保留并复用已有顺序、周期、模式、字段来源、权限和原子性测试。

## 3. 内容实现（可并行）

- [x] 内容执行者仅修改 `server/internal/service/builtin_autopilot_templates/*/PROMPT.md`：9 份业务化中文正文，保持原有规则和阈值。
- [x] 主执行者修改 `builtin_autopilot_templates_roster.go` 的中文文案和 4 个标题；缺陷模板版本升 2。
- [x] 修正 Go/TS 中旧英文正文约定的注释，同步内置自动化 skill 的来源合同。
- [x] 独立检查内容与测试，再运行窄范围测试使之全部通过。

## 4. 数据与交互回归

- [x] 扩展 `server/internal/handler/autopilot_template_test.go`，覆盖全部模板真实落库、用户编辑与目录重读；使用 `dbfx` 和 `testutil.Call`。
- [x] 扩展现有 Go 派发测试，覆盖中文内容与两种执行模式、带时区的日期标题。
- [x] 保留 `e2e/autopilot-template.spec.ts` 请求边界用例，增加真实中文模板 API 的浏览器创建/编辑证据，使用 TestApiClient 隔离工作区与假运行时并清理。
- [x] 使用实际开发 API 验证 9 个中文模板，检查浏览器截图和窄视口。每次视觉检查按 visual-verdict 记录结果。

## 5. 最终检查与 Trellis 收尾

- [x] Go 专项：`go test ./internal/service ./internal/handler ./internal/daemon -run 'TestAutopilotTemplate|TestListAutopilotTemplate|TestInterpolateTemplate|TestBuildPromptAutopilotRunOnly' -count=1 -v`，通过 CLI guard 使用独立数据库；调度器和 cmd/server 的其余自动化回归由完整 `make test` 的 race 套件覆盖。
- [x] TS 相关测试：`pnpm --filter @multica/views exec vitest run autopilots` 与 core 模板 schema 测试。
- [x] 浏览器：`pnpm exec playwright test e2e/autopilot-template.spec.ts --project=chromium`，使用当前 checkout 的服务端口；新的真实流程 spec 一并执行。
- [x] 全量：`pnpm lint`、`pnpm typecheck`、`pnpm test`、`make test`、Go vet / 仓库静态检查。
- [x] 独立审查任务全量差异与 R1–R7，对照原文验证业务语义；修复后复验。
- [x] 更新 `.trellis/spec/server/builtin-templates.md` 的中文正文、标题和 priority 合同，仅保留本任务增量。
- [x] 记录实际命令、结果、截图与覆盖范围到 `verification.md`，勾选 PRD，完成任务归档和日志。仅提交本任务拥有的差异，按 Lore 协议记录；保留其他已有工作。

## 审查与授权记录

用户已要求完整推进到验收，并在 workspace 指令中授权自主处理可逆步骤。本任务在需求和方案完成、评审结论落实后进入执行，不重复询问是否继续。计划存放在 Trellis 任务目录，遵循用户指定的任务管理方式。
