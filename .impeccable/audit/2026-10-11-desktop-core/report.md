# 桌面端核心页面技术审计

> 后续修复已完成，见 [修复结果与验证](remediation-report.md)。以下保留修复前的原始审计结论。

日期：2026-10-11。方法：`impeccable audit`，Operate 模式。范围为当前工作区的 Electron 桌面端；本次只审计，未修改产品代码、未创建 Trellis 任务。

## 实现完整性结论

**产品体系一致性：PASS。技术健康度：11/20，Acceptable，仍需较多改进。**

桌面标签栏、共享业务页面、语义 token、Base UI 控件和虚拟列表构成了连贯的工作界面。主要问题在状态恢复、键盘访问和缩放后的功能保留。这个 PASS 仅表示产品体系连贯，不代表发布验收或 WCAG 合规通过。

机械 detector 仅报告一条 `bounce-easing` advisory，位于 `packages/ui/styles/base.css:101` 的引导完成动画，不属于本次核心页面，未计入问题。不能以 detector 无重大输出代替交互检查。

## 评分与摘要

| 维度 | 分数 | 主要依据 |
| --- | ---: | --- |
| 可访问性 | 2/4 | 有共享 Checkbox、Tabs 和键盘导航基础；仍有不可见焦点、缺失名称、排序缺少键盘替代及文字对比度失败 |
| 性能 | 3/4 | 多处虚拟化、延迟挂载和有限渲染；本次样本未发现明确性能故障，但未做生产包和大数据剖析 |
| 主题 | 2/4 | 明暗 token 系统完整；项目状态与优先级误用填充色作为文字色，两个主题均有失败 |
| 响应式 | 2/4 | 900px 窗口能折叠侧栏；200% 原生缩放暴露项目搜索消失、紧凑视图行为不一致 |
| 实现完整性 | 2/4 | 任务、分拣台、聊天和批量删除的异常状态契约不一致 |
| **合计** | **11/20** | **Acceptable：需要修复关键缺口** |

共 **10 项问题：P0 0、P1 6、P2 4、P3 0**。同一原因影响多个按钮或主题时合并为一项，不按重复实例累加。

优先处理任务失败误报删除、项目搜索在缩放后不可恢复、文字对比度和键盘操作。错误恢复和可访问性修复应先于视觉微调。

## 范围、环境与证据边界

覆盖 11 个主页面：任务、我的任务、收件箱、聊天、项目、迭代、分拣台、智能体、技能库、运行时、设置。另深入检查任务详情、项目详情、任务属性抽屉和桌面标签栏。后四个辅助页面主要做页面状态、主题、布局抽查，未穷举所有设置与操作。

- macOS，Electron 39.8.7；实际桌面 preload、renderer、路由及 shell。
- 渲染入口 `http://localhost:5666`，源码来自本工作区的 Vite；API `http://localhost:18572`，health 报告后端 commit `f3c3b395c`。
- 主检查尺寸为 1440×1000、900×700；原生 `webContents.setZoomFactor(2)` 后有效 CSS 视口为 720×500。
- 11 页各有浅色宽窗、深色宽窗和浅色窄窗截图；明暗文字颜色抽样覆盖项目、迭代、智能体、任务。
- 使用临时独立 Electron profile。API 写操作被测试夹具拦截，只放行任务表查询所用的只读 POST；未执行实际删除、发送消息或智能体任务。
- native daemon/updater 等 IPC 使用仓库既有测试夹具隔离。因此运行时页面截图不代表真实守护进程安装或升级验收。
- 500 场景由 Playwright 拦截注入，通过实际 React Query 的 reset/invalidate 触发请求；没有把伪造错误文案直接写进页面。
- 工作区已有未提交改动，审计过程中其他工作将 HEAD 从 `f79662a56` 推进到 `ba96dfe206b6a4117dd7986f0458e1682eaccab1`。本报告采用当前文件与本次捕获证据；关键定位在报告生成前再次核对。

