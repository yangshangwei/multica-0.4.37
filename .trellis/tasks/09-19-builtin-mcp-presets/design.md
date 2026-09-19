# Design — 内置主流 MCP Server 预设模板库

## 架构选择

目录**内嵌在服务端二进制**，通过只读列表接口下发；前端拿到完整配置后预填现有弹窗，保存仍走既有的工作区 MCP 创建接口。

采纳理由（均有仓库先例，非新造）：

1. 仓库已有四个同形态内置目录：`ListAgentRoleTemplates` / `ListAutopilotTemplates` / `ListSquadTemplates` / `ListSkillTemplates`（`server/cmd/server/router.go:2050,2092,2174,2230`）。`.trellis/spec/server/builtin-templates.md:3` 已把契约写死：「模板是创建时拷贝进普通实例的预填内容，绝不是新实体类型」。
2. 最贴近的先例是 skill 模板目录（同 spec `:86-94`）：读内嵌注册表、不落库、不返回数据库身份，UI 编辑快照后用普通创建接口落地。本任务与它一一对应。
3. 桌面端是安装版，可能比后端旧。目录放服务端后，「新增一条内置 MCP」只需后端部署，已安装的桌面端立刻能看到；放前端常量则必须发桌面版本。用户明确表示目录会持续增长，这个差异会反复兑现。
4. 静态内容，无需数据库迁移（与现有四个目录一致）。

## 接口契约

```
GET /api/workspaces/{id}/mcp-servers/templates?language=zh
→ 200 {"templates":[{"key","title","description","config"}]}
```

- 挂在工作区成员可见分组（`router.go:1647` 同组），与 `ListAgentRoleTemplates` 的理由相同：内容与工作区无关，但放在工作区路由下客户端无需特殊调用形态。静态路径与 `/mcp-servers/{serverId}` 不冲突（该 path 无 GET，且 chi 静态优先）。
- `language` 复用 `templateLanguageFromRequest`（`server/internal/handler/agent_template.go:86-92`），未知/缺失回落 `en`，不返回 400。
- `config` 是**完整的 MCP 条目 JSON**，与只写的工作区库相反——模板是产品公开内容，不含任何凭据，因此可以下发。首批四条全部免密钥，这个性质由名册测试守住。

## 数据形状

```go
// server/internal/service/builtin_mcp_templates.go
type McpServerTemplate struct {
    Key          string            // 稳定标识，同时作为默认 server 名，必须匹配 ^[A-Za-z0-9_-]+$
    Config       map[string]any    // 原样落进弹窗的 MCP 条目
    Titles       map[string]string // en/zh/ja/ko
    Descriptions map[string]string
}
```

**刻意不带 `Version` 和 `Listed`**（与 agent/autopilot 名册不同）：

- `Version` 在那些名册里存在，只因为创建出的行会记 `template_key`/`template_version` 以便将来做升级 diff。本设计**不做来源标记**（见下），没有任何消费者，加了就是死字段。
- `Listed` 用于「可按 key 创建但不在选择器展示」的条目，本目录没有这种条目；不想展示就不要写进名册。

**不下发 `transport` 字段**：前端已有 `mcpTransport(config)`（`packages/views/agents/components/tabs/mcp-config-model.ts:17`），拿到 config 即可本地判定，服务端再算一遍是重复真相。

## 不做来源标记（关键取舍）

保存后的条目就是一条普通工作区 MCP Server，不记 `template_key`。

理由来自 `.trellis/spec/server/builtin-templates.md:198-202` 的告诫：客户端能自带内容却声称模板身份，就会把模板的身份戳在跑着别的东西的实例上。本流程**故意**允许用户在保存前改配置（R2），所以只要记了来源就必然是不诚实的来源。不记，就没有这个问题，也省掉一次数据库迁移。

代价：目录无法精确知道「这条模板是否已添加」，只能按名称匹配；用户重命名后会看不出已添加。可接受，写进文案与 PRD。

## 数据流

```
名册(Go slice) → service.McpServerTemplates() → handler 本地化 → JSON
  → api.listMcpServerTemplates(wsId, language) → zod 解析 → useMcpServerTemplates()
  → 设置页「内置 MCP」分区渲染
  → 点「添加」→ McpServerDialog(preset={name,config}) 预填
  → 用户可改 → 现有 POST /api/workspaces/{id}/mcp-servers
```

## 前端改动边界

