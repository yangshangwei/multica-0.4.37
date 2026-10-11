# 桌面核心页面审计修复结果

已按原审计的 F01–F10 完成修复，保留现有产品布局、原有未提交修改和数据边界。本轮没有创建 Trellis 任务。

| 问题 | 修复结果 | 主要证据 |
|---|---|---|
| F01 任务失败误报删除 | 区分临时失败、404 和权限丢失；保留授权缓存与编辑器，支持重试 | Electron 首次/缓存 500 与重试；canonical/detail/route 回归 |
| F02 缩放后搜索不可恢复 | 450/720 CSS px 均保留搜索、清除与无结果重置；清除后恢复焦点 | 200% 原生缩放截图、键盘与命中位置检查 |
| F03 项目菜单焦点不可见 | Tab 聚焦后菜单按钮完整显示并带焦点环 | 实际 Tab 遍历，opacity=1 |
| F04 控件缺少名称 | 项目详情/紧凑栏、收件箱批量菜单、任务批量清除及属性抽屉均有本地化名称 | DOM 语义测试与 Electron AX/可访问名称断言 |
| F05 状态文字对比度不足 | 状态与优先级使用语义文字色；在线文字增加独立 success foreground | 项目文字 5.35:1–8.57:1；在线文字浅色 5.95:1、深色 8.92:1 |
| F06 排序只能拖动 | 列表、看板、泳道增加上移/下移；遵守分组、排序方向、分页与固定泳道边界 | 100 行实际虚拟列表边缘移动、焦点保留；锚点、回滚与播报回归 |
| F07 紧凑范围与保存视图不一致 | 紧凑选择器复用宽版操作；保存视图可键盘选择，内置范围会退出保存视图 | Electron 任务/我的任务两个页面的 200% 缩放键盘路径 |
| F08 分拣失败丢失缓存 | 队列、历史、设置失败保留内容并提供重试；权限失效保护优先 | 实际队列节点/选择保留、历史重试；真实 QueryClient 回归 |
| F09 聊天错误伪装为空 | 页面与浮窗区分加载、空、失败与刷新失败；保留会话/消息/草稿并支持重试 | Electron 冷/热缓存失败与草稿保留；会话和消息真实 QueryClient 回归 |
| F10 批量删除提前清空 | 等待全部结果、禁止重复提交、只清除成功项选择并汇总失败 | 实际确认框等待状态和部分失败选择保留；隔离 mutation 夹具与组件回归 |

## 验证

去重后的定向测试共 **26 个文件、475 项通过**：

- 项目列表/详情与英中文 locale parity：3 文件、103 项。
- 聊天页面/浮窗/控制器及收件箱：4 文件、73 项。
- 任务 canonical/detail/route 与分拣：5 文件、136 项。
- 列表/看板/泳道、排序、紧凑视图、批量操作及 surface：12 文件、154 项。
- 桌面标签栏和布局：2 文件、9 项。

`core`、`views`、桌面 main/preload 与 renderer 类型检查通过。定向 ESLint 无错误；项目详情两条、泳道一条既有 Hook 依赖警告保留。`git diff --check` 通过。

Impeccable detector 本轮最终扫描返回 `[]`。视觉检查保留原有界面风格，无需额外视觉返工；最终 visual-verdict 为 96/100、PASS，共保存 28 张本轮原始截图。对比度测量排除屏幕阅读器专用隐藏文字；主题切换和抽屉截图在动画稳定后确认。

## 变更范围与复用

- `packages/core/issues/canonical-id.ts`：错误分类；`packages/core/projects/config.ts`：状态/优先级文字配色。
- `packages/views/issues/`：任务详情、列表/看板/泳道排序、紧凑视图与批量控件。
- `packages/views/projects/`：搜索、焦点、控件名称与批量删除；未改动本次开始前已有的项目概览/进展实现。
- `packages/views/chat/`、`triage/`、`inbox/`、`my-issues/`、`agents/presence.ts`：恢复行为、范围切换与无障碍/文字对比度。
- `packages/ui/styles/tokens.css`、英中文 locale、对应回归测试与桌面交互规范。

复用现有 React Query、mutation、排序锚点、Base UI 和语义配色。共享排序按钮、紧凑选择器与聊天恢复提示，未引入依赖、后端修改或新的服务数据状态层。

## 证据与验证边界

- [结构化 Electron 结果](remediation-native.json)
- [视觉前后对比](screenshots/remediation-comparison.jpg)
- [修复截图总览](screenshots/remediation-contact.jpg)
- [最终确认截图](screenshots/remediation-confirmation.jpg)
- [Detector 结果](remediation-detector.json)
- [原始审计](report.md)

运行环境为 macOS Electron 39.8.7、当前工作区 Vite renderer、隔离临时 profile。真实 API 写操作保持拦截；只放行任务表 rows/facets/groups 的只读 POST。排序成功和删除部分成功通过 renderer mutation 夹具验证，保留数据未被实际修改；失败请求通过网络层注入，缓存/编辑器断言使用实际 React Query 与 DOM。

本轮没有覆盖 VoiceOver、Windows/Linux、生产构建性能或真实服务写入的端到端验收。上述限制不影响本次已验证的前端修复结论，也不将结果等同于完整 WCAG 认证。
