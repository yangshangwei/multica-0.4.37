# 技术设计:周期性 Agent 自动化模板

> 配合 `prd.md` 阅读。本文件描述技术边界、契约、数据流与取舍。执行清单见 `implement.md`。

## 一、定位与不变量

给已有的 Autopilot 子系统补一个**服务端模板层**,对齐仓库中既有的两套模板范式(`AgentRoleTemplate`、`SquadTemplate`)。

**硬不变量(实现全程不得违反):**

- 不修改 `server/internal/scheduler/` 任何文件。
- 不修改 `server/internal/service/cron.go`。
- 不修改 autopilot dispatch 链路(`service/autopilot.go` 的 `DispatchAutopilotForPlan` / `dispatchCreateIssue` / `dispatchRunOnly`)。
- 创建出的实体是**普通 autopilot + 普通 schedule trigger**,与手工创建的无任何行为差异;`AutopilotScheduleDispatchJob`(`scheduler/jobs_autopilot.go:89`)自动接管。

模板对系统的唯一贡献是**内容与 provenance**:预置的 prompt、cron、执行模式,以及落在 autopilot 行上的 `template_key` / `template_version`。

**字段映射约束(实现期发现,阶段 3 必须遵守):** autopilot **没有**独立的 prompt 列。`create_issue` 模式下 `buildIssueDescription`(`service/autopilot.go:1736`)把 `autopilot.description` 直接写进所创建 issue 的正文。因此模板的 `Prompt()`(PROMPT.md 全文)必须落进 `autopilot.description` 列;模板的多语言 `Descriptions` 仅供选择器卡片展示,**不得**入库到 `description`。`Title()` 等展示文案同理仅用于 List 响应。

## 二、后端

### 2.1 注册表(照抄 `builtin_agent_templates*`)

新增文件:

```
server/internal/service/
  builtin_autopilot_templates.go          # 类型 + embed + 访问器
  builtin_autopilot_templates_roster.go   # 四条模板定义
  builtin_autopilot_templates/
    workday-repo-audit/PROMPT.md
    release-readiness/PROMPT.md
    daily-change-review/PROMPT.md
    hourly-queue-check/PROMPT.md
```

`AutopilotTemplate` 结构(照 `AgentRoleTemplate`,`builtin_agent_templates.go:97-132`):

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `Key` | `string` | 稳定标识,落在 autopilot 行,永不改名 |
| `Version` | `int32` | prompt / cron / mode 变更时递增 |
| `Category` | `string` | 分类标签(REPO 健康 / 发布准备 / 定期评审 / 维护),多语言见下 |
| `CronExpression` | `string` | 5 字段标准 cron,须过 `service.ComputeNextRun` 校验 |
| `ExecutionMode` | `string` | `create_issue` \| `run_only` |
| `AvatarEmoji` | `string` | 卡片图标,复用 `emoji:` 标记 |
| `Titles` | `map[string]string` | en/zh/ja/ko;`Title(lang)` 回退 en |
| `Descriptions` | `map[string]string` | 同上 |
| `Categories` | `map[string]string` | 分类的多语言文案 |
| `Listed` | `bool` | 是否在 picker 出现(四条均为 true) |

`Prompt()` 方法从 `builtin_autopilot_templates/<Key>/PROMPT.md` 读取(照 `Instructions()`,`builtin_agent_templates.go:136-146`)。复用既有 `TemplateLanguages`、`localizedTemplateString`、`IsSupportedTemplateLanguage`(同文件 `:158-180`),不重复定义。

访问器:`AutopilotTemplates() []AutopilotTemplate`、`AutopilotTemplateByKey(key) (AutopilotTemplate, bool)`。

**注册表测试**(照 `builtin_skills_test.go` / agent template 测试):每个 Key 的 `PROMPT.md` 可读且非空;cron 过 `ComputeNextRun`;`ExecutionMode` 是两个合法值之一;四语言 Titles/Descriptions 齐全;Key 唯一。

### 2.2 两个端点

新增 `server/internal/handler/autopilot_template.go`:

