# 调试 / RCA 能力验收记录

完整设计 / 实现审计、前端与文档修复、全仓检查及独立 skill 前向试用的汇总见 [audit.md](./audit.md)。以下保留并发验收轮次的独立资源、命令和结果。

验收日期：2026-09-23。最终完整 Go 套件结束于 11:28:47（Asia/Shanghai）。

**结论：本轮要求的完整 service/handler 测试、文本评审和过时任务记录更新均已完成。** AC1–AC8 已勾选；实现以 Lore 格式提交为 `e348b009a`，任务状态已更新为 `completed` 并归档。未发布。

## 完整数据库测试

从仓库当前本地连接配置派生单独的测试数据库 `multica_rca_verify_4ebce7bc2f`，没有在开发数据库上执行测试。PostgreSQL 为 17.11；通过项目迁移入口 `go run ./cmd/migrate up` 建库，迁移账本包含 480 条记录。另启用本轮独占的 Valkey 7.2.11 容器，设置 `REDIS_TEST_URL`，避免 Redis 用例因缺少环境而跳过。

在 `server/` 下执行，`DATABASE_URL` 与 `REDIS_TEST_URL` 均指向上述隔离资源：

```bash
bash ../scripts/go-test-with-agent-cli-guard.sh -- \
  go test -race -json -count=1 ./internal/service/... ./internal/handler/...
```

| 包 | 顶层测试通过 | 含子用例通过 | 失败 | 显式跳过 | 耗时 |
| --- | ---: | ---: | ---: | ---: | ---: |
| service | 418 | 656 | 0 | 0 | 32.128 s |
| handler | 2,196 | 3,702 | 0 | 2 | 79.451 s |
| 合计 | 2,614 | 4,358 | 0 | 2 | — |

退出码为 0，未报告 data race，也没有意外执行真实 agent CLI。JSON 事件确认数据库用例实际执行，没有数据库或 Redis 不可用造成的跳过。

两个原有显式跳过项：

- `TestWebhookHandler_DBErrorOnTokenLookupReturns500`：该测试源码尚未注入 stub Queries，主动跳过 500 分支；这是既有测试缺口。
- `TestFetchFromSkillsSh_AnthropicPptxIntegration`：需要显式开启 `MULTICA_RUN_SKILLS_SH_INTEGRATION=1` 的真实 GitHub 集成，本轮未开启。

新增的 `TestCreateAgentFromTemplate_DiagnosticianCopiesAndPreservesWorkspaceContent` 已在最终完整套件中通过。它通过真实数据库验证角色创建、服务端默认值、唯一 debugging skill 的物化、`quality` / `microscope` 元数据，以及再次创建角色时保留已有工作区自定义正文。

## 其他检查

下列检查均实际执行并通过：

- `go build ./...`。
- `go vet ./internal/service/... ./internal/handler/...`。
- `go test ./internal/service/... -run 'BuiltinSkill|RoleSkill|AgentRoleTemplate|Diagnostician' -count=1`。
- 本任务涉及的 5 个 Go 文件 `gofmt -l` 无输出。
- `git diff --check` 无输出。
- `pnpm --filter @multica/views exec vitest run skills/lib/skill-presentation.test.ts`：51 个测试通过。
- `pnpm --filter @multica/views typecheck`。
- `pnpm --filter @multica/views exec eslint skills/lib/skill-presentation.ts skills/lib/skill-presentation.test.ts`。

前端检查针对工作区并发补充的 debugging 内置识别和四语言展示；本轮没有编辑这些前端文件，也没有运行完整前端测试套件或浏览器 E2E。

## 文本评审与记录同步

独立审查代理复核了最终两份完整正文，AC7 通过，详见 [text-review.md](./text-review.md)。本轮修正的主要矛盾是：

- 未查明的结果允许如实交付证据缺口，不强迫编造具体根因、最小复现或修复方向。
- 数据丢失、安全缺陷或凭据泄露触发报告后停止，由人工决定后续处理。
- 明确失败复现测试的运行证据、已有改动保护，并校准 skill 的适用条件和 test-report 的边界。

最终并发实现还明确了无法自动化时的交付、依赖 / 配置 / 环境根因、skill 不授予权限，以及按证据进展和约定预算决定何时停止。这些最终文本已重新复核，并纳入最后一轮完整测试。

PRD、design、implement、上下文清单及 `task.json` 已同步实际分类 `quality`、图标 `microscope`、10 个角色 / 9 个角色 skill、当前测试名和依赖完成状态。taxonomy 功能提交为 `28c91e13d`，归档提交为 `2350999ce`。模板 spec 也已更新。`drafts/` 仅保留规划快照，不作为当前实现契约。

## 证据与限制

- 结构化结果、命令、关键源文件 SHA-256 和日志校验值见 [verification.json](./verification.json)。
- 完整原始日志保存在 `/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-rca-verification-20260923-9l0z4g50/`。最终结果以 `tests.jsonl`、`test-summary.json` 为准；早期运行另以 `initial-` / `second-` 前缀保留。
- 期间出现同任务的并发修改；最终完整运行期间受检 service/handler/skill 的 Go 与 Markdown 文件没有变化（`changed_during_run: []`）。落盘前再次核对了源文件指纹，保持一致。
- 本轮创建的测试数据库与 Redis 容器均已清理，日志保留。
- 静态文本评审和模板测试不等于真实模型的 RCA 行为评测；本轮没有调用真实模型、修改小队路由或验证生产部署。
