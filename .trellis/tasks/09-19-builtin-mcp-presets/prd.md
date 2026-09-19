# 内置主流 MCP Server 预设模板库

## Goal

让工作区 owner/admin 在「设置 → MCP」从平台维护的内置目录里挑一个主流 MCP，一键填入现有添加表单，确认后保存为工作区自己的 MCP Server。首条为 `chrome-devtools`，目录后续持续增长。

用户价值：今天添加一个 MCP 必须手抄 `command` / `args` / `env`，把 `npx -y chrome-devtools-mcp@latest` 拆成命令加逐条参数；配置保存后不可读回，任何一处拼错都要重填一份完整配置。模板把这件事变成「选一个 → 保存」。

## Background：既有实现（已勘察，无需再确认）

### 工作区 MCP 库

- 页面 `packages/views/settings/components/mcp-tab.tsx`，owner/admin 才能管理（`:54`）。
- 添加/替换弹窗 `packages/views/agents/components/tabs/mcp-server-dialog.tsx`，与智能体 MCP 页共用，含 `form` / `json` 两种模式（`:39`）。表单只能表达 stdio 与 http，其它传输强制走 JSON，否则保存会改写协议（`:129-159`）。
- 配置**只写**：`GET /api/workspaces/{id}/mcp-servers` 只回 `id/name/transport/created_at/updated_at`，永不回 `url`/`command`/`args`/`headers`/`env`（`server/internal/handler/workspace_mcp_api.go:23-31`）。设置页的「替换配置」因此从空表单重填。
- 名称须匹配 `^[A-Za-z0-9_-]+$` 且工作区内唯一（`mcp-tab.tsx:147-154`、`mcp-server-dialog.tsx:290-294`）。
- 路由：`server/cmd/server/router.go:1647`（成员可见列表）、`:1671-1673`（admin 写）、`:2200-2203`（智能体分配）。
- 添加后不自动分配给任何智能体。

### 可复用的内置目录先例

- 四个同形态目录：`ListAgentRoleTemplates` / `ListAutopilotTemplates` / `ListSquadTemplates` / `ListSkillTemplates`（`router.go:2050,2092,2174,2230`），契约见 `.trellis/spec/server/builtin-templates.md`。
- 名册是产品控制顺序的 Go slice，条目带多语言 `Titles`/`Descriptions`（`builtin_agent_templates_roster.go:12-35`）；本地化与回落在 `builtin_agent_templates.go:149,175`。
- 前端四个目录共用同一个展示组件 `packages/views/common/builtin-template-catalog.tsx`（工作区自有列表上方的可折叠分区 + 行内操作按钮），调用方见 `builtin-agent-catalog.tsx`、`builtin-skill-catalog.tsx` 等。
- 数据层惯例：`useRoleTemplates`（`packages/views/agents/create/use-role-templates.ts`）不按工作区分键、带 `language`、`staleTime` 30 分钟；zod 形状见 `packages/core/api/schemas.ts:3593-3610`。

### chrome-devtools / playwright 已被后端特殊照顾

`server/pkg/agent/browser_mcp_config.go:52,61` 在 Windows 下按 `name == "playwright"` / `"chrome-devtools"` 或 args 含包名注入浏览器可执行路径。模板必须产出同样的 key 与 args 形态，否则该兜底静默失效。

### 工程约束

- 四语言 locale 且有 parity 测试：`packages/views/locales/{en,zh-Hans,ja,ko}`，`parity.test.ts`、`mcp.test.ts`。
- 网络 JSON 必须过 `parseWithFallback` + zod（`packages/core/api/schema.ts`），禁止直接 cast，且需 malformed-response 测试。
- 目录为静态内嵌内容，不需要数据库迁移。

## Key Decisions

| 决定 | 结论 | 依据 |
| --- | --- | --- |
| 内置形态 | 预设模板库：填入表单后仍保存为工作区自己的 MCP Server，不做「平台自带免添加」 | 用户选定 |
| 目录存放 | 服务端内嵌 + 只读接口下发 | 四处先例；桌面端是安装版，新增模板只需后端部署即可触达 |
| 入口 | 设置页「共享 MCP Server」上方的可折叠「内置 MCP」分区，行内「添加」直接打开预填弹窗 | 用户选定；复用既有共享目录组件 |
| 首批范围 | 只放免密钥模板，密钥占位/必填机制留待后续 | 用户选定 |
| 来源标记 | 不记 `template_key` | 用户可在保存前改配置，记来源必然不诚实（`.trellis/spec/server/builtin-templates.md:198-202`）；也省掉一次迁移 |

