# 能力市场实施规划

状态：实施、验收和功能提交完成，正在归档；2026-10-17 恢复后的实际结果见 [verification.md](./verification.md)。

## 前提与工作边界

用户评审 PRD 和设计后，按 Trellis 工作流启动实施。保留其他任务未提交内容；调研起点已有 apps/web/next-env.d.ts 等外部改动。

本任务作为一个完整页面/导航交付，不拆成单独发布的子任务。Skill 和 MCP 抽取可由边界明确的实现代理并行完成，共享路径、sidebar、locale、store 由整合步骤统一负责。

## 执行顺序

### 1. 现有回归与新增契约

- [x] 恢复后运行受影响目录、创建会话、分配和导航回归，记录当前结果。实现前基线日志未留存，不补记为已通过。
- [x] 为新路由类型、两端入口、旧 libraryView 失效增加行为测试。
- [x] 复用现有领域测试，不在新页面重复纯 helper 的矩阵。
- [x] 确认无新 API、权限模型或依赖。

回退点：仅测试与任务记录。

### 2. 并行抽取领域面板

- [x] Skill：提取模板查询/筛选/卡片，复用复制编辑器和成功回调。职责范围 packages/views/skills/ 相关组件及测试。
- [x] MCP：复用目录，配接权限和自定义编辑，保持保存/分配弹窗生命周期。职责范围 packages/views/mcp/、settings/components/mcp-tab* 相关变更及测试。
- [x] 保护复制留页、焦点、脏草稿、MCP 仅保存及部分分配重试。
- [x] 原管理入口切换留待新市场可用后一起完成，避免中间断路。

两个实现代理不同时写共享接线文件；不新增第二套安装逻辑。回退点：保持原行为的组件抽取。

### 3. 共享页面及多端接线

依赖步骤 2。

- [x] 新增 packages/views/marketplace/ 及 package.json 子路径导出。
- [x] 新增 core 路径、WORKSPACE_PAGES 和图标映射。
- [x] 新增 Web 薄页面和 Desktop 工作空间会话路由。
- [x] 侧边栏增加能力市场，保留三个资源管理入口。
- [x] 补命令搜索、编辑器链接识别和路径一致性测试。
- [x] 实现 URL 类型状态、非法值回落、刷新及返回行为。
- [x] 限定活跃目录轮询，处理桌面隐藏标签。
- [x] 中英文文案、已有 token/Tabs 组件、窄容器布局。

### 4. 原管理页去除市场组合

依赖新市场完整可达。

- [x] 技能库去掉双 Tab/空库自动市场包装，保持自身滚动容器与虚拟器。
- [x] MCP 管理和 Settings 挂载去掉内嵌目录，保留管理流程。
- [x] 从模板创建/浏览/空态入口跳转对应市场类型。
- [x] 清理 libraryView 读写和旧持久化 merge，保留来源/分类。
- [x] 保留普通新建、导入、Agent 上下文入口和详情路径。
- [x] 删除失去用途的组合代码，避免双主入口。

步骤 3–4 作为完整交付评审；回退不改数据库、不删除用户资源。

### 5. 验证与文档

- [x] 旧 Tab 测试转为新入口契约，市场会话测试随页面所有者迁移。
- [x] 运行 TS、lint、测试与静态分析；945 文件 / 11,406 单测、lint/typecheck 通过，Knip 与基线相同，见 evidence/check-results.json。
- [x] 验证 production Web 和原生 Desktop 流程；最终原生完整流程无跳过通过，逐项结果与构建来源见 evidence/acceptance-final.md。
- [x] 更新部署发布、语言、默认工作空间及恢复登录测试中的旧入口。
- [x] 更新两份市场规范及用户文档，保留领域权限/恢复契约。
- [x] 独立 review 检查生命周期、授权、缓存/凭据及工作空间隔离；已修复上下文焦点和消费者测试夹具，见 evidence/source-review.md。
- [x] 按仓库规范记录文件、简化、证据及剩余风险；功能提交 581ba2ba5，验收记录与归档随后独立提交。

## 必须验证的行为

| 类别 | 最低覆盖 |
| --- | --- |
| 导航 | 侧边栏/命令搜索、两种 type、非法值、刷新/前进后退、Desktop 新标签与恢复 |
| 管理 | 原技能库/MCP/详情、空态 CTA、自定义新增/导入/管理 |
| Skill | 创建留页、打开副本、改名副本关联、普通创建及恢复差异、焦点、dirty/unknown-result |
| MCP | 仅保存、继续分配、部分失败、重试不重建、disabled 不被启用、Agent 上下文 |
| 权限 | 管理员/普通成员/Agent 所有者；不支持的运行时仍可浏览 |
| 失败恢复 | 首载、缓存刷新、已有资源未知；MCP 修改后重审/撤下禁存，Skill 撤下禁采用但已采用草稿继续可编辑且刷新不覆盖 |
| 隔离 | API 地址/工作空间/语言；切类型/工作空间/隐藏标签与轮询 |
| 偏好/凭据 | libraryView 退役，来源/分类保留，敏感输入不持久化 |
| 可访问性 | 中英文、360/375px 窄容器、键盘 tabs/预览、关闭焦点恢复 |

## 建议验证命令

以下为迁移后的命令。实际执行次数、退出码与分包复验见 verification.md 及独立审查/验收记录。

```bash
pnpm --filter @multica/core exec vitest run paths skills/stores/view-store.test.ts workspace/mcp-market-mutations.test.tsx workspace/mcp-template-queries.test.ts
pnpm --filter @multica/views exec vitest run marketplace skills/components/skill-template-catalog.test.tsx skills/components/skill-market-panel.test.tsx mcp settings/components/mcp-tab.test.tsx layout/app-sidebar.test.tsx search/search-command.test.tsx editor/utils/link-handler.test.ts
pnpm lint
pnpm typecheck
pnpm test
pnpm knip
```

新页面测试位于 marketplace，领域会话测试随对应面板所有者迁移。E2E 使用隔离的 production Web 服务，按 .trellis/spec/web/frontend/e2e-run-environment.md 准备，不能以 next dev 冷编译失败作为验收证据。

```bash
pnpm exec playwright test e2e/capability-market.spec.ts e2e/skill-market.spec.ts e2e/mcp-market.spec.ts e2e/skill-template-creation.spec.ts e2e/localized-template-defaults.spec.ts e2e/workspace-defaults.spec.ts
pnpm exec playwright test e2e/mcp-desktop.spec.ts e2e/agent-mcp.spec.ts e2e/admin-resource-publishing.spec.ts e2e/missing-user-session.spec.ts
```

Desktop 和部署发布按既有夹具准备环境。无法执行的项目明确记录，mock 不代替真实发布验收。后端预期无变化；如范围调整触及后端，先重新评审并补 Go 测试/静态检查。

## 完成条件

满足 PRD AC1–AC10，记录实际通过证据及未覆盖项，没有遗漏两端路由、旧入口、locale 或消费者测试。实施及最终验收完成，PRD 标准均已回填；提交归档见最终任务记录。
