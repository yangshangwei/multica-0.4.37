# Multica 桌面端核心页面 Impeccable 审计

日期：2026-10-13。模式：`impeccable audit`，Operate 工作界面。仅审计，不修改产品代码，不创建 Trellis 任务。

## 实现一致性结论

**产品设计体系一致性：通过；技术质量仍需修复。** 桌面壳层、角色命名字号、语义颜色、共享列表和设置框架形成了清晰的工作型界面。没有发现需要推倒重做的视觉体系问题。主要缺口在键盘交互、辅助技术语义，以及少数查询失败被伪装成正常空数据。

Impeccable detector 对共享 views/UI 的 9 条结果已逐条分类：6 条来自测试字符串中的 `<img>`，2 条是有明确语义用途的 Markdown 引用块左边线，1 条是入门完成动画的 advisory。它们不计入本报告缺陷。桌面 renderer 单独扫描返回 `[]`。机械扫描没有发现问题，不代表键盘和运行状态合格。

## 评分和范围

| 维度 | 分数 / 4 | 主要依据 |
| --- | ---: | --- |
| 可访问性 | 2 | 有 Base UI、明确标题和命名控件，但存在筛选吞键、透明焦点、无名称按钮和排序缺口 |
| 性能 | 3 | 已有列表/甘特虚拟化和延迟挂载菜单；未发现可确认的严重性能缺陷，未做生产性能测试 |
| 主题 | 3 | 双主题和语义 token 一致；标签颜色算法没有保证文字对比度 |
| 窗口适配 | 3 | 13 页在原生 200% 缩放下均无整页横向溢出；紧凑分支仍有对话框语义缺口 |
| 实现完整性 | 2 | 设计系统连贯，但三个业务查询仍有错误/空态混淆，公共组件的可访问性约定执行不一 |
| **合计** | **13 / 20** | **Acceptable：基本可用，仍有明显缺口** |

共 **9 项：P0 0、P1 6、P2 3、P3 0**。分数是本次抽样审计的启发式评估，不是 WCAG 认证或性能基准。

页面覆盖：任务、我的任务、项目、迭代、分拣台、收件箱、聊天、智能体、AI小队、技能库、MCP、运行时、设置，共 13 个一级页面；另检查项目详情、成员/通知/任务状态设置、任务筛选菜单。

实机环境：macOS，Electron 39.8.7，当前工作区 Vite renderer `http://localhost:5666`，本地 API `http://localhost:18572`（health commit `5804f3936`）。采用临时隔离 profile 和真实桌面 preload/router；原用户 profile 未改动。原生服务使用现有审计 harness，未启动智能体。API 写入被拦截，仅放行任务表查询所需的只读 POST。

采集正常窗口 1440×1000 的浅色/深色，以及 900×1000 窗口原生 200% 缩放（有效 450×500 CSS px），共 39 张初始页面截图、39 份辅助功能树、1,831 条可见文字对比度采样。首次冷加载截图中有加载状态，另对任务、我的任务、迭代、技能库、MCP补充确认截图。迭代页本轮采集仍处于加载状态，因此该页仅完成壳层/加载态与代码检查，数据列表和完整交互未获运行验证；技能库、MCP和我的任务另有初轮已加载内容的证据。该结果覆盖所列窗口与页面状态，不代表所有内容都没有内部溢出。

## 优先修复的问题

### F01 · P1 · 任务筛选框输入被菜单快捷导航截获

- **位置**：[issues-header.tsx:355](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/issues/components/issues-header.tsx:355)，同类位于 526、617 行。涉及负责人、创建者、项目、标签子菜单。
- **类别**：可访问性 / 交互正确性。WCAG 2.1.1。
- **证据**：Electron 打开负责人筛选后，点击输入框并真实键入 `abc`，输入值仍为 `""`。当前 Base UI Menu 的 typeahead 会处理冒泡的一字符按键；同文件自定义属性输入已在 800–810 行隔离这类键盘事件，其他输入没有同样处理。独立最小复现也确认按键被 preventDefault，并可使焦点跳到匹配菜单项。
- **影响**：成员、项目和标签较多时，用户无法用正常键盘输入查找候选。
- **建议**：复用同文件已有的输入键盘隔离策略；验证英文、中文输入法、空格、方向键以及 Escape/Tab 的退出行为。
- **命令**：`$impeccable harden`。
- **证据文件**：[输入结果](filter-search-evidence.json)、[截图](screenshots/filter-search-keyboard.png)。

