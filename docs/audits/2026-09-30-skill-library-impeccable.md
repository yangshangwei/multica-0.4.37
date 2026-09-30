# 技能库 Impeccable 审计

修复状态：用户确认后，已完成下列十项修复；文末附最终验证证据。原始评分与发现保留为修复前记录。

日期：2026-09-30。对象：用户截图中的工作空间 skill 卡片页，以及相连的列表视图、分类导航、工具栏和模板入口。模式：Operate；任务目标是快速找到、识别并管理 skill。

本次仅审计，未修改产品代码。依据为用户截图、当前源码、共享设计 token 和 Impeccable 静态检测。未运行应用、读屏软件、移动设备测试或性能基准，因此以下评分是静态审计评价，不是 WCAG 合规认证或运行时性能结论。

## 实现一致性判定

**设计系统一致性通过，交互完整性未通过。** 分类颜色来自语义 token，字号来自统一角色尺度，列表和卡片共享筛选状态，能够体现工作空间、skill、智能体之间的产品关系。需要修正的是键盘主入口、选择控件、筛选状态表达和无效设置。

执行 `node /Users/artisan/.agents/skills/impeccable/scripts/detect.mjs --json packages/views/skills/components`，退出码 0，结果 `[]`。这只表示没有命中该检测器的静态规则；下面的问题来自人工追踪实际实现，并非检测器报告。没有需要剔除的检测器误报。

## 健康评分

| 维度 | 分数 | 依据 |
| --- | --- | --- |
| 可访问性 | 2/4 | 搜索、视图切换已有名称；卡片主入口缺键盘支持，选择控件存在可见性和命名缺口 |
| 性能 | 3/4 | 列表与卡片均虚拟化，派生数据使用 memo；未做运行时或包体测量 |
| 响应式 | 2/4 | 按容器切换侧栏与分类 chips；窄屏隐藏了关键名称和清空筛选入口 |
| 主题 | 3/4 | 共享浅深色 token 完整；复选框低对比，选中筛选使用固定 text-white |
| 实现完整性 | 2/4 | 多分类仍高亮“全部”，卡片模式暴露不生效的列设置，来源图标容易误认 |
| **合计** | **12/20** | **可用，但有明显问题需要修正** |

共 10 项：P0 0、P1 4、P2 6、P3 0。P1 应优先处理；P2 的视觉判断与可确定的交互问题在下文明确区分。

## P1：优先处理

### 1. 卡片和列表行的主要打开方式没有键盘入口

- 位置：`packages/views/skills/components/skill-card.tsx:53`、`packages/views/skills/components/skills-page.tsx:1119`、`packages/views/navigation/use-row-link.ts:72`。
- 类别：可访问性。源码确定。
- 证据：容器是普通 div，标题是普通文本；`useRowLink` 仅返回 click、auxclick、mouseenter，没有链接语义、tabIndex 或键盘处理。
- 影响：Tab 无法聚焦标题并按 Enter 打开详情。更多菜单里的“在新标签页打开”可以作为绕行方式，但不等价于主要导航路径，也更难发现。
- 标准：WCAG 2.1.1 Keyboard。
- 建议：将标题接入现有 `AppLink`，保留整卡鼠标点击与嵌套操作的事件隔离；让主入口有明确焦点。不要把整卡包成包含其他按钮的链接。
- 命令：`$impeccable harden`。

### 2. 列表选择框和全选按钮没有名称，聚焦后仍然透明

- 位置：`packages/views/skills/components/skills-page.tsx:251`、`:558`。
- 类别：可访问性。源码确定。
- 证据：外层按钮只有 `aria-pressed`，没有文字或可访问名称；未选中时 `opacity-0`，仅 group-hover 恢复显示，缺少 focus-visible 对应状态。内层 Checkbox 设置 `tabIndex={-1}`。
- 影响：键盘用户进入不可见的焦点位置，读屏用户无法判断选择的是哪个 skill 或是否为全选。
- 标准：WCAG 2.4.7 Focus Visible、4.1.2 Name, Role, Value。
- 建议：优先直接使用有名称的 Checkbox，让单一交互控件负责状态和事件；提供“选择 {{name}}”“全选”等名称，并确保键盘聚焦时可见，混合选择状态可被表达。
- 命令：`$impeccable harden`。

### 3. 卡片复选框未选中状态几乎不可辨认

