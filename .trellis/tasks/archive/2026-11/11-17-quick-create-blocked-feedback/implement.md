# 执行计划

前提：负责人明确批准最终规划摘要之后才执行 `task.py start`。在那之前不改产品代码。

## 动手前

- [x] `git status --short -- packages/views/modals packages/views/locales/en/modals.json packages/views/locales/zh-Hans/modals.json packages/core/issues .trellis/spec/views/frontend e2e` 确认这些路径上没有别的会话的未提交改动（2026-11-17 核对时是干净的）。有的话先停下说明，不覆盖。
- [x] 重读 `git log -5`，确认 HEAD 没被别的会话推进到影响这些文件。
- [x] 加载 `trellis-before-dev`，读 `implement.jsonl` 里的规格。
- [x] 读 `/Users/artisan/.claude/skills/impeccable/reference/craft-floor.md`（改界面之前必读）。

## 步骤

每一步先写失败的测试，再实现，再跑这一步列出的检查。

### 1. 提交状态纯函数（BLK-6）

- [x] 新增 `packages/core/issues/quick-create-submit-state.test.ts`（首行 `// @vitest-environment node`）：判定顺序的每一条；全部布尔组合下 `ready` 与旧按钮条件（`quick-create-issue.tsx:1090`）等价。
- [x] 新增 `packages/core/issues/quick-create-submit-state.ts`。
- 检查：`pnpm --filter @multica/core exec vitest run issues/quick-create-submit-state.test.ts`

### 2. 卡住时给出原因（BLK-1 至 5）

- [x] `quick-create-issue.test.tsx`：缺描述时按钮 `aria-disabled`、可访问描述、激活后焦点进编辑器且不发请求；缺「交给谁」时焦点到触发器；自治等级拦截时焦点到提示按钮；没有可交给时出现提示和两个出口；两个出口各自的结果；提交中、上传中仍是原生禁用。把现有 9 处 `toBeDisabled` 逐条归类改写。
- [x] `creator-recommendations.test.tsx`：零候选时不发请求并有说明。
- [x] `quick-create-issue.tsx`：接入状态函数；按钮改 `aria-disabled` 加 Tooltip 加可访问描述；`submit()` 里的焦点路由；没有可交给时的提示。
- [x] `creator-recommendations.tsx`：零候选处理。
- [x] 两个语言包加新键。
- 检查：`pnpm --filter @multica/views exec vitest run modals/quick-create-issue.test.tsx modals/creator-recommendations.test.tsx locales/parity.test.ts`

### 3. 两个辅助按钮一致（AID-1 至 3）

- [x] `creator-recommendations.test.tsx`：请求中按钮变「取消」；出错有「重试」且会重新请求；结果作废时出现说明。
- [x] `creator-recommendations.tsx`：取消、出错、作废说明。
- [x] `issue-description-assist.tsx`：导出按钮颜色类常量，只改这一处类名。
- [x] 两个语言包：`ai_optimize` 的四条状态文案；推荐的新键。
- [x] token 计算复核两种主题下可用、禁用的对比度，记进交付说明。
- 检查：`pnpm --filter @multica/views exec vitest run modals/creator-recommendations.test.tsx modals/creator-recommendations-cache.test.tsx modals/issue-description-assist.test.tsx modals/issue-description-assist-editor.test.tsx modals/create-issue.test.tsx collaboration/creation-description-assist.test.tsx collaboration/creation-form.test.tsx`
- 已知要改的文案断言：`packages/views/modals/issue-description-assist.test.tsx`（Optimization failed）。

### 4. 「交给谁」一行（RCP-1 至 3）

- [x] `quick-create-issue.test.tsx`：采纳推荐后按 ⌘↵ 提交；焦点在非输入控件上按 ⌘↵ 提交；编辑器里按一次只提交一次；发送键是回车时不绑定。
- [x] `quick-create-actor-picker.tsx`：默认触发器加外框。
- [x] `quick-create-issue.tsx`：提示间距；弹窗级 `keydown` 监听。
- 检查：`pnpm --filter @multica/views exec vitest run modals/quick-create-issue.test.tsx modals/quick-create-actor-picker.test.tsx modals/create-issue-dialog.test.tsx chat/components/chat-new-chat-button.test.tsx`
- 注意：jsdom 里焦点和按键的表现与 Chromium 不完全一样，这一步的结论以第 7 步的端到端为准。

### 5. 用词（VOC-1 至 4）

- [x] 按 `design.md` 第 5 节的表改两个语言包。逐条对照 `apps/docs/content/docs/developers/conventions.zh.mdx`（词表、中英混排空格、省略号）。
- [x] `creator-recommendations.tsx`：条目按钮的可见文字。
- [x] 对每条旧文案在 `packages/`、`apps/`、`e2e/` 里全文搜索（排除 `.next/`、`node_modules/`），更新断言。已知：
  - `packages/views/chat/components/chat-new-chat-button.test.tsx`、`packages/views/modals/quick-create-actor-picker.test.tsx`（No matching actors）
  - `e2e/quick-create-actor-picker-phase2.spec.ts`（Finding suitable assistants；`Create` / `创建` 精确匹配，`:64、99、103、110、146、154`）
  - `e2e/issue-assist-flow.spec.ts`（Optimization failed；`Create` 精确匹配，`:183、211`）
  - 逐个核对：`e2e/quick-create-actor-picker.spec.ts`、`e2e/quick-create-autonomy.spec.ts`、`e2e/core-business-agent-loops.spec.ts`、`e2e/settings-preferences.spec.ts`、`e2e/upstream-selected-fixes.spec.ts`、`e2e/main-supplemental-qa.spec.ts`
