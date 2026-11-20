# 优化通过智能体创建弹窗的阻塞反馈与一致性

## Goal

让「通过智能体创建」弹窗在不能提交时说清原因并给出下一步，同时消除弹窗内部三处不一致：两个 AI 辅助按钮的行为、「交给谁」一行的层级与键盘路径、「交给」这一动作的用词。

用户价值：智能体模式是默认的新建入口。现在提交按钮灰着不解释，工作空间里没有可用智能体的新用户永远点不亮它，也没人告诉他为什么；熟练用户按按钮上印着的 ⌘↵，焦点一离开编辑器就没反应。

## Background

### 来源与负责人决定

来源是 2026-11-16 的 impeccable 评审，快照在 `.impeccable/critique/2026-11-16T19-05-31Z__packages-views-modals-quick-create-issue-tsx.md`（25/40；1 个 P1、3 个 P2、1 个 P3）。负责人 2026-11-17 的决定：

- **D1** 先处理 P1：卡住时给出原因。没有可用智能体时给两个出口：去配置智能体、切换到手动。
- **D2** 范围是 P1 加三个 P2。P3 对齐和小问题留给最后的 polish，不属于本任务。
- **D3** 主按钮写「创建」不是有意的。按钮、进行中、计数统一成「交给」。
- **D4** 建 Trellis 任务，先规划。实现要等最终规划摘要获得明确批准。
- **D5** 辅助按钮的统一只改本弹窗：让「帮我选智能体」向共用的「帮我完善」看齐。共用组件只做两处不改行为的修改，三个界面都会看到：禁用态不再二次变淡；状态文案里的「优化」改成「完善」。
- **D6** 「交给谁」这个位置不设统称。动作一律说「智能体」；上下文已经清楚的地方去掉名词；必须两者都提时写「智能体或AI小队」。「助手」和「对象」从这个弹窗消失，词表不加新词。这推翻了 2026-09-30 第二期「创建助手」的叫法（`.trellis/tasks/archive/2026-09/09-30-quick-create-actor-picker-phase2/prd.md:21`）。

### 代码现状：提交拦截

- 提交按钮用原生 `disabled`，条件是 `!hasContent || !actor || submitting || versionBlocked || autonomyBlocked || gate.uploading || (anchorCommentId && !sourcePreview)`（`packages/views/modals/quick-create-issue.tsx:1090`）。只有自治等级和版本两种拦截带原生 `title`（`:1094-1100`）。原生禁用的按钮不在 Tab 顺序里，`title` 键盘和读屏都读不到。
- ⌘↵ 只绑在描述编辑器上（`ContentEditor onSubmit`，`:866`）。被拦下时在 `onSubmit` 守卫里返回 false（`:497`），界面没有任何变化。`useComposerSubmit.submit()` 对空内容、进行中、上传中也是静默返回（`packages/views/editor/use-composer-submit.ts:153-156`）。
- 手动模式已有现成做法：单一的 `submitState`（`packages/views/modals/create-issue.tsx:875-889`）；缺标题、来源不可用、迭代不可用时按钮用 `aria-disabled` 保持可聚焦（`:893-909`）；缺标题时 Tooltip 说明原因（`:1490-1502`）；提交被拦时把焦点送到标题（`:818`）。原生 `disabled` 只用于提交中和上传中。
- 自治等级拦截提示（`quick-create-issue.tsx:807-824`）说清了是什么、为什么、下一步，并带「切换到手动」按钮。它调用 `switchToManual(actor)`（`:676-699`），带着草稿切换并预选负责人。版本拦截提示（`:826-835`）没有按钮。
- 头部的展开、关闭按钮用原生 `title` 而不是 Tooltip，原因写在注释里（`:724-727`）：Tooltip 会在键盘聚焦时打开，而弹窗的焦点陷阱在挂载时会短暂聚焦第一个可聚焦元素。提交按钮不是第一个可聚焦元素，不受这个问题影响。

### 代码现状：「交给谁」为空的原因

- 选人逻辑 `resolveQuickCreateCreator`（`packages/core/issues/creator-selection.ts`）在列表加载完后依次尝试：显式候选（调用方、草稿）→ 默认 → 上次 → 第一个可用的智能体。兜底只找智能体，不找 AI小队。
- 所以加载完成后「交给谁」仍为空，只有三种情况：
  1. 智能体或成员列表加载失败且没有缓存。
  2. 没有任何可以交给的智能体或AI小队：`visibleAgents`（未归档、已绑定运行时、当前用户可分配，`quick-create-issue.tsx:164-188`）和 `visibleSquads` 里没有一个通过 `canCreateAs`（`:205-222`）。`recommendationActors`（`:336-339`）正是这个可用集合。
  3. 列表或本地偏好其实还没加载完，选人逻辑在等它们。这种情况会自己结束。（可用的 AI小队必然有一个可用的队长智能体，而它会被自动选中，所以不存在"只有 AI小队可选"的状态。）