### F02 · P1 · 智能体行菜单获得焦点后仍透明

- **位置**：[agent-row-actions.tsx:145](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/agents/components/agent-row-actions.tsx:145)。
- **类别**：可访问性。WCAG 2.4.7。
- **证据**：宽窗口下，行按钮通过 `@2xl:opacity-0` 隐藏，仅 hover/open 恢复；没有 focus-visible 或 focus-within 显示规则。Electron 聚焦后测得 `opacity: 0`，按钮尺寸 28×28，存在于焦点序列但不可见。
- **影响**：键盘用户无法知道焦点所在位置，也难以发现归档、复制等菜单操作。
- **建议**：添加焦点显示和可见焦点环，复用项目/AI小队列表已有模式；同时保留鼠标悬停行为。
- **命令**：`$impeccable harden`。
- **证据文件**：[焦点测量](agent-focus-evidence.json)、[截图](screenshots/agent-menu-keyboard-focus.png)。

### F03 · P1 · 看板按钮、成员操作及项目抽屉缺少可访问名称

- **位置**：[board-column.tsx:221](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/issues/components/board-column.tsx:221)（230、253 行同类）、[members-tab.tsx:192](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/settings/components/members-tab.tsx:192)、[project-detail.tsx:653](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/projects/components/project-detail.tsx:653)。
- **类别**：可访问性。WCAG 4.1.2。
- **证据**：看板列头更多/新增按钮在辅助功能树中显示为无名称 `button`；成员管理按钮没有文字、aria-label 或 title。900px 窗口原生 200% 缩放后打开项目属性抽屉，实际 `role=dialog` 的 aria-label/aria-labelledby 均为 null。共享 Button/Sheet 不自动补名称；DeferredTooltip 只处理指针悬停，不能代替按钮命名。
- **影响**：读屏用户无法辨识多个列头操作、管理哪位成员，或当前进入了哪个模态区域。
- **建议**：按钮使用包含列/成员上下文的本地化名称；项目抽屉添加 SheetTitle，复用任务详情已有做法。延迟菜单的冷/热触发分支都需命名。
- **命令**：`$impeccable harden`。
- **证据文件**：[任务辅助功能树](issues-dark-wide.txt)、[成员辅助功能树](members.txt)、[抽屉语义](project-drawer.json)、[抽屉截图](screenshots/project-drawer-200pct.png)。

### F04 · P1 · 标签文字色算法无法保证对比度

- **位置**：[label-chip.tsx:25](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/labels/label-chip.tsx:25)。
- **类别**：可访问性 / 主题。WCAG 1.4.3。
- **证据**：真实任务标签的 12px 文字为 `#f9fafb`；红色 `#ef4444` 底上约 **3.60:1**，蓝色 `#3b82f6` 底上约 **3.52:1**，紫色 `#8b5cf6` 底上约 **4.05:1**，均低于 4.5:1。当前算法使用未经 sRGB 线性化的加权亮度和固定 0.55 阈值，没有比较实际对比度。深浅主题中都可出现。
- **影响**：任务类型等标签内容对低视力用户不易辨认；更换整体主题不能解决。
- **建议**：按 WCAG 相对亮度计算候选前景的对比度，选满足 4.5:1 的颜色；若现有候选不足，使用足够深/浅的文字色或调整标签填充。保留用户标签色身份。
- **命令**：`$impeccable harden`。
- **证据文件**：[全部对比度测量](baseline-metrics.json)、[深色任务截图](screenshots/issues-dark-wide.png)、[浅色缩放截图](screenshots/issues-light-200pct.png)。
- **排除项**：头像中的单字与相邻完整姓名冗余，不将该装饰性头像采样单独列为文字对比度缺陷。

