# 当前功能与使用文档覆盖核查

核查日期：2026-11-07。基准为 `codex/cross-workspace-collaboration` 当前工作区（开始时 HEAD 为 `e54d09be4`，包含既有未提交实现），不是截图安装包的发布清单。核查结合页面、路由、中文标签、服务端权限与已有测试；没有运行真实 AI 或逐项执行全部业务流程。

## 核查结论

主要缺口是账号级协作、上下文与知识、MCP 配置到分配、统计和日常任务管理操作。项目、迭代、分拣台、自动化和管理员指南已覆盖主要流程，本次通过功能总览统一导览。新增五页并修正旧页后，用户可以从内置目录找到下表各主功能模块。

文档源在 `apps/docs/content/docs/`，由 `scripts/generate-docs-bundle.mjs` 生成 `server/internal/docs/content/`。帮助菜单读取当前服务器的文档；这些改动需要随服务器更新后才能出现在现有部署中，更新桌面客户端本身不会替换该文档包。

## 功能覆盖矩阵

| 模块 | 代码核对入口 | 文档去向与本次处理 |
| --- | --- | --- |
| 帮助及版本 | `packages/views/layout/help-launcher.tsx`、`packages/views/docs/` | 重写欢迎页，新增 `feature-overview`；说明客户端与服务器版本差异 |
| 登录与账号 | `server/internal/handler/auth_device.go`、`auth_password.go`、`packages/views/settings/components/account-tab.tsx` | 已有 `auth-setup`、`auth-tokens`；总览补个人设置入口 |
| 工作空间、成员 | `packages/views/settings/components/settings-nav.ts`、`server/internal/handler/workspace.go` | `workspaces` 修正显式共享及保留协作记录时不可删除；`members-roles` 保留 |
| 任务及我的任务 | `packages/views/issues/components/save-view-dialog.tsx`、`batch-action-toolbar.tsx`、`table-view.tsx`、`server/internal/handler/issue_view.go` | `issues` 补保存视图、共享/私有、批量处理、表格 CSV 导出 |
| 快捷操作 | `packages/views/settings/components/quick-actions-tab.tsx`、`server/internal/handler/quick_action.go` | `issues` 补保存提示词、个人/共享权限、任务侧栏触发与评论框插入 |
| 属性与分类 | `packages/views/settings/components/properties-tab.tsx`、`server/internal/handler/property.go` | `issues` 补自定义属性入口、类型、权限和归档 |
| 评论与子任务 | `packages/views/issues/components/comment-card.tsx`、`server/internal/handler/source_context.go` | `comments`、`issues` 补评论转子任务、来源快照和变化提示 |
| 项目与资源 | `packages/views/projects/components/project-detail.tsx`、`project-overview.tsx`、`project-resources-section.tsx` | 已有 `projects`、`project-resources`；总览归档入口，知识页补贡献位置 |
| 分拣台 | `packages/views/triage/` | `triage` 已覆盖启用、审核、接纳与执行、批量和 CSV 导入 |
| 迭代 | `packages/views/iterations/`、`packages/core/iterations/` | `iterations` 已覆盖规划、预览/确认、统计、结束结转与恢复 |
| 智能体与角色 | `packages/views/agents/`、`server/internal/handler/agent.go` | `agents`、`agents-create` 修正 MCP 凭据展示规则并补入口 |
| 人工审批 | `packages/views/agents/approvals/`、`server/internal/handler/agent_approval.go` | `agents` 补审批人权限、步骤、撤回和上报结果边界 |
| AI小队 | `server/internal/handler/squad_briefing.go`、`server/cmd/multica/cmd_comment_trigger_outcomes.go` | `squads` 补逐目标派发回执、部分失败、受限角色回流例外 |
| 普通执行与触发 | `server/internal/service/issue_trigger.go`、`server/internal/handler/comment.go` | `triggering-agents`、`assigning-issues`、`mentioning-agents`、`tasks` 保留入口及状态说明，统一用户术语并区分协作流程 |
| 对话 | `packages/views/chat/components/new-chat-button.tsx`、`packages/views/modals/quick-create-actor-picker.tsx` | `chat` 补实际入口及队长选择边界；仍是单智能体对话，不宣称 AI小队群聊 |
| 收件箱 | `packages/views/issues/hooks/use-issue-subscribers.ts`、`packages/views/issues/components/issue-detail.tsx` | `inbox` 补取消订阅的任务/子任务范围，并区分协作通知 |
| 协作任务 | `packages/views/collaboration/`、`server/internal/collaboration/access.go` | 新增 `collaboration`：创建、转交、补充信息、附件、结果验收、权限和请求恢复 |
| 协作 AI | `packages/views/collaboration/resource-panel.tsx`、`execution-panel.tsx`、`comment-composer.tsx` | `collaboration` 补资源提供/采纳/授权、显式恢复、触发预览、停止和配置切换 |
| 上下文与知识 | `server/internal/service/collaboration_memory.go`、`collaboration_knowledge.go`、`collaboration_knowledge_runtime.go` | 新增 `collaboration-knowledge`；环境变量页补默认关闭的知识使用开关 |
| Skill 与市场 | `packages/views/skills/`、`packages/views/marketplace/marketplace-page.tsx` | `skills` 补市场入口；原有导入、编辑、模板及绑定说明保留 |
| MCP 与市场 | `packages/views/mcp/`、`server/internal/handler/workspace_mcp_api.go` | 新增 `mcp`：模板/自定义、保存/显式分配、权限、凭据、同名覆盖和失败处理 |
| 运行时与工具 | `packages/views/runtimes/`、`packages/core/agents/mcp-support.ts` | 已有 `daemon-runtimes`、`install-agent-runtime`、`providers`；补 MCP 指南互链 |
| 统计 | `packages/views/dashboard/components/dashboard-page.tsx`、`server/internal/handler/dashboard.go` | 新增 `usage`：用量/错误、筛选、时区、费用和数据边界 |
| 自动化 | `packages/views/autopilots/`、`server/internal/handler/autopilot.go` | `autopilots` 已覆盖定时、Webhook、报告、投递记录与权限 |
| 仓库与集成 | `packages/views/settings/components/repositories-tab.tsx`、`vcs-tab.tsx` | 已有 `vcs-integration`；总览强调实际服务可达性 |
| 桌面与 CLI | `apps/desktop/src/renderer/src/platform/account-workbench-route.tsx`、`server/cmd/multica/` | `desktop-app` 补独立账号协作标签页；已有 `cli` 操作参考 |
| 部署与平台管理 | `packages/views/admin/`、`server/cmd/server/router.go` | 已有 `platform-administration`、`self-host-quickstart`、`security-model`、`environment-variables` 和 `troubleshooting` |
| 可选插件与计费 | `packages/views/settings/components/settings-nav.ts` | 总览只说明受开关和配置控制；不把可选入口描述为默认启用能力 |