**`GET /api/autopilots/templates`** — `ListAutopilotTemplates`(照 `ListAgentRoleTemplates`,`agent_template.go:54`)。读 `?language=`,只读、与工作区无关,返回展示字段 + prompt 全文(供前端预览,和 agent template 一样"a person is about to adopt a prompt and should read it first",`agent_template.go:30-32`)。

**`POST /api/autopilots/from-template`** — `CreateAutopilotFromTemplate`。**在一个事务内**完成 autopilot + trigger 创建,消灭现有 `autopilot-dialog.tsx:345-365` 的部分成功路径。

请求体(遵循 provenance 诚实原则:模板决定的内容不由客户端传):

```
{
  "template_key": string,          // 必填
  "assignee_id":  string,          // 必填,agent 或 squad 的 UUID
  "assignee_type": "agent"|"squad",// 可选,默认 agent
  "project_id":   string|null,     // 可选
  "timezone":     string,          // 可选,默认 UTC
  "language":     string,          // 可选,选本地化 title/description 落库
  "subscribers":  [...]            // 可选,同 CreateAutopilotRequest
}
```

`title` / `description` / `execution_mode` / `cron_expression` **全部来自服务端模板**,客户端不传(见 §五 待确认点 D1)。

### 2.3 单事务:必须复现的副作用清单

`CreateAutopilotFromTemplate` 不能只插两行。它必须在**同一个 `h.TxStarter.Begin` 事务**内,按顺序复现 `CreateAutopilot`(`handler/autopilot.go:686-810`)与 `CreateAutopilotTrigger`(`:1410-1540`)的全部实质副作用:

1. **autonomy 门禁** `requireAgentAutonomy(..., AutonomyCoordinator, ...)` — `autopilot.go:690`。standing automation 是协调级操作。
2. **subscribers 解析 + 锁** `parseAutopilotSubscribers` → `lockAndValidateAutopilotSubscribers` — `:751,769`。必须是事务内第一个锁族。
3. **assignee readiness 校验** `validateAutopilotAssigneeForSave` — `:776`。与 runtime teardown 串行化。
4. **`CreateAutopilot`** 插 autopilot 行,**新增 `template_key` / `template_version` 两列写入**。
5. **`recordAutopilotRuleVersion`** v1 + 问责人 — `:801`。漏掉则派发时无 accountable human。
6. **`AddAutopilotSubscriber`** 循环 — `:806`。
7. **trigger 校验 + `computeNextRun`** — `:1521`。cron 来自模板,tz 来自请求(默认 UTC)。
8. **插 schedule trigger + 该 trigger 的 rule version**(atomic,`:1421-1426`)。
9. `tx.Commit`。
10. commit 后 publish `EventAutopilotCreated` + trigger 相关事件(照两个原 handler 的 publish 段)。

**不需要** squad 那种 per-template `pg_advisory_xact_lock`(`squad_template.go:352`):autopilot from-template 是纯插入,无 reuse-or-create 竞态。但 §3 的 subscriber 锁与 assignee 校验锁必须保留。

**实现策略**:把 `CreateAutopilot` handler 中步骤 4–6 的事务内主体、`CreateAutopilotTrigger` 中步骤 7–8 的事务内主体,各抽成一个接收 `qtx` 的内部函数(如 `createAutopilotInTx` / `createScheduleTriggerInTx`),让原两个 handler 与新 from-template 端点复用同一份逻辑。这是净收敛,不是新增并行实现——符合 CLAUDE.md"prefer existing patterns, avoid parallel abstractions"。

### 2.4 路由

`server/cmd/server/router.go` 的 `/api/autopilots` 组(`:2073` 起)内添加。静态路径必须排在 `/{id}` 之前(参照 `:2152-2158` agent templates 的注释):

```go
r.Get("/templates", h.ListAutopilotTemplates)
r.Post("/from-template", h.CreateAutopilotFromTemplate)
```

### 2.5 Migration

新增 `server/migrations/<next>_autopilot_template.up.sql`(照 `450_agent_role_template.up.sql`):

