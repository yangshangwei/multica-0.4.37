# 工作管理与导航文档核查

核查日期：2026-11-07。依据当前工作区源码（包含尚未提交的实现），不是仅依据最近提交或截图版本。只读核查产品代码；本报告不代表运行时端到端验收。

## 优先补充

### 1. 任务页缺少保存视图、批量处理和 CSV 导出操作

现有 `apps/docs/content/docs/issues.zh.mdx:105` 只有一段五种视图介绍，未说明已实现的日常管理能力。

证据：

- `packages/views/issues/components/save-view-dialog.tsx:91` 定义工作空间、我的任务和项目范围；`:574` 强制我的任务视图私有；`:723` 提供仅自己/工作空间成员可见选项。
- `packages/views/locales/zh-Hans/issues.json:781` 给出实际入口名称“保存为视图”，`:834` 的视图菜单有“新增视图”“管理视图”“固定到侧栏”。管理视图排序和显隐只影响个人。
- `server/internal/handler/issue_view.go:339`：创建者可管理自己的视图；工作空间所有者/管理员可管理共享视图，不能因此读取其他人的私有视图。
- `packages/views/issues/components/batch-action-toolbar.tsx:238` 起提供状态、优先级、负责人、迭代和删除；`:116` 拒绝含协作任务的批量普通编辑。
- `packages/views/issues/components/table-view.tsx:2450` 为表格提供“导出全部”“导出已选择”；`:2327` 使用当前表格列；`:2409` 生成带 UTF-8 BOM 的 CSV。
- `packages/views/issues/surface/use-issue-surface-controller.ts:806` 按当前 `tableQuerySpec` 分页获取全部匹配任务，并检查完整性。“全部”不能写成工作空间所有任务或仅已加载行。

拟改说明：在任务页加入“保存常用视图”“批量处理”“从表格导出”三节；说明作用范围、可见性、管理权限、个人显隐与共享定义的区别，以及导出遵循当前查询和列配置。含协作任务的批量普通编辑应改为逐条打开协作详情处理。

### 2. 评论创建子任务及来源快照完全缺失

现有任务页只介绍父子任务及阶段；评论页未说明评论菜单中的创建子任务。

证据：

- `packages/views/issues/components/comment-card.tsx:688`、`:1016` 的菜单项：“从这条评论创建子任务”。
- `packages/views/issues/components/issue-detail.tsx:1415` 把评论 ID 交给创建弹窗；`:2746` 受 `executionAllowed` 控制；`:3135` 展示捕获上下文。
- `packages/views/modals/create-issue-dialog.tsx:79` 先取来源预览，手动与智能体创建共享该预览。
- `server/internal/handler/source_context.go:464` 创建时验证 capture token，`:505` 再比较来源摘要，变更后需要刷新重试；`:749` 将原任务设为父任务；`:879` 为智能体模式创建一次快速建任务执行。
- `server/internal/service/source_context.go:26` 定义普通捕获上限：256 条评论、1 MiB 文本、100 个附件、附件合计 500 MiB；智能体模式另有较小上下文预算（64 KiB snapshot / 72 KiB input），不能把普通模式上限说成所有模式统一上限。
- `server/internal/service/source_context.go:272` 拒绝协作任务作为普通来源上下文；不要声称该流程适用于所有协作详情。
- `packages/views/issues/components/source-context-viewer.tsx`、`server/internal/handler/source_context.go:151` 起比较来源标题、描述、附件和线程变化；已捕获快照与实时来源分开显示。

拟改说明：普通任务评论更多菜单 → 从这条评论创建子任务 → 检查原任务描述、评论线程及附件预览 → 手动填写或交给智能体创建。提交前来源变化需刷新；创建后快照保留当时内容，原文修改不覆盖快照，可查看差异或回到来源。限额可用“超出提示时缩小材料或使用手动模式”描述，避免堆砌底层预算。

### 3. 自定义属性只有名词，没有配置和使用说明

证据：

