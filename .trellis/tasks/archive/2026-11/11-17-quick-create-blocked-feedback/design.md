# 技术设计

需求和验收见 `prd.md`。这里只写怎么做、边界在哪、为什么这样取舍。

## 边界

| 层 | 改动 | 不动 |
|---|---|---|
| `packages/core/issues/` | 新增一个纯函数，判定提交状态 | `creator-selection.ts`、提交流程、store |
| `packages/views/modals/quick-create-issue.tsx` | 提交按钮、拦截提示、焦点去向、弹窗级快捷键、提示间距 | 草稿、上传、来源上下文、属性行、`pen-sheen` |
| `packages/views/modals/creator-recommendations.tsx` | 取消方式、出错样式与重试、作废说明、零候选 | 请求身份、缓存、采纳前的校验 |
| `packages/views/modals/quick-create-actor-picker.tsx` | 默认触发器加外框 | 列表、筛选、默认与收藏、传入触发器的两个调用方 |
| `packages/views/modals/issue-description-assist.tsx` | 只改按钮的颜色类 | 所有行为、props、状态机 |
| `packages/views/locales/{en,zh-Hans}/modals.json` | 文案 | 其他语言包文件 |
| 服务端、API、数据库、移动端 | 无 | 全部 |

依赖方向不变：`views -> core + ui`。不新增 store，不新增 Context。

## 1. 提交状态（BLK-1、2、3、6）

### 纯函数

新文件 `packages/core/issues/quick-create-submit-state.ts`：

```ts
export type QuickCreateSubmitState =
  | "submitting" | "uploading"
  | "no_recipients" | "autonomy_blocked" | "version_blocked" | "source_unavailable"
  | "missing_description"
  | "recipients_failed" | "loading_recipients" | "missing_recipient"
  | "ready";

export interface QuickCreateSubmitInput {
  submitting: boolean;
  uploading: boolean;
  hasContent: boolean;
  hasActor: boolean;
  autonomyBlocked: boolean;
  versionBlocked: boolean;
  sourceRequired: boolean;   // anchorCommentId 存在
  sourceReady: boolean;      // sourcePreview 存在
  recipientsKnown: boolean;  // 智能体、成员、AI小队三个列表都有数据，且偏好已就绪
  recipientsFailed: boolean; // 任一列表报错且没有数据
  usableRecipientCount: number; // recommendationActors.length
}
```

判定顺序，自上而下取第一条成立的：

1. `submitting`
2. `uploading`
3. `no_recipients`：`!hasActor && recipientsKnown && usableRecipientCount === 0`。这是死路，写不写描述都过不去，所以排在缺描述前面。
4. `autonomy_blocked`
5. `version_blocked`
6. `source_unavailable`：`sourceRequired && !sourceReady`
7. `missing_description`：`!hasContent`
8. `recipients_failed`：`!hasActor && recipientsFailed`
9. `loading_recipients`：`!hasActor && !recipientsKnown`
10. `missing_recipient`：`!hasActor`
11. `ready`

列表还在加载而描述为空时，结果是 `missing_description`：加载会自己结束，用户该做的是写描述。

**等价性（BLK-6）。** `ready` 当且仅当现有的按钮条件全部为假（`quick-create-issue.tsx:1090`）。测试用全部布尔组合对照旧表达式证明这一点。

`packages/core/issues/quick-create-submit-state.test.ts` 以 `// @vitest-environment node` 开头，承担状态矩阵。组件测试不重跑矩阵。

### 按钮

照搬手动模式（`create-issue.tsx:893-909、1490-1502`）：

- `disabled` 只在 `submitting`、`uploading`。
- 其余非 `ready` 状态设 `aria-disabled`，类名沿用手动模式那组 `aria-disabled:opacity-50 aria-disabled:cursor-not-allowed aria-disabled:active:translate-y-0`，不加 `pointer-events-none`。
- 原生 `title` 去掉。原因用 `Tooltip`（`TooltipProvider delay={200}`）显示，同一句话放进一个 `sr-only` 元素，经 `aria-describedby` 成为按钮的可访问描述。这样不依赖 Tooltip 是否打开。
- 自治等级和版本两种拦截沿用现有的 `autonomy_blocked_tooltip`、`version_blocked_tooltip` 文案。

### 被拦下时的焦点

`submit()` 是按钮点击和编辑器快捷键共同的入口（`:662-664`）。在它里面先看状态：

```ts
const submit = () => {
  if (submitState === "ready") { void composer.submit(); return; }
  if (submitState === "submitting" || submitState === "uploading") return;
  focusBlockedTarget(submitState);
};
```

