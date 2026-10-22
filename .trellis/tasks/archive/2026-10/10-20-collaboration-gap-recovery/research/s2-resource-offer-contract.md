# Research: S2 私人资源提供与采纳合同

- Query: 用现有资源目录、RegisterTarget、owner consent 和 operation 边界，确定 source-hidden owner ≠ organizer 的最小可实施提供/采纳合同。
- Scope: internal；只读 service/handler/SQL/core/views、相关规范和既有测试；不修改并行 S1 文件、不实现、不运行测试。
- Date: 2026-10-21
- Active task: `.trellis/tasks/10-20-collaboration-gap-recovery`（in_progress；用户已批准实施）。

## Findings

### 1. 实施结论与已有事实

推荐增加一个**本单资源提供记录**：当前参与的实际资源 owner 提供自己有权查看的具体 agent/squad 完整组合；当前 creator/handler 采纳后生成现有 pending target；各实际 owner 随后通过原 consent 路径分别授权。提供记录不承担 grant、Switch 或执行职能。

- 缺口在公开选择/披露链路。`server/internal/service/collaboration_resource_options.go:39` 要求 organizer，`:47` 要求其执行空间 membership；`packages/core/collaboration/resource.ts:97` 只列当前 viewer 的空间。保留这些旧入口的权限。
- `RegisterTarget` 本身没有 organizer 必须加入执行空间的条件：`server/internal/service/collaboration_targets.go:86`–`:109` 分别检查组织权、目标集合版本、各 owner 参与和实际配置；`:120`–`:135` 创建 held target、通道及事件。可以复用，不能在另一个事务内调用它的事务 owner wrapper。
- 实际资源 owner 是 `runtime.OwnerID`，同时须有该 agent 的既有 invoke 权；不能只验 `agent.OwnerID` 或 workspace admin（`collaboration_authority.go:161`–`:185`）。普通参与者具有 `ControlAI`，creator/handler 才有 `Invite`（`server/internal/collaboration/access.go:90`–`:98`）。
- 多 owner 小队仍须完整、真实 roster；目前一个 squad 的所有资源属于 squad 所在执行空间，不能借 offer 创建跨空间拼装小队（`collaboration_authority.go:103`–`:145`）。source workspace 可以完全不同。

以下均为**推荐实施合同**，不是现有 API 已具备的行为。

### 2. 路由与请求

基址 `B=/api/collaboration/issues/{id}/resources`。所有新路由放在现有 Auth-only collaboration 路由组（`server/cmd/server/router.go:1661`–`:1666`），按 issue UUID 授权、清除 ambient workspace headers、`Cache-Control: no-store`。

| 路由 | 输入 / 输出与权限 |
| --- | --- |
| `GET B/offers/options?execution_workspace_id=UUID` | owner 的候选入口；只列 viewer 明确选择且已有目录权限的空间。使用原完整 roster 构造逻辑，加“viewer 实际拥有至少一个所选 runtime 资源”的条件。 |
| `GET B/offers?limit=50&cursor=...` | 当前参与的提供者只读自己的提供记录；当前 creator/handler 可读本单全部明确提供记录。其他普通参与者不读取尚未组织的他人 offer。 |
| `GET B/offers/{offerId}` | 与列表完全相同的 predicate；只查该 issue 内此 offer，供恢复后定位和确认表单刷新。 |
| `POST B/offers` | `offer_resource`：冻结当前合法资源组合；不创建 target。 |
| `POST B/offers/{offerId}/withdraw` | `withdraw_resource_offer`：提供者撤回未采纳记录。 |
| `POST B/offers/{offerId}/adopt` | `adopt_resource_offer`：当前 creator/handler 将原组合注册为一个 pending target。 |
| 既有 `GET B/operations/{requestId}` | 在原 resource operation kind allowlist 中加入以上三个 action；不建立第二套恢复 API/账本。 |

POST 共用 `collaborationResourceRequest` 的 `Idempotency-Key`、`expected_lifecycle_epoch`、`expected_participant_epoch`、`expected_account_epoch`（正 decimal string）；UI 继续生成 UUID request key。严格 body、UUID、重复 query 参数和 unknown 字段检查沿 `server/internal/handler/collaboration_authority.go:12`–`:45`、`:56`–`:83`。

