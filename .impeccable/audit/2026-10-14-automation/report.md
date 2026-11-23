# 自动化模板页审计

审计日期：2026-10-14

模式：impeccable audit；只读审计，未修改产品代码。

范围：用户截图中的空白自动化列表、模板目录，以及选择模板、查看配置、返回目录这段流程。

## 实现一致性结论

**通过。** 这个页面沿用了 Multica 的字体、表面色、分类色和路由适配器，结构适合操作型产品。卡片有明确的任务场景、周期和产出信息；三个文件的 impeccable detector 结果为 `[]`。

主要问题是浅色键盘焦点对比度，以及首次使用时的信息安排。静态检测没有发现问题，不代表交互和信息层级已达到要求。

## 评分与优先级

| 维度 | 评分 | 依据 |
| --- | ---: | --- |
| 无障碍 | 2/4 | 控件语义、正文对比度和基础键盘操作完整；浅色焦点圈不足 3:1，返回缺少原卡片焦点恢复 |
| 性能 | 3/4 | 10 张卡片、简单筛选、模板缓存 30 分钟；无明显静态性能问题，未测生产包或生产 Web Vitals |
| 主题 | 3/4 | 浅色和深色使用语义 token；浅色 focus token 对比度需要修正 |
| 响应式 | 3/4 | 三列、两列、单列均无目录横向溢出；窄屏空白创建入口仅 32 × 32px |
| 实现一致性 | 4/4 | 共享目录、分类色、状态和平台边界一致；detector 无命中 |
| **合计** | **15/20** | **良好；优先处理无障碍与首次使用引导** |

共 **7 项：P0 0、P1 1、P2 6、P3 0**。A01、A02、A07 有运行时验证；A03–A06 是结合截图与源码的设计判断，尚未通过用户可用性测试量化。

## 详细发现

### A01 · P1 · 浅色卡片焦点圈对比度不足

- 类别：无障碍 / 主题。
- 位置：[卡片焦点样式](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/autopilots/components/autopilot-template-catalog.tsx:202)、[浅色 ring token](/Volumes/artisan/code/2026/multica-0.4.37/packages/ui/styles/tokens.css:206)。
- 证据：Tab 进入第一张卡片后，`:focus-visible` 为 true，outline 为 none，唯一新增焦点指示使用 `--ring`。实测颜色约为 RGB(159,159,169)，对白色间隔对比度 **2.62:1**，低于 WCAG 1.4.11 非文本状态指示的 3:1 要求。深色卡片对应值为 **3.67:1**。
- 影响：键盘用户和低视力用户难以快速辨认当前卡片。
- 建议：使用在相邻表面上达到至少 3:1 的语义焦点色，保留现有环宽和间隔。若修改共享 `--ring`，同时检查其他控件的透明度叠加和两个主题。
- 建议命令：`$impeccable harden`。
- 证据文件：[焦点截图](focus-light.png)、[实际焦点样式](focus-evidence.json)、[对比度与视口测量](responsive-theme-metrics.json)。

### A02 · P2 · 返回目录保留了浏览位置，但没有恢复原卡片焦点

- 类别：无障碍 / 交互连续性。
- 位置：[选择时只保存分类和滚动](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/autopilots/components/autopilot-template-catalog.tsx:194)、[配置页返回](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/autopilots/components/template-create-autopilot-page.tsx:118)。
- 复现：在 886 × 540 视口选择“仓库维护”，聚焦“文档同步检查”，按 Enter 进入配置，再用键盘激活“返回”。
- 证据：返回前后分类都是“仓库维护 4”，滚动位置都是 **168px**；原卡片没有重新获得焦点，下一次 Tab 落在“从空白开始”。鼠标返回的另一条路径中，焦点曾落到 body。
- 影响：连续用键盘比较模板时，需要重新寻找刚才的卡片。
- 建议：把选中模板 key 作为返回焦点目标；数据加载并完成滚动恢复后恢复到该卡片。模板已不存在时回退到分类控件。
- 标准：关联 WCAG 2.4.3 的焦点顺序与可预测性；此处按操作成本增加列 P2，不单独认定为 AA 失败。
- 建议命令：`$impeccable harden`。
- 证据文件：[返回与触摸操作结果](return-focus-and-touch.json)。