| 状态 | 焦点去向 | 怎么取到 |
|---|---|---|
| `missing_description` | 描述编辑器 | `editorRef.current?.focus()` |
| `missing_recipient`、`recipients_failed`、`loading_recipients`、`version_blocked` | 选择器触发器 | 现有写法 `creatorRef.current?.querySelector("button[data-slot=popover-trigger]")`（`:758`） |
| `autonomy_blocked` | 提示里的「切换到手动…」 | 新增 ref |
| `no_recipients` | 新提示里的「去配置智能体」 | 新增 ref |
| `source_unavailable` | 不移动焦点 | 来源卡片自带加载和重试 |

`onSubmit` 里原有的二次守卫（`:497`）保留，它防的是状态落后一帧的情况。

**取舍。** 没有做"被拦下时在页面里弹一条原因"。手动模式的先例是 Tooltip 加焦点，负责人选的也是照搬它。代价是触屏上没有悬停，点一下只有焦点移动这一种反馈；`no_recipients` 和 `autonomy_blocked` 有常驻提示，不受影响。

## 2. 没有可以交给的智能体或AI小队（BLK-4、5）

- 显示条件：`submitState === "no_recipients"`，与按钮用同一个判定，不会出现"提示说没有、按钮说别的"。
- 位置：自治等级提示所在的位置（`:807` 之前），样式照抄它：`role="status"`、warning 色、三段文字加按钮行。
- 出口一「去配置智能体」：`AppLink` 指向 `workspacePaths.agents()`，外观是 outline 小按钮。点击时沿用 `openFieldSettings` 的做法（`:704-711`）：先把编辑器内容写进草稿，原地跳转才关闭弹窗，带修饰键的点击在新标签页打开、弹窗留着。
  - 选智能体列表页而不是新建页：还覆盖"智能体已经有了，只是没绑运行时"这种情况；列表页在一个智能体都没有时自带创建入口（`agents-page.tsx:1223`）。
- 出口二「切换到手动」：`switchToManual()`，文案复用现有的 `create_issue.switch_to_manual`。这里是普通按钮，不带 `pen-sheen`；底栏那个带效果的切换按钮原样保留。
- 「帮我选智能体」：`props.actors.length === 0` 时禁用，`aria-describedby` 指向一句可见说明，`start()` 开头同样返回。
- 列表加载失败不显示这条提示。选择器里已有重试（`quick-create-actor-picker.tsx:216-228`），按钮的原因说明会指过去。

## 3. 「帮我选智能体」向「帮我完善」看齐（AID-1、3）

只改 `creator-recommendations.tsx`：

| 项 | 改成 |
|---|---|
| 取消 | 请求中主按钮自己显示「取消」，点击走现有的 `clear()`。删掉旁边那个下划线「取消」。`role="status"` 的进行中文字保留 |
| 出错 | `text-destructive`，后面跟一个 outline 的「重试」，点击调用 `start()`。`role="alert"` 保留 |
| 作废说明 | 新增 `notice: "stale" \| null`。现有的清理 effect（`:52-54`）在因为上下文变化而丢弃**结果或进行中的请求**时置为 `stale`；只丢弃错误时不提示。下一次 `start()` 或手动关闭时清掉。用 `role="status"` 输出 |
| 零候选 | 见上一节 |
| 可见文字与可访问名称 | 条目按钮可见文字「使用」，`aria-label`「使用 {{name}}」，可见文字包含在名称里 |

### 颜色

两个按钮共用一组类。在 `issue-description-assist.tsx` 导出常量，`creator-recommendations.tsx` 引用它，避免两处各写一份：

- 可用：`text-muted-foreground`，悬停沿用 ghost 变体。
- 禁用：不再让灰色叠 50% 透明度。实现采用 `disabled:text-foreground`（保留 Button 自带的禁用透明度，约 3.7:1 浅色 / 5:1 深色）。最初的候选 `disabled:opacity-100 disabled:text-faint-foreground` 已作废：`tokens.css` 注明 `--faint-foreground` 不能用于文字，下表该行仅作记录。

候选值的 token 计算（对 `--surface-raised`）：

| | 浅色 | 深色 |
|---|---|---|
| 可用 `--muted-foreground` | 5.89:1 | 6.35:1 |
| 禁用 `--faint-foreground` | 3.87:1 | 4.21:1 |
| 现状「帮我完善」禁用 | 2.13:1 | 2.60:1 |

**取舍。** 另一条路是两个按钮可用时都用正文色、禁用时正文色叠 50%。那样可用与禁用的差别更大，但会让手动创建和协作创建里的「帮我完善」明显变重，超出 D5 说的"只改禁用态颜色"。所以保留它现在的可用色，只让「帮我选智能体」降到同一级。这也顺带让加了外框的选择器成为那一行的主角。

