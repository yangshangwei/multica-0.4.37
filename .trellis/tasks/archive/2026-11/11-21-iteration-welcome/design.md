# 技术设计

## 边界

实现一个共享的 `IterationWelcome` 组件，并在 `IterationOverview` 的成功空目录分支中挂载。组件只负责展示和触发现有创建对话框，不拥有迭代数据，也不改变 Query 或导航契约。

预期修改：

- `packages/views/iterations/iteration-welcome.tsx`：欢迎页结构、静态示例和创建回调。
- `packages/views/iterations/iteration-welcome.module.css`：容器查询、示例卡片和时间线视觉细节，复用运行时令牌。
- `packages/views/iterations/iteration-overview.tsx`：区分首次空目录与筛选后空结果，并把创建动作传给欢迎页。
- `packages/views/locales/en/projects.json`、`packages/views/locales/zh-Hans/projects.json`：欢迎页文案。
- `packages/views/iterations/iteration-navigation.test.tsx`、必要时新增组件测试：首次空目录、点击创建、筛选空结果和刷新边界。

## 空状态判定

`catalogue.data` 是完整迭代目录。首次欢迎只由 `catalogue.data.length === 0`、`activeTab === "open"` 和 `enabled` 共同决定；`catalogue.isPending`、`catalogue.error`、有筛选条件或任何目录项存在时走现有列表状态。该集合只能证明当前没有保存的迭代，不能证明过去没有创建后删除的迭代；这样历史迭代存在但“进行中”为空时不会误判首次使用。

创建按钮调用当前 `setCreateVisited(true); setCreateOpen(true)`，由父级继续渲染现有 `Dialog` 与 `IterationForm`；实现可使用共享 `DialogTrigger` 保持焦点回收。欢迎页显示设置入口时，列表头部不再重复显示创建或设置入口。

## 视觉与交互

欢迎页采用与智能体/小队一致的左右分栏：左侧价值说明、主 CTA 与三步流程；右侧为标注“示例”的静态迭代卡片。示例展示时间段、任务行、进度条和结束交接，内容全部来自本地化文本。窄屏下分栏堆叠，示例卡片位于 CTA 下方；不引入轮播以缩短交互路径。

使用 `section`、标题层级和按钮的原生语义；示例不包含交互控件。主 CTA 保持 44px 粗指针高度。CSS 模块只使用现有 `--radius`、语义色和类型令牌，并尊重 reduced-motion。

## 兼容与回退

不新增 API 或持久状态，不影响 Web/Desktop 平台路由。目录请求错误继续由现有重试 UI 展示。历史迭代、筛选无结果和禁用状态继续保留既有行为。回滚时移除欢迎组件与对应两种语言文案即可。

## 验证

- 用视图集成测试覆盖成功首次空目录、创建动作、已有历史/筛选结果、功能禁用、pending/error。
- 保留现有单个空计划迭代任务动作测试。
- 运行迭代导航聚焦测试、views lint 与 typecheck；必要时做 Web/桌面截图确认布局和粗指针目标。