### A03 · P2 · “推荐起点”没有形成足够明确的选择引导

- 类别：实现一致性 / 信息层级（设计判断）。
- 位置：[推荐标识及卡片尾部](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/autopilots/components/autopilot-template-catalog.tsx:217)。
- 证据：推荐信息放在描述之后，与周期使用相同的 12px 灰字。第一排卡片实测 **222px** 高，后续卡片 **194px** 高；推荐独占一行让第一排增加 28px，但没有明显提高推荐的辨识度。
- 影响：第一次进入的用户仍要逐张比较，推荐信息提供的决策帮助有限。
- 建议：将现有推荐文案移到标题附近，使用轻量且有文字的标识。调整尾部布局，减少由推荐附加行造成的行高差异；继续保持目录的紧凑和一致。
- 建议命令：`$impeccable layout`。

### A04 · P2 · 周期没有说明是默认计划，时区也没有在目录交代

- 类别：实现一致性 / 信息说明（设计判断）。
- 位置：[周期渲染](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/autopilots/components/autopilot-template-catalog.tsx:221)、[配置页已有修改说明](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/autopilots/components/template-create-autopilot-page.tsx:442)。
- 证据：卡片直接显示“09:00 · 周一至周五”等周期；目录没有“默认”说明或时区信息。当前实现的计划由服务端模板决定，创建时只选择时区，计划要在创建后修改。
- 影响：用户可能把时间理解为必须接受的固定条件，或到下一步才发现本次不能修改计划。
- 建议：在目录标题下补充一条公共说明，例如“以下为默认计划，下一步选择时区，创建后可调整计划”。文案须保留当前的服务端模板契约。
- 建议命令：`$impeccable clarify`。

### A05 · P2 · “按需跟进”与“每次创建任务”的比较维度不一致

- 类别：实现一致性 / 产出说明（设计判断）。
- 位置：[产出模式映射](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/autopilots/components/autopilot-template-catalog.tsx:183)、[中文产出标签](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/locales/zh-Hans/autopilots.json:474)。
- 证据：一个标签描述跟进方式，另一个标签描述任务是否创建。已有契约说明：`run_only` 不预先创建任务，部分巡检仍可创建或更新后续任务。
- 影响：用户不容易判断启用模板后会不会新增任务，以及何时产生后续动作。
- 建议：统一围绕“运行后会发生什么”说明产出；为巡检类补充条件和可能的后续任务行为，以各模板真实行为为准。
- 建议命令：`$impeccable clarify`。

### A06 · P2 · 可修改性和执行者要求放在全部卡片之后

- 类别：实现一致性 / 首次使用引导（设计判断）。
- 位置：[目录底部提示](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/autopilots/components/autopilot-template-catalog.tsx:238)、[提示文案](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/locales/zh-Hans/autopilots.json:476)。
- 证据：“所有模板均可在创建后修改。运行前需指定智能体或AI小队。”位于 10 张卡片之后；1771 × 1080 的实测窗口中，提示顶部在 **y=1144px**，第一屏不可见。
- 影响：影响选择的关键信息要滚到目录末尾才能完整看到；窄屏上距离更长。
- 建议：把简短提示并入标题说明，交代“选择模板 → 指定智能体或 AI 小队 → 创建并启用”。保留配置页的具体校验。
- 建议命令：`$impeccable onboard` 或 `$impeccable clarify`。

### A07 · P2 · 窄屏空白创建入口只剩 32px 的加号