- 情况 2 下，选择器里只有一句「暂无可用智能体或AI小队。」（`packages/views/modals/quick-create-actor-picker.tsx:334`），没有任何操作。自治等级不够的会灰着列出并说明原因；没绑定运行时或不可分配的直接不出现。
- 任何成员都能创建智能体：`POST /api/agents` 没有角色门槛（`server/cmd/server/router.go:2528`、`server/internal/handler/agent.go:1328`），智能体页的创建按钮也不按角色隐藏（`packages/views/agents/components/agents-page.tsx:1176`）。路径是 `paths.agents()` 和 `paths.newAgent()`（`packages/core/paths/paths.ts:62-63`）。智能体页在没有智能体时自带空状态和创建按钮（`agents-page.tsx:1223`）。

### 代码现状：两个 AI 辅助按钮

| | 帮我选智能体 | 帮我完善 |
|---|---|---|
| 组件 | `packages/views/modals/creator-recommendations.tsx`，只在本弹窗使用 | `packages/views/modals/issue-description-assist.tsx`，手动创建、智能体创建、协作创建三处共用 |
| 可用时的文字色 | ghost 按钮默认的正文色（`:90`） | `text-muted-foreground`（`:248-265`） |
| 禁用时的文字对比度（浅色） | 3.74:1 | 2.13:1。灰字再叠 50% 透明度，比自己的提示（5.9:1）还淡 |
| 取消 | 状态文字旁单独的下划线「取消」（`:94-95`） | 按钮自己变成「取消」（`:248-265`） |
| 出错 | `role="alert"` 但灰色，没有重试（`:99-100`） | `role="alert"` 红色，加描边的「重试」（`:317-327`） |
| 描述改动后 | 结果静默清空（`:52-54`） | 提示「内容已更新…」（`:163-175`） |
| 没有候选时 | `start()` 没有处理（`:56-80`） | 不适用 |

- 动词不统一。按钮叫「帮我完善」「完善描述」，状态却写「请重新优化」「AI 优化暂不可用」「优化失败」「AI 优化建议」（`packages/views/locales/zh-Hans/modals.json:187、191、192、194`）。英文同样：Refine 对 Optimize（`packages/views/locales/en/modals.json:189、193、194`）。协作创建自己的按钮名也是「帮我完善」「AI 完善」（`packages/views/locales/zh-Hans/collaboration.json:588-590`），所以把状态文案改成「完善」对三个界面都是变一致。
- 协作创建通过 `packages/views/collaboration/creation-description-assist.tsx:32` 以 `mode="manual" textFormat="plain"` 加自己的 `actionLabel` 使用共用组件。
- 规格记录的约定：
  - `.trellis/spec/views/frontend/issue-description-assist.md:59-60`：全新的空草稿仍显示**禁用的**辅助按钮和空输入提示。
  - 同一文件 `:49-55`：`disabledReason` 只管生成、重试、重新生成。
  - `.trellis/spec/views/frontend/quick-create-actor-picker.md:153`：手动与智能体的辅助动作名各自保留。

### 代码现状：「交给谁」一行

- 默认触发器是无边框的 12px 灰字（`quick-create-actor-picker.tsx:233-239`），旁边选填的「无项目」反而有边框和深色字。已选的名字已经是正文色（`:236`）。这个默认触发器只有本弹窗使用（`quick-create-issue.tsx:761`）；`packages/views/chat/components/new-chat-button.tsx:76-77` 和 `packages/views/modals/manual-create-assignee-picker.tsx:93-94` 都传入自己的触发器。
- 常驻提示（`quick-create-issue.tsx:802-804`）离选择器 15.5px，离「任务描述」（`:849`）12.5px，读起来像描述的说明。
- 采纳推荐后，焦点落在选择器触发器上（`:757-760`），此时 ⌘↵ 无效。手动模式的 ⌘↵ 同样只绑在编辑器上（`create-issue.tsx:1013`）。
- 面板返回的是 Fragment，`<DialogContent>` 由外壳 `packages/views/modals/create-issue-dialog.tsx:148-172` 持有，两种模式共用。
- 发送快捷键可由用户配置，也可以是不带修饰键的回车。弹窗级绑定已有先例：`packages/views/modals/run-confirm.tsx:228`。

### 代码现状：用词