本次不是完整 WCAG 认证：未实测 VoiceOver、Windows/Linux、触摸设备、400% 缩放、所有长文本/语言组合。性能分数来自实现检查和有限真实数据，不代表生产启动时间、帧率或大数据吞吐基准。桌面普通鼠标按钮未按移动端 44px 规则一律判错。

## P1：发布前应修复

### F01：真实任务遇到 500 后显示为“不存在或已删除”

**类别：实现完整性。证据：Electron 实测及代码。**

位置：`packages/core/issues/canonical-id.ts:98`、`packages/views/issues/components/issue-detail-route.tsx:90`；首次 UUID 请求也受 `issue-detail.tsx:1353`、`:2264` 的 `!issue` 分支影响。

复现：打开真实任务 DQA-15，令标识符详情请求返回 HTTP 500，触发刷新并等待重试结束。实际发生 2 次失败请求，Query 仍有已授权缓存，页面却显示“这个任务不存在或已在该工作空间被删除。”，只提供“返回”。

影响：短暂服务故障被当成任务删除，已加载内容被替换，用户无法就地重试，也无法继续查看当前工作。

原因：`resolve.isError` 直接映射到 `notFound`，未区分 404、网络错误和有缓存的刷新错误。违反仓库 `desktop-core-accessibility.md` 的错误恢复矩阵。

建议：404 使用不存在状态；首次暂时失败显示错误与重试；有授权缓存时保留内容和编辑器，仅附加刷新失败提示。权限撤销仍按独立保护流程处理。命令：`$impeccable harden`。

证据：[请求与缓存状态](issue-failure.json)、[错误页面截图](screenshots/issue-cached-500-not-found.png)。

### F02：项目搜索在 200% 缩放后消失，条件仍生效

**类别：响应式。证据：Electron 实测及代码。**

位置：`packages/views/projects/components/projects-page.tsx:985`；无匹配状态约在 `:1245`。

复现：宽窗搜索 `audit-no-such-project-20261011`，再将 1440px 原生窗口缩放到 200%。实际 CSS 宽度 720px，搜索框被 `hidden md:block` 隐藏；输入值仍保留，页面显示“没有匹配的项目”，没有清空搜索入口。

影响：低视力用户放大页面后不能修改或移除当前搜索，只能先缩小页面。属于 WCAG 1.4.4/1.4.10 所要求的放大后功能保留问题；本次实测为 200%，未声称完成 400% 验收。

建议：紧凑工具栏保留搜索或提供明确展开入口；有查询条件时显示可清除条件；无匹配状态提供重置。命令：`$impeccable adapt`。

证据：[DOM 测量](project-interaction-evidence.json)、[辅助功能树](projects-search-200pct.txt)、[原生缩放截图](screenshots/projects-search-stranded-200pct.png)。

### F03：项目行菜单通过 Tab 获得焦点后仍完全透明

**类别：可访问性。证据：Electron 键盘实测及代码。**

位置：`packages/views/projects/components/projects-page.tsx:257`。

复现：鼠标移出项目行，通过 Shift+Tab / Tab 进入“项目操作”。测得 `document.activeElement === button`、`:focus-visible === true`，但计算样式 `opacity: 0`。仅 hover 或菜单打开后才显示。

影响：键盘用户无法辨认焦点落点，违反 WCAG 2.4.7；列表和卡片共用此按钮。

建议：增加 `focus-visible:opacity-100` 或行内 `focus-within` 显示，并提供明确焦点环。命令：`$impeccable harden`。

证据：[实际焦点与透明度](project-keyboard-focus.json)、[截图](screenshots/project-menu-invisible-keyboard-focus.png)。

### F04：核心图标按钮和任务属性对话框缺少可访问名称

**类别：可访问性。证据：部分 Electron 实测，其余由组件实现确认。**

