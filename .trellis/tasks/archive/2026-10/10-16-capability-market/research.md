# 能力市场调研记录

日期：2026-10-16；调研起点 main / 5049e0f6a。只读检查业务代码，未运行产品测试。

## 导航与平台

| 事实 | 代码证据 |
| --- | --- |
| 技能库、MCP、运行时位于“资源与连接”分组 | packages/views/layout/app-sidebar.tsx:175 |
| 现有路径 skills / mcp / skillDetail | packages/core/paths/paths.ts:101 |
| Web 是共享页面薄导出 | apps/web/app/[workspaceSlug]/(dashboard)/skills/page.tsx:1；apps/web/app/[workspaceSlug]/(dashboard)/mcp/page.tsx:1 |
| Desktop 注册工作空间会话路由 | apps/desktop/src/renderer/src/routes.tsx:181 |
| 页面注册表控制导航图标和标签身份 | packages/core/paths/route-icons.ts:101；packages/core/paths/tab-subject.ts:126；packages/views/layout/route-icon-components.tsx:36 |
| 命令搜索读取注册表，但关键词需同步补全 | packages/views/search/search-command.tsx:79、:95 |
| 编辑器无 workspace 前缀的页面链接有独立识别集合 | packages/views/editor/utils/link-handler.ts:23 |
| 共享导航不依赖框架路由 | packages/views/navigation/types.ts:1；CLAUDE.md |
| 中文 skill 术语与技能库导航名称 | apps/docs/content/docs/developers/conventions.zh.mdx:138 |

新增工作空间子路由不等于新增全局路由，无须仅因使用 marketplace 名称修改全局保留字。路径一致性测试、命令搜索、编辑器链接和桌面标签均需纳入变更。

## Skill：模板目录与工作空间副本

| 事实 | 代码证据 |
| --- | --- |
| SkillLibraryCatalog 组合工作空间列表与市场双 Tab | packages/views/skills/components/skill-library-catalog.tsx:35、:89 |
| 可提取模板查询/筛选及模板面板，不应挂载完整 SkillsPage | 同文件 :38、:84、:121；packages/views/skills/components/skills-page.tsx:709 |
| 卡片已有来源、分类、关联副本和详情链接 | packages/views/skills/components/skill-template-card.tsx:16、:71、:90 |
| 副本通过 config.template_source.name 关联，支持改名和多个副本 | packages/views/skills/lib/skill-template-discovery.ts:16、:40 |
| 创建为独立工作空间副本，记录模板来源/版本 | packages/core/skills/template-draft.ts:105 |
| 模板和工作空间列表分别查询，有前台轮询 | packages/core/workspace/queries.ts:141、:148 |
| 普通创建进详情，市场创建留页并通过 toast 打开详情 | packages/views/skills/components/skills-page.tsx:918、:922 |
| 现有编辑器支持指定模板、完成回调和恢复结果 | packages/views/skills/components/skills-page.tsx:980；packages/views/skills/components/create-skill-dialog.tsx:650、:699、:725 |
| 关闭恢复触发控件焦点，失效时使用稳定页内操作 | packages/views/skills/components/skills-page.tsx:1212 |
| libraryView、来源和分类在同一持久化 store | packages/core/skills/stores/view-store.ts:87、:196、:209 |
| Agent SkillAddDialog 只消费已有工作空间实例 | packages/views/agents/components/skill-add-dialog.tsx:25、:48、:78 |
| 模板读取不创建资源，现有 API 可复用 | server/internal/handler/skill_template.go:26；packages/core/api/client.ts:3926、:3939、:3953 |

提取风险：

1. 移除 libraryView 对所在页面的决定权，以及确认空库后自动进入市场的逻辑。持久化 merge 会展开旧 payload，仅删除类型不足以清理废弃字段；来源/分类继续保留。
2. 旧双 Tab 的 keepMounted 用于保护列表虚拟器的滚动节点。独立技能库直接拥有列表生命周期，不能由子组件隐式卸载父级虚拟器的容器。
3. 保留模板创建 dirty / pending / unknown-result 保护及两种成功回调；类型切换不能悄悄卸载编辑器。
4. 查询失败不是“没有副本”；改名副本不能通过显示名称匹配。
5. 独立市场不应初始化完整 SkillsPage 的管理查询、工具栏和虚拟器。

规范：.trellis/spec/views/frontend/skill-market-discovery.md。
历史背景：.trellis/tasks/archive/2026-09/09-29-skill-library-separation/design.md；.trellis/tasks/archive/2026-09/09-28-skill-market/prd.md。

## MCP：模板、实例与显式分配