实现时用 token 计算复核两种主题，再对一张真实截图取样。如果深色下可用与禁用分不清，在同一个常量里调整，两个按钮一起变。

共用组件除了这一个类名没有别的代码改动。

## 4. 「交给谁」一行（RCP-1、2、3）

### 外框

改 `quick-create-actor-picker.tsx:233` 的默认 `triggerRender`：加 `border`，底色和悬停沿用属性 pill 的写法。只有本弹窗用默认触发器，另外两个调用方传入自己的。

不用负外边距（规格 `quick-create-actor-picker.md:143`）。加外框后，框的左边缘落在 20px 的正文列上，文字位置不变。

### 间距

提示所在容器是 `px-5 pt-1 pb-2`（`quick-create-issue.tsx:754`），提示本身 `mt-1`（`:802`）。调整容器的下内边距，使"提示到选择器外框"小于"提示到「任务描述」"。数值在实现时量出来定，验收看实测的两段距离。

### 弹窗级发送快捷键

面板是 Fragment，没有自己的根元素。做法：在面板的 `useEffect` 里，给最近的弹窗元素（`creatorRef.current.closest('[role="dialog"]')`）加一个原生 `keydown` 监听。

用原生监听而不是在包一层 `div` 上写 `onKeyDown`，原因有两个：

- 不改 `DialogContent` 的直接子元素结构，布局零风险。
- React 的合成事件会沿组件树穿过 portal 冒泡，选择器弹层里的按键会传到面板；原生监听只看 DOM 树，弹层挂在别处，天然收不到。

监听里按顺序排除：

1. `event.defaultPrevented`：描述编辑器已经处理过。ProseMirror 在 `handleKeyDown` 返回 true 时会调用 `preventDefault`（`packages/views/editor/extensions/submit-shortcut.ts`），所以编辑器里按一次只提交一次。
2. `event.repeat`、输入法组合中（`isImeComposing`）。
3. 发送键是不带修饰键的回车（`isPlainShortcut(sendShortcut, "Enter")`）：整条绑定不生效，否则回车会劫持按钮的激活。
4. 目标是可编辑元素（`isEditableShortcutTarget`）：比如「帮我完善」里补充信息的输入框，在那里按 ⌘↵ 不该把整个任务交出去。
5. `shortcutMatchesEvent(sendShortcut, event)` 不成立。

通过后 `preventDefault()` 并调用 `submit()`，于是被拦下时的焦点规则同样适用。先例：`packages/views/modals/run-confirm.tsx:228`。

手动模式不加这条绑定（不在范围内）。

## 5. 文案（VOC-1、2、3，AID-2）

键名不变，只改值。新增的键两种语言同时加。下面的中文是定稿方向，实现时逐条对照 conventions 的词表和标点规则（Runtime 的译法以词表为准）。

### 动词

| 键（`create_issue.agent.*`） | zh-Hans | en |
|---|---|---|
| `submit` | 交给智能体 | Send to agent |
| `sending` | 正在交给... | 不变（Sending…） |
| `sent_label` | 已交给 | 不变（Sent） |
| `sent_count` | 已交给 {{count}} | 不变（{{count}} sent） |

### 名词（不设统称）

| 键 | zh-Hans 现在 → 改成 | en 现在 → 改成 |
|---|---|---|
| `agent.autonomy_blocked_tooltip` | 所选对象不能… → 所选智能体或AI小队不能创建任务，请改为手动创建。 | 核对，无 actor/assistant 则不动 |
| `agent.error_autonomy` | …或换一个创建对象。 → …或换一个智能体或AI小队。 | 同上 |
| `actor_picker.no_matches` | 没有匹配的对象 → 没有匹配结果 | No matching actors → No matches |
| `actor_picker.loading` | 正在加载创建对象... → 正在加载... | 核对 |
| `actor_picker.set_default` | 将当前助手设为默认 → 将当前选择设为默认 | 不变 |
| `actor_picker.clear_default` | 清除默认助手 → 清除默认 | 不变 |
| `actor_picker.default_unavailable` | 默认助手暂不可用 → 默认选择暂不可用 | Default assistant is unavailable → Default selection is unavailable |
| `actor_picker.creator_hint` | 默认由所选对象负责，… → 默认由你选的智能体或AI小队负责，也可在描述中另行指定。 | 不变 |
| `creator_recommendations.loading` | 正在查找合适的助手... → 正在为你推荐... | Finding suitable assistants... → Finding suggestions... |
| `creator_recommendations.unavailable` | …你仍可手动选择助手。 → 推荐暂不可用，你仍可手动选择。 | …choose an assistant manually. → …choose manually. |
| `creator_recommendations.failed` | 暂时无法推荐助手，… → 暂时无法推荐，请重试或手动选择。 | Could not recommend an assistant. … → Could not get recommendations. Try again or choose manually. |
| `creator_recommendations.empty` | …职责依据的助手，… → 暂未找到有明确职责依据的智能体或AI小队，你仍可搜索或手动选择。 | 核对 |
| `creator_recommendations.use` | 使用此助手 → 使用 | Use assistant → Use |

