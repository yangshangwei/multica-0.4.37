# 执行计划：协作评论区降噪与未解决讨论突出

需求见 `prd.md`，技术设计见 `design.md`。本文件只列顺序、验证命令与检查点。

## 开工前

- [ ] 读 `apps/docs/content/docs/developers/conventions.zh.mdx`（改 locale 与中文文案前的硬性要求）。
- [ ] `git status --short` 记录下列文件的现有未提交改动，作为「非本任务」基线：`packages/views/collaboration/collaboration-detail.tsx`、`packages/views/locales/{en,zh-Hans}/collaboration.json`、`packages/core/collaboration/index.ts`、`e2e/fixtures/collaboration-editing.ts`。把基线 diff 存到 `/tmp`（不进仓库），提交时用它区分。
- [ ] `ListAgents` 确认没有别的会话正在改这些文件；有则先沟通。
- [ ] 不执行 `git commit`，直到第 3 阶段并得到用户确认。

## 步骤

1. **core：串推导（先写失败测试）**
   - 新增 `packages/core/collaboration/comment-threads.test.ts`（`// @vitest-environment node`），覆盖：单条未解决；根 + 多级回复的顺序与 depth；任一成员已解决；根已删除无回复；全部成员已删除；根已删除但有存活回复；根未加载的多条回复归为一组；`resolveTarget` 在根不可解决时的选择；成环输入不死循环。
   - 实现 `comment-threads.ts`，在 `index.ts` 加导出。
   - 验证：`pnpm --filter @multica/core exec vitest run collaboration/comment-threads.test.ts`

2. **views：讨论操作 hook 与受控历史**
   - `discussion-actions.tsx` 抽出 `useCollaborationDiscussion`；`CollaborationDiscussionActions` 改为薄封装。
   - `comment-history.tsx` 改为 `{ open, onClose }` 受控。
   - 这一步结束时界面应与改动前等价（卡片里临时用现有布局接上），用现有测试确认没有行为回归。
   - 验证：`pnpm --filter @multica/views exec vitest run collaboration/comment-list.test.tsx collaboration/collaboration-detail.test.tsx`

3. **views：卡片重排（R1）**
   - 更多操作菜单、单行操作、已删除单行、相对时间、「已编辑」、焦点回到菜单触发按钮。
   - 同步改 `comment-list.test.tsx` 里按按钮名定位的用例（编辑、删除、评论历史改为先开菜单）；时间用例改为断言 `datetime` 与 `title`。
   - 新增用例：菜单内可读权限；已删除评论单行且可从菜单回复；「已编辑」打开历史。

4. **views：串与折叠（R2）**
   - 列表接 `threads`、`allLoaded`、`revealCommentId`、`revealKey`；扁平 `<ol>`、缩进竖线、折叠条、隐藏不卸载、强制展开、reveal。
   - 新增用例：回复缩进且父已加载时无链接、父未加载时有链接；已解决串初始折叠、展开后可取消解决；页面内解决后不折叠；有待恢复草稿时强制展开；折叠再展开后编辑内容仍在；reveal 展开。
   - 既有用例 `keeps a recovered conflict editor visible when its comment is outside the loaded page` 必须不改断言地通过。

5. **views：头部与宽度（R3, R1.7）**
   - `collaboration-detail.tsx`：75ch 容器、总数、未解决数量按钮与跳转、「已加载」移到加载更多旁、传 reveal props。
   - 改 `collaboration-detail.test.tsx:100` 一带的用例；新增全部加载后不显示「已加载」、未解决数量两种文案的用例。

6. **文案**
   - en 与 zh-Hans 同步新增键；带数量的键 en `_one` + `_other`，zh-Hans 仅 `_other`。
   - 验证：`pnpm --filter @multica/views exec vitest run locales/parity.test.ts`

7. **E2E 选择器同步**
   - 新增 `e2e/fixtures/collaboration-comment-menu.ts`，导出打开某条评论菜单并返回菜单项的辅助函数。
   - 改这些位置：
     - `e2e/fixtures/collaboration-editing.ts:201,234,238,257,262,264`
     - `e2e/fixtures/collaboration-command-history.ts:82,95`
     - `e2e/fixtures/collaboration-retained-attachments.ts:190,199,220,237,249,253`
     - `e2e/fixtures/collaboration-mention-journeys.ts:509`
     - `e2e/fixtures/collaboration-runtime-handoff.ts:142,150`
     - `e2e/collaboration-editing-web.spec.ts:61`、`e2e/collaboration-multi-account-closure.spec.ts:176-177`（`toHaveCount(0)` 的按钮断言改为菜单项断言，否则会变成恒真）
     - `e2e/collaboration-runtime-handoff-web.spec.ts:67`（全部加载后不再有「Loaded 55 of 55」）
   - 改完后重新 `rg` 一遍这些英文标签，并按路由再扫一遍 `e2e/` 里访问协作详情的 spec，确认没有漏掉的正则选择器。
   - `Resolve thread` / `Reopen thread` / `Resolved by` / `Comment deleted` / `Reply`（未删除评论）预期不需要改；逐处确认。

8. **验证**
   - `pnpm typecheck`
   - `pnpm lint`
   - `pnpm --filter @multica/core exec vitest run collaboration`
   - `pnpm --filter @multica/views exec vitest run collaboration locales`
   - 视觉验证：按记忆里的方式起真实应用（Web 或开发版 Desktop），用一个带多条评论的协作任务，分别在 zh-Hans 与 en、常规与窄宽度下截图，对照 `prd.md` 的 AC1–AC15。至少覆盖：未解决串、已解决折叠串、已删除评论、多级回复、长名字。
   - E2E：尝试用协作门控环境跑第 7 步涉及的 spec。跑不起来就在交付说明里列出「已改未跑」的清单，不写成通过。

9. **质量检查**：派发 `trellis-check`（审查与实现分开）。

## 检查点

- 第 2 步后：行为等价，现有测试全绿。这里出问题说明 hook 抽取改了语义，先修再继续。
- 第 4 步后：恢复类用例（未知回执、冲突、单独恢复）全绿。这是本任务最容易破坏既有契约的地方。
- 第 7 步后：`rg` 确认 E2E 里不再有指向已移入菜单的按钮角色选择器。

## 回滚点

- 第 2 步是独立可保留的重构；第 3–5 步出现无法解决的契约冲突时，可回退到第 2 步结束的状态。
- 整体回滚见 `design.md` §7。

## 风险文件

- `packages/views/collaboration/comment-list.tsx`：恢复与焦点逻辑密集，单行很长，改动时逐段保留条件表达式。
- `packages/views/locales/*/collaboration.json`：已有约 700 行未提交改动，只追加键，不重排。
- `e2e/fixtures/collaboration-editing.ts`：已有 1 行未提交改动。

## 第 3 阶段

- 更新 `.trellis/spec/views/frontend/collaboration-detail.md`：补充「扁平列表表达串、折叠隐藏不卸载、强制展开、未解决定义与数量限定、菜单内的低频操作」契约与对应测试归属。
- 提交前向用户确认提交范围（只含本任务的改动）。