额外 body：

```text
offer_resource:
  execution_workspace_id: UUID
  kind: "agent" | "squad"
  resource_id: UUID
  roster: [{role, agent_id, runtime_id, execution_workspace_id, owner_user_id}]

withdraw_resource_offer:
  expected_offer_version: positive decimal string

adopt_resource_offer:
  expected_offer_version: positive decimal string
  expected_target_set_version: positive decimal string
```

`offerId` 从 path 进入内部 mutation payload，使 hash 包含精确操作对象；采纳不接收新的 roster、owner、binding 或 fingerprint。提供时服务器重建当前实际组合并比较所选 roster，客户端声明不是目录权限或 owner 身份证明。先规范 UUID、按 agent ID 排序，再计算原键 payload hash。

提供请求不捕获 `target_set_version`：它不改变 target set，其他目标的合法增减不应要求重新提供。采纳确认才捕获当前 target set version，并在同事务复核。

### 3. 数据形状与状态

新增 `collaboration_resource_offer` 一张表即可；没有 offer-consent 表，也没有资源 offer token。

| 字段 | 合同 |
| --- | --- |
| `id uuid NOT NULL DEFAULT gen_random_uuid()` | 永久 offer identity；唯一索引在后续独立 migration。 |
| `issue_id uuid NOT NULL` | 唯一业务作用域；查询必须同时匹配 issue 与 offer ID。永久 session 从 issue 解析，不需要另存 session 副本。 |
| `offered_by uuid NOT NULL`, `created_at timestamptz NOT NULL DEFAULT now()` | 真实提供者与提供事实。actor 必须是 roster 中至少一个实际 runtime owner。 |
| `lifecycle_epoch bigint NOT NULL CHECK (>0)` | 提供时本单生命周期；close/reopen 后旧提供不能被重新采纳。 |
| `kind text`, `resource_id uuid`, `execution_workspace_id uuid` | typed 目标与明确执行空间；`kind` CHECK agent/squad，所有 roster workspace 必须一致。 |
| `roster jsonb NOT NULL` | 1–256 个规范化 `TargetResource`，不可编辑；agent 唯一、角色完整。 |
| `configuration_fingerprint text NOT NULL CHECK (length=64)` | 复用 `collaborationConfigurationSnapshot`；仅服务器持有，不暴露给 organizer 或写入 client draft。 |
| `resource_authority jsonb NOT NULL` | 按 agent ID 固定的内部快照：`agent_id`, `owner_user_id`, `owner_auth_epoch`, `owner_participant_epoch`, `identity:{kind,installation_id,binding_id,binding_epoch}`。与 roster 精确一一对应，owner 多资源不得出现相矛盾的代次。 |
| `version bigint NOT NULL DEFAULT 1 CHECK (>0)` | 提供内容不更新；withdraw/adopt 成功各推进一次版本。 |
| `state text NOT NULL DEFAULT 'offered'` | CHECK `offered/withdrawn/adopted`。 |
| `withdrawn_at timestamptz` | 只在 withdrawn 非空；撤回 actor 恒为 `offered_by`，原 action 另由 event/operation 保留。 |
| `adopted_at timestamptz`, `adopted_by uuid`, `target_id uuid` | 只在 adopted 全部非空；target 与采纳事实同事务提交。 |

状态 CHECK 保证 offered 的两类终结字段均空；withdrawn 仅 `withdrawn_at` 非空；adopted 仅三个采纳字段非空。不使用 `updated_at` 作为并发版本，不允许 PATCH roster。

`offered → withdrawn` 或 `offered → adopted`；终态不可逆。配置/账号/参与/绑定变化以 `available=false`、准确 reason 投影，不能通过 GET 更新记录。冻结的 config/owner/lifecycle/binding 代次防止退出后重加、禁用后恢复、资源换绑后旧记录重新有效；改进后的资源必须明确新建 offer。单纯改名、heartbeat、offline、忙碌和容量不改变 fingerprint（`collaboration_authority.go:87`–`:88`），不能伪装成资源授权变化。

