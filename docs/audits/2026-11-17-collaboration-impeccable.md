# 协作空间 Impeccable 审计与修复验收条件

审计日期：2026-11-17。范围为账号级 `/account` 协作空间：当前处理、我参与的、任务详情、创建、工作流、通知、上下文与知识资料。依据当前工作区源码、实际 Electron 界面，以及加载原始共享组件和 CSS 的隔离 Chromium 夹具。运行分支为 `codex/cross-workspace-collaboration`；工作区存在其他进行中的修改，审计没有修改业务实现。

## 实现一致性结论

**产品表达一致，通过；交互完整性存在需要修复的缺口。** 标题、处理责任、角色动作、交付结果与讨论构成清晰的任务流程。语义 token、共享 Web/Desktop 组件和现有权限边界一致。机械扫描 57 个文件得到 **0 个主要告警、1 个建议级提示**；提示来自 `base.css:101` 的新手引导完成动画，协作页面没有使用该动画，因此不计入协作缺陷。扫描不能验证搜索是否生效、辅助技术状态、刷新时的光标以及长内容弹窗几何。

## 审计健康评分

| 维度 | 分数 | 主要依据 |
| --- | ---: | --- |
| 可访问性 | 2/4 | 多选状态、具体输入名称和定时刷新时的光标连续性存在缺口 |
| 性能 | 3/4 | 主要数据有分页和延迟读取，但隐藏的历史页、知识候选仍可周期读取 |
| 主题 | 4/4 | 使用共享语义 token；所核对的次要文字背景组合在明暗主题均超过 4.5:1 |
| 响应式 | 2/4 | 主框架和资料抽屉有响应式边界；清理弹窗长理由越界，部分触摸目标只有 32px |
| 实现完整性 | 2/4 | 搜索显示与提交结果不一致，错误恢复与字段定位仍有缺口 |
| **合计** | **13/20** | **可用，需完成重要交互与无障碍整改** |

评分只代表本次审计范围，不是全产品的 WCAG 认证或生产性能验收。共确认 **9 项：P0 0 项、P1 5 项、P2 4 项、P3 0 项**。

## P1：发布前修复

### C01 — 自定义下拉搜索没有过滤选项

- **位置：** `packages/views/issues/components/pickers/custom-property-picker.tsx:211`、`:247`；协作入口 `packages/views/collaboration/field-inputs.tsx:59`。
- **分类：** 实现完整性。
- **触发及影响：** 单选或多选字段超过 7 个选项后显示搜索框，但没有给调用方接入查询和过滤。隔离浏览器中输入 `Choice 10` 后仍保留所有选项，按 Enter 实际写入 `choice-1`，可能导致错误的任务属性。
- **修复建议：** 接入本地过滤和无匹配提示，保持清空行的键盘规则，选中值应与实际可见匹配一致。建议 `$impeccable harden`。
- **回归条件：** 8 项以上单选和多选；精确、部分、无匹配搜索；键盘 Enter 不选择被过滤掉的选项；清空值、关闭重开、既有已选值保留。

### C02 — 标签和自定义多选项没有可访问的选中状态

- **位置：** `packages/views/collaboration/field-inputs.tsx:24`；`packages/views/issues/components/pickers/property-picker.tsx:356`。
- **分类：** 可访问性，WCAG 4.1.2（名称、角色、值）。
- **触发及影响：** 未传 `optionId` 的 `PickerItem` 只输出普通按钮，勾选图标又对辅助技术隐藏。已选标签在 Chromium AX 中只显示 `button "Design"`，没有 `aria-selected` 或 `aria-pressed`。读屏用户无法确认切换后的状态。
- **修复建议：** 采用与交互一致的可访问选择状态，复用已有原语；保持清空动作的合理语义。建议 `$impeccable harden`。
- **回归条件：** 初始已选、点击选中、取消选中均向辅助技术暴露当前状态；账号选择器原有 listbox 语义与键盘行为保持正常。

### C03 — 自定义文本、数值和 URL 的实际输入缺少字段名称

- **位置：** `packages/views/issues/components/pickers/custom-property-picker.tsx:437`；协作触发器 `packages/views/collaboration/field-inputs.tsx:56`。
- **分类：** 可访问性，WCAG 1.3.1、3.3.2。
- **触发及影响：** 外层按钮有字段名称，打开后聚焦的输入没有关联该名称。浏览器中“Acceptance criterion”的输入 AX 名称退化为通用占位符 `Enter value…`，按真实字段名称查找输入得到 0 个结果。
- **修复建议：** 将真实属性名称关联到弹层中的输入，提供持久可见的上下文。建议 `$impeccable harden`。
- **回归条件：** 三种字段类型均可用真实属性名定位；数值格式、URL 打开和 Enter 提交行为保持正常。