- 位置：`packages/views/skills/components/skill-card.tsx:95`、`packages/ui/components/ui/checkbox.tsx:15`；颜色来自 `packages/ui/styles/tokens.css`。
- 类别：可访问性 / 主题。截图可见，token 计算支持。
- 证据：空框使用 `border-input`，卡片将整个控件降到 `opacity-30`。按当前浅色 token 在白色卡片上进行 sRGB 合成，边框约 **1.07:1**；恢复 100% 不透明度仍只有约 **1.27:1**。这是源码颜色推算，不是截图像素或实际 DOM 测量。
- 影响：用户难以发现批量选择入口，尤其在触控或低对比环境。截图右上角的空框正体现这一问题。
- 标准：WCAG 1.4.11 Non-text Contrast 的 3:1 控件辨识要求；最终应以运行时样式复核。
- 建议：去掉未选中状态的过低整体透明度，为控件边界采用满足 3:1 的语义颜色；不要只把 opacity 改成 100% 就结束。若采用显式批量选择模式，入口也必须清晰且支持键盘。
- 命令：`$impeccable harden`，随后 `$impeccable polish`。

### 4. 窄屏筛选按钮丢失功能名称

- 位置：`packages/views/skills/components/skill-list-toolbar.tsx:214`。
- 类别：可访问性 / 响应式。源码确定。
- 证据：按钮没有 aria-label；文字使用 `hidden md:inline`。在 md 以下，无筛选时只剩图标，有筛选时只剩图标与数字。
- 影响：读屏用户无法得知这是筛选按钮，数字也无法解释它代表的状态。
- 标准：WCAG 4.1.2 Name, Role, Value。
- 建议：始终提供本地化名称，例如“筛选，已启用 2 项”；视觉上隐藏文字不应隐藏可访问名称。排序入口也应提供包含当前字段与方向的明确名称，不仅依赖“显示设置”提示。
- 命令：`$impeccable adapt`。

## P2：下一轮修正

### 5. 来源铅笔容易被误认为编辑按钮

- 位置：`packages/views/skills/components/skill-card.tsx:152`、`packages/views/skills/components/skill-list-toolbar.tsx:93`。
- 类别：实现完整性 / 信息表达。源码确定其含义；误认风险来自截图布局与通用图标习惯。
- 证据：右下角是普通 span，`originIcon("manual")` 返回 Pencil；没有来源文字或提示。整卡又有 cursor-pointer 和详情导航行为。
- 影响：用户可能点击铅笔期待进入编辑，却打开详情；辅助技术也无法从这个无文字图标理解来源。
- 建议：使用明确的“手动创建”来源说明，或把来源放到详情和列表中；真实操作放在清晰的操作菜单里。相同来源无需在每张卡片上强调。
- 命令：`$impeccable clarify`。

### 6. 多分类筛选时侧栏错误显示“全部”选中

- 位置：`packages/views/skills/components/skill-category-sidebar.tsx:27`、`:64`；分类菜单位于 `skill-list-toolbar.tsx:290`。
- 类别：实现完整性。源码确定。
- 证据：筛选菜单允许多分类，`activeCategory` 对分类数不等于 1 的情况统一返回 null；null 又使“全部”高亮并带 `aria-current`。实际过滤仍按所选分类执行。
- 影响：导航展示的范围与实际结果不一致，用户容易误以为数据缺失。
- 建议：只有 `filters.categories.length === 0` 才将“全部”标为选中；多选时高亮所有选中分类，或显式呈现“多个分类”状态。侧栏与 chips 应采用相同含义。
- 命令：`$impeccable harden`。

### 7. 卡片模式暴露不生效的“列显示”开关

- 位置：`packages/views/skills/components/skill-list-toolbar.tsx:546`、`packages/views/skills/components/skills-page.tsx:1078`。
- 类别：实现完整性。源码确定。
- 证据：列开关没有以 viewMode 为条件隐藏；hiddenColumns 只影响列表渲染，SkillCardGrid 不接收它。
- 影响：用户改变设置后当前页面没有反馈，却改变了另一种视图的配置。
- 建议：卡片模式仅显示有效的排序设置，列表模式再显示列管理；若确需跨视图配置，应明确说明其作用对象。
- 命令：`$impeccable clarify`。

### 8. 卡片空间与辨识信息分配失衡，长标题的提示也不一致