```sql
ALTER TABLE autopilot ADD COLUMN IF NOT EXISTS template_key TEXT NOT NULL DEFAULT '';
ALTER TABLE autopilot ADD COLUMN IF NOT EXISTS template_version INTEGER NOT NULL DEFAULT 0;
```

无新索引(不涉及按 template_key 查询)。若后续需要,单独一个 `CREATE INDEX CONCURRENTLY` 文件。配套 `.down.sql`。之后 `make sqlc` 重新生成,`CreateAutopilot` 的 sqlc 查询(`pkg/db/queries/autopilot.sql`)加这两列。

## 三、前端

### 3.1 交互流程(两步,对齐 `template-create-agent-page.tsx`)

autopilot 不可能纯"一键":`assignee_id` 必填,模板无法预知用户有哪个 agent。

1. **模板选择器** — 卡片网格(照 `RoleTemplatePicker`,`template-create-agent-page.tsx:87`),`grid gap-3 sm:grid-cols-2 lg:grid-cols-3`,含分类 badge、标题、描述。三态齐全(skeleton / load_failed / empty)。
2. **配置步** — 选中模板后进入。展示只读的 prompt 预览 + cron 描述(复用 `schedule-editor/describe.ts`),用户**必须选 assignee**,可选 project / timezone。
3. **提交** — 调 `from-template` 端点,单请求。

### 3.2 入口解放

现状:模板卡片藏在 `autopilots-page.tsx:812-834`,仅 `showEmpty`(`:760`,`totalCount===0`)渲染。

改为:header 的 "New autopilot"(`:772-777`)进入模板选择器,不再直接开空表单;新增路由 `paths.newAutopilotTemplate()`(照 `paths.newAgentTemplate()`,`paths.ts:53`)。空表单仍可从选择器里的"从空白创建"入口到达(保留 `AutopilotDialog` 现有创建能力)。

### 3.3 删除项(旧路径,产品未上线,直接删)

- `autopilots-page.tsx` 的 `TemplateId`(`:127`)、`AutopilotTemplate`(`:135`)、`TEMPLATES` 常量(`:145-207`)。
- 对应 locale 的 `templates.*`:`packages/views/locales/{en,zh-Hans,ja,ko}/autopilots.json`。
- **不删** `autopilot-dialog.tsx` 的部分成功处理与 `autopilot-dialog-toast.ts`(实现期修正)。原计划认为 `from-template` 的单事务会取代所有创建路径,实际不然:模板路径走单事务端点,而**空白创建流仍走 dialog 的两次调用**(`createAutopilot` + `createTrigger`),因此那段部分成功处理仍是必要的正确性代码。删除它会引入真实缺陷。若将来要消灭它,需要另做一个非模板的"建 autopilot 连带 trigger"单事务端点,超出本任务范围。

### 3.4 API 客户端 + schema

- `packages/core/api/schemas.ts`:加 `AutopilotTemplateSchema` + `AutopilotTemplateListResponseSchema`。遵循容错约定(`.loose()` + 全字段 `.default()`,枚举字段用 `z.string()` 而非 `z.enum()`,见 `schemas.ts:3382-3389`)。照 `AgentRoleTemplateSchema`(`:3391`)。
- `packages/core/api/client.ts`:加 `listAutopilotTemplates(language?)` 与 `createAutopilotFromTemplate(data)`,列表走 `parseWithFallback`。照 `:1562,1580`。
- `packages/views/autopilots/use-autopilot-templates.ts`:查询 hook,照 `views/agents/create/use-role-templates.ts`。**放在 views 而非 core**:它需要 `useLocale()` 决定语言,而 `packages/core` 不得依赖 `packages/views`(CLAUDE.md 硬边界),既有的 `useRoleTemplates` 正因此放在 views。语言映射直接复用已导出的 `templateLanguageFor`(`zh-Hans → zh`)。查询不做 workspace scoping——模板随后端二进制走,每个工作区答案相同。
- `packages/core/autopilots/mutations.ts`:加 `useCreateAutopilotFromTemplate`(不需要 locale,可留在 core),带正确的 query 失效,照既有 `useCreateAutopilot`。

### 3.5 平台接线

