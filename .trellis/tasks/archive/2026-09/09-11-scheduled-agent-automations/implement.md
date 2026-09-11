# 执行计划:周期性 Agent 自动化模板

> 依赖 `prd.md`(需求与验收)与 `design.md`(技术设计)。按阶段顺序执行,每阶段末尾的验证命令必须通过后才进入下一阶段。

## 阶段 0 — 纯重构:抽出可复用的事务内主体

**目的**:让 `from-template` 与现有两个 handler 共用同一份创建逻辑,而不是并行实现一套。此阶段**不改变任何外部行为**。

- [x] 从 `server/internal/handler/autopilot.go` 的 `CreateAutopilot`(`:686-810`)中,把事务内主体抽成 `createAutopilotInTx(ctx, qtx, params) (db.Autopilot, error)`,涵盖:`qtx.CreateAutopilot` 插行、`recordAutopilotRuleVersion` v1、`AddAutopilotSubscriber` 循环。
- [x] 从 `CreateAutopilotTrigger`(`:1410-1540`)中,把 schedule 分支的事务内主体抽成 `createScheduleTriggerInTx(ctx, qtx, params) (db.AutopilotTrigger, error)`,涵盖:`computeNextRun`、插 trigger、写该 trigger 的 rule version。
- [x] 两个原 handler 改为调用抽出的函数,保持锁顺序不变(subscriber 锁仍是事务内第一个锁族)。

**验证**:

```bash
cd server && go build ./... && go vet ./...
make test
```

**Review gate**:现有 autopilot handler 测试必须全绿且无新增/修改。若为通过测试而改动了断言,说明重构不是行为等价的,回退重做。

**回滚点**:此阶段可独立提交(`refactor(server): extract autopilot create bodies for reuse`),后续阶段失败不需回退它。

---

## 阶段 1 — 模板注册表与内容

- [x] 新建 `server/internal/service/builtin_autopilot_templates.go`:`//go:embed builtin_autopilot_templates`、`AutopilotTemplate` 结构(字段见 design §2.1)、`Prompt()`、`Title()/Description()/Category()`。复用既有 `TemplateLanguages`、`localizedTemplateString`、`IsSupportedTemplateLanguage`,**不要重复定义**。
- [x] 新建 `builtin_autopilot_templates_roster.go`:四条定义 + `AutopilotTemplates()` + `AutopilotTemplateByKey()`。

| Key | Category | cron | ExecutionMode |
| --- | --- | --- | --- |
| `workday-repo-audit` | REPO 健康 | `0 9 * * 1-5` | `run_only` |
| `release-readiness` | 发布准备 | `0 17 * * 1` | `create_issue` |
| `daily-change-review` | 定期评审 | `0 18 * * *` | `create_issue` |
| `hourly-queue-check` | 维护 | `0 * * * *` | `run_only` |

- [x] 写四个 `builtin_autopilot_templates/<key>/PROMPT.md`。英文。两个 `run_only` 模板的 prompt **必须**包含去重指引:先检索本工作区由本 autopilot 创建的未关闭 issue,同一问题追加评论而非新建;仅在有实质发现时创建 issue。
- [x] 四语言 `Titles` / `Descriptions` / `Categories`(en/zh/ja/ko),文案以 `prd.md` §4 的中文为准翻译。
- [x] 新建 `builtin_autopilot_templates_test.go`:每模板 PROMPT 非空;cron 过 `service.ComputeNextRun`;ExecutionMode 是两个合法值之一;四语言齐全;Key 唯一。

**验证**:

```bash
cd server && go test ./internal/service/ -run AutopilotTemplate -count=1 -v
```

---

## 阶段 2 — Migration 与 sqlc

- [x] 新建 `server/migrations/<next>_autopilot_template.up.sql`:给 `autopilot` 加 `template_key TEXT NOT NULL DEFAULT ''`、`template_version INTEGER NOT NULL DEFAULT 0`,均用 `IF NOT EXISTS`。配套 `.down.sql`。不加索引。
- [x] 更新 `server/pkg/db/queries/autopilot.sql` 的 `CreateAutopilot`,写入这两列。
- [x] `make sqlc` 重新生成。

**验证**:

```bash
make sqlc && cd server && go build ./...
make test
```

**Review gate**:确认迁移文件里没有 `FOREIGN KEY` / `REFERENCES` / 级联,且没有非 `CONCURRENTLY` 的索引创建(CLAUDE.md 硬要求)。

---

## 阶段 3 — 两个端点

- [x] 新建 `server/internal/handler/autopilot_template.go`。
- [x] `ListAutopilotTemplates`:照 `agent_template.go:54`,`templateLanguageFromRequest` 读 `?language=`,返回 `{"templates": [...]}`,含 prompt 全文。
- [x] `CreateAutopilotFromTemplate`:请求体见 design §2.2。按 design §2.3 的十步清单在**单事务**内复现全部副作用——特别注意不要漏 `requireAgentAutonomy(AutonomyCoordinator)`、`lockAndValidateAutopilotSubscribers`、`validateAutopilotAssigneeForSave`、`recordAutopilotRuleVersion`。未知或 `Listed=false` 的 `template_key` 返回 400。
- [x] commit 后 publish 事件,照原两个 handler 的 publish 段。
- [x] `server/cmd/server/router.go` 的 `/api/autopilots` 组(`:2073` 起)加两条路由,**静态路径排在 `/{id}` 之前**。
- [x] 新建 `server/internal/handler/autopilot_template_test.go`,用 `dbfx` 建行、`testutil.Call` 驱动:list 返回四条;from-template 成功后 autopilot 与 trigger 同在、`template_key`/`template_version` 正确、trigger cron 与模板一致;**trigger 创建失败时 autopilot 一并回滚,库中无孤儿 autopilot**;autonomy 不足 403;非法 template_key 400。