| 层 | 文件 | 改动 |
| --- | --- | --- |
| core/types | `packages/core/types/mcp-template.ts`（新） | `McpServerTemplate` 接口，仿 `types/agent-template.ts` |
| core/api | `packages/core/api/schemas.ts` | `McpServerTemplateSchema` + `...ListResponseSchema`，仿 `:3593-3610`，全字段带 `.default()` 且 `.loose()` |
| core/api | `packages/core/api/client.ts` | `listMcpServerTemplates(workspaceId, language, signal)`，走 `parseWithFallback` |
| views | `packages/views/settings/hooks/use-mcp-server-templates.ts`（新） | 仿 `useRoleTemplates`：`queryKey: ["mcp-server-templates", wsId, language]`，`staleTime: 30 * 60 * 1000`，复用已导出的 `templateLanguageFor` |
| views | `packages/views/common/builtin-template-catalog.tsx` | 加两个可选 prop：`className`（覆盖为列表页设计的 `border-b px-4 py-2` 外壳）、`defaultOpen`。四个既有调用方不传即保持现状 |
| views | `packages/views/settings/components/mcp-builtin-catalog.tsx`（新） | 用上面的共享目录组件 + `BuiltinTemplateRow` 渲染；行内「添加」按钮；已存在同名条目时禁用并显示「已添加」 |
| views | `packages/views/settings/components/mcp-tab.tsx` | 在「共享 MCP Server」分区**上方**挂载目录；`canManage` 为假时不渲染（成员没有添加动作，渲染即噪音） |
| views | `packages/views/agents/components/tabs/mcp-server-dialog.tsx` | 新增 `preset?: { name; config } \| null`；仅影响 `useEffect` 的初始种子 |
| i18n | `packages/views/locales/{en,zh-Hans,ja,ko}/settings.ts` | `settings.mcp.builtin_*` 分区外壳文案。**模板标题/说明来自服务端**，不进 locale 文件，与其余四个目录一致 |

### 弹窗改造的精确形状

`mcp-server-dialog.tsx:256-273` 的 `useEffect` 目前从 `server` 取种子。新增 `preset` 后：

- `setName(preset?.name ?? server?.name ?? "")`
- config 种子：`replacementMode ? {} : (preset?.config ?? server?.config ?? {})`
- `preset` 存在时 `server` 为 `null`，因此标题/按钮仍是「添加」，重名校验（`:290-294`）照常生效——这正是 R6 需要的。
- `preset` 缺省时行为逐字不变，四个既有调用点零影响。

不走「造一个假的 `ManagedMcpServer` 传给 `server`」这条路：那会让弹窗以为在编辑，标题、按钮文案和重名校验全部走错分支。

## 兼容性

- 旧桌面端 + 新后端：不认识新接口 → 不发请求 → 无目录分区，其余功能不变。
- 新前端 + 旧后端：`GET .../templates` 返回 404 → `useQuery` 进 error 分支 → 共享目录组件已有 `failed` 分支渲染「加载失败 + 重试」，不影响下方 MCP 列表。
- 后端新增字段：schema `.loose()` + 全字段 `.default()`，老客户端照常渲染（`schemas.ts:3585-3591` 的既有理由）。
- 模板 config 解析失败 → 默认 `{}` → 前端跳过该行，不渲染一个点了什么都不填的按钮。

## 运行时前提（文案需要说明，不做探测）

四条模板都依赖智能体运行时里有 `npx`（context7 走 HTTP，只需要出网）。chrome-devtools / playwright 还需要浏览器。后端已有 Windows 兜底：`server/pkg/agent/browser_mcp_config.go:52,61` 按 `name == "playwright"` / `"chrome-devtools"` 或 args 含包名注入浏览器路径——**这要求模板的 key 与 args 必须保持这两个形态**，改名会静默失效 Windows 兜底。名册注释里写明这条耦合。

不做连通性探测（Out of Scope）：失败信息在智能体侧才有意义，此处探测只会给出一个与实际运行环境无关的假绿灯。

## 首批名册（已核官方 README）

| key | transport | config | 上游依据 |
| --- | --- | --- | --- |
| `chrome-devtools` | stdio | `{"command":"npx","args":["-y","chrome-devtools-mcp@latest"]}` | 官方 README 标准配置，无密钥 |
| `playwright` | stdio | `{"command":"npx","args":["-y","@playwright/mcp@latest"]}` | 官方 README 为 `npx @playwright/mcp@latest`；**我们补 `-y`**，因为智能体运行时无 TTY，缺 `-y` 时 npx 的确认提示会挂住 |
| `sequential-thinking` | stdio | `{"command":"npx","args":["-y","@modelcontextprotocol/server-sequential-thinking"]}` | 官方 README 原样，无密钥 |

（曾计划的 `context7`（`{"type":"http","url":"https://mcp.context7.com/mcp"}`）已按用户要求移除，首批全部为 stdio。若将来重新加入 http 模板，其 `type:"http"` 写法需与弹窗 `configFromForm` 的输出一致（`mcp-server-dialog.tsx:169`），以保证「模板填入 → 切表单 → 保存」不改写协议。）

## 名册守卫（R5 的实现手段）

新增 `builtin_mcp_templates_test.go`，对每条断言：

1. `Key` 匹配 `^[A-Za-z0-9_-]+$`（否则前端保存必被 `:290` 的校验拒掉）；
2. `Key` 全局唯一；
3. `Config` 非空且至少含 `command` 或 `url`（对齐 `parseServerJson` 的 `missing_target`，`mcp-server-dialog.tsx:183`）；
4. `Titles`/`Descriptions` 覆盖 en/zh/ja/ko 四语言且非空；
5. 配置里不出现疑似密钥的键（`headers.Authorization` / `env` 中含 `TOKEN|KEY|SECRET`）——这是「首批只放无密钥模板」这个产品决定的可执行形式，将来要放需密钥模板时，必须先显式改这条测试，而不是悄悄塞进去。

## 回滚

纯新增：删掉新文件 + 回退 `mcp-tab.tsx`、`mcp-server-dialog.tsx` 的 prop、`builtin-template-catalog.tsx` 的两个可选 prop 即可。无迁移、无数据形状变更，已保存的 MCP Server 不受影响。