| 入口 | 位置 | 证据 |
| --- | --- | --- |
| 项目详情“更多” | `packages/views/projects/components/project-detail.tsx:562` | 图标按钮无文本、aria-label 或 title；实际辅助功能树中无名称 |
| 窄窗项目筛选 | `packages/views/projects/components/projects-page.tsx:1010` | 720px 时文案隐藏，没有 aria-label；实际树中为无名按钮 |
| 收件箱批量操作菜单 | `packages/views/inbox/components/inbox-page.tsx:551` | 无名图标触发器；900px 辅助功能树确认 |
| 任务批量工具栏“清除选择” | `packages/views/issues/components/batch-action-toolbar.tsx:216` | 仅 X 图标，代码确认；本次未单独打开该工具栏复测 |
| 任务属性抽屉 | `packages/views/issues/components/issue-detail.tsx:3600` | 实际 `role=dialog`，aria-label 与 aria-labelledby 都为 null |

影响：读屏用户不能辨认操作或当前模态区域的用途，违反 WCAG 4.1.2。可见“属性”按钮并不会自动成为整个 dialog 的名称。

建议：为触发器补充稳定的本地化名称，为 Sheet 提供 SheetTitle 或显式关联名称；新增验证应按用途名称定位控件。命令：`$impeccable harden`。

证据：[项目详情树](project-detail.txt)、[项目缩放树](projects-search-200pct.txt)、[收件箱窄窗树](inbox-light-narrow.txt)、[抽屉语义](issue-dialog-evidence.json)、[抽屉截图](screenshots/issue-properties-unnamed-dialog-200pct.png)。

### F05：项目状态与优先级文字对比度不足

**类别：可访问性 / 主题。证据：实际计算颜色与背景测量。**

位置：`packages/core/projects/config.ts:16`、`:18`、`:35`；使用位置 `packages/views/projects/components/project-badge.tsx:34`、`:70`。

| 文本 | 主题 | 实测比例 | 所需比例 |
| --- | --- | ---: | ---: |
| 进行中，白字/黄色状态底 | 浅色 | 2.25:1 | 4.5:1 |
| 高，黄色文字/页面底 | 浅色 | 2.17:1 | 4.5:1 |
| 进行中，白字/黄色状态底 | 深色 | 2.70:1 | 4.5:1 |
| 已完成，白字/蓝色状态底 | 深色 | 3.24:1 | 4.5:1 |

上述均为 12px 文字。使用 CSS token 并不保证组合后的对比度；这里的 `text-white` 与语义填充色没有配套的文字对比度约束。违反 WCAG 1.4.3。

建议：为状态背景与文字定义成对 token；优先级文本使用适合文字的语义 foreground，避免直接用 warning 填充色。分别验证明暗、hover 和 selected 组合。命令：`$impeccable colorize`，随后 `$impeccable harden`。

另外，智能体页浅色“在线”文字抽样为 4.40:1，接近但低于门槛，应在同一语义色修复中核对；不单独计项。重复名称旁的头像首字不作为额外缺陷。

证据：[对比度原始数据及算法](contrast-evidence.json)、[测量脚本](scripts/contrast.mjs)、[浅色项目](screenshots/projects-light-wide.png)、[深色项目](screenshots/projects-dark-wide.png)。

### F06：任务及泳道手动排序缺少键盘与非拖动替代

**类别：可访问性。证据：代码路径确认；未执行真实排序写操作。**

位置：`packages/views/issues/components/list-view.tsx:163`、`board-view.tsx:506`、`swimlane-view.tsx:1037`。

三个 DnD 上下文仅配置 PointerSensor；现有任务菜单和快捷键没有调整同组相对位置的命令。键盘可以改任务状态，但不能完成同组内排序或泳道重排。

影响：依赖键盘或不能拖动的用户无法执行这类操作，涉及 WCAG 2.1.1 和 2.5.7。