- 类别：响应式 / 操作可发现性。
- 位置：[共享页头操作](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/layout/collection-page.tsx:143)、[空列表仅使用页头入口](/Volumes/artisan/code/2026/multica-0.4.37/packages/views/autopilots/components/autopilots-page.tsx:674)。
- 证据：390px 模拟视口中，“从空白开始”的可见文字被隐藏，按钮为 **32 × 32px**，位于页面右上角。它有正确的 aria-label，基础点击可用。
- 影响：触摸用户的点击余量偏小，首次使用者难以从单独的加号判断它通向空白创建。
- 建议：窄容器保留可读的“从空白开始”次级入口，并将触摸区域提高到至少 44px。
- 标准：44px 属于 WCAG 2.5.5 的增强目标；32px 本身不能直接认定违反 WCAG 2.5.8 的 24px AA 最小目标要求，因此列 P2。
- 建议命令：`$impeccable adapt`。
- 证据文件：[窄屏截图](narrow-light.png)、[尺寸测量](responsive-theme-metrics.json)。

## 已验证的优点与不计入问题的项目

- 正文对比度：浅色 **5.91:1**，深色 **6.75:1**，满足普通文本 AA 的 4.5:1 要求。字体较小是密度选择，不能据此报告对比度不合格。
- 卡片整体是原生 button，支持 Tab / Enter；右下角“查看模板”并非唯一点击热区。
- 分类控件有分组名称和 aria-pressed，筛选数量实测 **4 / 4 / 2 / 10**，选中样式不会被 hover 覆盖。
- 目录使用容器查询；实测三列、两列、单列均没有横向溢出。390px 下分类按钮高 44px，合成触摸可以正确切换。
- 分类和 168px 滚动位置在配置页返回后正确恢复。
- 选择模板只进入配置，缺少执行者时“创建并启用”禁用。
- 三类图标色来自已有语义 token；这种操作型目录使用一致卡片是合理的，没有理由仅因卡片重复而重做整页。
- 用户截图中“每周进展报告”的深边框符合 hover 样式；当前目录没有持久卡片选中状态，不将它作为“选中状态丢失”报告。
- 代码区分加载、失败、空列表和未知模板，并保留重试和空白创建路径。

## 验证记录

- 使用现有真实 Electron preload、共享 React 页面、桌面路由和本地只读 API，在独立临时配置目录中审计。
- 引擎：Electron **39.8.7** / Chromium **142.0.7444.265**。
- 视口：1771 × 1080、886 × 540、390 × 844；浅色和深色；窄屏使用 CDP 视口模拟。
- 触摸证据：Chromium 的 `Input.dispatchTouchEvent` 合成一次分类点击。未用实体触摸设备，未审计独立 Expo 移动应用。
- 对比度按稳定后的实际计算样式转换为 sRGB 测量；主题过渡中的中间颜色已排除。
- impeccable detector 扫描目录、列表页和模板创建页，结果 **0 项**。
- 现有模板测试：**2 个文件、23 项测试通过**。
- 测试命令：`pnpm --filter @multica/views test autopilots/components/autopilot-template-catalog.test.tsx autopilots/components/template-create-autopilot-page.test.tsx`。
- 未执行真实创建、启用或智能体任务；未测生产性能、实体设备、独立 Web 平台和屏幕阅读器完整朗读。紧凑视口不是实际 200% 浏览器缩放测试。
- 产品代码没有变更；没有运行全仓 lint、typecheck 或后端测试。本次发现仍待修复。

## 建议执行顺序

1. `$impeccable harden`：修复浅色焦点对比度及返回焦点恢复。
2. `$impeccable clarify`：说明默认周期、时区、后续任务行为；前移执行者和可修改性提示。
3. `$impeccable layout`：提高推荐标识的辨识度，整理卡片内的信息层级。
4. `$impeccable adapt`：改善窄屏空白创建入口。
5. `$impeccable audit`：复核修复结果和评分。
6. `$impeccable polish`：完成最后一轮细节检查。

这些命令可以逐项或合并执行。本轮交付为审计报告与证据，没有实施上述建议。
