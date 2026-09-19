# Implement — 内置主流 MCP Server 预设模板库

## 顺序

自底向上：服务端名册先落地并自测通过，再往上接。每一步都能独立验证，不会出现「前端写完了才发现接口形状不对」。

### 0. 实现前置

- [ ] 读 `.trellis/spec/server/builtin-templates.md`（内置目录契约）与 `.trellis/spec/core/frontend/type-safety.md`（网络 JSON 必须过 zod）。
- [ ] 逐条复核四个上游 README 的启动命令仍与 `design.md` 的名册一致（上游改包名会让预设当场失效）。

### 1. 服务端名册 + service

- [ ] 新建 `server/internal/service/builtin_mcp_templates.go`：`McpServerTemplate` 结构、`McpServerTemplates()` 返回名册、`Title(language)` / `Description(language)` 复用 `builtin_agent_templates.go:149,175` 的本地化与回落写法。
- [ ] 名册写入首批四条（`design.md` 表格），注释写明 key/args 与 `server/pkg/agent/browser_mcp_config.go:52,61` Windows 兜底的耦合。
- [ ] 新建 `server/internal/service/builtin_mcp_templates_test.go`：`design.md`「名册守卫」五条断言。
- [ ] 验证：`(cd server && go test ./internal/service -run TestMcpServerTemplates -count=1 -v)`

### 2. 服务端 handler + 路由

- [ ] 新建 `server/internal/handler/mcp_template.go`：`McpServerTemplateResponse{key,title,description,config}` 与 `ListMcpServerTemplates`，仿 `handler/skill_template.go` 的最简形状，`language` 走既有 `templateLanguageFromRequest`（`handler/agent_template.go:86`）。
- [ ] `server/cmd/server/router.go`：在成员可见分组内、`r.Get("/mcp-servers", h.ListWorkspaceMcpServers)`（`:1647`）旁加 `r.Get("/mcp-servers/templates", h.ListMcpServerTemplates)`，注释说明成员可见的理由。
- [ ] 新建 `server/internal/handler/mcp_template_test.go`：200 形状、`language=zh` 返回中文、未知 language 回落 en、非成员 403。用 `testutil.Call(h, req).Want(...)`，不要手写 `httptest` 四件套。
- [ ] 验证：`(cd server && go test ./internal/handler -run McpServerTemplate -count=1 -v)` —— 需要可达的 `DATABASE_URL`，否则 DB 用例静默跳过（见记忆：DB-backed Go tests skip silently）。

### 3. core 类型与 API 客户端

- [ ] 新建 `packages/core/types/mcp-template.ts`，在 `packages/core/types/index.ts` 导出。
- [ ] `packages/core/api/schemas.ts`：`McpServerTemplateSchema` / `McpServerTemplateListResponseSchema` / `EMPTY_MCP_SERVER_TEMPLATE_LIST`，紧贴 `:3593-3610` 的写法（全字段 `.default()` + `.loose()`）。
- [ ] `packages/core/api/client.ts`：`listMcpServerTemplates(workspaceId, language, signal)`，`parseWithFallback` + `endpoint` 标注，仿 `:3281` 的 `listSkillTemplates`。
- [ ] 新建 `packages/core/api/mcp-template-schemas.test.ts`（`// @vitest-environment node` 开头）：正常解析、缺字段取默认、`templates` 非数组、`config` 非对象 —— **malformed-response 测试是 CLAUDE.md 的硬要求**。
- [ ] 验证：`pnpm --filter @multica/core test -- mcp-template`

### 4. 共享目录组件扩展

- [ ] `packages/views/common/builtin-template-catalog.tsx`：加可选 `className`（与现有 section 类名 `cn` 合并/覆盖）与 `defaultOpen`（默认 `false`）。
- [ ] 确认 agents / autopilots / squads / skills 四个既有调用方未传新 prop，渲染逐字不变。
- [ ] 验证：`pnpm --filter @multica/views test -- builtin` 与 agents/skills 页面既有测试。