首批三条（均已核对上游官方 README，全部免密钥）：

| key | transport | config |
| --- | --- | --- |
| `chrome-devtools` | stdio | `npx -y chrome-devtools-mcp@latest` |
| `playwright` | stdio | `npx -y @playwright/mcp@latest`（上游无 `-y`，我们补上：智能体运行时无 TTY，npx 确认提示会挂住） |
| `sequential-thinking` | stdio | `npx -y @modelcontextprotocol/server-sequential-thinking` |

（原先候选的 `context7`（http）已按用户要求移除。首批因此全部为 stdio；弹窗的 http 分支是既有功能仍可手动使用，只是当前没有内置模板走这条路。）

## Requirements

- R1 平台维护一份内置 MCP 模板目录，首批为上表四条。
- R2 设置页可浏览目录并一键将模板配置填入现有表单/JSON 编辑器，用户可在保存前修改。
- R3 保存仍走既有 `POST /api/workspaces/{id}/mcp-servers`，产出普通工作区 MCP Server：只写、可重命名、可替换、可删除、仍需单独分配给智能体。
- R4 目录条目带多语言展示名与说明，覆盖 en / zh-Hans / ja / ko；分区外壳文案进 locale 文件，条目文案由服务端按 `language` 下发。
- R5 新增一条内置模板的成本 = 改一处名册 + 补四语言文案，不需要迁移、不需要改交互代码；名册测试守住 key 格式、唯一性、config 有效性、四语言齐备、无密钥。
- R6 与既有条目重名时不得静默覆盖：目录行显示「已添加」且按钮禁用，弹窗侧重名校验仍然生效。
- R7 目录不可用（旧后端 404、网络失败、条目 config 畸形）时，下方 MCP 列表与手动添加流程不受影响。

## Acceptance Criteria

- [ ] AC1 设置 → MCP 页出现「内置 MCP」分区；点 `chrome-devtools` 的「添加」，弹窗已填好 `npx` + `-y` + `chrome-devtools-mcp@latest`，直接保存成功。
- [ ] AC2 保存后的条目在「共享 MCP Server」列表中显示为 stdio，可重命名、可替换配置、可删除，且未分配给任何智能体。
- [ ] AC3 已存在同名条目时该行显示「已添加」且按钮禁用；即使状态陈旧绕过，弹窗仍报重名且不提交、不覆盖。
- [ ] AC4（已作废）原为验证 `context7`（http）模板；context7 已移除，首批全为 stdio。弹窗 http 分支的正确性由 `mcp-server-dialog.test.tsx` 的既有用例保证。
- [ ] AC5 目录响应经 zod 解析：缺字段取默认、`templates` 非数组、`config` 非对象都不会让页面崩溃（有 malformed-response 测试）；接口 404 时分区显示「加载失败 + 重试」，下方列表正常。
- [ ] AC6 选中模板后切到 JSON 模式看到同一份配置，手改后保存生效；弹窗标题与主按钮仍是「添加」而非「更新」。
- [ ] AC7 四语言 locale parity 与 `mcp.test.ts` 键一致性测试通过。
- [ ] AC8 名册测试覆盖 R5 的五条断言，其中「无密钥」断言使得将来加入需密钥模板必须显式修改该测试。

## Out of Scope

- 不做「平台自带、无需添加即可分配」的运行时内置 MCP。
- 不做模板版本升级 / diff，不记来源标记，不加数据库列。
- 不做需要密钥或需要用户补参数的模板（如 filesystem 需要路径），以及配套的占位符与必填校验机制。
- 不做 MCP 连通性探测或工具列表预览。
- 不动 Composio 集成面板（`agent-mcp-tab.tsx` 是另一条线）。
- 不提供 `workspace mcp templates` CLI 子命令。
