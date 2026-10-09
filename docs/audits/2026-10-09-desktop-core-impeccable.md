# 桌面端核心页面 impeccable 审计 · 2026-10-09

**实现一致性结论：未通过完整性检查，但应保留现有设计体系。** 共享的中性色、字号、列表、侧栏和表单形成了清楚的工作型产品界面。主要缺口集中在键盘可达性、隐藏内容的焦点管理和请求失败反馈；这些问题无法靠调整外观解决。

**审计健康度：13/20，Acceptable（仍需重点修复）。共 10 组问题：P0 0、P1 6、P2 4、P3 0。** 优先修复关闭的聊天浮窗、批量选择和错误空状态。

本次按 `impeccable audit` 的五维技术标准执行，采用 Operate 场景。未修改产品代码，未创建 Trellis 任务。问题按共同原因分组，同一缺陷在多个页面出现只计一组。

| 维度 | 分数 /4 | 依据 |
|---|---:|---|
| 可访问性 | 2 | 有标准组件与键盘支持，但隐藏聊天、列表选择和部分标签导航有明确缺口 |
| 性能 | 3 | 多个长列表已虚拟化；甘特图日期跨度存在无界增长风险，未做生产性能跑分 |
| 主题 | 3 | 浅深主题整体一致；甘特图固定白字与部分状态色对比不足 |
| 响应式 | 3 | 900px 页面整体可用，设置在原生 200% 缩放下可重排；分拣分页与聊天入口重叠 |
| 实现完整性 | 2 | 收件箱和项目将服务失败显示为空数据；同类控件实现不一致 |
| **合计** | **13/20** | 评分限于下述范围，不是 WCAG 合规认证或发布验收 |

**范围与证据**

- 页面：桌面外壳、任务列表/详情、我的任务、收件箱、项目列表/详情、迭代列表/历史详情/进展/范围变化、分拣台、智能体列表/详情、技能库、运行时、设置。skill 详情的标签契约补充静态检查。
- macOS，Electron 39.8.7；当前开发渲染器 `http://localhost:5666`，本地 API `http://localhost:18572`，后端健康检查报告提交 `758168907`。分支 `codex/projects-p1` 有既存未提交修改，结论针对审计时的工作副本。
- 使用独立 Electron profile 和现有登录态的本地副本，加载项目真实 preload、renderer 和本地数据。原用户窗口未被接管。沿用仓库测试夹具隔离守护进程、更新器等 native IPC；运行时页只观察界面，不代表守护进程功能验收。
- 1440×1000 主视口；8 个页面补查 900×700；任务、智能体、设置补查深色主题；设置使用 Electron `setZoomFactor(2)`，有效视口 720×500，另由原生 `capturePage` 截图，避免 Playwright 缩放截图裁切。
- 常规请求使用真实 API；仅故障注入阶段对 `/api/inbox` 和 `/api/projects` 返回 500。业务写请求被拦截；两个使用 POST 的只读表格查询 `/api/issues/table/rows`、`/api/issues/table/facets` 明确放行。未运行智能体、发送消息或创建业务数据。
- [证据目录](../../.impeccable/audit/2026-10-09-desktop-core/)、[清单与校验值](../../.impeccable/audit/2026-10-09-desktop-core/manifest.json)。页面截图、无障碍快照、DOM/焦点记录和 detector 原始输出分别保留。

**P1-01 · 聊天浮窗关闭后，内部控件仍接受键盘焦点**

类别：可访问性。位置：[chat-window.tsx:792](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/chat/components/chat-window.tsx:792)、[chat-window.tsx:826](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/chat/components/chat-window.tsx:826)。

关闭只改变 `opacity`、`scale` 和 `pointerEvents`，没有让子树退出键盘顺序或无障碍树。实测聚焦可见的“问 Multica”，按 Shift+Tab，焦点进入不可见的智能体选择按钮；祖先 `opacity:0`、`pointer-events:none`、`inert:false`。关闭状态下页面无障碍快照也包含聊天内容。

影响：用户在任何核心页面都可能进入看不见的控件，失去当前操作位置。涉及 WCAG 2.4.3、2.4.7。建议关闭时令保留的子树 `inert` 并退出无障碍树，处理打开后的初始焦点和关闭后的焦点归还；仅加 `aria-hidden` 不足以阻止键盘进入。命令：`$impeccable harden`。

