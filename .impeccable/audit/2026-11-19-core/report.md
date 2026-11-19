# 核心 UI 技术审计 · 2026-11-19

**实现一致性结论：通过产品系统一致性检查；交互质量仍有明显缺口。** 当前界面有一致的语义颜色、字号角色、共享页面框架和业务组件，具有明确的任务协作产品结构。Impeccable 检测器没有产生经核实的产品缺陷；本报告的问题来自原生键盘/辅助功能检查、源码调用链和颜色计算。检测器干净不代表可访问性或数据恢复行为合格。

本轮按 Operate 模式审计 Web 与 Electron 共用核心界面：导航、任务列表/看板/详情、属性选择器、评论/回复、快速创建、智能体与 AI 小队目录及创建/详情。未覆盖原生移动应用、营销站、完整协作空间、全部设置与管理后台。仅记录问题，未修改产品代码。

| 维度 | 分数 | 主要依据 |
|---|---:|---|
| 可访问性 | 2/4 | 有共享原语和键盘排序，但隐藏侧栏可聚焦，选择器/编辑器/折叠状态语义不完整 |
| 性能 | 3/4* | 列表、看板、Gantt 和智能体目录有虚拟化；未做运行时性能分析 |
| 主题 | 3/4 | 语义 token 基础完整；深色品牌实心按钮文字对比度不足 |
| 响应式 | 2/4* | 已有紧凑导航与部分 44px 触控适配，共享页头操作仍为 32px；两个复杂布局待实测 |
| 实现完整性 | 2/4 | 小队草稿、失败恢复、只读展示没有一致复用现有成熟行为 |
| **总计** | **12/20** | **Acceptable：可用基础明确，仍需重要修复** |

\* 分数是本轮有限范围的工程判断，不是 Lighthouse 分数、设备认证或完整 WCAG 合规结论。性能和响应式缺少全设备实测证据，不应据此宣称已通过验收。

**已确认 11 项：P0 0，P1 7，P2 4，P3 0。** 首先修复指令草稿覆盖和隐藏焦点，再处理加载失败、选择器语义及深色文字对比度。另有两项响应式风险单独列出，未计入缺陷数。

## 证据与边界

- 当前分支：`codex/cross-workspace-collaboration`。审计对象为有大量未提交改动的工作树，而非纯 HEAD；关键文件散列记录于 [manifest.json](./manifest.json)。
- 使用 `cua_repl` 检查本机 `Multica Canary work-management-local`，renderer 为 `localhost:5666`；进程工作目录确认指向当前 checkout 的 `apps/desktop`。未记录打包产物散列，不将该观察声称为某个发布版本验收。
- 实际使用了鼠标、Tab、方向键、原生辅助功能树和截图。查看了收件箱、任务看板/详情、智能体目录、快速创建、小队目录/详情。结束时返回收件箱；未提交任务、评论、状态、成员或指令变更。
- `make status` 显示 Web 服务未启动，API health 不能证明归属当前 checkout，因此未把当前 API 当作完整端到端验收环境，也未重启已有服务。
- 没有执行触摸手势、手机实机、窄视口、200% 缩放、深色截图或性能录制。颜色结论来自当前 token 计算，不能冒充屏幕取色。
- 原生观察摘要见 [native-observations.json](./native-observations.json)，截图保留在本次工具对话中，没有另存独立截图文件。

## P1：优先修复

### F01 · 小队指令会被实时更新覆盖，未保存草稿丢失

**类别：** 实现完整性。**证据：** 源码调用链确认，未在用户数据上制造并发写入。