- 位置：`packages/views/skills/components/skill-card.tsx:22`、`:69`、`:108`、`:115`；虚拟行尺寸契约在 `skill-card-grid.tsx:64`。
- 类别：信息层级 / 响应式。固定尺寸与截断为源码事实；视觉效率为截图判断。
- 证据：卡片固定 172px，高度中始终保留标签区，说明仅两行，标题单行。截图中的“发布后与 Canary …”被截断，说明下方却有明显空白。标题展示 presentation.name，title 属性却使用 skill.name；内置 skill 的提示可能变成内部英文标识，而不是完整中文标题。
- 影响：用于挑选 skill 的名称和用途需要进入详情才能完整辨识，卡片占用的空间却没有增加辨识信息。
- 建议：为标题与简短用途优先分配空间；无标签时不保留无效内容行，必要时采用明确的紧凑卡片规格。可以复用已有的展示摘要能力，但不要改写运行时使用的 description。完整标题提示应与展示名称一致并支持键盘。调整尺寸时同步虚拟列表的估算契约，不能只删除固定高度。
- 命令：`$impeccable layout`，随后 `$impeccable clarify`。

### 9. 清空筛选入口无法用键盘直接操作，窄屏完全隐藏

- 位置：`packages/views/skills/components/skill-list-toolbar.tsx:241`。
- 类别：可访问性 / 响应式。源码确定。
- 证据：唯一 onClearFilters 入口是嵌在主按钮内的 span，`role="button"`、`tabIndex={-1}`、仅 onClick，并且 `hidden md:inline-flex`。
- 影响：鼠标用户可一键清空，键盘和窄屏用户需要重新打开菜单逐项取消；不能发现当前筛选时更难恢复。
- 标准：该快捷操作存在 WCAG 2.1.1 风险；逐项取消仍是功能上的绕行方式。
- 建议：将清空设置为独立可聚焦按钮，或在筛选菜单加入所有输入方式均可到达的“清空筛选”。
- 命令：`$impeccable harden`、`$impeccable adapt`。

### 10. 工作空间无结果状态缺少原因与恢复指引

- 位置：`packages/views/skills/components/skill-card-grid.tsx:73`、`packages/views/skills/components/skills-page.tsx:1110`。
- 类别：实现完整性 / 状态反馈。源码确定。
- 证据：两种工作空间视图都仅输出“无匹配”；已有本地化 query/filter 文案未被使用。模板页则已经提供原因与清空按钮（`skill-library-catalog.tsx:165`）。
- 影响：用户不能就地分辨是搜索词、分类还是其他筛选造成空结果，需要回到工具栏排查；持久化筛选使下次进入仍可能遇到此状态。
- 建议：区分“搜索无结果”和“当前筛选无结果”，显示关键词或筛选摘要，提供“清空搜索 / 清空筛选”；为结果数量变化提供适当的 status 通知。
- 命令：`$impeccable harden`。

## 系统性问题与应保留的做法

问题集中于三个重复模式：鼠标 hover 承担过多交互发现责任；视觉隐藏文字时遗漏辅助技术名称；多处控件操作同一状态却没有一致地表达当前状态。建议优先修正这些模式，再做间距微调。

应保留：

- 克制的白色工作界面和语义分类色，符合操作型页面定位。彩色图标在这里是分类编码，不应仅因样式常见而视为模板化缺陷。
- 统一字号与浅深色 token。当前 muted 正文按 token 计算在白卡上约 5.89:1、页面底色上约 5.70:1；不应仅凭截图偏灰就认定正文对比失败。
- 卡片与列表共享筛选、排序和持久化视图偏好；分类使用容器断点，考虑了桌面分栏。
- 列表和卡片虚拟化，以及成员、运行时、分配关系的 memo 派生。未发现足以在本次范围内单列的性能故障。
- 搜索具有可访问名称，视图切换具有 aria-pressed，主查询错误可重试，辅助查询失败有状态提示。
- 当前 “Skills / skill / 技能库” 命名符合 `apps/docs/content/docs/developers/conventions.zh.mdx:138` 的明确规则，不列为翻译混乱，也不建议本次改名。

## 推荐执行顺序

1. `$impeccable harden`：处理主导航、选择控件、分类状态与清空/空结果行为。
2. `$impeccable adapt`：处理窄屏名称与恢复入口，实测 320/375/768px 和桌面窄分栏；触控设备可扩大当前 24–32px 的小控件。小于 44px 本身不自动等于 WCAG AA 失败。
3. `$impeccable clarify`：明确来源与操作，隐藏不适用于当前视图的设置，修正完整标题提示。
4. `$impeccable layout`：调整卡片标题、用途、标签与底部元数据的空间。建议将三个顶部区域的主次和左右对齐一并复核，而不是简单增加装饰。
5. `$impeccable polish`：在功能修正后统一焦点、对比、间距和可见状态；最后重跑 `$impeccable audit`。