## 修正的关键误解

- 工作空间保留数据归属与成员边界，但可以显式共享单条协作任务；参与者不因此成为来源空间成员。
- MCP 配置保存、分配和执行环境就绪是不同步骤；工作空间配置库只写不可回显，智能体自有 MCP 与环境变量的读取机制不同。
- 派活评论保存、触发被受理、执行成功和成果验收是不同状态。
- 批准高风险操作只产生授权记录，不会由审批页自动执行命令；执行上报也不自动证明发布成功。
- 知识确认不等于运行已使用；需要启用配置，并满足工具、运行模式、来源权限及有效期条件。
- “停止请求已受理”不代表本机已终止；工作目录隔离不等于系统沙箱。

## 范围与限制

- 这是主功能入口覆盖与重点语义校对，不是所有 API 参数、所有权限组合或全平台业务回归测试。
- 不把在研的受控发布候选制品执行方案写成当前已支持功能。
- 公共英文文档、移动端专用手册、历史更新记录和全部旧截图不在本次同步范围。
- 本次不部署服务器或发布桌面安装包；用户现有部署是否具备这些工作区能力，需要按实际服务器版本确认。

## 验证记录

- 独立语义审核完成：协作待审父任务、统计终态计数、触发合并、快捷操作、聊天入口和 Defender 告警措辞等发现全部关闭。
- `node scripts/generate-docs-bundle.mjs`：44 页、58 个资源，新增 5 页。
- 临时输出逐字节比较：103 个文件一致；347 处内部链接、57 处 Markdown 图片引用均可解析，无缺页、缺锚点或缺图。
- `pnpm --filter @multica/docs test`：9 个测试文件、62 项测试通过；生成后重跑深链校验 4 项通过。
- `pnpm --filter @multica/docs typecheck`：通过；`pnpm --filter @multica/docs build`：通过，生成 104 个静态页面。
- `go test ./internal/docs/... -count=1 -v`：通过，含嵌入资源、访问白名单和 CLI 不嵌入文档包的检查。
- `go test ./internal/handler/ -run Docs -count=1 -v`：12 项文档接口测试全部实际运行并通过，覆盖目录中每个页面/资源、缓存和未发布路径。
- Handler 测试仅使用本任务新建的隔离数据库 `multica_docs_audit_20261107`。从已核实为空且无其他连接的模板克隆，再应用当前第 753 条迁移记录；不在既有开发库运行测试。
- `git diff --check`：通过。文档包没有独立 lint 脚本，以构建、类型、链接及空白检查验证本次纯文档修改。

以上证明文档可编译、可发布、链接完整且与内置内容一致，不等同于全部业务流程或实际 AI 执行的端到端验收。