- web:`apps/web/app/[workspaceSlug]/(dashboard)/autopilots/` 下加模板选择器页面(照 agent template 页)。
- desktop:对应路由接线。
- 共享组件放 `packages/views/autopilots/`,用 `useNavigation()` / `<AppLink>`,不碰 `next/*`。

## 四、四个模板的内容要点

prompt 全文在实现时写入各 `PROMPT.md`。要点:

- **workday-repo-audit**(`run_only`,`0 9 * * 1-5`):检查依赖健康、失败的测试、有风险的开放变更。因是 `run_only` 巡检型,PROMPT 须指引:先查本工作区由本 autopilot 创建的未关闭 issue,同一问题追加评论而非新建;仅在有实质发现时创建 issue。
- **release-readiness**(`create_issue`,`0 17 * * 1`):基于当前项目状态生成每周发布风险摘要。摘要型,每周一份。
- **daily-change-review**(`create_issue`,`0 18 * * *`):扫描近期工作,指出正确性、UX、测试覆盖率风险。摘要型,每日一份。
- **hourly-queue-check**(`run_only`,`0 * * * *`):查卡住的工作、陈旧生成物、失败的本地验证。高频巡检,同 workday-repo-audit 的去重指引。

所有 prompt 为英文(与仓库所有 agent-harness 文本一致,见 `builtin_agent_templates.go:124` 注释);卡片的 Titles/Descriptions/Categories 提供 zh-Hans 等四语言。

## 五、已确认的设计决策

**D1 —— 创建时不可编辑 prompt / 周期,创建后可改。**

`from-template` 请求体不接受 `title` / `description` / `execution_mode` / `cron_expression`,这些一律由服务端从模板取。配置步只需选 assignee(必)与 project / timezone(可选),prompt 与周期以只读形式预览。

理由:①与 `AgentRoleTemplate` 的 provenance 诚实原则一致(`agent_template.go:94-97`:客户端不得一边声称模板溯源、一边提供自己的 prompt);②配置步无需挂 prompt / cron 编辑器,前端最简;③想改的用户在 autopilot 详情页改,实例本就可编辑,`template_key` 记录的是"这份 copy 来自哪里",与 agent 模板的 copy 语义一致。

已否决:创建时即可编辑。对 autopilot 而言安全上可行(创建 autopilot 本就开放任意 prompt,无额外权限逃逸),但需要在配置步挂完整的 prompt 编辑器与 cron 编辑器,且 `template_key` 语义弱化。

## 六、测试

| 层 | 位置 | 覆盖 |
| --- | --- | --- |
| 注册表 | `server/internal/service/builtin_autopilot_templates_test.go` | 每模板 PROMPT 非空、cron 合法、mode 合法、四语言齐全、Key 唯一 |
| handler | `server/internal/handler/autopilot_template_test.go` | list 返回四条;from-template 成功后 autopilot+trigger 同在、template_key/version 正确、cron 与模板一致;trigger 校验失败时 autopilot 回滚无孤儿;autonomy 不足 403;非法 template_key 400。用 `dbfx` / `testutil.Call` |
| schema | `packages/core/api/autopilot-template-schemas.test.ts` | 正常解析 + malformed-response 降级(照 `agent-template-schemas.test.ts`) |
| 组件 | `packages/views/autopilots/*.test.tsx` | picker happy path、三态、选中态 hover 可辨识 |
| e2e | `e2e/autopilot-template.spec.ts` | 选模板→选 agent→创建→列表出现。照 `agent-role-template.spec.ts`,用 `TestApiClient` |

## 七、风险与回滚

- **风险:抽取 `createAutopilotInTx` 改动了现有 CreateAutopilot 行为。** 缓解:抽取为纯重构,原 handler 测试须全绿;先重构、后接新端点,分两步提交。
- **风险:migration 加列影响现有 autopilot 读写。** 缓解:两列均 `NOT NULL DEFAULT`,存量行自动填默认值;`make sqlc` 后跑 `make test`。
- **回滚点**:注册表 + 端点 + migration 为一组(后端可独立上线,不改前端仍安全);前端 picker + 删旧模板为一组。后端先行,前端后接。