**验证**:

```bash
cd server && go test ./internal/handler/ -run AutopilotTemplate -count=1 -v
make test
```

**Review gate**:回滚测试必须真实断言"库里查不到该 autopilot",不能只断言 HTTP 状态码。后端到此可独立上线,前端未改仍安全。

---

## 阶段 4 — 前端数据层

- [x] `packages/core/api/schemas.ts` 加 `AutopilotTemplateSchema` + `AutopilotTemplateListResponseSchema`。`.loose()` + 全字段 `.default()`,枚举字段用 `z.string()` 不用 `z.enum()`(见 `schemas.ts:3382-3389` 的理由)。
- [x] `packages/core/api/client.ts` 加 `listAutopilotTemplates(language?)`(走 `parseWithFallback`)与 `createAutopilotFromTemplate(data)`。
- [x] `packages/core/autopilots/mutations.ts` 加 `useCreateAutopilotFromTemplate`,失效正确的 query key(照既有 `useCreateAutopilot`)。
- [x] 新建 `packages/core/api/autopilot-template-schemas.test.ts`:正常解析 + malformed-response 降级不抛(照 `agent-template-schemas.test.ts`)。首行加 `// @vitest-environment node`。

**验证**:

```bash
pnpm typecheck
pnpm test --filter @multica/core
```

---

## 阶段 5 — 前端界面

- [x] `packages/views/autopilots/` 下新建模板选择器组件:卡片网格 `grid gap-3 sm:grid-cols-2 lg:grid-cols-3`,每卡呈现分类 badge、标题、描述;skeleton / load_failed / empty 三态。照 `template-create-agent-page.tsx:87` 的 `RoleTemplatePicker`。
- [x] 配置步:只读 prompt 预览 + cron 人类可读描述(复用 `schedule-editor/describe.ts`),assignee 选择器(必填,未选不可提交),可选 project / timezone(timezone 默认取浏览器时区)。
- [x] `packages/core/paths/paths.ts` 加 `newAutopilotTemplate()`(照 `newAgentTemplate()`,`:53`)。
- [x] `autopilots-page.tsx` header 的 "New autopilot"(`:772-777`)改为进入模板选择器;选择器内保留"从空白创建"入口指向现有 `AutopilotDialog`。
- [x] web 平台接线:`apps/web/app/[workspaceSlug]/(dashboard)/autopilots/` 下加页面。
- [x] desktop 平台接线:对应路由。
- [x] 组件测试:`packages/views/autopilots/*.test.tsx`,覆盖 happy path、三态、**选中态在 hover 下仍可辨识**。不得 mock `next/*` 或 `react-router-dom`。

**验证**:

```bash
pnpm typecheck && pnpm lint
pnpm test --filter @multica/views
```

**Review gate**:UI 用语义 token(`bg-background` / `text-muted-foreground`),字号用 `--text-*` 角色标度(`text-caption` / `text-body` / `text-title`),不得出现硬编码颜色或 Tailwind 默认 `text-sm` / `text-base`。

---

## 阶段 6 — 删除旧路径

- [x] 删 `autopilots-page.tsx` 的 `TemplateId`(`:127`)、`AutopilotTemplate`(`:135`)、`TEMPLATES`(`:145-207`)及其渲染分支(`:812-834`)与 `openCreate(tpl)` 的模板分支。
- [x] 删 `packages/views/locales/{en,zh-Hans,ja,ko}/autopilots.json` 的 `templates.*` 条目。
- [x] 删 `autopilot-dialog.tsx:345-365` 的部分成功处理,以及 `autopilot-dialog-toast.ts` 中仅服务于该场景的部分。若 dialog 的直接创建路径仍需两次调用,保留其自身逻辑,仅删 from-template 已覆盖的部分。

**验证**:

```bash
grep -rn "TEMPLATES" packages/views/autopilots/    # 应无结果
pnpm typecheck && pnpm lint && pnpm test
```

---

## 阶段 7 — 端到端与全量验证

- [x] 新建 `e2e/autopilot-template.spec.ts`,照 `e2e/agent-role-template.spec.ts`,用 `TestApiClient` 做 setup/teardown:进入模板选择器 → 选一个模板 → 选 agent → 创建 → 断言列表中出现且 trigger 已生成。
- [x] 逐条比对 `prd.md` 的 Acceptance Criteria 打勾。

**验证**:

```bash
make test
pnpm typecheck && pnpm lint && pnpm test
pnpm exec playwright test autopilot-template
make check
```

**最终 Review gate**:

- 确认 `git diff --stat` 中**没有** `server/internal/scheduler/`、`server/internal/service/cron.go`、`service/autopilot.go` 的 dispatch 段改动(design §1 硬不变量)。
- 确认迁移合规(无外键、无非 CONCURRENTLY 索引)。
- 确认没有为通过测试而放宽断言。

---

## 提交切分

| 提交 | 范围 |
| --- | --- |
| `refactor(server): extract autopilot create bodies for reuse` | 阶段 0 |
| `feat(server): add builtin autopilot template registry` | 阶段 1 |
| `feat(server): record template provenance on autopilot` | 阶段 2 |
| `feat(server): add autopilot template list and from-template endpoints` | 阶段 3 |
| `feat(core): add autopilot template api and hooks` | 阶段 4 |
| `feat(views): add autopilot template picker` | 阶段 5 |
| `refactor(views): drop hardcoded autopilot templates` | 阶段 6 |
| `test(e2e): cover autopilot template creation` | 阶段 7 |
