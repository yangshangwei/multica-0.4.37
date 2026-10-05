# 管理后台视觉改造验收

本次按参考图完成局部视觉改造：灰青背景、白色内容面板、四项核心指标、统一导航选中状态及手机布局。原有业务数据、权限、筛选、操作表单和跳转语义保留。未新增依赖，未提交或发布。

## 改造范围

| 文件 | 用途 |
| --- | --- |
| `packages/views/admin/admin-shell.tsx` | 后台外壳与导航、移动抽屉主题、非总览页面容器 |
| `packages/views/admin/admin-visual.module.css` | 仅后台引用的局部主题、面板与响应式布局 |
| `packages/views/admin/admin-visual.module.css.d.ts` | 仅此 CSS Module 的类型声明 |
| `packages/views/admin/overview/overview-page.tsx` | 核心指标与业务模块分组、刷新入口 |
| `packages/views/admin/overview/overview-page.test.tsx` | 指标可访问描述、未知与零、刷新状态回归 |

方案见 [实施方案](2026-10-04-admin-visual-refresh.md)。组件规范补充在
`.trellis/spec/views/frontend/component-guidelines.md`。复用原有 Token、按钮、表格、筛选器和跳转构造函数；局部面板与指标组件减少重复布局。

## 桌面端保护依据

- Web 通过独立的 `@multica/views/admin` 出口挂载后台；Electron 不挂载这些组件。
- Electron 的 Tailwind 会扫描后台 TSX，因此没有更改全局主题、通用组件或加入全局 CSS 选择器。后台变量只作用于后台根节点及其移动抽屉。
- `apps/desktop`、`packages/ui`、`packages/core`、共享 layout/settings 共 **996 个受保护源文件哈希一致**。
- 桌面完整 Vitest：**84 个测试文件、957 项测试通过**。
- 桌面 Node/renderer 类型检查通过；主进程、preload、renderer 的生产构建通过。产物输出到独立验收目录，没有覆盖正在运行的桌面构建。
- 398 个桌面 CSS/JS 构建文件均不包含后台专属 CSS 标记。
- 全项目 `pnpm typecheck`：**9/9 项任务通过**。

## 后台验证

- 变更前 83 项后台视图测试通过；变更后 views/admin + 语言一致性 **148 项通过**，core/admin **142 项通过**。
- 本次源文件 ESLint、`git diff --check` 通过；Impeccable 静态检测为空。
- 5 个代表页面覆盖中文、英文、宽屏、手机，共 **20 份页面测量**，另有深色及导航抽屉截图：主体溢出 0、可见文本对比度问题 0、手机不足 44px 的可用控件 0。
- 有数据的核心指标以浏览器临时样例验证，覆盖 320px/720px、深浅色。无溢出或过小目标；四项彩色指标的文本对比度均通过。
- 完整页面矩阵覆盖 **17 个路由、63 个状态**（含详情、展开审计、空状态与深色抽样）：页面异常、主体溢出、对比度问题、手机过小目标均为 0。
- **14/14 项浏览器交互检查通过**：含原有 13 项审计回归与后台离开后的样式隔离检查。进入 `/login` 后，后台根节点不存在，body/html 的后台变量为空；管理写请求为 0。
- 独立审查确认原有指标、查询参数、权限、焦点与未知值语义保留；首轮背景色偏粉的问题已修正为 OKLab 混色，复核通过。

## 验证边界

这是本地开发环境验收。部分执行、告警、操作回执及有数据的核心指标使用明确标注的浏览器临时样例；产品源码没有加入演示数据。没有执行禁用账号、恢复密码或取消真实执行。

**未进行原生 Electron 窗口视觉自动化**：当前 CUA 不可用，已有 Electron 未开放调试端口。桌面保护结论依据源文件隔离、构建产物、类型检查和测试；不把 Web 截图当作原生桌面验收。运行中的桌面进程和用户配置未因本次任务重启或修改。

pnpm 的既有配置迁移警告仍存在，已通过的检查退出码均为 0。

## 查看结果

- [本地管理后台](http://localhost:13493/admin)
- [前后对比与深浅色截图](../../.omx/reports/admin-visual-refresh-2026-10-04/gallery.html)
- [桌面边界与验证证据](../../.omx/reports/admin-visual-refresh-2026-10-04/desktop-boundary.md)
- [页面测量汇总](../../.omx/reports/admin-visual-refresh-2026-10-04/visual-summary.json)

- [完整页面矩阵和交互结果](../../.omx/reports/admin-visual-refresh-2026-10-04/matrix/summary.json)