本建议要求成功提供时每个资源有可确认的现有 managed/enrolled identity；缺少或歧义的 identity 在候选中标 `runtime_registration_required`，供给写入拒绝，不猜测绑定。复用 `currentCollaborationResourceIdentity`（`collaboration_resources_view.go:276`–`:321`）并锁内重验；这不是要求运行端 online、机器有空槽或执行 gate 已开放。identity 只是冻结选择，后续仍须每个 owner 的 consent。

请求身份继续使用现有 `collaboration_operation(scope_kind='issue', scope_id, actor_user_id, request_id, kind, payload_hash, result)`；offer ID/version 出现在原 result 和对应 event，即可精确关联提供/采纳/撤回。无需重复请求表或另一组去重唯一键。操作/事件 kind 目前为开放 text（`server/migrations/571_collaboration_schema.up.sql:123`–`:145`）。

迁移采用加法：一份 CREATE TABLE（无 inline PK/UNIQUE/FK/cascade）；随后两份单语句 migration，分别 `CREATE UNIQUE INDEX CONCURRENTLY ... (id)`、`CREATE INDEX CONCURRENTLY ... (issue_id, created_at DESC, id DESC)`。owner 列表先按 issue 缩小再过滤，暂不增加重复目录索引。没有 active typed-resource 唯一索引，以免 stale offer 阻止明确重新提供；真正 target 的唯一性由原 RegisterTarget 维护。

每个并发索引登记 `server/cmd/migrate/main.go:122` 的 `concurrentIndexCleanups`，参照现有 713/714/716 模式及 `migrate_collaboration_switch_test.go:159`–`:185`。迁移号在 S1 编号稳定后由协调者分配；sqlc 生成串行进行。保留 offer/operation/event 历史，沿现有协作迁移拒绝破坏性 downgrade，不对带活数据的库删表回退。

### 4. ACL、披露和事务内复用

**候选与提供：** 当前有效 human participant、非终态 issue（paused 允许）；viewer 有所选 execution workspace membership，所有 roster 项通过既有 inventory visibility predicate。至少一项 `runtime.OwnerID == actor`，且该 owner 具备实际 agent invoke 权。目录管理员能看见他人资源不等于其能提供完全不属于自己的组合。直接 POST 与候选 GET 必须使用相同的供应者资格和完整可见性检查；不能靠“这份 roster 格式有效”绕过选择器。

抽取 `ResourceOptions` 中复用的候选构造/精确 roster 可见性 helper，保留原 organizer predicate；新 `ResourceOfferOptions` 使用独立 participant+actual-owner predicate。`memberAllowedToViewAgent` 与 `canUseRuntimeForAgent` 的既有选择逻辑由 HTTP adapter 传入（`collaboration_resources.go:66`–`:68`；`agent_access.go:204`；`runtime.go:717`），POST 同样传入可信 adapter predicate，不能接收客户端谓词。精确 POST 校验无需遍历整个空间目录。

**多 owner：** 每个 actual owner 在提供及采纳时都须为当前完整参与者、账号有效、具备自己实际资源权限。复用 `validateResourceOwners`，其 source eligibility 按本单 source workspace 检查（`collaboration_authority.go:664`–`:685`），不把 source workspace membership 加为前提。冻结所有 owner 的账号/participant 代次，不能只保存提供者。若供应者看不到完整小队，整项不返回，不揭示隐藏成员 ID/数量或生成部分 roster；不自动邀请、不代签、不把成员创建为独立 @agent target。

**读：** `readCollaborationContent` 提供当前对象权限及 repeatable-read snapshot（`collaboration_comments.go:101`–`:118`）；再要求 `ControlAI` 和 `(offered_by == viewer || current creator/handler)`。这个 predicate 在 SQL 的 LIMIT 之前应用。未参与的 source member、平台管理员身份、机器凭据均不扩权。采纳后普通当前参与者通过原 `Resources` 读取已登记目标和自己的 consent 操作，不为 pending offer 增加旁观者权限。