- 同一个动作的说法（`packages/views/locales/zh-Hans/modals.json`）：标题「通过智能体创建」；标签「交给」；按钮 `submit`「创建」（`:316`）；进行中 `sending`「发送中...」（`:317`）；完成态 `sent_label`「已发送」（`:319`）；计数 `sent_count`「已发送 {{count}}」（`:320`）；完成提示 `toast_sent`「已交给智能体——完成后会在收件箱通知你」。
- 英文只有按钮不一致：`submit` 是 "Create"，其余是 "Send to"、"Sending…"、"Sent"、"{{count}} sent"、"Sent to agent — …"（`packages/views/locales/en/modals.json:303-325`）。
- 同一个位置三个名词：
  - 「智能体」：`pick_an_agent`「选一个智能体...」（`:302`）、`choose`「帮我选智能体」（`:390`）。
  - 「助手」8 处：`:375、376、378、391、395、398、399、402`。
  - 「对象」5 处：`:333、334、366、367、384`（其中 `:384` 是常驻提示「默认由所选对象负责，也可在描述中另行指定。」）。
  - 英文是 agent、actors、assistant 三个（`en/modals.json:305、369、381、393、394、398、401、405`）。
- 词表（`apps/docs/content/docs/developers/conventions.zh.mdx`）只有「智能体」和「AI小队」。
- 选的是 AI小队时，实际建任务的是队长这个智能体（`apps/docs/content/docs/agents.zh.mdx:80`），所以「交给智能体」对 AI小队同样成立，完成提示现在就是这么写的。
- 看到的和念出来的不一致：推荐条目的按钮显示「使用此助手」（`use`），可访问名称是「使用 {{name}}」（`use_named`，`creator-recommendations.tsx:106-110`）。

### 不能"修掉"的既定决定

- 「切换到手动」的 `pen-sheen` 和「切换到智能体」的 `border-beam` 是负责人 2026-10-11 的决定（`quick-create-actor-picker.md:140、150`）。
- 标签「交给」和选择器下方的常驻提示（`:93-97、132`）。提示不得暗示立即执行（`:139`）。
- 描述为空时，推荐的前提必须可见（`:136`）。推荐必须点击并明确采纳（`:93-97`）。
- 宽度随内容变化的触发器不配负外边距（`:143`）。
- 44px 触控目标（`:141`）；375×812 下的推荐列表几何，以及提交按钮完整可达（`:142`）。
- 提交失败用 alert 区域，警告和已接收计数用 status 语义（`:138`）。

## Requirements

### 卡住时给出原因（P1）

- **BLK-1** 提交按钮只在提交中、上传中使用原生 `disabled`。其余拦截改用 `aria-disabled`：按钮可聚焦、可激活，外观仍是禁用态。
- **BLK-2** 每一种拦截都有一句用户读得到的原因，鼠标悬停、键盘聚焦、读屏三种方式都能得到。至少区分：没写描述；还没选交给谁；列表正在加载；列表加载失败；没有可以交给的智能体或AI小队；自治等级不够；CLI 版本不够；来源上下文不可用。几种同时成立时，只说最该先处理的一条。
- **BLK-3** 点击或按发送快捷键被拦下时，焦点移到用户该处理的那一项，草稿保持不变：
  - 缺描述 → 描述编辑器。
  - 还没选、列表加载失败、版本不够 → 选择器触发器。
  - 自治等级不够 → 提示里的「切换到手动…」按钮。
  - 没有可以交给的 → 新提示里的第一个出口。
- **BLK-4** 列表加载完成、且没有任何可以交给的智能体或AI小队时，在选择器下方显示一条提示。写法向自治等级提示看齐（是什么、为什么、下一步），带两个出口：
  - 「去配置智能体」：前往智能体页。离开前保存草稿并关闭弹窗。
  - 「切换到手动」：复用 `switchToManual`，草稿带过去。
- **BLK-5** 没有可以交给的智能体或AI小队时，「帮我选智能体」不发起请求，并说明原因。
- **BLK-6** 只改变"不能提交时的表现"，不改变"能不能提交"的条件。服务端校验和提交流程不动。

### 两个 AI 辅助按钮一致（P2）

- **AID-1** 两个辅助按钮在同一状态下表现一致：可用和禁用时的颜色各自相同，禁用态不再二次变淡；取消方式相同；出错都用提醒色加「重试」；内容变动导致结果作废时都给一句说明。
- **AID-2** 「帮我完善」的状态文案统一用「完善」（英文统一用 refine），不再出现「优化」（optimize）。
- **AID-3** 统一的方向按 D5：「帮我选智能体」改成和「帮我完善」一样的做法（请求中按钮自己变成「取消」）。共用组件 `issue-description-assist.tsx` 的行为不变，手动创建和协作创建只会看到禁用态颜色和状态文案这两处变化。