**位置：** [squad-detail-page.tsx:1299](../../../packages/views/squads/components/squad-detail-page.tsx#L1299)；`packages/core/realtime/use-realtime-sync.ts:830`；`packages/views/editor/content-editor.tsx:877`。

**触发与影响：** 用户编辑小队指令，停顿超过 150ms，但还没点击保存；另一成员修改同一指令后，实时事件使 Query 重新获取数据。页面 effect 无条件把 `squad.instructions` 写入本地 `value`。编辑器的 dirty guard 仅保护尚未交给 `onUpdate` 的输入，debounce 已结束的本地草稿会被替换，未保存提示也可能消失。

**建议：** 分离最后确认的服务器快照与本地草稿；本地干净时才采纳远端更新，脏状态下保留内容并显示冲突。参考已有 `agents/components/tabs/instructions-tab.tsx:86` 的 `persistedRef/localRef` 逻辑。回归覆盖“编辑 → debounce → 远端更新 → 草稿仍在”，以及保存失败重试。**命令：** `$impeccable harden`。**标准：** 数据保留/编辑恢复契约，不直接映射 WCAG 条款。

### F02 · 收起任务属性栏后，隐藏控件仍进入 Tab 顺序

**类别：** 可访问性。**证据：** 原生实机与源码确认。

**位置：** `packages/views/layout/animated-right-sidebar.tsx:134`；任务调用位于 `issues/components/issue-detail.tsx:3722`，项目详情也复用该组件（`projects/components/project-detail.tsx:666`）。

**复现：** 打开任务详情，收起右侧栏；截图显示属性栏已经消失，但 AX 树保留属性、状态、负责人等控件。从开关连续按 12 次 Tab，焦点落到不可见的“属性”按钮。

**原因与影响：** 关闭仅设置 `opacity: 0`、位移与 `pointer-events-none`。这些都不禁止键盘聚焦，也不从辅助功能树隐藏内容，用户会丢失可见焦点并可能操作看不见的字段。项目详情是源码上的同类受影响路径，未单独做原生复现。

**建议：** 保留组件状态的同时，在关闭时正确应用 `inert`/隐藏语义，处理可能留在 body 的门户，并在需要时把焦点交回开关。复用已有隐藏聊天窗口的焦点所有权契约。**标准：** WCAG 2.4.7 Focus Visible、2.4.3 Focus Order。**命令：** `$impeccable harden`。

### F03 · 小队详情与创建把读取失败当作持续加载或没有资源

**类别：** 实现完整性。**证据：** 源码分支确认，未进行网络故障注入。

**位置：** `squads/components/squad-detail-page.tsx:75,81,185`；`squads/create/manual-create-squad-page.tsx:62,292`；`squads/create/template-create-squad-page.tsx:56,214`。

**触发与影响：** 详情初次请求失败或返回 403/404 后，`if (!squad)` 仍一直渲染骨架，没有错误和重试。成员查询失败被默认成 `[]`，显示零成员。手动创建的候选智能体在加载/失败时显示“没有可用智能体”；模板创建的运行时请求失败显示“无可用运行环境”。用户无法判断应等待、重试还是重新创建资源。

**建议：** 独立处理 pending、初次错误、真实空数据、缓存刷新失败；提供局部 Retry，保留成功数据和输入草稿，并让明确的权限失效/404 走正确分支。可复用快速创建候选列表和目录页已有模式。**标准：** 项目 collection recovery 契约。**命令：** `$impeccable harden`。

### F04 · 属性选择器和小队成员多选只在视觉上表达选中状态

**类别：** 可访问性。**证据：** 状态选择器原生 AX 与源码确认；其他调用为源码确认。

**位置：** `issues/components/pickers/property-picker.tsx:233,355`；`status-picker.tsx:99`、`priority-picker.tsx:98`、`assignee-picker.tsx:186`、`label-picker.tsx:266`；`squads/create/manual-create-squad-page.tsx:533`。

**触发与影响：** 已选状态仅靠被 `aria-hidden` 的勾号表达。相关 `PickerItem` 未传 `optionId` 或 `toggle`，不输出 `aria-selected`/`aria-pressed`。原生状态弹层中，所有选项均是普通按钮，当前“待办”没有选中语义。搜索方向键的高亮只改变样式，相关调用也没有提供 `listboxId` 来关联 `aria-activedescendant`。读屏用户无法可靠获知将提交哪项或哪些标签已选择。

小队创建进一步把普通按钮放入 `role="listbox"`，却没有 option 子项或多选声明。原生按钮仍可通过 Tab 操作，因此不能称为“完全不支持键盘”。

**建议：** 单选完整接入 listbox/option 和活动候选关联；多选使用合适原语或明确 pressed/selected 状态，保留搜索和 IME 规则。**标准：** WCAG 4.1.2、1.3.1，ARIA listbox 结构。**命令：** `$impeccable harden`。

### F05 · 描述、评论、回复编辑器缺少稳定的可访问名称

**类别：** 可访问性。**证据：** 源码确认；原生描述与激活后的评论字段辅助观察。

**位置：** `issues/components/issue-detail.tsx:3194`；`comment-input.tsx:220`；`reply-input.tsx:242`；底层 `editor/content-editor.tsx:696`。

**触发与影响：** 三个调用仅传 placeholder，不传 `aria-label`/`aria-labelledby`。底层输出 `role="textbox"`，但不会把 placeholder 自动转为名称。描述 AX 中只有内容值；评论激活后显示提示文字，但没有字段身份的显式关联。原先有名字的静态评论/回复按钮在替换为编辑器后失去稳定标签，尤其有内容后难以区分描述、评论和回复。

**建议：** 为实际 Tiptap 编辑器传本地化字段名；回复名称包含线程或对象上下文。断言真实编辑控件名称，不能只检查静态入口。快速创建的“任务描述”已经正确实现，可作为参考。标题编辑器有自己的名称回退，不纳入此项。**标准：** WCAG 4.1.2、3.3.2。**命令：** `$impeccable harden`。

### F06 · 深色主题品牌实心按钮文字对比度约 3.11:1

**类别：** 主题/可访问性。**证据：** 当前 token 的 WCAG 相对亮度计算；未做深色屏幕取色。

**位置：** `packages/ui/styles/tokens.css:325`；`packages/ui/components/ui/button.tsx:30`；`issues/components/workspace-agent-working-chip.tsx:48`；`issues/components/issues-header.tsx:1876`。

**触发与影响：** 深色主题选中“智能体工作中”筛选时，品牌底色约为 `rgb(67,144,238)`，前景约为 `rgb(250,250,250)`，计算对比度 **3.113:1**。这是 13px 左右的普通 UI 文字，低于 **4.5:1**。同一底色上的硬编码白字约 **3.25:1**，也不足。浅色品牌对比度约 **4.68:1**，不能用浅色通过推导深色通过。

**建议：** 为品牌实心控件建立合格的前景/背景组合，覆盖默认、hover、active 和 expanded；移除该处独立 `text-white` 配对。现有 `text-contrast.test.ts` 主要覆盖 muted/faint 文本阶，`brand-variant-cascade.test.ts` 覆盖级联而非数值对比度，需要补相应配对验证。数据见 [contrast.json](./contrast.json)。**标准：** WCAG 1.4.3 Contrast Minimum。**命令：** `$impeccable colorize`。

### F07 · 折叠区和小队详情页签缺少状态语义

**类别：** 可访问性。**证据：** 源码及原生 AX 确认。

**位置：** `issues/components/issue-detail.tsx:2406,2613,2654,2677,3332`；`squads/components/squad-detail-page.tsx:973`。

**触发与影响：** 任务的属性、父任务、PR、详情、子任务折叠按钮只改变布尔值和箭头，没有 `aria-expanded`。小队详情“成员/指引”只呈现普通按钮，当前项仅用边线/颜色表示，没有 selected/pressed 或面板关联。读屏用户无法知道当前展开内容或所处页签。小队目录已经使用正确 tab group，但详情没有沿用。

**建议：** 折叠按钮公开 `aria-expanded`，用稳定 ID 关联区域；页签复用 Base UI Tabs，保留已有未保存确认及手动激活策略。**标准：** WCAG 4.1.2、1.3.1；`aria-controls` 是建议补充，不能将它单独缺失一概判成违规。**命令：** `$impeccable harden`。

## P2：下一轮处理

### F08 · 无管理权限的小队指令仍可通过键盘编辑

**类别：** 实现完整性/可访问性。**证据：** 源码确认，未切换用户身份现场复现。

**位置：** `squads/components/squad-detail-page.tsx:1325`。

无管理权限时只在外层加 `pointer-events-none`/`aria-disabled`，内部仍挂载可编辑的 `ContentEditor`。空 `onUpdate` 不会阻止编辑器自身文档变化，Tab 到编辑器后可输入，却没有保存入口。造成鼠标与键盘体验不一致、误以为可修改；**不是服务器权限绕过**。

**建议：** 使用现有 `ReadonlyContent` 呈现只读指令，保持选择/复制能力。**标准：** 正确的只读交互契约。**命令：** `$impeccable harden`。

### F09 · 负责人选择器的冷加载失败没有局部恢复入口

**类别：** 实现完整性。**证据：** 源码确认。

**位置：** `issues/components/pickers/assignee-picker.tsx:113,300`。

成员、智能体、小队查询只读取 `data = []`，忽略 pending/error/refetch。无缓存时失败或未加载完成，用户只看到“未分配”，输入已知姓名后可能收到无结果。已有合法缓存的刷新失败不一定为空，问题范围限于冷读取/失败组。

**建议：** 按候选组展示加载和失败，提供 Retry，保留搜索词及成功组；频率排序失败不必阻断分配。**标准：** collection recovery 契约。**命令：** `$impeccable harden`。

### F10 · 共用页头操作在触控场景仍只有 32px

**类别：** 响应式。**证据：** 样式与调用源码确认，未测手机真实命中区域。

**位置：** `layout/collection-page.tsx:143`；调用包括 `agents/components/agents-page.tsx:291,327`、`projects/components/projects-page.tsx:964`、`runtimes/components/runtimes-page.tsx:415`。

共用按钮固定 `h-8 w-8`，没有 coarse-pointer 的最小目标扩展或更大的伪元素命中区。智能体创建、审批、新项目、连接运行时等高频操作在窄屏仍是 32×32 CSS px，增加触控误触和精细定位负担。

**建议：** 统一在紧凑/粗指针环境至少 44×44，并保留桌面密度。**标准：** 本次 Impeccable 44px 触控目标与 WCAG 2.5.5 AAA；**32px 不应直接被说成违反 WCAG 2.5.8 AA 的 24px 要求**。**命令：** `$impeccable adapt`。

### F11 · 减少动态效果只关闭侧栏宽度过渡，没有关闭内容弹簧位移

**类别：** 可访问性/动态效果。**证据：** 源码确认，未切换系统偏好现场复现。

**位置：** `layout/animated-right-sidebar.tsx:135,142`；`packages/ui/styles/base.css:386`。

CSS 在 `prefers-reduced-motion: reduce` 下移除了外层 flex-grow transition，但内部 `motion.div` 仍根据 `motionEnabled` 使用 spring 对 `x` 做 12px 位移动画。组件没有读取 reduced-motion，未发现上层 MotionConfig 统一处理。偏好开启时，宽度直接变化、内容仍滑入，行为不一致。

**建议：** 明确读取减少动态效果偏好，在该模式下让内容位置直接到最终状态，同时保留清晰的状态反馈；不能只覆盖 CSS 宽度，也不宜全局把动画压成 0.01ms。**标准：** 用户动态效果偏好、WCAG 2.3.3 AAA；不将短距离位移一概列为 AA 失败。**命令：** `$impeccable animate`。

## 待实测风险：未计入上述 11 项

1. **AI 创建工作台窄屏布局。** `agents/create/builder-workspace.tsx:298` 固定横向，聊天最小 30%、配置最小 340px。容器小于约 486px 时两侧约束不能同时满足；需在 375/390/450px 与缩放下实测库的最终布局、操作可达性。推荐验证后使用 `$impeccable adapt`。
2. **任务批量工具条溢出。** `issues/components/batch-action-toolbar.tsx:226` 是单行 flex，多个操作不收缩，外层 max-width 没有换行/滚动策略。需在英文、迭代操作出现且 390/450px 时测按钮矩形，不能仅凭类名宣布已发生溢出。推荐验证后使用 `$impeccable adapt`。

## 检测器结果与排除项

执行 `impeccable detect --json packages/views packages/ui apps/web apps/desktop/src/renderer`，退出码 2，原始记录见 [detector.json](./detector.json)。共 8 条主要命中、1 条 advisory：

- 6 条 broken-image 均是测试名称/说明字符串里的字面量 `<img>`，不是产品 DOM 中损坏的图片。
- 2 条 side-tab 来自 Markdown blockquote 的传统引用边线，不是无意义的装饰卡片。
- 1 条 bounce-easing 是引导完成标记的短动画，有明确状态含义且 reduced-motion 已关闭；未据此提出缺陷。

上述 9 条均未计入确认缺陷。仓库缺少 Impeccable PRODUCT.md/DESIGN.md，但现有 token、组件、spec 是有效的视觉依据；文档缺失本身不作为 UI 故障，也未在审计中生成新视觉规范。

## 值得保留的实现

- 快速创建已有持续可见字段标签、明确的阻塞原因、输入焦点引导、上传门禁及紧凑/粗指针目标；原生 AX 能正确读到“任务描述”。
- 智能体目录、任务详情路由和多处 collection recovery 已区分临时失败、真实空列表与权限失效，适合推广到小队详情和旧选择器。
- 任务列表/看板/泳道有拖拽之外的上下移动入口和播报；表格有键盘排序配置；不能将其误报成鼠标专用。
- 虚拟列表、看板按规模切换虚拟化、Gantt 有界日期/行渲染是已有性能资产。缺少性能录制时不提出“必须加 memo”之类泛化问题。
- 统一字号角色、共享 Web/Desktop token、低调的中性色阶和清晰业务分组保持了工作型产品的一致性。普通 muted 文本已有数值对比度回归。
- 隐藏聊天窗口的 inert、门户清理、焦点所有权和草稿保留已有契约；适合修复 F02，而非另造不一致实现。

## 验证结果

本轮执行的现有测试全部通过：**7 个测试文件，113 项测试，0 失败**。

```text
pnpm -C apps/web exec vitest run \
  app/text-contrast.test.ts app/type-scale.test.ts \
  app/brand-variant-cascade.test.ts --reporter=dot
# 3 files, 50 tests passed

pnpm -C packages/views exec vitest run \
  layout/animated-right-sidebar.test.tsx \
  issues/components/pickers/property-picker.test.tsx \
  issues/components/pickers/status-picker.test.tsx \
  issues/components/comment-composers.test.tsx --reporter=dot
# 4 files, 63 tests passed
```

views 测试有 React `act` 和 `workspaces/list` mock 返回 undefined 警告；pnpm 也提示当前版本忽略 package.json 中的部分 pnpm 配置。本轮没有修改工具链或 mock。通过仅说明这些现有断言通过，不能推翻未被覆盖的隐藏焦点、字段命名、品牌配色或并发草稿问题。

未运行全仓 lint/typecheck/Go/E2E：本轮未修改产品代码，仅做审计与针对性验证。没有新增测试、依赖或提交。

## 建议执行顺序

1. **P1 — `$impeccable harden`：** 先处理 F01 草稿、F02 隐藏焦点与 F03 失败恢复，再统一 F04/F05/F07 的状态语义。补真实编辑器及并发/焦点回归。
2. **P1 — `$impeccable colorize`：** 修复 F06 的深色品牌前景/背景组合，并覆盖交互状态。
3. **P2 — `$impeccable harden`：** 处理 F08 只读展示和 F09 负责人读取恢复。
4. **P2 — `$impeccable adapt`：** 扩大 F10 触控区域，并实测两个待确认窄屏风险。
5. **P2 — `$impeccable animate`：** 让 F11 的减少动态效果覆盖 CSS 与 Motion 两个动画所有者。
6. **`$impeccable polish`：** 功能和状态验证完成后再做最后一轮视觉统一。

这些命令可以逐项执行、一起执行或按优先级调整顺序。修复后重新运行 `$impeccable audit`，用同样的证据标准检查剩余问题与评分变化。

本轮新增文件仅为本目录的审计报告、检测结果、颜色计算及证据摘要；产品文件未改，未创建 Trellis 任务，未做代码简化。所有列出的问题均仍待修复。
