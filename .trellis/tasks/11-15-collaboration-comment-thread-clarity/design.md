# 设计：协作评论区降噪与未解决讨论突出

## 1. 边界

| 层 | 改动 |
| --- | --- |
| `packages/core/collaboration` | 新增纯函数 `deriveCollaborationCommentThreads`（`comment-threads.ts`）及其 `.test.ts`；`index.ts` 加一行导出。无网络、无 React。 |
| `packages/views/collaboration` | `comment-list.tsx`（列表与卡片重排）、`discussion-actions.tsx`（抽出 hook）、`comment-history.tsx`（改为受控）、`collaboration-detail.tsx`（头部与 reveal 接线）。 |
| `packages/views/locales/{en,zh-Hans}/collaboration.json` | 新增文案键。 |
| `e2e/` | 选择器同步；新增一个共享的菜单辅助函数。 |
| `.trellis/spec/views/frontend/collaboration-detail.md` | 第 3 阶段补充契约。 |

不改：服务端、core 的 schema / query / mutation、`comment-composer.tsx`、`packages/ui`。

## 2. 串的推导（core）

```ts
export interface CollaborationCommentThread {
  /** 最上层可知祖先的 id；祖先未加载时就是那个缺失的 parent_id。 */
  id: string;
  /** 已加载的根评论；根未加载时为 null。 */
  root: CollaborationComment | null;
  /** 深度优先、同级按 created_at 再按 id 升序；根在最前。depth 从 0 起。 */
  items: { comment: CollaborationComment; depth: number }[];
  /** 串内带 resolved_at 的已加载评论。 */
  resolvedBy: CollaborationComment | null;
  /** 常显「解决讨论」按钮所在评论：根可解决则为根，否则串内第一条 can_resolve 的评论。已解决时为 null。 */
  resolveTarget: CollaborationComment | null;
  /** 未解决讨论：resolvedBy 为空且至少一条 deleted_at 为空。 */
  pending: boolean;
}
export function deriveCollaborationCommentThreads(comments: CollaborationComment[]): CollaborationCommentThread[];
```

- 输入是已去重的已加载评论。输出按每个串首条评论的 `created_at` 升序。
- 祖先链用 visited 集合防环；成环时把当前评论当作根。
- 根未加载时，回复的 depth 从 1 起（视觉上仍缩进），列表层据此渲染「查看被回复评论」链接。
- 这是纯状态矩阵，测试放 `comment-threads.test.ts`（`// @vitest-environment node`），views 测试不重跑矩阵。

## 3. 列表结构（views）

### 3.1 保持扁平的 `<ol>`

所有评论仍是同一个 `<ol>` 下以评论 id 为 key 的 `<li>`，串结构只通过 `data-depth` 与缩进样式表达，不嵌套 DOM。原因：单独恢复的评论（`additionalIds`，数据在 `CommentItem` 自己的 query 里）并入已加载页时，如果换了父节点会重新挂载并丢失未发送的编辑内容（`comment-list.tsx:42` 的既有契约）。

渲染顺序：`additionalIds`（不在已加载页、无串信息，照旧置顶、depth 0）→ 各串按序展开为 `[折叠条 li?] + items`。

### 3.2 折叠 = 隐藏，不卸载

折叠串的评论 `<li>` 加 `hidden`，组件保持挂载。折叠条是该串前的一个独立 `<li>`（`key = thread:<id>`），内含 `<button aria-expanded>`，文案为已解决标记 + 解决人（`commentActions.resolvedBy`）+ 评论条数（仅 `allLoaded` 时）。

展开状态：`useState<Map<threadId, boolean>>`，在渲染期为首次出现的串写入初值 `!resolved`（与文件里 `restoredIds` 相同的渲染期派生写法）。之后只由用户操作或下面两条规则改变：

- 强制展开：该串任一评论在 `drafts.commentChanges` / `drafts.discussion` 里有待恢复请求 → 展开并写入状态，期间折叠按钮禁用。（实施时的修订：原设计是不写状态、草稿清掉后回到用户上次的选择；实际改为记录展开，这样恢复完成后结果与编辑框不会立刻被折叠收走。）
- reveal：`revealCommentId` 与 `revealKey` 组合变化时，把包含它的串设为展开（渲染期处理，保证父组件的锚点 effect 运行时目标已可见）。

页面内变为已解决的串：状态里已有 `true`，不会自动折叠（R2.6）。

### 3.3 `CollaborationCommentList` 的新 props

```ts
{ ...现有, threads: CollaborationCommentThread[]; allLoaded: boolean;
  revealCommentId: string | null; revealKey: number | string | undefined }
```

`collaboration-detail.tsx` 对 `rows` 调一次 `deriveCollaborationCommentThreads`（`useMemo`），既用来算头部的未解决数量与第一个未解决讨论，也通过 `threads` 传给列表；列表不重复推导，也不向上回调。

### 3.4 评论卡片

```
[头像] 作者 [Agent][外部协作者] · 相对时间 · 已编辑 · 由 X 解决          [⋯]
（父未加载时）查看被回复评论
正文 / 省略占位 + 加载全文
附件
回复  👍2  ☺                                              [解决讨论]
编辑或删除表单 / 冲突 / 待恢复 / 状态反馈 / 历史区域
```