| 事实 | 代码证据 |
| --- | --- |
| McpPage 是 McpTab 的页面壳 | packages/views/mcp/mcp-page.tsx:5 |
| McpTab 管理创建、改名、替换、删除及再次分配 | packages/views/settings/components/mcp-tab.tsx:57、:428 |
| McpLibraryCatalog 组合管理 children 与目录双 Tab | packages/views/mcp/mcp-market.tsx:31、:51 |
| McpTemplateCatalog 已独立，包含筛选、卡片、实例关联及弹窗 | packages/views/mcp/mcp-market.tsx:128、:136 |
| 模板身份是 source + key，一个模板可以有多个实例 | packages/views/mcp/mcp-catalog.ts:5、:9 |
| 保存实例 ID 后再显式分配 | packages/views/mcp/mcp-setup-dialog.tsx:76、:245、:311 |
| Agent 入口传目标及运行时上下文 | packages/views/agents/components/tabs/mcp-config-tab.tsx:414、:539 |
| Agent 发现先列已有实例，再复用模板目录 | packages/views/mcp/mcp-agent-discovery.tsx:125、:238 |
| 成员读目录及摘要，配置写入限定 human owner/admin | server/cmd/server/router.go:1735；server/internal/handler/workspace_mcp_api.go:127 |
| Agent 所有者或 workspace 管理者可管理对应分配 | server/internal/handler/workspace_mcp_api.go:455 |
| 部署目录通过工作空间 API 提供访问 | server/internal/handler/mcp_template.go:31 |
| 创建 mutation gcTime: 0；安全摘要不含配置/凭据 | packages/core/workspace/mutations.ts:106；server/internal/handler/workspace_mcp_api.go:17 |
| 现有目录、模板创建、分配 API 可复用 | packages/core/api/client.ts:3428、:3439、:3478、:3542 |
| Settings 另有 McpTab 挂载点 | packages/views/settings/components/settings-page.tsx:263 |

注意 mcp-config-tab.tsx 才是本次工作空间 MCP 入口；agent-mcp-tab.tsx 管理 Composio allowlist，属于其他功能。

提取风险：

1. 目录仍需 workspace、成员权限、已有实例及旧版内置模板的自定义编辑回调，不能只搬卡片。
2. 独立目录当前 poll: true；新类型切换和桌面隐藏标签必须控制轮询。缓存继续按 API 地址、工作空间、语言分区。
3. 保存后实例数从 0 变 1，不能自动跳到管理页并中断分配。
4. 保存、分配、启用、运行时已连接是四种不同状态，不互相推断。
5. 部分分配重试只提交失败项，已有 disabled 保持 disabled。
6. 409 模板修改/撤下需重新审阅；503 保留缓存和草稿。保存后的实例不随目录刷新改写。
7. 凭据仅在弹窗本地状态，保存/复用后清空，不进 URL、持久化 store 或安全摘要。

规范：.trellis/spec/views/frontend/mcp-market.md。当前有 16 个内置模板及部署模板，数量属于目录内容，不作为本任务固定验收值。

## 回归覆盖地图

| 范围 | 现有测试 |
| --- | --- |
| 导航 / 标签 / 命令搜索 | packages/core/paths/*.test.ts；packages/views/layout/app-sidebar.test.tsx；packages/views/search/search-command.test.tsx |
| 编辑器链接 | packages/views/editor/utils/link-handler.test.ts |
| Skill 目录 / 筛选 / 缓存 | packages/views/skills/components/skill-library-catalog.test.tsx；packages/views/skills/lib/skill-market.test.ts |
| Skill 创建 / 留页 / 焦点 / 恢复 | packages/views/skills/components/skills-page-template-session.test.tsx；packages/views/skills/components/create-skill-template-flow.test.tsx |
| Skill 持久化 | packages/core/skills/stores/view-store.test.ts |
| MCP 创建 / 分配 / 重试 | packages/views/mcp/mcp-market.test.tsx；packages/core/workspace/mcp-market-mutations.test.tsx |
| MCP 管理 / Agent 复用 | packages/views/settings/components/mcp-tab.test.tsx；packages/views/mcp/mcp-agent-discovery.test.tsx |
| MCP 缓存 / 轮询 | packages/core/workspace/mcp-template-queries.test.ts |
| Web 市场 / 创建 | e2e/skill-market.spec.ts；e2e/mcp-market.spec.ts；e2e/skill-template-creation.spec.ts |
| 旧入口 / 部署发布 | e2e/localized-template-defaults.spec.ts；e2e/workspace-defaults.spec.ts；e2e/admin-resource-publishing.spec.ts；e2e/fixtures/mcp-deployment.ts；e2e/missing-user-session.spec.ts |
| Desktop / Agent | e2e/mcp-desktop.spec.ts；e2e/agent-mcp.spec.ts |

## 结论与限制

复用现有后端足以支持本次入口和页面拆分。主要工程量在共享页面抽取、多端导航、生命周期、旧偏好清理及测试入口迁移。此次为源码和规范调研，没有浏览器实测；实施时需要验证运行时行为。

## 实施复核修正

`apps/desktop/src/renderer/src/components/tab-content.tsx:14` 明确每次仅挂载当前标签，非活跃标签只保存 URL/滚动状态。因此先前“桌面保活隐藏标签”的假设不适用于当前版本，切换标签会移除目录观察者。现有 CreateSkillDialog 仅保护自身关闭和相关资源跳转，没有全局导航拦截。本次保留跨工作空间/页面/桌面标签的原有隔离与临时弹窗关闭行为；市场内类型切换则保留面板、草稿与编辑器 DOM，隐藏 portal 并停止轮询。
