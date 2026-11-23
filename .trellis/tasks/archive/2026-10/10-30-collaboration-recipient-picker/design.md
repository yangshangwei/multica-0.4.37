# 技术设计：协作接收人选择器

## 1. 边界与数据流

```text
creation-form / workflow / field-inputs
        ↓ shared CollaborationAccountPicker
React Query infinite query + exact resolver fallback
        ↓ typed ApiClient + Zod response validation
POST /api/collaboration/recipients/search
        ↓ human principal / rate limit / context authority
CollaborationService.SearchRecipients
        ↓ SQL availability + related scope + recent + keyset cursor
PostgreSQL user/member/issue_participant tables
```

`POST /api/collaboration/recipients/resolve` 保持原契约，选择器只把它作为“精确查找”动作使用。候选列表永远只是发现投影，既有邀请、转交、信息请求和属性写入继续在服务端事务中重验。

## 2. Server/API

- 在现有 recipient handler 中复用严格 JSON、单一 context、human principal 和 `Cache-Control: no-store` 规则。
- 新增 `POST /api/collaboration/recipients/search`，请求支持 `issue_id | source_workspace_id`、`q`、opaque cursor、`limit`、`members_only`。
- query 只按转义后的姓名子串匹配；排序固定为 `lower(name), user_id`，以 `(sort_name, user_id)` 做 keyset 游标，取 `limit+1` 判断下一页。
- SQL 在 `LIMIT` 前过滤停用、待改密、密码凭据、应急停用、related scope 和 members-only，避免短页与漏项。
- recent 单独按共同协作时间排序，最多 5 条；不把本人放进任何列表。
- 目录开关放在 server config，默认开启，只有字面量 `false` 关闭；开启且允许自助注册时启动警告。
- 以现有密码限流器建立独立 search actor/IP budget；限流器故障 fail closed。
- 查询前后各做一次 `authorizeRecipientContext`，第二次不一致返回 `404 not_found`；错误不泄露账号存在性。

## 3. Core/API

- 增加 `CollaborationRecipientSearchSchema`，顶层结构异常返回 null，单项坏账号丢弃，响应 context 不匹配返回 null。
- 增加 `ApiClient.searchCollaborationRecipients`，校验单一 context、limit/cursor，传递 AbortSignal，并按当前 session/context 丢弃过期结果。
- 增加 `collaborationRecipientSearchOptions`，按 issue 或 creation delivery scope 建立 query key，使用 `infiniteQueryOptions`、30 秒 stale time、关闭自动重试。
- 增加纯函数 `collaborationRecipientSections`，负责 self/selected/recent/all 的稳定去重，组件不重复实现该规则。
- `recipientQuery` 留在严格草稿 schema 以兼容旧草稿，新写入统一为空字符串。

## 4. Shared view

`packages/views/collaboration/account-picker.tsx` 只负责展示和交互：

- 触发器显示头像、姓名和可选外部标记；弹层包含搜索框、分组列表、加载更多、精确查找行和错误/空状态。
- 通过 props 注入上下文、当前选中项、是否 self、是否 members-only、选择回调和 disabled 状态；不把业务授权写进组件。
- 搜索词变化防抖 250ms，使用 keep-previous-data 语义，过期查询不能覆盖新选择。
- 使用现有 Dialog/Popover/PropertyPicker/QuickCreateActorPicker 的焦点和 Escape 模式；不自建另一套弹层关闭协议。
- 保留选中项在前；成员属性入口传 `membersOnly=true` 并阻止 `external=true`。

## 5. 三入口接线

- `creation-form.tsx`：本人作为默认 recipient，选中结果写 draft，清空旧 `recipientQuery`；提交仍使用现有 creation delivery。
- `workflow.tsx`：邀请、转交、请求补充信息复用 picker，选中用户 ID 进入原 action intent；精确查找仍可打开。
- `field-inputs.tsx`：单选/多选 actor 属性复用 picker，使用成员-only 数据和既有 `member:<uuid>` 引用格式。
- 任何入口都不能仅依赖列表候选完成授权，提交前沿用现有 `validTarget`/eligibility 事务。

## 6. Compatibility, rollout and rollback

- 保留 resolver endpoint、旧 Desktop 依赖、旧草稿字段和旧 API response。
- 新 API 使用环境变量控制目录范围；关闭开关只收窄列表，不破坏精确查找。
- 不新增数据库迁移；若未来启用 `pg_trgm`，必须另立单语句 `CREATE INDEX CONCURRENTLY` 迁移。
- 回滚顺序：先将入口切回 resolver UI，再关闭目录开关；服务端 search endpoint 可保留为兼容只读接口，不影响旧客户端。
- 所有日志不写 query、邮箱或用户名；只记录脱敏 route、scope、结果数量、耗时和错误码。

## 7. Validation evidence

- Core/schema/section 单测验证契约和去重。
- Go service/handler race 与真实 PostgreSQL 验证授权、分页、available 条件、recent 和撤权竞态。
- Views 测试使用真实 Dialog/Popover primitive 验证键盘焦点、Escape、搜索状态、成员-only 和错误恢复。
- Web E2E 使用生产构建服务器，覆盖新建、邀请/转交、成员属性、分页、精确查找、撤权和窄屏；视觉检查记录截图和结论。