- `server/internal/handler/property.go:27`：属性定义只由人类所有者/管理员维护；成员和智能体可以填写值；归档保留已有值并拒绝新值。
- `server/internal/handler/property.go:41`：最多 20 个活跃定义；`:52` 支持文本、数字、单选、多选、日期、复选框、URL、单个参与者和多个参与者。
- `packages/views/settings/components/properties-tab.tsx:126` 使用成员角色控制管理；已有创建、编辑、归档和恢复界面。
- `packages/views/issues/components/save-view-dialog.tsx:168`：单选可用于分组；数字和日期可用于排序。不要概括为所有属性都能分组/排序。

拟改说明：任务页增加“标签和自定义属性”：管理员在设置维护字段，成员在任务详情/表格填写；已归档字段的旧值仍保留。说明字段可用于筛选和显示，具体排序/分组以控件提供的类型为准。

### 4. 工作空间及桌面导航说明未覆盖账号协作范围

`workspaces.zh.mdx:8` 写“最上层边界”“彼此完全隔离”；`desktop-app.zh.mdx:47` 的标签页说明全部基于工作空间。现在需要为显式授权的账号协作任务补充例外，不能让用户误以为跨工作空间协作不存在或进入协作必须先加入一个工作空间。

证据：

- `packages/views/layout/global-scope-navigation.tsx:28` 两个入口“工作空间”和“协作”。
- `apps/web/platform/scope-shell.tsx:48`、`apps/desktop/src/renderer/src/components/account-navigation.tsx:19` 接入相同全局切换；切换恢复各自上次位置。
- `packages/core/paths/account.ts:1`：账号路由独立于工作空间成员身份；`:2` 有当前处理和我参与的两个列表范围；`:70` 使用同一账号标签页但不同任务/列表各自保存状态。
- `apps/desktop/src/renderer/src/routes.tsx:121` `/account` 是持久全局标签；`apps/desktop/src/renderer/src/platform/account-workbench-route.tsx:18` 只检查登录及账号/密码设置，不要求 workspace membership。
- `packages/views/collaboration/account-workbench-page.tsx:45` 实际列表标签，`:104` 搜索按已加载列表进行；未加载完时有 partialSearch 提示，不宜描述为全服全文搜索。

拟改说明：工作空间仍隔离普通任务/配置与成员权限；需要跨账号协作时通过独立协作入口及显式任务权限共享，不等于加入来源工作空间或获得整个空间读取权限。桌面文档加“工作空间与协作切换”，说明独立账号标签页及恢复位置，并链接协作专题。

### 5. 收件箱缺少对子任务整体取消订阅的说明

证据：`packages/views/issues/components/issue-detail.tsx:3430` 起，当任务有子任务时取消订阅菜单提供“仅此任务”和“此任务及其子任务”；代码注释明确 subtree opt-out 也抑制之后新建子任务的自动订阅。`packages/views/issues/hooks/use-issue-subscribers.ts:191` 调用并展示不支持/失败反馈。现有 `inbox.zh.mdx:34` 只说“随时订阅或取消”。

拟改说明：在订阅节补充两个范围，避免用户反复取消单个子任务；直接提及仍按原通知规则处理。若旧服务端不支持，界面提示需升级。

## 已有较完整覆盖，暂不建议重写

- `projects.zh.mdx`：正式范围统计、取消计数、健康、目标模板与并发编辑、进展与验收版本、项目删除影响、默认 AI小队及自动化已有说明。
- `iterations.zh.mdx`：入口启用、权限、手动生命周期、预览确认、开始范围、统计口径、结束结转、取消和禁用已详细覆盖。
- `triage.zh.mdx`：单条/批量审核、普通接受与接受并执行区别、执行失败重试、CSV 预览/导入/失败恢复、迭代接纳已有说明。

这些页面主要需要统一术语和与新任务/协作说明互链，不应在无证据时推断新功能。

## 术语问题

任务页标题仍为 `Issues`，正文大量 `issue`、`小队` 和生硬英文状态；工作空间、项目、收件箱也重复这些旧词。`developers/conventions.zh.mdx` 明确用户产品名词采用“任务”“AI小队”，状态界面用“待规划/待办/进行中/审核中/已完成/已阻塞/已取消”，API、命令和标识符仍保留英文。建议按编辑范围修正文案，保留 `/issues` 路由和命令。