### 5. 设置页内置目录

- [ ] 新建 `packages/views/settings/hooks/use-mcp-server-templates.ts`（`useLocale` + 已导出的 `templateLanguageFor`，`staleTime: 30 * 60 * 1000`）。
- [ ] 新建 `packages/views/settings/components/mcp-builtin-catalog.tsx`：目录分区 + 行 + 「添加」按钮；`existingNames.has(key)` 时按钮禁用并显示「已添加」；`config` 为空的条目跳过不渲染。
- [ ] `packages/views/settings/components/mcp-tab.tsx`：`canManage` 时在 `SettingsSection`（共享 MCP Server）上方挂载目录，`onAdd` 设置 `preset` 并打开弹窗；`existingNames` 复用已有的 `useMemo`（`:63-66`）。
- [ ] 打开弹窗前清掉 `editingServer`，避免与 `replacementMode` 串台。
- [ ] 验证：`pnpm --filter @multica/views test -- mcp-tab`

### 6. 弹窗 preset 预填

- [ ] `packages/views/agents/components/tabs/mcp-server-dialog.tsx`：新增 `preset?: { name: string; config: Record<string, unknown> } | null`，只改 `:256-273` 的 `useEffect` 种子（形状见 `design.md`）。
- [ ] `mcp-server-dialog.test.tsx` 补：传 `preset` 打开后表单已填好 command/args；标题与按钮仍是「添加」而非「更新」；切到 JSON 页签看到同一份 config；名称与既有条目重复时报重名且不提交。
- [ ] 验证：`pnpm --filter @multica/views test -- mcp-server-dialog`

### 7. i18n

- [ ] `packages/views/locales/{en,zh-Hans,ja,ko}/settings.ts` 补 `mcp.builtin_*`（标题、说明、加载中、加载失败、重试、空、添加、已添加）。
- [ ] 中文文案遵循 `apps/docs/content/docs/developers/conventions.zh.mdx` 的产品语气。
- [ ] 说明文案必须点明两件事：模板保存后就是一条普通的共享 MCP Server，仍需去智能体 MCP 页分配；以及运行时需要 `npx`（浏览器类还需要 Chrome）。
- [ ] 验证：`pnpm --filter @multica/views test -- locales`（parity + mcp 键一致性）

### 8. 文档同步

- [ ] `server/internal/service/builtin_skills/multica-creating-agents/references/creating-agents-source-map.md:118` 的工作区 MCP 段落补一行新端点（CLAUDE.md 要求 API 变更与内置 skill 文档同 PR）。

### 9. 收口验证

- [ ] `pnpm typecheck`
- [ ] `pnpm lint`
- [ ] `pnpm test`
- [ ] `(cd server && go test ./internal/service ./internal/handler -count=1)`
- [ ] 人工过一遍 AC1–AC6。

## 风险点与回滚锚

| 文件 | 风险 | 回滚 |
| --- | --- | --- |
| `packages/views/agents/components/tabs/mcp-server-dialog.tsx` | 849 行、被设置页与智能体页共用；改错 `useEffect` 种子会让「替换配置」从空表单变成带旧值，破坏只写语义 | 该文件只增一个可选 prop 与三行种子逻辑；删 prop 即复原 |
| `packages/views/common/builtin-template-catalog.tsx` | 四个既有目录共用 | 两个 prop 都有默认值，不传即原样 |
| `server/cmd/server/router.go` | 路由顺序 | 静态路径，且 `/mcp-servers/{serverId}` 无 GET，无遮挡 |
| 名册 config | 上游改包名后预设静默失效 | 名册测试守不住上游变更，只能靠步骤 0 的复核；在名册注释里留下 README 链接 |

## 不在本次改动内

- 不加数据库列、不加迁移。
- 不动 `workspace_mcp_api.go` 的只写语义。
- 不动 `cmd_workspace.go` 的 CLI（本次不提供 `workspace mcp templates` 子命令）。