DTO 只含 offer identity/version/state、提供者最小 account summary、typed 资源与原 roster 中的最小 agent/runtime/owner 摘要、时间、adopted target ID、当前 available/reason/can_adopt/can_withdraw。复用 `collaborationResourceSummary` 的白名单（`collaboration_resources_view.go:131`–`:160`），不返回 fingerprint、内部 authority vector、installation/binding、grant IDs、配置、目录、凭据或私人 session。读取 stale offer 仍限原已披露 ID；不按当前 squad roster扩展披露。名字可采用现有最小摘要读取，名单与资源绑定始终是冻结的。

列表响应推荐 `{protocol_version:1,issue_id,lifecycle_epoch,target_set_version,can_offer,items,next_cursor}`；detail 返回同一 offer item 并含 issue identity。游标用严格 Base64URL JSON `{issue_id,viewer_id,before_created_at,before_id}`，最多 512 bytes，默认 50/max 100；按 `(created_at,id)` 降序读取 limit+1，无额外 count。沿 `collaboration_activity.go:27`–`:72` 的边界验证模式，每页重新授权；游标不是权限。

**采纳：** `Invite && SubmitCommand` 是服务端当前 organizer/nonterminal 条件，且必须仍为 participant。冻结提供者仍是实际 owner，所有 owner 代次/完整资源配置/fingerprint/identity 均匹配。组织者不需要加入 owner 的执行空间，也不能借采纳枚举该空间。提供时已合法披露的固定摘要可以使用；不要在采纳时要求 organizer 通过旧 `ResourceOptions` 的目录 membership 条件。

**撤回：** 当前有效提供者 participant 可撤回自己的 offered 记录，包括配置已 stale 或本单 closed；只验当前人类/本单 authority、offer ID/version/state，不要求资源仍可执行或提供者仍能取得它的新绑定。其他实际 owner 不能撤回别人提供的记录，但仍能拒绝/撤销自己的执行 consent。已 adopted 的 offer 不可撤回；要撤执行授权仍走原 `RevokeGrant`（`collaboration_targets.go:35`–`:72`），不删除 offer、直接改 grant 或悄悄 Stop。

写入统一调用 `mutateCollaborationQueue`，保持已有顺序：发现并排序锁所有当前参与用户 → source workspace/issue/relation/session → channels/targets → actor participant/当前权限 → actor-key operation → 此 offer → 资源/identity 锁。现有用户锁集合包括所有有效参与 owner（`collaboration_workflow.sql:3`–`:10`）；不能在资源锁后临时补用户锁。

将 `RegisterTarget` 的现有 callback 抽为事务内 `t.registerTarget(ctx, params)`，原入口和 adopt 调用同一 primitive，完整保留角色、target-set、重复目标、owner、配置、initial hold、通道及事件检查。adopt 在同一 `t` 内先验证 offer 快照，调用该 primitive，成功后 CAS `state='offered' AND version=expected` 更新 offer 为 adopted，并写 offer event、完成唯一 operation。任何一步失败全部回滚。允许保留原 `target_registered` 事件再追加一次精确 offer-adopted 事件；两者均属同一 request/事务，不双发通知。

为三个新 payload 在 `collaborationQueueMutationInTx` 的 action 初始化分支登记 request/action（`collaboration_queue.go:262`–`:295`），给 `CollaborationQueueReceipt` 增加可选 `offer_id`/`offer_version`；创建/撤回是 issue scope，采纳成功复用 target scope receipt。不要忽略 `t.receipt` 的 action 初始化，否则存下的回执无法被 recovery 校验。

提供/采纳所用资源锁沿 `lockCollaborationResources` + `lockCollaborationResourceIdentity`（`collaboration_authority.go:278`–`:342`）；全部用当前 tx 的 q，NOWAIT 冲突只重试已回滚事务。原 operation replay 位于新业务决策之前，commit 不确定不再 mutation（`collaboration_queue.go:224`–`:228`、`:367`–`:405`）。

### 5. 竞争、拒绝与恢复