证据：[焦点记录](../../.impeccable/audit/2026-10-09-desktop-core/hidden-chat-focus.json)、各页面的无障碍快照。

**P1-02 · 服务失败被显示成“暂无通知”和“还没有项目”**

类别：实现完整性。位置：[inbox-page.tsx:123](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/inbox/components/inbox-page.tsx:123)、[inbox-page.tsx:592](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/inbox/components/inbox-page.tsx:592)、[projects-page.tsx:818](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/projects/components/projects-page.tsx:818)。

两个主查询只消费 `data/isLoading`，失败后使用默认空数组。冷缓存注入 500，分别等待两次失败请求后，收件箱显示“暂无通知 / 收件箱为空”，项目显示“还没有项目 / 创建第一个项目”，均无对应错误提示与重试入口。项目详情 [project-detail.tsx:284](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/projects/components/project-detail.tsx:284) 还存在将无数据显示为不存在的同类代码路径，该详情失败分支未注入复现。

影响：用户可能误以为通知已处理或项目消失，转而重复创建内容。建议区分首次加载失败、后台刷新失败、真实空列表与 404；首次失败提供重试，刷新失败保留已有数据与当前位置。命令：`$impeccable harden`。

证据：[故障记录](../../.impeccable/audit/2026-10-09-desktop-core/failure-evidence.json)、[收件箱 500](../../.impeccable/audit/2026-10-09-desktop-core/screenshots/inbox-500.png)、[项目 500](../../.impeccable/audit/2026-10-09-desktop-core/screenshots/projects-500.png)。

**P1-03 · 批量选择控件对键盘不可见，且缺少名称**

类别：可访问性。位置：[list-row.tsx:97](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/issues/components/list-row.tsx:97)、[list-view.tsx:557](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/issues/components/list-view.tsx:557)、[agents-page.tsx:400](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/agents/components/agents-page.tsx:400)、[projects-page.tsx:339](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/projects/components/projects-page.tsx:339)。

任务列表未选复选框使用 `hidden group-hover/row:block`：实测 7 个逐行复选框为 `display:none`；组全选虽可聚焦，也没有名称。智能体列表选择按钮未选时 `opacity:0`，仅 hover 恢复；实测全选按钮获得焦点后仍透明且无名称。项目列表源码复用了同类模式。

影响：鼠标能完成的逐行选择无法用相同的键盘路径完成，读屏也无法识别选择对象。涉及 WCAG 2.1.1、2.4.7、4.1.2。建议复用技能库已有的带标签 Checkbox：焦点可见，名称包含任务编号或资源名称，全选名称描述范围。命令：`$impeccable harden`。

证据：[DOM 与焦点数据](../../.impeccable/audit/2026-10-09-desktop-core/a11y-evidence.json)、[任务列表](../../.impeccable/audit/2026-10-09-desktop-core/screenshots/issues-list-1440.png)、[智能体焦点截图](../../.impeccable/audit/2026-10-09-desktop-core/screenshots/agents-keyboard-focus-1440.png)。

**P1-04 · 项目表格标题没有直接打开详情的键盘入口**

类别：可访问性。位置：[projects-page.tsx:390](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/projects/components/projects-page.tsx:390)、[use-row-link.ts:77](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/navigation/use-row-link.ts:77)。

项目标题是普通 `span`，整行绑定鼠标事件。实测项目行 `tabIndex:-1`，内部没有 `a[href]`。Tab 能进入编辑控件和操作菜单，却不能聚焦项目标题打开详情；“新标签页打开”菜单是绕行路径。

影响：核心导航缺少直接的键盘等价操作，涉及 WCAG 2.1.1。建议将标题恢复为 `AppLink`，保留整行鼠标操作并处理事件冒泡。命令：`$impeccable harden`。证据同 [DOM 数据](../../.impeccable/audit/2026-10-09-desktop-core/a11y-evidence.json)。

**P1-05 · 甘特条固定白字与部分状态色对比不足**

类别：可访问性、主题。位置：[gantt-view.tsx:298](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/issues/components/gantt-view.tsx:298)、[gantt-view.tsx:422](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/issues/components/gantt-view.tsx:422)。

横条宽度大于 60px 时，标题固定 `text-white text-micro`。将当前主题 token 经 Electron Canvas 转为 sRGB 后计算：warning 背景白字浅色 **2.25:1**、深色 **2.70:1**；深色 success **3.06:1**、info **3.24:1**，低于普通文字 WCAG 1.4.3 的 4.5:1。浅色 success/info 分别约 4.56/4.76，未一并判错。