### C04 — 知识审阅的定时刷新中断理由输入

- **位置：** `packages/views/collaboration/knowledge-review.tsx:97`、`:114`；`packages/core/collaboration/knowledge-queries.ts:23`；`knowledge-cache.ts:61`。
- **分类：** 可访问性与交互完整性。
- **触发及影响：** 候选每 15 秒重读，授权投影先被清除，理由输入框随 `return null` 从 DOM 移除，返回后重建。文字与勾选值保留，焦点及选区不保留，连续输入因此被打断。现有测试明确验证过 DOM 消失与文字恢复；没有验证光标连续性。Base UI 可能将焦点回收到弹窗容器，不应表述为必然逃出焦点陷阱。
- **修复建议：** 保留同一审核身份下客户端理由编辑器的 DOM，续期期间锁定提交并继续清除受保护材料；或在同身份、仍 active、续期成功时精确恢复原焦点与选区。建议 `$impeccable harden`。
- **回归条件：** 同一候选的延迟刷新保持光标和选区；切换候选不混用草稿；失去权限仍清除材料及相应交互；隐藏面板不恢复焦点。

### C05 — 合法长理由使清理上下文弹窗超出可视区

- **位置：** `packages/views/collaboration/memory-panel.tsx:218`；`packages/ui/components/ui/alert-dialog.tsx:55`；`textarea.tsx:10`。
- **分类：** 响应式与可访问性，涉及 WCAG 1.4.10 的内容可达性。
- **触发及影响：** 输入 1,000 UTF-8 字节、40 行的合法理由。固定居中弹窗没有高度或滚动边界，`field-sizing: content` 使其扩展。1280×800 下弹窗高 1041px，按钮顶部为 872.5px；320×568 下高 1245px，确认/取消顶部为 818.5/858.5px。两主题均复现，按钮位于可视区外。
- **修复建议：** 为该确认弹窗和理由编辑区设置合理视口高度、内部滚动与操作区布局，使取消和确认可到达。优先在该调用处修复，避免无关 AlertDialog 改动。建议 `$impeccable adapt`。
- **回归条件：** 桌面、320px 窄屏、矮窗口和放大文字时，合法多行内容可编辑，标题与两项操作可访问；保持原确认和请求语义。

## P2：同批修复

### C06 — 初始字段校验没有定位真实错误

- **位置：** `packages/views/collaboration/creation-form.tsx:254`、`:368`；`field-inputs.tsx:145`。
- **分类：** 错误信息与可访问性，WCAG 3.3.1。
- **触发及影响：** 阶段编号为 0、小数或超出上限时，统一显示“部分所选值已不可用”，创建按钮禁用，未关联实际错误字段。用户必须逐个排查，格式错误被解释成目录失效。
- **修复建议：** 保留校验路径，显示并关联具体字段原因；目录失效仍使用失效选项提示。建议 `$impeccable clarify`、`$impeccable harden`。
- **回归条件：** 阶段上下界、小数和有效值；字段可访问错误描述；修正后恢复提交；失效标签/属性仍保留正确反馈。

### C07 — 工作流状态候选失败后缺少原地重试

- **位置：** `packages/views/collaboration/workflow.tsx:302`、`:254`；`packages/core/collaboration/queries.ts:46`。
- **分类：** 错误恢复。
- **触发及影响：** `status-options` 网络错误或 5xx 后，确认按钮要求读取成功，而错误组件没有 `retry`。关闭重开是绕路，并可能丢失局部理由等输入；后台窗口重聚焦不是明确的恢复入口。
- **修复建议：** 接入候选 `refetch()` 和可发现的加载状态，原地保留表单。建议 `$impeccable harden`。
- **回归条件：** 失败后可显式重读，同弹窗草稿不丢；读权限拒绝仍执行既有保护边界；读取期间不能提交无效状态。

### C08 — 隐藏的历史与知识候选仍可周期读取

- **位置：** `packages/views/collaboration/detail-auxiliary.tsx:85`；`memory-panel.tsx:60`；`packages/core/collaboration/knowledge-queries.ts:72`。
- **分类：** 性能。
- **触发及影响：** `keepMounted` 保留草稿是正确合同，但 `active` 没有控制所有读取。已加载 N 页上下文历史后，隐藏状态每次 refetch 仍可能重读 N 页；保留的已选知识候选还可每 15 秒重读正文。全局文档保持前台时，`refetchIntervalInBackground: false` 不识别关闭抽屉。根列表和当前上下文的名义两组读取约 8 GET/分钟；此数字来自配置推导，不是生产实测。根读取可能承担工具栏计数与错误提示，不能一概判为无意义。
- **修复建议：** 给不可见历史页和候选正文添加读取门禁，保留挂载、草稿、必要根状态与原请求恢复；保留租约到期擦除，在重开时验证当前授权。建议 `$impeccable optimize`。
- **回归条件：** 关闭/切分类后不续读不必要历史或候选正文；重新打开可恢复读取；隐藏期间到期仍擦除私有投影；计数、错误和待确认请求不被误报。