| 情况 | 必须结果 |
| --- | --- |
| owner 无本单参与/禁用、错误 credential 或未知 issue | 无写入；当前权限拒绝，不能泄漏 roster。 |
| 可见但并非任何实际资源 owner | `resource_not_owned`；不能把 workspace admin 或 invoke 资格当实际 owner。 |
| 选择后 roster/owner/config/binding 变化 | `target_configuration_changed` / `runtime_registration_required` / `stale_offer`；不换为新资源或自动刷新后重发。 |
| 任一 owner 离开再加入、账号代次改变、close/reopen | 原快照不匹配；旧 offer 不能因当前表面状态恢复而被采纳。 |
| adopt ↔ withdraw | 同 source/session/offer 锁串行；先提交者成功，后者 `stale_offer` 或终态拒绝，无半个 target/offer。 |
| 两个 organizer 同时 adopt、不同 key | 一次注册/一次 offer 转态；另一请求拒绝，不能重复 target/event。 |
| 同 key 重发或 HTTP 响应丢失 | 原 actor/key/payload 对应同一 receipt；GET 不创建 target、不推进 version。 |
| 同 key 改 offer/roster/version/action | `idempotency_conflict`；不能把旧 key 重新用于当前正确状态。 |
| 另一 offer 或原 RegisterTarget 已注册相同 typed 目标 | 原 primitive 的 `target_exists`；此 offer 不转 adopted、不覆盖配置，不改成 Switch。 |
| 采纳后未 consent、部分 owner consent | target 保持原 hold；不进入完整授权 @列表、不发许可。 |
| 采纳后某 owner revoke，或 run 可能存活 | 原 revoke/invalidation 合同收敛；waiting/未知 proof/真实容量不由 offer 更改。 |
| offer 已失效但本人想撤回 | 使用当前 authority + 当前 offer version 正常撤回；不要求旧绑定复活。 |
| 失去组织权但仍参与 | 不能新 adopt；可恢复自己的原 receipt；不重算旧成功。pending 他人 offer 列表即时按新角色收窄。 |

成功码建议 `resource_offered`(201)、`resource_offer_withdrawn`(200)、`resource_offer_adopted`(201)。终态拒绝可用 `offer_withdrawn`/`offer_adopted`(409)，版本/快照过期用 `stale_offer`(409)，其余复用 `stale_authority/stale_scope/target_exists/issue_closed/resource_owner_not_participant/resource_owner_unavailable` 等已存在语义。offer 局部缺失用 `resource_offer_not_found`(404)；保留 `not_found` 专指本单当前访问拒绝，避免可选资源错误清空合法 issue 缓存。

原 `getCollaborationResourceOperation` 先锁并验证实际账户，只查 actor 的 issue/key/kind，然后按当前 `ControlAI` 决定完整/精确七字段 redacted receipt（`collaboration_switch_view.go:214`–`:272`）。新增 kind 不改变这套授权。redacted 不允许包含 `offer_id`、`offer_version`、target 或任何零值扩展字段。完整 receipt 的 offer ID/version 与 action/status 必须自洽；失败不伪装成功，缺失/畸形关联仍保留 pending 原键。

offer event 只携最小 receipt，复用 `t.event`（`collaboration_queue.go:187`–`:197`）和既有 retained outbox；不增加通知系统，也不向 WS 发送 roster（`.trellis/spec/server/collaboration-realtime.md:37`–`:55`）。

### 6. Core / Views 最小接线