上述命令可以单独执行，也可以合并为一轮修复。

## 验证范围与剩余风险

- 本次变更文件只有这份报告；没有做代码简化或产品实现变更，也未运行构建、lint、单元测试或 E2E。
- 动态焦点、读屏实际朗读、移动触控、暗色所有交互状态、200% 文字缩放和大量 skill 数据下的性能仍需运行时验证。
- 固定 172px 卡片及虚拟行估算对文字缩放存在风险，尚未实测，不作为已确认故障计数。
- 本次分析的是当前源码与用户提供的截图，不假设两者来自完全相同的构建。


## 修复完成与验证记录

本节对应用户后续的“请按照建议修复”。前文 12/20 是修复前的静态评分，不代表当前实现。

- 十项均已处理：真实标题链接、命名且可见的选择控件、较高对比的边界、窄屏名称、文字来源、多分类状态、仅列表列设置、紧凑卡片与展示摘要、菜单清除筛选、共享空结果恢复。
- 删除了选择控件外层的伪按钮、空标签占位和两份重复的无结果 UI。保留一个直接交互的 Checkbox，并在包含隐藏 input 的外层隔离点击与中键事件。列表操作菜单也隔离中键，避免意外打开详情。
- 无标签行 144px、有标签行 168px；虚拟列表的估算和行 key 一并随行内容变化。原始 skill 描述和指令未修改。
- 粗指针设备上的操作菜单常显，点击区域至少 44px。分类导航和工具栏沿用现有视觉与术语。

最终检查：

| 检查 | 结果 |
| --- | --- |
| skills Vitest + locale parity | **27 个文件、454 项通过** |
| views TypeScript | **通过** |
| 受影响 TS/TSX ESLint | **通过** |
| `git diff --check` | **通过** |
| Impeccable detector，7 个产品组件 | **退出 0，结果 `[]`** |
| 真实 Chromium E2E | **1 项通过**，使用真实本地 API 和独立测试工作空间，结束后清理测试数据 |
| 视觉确认 | **92/100，通过**；桌面浅/深色、320/375px 粗指针视口完成两轮有界检查 |

E2E 验证标题 Enter 跳转、Space 选择和全选、分类多选状态、菜单键盘清除、空搜索恢复与焦点、长标题加三个标签的卡片不溢出、窄屏卡片不重叠、操作菜单 44px 且常显，并核对服务端存储的描述未改变。测试菜单时等待焦点转移完成，避免把弹层过渡误认成键盘功能失败。

截图保存在 `.omx/artifacts/skill-library/`，视觉结论保存在 `.omx/state/skill-library/ralph-progress.json`。验证曾遇到本地前后端端口/CORS 不一致；最终通过临时本地转发使用后端已允许的来源完成验证，没有修改产品 CORS 配置。

变更文件：

- 产品：`packages/views/skills/components/skill-card.tsx`、`skill-card-grid.tsx`、`skills-page.tsx`、`skill-list-actions.tsx`、`skill-list-toolbar.tsx`、`skill-category-sidebar.tsx`、`skill-category-chips.tsx`。
- 翻译：`packages/views/locales/en/skills.json`、`packages/views/locales/zh-Hans/skills.json`。
- 单元测试：对应的 `skill-card.test.tsx`、`skills-page.test.tsx`、`skill-list-toolbar.test.tsx`、`skill-category-sidebar.test.tsx`、`skill-category-chips.test.tsx`。
- E2E：`e2e/skill-library-accessibility.spec.ts`、`e2e/skill-category-taxonomy.spec.ts`（同步选择控件和分类语义断言）。
- 文档：本报告、`docs/plans/2026-09-30-skill-library-audit-fixes.md`、`.trellis/spec/views/frontend/skill-presentation.md`。

剩余验证范围：未在真实手机、Safari、读屏软件或 Electron 打包版本实测，也未做大型数据性能基准。完整 skills 测试中已有的详情页 act 提示和 jsdom 导航提示仍可见，但所有测试通过；未为此扩大修改范围。未添加依赖，未更改数据库或 API。