`actor_picker.no_matches` 和 `actor_picker.loading` 也会出现在手动模式的负责人选择器和新建聊天里。改成不带名词的说法后，在那两处同样成立（现在的「创建对象」在那两处本来就不对）。

### 「帮我完善」的状态（`create_issue.ai_optimize.*`）

| 键 | zh-Hans | en |
|---|---|---|
| `stale` | 内容已更新，请重新完善以使用最新内容。 | Content changed. Refine again to use your latest edits. |
| `unavailable` | AI 完善暂不可用，你仍可使用当前内容创建任务。 | AI refinement is unavailable. You can still create with your current content. |
| `failed` | 完善失败，请重试，或使用当前内容创建任务。 | Refinement failed. Try again or create with your current content. |
| `suggestion` | AI 完善建议 | 不变（AI suggestion） |

### 新增的键

| 用途 | zh-Hans | en |
|---|---|---|
| 原因：缺描述 | 先填写任务描述 | Write a task description first |
| 原因：还没选 | 先选择交给谁 | Choose who to send this to |
| 原因：加载中 | 列表还在加载，请稍候 | Still loading. Try again in a moment. |
| 原因：加载失败 | 列表加载失败，请打开选择器重试 | Could not load the list. Open the picker to retry. |
| 原因：来源未就绪 | 来源上下文还没准备好 | The source context is not ready |
| 提示标题（也作按钮原因） | 没有可以交给的智能体或AI小队 | No agent or squad can take this |
| 提示原因 | 通过智能体创建需要一个你可以分配、已绑定运行时、并且允许创建任务的智能体。这个工作空间目前没有。 | Create with agent needs an agent you can assign, with a runtime, that is allowed to create issues. This workspace has none yet. |
| 提示下一步 | 可以去配置一个智能体，或者改为手动创建，草稿会保留。 | Set up an agent, or create the issue manually. Your draft is kept. |
| 出口一 | 去配置智能体 | Set up an agent |
| 推荐：重试 | 重试 | Try again |
| 推荐：作废说明 | 内容已变动，推荐已清除，可重新获取。 | Content changed. Recommendations were cleared; ask again. |
| 推荐：零候选 | 没有可推荐的智能体或AI小队。 | No agent or squad to suggest. |

英文底栏：「Send to agent」加键帽比「Create」宽。按钮已有 `min-w-28`，触屏和窄屏下键帽隐藏（`max-sm:hidden`）。375×812 的验收专门看这一点。

## 兼容与回滚

- 没有接口、数据结构、持久化格式的变化。草稿 store 不变。
- 语言包只改值、加键，不删键。`select_agent_aria` 这个没人引用的键不在范围内，留着。
- 回滚就是回退提交，没有数据要迁移。按执行计划里的分组提交，可以单独回退某一组。

## 风险

| 风险 | 应对 |
|---|---|
| jest-dom 的 `toBeDisabled` 不认 `aria-disabled`，`quick-create-issue.test.tsx` 里 9 处断言会变红 | 逐条改成断言 `aria-disabled`；提交中、上传中的仍用 `toBeDisabled` |
| 端到端按名称 `Create` / `创建` 精确匹配提交按钮 | 逐个规格更新，见 `implement.md` 的清单 |
| 文案改动波及按文字匹配的单测和端到端 | 对每条旧文案在 `packages/`、`apps/`、`e2e/` 里全文搜索 |
| 共用辅助按钮的颜色类影响协作创建，而协作创建正在这条分支上由别的会话验收 | 只改一个类名，不改行为；动手前确认这些文件没有别人的未提交改动；协作创建的单测一起跑 |
| 共享检出里有约 130 个别的会话的未提交改动 | 只提交本任务的路径；每次提交前重读 `git status` 和 `git log` |
| 端到端需要一套运行环境 | 不在共享检出上执行 `make up/down/destroy`，不碰正在运行的桌面端；用手工起的隔离环境跑受影响的规格，跑不了就在交付时明说没跑 |