### C09 — 部分字段弹层触摸目标小于 44px

- **位置：** `packages/views/collaboration/field-inputs.tsx:25`；`packages/views/issues/components/pickers/property-picker.tsx:364`；`custom-property-picker.tsx:445`。
- **分类：** 响应式与触摸易用性。
- **触发及影响：** 在 Chromium `hasTouch/isMobile` 且 `(pointer: coarse)` 为真的 320px 视口，标签行实测 32px 高，文字输入也只有 32px。相邻行容易误触，与已采用 44px 的账号和父任务选择不一致。
- **修复建议：** 补上粗指针时的最小尺寸，维持桌面紧凑布局。建议 `$impeccable adapt`。
- **标准说明：** 44px 是本次 skill 的触摸建议；32px 本身不等于 WCAG 2.2 AA 的 24px 目标违规。
- **回归条件：** 粗指针时相关行和输入至少 44px；不因长名称产生横向溢出；桌面交互和选择状态保持正常。

## 共性与应保留的实现

共同缺口主要落在共享选择器的真实输入语义、异步读取和客户端输入生命周期之间的边界，以及少数弹窗缺少视口边界。修复应复用已有原语并补实际行为测试，避免再建一套选择器或状态模型。

以下实现应保留：

- 账号选择器完整身份、listbox/option 状态、IME、延迟搜索和焦点处理。
- 任务要求 → 交付结果 → 讨论的层级，按角色确定主动作。
- 辅助抽屉 `hidden` + `inert`、保留挂载、嵌套弹层 active 门禁和关闭完成后的焦点交接。
- 原请求身份恢复、上传取消与草稿保留；拒绝访问时清除受保护内容。
- 分页与延迟运行记录读取，以及语义颜色、角色字体 token。

主题数值核对：明亮主题次要文字在页面/浮层/选中背景的对比度约 5.09–5.89:1；最严格的已检查侧栏状态为 4.866:1。深色主题所核对组合为 5.191–7.155:1。标签配置颜色属于业务数据，不算页面硬编码主题缺陷。

## 建议执行顺序

1. **P1 `$impeccable harden`：** 修复 C01–C04，先补会暴露当前缺口的组件/生命周期测试，再调整实现。
2. **P1/P2 `$impeccable adapt`：** 修复 C05、C09，使用真实组件检查明暗主题、粗指针、窄屏和长内容。
3. **P2 `$impeccable clarify` + `$impeccable harden`：** 修复 C06、C07，验证原地恢复与可定位错误。
4. **P2 `$impeccable optimize`：** 修复 C08 的不必要隐藏读取，同时保护草稿、租约和工具栏状态。
5. **`$impeccable polish`：** 一次有界的综合视觉与交互确认，然后重跑 `$impeccable audit`。

## 已执行验证与边界

- 子审计执行了 `pnpm --dir packages/views exec vitest run collaboration/account-picker.test.tsx collaboration/creation-form.test.tsx collaboration/creation-description-assist.test.tsx collaboration/field-inputs.test.tsx collaboration/attachment-picker.test.tsx collaboration/resource-panel.test.tsx`，6 文件、78 测试通过。这些既有测试没有覆盖 C01–C03 的实际行为。
- 知识审阅现有用例 `knowledge-panel.test.tsx -t 'keeps typed decision input and review confirmation'` 1 项通过、41 项跳过；它证明文字恢复，未证明焦点和选区。
- 当前隔离组件夹具构建成功；Chromium 145.0.7632.6 的 6 个检查场景复现 C01/C02/C03/C05/C09，没有页面异常或外部请求。没有点击清理确认或提交业务数据。
- 实际 Electron 已查看列表、待验收详情和通知。后续桌面输入控制未能稳定完成弹层关闭与创建操作；不把该工具症状当作产品缺陷或桌面回归通过。
- 没有独立启动生产 Web；没有物理手机、VoiceOver 全流程、生产包大小、LCP/INP 或大数据量测量。旧截图已核对来源 hash，仅将匹配的 token/共享原语用作参考，未把旧创建截图冒充当前源码的验收。
- 本轮没有业务源码修改，因此没有声称 lint、类型检查、全仓测试或修复后回归通过。