建议：增加遵守分组边界的上移/下移菜单或快捷键，执行后保留焦点并播报位置。可参照已实现替代操作的桌面标签栏。命令：`$impeccable harden`。

## P2：下一轮应修复

### F07：紧凑任务范围菜单不能退出保存视图

**类别：响应式 / 实现完整性。证据：代码确认，未创建保存视图进行运行时复测。**

位置：`packages/views/issues/components/issues-header.tsx:1207`、`packages/views/my-issues/components/my-issues-header.tsx:150`；实际范围选择逻辑在 `packages/views/issues/surface/issue-surface.tsx:111`。

宽屏内置范围操作会清空 activeView；紧凑菜单只更新 scope。保存视图存在时，effectiveScope 仍优先采用保存视图，且保存视图栏在紧凑布局隐藏。

复现路径：宽窗打开保存视图，再缩到小于 768 CSS px，选择其他内置范围。范围标签可以变化，实际数据范围仍受旧保存视图控制。

影响：标签与结果含义不一致，用户需要放大窗口才能可靠退出或改选保存视图。建议共用同一范围切换逻辑，并保留保存视图入口。命令：`$impeccable adapt`。

### F08：分拣台后台请求失败会移除已缓存队列

**类别：实现完整性。证据：队列 Electron 实测；历史分支代码确认。**

位置：`packages/views/triage/triage-page.tsx:380`、`:868`；30 秒刷新见 `packages/core/triage/queries.ts:60`。

复现：进入“全部未决”，已有 1 行；注入 500 后刷新。发生 2 次失败请求，Query 仍有 1 行缓存，实际队列却被原始错误字符串和重试按钮替换。历史分支也在 isError 时替换时间线，而且没有显式 Retry。

影响：暂时网络故障打断阅读与焦点，已展开的子内容可能因卸载丢失状态。与项目列表已经实现的保留缓存恢复模式不一致。

建议：有授权缓存时保留列表和展开状态，附加本地化错误提示；首次失败及历史页面提供带 pending 状态的重试。命令：`$impeccable harden`。

证据：[前后辅助功能树、缓存行数和请求次数](triage-cached-failure.json)、[截图](screenshots/triage-cached-rows-500.png)。

### F09：聊天历史首次加载失败显示为“暂无历史对话”

**类别：实现完整性。证据：Electron 实测及代码。**

位置：`packages/views/chat/components/use-chat-controller.ts:222`、`:230`，`packages/views/chat/components/chat-thread-list.tsx:514`；页面消费入口 `packages/views/chat/chat-page.tsx:228`。

复现：原有 1 个对话；模拟无缓存请求，将 sessions GET 返回 500。实际 2 次失败后，Query 状态 error、无 data，但界面展示“暂无历史对话”，没有错误或重试入口。

影响：用户误以为历史丢失或账号没有对话。sessions 被默认成空数组，页面没有消费错误状态；消息查询也应一并检查同类分支。

建议：分别展示 pending、成功空列表、首次错误、缓存刷新错误，保留当前会话与草稿并提供 Retry。命令：`$impeccable harden`。

证据：[完整请求与页面状态](chat-failure.json)、[截图](screenshots/chat-500-false-empty.png)。

### F10：项目批量删除尚未完成就清空选择，失败缺少反馈

**类别：实现完整性。证据：事件处理器、mutation 和 QueryClient 配置确认；未实际删除数据。**

位置：`packages/views/projects/components/projects-page.tsx:767`；`packages/core/projects/mutations.ts:129`；`packages/core/query-client.ts:3`。

确认后对每行调用 mutate，立即关闭弹窗并 onClear；无等待、失败项保留、pending 禁用或错误汇总。删除 hook 也没有错误提示的兜底。

影响：部分请求失败时用户失去原选择，不知道哪些成功，只能逐项核对。建议等待所有结果，保留失败项选择，汇总结果并阻止重复提交。命令：`$impeccable harden`。