### F05 · P1 · 收件箱方向键换选缺少辅助技术选择模型

- **位置**：[inbox-list.tsx:99](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/inbox/components/inbox-list.tsx:99)（128–151、220–227 行相关）、[inbox-list-item.tsx:92](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/inbox/components/inbox-list-item.tsx:92)。
- **类别**：可访问性。WCAG 1.3.1 / 4.1.2。
- **证据层级**：代码路径核实，页面辅助功能树佐证；未使用 VoiceOver 实测播报。
- **证据**：点击或方向键切换通知时，焦点交给 tabIndex=-1 的普通滚动 div。容器没有复合控件角色或 aria-activedescendant；行没有 aria-selected/aria-current/aria-pressed，仅背景颜色表示选中。
- **影响**：视觉高亮和详情内容变化，但辅助技术没有稳定的当前项关联。
- **建议**：为虚拟化通知列表建立明确的焦点与选择模型；保留行内归档/菜单按钮的独立操作，处理虚拟行卸载，使用真实读屏器验证选中项和换选播报。
- **命令**：`$impeccable harden`。
- **证据文件**：[收件箱辅助功能树](inbox-light-wide.txt)。

### F06 · P1 · 自定义任务状态只能拖动排序

- **位置**：[issue-statuses-tab.tsx:243](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/settings/components/issue-statuses-tab.tsx:243)，314、390–398、443–449 行相关。
- **类别**：可访问性。WCAG 2.1.1；缺少无拖动的指针替代路径涉及 WCAG 2.2 的 2.5.7。
- **证据层级**：代码核实。本地工作空间目前仅有内置状态，没有创建额外数据来运行排序。
- **证据**：DndContext 显式只注册 PointerSensor；菜单只有编辑/归档，没有上移/下移操作。手柄即使可聚焦，键盘也不能触发排序。
- **影响**：键盘用户无法完成工作流配置；不便拖动的用户缺少替代方式。
- **建议**：复用任务列表现有的上移/下移模式，并可补 KeyboardSensor；维持分类/归档边界，排序后保留焦点并播报结果。
- **命令**：`$impeccable harden`。
- **证据文件**：[本地状态设置辅助功能树](settings-statuses.txt)。

### F07 · P2 · 项目甘特图将请求失败显示为无排期任务

- **位置**：[use-issue-surface-data.ts:149](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/issues/surface/use-issue-surface-data.ts:149)、[gantt-view.tsx:627](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/issues/components/gantt-view.tsx:627)。
- **类别**：实现完整性 / 错误反馈。
- **证据**：在有正式任务和排期的项目中，仅对 `GET /api/issues?...&scheduled=true` 注入 500。两次请求失败后实际 Query 为 error 且无 data，页面却显示“暂无已排期的任务 — 给任意任务设置开始或截止日期，即可显示在时间轴上。”没有对应错误或重试入口。查询 data 被回退为空数组，只透出 loading。
- **影响**：用户可能把服务不可用误判成未做排期，并重复检查或编辑数据。
- **建议**：将该查询的 error/refetch 传入共享任务表面；首次失败显示重试，缓存刷新失败保留任务与提示，成功零结果才显示排期引导。
- **命令**：`$impeccable harden`。
- **证据文件**：[请求和 Query 状态](gantt-failure.json)、[截图](screenshots/gantt-false-empty.png)。

### F08 · P2 · 通知设置及运行时用量缺少读取失败状态

- **位置**：[notifications-tab.tsx:34](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/settings/components/notifications-tab.tsx:34)、[usage-section.tsx:139](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/runtimes/components/usage-section.tsx:139)。
- **类别**：实现完整性 / 错误反馈。
- **证据**：通知偏好冷缓存 GET 两次 500 后，实际 Query 为 error、无 data，面板仍展示七个开启的开关，没有错误提示或重试。`data?.preferences ?? {}` 将未知配置当默认开启。运行时用量的同类代码只读取 data/loading，失败后进入 UsageEmpty；用量部分未另注入网络故障。
- **影响**：用户无法区分真实配置/无用量和读取失败，界面表达了尚未获取到的事实。
- **建议**：区分 pending、error、成功空态；无已知配置时不要展示可提交的伪默认值。有缓存时保留上次结果并标记刷新失败。
- **命令**：`$impeccable harden`。
- **证据文件**：[通知失败与开关状态](notification-failure.json)、[截图](screenshots/notification-failure-defaults.png)。