- `packages/core/collaboration/resource-schema.ts`：扩展原 action/input/receipt/path mapper；为 offer list/detail/options 增加纯 schema（可独立 `resource-offer-schema.ts`，只由纯 schema 被 ApiClient 导入）。沿 `positiveVersion`、`parseWithFallback(...,{redact:true})`、完整 roster 去重与同单身份检查。内部 fingerprint/identity vector 不进入 client 类型。
- 校验 adopted 必须有 target/adopted_by/adopted_at，withdrawn 必须有 withdrawn_at，offered 无终结字段；只从 code/status/action 一致的 receipt 结清。withdraw/adopt 回执必须对应输入 offer ID；成功版本是 expected+1（保持 exact bigint string）。旧 register/consent/revoke receipt 不要求 offer 字段。
- `packages/core/api/client.ts:1406`–`:1480` 已有不依赖 ambient workspace 的 transport、可选资源 404 隔离、body/status 及原键关联。新增读方法复用该边界，`optionalResourceRead=true`；新写仍经 `writeCollaborationResource`，无需第二套 HTTP/恢复实现。普通 issue denial 与资源局部 unavailable 必须区分。
- `packages/core/collaboration/resource.ts:29`、`:66`：扩展 selection/prepare 分支和原 draft domain `target_resource`；保存原 offer/version/target-set/authority，确认成功后才清 exact draft 并 invalidation。复用原执行/recovery/access epoch 路径，不能把 server offer 存进 Zustand 或普通 localStorage。
- 新 Query keys 位于 `collaborationKeys.issue(scope,id)` 下：`resource-offers,cursor`、`resource-offer,offerId`、`resource-offer-options,executionWorkspaceId`；Server/viewer 由已有 scope 提供。Query `staleTime:0,retry:false`、signal 和 `protectCollaborationRead` 与现有 resource queries 相同；普通事件 invalidation 已覆盖 issue 前缀。
- **不能用 `issue.capabilities.submit_command` 控制提供按钮。** 核心 issue schema 当前在 `schema.ts:233` 将其固定为 false；沿现有 `canOrganizeCollaborationResources` 的 current scope/write_context/control_ai/status/session 判断，供应者无需 creator/handler；再结合服务端 `can_offer`、自己拥有的可用候选。组织者采纳继续同时验当前角色和 per-offer `can_adopt`。
- `packages/views/collaboration/resource-panel.tsx:17`–`:26` 增加“提供资源”、自己的未采纳提供列表，以及 organizer 的明确采纳入口。现有 Add/Register、owner Consent/Revoke 入口保留。candidate form 的目录 query 可用显式 `purpose: organize|offer` 选择现有/新 options；不要复制整套 workspace/roster UI。
- `resource-form.tsx:21`、`:70` 的摘要与 consent 表单复用。采纳后刷新原资源区、逐 owner 点原 consent；不弹自动 Resume、不代其他 owner 确认。任何 stale refusal 保留用户输入，刷新后由用户明确重新选择/提交。
- `execution-panel.tsx:20`–`:30` 已将 resource draft 路由到原执行 helper，扩展原 union 即可；`:68` 的 action result 文案列表补三个 action。`packages/core/collaboration/index.ts` 和两份 locale 由协调者合并。共享 panel 已在 source-hidden/account Web/Desktop 挂载（`:71`）；不新增平台路由，不在 views 引入 Next/React Router。

### 7. 主验证位置与必须扩展的测试

所有下列项目是**待实现/待运行**，没有本次通过结果。

| 主测试层 / 文件 | 必须覆盖 |
| --- | --- |
| 新 `server/internal/service/collaboration_resource_offers_test.go`，复用 `newCollaborationFixture` / dbfx | 提供不增 target/grant/command、采纳仅增 held target；snapshot/owner/角色矩阵，original-key replay 与 deterministic 竞争。 |
| `collaboration_authority_test.go:14`、`:67`、`:100` | 原实际 owner、防代签、完整 squad consent、@目录语义保持；采纳不独立注册 squad member。 |
| `collaboration_queue_test.go:385`、`:438`、`:475`、`:533` | 借用事务屏障和 commit-fault 模式，覆盖 offer/adopt/event 回滚、unknown commit、同 key 并发；不要用睡眠猜顺序。 |
| `collaboration_resources_view_test.go:14`、`:43`、`:78` 与 `collaboration_switch_recovery_test.go:15` | owner-only identity、snapshot summary 不泄漏、自己的原键与精确七字段失权恢复。 |
| `server/internal/handler/collaboration_resources_test.go:22`、`:49`、`:105`；新增 offer handler suite | 旧目录 membership/admin/visibility 拒绝保持；新跨空间角色路径、strict query/body/path scope、machine-before-DB、no-store、无 ambient header。 |
| `server/cmd/migrate/migrate_collaboration_switch_test.go:159` 及邻近 migration helpers | 两个 concurrent index 独立单语句/cleanup、空库与重复升级、invalid index 恢复、无 FK/inline PK、历史/拒绝 downgrade。 |
| core `resource-schema.test.ts`, `resource-api.test.ts`, `resource.test.ts` | malformed/旧 endpoint、UUID/版本/状态/cursor/receipt 关联；原 key、局部 404、撤权 cleanup、late success/error、失权后 redacted recovery。 |
| views `resource-panel.test.tsx` | source-hidden 非 organizer owner 能供给；organizer 仅采纳已提供项；多 owner 独立 consent 接线、stale/unknown 原观察、撤回不隐式 revoke/Stop/Resume。不通过 DOM 重跑服务端权限矩阵。 |
| `e2e/fixtures/collaboration-resources.ts`、`e2e/collaboration-resources-web.spec.ts` + 实际 Electron | 新增三账号两空间旅程，生产 Web 与实际 Desktop 各验接线；原 fixture 的 creator==owner（`:38`–`:42`、`:62`–`:71`）不能充当该证据。 |