### 「交给谁」一行（P2）

- **RCP-1** 本弹窗里的选择器触发器有一个和「无项目」同级的外框。占位文字「选一个智能体...」保持次要色。新建聊天入口和手动模式负责人选择器的触发器不变。
- **RCP-2** 常驻提示在视觉上属于选择器：它到选择器的间距小于它到「任务描述」的间距。
- **RCP-3** 发送快捷键是带修饰键的组合（默认 ⌘↵）时，在本弹窗内的非输入控件上按下也触发提交，包括刚采纳推荐、焦点在选择器上的时候。选择器弹层等浮层里的按键、其他输入框里的按键不触发；在描述编辑器里按一次只提交一次。用户把发送键配置成不带修饰键的回车时，这条弹窗级绑定不生效，回车仍然只激活当前聚焦的控件。
- **RCP-4** 描述为空时两个辅助按钮保持原生禁用（按 D5，也符合 `issue-description-assist.md:59-60` 的约定）。它们的前提提示继续作为按钮旁的可见文字。

### 用词（P2）

- **VOC-1** 按钮「交给智能体」；进行中「正在交给...」；完成态「已交给」；计数「已交给 {{count}}」。英文按钮改为 "Send to agent"，其余已一致，不动。
- **VOC-2** 按 D6，本弹窗用到的文案（`create_issue.agent`、`create_issue.actor_picker`、`create_issue.creator_recommendations` 三组）不再出现「助手」和「对象」，英文不再出现 assistant 和 actors。「选一个智能体...」「帮我选智能体」保留。推荐条目的按钮可见文字改为「使用」，可访问名称保持「使用 {{name}}」。逐条改法见 `design.md`。
- **VOC-3** 常驻提示只换名词（「所选对象」→「你选的智能体或AI小队」），含义不变，仍然不暗示立即执行。
- **VOC-4** 中英文语言包的键保持对等（`parity.test.ts`）；中英混排、省略号遵守 conventions。

### 测试、文档与规格

- **QA-1** 行为类改动先写失败的测试。纯状态判定放在不需要 DOM 的 `.test.ts` 里，组件测试只保留主路径、接线、可访问性和点名的回归。
- **QA-2** 按旧名称或旧文案匹配的端到端选择器全部更新。已知位置：`e2e/quick-create-actor-picker-phase2.spec.ts:64、99、103、110、146、154`，`e2e/issue-assist-flow.spec.ts:183、211`；另外 `e2e/quick-create-actor-picker.spec.ts`、`e2e/quick-create-autonomy.spec.ts`、`e2e/core-business-agent-loops.spec.ts`、`e2e/settings-preferences.spec.ts`、`e2e/upstream-selected-fixes.spec.ts`、`e2e/main-supplemental-qa.spec.ts` 需要逐个核对。
- **QA-3** 规格 `.trellis/spec/views/frontend/quick-create-actor-picker.md` 同步更新：新的提交按钮名、拦截状态的表现、没有可交给时的出口、不设统称的用词规则。`.trellis/spec/views/frontend/issue-description-assist.md` 补一句禁用态颜色的约定。
- **QA-4** `server/internal/service/builtin_skills/` 里没有提到这些按钮名（已查），不需要改。`apps/docs/content/docs/` 的正文只提到模式名「通过智能体创建」，不提按钮名，也不需要改。

## Acceptance Criteria

2026-11-20 completion evidence: see [acceptance](research/acceptance.md), [independent code check](research/final-code-check.md), and [visual verdict](research/visual-verdict.json). Browser mechanics and the final four locale/theme cases pass in the isolated production build; native replay also passed. All 30 acceptance items are closed; submission and archival bookkeeping follow the execution plan.

### 卡住时给出原因

- [x] 描述为空时，提交按钮能用 Tab 聚焦，聚焦或悬停能看到"先填写任务描述"的说明，同一句话也是按钮的可访问描述。点击或按 ⌘↵ 后光标进入描述编辑器，不发请求。
- [x] 描述已写、列表或本地偏好还在加载、「交给谁」为空时，说明是"列表还在加载，请稍候"。激活后焦点在选择器触发器上。
- [x] 列表加载完成且没有可以交给的智能体或AI小队时，选择器下方出现提示，含原因和两个按钮。
- [x] 点「去配置智能体」：到达智能体页，弹窗关闭；重新打开弹窗，草稿还在。
- [x] 点「切换到手动」：手动表单的描述就是刚才的草稿。
- [x] 没有可以交给的智能体或AI小队时，「帮我选智能体」不发请求，界面说明原因。
- [x] 自治等级不够时激活提交，焦点落在提示里的「切换到手动…」按钮上。
- [x] 版本不够时，按钮的说明沿用现有的版本提示。激活后焦点在选择器触发器上。
- [x] 提交中、上传中，按钮和现在一样是原生禁用。
- [x] 今天能提交的组合仍然能提交，今天不能的仍然不能。由现有单测加一份新的状态矩阵测试证明。