证据目录：`artifacts/qa/collaboration-impeccable-20261117/`。包含 `impeccable-detect.json`、`source-manifest.json`、`token-contrast.json`、真实组件夹具、`browser-audit/results.json` 与截图。`results-unsettled.json` 保留了首轮未等待动画完成的几何；审计数值只使用最终等待字体、有限动画与绘制后的 `results.json`。

## 2026-11-18 修复与回归结果

用户已授权修复与回归，并批准创建 Trellis 任务。**C01–C09 已全部修复并完成本次范围内的验证。** 对应任务为 `11-18-collaboration-impeccable-remediation`。

| 问题 | 修复后的行为 | 验证 |
| --- | --- | --- |
| C01 | 自定义单选、多选按真实查询过滤；无匹配不选择、不清空；重开恢复浏览 | 实际组件测试及 Chromium `Choice 10`/无匹配/Enter 场景 |
| C02 | 可逆多选项暴露 `aria-pressed`；listbox 选项和清空、失效移除命令保持原语义 | 原语、协作标签、自定义多选测试及浏览器 AX 状态 |
| C03 | 文本、数值和 URL 输入有持久的具体字段标签 | 组件名称/格式测试，Chromium 三种输入 Enter 提交 |
| C04 | 同一审核身份的理由输入保留实际 DOM、焦点及选区，续期中仍可输入；当前授权决定动作 | 真实 Query 延迟续期、空焦点输入、拒绝、候选切换测试；Chromium 连续输入与选区 |
| C05 | 上下文确认弹窗有视口边界、独立滚动内容和可达操作区 | 1,000 字节多行理由、两主题、320/390/1280px、420px 矮窗、200% 字号 |
| C06 | 阶段错误关联实际字段，修正恢复提交；父任务缺失的错误草稿仍可直接清空或编辑 | 中英文边界/目录失效组件测试及浏览器创建场景 |
| C07 | 状态候选失败可在同一弹窗重试，保留理由，读取期间禁止提交 | 失败→延迟重试→成功的组件及 Chromium 场景 |
| C08 | 隐藏历史、知识详情及保留提议的可选发现读取停止；保留根状态与租约擦除 | 实际 Query 计时器/失效/重开/可见共享观察者测试，Chromium 隐藏清除与重开 |
| C09 | 粗指针的相关行及输入实测 44px；普通桌面仍为 32px | 编译原始 CSS 的 Chromium 矩形及触摸 `tap()` 操作 |

修复没有引入新依赖或另一套选择器、状态存储、授权机制。字段搜索留在原调用方，知识理由留在客户端输入层，授权材料仍只存在于 Query；清理弹窗在原使用处修正，未改全局 AlertDialog。

综合检查：

- **Views：51 个文件、590 项测试通过**，包含整个协作组件目录和共享选择器兼容测试。
- **Core：7 个文件、56 项测试通过**，覆盖知识租约、缓存、来源、动作、schema 和上下文查询。
- **Chromium：17 个场景通过**，没有失败、页面异常或外部请求。
- **整仓类型检查：9/9 个任务通过**，包括 Web 和 Desktop 的 Node/renderer 检查。
- **整仓 lint：6/6 个任务通过**；仓库其他路径有 31 项既有警告，本任务没有新增告警。
- **Impeccable 修复后扫描：11 个改动实现文件，0 个告警。** 独立 Trellis 检查没有发现未修复的范围内问题。

类型检查还修正了新增测试中的三处不支持的角色查询选项。数字输入的 DOM 测试使用实际表单提交：安装中的 `user-event` 不模拟数值输入的隐式 Enter 提交；真实 Chromium 已单独验证 `1.25` 经 Enter 正确写入。分页测试修正了局部假计时器与 `waitFor` 的观察方式，保留了两页重读和精确请求数断言。

修复后健康评分为 **18/20**：可访问性 3、性能 3、主题 4、响应式 4、实现完整性 4。此分数仍限于本次可测范围：没有执行生产 Web/API 端到端、安装版 Electron Main、物理手机或完整 VoiceOver 口述流程，也没有把模拟接口组件验证当作真实 AI/运行时验收。

修复证据保存在原证据目录的 `verification-summary.json`、`browser-regression/results.json`、截图、`impeccable-detect-after.json` 和 `visual-verdict-after.json`。浏览器构建保留原源码清单；独立检查之后只改变了清单中一个未打包的测试文件，`browser-regression/provenance-review.json` 单独记录该测试修正，所有实现及 CSS 来源 hash 与截图保持一致。Trellis 研究目录记录每组修复前失败、修复后通过以及综合检查的完整命令和日志。