这是源码与当前 token 的可重复计算，未构造完整甘特业务样例或宣称已测全部状态。建议使用经过对比度验证的状态前景/背景组合，同时检查半透明状态背景。命令：`$impeccable colorize`。

证据：[主题值、RGB 与比值](../../.impeccable/audit/2026-10-09-desktop-core/gantt-token-contrast.json)。

**P1-06 · 桌面标签重排只有指针拖动入口**

类别：可访问性。位置：[tab-bar.tsx:570](/Volumes/artisan/code/2026/multica-0.4.37/apps/desktop/src/renderer/src/components/tab-bar.tsx:570)。

静态检查确认 DnD 仅注册 `PointerSensor`，UI 的 `moveTab` 调用来自拖动结束，上下文菜单没有左右移动操作。键盘用户可以切换标签，但不能完成重排，涉及 WCAG 2.1.1；拖动也缺少 WCAG 2.5.7 所要求的非拖动单指针替代路径。

建议增加可用键盘和鼠标点击的“向左移动 / 向右移动”命令；单加 KeyboardSensor 不能覆盖全部非拖动场景。此项为源码确认，未进行完整辅助技术重排验收。命令：`$impeccable harden`。

**P2-07 · 900px 分拣台分页被聊天入口遮挡**

类别：响应式。位置：[triage-page.tsx:885](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/triage/triage-page.tsx:885)、[chat-fab.tsx:74](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/chat/components/chat-fab.tsx:74)。

900×700 下，“下一页”矩形为 `(816,652,64,32)`，聊天按钮为 `(844,644,40,40)`，两者明显重叠。样例仅一条数据，下一页处于 disabled，未宣称成功复现翻页失败；但启用与否不改变该布局，目标区域会被遮挡。

建议分页栏为右下角预留已有 `--chat-launcher-clearance`，并验证 enabled 分页在窄窗口与放大后的点击范围。命令：`$impeccable adapt`。

证据：[截图](../../.impeccable/audit/2026-10-09-desktop-core/screenshots/triage-900.png)、[矩形记录](../../.impeccable/audit/2026-10-09-desktop-core/triage-fab-overlap.json)。

**P2-08 · 标签组件的语义与键盘约定不一致**

类别：可访问性、实现完整性。位置：[tab-bar.tsx:304](/Volumes/artisan/code/2026/multica-0.4.37/apps/desktop/src/renderer/src/components/tab-bar.tsx:304)、[agent-overview-pane.tsx:346](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/agents/components/agent-overview-pane.tsx:346)、[skill-detail-page.tsx:1336](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/skills/components/skill-detail-page.tsx:1336)。

桌面当前标签仅使用 `data-tab-active`，没有向辅助技术暴露当前项。智能体/skill 详情声明 `tablist/tab`，却缺少漫游 tabIndex、方向键与 tabpanel 关联。智能体详情实测：按 ArrowRight 后焦点仍在“概览”，4 个标签均为 `tabIndex:0`，无 `aria-controls`。

仍可逐个 Tab、Enter 切换，故不报告为完全不可操作。建议详情复用设置页已经使用的 Base UI Tabs；桌面外壳按实际导航语义补充 `aria-current` 或完整标签契约。命令：`$impeccable harden`。

证据：[键盘记录](../../.impeccable/audit/2026-10-09-desktop-core/agent-tabs-keyboard.json)。

**P2-09 · 侧栏布局动画忽略减少动态效果偏好**

类别：可访问性。位置：[desktop-layout.tsx:127](/Volumes/artisan/code/2026/multica-0.4.37/apps/desktop/src/renderer/src/components/desktop-layout.tsx:127)、[desktop-layout.tsx:156](/Volumes/artisan/code/2026/multica-0.4.37/apps/desktop/src/renderer/src/components/desktop-layout.tsx:156)。

启用 `prefers-reduced-motion:reduce` 后切换侧栏，连续动画帧中的 `padding-left` 仍从 0 经 28.9、48.3、64.8 等值变化，`margin-left` 同时插值。Motion 布局弹簧不受相邻 CSS 的减少动画规则控制。