### 两个 AI 辅助按钮一致

- [x] 浅色、深色主题下，两个辅助按钮可用时颜色相同，禁用时颜色也相同；禁用态文字对背景不低于 3:1，不再出现 2.13:1 这种灰字再叠透明度的情况。
- [x] 「帮我选智能体」请求中，按钮自己变成「取消」，和「帮我完善」相同。
- [x] 「帮我选智能体」出错时是提醒色文字加「重试」，点「重试」会重新发起请求。
- [x] 推荐结果显示着的时候改动描述、项目或「交给谁」，结果消失的同时出现一句说明，读屏会播报。
- [x] 「帮我完善」的所有状态文案里没有「优化」，英文里没有 optimize。
- [x] 手动创建和协作创建里，「帮我完善」的行为没有变化（现有单测不改断言即通过，文案断言除外）。

### 「交给谁」一行

- [x] 本弹窗的选择器触发器有外框，和「无项目」同级。新建聊天入口、手动模式负责人选择器的外观没有变化。
- [x] 实测：提示到选择器的间距小于提示到「任务描述」的间距。
- [x] 采纳一条推荐后直接按 ⌘↵ 能提交。
- [x] 焦点在弹窗内的关闭按钮、属性 pill、开关上按 ⌘↵ 都能提交。选择器弹层打开时在搜索框里按 ⌘↵ 不提交。在描述编辑器里按一次 ⌘↵ 只发一次请求。
- [x] 发送键配置成不带修饰键的回车时，焦点在关闭按钮上按回车是关闭弹窗，不是提交。

### 用词

- [x] 中文：按钮「交给智能体」，进行中「正在交给...」，完成态「已交给」，计数「已交给 N」。英文按钮 "Send to agent"。
- [x] 中文语言包的 `create_issue.agent`、`create_issue.actor_picker`、`create_issue.creator_recommendations` 三组里搜不到「助手」和「对象」；英文对应三组里搜不到 assistant 和 actors。
- [x] 推荐条目按钮显示「使用」，可访问名称是「使用 {{name}}」。
- [x] 375×812 和默认宽度下，中英文底栏都不溢出，提交按钮完整可见、可点。
- [x] 语言包对等测试通过。

### 整体

- [x] `pnpm typecheck`、`pnpm lint`、受影响的单元测试、受影响的端到端规格全部通过；没跑的检查在交付时明说。
- [x] `pen-sheen` 和 `border-beam` 相关的类名与样式没有出现在改动里。
- [x] 两份规格文件已更新。
- [x] 改完后重新评审，原来的 P1 不再出现。

## Out of Scope

- P3 对齐与节奏：五条左边缘，以及两种模式的描述标签样式不同。
- 评审里的小问题：键帽图标 14px；pill 高度 26px 对 24px；头部按钮的原生 `title`；属性行缺 `role="group"`；「任务描述」是 `<p>`；没有引用的 `select_agent_aria`；`AI小队` 与 `AI 小队` 并存；`…` 与 `...` 混用。
- 共享 Switch 组件的非文本对比度；属性 pill 的焦点环。
- `pen-sheen` 和 `border-beam` 不动。
- 手动模式提交按钮的行为和它的 ⌘↵ 范围不动。
- 版本拦截提示不加按钮。
- 共用的「帮我完善」不改行为（按 D5）：取消不改成独立按钮；描述为空时不改成可聚焦；按钮原地变「取消」带来的连点风险不处理。
- 弹窗的读屏名称「快速创建任务」不改。它和可见的「通过智能体创建」不一致，但改它要牵动 4 个以上端到端规格的选择器，留给 polish。
- 把「交给」和「帮我选智能体」挪到描述下方；让没绑定运行时或不可分配的智能体也灰着列出。这两条是评审提出的开放问题，不做。
- 在常驻提示里补"完成后收件箱通知"。
- 其他界面里的「助手」（小阿孚的「通用助手」、协作里的「助手消息」）不动。
- 服务端、API、数据库不动。
- 文档里的截图不重拍（`apps/docs/content/docs/tutorial.zh.mdx:151、295`，`tutorial.mdx:288` 引用的截图会显示旧的按钮文字）。
