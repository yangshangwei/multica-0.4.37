# 接收人选择器（2026-10-29）

依据 [discussion.md §67](discussion.md)。本文件是实现契约：后端、core、views、E2E 以此为准，冲突时改本文件再改代码。

## 范围

一个共用选择器替换三处"输入完整标识再查找"：

| 入口 | 文件 | 自己一行 | 外部账号 |
| --- | --- | --- | --- |
| 新建协作任务 | `packages/views/collaboration/creation-form.tsx` | 有，默认选中 | 可选，显示"外部" |
| 邀请 / 转交 / 请求补充信息 | `packages/views/collaboration/workflow.tsx` | 无 | 可选，显示"外部" |
| 成员类自定义属性（单选/多选） | `packages/views/collaboration/field-inputs.tsx` | 无 | 不可选（`members_only`） |

`POST /api/collaboration/recipients/resolve` 保持原样：已安装的旧 Desktop 依赖它，选择器的"精确查找"一行也继续用它。

## 部署开关

`MULTICA_COLLABORATION_DIRECTORY_ENABLED`，默认 `true`，只有字面值 `false` 关闭（同 `ALLOW_SIGNUP` 的写法）。

- 开启（scope=`directory`）：列出整台 Server 的可用账号。
- 关闭（scope=`related`）：只列出与本人共享至少一个工作空间，或与本人共同参与过协作单的账号。其他人仍可通过精确查找定位。
- 开启且 `ALLOW_SIGNUP` 未关闭时，API 启动打印一条 warn：自助注册的账号可以看到全部可用账号的姓名和头像。
- 文档：`.env.example`、`apps/docs/content/docs/environment-variables{,.zh}.mdx`、`docker-compose.selfhost.yml`、`docs/offline-upgrade.zh-CN.md` 的环境变量表。

## API

`POST /api/collaboration/recipients/search`，`Cache-Control: no-store`，与 resolve 同样只接受人类账号（拒绝 `X-Agent-ID` / `X-Task-ID`）。

请求体（`application/json`，≤4096 字节，`DisallowUnknownFields`）：

```json
{ "source_workspace_id": "uuid" | "issue_id": "uuid",
  "q": "", "cursor": "opaque", "limit": 30, "members_only": false }
```

- `source_workspace_id` 与 `issue_id` 恰好一个，规则同 resolve（`decodeCollaborationRecipientQuery`）。
- `q`：trim 后 ≤64 个字符、合法 UTF-8、不含 NUL；只按姓名子串匹配（`lower(name) LIKE '%' || 转义后的 q || '%' ESCAPE '\'`，转义 `\`、`%`、`_`）。不匹配邮箱、用户名或 ID，这些只走精确查找。
- `limit`：默认 30，范围 1–50。
- `cursor`：base64url(JSON `{"n": sort_name, "i": user_id}`)，解码失败、id 非 UUID 或 `n` 超过 1024 字节时返回 400 `invalid_recipient_query`。

授权：复用 `authorizeRecipientContext`，查询前后各校验一次（同 resolve，第二次不一致返回 404 `not_found`）。创建上下文要求来源成员 + 创建开关；本单上下文要求邀请或转交能力。

响应：

```json
{ "issue_id": "uuid|null", "source_workspace_id": "uuid",
  "scope": "directory" | "related",
  "source_member": true,
  "recent": [Account],
  "recipients": [Account],
  "next_cursor": "opaque|null" }
```

`Account = { user_id, name, avatar_url, external: boolean | null }`，不含邮箱、用户名、工作空间。

- `source_member`：调用者本人当前是否为来源工作空间成员（只说调用者自己的身份，不构成泄露）。
- `external`：`source_member=false` 时一律为 `null`。否则表示该账号是否不是来源成员。这样不是来源成员的协作者无法借列表批量获知来源空间的成员名单；需要确认单个账号时仍走 resolve（每分钟 20 次的原限额）。
- `members_only=true`：只返回来源成员；`source_member=false` 时返回空的 `recent` / `recipients`、`next_cursor=null`，状态码仍为 200。**不得用 403 表示这种情况**：本单上下文里 core 的 `protectCollaborationRead` 把 403 当作失去本单访问并清空缓存（`packages/core/collaboration/access.ts:127`）。
- `recent`：仅在 `q` 为空且无 `cursor` 时返回，最多 5 个，其余情况为 `[]`。
- 调用者本人永远不出现在 `recent` 和 `recipients` 中。

可用账号：与 `FindCollaborationRecipients` 的 `available` 同一条件（未停用、无需改密、密码模式下有凭据），并在 SQL 中排除应急停用名单 `auth.TemporarilyDisabledUsers()`（id 和 lower(email) 两个数组参数）。必须在 SQL 里排除，不能分页后再在内存里过滤，否则会出现不满的页和翻页漏项。注意 `email` 可为 NULL：用 `COALESCE(lower(u.email), '') <> ALL(...)` 一类写法，避免 NULL 比较把行整体排除。

排序与分页：`ORDER BY lower(u.name), u.id`，keyset `(lower(u.name), u.id) > (cursor.n, cursor.i)`，取 `limit+1` 行判断是否还有下一页。中文姓名按数据库排序规则排序，不按拼音；拼音放二期。

related 范围（开关关闭时）：

```sql
EXISTS (SELECT 1 FROM member mine JOIN member theirs ON theirs.workspace_id = mine.workspace_id
        WHERE mine.user_id = @actor AND theirs.user_id = u.id)