## 共性问题与已排除的噪声

1. 错误恢复契约已有规范和可复用实践，但任务、聊天、分拣台尚未一致采用。优先沿用 React Query 的 data/isError/isFetching/refetch，而非新建数据状态层。
2. 宽屏/紧凑布局分别实现交互，容易只保留标签或图标而丢失行为、搜索入口和可访问名称。
3. 部分状态色只有“填充”语义，没有对应文字 foreground；主题切换时文字可读性并未得到保证。
4. 原始颜色扫描中的 `.sr-only` 文本不是可见文字，不计对比度缺陷；重复头像字母未另计。表格/看板内部的横向滚动不自动算页面响应式错误。
5. 没有 PRODUCT.md / DESIGN.md 属于上下文文档缺口，不阻止已有产品审计，也未据此扣分或改写视觉体系。

## 应保留的实践

- 桌面标签栏已有明确当前目的地、菜单及 Alt+Shift+方向键排序，且遵守固定标签边界；此前同类问题已修复。
- Shell 对减少动态效果的运行中切换有处理，能终止正在执行的 spring；相关测试通过。
- 项目与任务列表已有具名选择框、标题链接、虚拟化；项目列表错误恢复测试使用真实 QueryClient 验证保留行和选择。
- 迭代页使用具名 Tabs、筛选、分组和重置；图表提供语义数据表，状态不只依赖颜色。
- 明暗主题整体能正确切换。普通 muted 文字抽样约为 5.38:1（浅色）和 5.68:1（深色），问题主要集中于特定状态色组合。
- 隐藏聊天内容可能仍在 DOM 中，但本次辅助功能树未包含其编辑入口；不能仅根据 DOM 文本判定隐藏内容可被键盘访问。

## 已运行的验证

定向测试共 **9 个文件、154 项通过**：

```sh
pnpm --filter @multica/views exec vitest run issues/components/issue-detail-route.test.tsx issues/components/batch-action-toolbar.test.tsx issues/components/list-view.test.tsx
# 3 files / 21 tests passed

pnpm --filter @multica/desktop exec vitest run src/renderer/src/components/tab-bar.accessibility.test.tsx src/renderer/src/components/desktop-layout.test.tsx
# 2 files / 9 tests passed

pnpm --filter @multica/views exec vitest run projects/components/projects-page.test.tsx triage/triage-page.test.tsx iterations/iteration-navigation.test.tsx iterations/iteration-details.test.tsx
# 4 files / 124 tests passed
```

这些测试验证现有受保护行为，不覆盖本报告全部缺陷，不能把通过数量解释为问题不存在。本次未改产品代码，未运行全仓 lint/typecheck、Go tests 或完整 E2E。静态检测使用 impeccable detector，交互证据来自实际 Electron。

原生缩放截图使用 CDP 完整表面捕获，校验 CSS 720×500、devicePixelRatio 4、PNG 2880×2000，避免 Playwright 常规截图在 Electron 原生 zoom 下裁切错误；见 [截图几何记录](native-zoom-captures.json)。

## 建议执行顺序

1. **P1 `$impeccable harden`**：F01、F03、F04、F06；补齐错误分类、焦点、名称和排序替代。
2. **P1 `$impeccable adapt`**：F02，并一起处理 F07 的紧凑范围切换。
3. **P1 `$impeccable colorize`**：F05，为状态建立合格的前景/背景配对。
4. **P2 `$impeccable harden`**：F08、F09、F10，统一缓存保留、重试和批量结果反馈。
5. **`$impeccable audit`**：修复后复测以上具体场景，重新评分。
6. **`$impeccable polish`**：最终检查修复后的对齐、间距及控件状态一致性。

可以逐项、一次性或按指定顺序安排后续修复。本次仅新增此目录下的报告、截图、测量数据和复现工具；产品代码未修复或重构，以上问题仍待处理。