- 「⋯」用 `@multica/ui` 的 `DropdownMenu`，触发按钮带 `aria-label`（新键 `commentActions.more`），触屏 `min-h-11 min-w-11`。菜单项：编辑、删除、评论历史、引用为结果、（非 resolveTarget 且 `can_resolve`）解决讨论 / 取消解决；最后一组是非交互的 `DropdownMenuLabel`，内容为角色与 AI 指令提交权限。菜单项沿用现有的禁用条件（`locked`、`drafts.error` 等）。
- 表单关闭后的焦点回到菜单触发按钮（替换现在的 `editButton` ref），沿用 `workflow.tsx:123` 的 `finalFocus` 写法避免菜单关闭与表单聚焦抢焦点。
- 已删除评论：单行（小头像、作者、`detail.deleted`、时间、⋯），有表情时在同一行后显示。`detail.deleted` 文本元素保持独立，现有 E2E 的 `getByText("Comment deleted")` 不受影响。
- 时间：`useTimeAgo()`（7 天内）或 `toLocaleDateString(locale)`；`title` 为 `toLocaleString(locale)`；`dateTime` 为 ISO。
- 「已编辑」：`deleted_at === null && Date(updated_at) > Date(created_at)`，是打开历史的按钮。
- 竖线与缩进：`<li>` 左边框 + 按 `min(depth, 3)` 的左内边距；未解决串用更强的中性语义色，其余用 `border-border`。只用语义 token。

### 3.5 讨论操作

`discussion-actions.tsx` 抽出 `useCollaborationDiscussion(scope, issue, comment?)`，返回 `{ submit, toggle, locked, canReact, canResolve, reactions, status }`，其中 `status` 是现有的 pending / error / 结果 / 待恢复那段 JSX。`CommentCard` 用它把表情、解决按钮、菜单项放进同一行，并在下方渲染 `status`。`CollaborationDiscussionActions` 组件保留为这个 hook 的薄封装，继续给 `CollaborationIssueReactions` 用，任务级表情区不变。mutation key、请求、幂等键、恢复逻辑一字不改。

### 3.6 评论历史

`CollaborationCommentHistory` 改为受控：`{ open, onClose }`，不再自带触发按钮；区域内加一个收起按钮（新键 `commentActions.hideHistory`）。查询仍是 `enabled: open` 的懒加载。

### 3.7 头部与宽度（`collaboration-detail.tsx`）

- `#discussion` 内包一层 `max-w-[75ch]`，标题、列表、加载更多、输入框都在其中；`comment-composer.tsx` 不动。
- 标题行：`评论` + 总数（`pages[0].total` 有值时）+ 未解决数量按钮（>0 时）。按钮点击后 `document.getElementById('comment-' + firstPendingId)`，沿用锚点 effect 里的容器滚动算法并聚焦。
- `allLoaded = comments.isSuccess && !comments.hasNextPage`。未 `allLoaded` 时未解决数量用带限定的文案键。
- 「已加载 x / y」移到「加载更多」按钮同一行，只在 `hasNextPage` 时渲染；`coverageUnknown` 说明保持原条件。
- 向列表传 `revealCommentId={destination.commentId}`、`revealKey={navigation.navigationKey}`。锚点 effect 本身不改。

## 4. 文案键（en + zh-Hans）

新增（最终用词在实施时按 conventions 定）：`commentActions.more`、`commentActions.hideHistory`、`commentActions.edited`、`detail.pendingThreads`、`detail.pendingThreadsLoaded`、`detail.threadComments`、`detail.expandThread`、`detail.collapseThread`。带数量的键：en 给 `_one` + `_other`，zh-Hans 只给 `_other`（parity 的 dead-plural 守卫）。不删除、不改写现有键。

## 5. 兼容性

- 无接口变化，已安装的桌面端与新旧后端组合不受影响。
- 评论字段全部来自现有 zod schema；`author_authority`、`total` 可缺省的分支保持。
- 无持久化状态：展开状态是组件内存状态，随 `key=comments:<identity>` 重置。

## 6. 取舍

- **扁平 DOM 而非嵌套列表**：读屏听不到「嵌套列表」层级，换来恢复编辑内容不丢。用 `aria-level` 不适用于 `li`；改为在回复的作者行提供可见的缩进与竖线，并保留父未加载时的链接。
- **不自动折叠刚解决的串**：页面上已解决的串可能同时有展开和折叠两种形态，换来反馈不消失、编辑不被藏起。
- **未解决数量只基于已加载评论**：未加载完时可能偏大（解决标记在未加载的更早评论上），文案明确限定范围而不是隐藏数字。
- **「解决讨论」只在一处常显**：其它评论的入口进菜单，能力不减，少一排按钮。

## 7. 回滚

改动集中在 4 个 views 文件、1 个 core 新文件与文案键。回滚 = 还原这些文件；新增文案键与 core 函数留着也不影响旧界面。无数据、无迁移。