OR EXISTS (SELECT 1 FROM issue_participant mine JOIN issue_participant theirs ON theirs.issue_id = mine.issue_id
           WHERE mine.user_id = @actor AND theirs.user_id = u.id
             AND theirs.joined_at <= COALESCE(mine.revoked_at, now()))
```

最近协作（`recent`）：与调用者共同参与过协作单的可用账号，按 `max(greatest(mine.joined_at, theirs.joined_at))` 倒序。条件 `theirs.joined_at <= COALESCE(mine.revoked_at, now())`：调用者退出后才加入的人不算，因为调用者从未与其共处一单。`members_only` 时同样只留来源成员。

限流：沿用 `h.PasswordLimiter`，新键 `collaboration-recipient-search:actor:` 每分钟 120 次、`collaboration-recipient-search:ip:` 每分钟 300 次；后端故障时拒绝请求（同 resolve）。

性能：一期不加索引。`ILIKE` 子串查询在内网规模（几千账号）下顺序扫描可接受；超过 5 万账号或实测 p95 > 200ms 时，再单独用 `CREATE INDEX CONCURRENTLY` 迁移加 `pg_trgm` 索引。

## core

- `api.searchCollaborationRecipients(input, { signal })`：zod 解析；顶层结构不合法时返回 `null`（与 `resolveCollaborationRecipient` 一致）；单个条目不合法时丢弃该条目，不让一条坏数据清空整个列表；`issue_id` / `source_workspace_id` 与请求的上下文不一致时返回 `null`。
- `collaborationRecipientSearchOptions(client, scope, context, q, membersOnly)`：`infiniteQueryOptions`。本单上下文通过 `protectCollaborationRead`，创建上下文通过 `protectCollaborationDelivery`。query key 挂在 `collaborationKeys.issue(scope, issueId)` 或 `collaborationDeliveryKey(scope, context)` 之下，再加 `"recipient-search", q, membersOnly`。`staleTime` 30 秒，`retry: false`。
- 纯函数 `collaborationRecipientSections({ selected, recent, pages, self })`：按 `user_id` 去重，顺序为 自己 → 已选 → 最近 → 全部，已出现在前面分组中的账号不再出现在后面。测试放在旁边的 `.test.ts`，首行 `// @vitest-environment node`。
- 草稿 `CollaborationDeliveryFormSchema.recipientQuery` 保留：schema 是 `strictObject`，删掉这个字段会让已保存的旧草稿解析失败。表单不再读写它，新草稿写 `""`；在字段旁加一行英文注释说明原因。

## views

新组件 `packages/views/collaboration/account-picker.tsx`（`CollaborationAccountPicker`），参照 `packages/views/modals/quick-create-actor-picker.tsx` 和 `PropertyPicker`：

- 触发按钮显示当前选中人的头像和姓名（新建入口默认显示"我自己"）。弹层内是搜索框，加分组列表。
- 无搜索词时依次显示：自己（仅新建入口）、已选、最近协作、全部（无限加载，"加载更多"按钮可用键盘操作）。有搜索词时合并为一个结果列表，已选的人仍排第一。
- 搜索词非空时，列表底部固定一行"按完整用户名、邮箱或账号 ID 精确查找"。点击后调用现有的 `collaborationRecipientOptions` / `collaborationCreationRecipientOptions`，命中后可选；未命中显示现有的 `recipientNotFound` 文案。
- `scope=related` 时在列表上方说明：只列出共享工作空间或共同参与过协作单的人，其他人请用精确查找。
- `external === true` 显示"外部"标记；`external === null` 不显示标记。`members_only` 下若 `source_member=false`，提示只能精确查找来源成员。
- 搜索防抖 250ms，翻页和换词时用 `keepPreviousData`，避免列表闪烁。
- 键盘：上下键移动、回车选中；Escape 只关闭弹层，不关闭外层对话框（参照 `quick-create-actor-picker.tsx:146`、`:269`）。触控目标用 `pointer-coarse:min-h-11`；字号用 `text-body` / `text-caption`。
- 文案改 en 与 zh-Hans 两套 `collaboration.json`，删除不再使用的 key，并跑 parity 测试。

## 二期（不在本次范围）

- 重名区分：待生产重名率数据（查询见下）。重名多时在姓名下加一行脱敏标识。
- `pg_trgm` 索引、拼音首字母搜索、常用联系人收藏。
- 管理后台只读显示目录开关状态。

重名率查询（在生产库只读执行）：

```sql
SELECT count(*) AS available_accounts,
       count(*) FILTER (WHERE dup.n > 1) AS accounts_sharing_a_name,
       count(DISTINCT lower(u.name)) FILTER (WHERE dup.n > 1) AS duplicated_names
FROM "user" u
LEFT JOIN user_password_credential c ON c.user_id = u.id
JOIN (SELECT lower(name) AS k, count(*) AS n FROM "user" WHERE disabled_at IS NULL GROUP BY 1) dup ON dup.k = lower(u.name)
WHERE u.disabled_at IS NULL AND NOT COALESCE(c.must_change_password, false);
```
