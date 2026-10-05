# 管理后台列表与详情优化验收

本次承接已确认的建议，优化审计、终端、执行详情、账号/工作空间列表和筛选区。保持后台局部样式边界，未新增依赖或改动 API、数据库、桌面路由及共享主题。

## 完成内容

- 审计：已知事件、结果、阶段显示中英文说明；原始代码、完整 ID 和快照保留在详情；未知值原样回退。
- 终端：已有名称优先，缺少名称时仅缩短 UUID 的视觉展示；新增完整 ID 展开/复制及剪贴板失败手动回退。状态与新鲜度分别表达，过期状态保持中性色。
- 执行详情：突出任务标题和执行状态，时间与用量分区，技术标识可展开；权限、控制操作、关联跳转和时区保留。
- 账号/工作空间：手机列表更紧凑；账号补齐重置和刷新，保留管理员目录及分页条件。
- 筛选：复用 AdminFilterSummary，只展示提交到 URL 的生效条件，排除草稿、隐式默认值和游标。

## 检查结果

- 后台视图与中英文语言一致性：167 项测试通过。
- 桌面端：957 项测试通过，Node/renderer 类型检查和独立目录生产构建通过。
- Web 类型检查、views 产品源码类型检查通过。
- 最终全仓 `pnpm typecheck`：**9/9 项任务通过**。并行 MCP 测试错误已由对应工作修正；本次未编辑该文件。
- 后台 ESLint 与静态设计检查通过。
- 400 个桌面 CSS/JS 构建文件未包含后台专属样式标记。
- 独立复核确认权限、回执、状态、时间、查询参数与复制行为保留；发现的长未知审计代码换行问题已修复。

## 浏览器验收

- **17 个路由、89 个页面状态、25 次深色检查**，含 16 个英文手机状态。
- **14 项原回归与 16 项新增功能探针通过**，新增 93 条断言通过。
- 主体溢出、手机可用控件不足 44px、对比度问题、页面 JS 错误及管理写请求均为 0。
- 长未知审计码、原始码/对象属性名回退、完整 ID 复制/失败回退、技术详情、账号重置/刷新及生效筛选全部通过。剪贴板使用浏览器内 stub，没有操作系统剪贴板。
- Next 开发服务器的自动内存重启曾中断一次检查；新建隔离浏览器 context 后恢复并完成验收，没有手动更改服务。

[前后对比](../../.omx/reports/admin-detail-refinement-2026-10-04/gallery.html) · [全部浏览器证据](../../.omx/reports/admin-detail-refinement-2026-10-04/browser/README.md)

## 改动文件

- `packages/views/admin/admin-visual.module.css`
- `packages/views/admin/admin-visual.module.css.d.ts`
- `packages/views/admin/audit/audit-code-labels.ts`
- `packages/views/admin/audit/audit-page.tsx`
- `packages/views/admin/executions/execution-detail.test.tsx`
- `packages/views/admin/executions/execution-detail.tsx`
- `packages/views/admin/executions/list-controls.tsx`
- `packages/views/admin/filter-summary.test.tsx`
- `packages/views/admin/filter-summary.tsx`
- `packages/views/admin/installations/installation-common.test.tsx`
- `packages/views/admin/installations/installation-common.tsx`
- `packages/views/admin/installations/installation-detail-page.tsx`
- `packages/views/admin/installations/installation-identity.tsx`
- `packages/views/admin/installations/installations-page.test.tsx`
- `packages/views/admin/installations/installations-page.tsx`
- `packages/views/admin/observability/common.tsx`
- `packages/views/admin/observability/read-pages.test.tsx`
- `packages/views/admin/users/users-page.test.tsx`
- `packages/views/admin/users/users-page.tsx`
- `packages/views/admin/workspaces/workspaces-page.tsx`
- `packages/views/locales/en/admin.json`
- `packages/views/locales/zh-Hans/admin.json`

## 边界

不把 Web 截图作为原生 Electron 视觉验收。本次没有重启原生桌面进程。后台页面使用本地测试数据；部分状态使用标明的浏览器临时样例，未写入产品数据。

源文件基线存在其它并行工作：受保护范围内的 `packages/views/settings/components/mcp-tab.test.tsx` 由其它任务更新，本次未编辑它。验收资料保留基线、增量补丁及文件指纹，避免把此前改动归入本次。

方案：[实施计划](2026-10-04-admin-detail-refinement.md)。证据：`.omx/reports/admin-detail-refinement-2026-10-04/`。

代码仍在工作区，未提交或发布。