**不可省的正例：** P 为 source creator，H 为 handler，D 为普通已参与 owner；D 不在 source 空间，P/H 不在 D 的执行空间。D 从账号工作台打开本单（只有 source 最小摘要，没有进入来源空间的权限）→ 只在自己的目录供给具体 agent → P 或 H 在本单采纳 → D 用原 identity/consent 入口确认 → 当前合法参与者在 @中见到该目标，但保持原 hold/pause。P/H 对 D 空间的旧 inventory GET 仍 404；不能通过该流程获得 D 的其他 agent/runtime。这里的 source-hidden 指缺少 source membership；现有 `source_workspace` 只有 id/name/identifier（`packages/core/collaboration/schema.ts:23`–`:27`），不引入额外 hidden 字段。

另设多 owner squad 正例与隐藏成员负例：D 能合法查看的完整 squad，H 拥有成员资源且已参与；D 的提供/consent 均不能代替 H 的独立 consent。H 退出/禁用、D 资源转让、绑定更换、配置改变和 close/reopen 与 adopt 并发时，原 offer 没有部分采纳或旧身份复活。完整授权但 offline/忙碌/paused 的目标仍可按原语义选择并等待。

## Files Found / Related Specs

关键源码和测试用途已在上文逐项列出。规范依据：

- `CLAUDE.md`：包边界、server state、API schema、dbfx 测试、无新依赖、并发索引与迁移。
- `.trellis/spec/server/collaboration-foundation.md:19`–`:47`、`:100`–`:108`：实际 human/source/participant 访问边界，source-hidden 不失去有效参与权限。
- `.trellis/spec/server/collaboration-targets.md:30`–`:39`、`:106`–`:124`：组织/actual owner consent、独立 hold、非执行 gate 与管理操作。
- `.trellis/spec/server/collaboration-switch-and-skills.md:35`–`:70`：staged 事实、各 owner、最小目录/identity 及原键恢复，不能将已有 grant 代为新确认。
- `.trellis/spec/core/frontend/collaboration-workbench.md:26`–`:29`、`:47`–`:63`、`:79`–`:96`、`:110`–`:121`：Query、原意图、纯 schema、资源局部错误与 account/access epoch。
- `.trellis/spec/server/collaboration-realtime.md:37`–`:55`：retained event outbox、无内容 WS invalidation 与权限变动清理。
- 本任务 `design.md:38`–`:44`、`implement.md:37`–`:38`；父 R13/M03/M05 和 AC15/16/96 是已批准产品分工，本研究不重新提出产品选择。
- External references: none；仅依赖当前仓库与现有 Go/pgx/sqlc/React Query/Zod 能力，无外部 API/新依赖。

## Caveats / Not Found

- 当前没有提供记录/对应公共 API；以上明确区分推荐新合同与已有复用点，没有把方案写成已实现。
- 本次未运行测试、构建、sqlc、数据库、浏览器或 provider；既有测试只作为扩展入口，不能声称 owner≠organizer/source-hidden 旅程已通过。
- 没有必须重新询问用户的产品选择，也没有阻止实现该合同的未决技术问题。启动 S2 前仅需协调者确定 S1 后的迁移编号与共享 queue receipt/router/API barrel/locale 合并顺序；这些是并行写入安排，不应重开 M03/M05。
- 实施仍须用确定性测试验证上面的动态权限/锁/恢复不变量；研究给出的 API/DTO 名称是可直接落地的推荐，不能用临时目录放权、普通参与者 RegisterTarget 或自动 grant 作为替代。