### F09 · P2 · 项目侧栏折叠分区不暴露展开状态

- **位置**：[project-detail.tsx:348](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/projects/components/project-detail.tsx:348)，487、512 行相关。
- **类别**：可访问性。WCAG 4.1.2。
- **证据**：“属性”“范围闭合”“描述”按钮根据组件状态显示内容、旋转箭头，但没有 aria-expanded 和区域关联；实际辅助功能树不含这些展开状态。同页“自动化”分区已有 expanded 信息，形成不一致。
- **影响**：读屏用户难以知道操作将展开还是折叠、相关内容目前是否可见。
- **建议**：复用已有 Collapsible，或补充 aria-expanded、稳定内容 ID 与 aria-controls。
- **命令**：`$impeccable harden`。
- **证据文件**：[项目详情辅助功能树](project-detail.txt)。

## 系统性原因与值得保留的实现

问题主要出现在四个边界：菜单中嵌套文本输入、hover 隐藏操作、公共 primitive 的命名责任，以及 Query 的 data 默认值。这些缺口应在所属共享组件或业务查询边界解决，再覆盖各使用方，无需建立新的设计系统。

值得保留：

- 桌面导航与页面层级稳定，紧凑窗口提供设置目录和侧栏抽屉；本次 13 页均没有整页水平溢出。
- 字号使用角色命名 token，页面、表面和状态颜色语义统一；无需因缺少 DESIGN.md 而另造一套视觉风格。
- 迭代/分拣等页面已有明确标题、错误区域和重试；任务手动排序已有拖拽之外的入口，适合复用到状态管理。
- 任务看板、收件箱和甘特已有虚拟化，菜单延迟挂载；没有把缺少 memo、固定图标尺寸或正常局部滚动当成性能问题。
- 减少动态效果已有媒体查询和桌面壳层处理；本轮没有发现足以新增报告的严重动画缺陷。

## 建议执行顺序

1. **P1 — `$impeccable harden`**：先处理 F01–F03，恢复可靠键盘输入、可见焦点和可访问名称。
2. **P1 — `$impeccable harden`**：处理 F04–F06，修正对比度算法、通知选择模型与状态排序入口。
3. **P2 — `$impeccable harden`**：处理 F07–F09，补齐读取失败反馈与折叠语义。
4. **复核 — `$impeccable audit`**：复跑这里的键盘、网络故障与缩放场景，比较剩余缺陷和评分。
5. **收尾 — `$impeccable polish`**：对已修复的焦点、错误提示、标签和抽屉做一次有界视觉确认。

可按项、合并或调整顺序执行。上述是修复建议，本次没有实施修复。

## 证据与验证边界

- [截图总览](screenshots/contact.jpg)、[基线测量](baseline-metrics.json)、[补充确认记录](confirmation.json)。
- [共享代码 detector](detector.json)、[桌面 renderer detector](detector-desktop.json)。
- 对比度测量使用实际 computed style 和逐层背景合成，排除不可见、禁用、透明动画和带背景图的节点；采样包括重复文本，不是 1,831 个独立组件。
- F01–F04、F07、通知设置和项目抽屉使用当前 Electron 实测；F05、F06、运行时用量补充代码证据。未把静态检查声称为真实读屏器验证。
- 未覆盖 Windows/Linux、VoiceOver/NVDA、物理触屏、生产构建性能、自动化/统计等未列出的二级流程、全部弹窗和真实服务写入。开发模式首次加载延迟和 harness 的 WebSocket 失败未列为产品性能缺陷。
- 本轮仅新增 `.impeccable/audit/2026-10-13-core/` 下报告、脚本和证据；没有修改产品实现、依赖或测试，没有运行完整 lint/typecheck/单元测试。浏览器验证是审计取证，不等同于全量回归通过。