建议在减少动态效果时立即设置最终布局，保留必要状态反馈。此项对应用户偏好与 WCAG 2.3.3（AAA），不冒称 AA 失败。命令：`$impeccable animate`。证据：[动画帧](../../.impeccable/audit/2026-10-09-desktop-core/reduced-motion.json)。

**P2-10 · 甘特日期网格随完整跨度无界增长**

类别：性能。位置：[gantt-view.tsx:87](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/issues/components/gantt-view.tsx:87)、[gantt-view.tsx:173](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/issues/components/gantt-view.tsx:173)、[gantt-view.tsx:257](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/issues/components/gantt-view.tsx:257)。

时间范围从全部任务日期扩展；刻度和背景各按每天生成元素，月缩放仍保留逐日 DOM。十年跨度仅这两层即约 7,300 个元素，任务行另计。此项确认的是增长方式，未构造十年样例或测得实际卡顿。

建议按可见时间窗口绘制刻度和背景，长任务列表窗口化，增加极端合法日期跨度的性能验收。命令：`$impeccable optimize`。

**机械扫描与误报排除**

执行一次 `impeccable detect --json apps/desktop/src/renderer/src packages/views packages/ui`，退出码 2，原始命中 10 条。核查后，**没有将这些命中直接计入上述问题**：

- 6 条 `broken-image` 来自测试文件。
- 1 条 `broken-image` 来自任务详情中的英文代码注释，非 JSX 图片。
- 2 条 `side-tab` 实际是编辑器/Markdown 的 blockquote 左边线，语义正确。
- 1 条 `bounce-easing` 属于完成引导的有意动画，且有 reduced-motion 规则；不属于核心工作页的缺陷。

[原始 detector JSON](../../.impeccable/audit/2026-10-09-desktop-core/detector.json)。扫描没有找到上述主要交互问题，说明不能用“机械扫描通过”代替页面审计。

**值得保留的实现**

- 共享语义 token、统一页面外壳与列表密度适合任务管理，浅深色没有整体风格断裂。
- 收件箱、智能体、技能库已有虚拟化；收件箱实现了虚拟行键盘导航，归档入口也考虑了 focus-within。
- 技能库批量选择、设置的 Base UI Tabs、分拣台标签键盘操作都可作为其他页面修复的范式。
- 当前迭代历史已将任务快照、进展与范围变化分开；单日数据不再强行表达完整趋势，图表还有语义数据表。未照搬 10 月 8 日旧审计作为当前缺陷。
- 8 个 900px 页面未发生文档级横向溢出；表格内部的横向滚动单独处理，未将数据表滚动本身判错。设置 200% 缩放切换为目录入口且内容可滚动：[原生截图](../../.impeccable/audit/2026-10-09-desktop-core/screenshots/settings-native-200pct.png)。

**建议执行顺序**

1. **P1 · `$impeccable harden`**：关闭聊天的焦点管理、请求失败状态、列表选择、项目标题链接、标签重排；同时统一 P2 标签契约。
2. **P1 · `$impeccable colorize`**：修复甘特状态条的前景/背景对比。
3. **P2 · `$impeccable adapt`**：分拣分页避让聊天入口，并验证启用状态。
4. **P2 · `$impeccable animate`**：将桌面布局动画接入减少动态效果偏好。
5. **P2 · `$impeccable optimize`**：限制甘特日期 DOM 的增长。
6. **`$impeccable polish`**：统一修复后的焦点、间距和主题细节，作为最后一步。

可按上述顺序逐项处理，也可合并执行。修复后重新运行 `$impeccable audit`，比较同一证据范围下的评分。

**校验与边界**

- `pnpm --filter @multica/views typecheck`：退出码 0。
- 在 `packages/views` 执行 `pnpm exec eslint issues inbox projects iterations triage my-issues`：退出码 0，0 errors、9 个 `react-hooks/exhaustive-deps` warnings。涉及执行用量计算的多余依赖及 swimlane/table/group/project 的缺失依赖；本次未改动或清理。
- 未运行完整测试、生产构建、性能基准、VoiceOver/NVDA 全流程、Windows/Linux 实机或所有权限角色。深色与 200% 为抽样，不代表所有页面在这些条件下全部通过。
- 独立窗口的 CORS/preload 配置调整、只读 POST 查询放行以及 Playwright 缩放截图裁切均属审计夹具问题，已处理，未计为产品缺陷。
- 新增文件仅为本报告及对应截图、证据和审计脚本；无产品代码简化、提交或推送。既存工作副本改动保持原样。
