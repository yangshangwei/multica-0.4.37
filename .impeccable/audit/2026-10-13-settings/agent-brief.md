# 设置页面静态审计：子任务说明

仓库：/Volumes/artisan/code/2026/multica-0.4.37（Multica，面向小团队的 AI 任务管理平台；web/desktop 共享 `packages/views` 与 `packages/ui`）。
本次是 impeccable `audit`（技术质量审计，Operate 工作界面）。对象是「设置」菜单下的页面。

## 硬约束

- 只读。不要修改、创建或删除任何文件（本说明文件除外，也不要改它）。不要运行 git 写操作、pnpm install、make、测试或任何启动服务的命令。
- 只读你分到的文件，以及为了确认行为而必须追踪的直接依赖（共享组件、hook、locale JSON）。
- 不确定的问题标为「待运行验证」，写清需要什么运行证据确认（例如：打开某 tab、注入某请求 500、Tab 键遍历）。不要把猜测写成事实。

## 共享框架（先读）

- `packages/views/settings/components/settings-layout.tsx`：标准 `SettingsTab` / `SettingsSection` / `SettingsCard` / `SettingsRow` / `SettingsSaveState`。
- `packages/views/settings/components/settings-page.tsx`：Tabs 外壳、目录、内容宽度。
- `packages/ui/styles/tokens.css`：语义颜色（oklch）和角色字号 `--text-*`（`text-caption`、`text-body`、`text-title` 等）。浅色 `--page-canvas` 约 0.988，不是纯白。
- `.trellis/spec/views/frontend/desktop-core-accessibility.md`：上一轮修复沉淀的交互规范，可作为对照。
- 仓库规则见根目录 `CLAUDE.md`（API JSON 须经 zod `parseWithFallback`；服务端字段用 `=== true`；服务端枚举 switch 要有 default；语义 token，不用硬编码颜色；字号用 `--text-*`）。

## 检查维度

1. 可访问性：图标按钮缺可访问名称；输入框没有关联 label（`htmlFor`/`aria-labelledby`/包裹）；错误信息没用 `aria-invalid`/`aria-describedby` 关联；Switch/Checkbox 缺名称；标题层级跳跃（页面 h1 → tab 标题 h2 → 分区 h3）；div/span 当按钮用；hover 才显示、键盘聚焦时仍不可见的操作；仅靠颜色表达状态；对话框缺标题；折叠区缺 `aria-expanded`；只能拖拽的操作；异步结果没有 `role="status"`/live region。
2. 主题：硬编码颜色（`text-red-500`、`bg-white`、hex、`text-gray-*` 等）；字号用 Tailwind 默认档（`text-xs`/`text-sm`/`text-base`/`text-lg`）而非角色 token；缺深色变体；token 组合可能低对比（读 tokens.css 的 oklch 估算，并说明是估算）。
3. 响应式：固定宽度（`w-[480px]` 等）在窄窗口/200% 缩放溢出；长名称、URL、token 字符串缺 `truncate`/`break-all`/`min-w-0`；表格或宽内容缺横向滚动容器；明显过小的点击目标。
4. 实现完整性：查询失败被显示成空数据或默认值（只读 `data`/`isLoading`，没处理 `isError`）；mutation 失败无反馈；危险操作无确认；禁用控件无原因；与标准 Settings* 组件平行的自造布局；网络 JSON 未经 schema 校验直接断言类型；硬编码英文/中文 UI 文案未走 `t()`；假数据、占位文案、误导性内容。
5. 性能：只报能确认的真实问题（大列表无虚拟化且数据量可能很大、过频轮询、每次渲染重建昂贵对象导致明显重渲染）。不要把「缺少 memo」本身当问题。

## 已修复、不要重复报告

上一轮（`.impeccable/audit/2026-10-13-core/remediation-report.md`）已修：成员管理按钮带成员姓名的可访问名称；通知设置区分加载/失败/空态；自定义任务状态的上移/下移键盘排序和归档保护。除非你发现这些修复本身有新缺陷。

## 严重度

- P0：阻断任务完成。
- P1：明显困难或违反 WCAG AA。
- P2：有变通办法的小问题。
- P3：打磨项。整组最多报 3 条 P3。

宁缺毋滥：只报真实影响用户、且你在代码里核实过的问题。

## 输出格式（中文，代码标识保留英文）

每条发现：

```
### [P?] 简短标题
- 位置：path/to/file.tsx:行号（同类位置也列出）
- 维度：可访问性 / 主题 / 响应式 / 实现完整性 / 性能
- 证据：关键代码片段（≤3 行）和你的推理
- 影响：对用户的具体影响
- 标准：WCAG 条款或仓库规则（如适用）
- 建议：具体修法，优先复用仓库已有组件/模式
- 核实程度：代码已核实 / 待运行验证（附需要的运行证据）
```

最后附：
- 「值得保留」：2–4 条做得好的实现。
- 「已检查无问题」：逐个列出你检查过但没有发现问题的文件。
- 「需要运行时确认的点」：给主审计员在真实界面里核对的清单（具体到 tab、操作和期望）。