- [x] 用 Python 脚本核对验收里的"搜不到"：中文三组里没有「助手」「对象」，英文三组里没有 assistant、actors。
- 检查：`pnpm --filter @multica/views exec vitest run locales/parity.test.ts modals chat/components/chat-new-chat-button.test.tsx`

### 6. 规格

- [x] `.trellis/spec/views/frontend/quick-create-actor-picker.md`：提交按钮名；拦截状态与焦点去向；没有可交给时的出口；不设统称的用词规则（并注明它取代第二期的「创建助手」）；默认触发器有外框。
- [x] `.trellis/spec/views/frontend/issue-description-assist.md`：禁用态颜色的约定，以及它和「帮我选智能体」共用一组类。

### 7. 整体验证

- [x] `pnpm typecheck`
- [x] `pnpm lint`
- [x] `pnpm --filter @multica/views test` 与 `pnpm --filter @multica/core test`。已知与本任务无关的干扰见项目记忆（`mcp-tab` 在高负载下超时等），失败时先单独重跑那个文件。
- [x] 端到端，每次一个文件：`e2e/quick-create-actor-picker.spec.ts`、`e2e/quick-create-actor-picker-phase2.spec.ts`、`e2e/quick-create-autonomy.spec.ts`、`e2e/issue-assist-flow.spec.ts`，再加第 5 步核对后确认受影响的。运行环境按 `.trellis/spec/web/frontend/e2e-run-environment.md`，用手工起的隔离环境；不在共享检出上执行 `make up/down/destroy`，不连正在运行的桌面端。退出码用重定向加 `echo $?` 取，不接 `tail`。
- [x] 截图核对：375×812 和默认宽度，中英文各一张，看底栏不溢出、提示间距、触发器外框。空状态、没有可交给的状态各一张。
- [x] 一轮批量检查、一批修正、最多再确认一轮，然后停。
- [x] 派 `trellis-check` 做独立检查（作者不自批）。

## 提交

2026-11-20 用户授权继续完成并归档。最终按 `research/closeout-plan.md` 的集成提交分组执行，避免把已合并的组件与语言包拆成未验证的中间状态。

以下是原规划分组，已由上面的收尾计划替代。提交范围保持本任务拥有的路径：

1. `feat(views): explain why agent create is blocked`（步骤 1、2）
2. `fix(views): align the two AI helpers in agent create`（步骤 3）
3. `fix(views): outline the recipient picker and widen the send shortcut`（步骤 4）
4. `fix(views): say hand off, not create, in agent create`（步骤 5）
5. `docs(spec): record agent create blocked states and wording`（步骤 6，连同任务目录和评审快照）

每次提交前重读 `git status` 和 `git log`，确认没有带进别的会话的文件。

## 风险文件与回退点

| 文件 | 为什么要小心 | 回退 |
|---|---|---|
| `packages/views/modals/issue-description-assist.tsx` | 三个界面共用，协作创建正在验收 | 只改一个类名；回退提交 2 |
| `packages/views/modals/quick-create-actor-picker.tsx` | 新建聊天和手动负责人选择器共用 | 只改默认触发器；回退提交 3 |
| `packages/views/locales/*/modals.json` | `actor_picker.no_matches`、`loading` 在三处显示 | 回退提交 4 |
| `quick-create-issue.tsx` 的提交路径 | 条件不能变 | 第 1 步的等价性测试；回退提交 1 |

## 交付时要说明的

- 实际跑了哪些检查，哪些没跑、为什么。
- 对比度复核的数字。
- 文档截图会显示旧按钮文字（不在范围内）。
- 之后可以重新运行评审，看分数变化。

## 2026-11-20 收尾

- [x] 核对现有四组实现、历史完整回归与未提交路径归属。
- [x] 修复独立检查确认的三处行为边界，并保留 red/green 测试收据。
- [x] 实测并修正 RCP-2 换行后的提示归属；完成最终中英文/浅深色/1280与375矩阵。
- [x] 补齐最终 Electron 两项回归，确认独立检查无未解决项。
- [x] 提交产品、测试、规格及验收记录。
- [x] 归档本任务。
- [ ] 记录本次收尾 journal。

证据复用说明：全量 Core/Views 与 issue-assist-flow 等未改行为路径采用 11-20 r2 完整基线加源码一致性核对；本轮新跑 33 个浏览器用例、330 条相关 Views、8 条 Core 及最终分组后 87 条回归。没有把复用记录写成全量新跑。详见 research/acceptance.md。
