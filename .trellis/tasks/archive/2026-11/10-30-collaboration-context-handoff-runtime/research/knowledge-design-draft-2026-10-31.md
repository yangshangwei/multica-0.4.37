# Design draft: 知识候选与受控上下文贡献

- Query: 为已批准的知识沉淀范围固定可实施的最小技术合同。
- Scope: internal；方案草案，未实施、未运行测试或操作数据库。
- Date: 2026-10-31
- Revision: 2026-11-01；分离提案权与目标可见性，保留外部当前处理人向来源空间提出候选的能力。
- Status: **DRAFT / 等待 runtime 契约及主会话设计复核**。不得把本文当作子任务已 start、已通过验收或已发布。
- Requirement source: `10-30-collaboration-knowledge-promotion/prd.md`、父任务 R5/AC8，以及同目录 `knowledge-promotion-preflight-2026-10-31.md`。

## 1. 固定范围与依赖门

一个协作单可以提出一个有来源的知识候选；只有符合既定权限的人确认后，它才进入指定项目或 workspace 的受控贡献集合。正文不拼进用户维护的 `project.description` / `workspace.context`，不进入 skills，也不新增通用知识库、向量检索、总结模型或依赖。

人工确认不扩大来源可见性。每次读取和实际消费仍重新检查 source 权限、目标适用范围与可信消费者/输出受众；这项策略已经确定，不再作为产品问题重问。

实施入口必须满足：

| Gate | 必须已有的事实 |
| --- | --- |
| G0 | runtime 集成基线固定，原始输入、memory version、run record、comment revision/hash、缺失和 purge 语义已验收 |
| G1 | handoff source DTO、启动绑定及真正 Provider 输入的合同稳定；消费者不能把错误/缺失结果伪称 final answer |
| G2 | 主会话复核本草案的普通 claim 消费/启动授权接线，尤其所有第一方 issue/chat 路径；已有代码并不自动具备这套新功能 |
| G3 | 新迁移编号、sqlc owner、共享文件 owner 明确；没有和 runtime owner 并发改同一生成文件或启动文件 |

知识阶段的测试不替代尚未完成的 E1–E6；也不把旧 E0 通过套到新代码版本。

## 2. 选定的数据模型

仅新增一张业务实体表 `collaboration_knowledge_candidate`。每一行是不可变的内容版本；审批状态可以按 CAS 单向变化。沿用 `collaboration_operation` 的原请求回执和 `collaboration_event` 的内容无关审计，不增加第二套通用审批/event/outbox 系统。

| 字段组 | 合同 |
| --- | --- |
| Identity | `id uuid`；`lineage_id uuid` 为首个版本的 ID；`version bigint >= 1`，同 lineage 唯一；`source_workspace_id`、`source_issue_id` |
| Immutable proposal | `target_kind=project|workspace`、`target_workspace_id`、可空 `target_project_id`、`content`、`summary`、`content_sha256`、typed `sources/evidence`、`applicability`、`proposed_by_user_id`、`proposed_at`、可空 `expires_at` |
| Correction | `supersedes_candidate_id` 指向同 source、同 target、同 lineage 的前一个版本；纠错创建新行，不能 PATCH 原内容；可空 `derived_from_candidate_id` 记录从已批准项目贡献另提空间候选的来源，不自动撤销项目贡献 |
| Decision | `state=pending|approved|rejected|withdrawn|superseded`；`state_revision bigint >= 1`；`approved_by_user_id/approved_at`；最后决定的 actor/time/reason；批准事实在后续撤回时不抹掉 |
| Publication | `published_target_revision`：批准时取得的目标知识修订号；不是 source visibility grant |

同 lineage 最多一个 `approved` 行。新纠错 pending 时，旧批准版本继续生效；确认新版本时，在一个事务里将旧版标为 superseded、批准新版并递增一次目标修订。lineage 的 source 和 target 不可改变。跨层提案使用新 lineage + `derived_from_candidate_id`，仍需原 source 管理者提出和目标人工确认。

在 `workspace` 和 `project` 各增加 `knowledge_revision bigint NOT NULL DEFAULT 0`。只有受控贡献集合的 approve/withdraw/supersede 递增它；人工 base context 的现有 revision 语义不变。过期/source access 的投影变化另外体现在实际 eligible manifest 的 hash，不能只凭 target revision 缓存正文。

普通任务增加服务端专用的 `agent_task_queue.knowledge_context_version`（NULL 表示该 task 从未协商此能力）及 `knowledge_delivery jsonb`。finalized claim 只保存不含私有 source/candidate 身份的 mode、claim generation、运行身份及阶段；另行授权的 preparation 事务才保存 candidate ID/version/hash、consumer/target identity、manifest digest，以及有限的披露授权/启动标记。JSON 不存候选正文、认证 token 或私有 Provider transcript，也不借用带用户内容的 `task.context`。

resume 的键必须是精确的 `(provider, runtime_id, native_session_id, scope_kind, scope_id, scope_generation)`，并保留 agent/配置命名空间匹配事实；generation 是服务器为新逻辑会话分配、由后续同会话续作继承的标识。每次选中的 session 必须解析其累计 exposure 集合，包含所有同 session 的已披露/可能已投喂贡献，不只看最新 task。通过所有绑定到该键的 delivery 元数据合并，或维护可证明覆盖完整历史的累计 manifest；取消、失败、最后一轮 session_id=NULL 都不能丢掉早先 exposure。相关 SQL 查询必须同时覆盖 metadata 中的原 resume binding，不可只过滤 queue.session_id。

`knowledge_context_version=1` 而已披露阶段的 manifest 缺失/损坏，属于不可证明的 negotiated history；从未协商且已证明没有 managed exposure 的 legacy session 是另一状态，不应无故重置所有旧对话。返回/固定新 native session 指针时须与该 session 的 exposure binding 原子保存；未知 fresh session ID 暂记受控 generation，取得真实 ID 后才可成为可复用指针。此合同复用 task 元数据而不新建知识会话业务实体，但查询/绑定完整性仍是必须通过 G2 的实现门。

### 固定预算

这些是草案选择的工程常量，后续改动必须同步边界测试：

| 项目 | 上限/规则 | 依据 |
| --- | --- | --- |
| 单版本正文 | 64 KiB UTF-8 | 与既有协作评论级别相称；长日志/文件只引用 |
| 摘要 | 2 KiB UTF-8，非空 | 用于候选列表，不作为自动生成结论 |
| sources + evidence | 合计最多 32 项；除正文/摘要外的提案 JSON 解码后最多 16 KiB | 沿用有界来源集合；禁止递归任意 JSON |
| 写请求 HTTP | `6 * (64 KiB + 2 KiB + 16 KiB) + 16 KiB` | 延用 memory 的正文预算与 JSON 转义膨胀分离规则 |
| 原 request_id | 1–128 字节 | 与 collaboration_operation 一致 |
| 人类决定说明 | 1–1000 字节，存受权实体字段；event/receipt 不复制自由文本 | 与 memory purge reason 一致 |
| 单目标有效贡献 | 最多 16 个、合计 128 KiB 解码材料（正文、摘要、来源和投影元数据都计入） | 小型受控上下文集合，不提供无限知识仓库 |
| runtime 合并包 | workspace + 当前 project 最多 32 项、256 KiB 解码材料；完整 UTF-8 JSON 最多 2 MiB | 两个有界目标集合的加和，独立于原始任务输入预算 |
| 人类列表 | 默认 20、最大 50；稳定 `(created_at,id)` 游标；列表不带全文 | 复用现有 keyset 分页方式 |
| 每 task 的启动授权记录 | 最多 32 项；元数据列最多 64 KiB | 超限关闭新知识启动授权，不挤掉旧记录 |

approve 前计算批准后的完整目标集合预算；超限返回明确错误，不静默驱逐或截断其他已批准贡献。source 过滤只会缩小集合。已过期/撤回/superseded 不计有效集合预算；source 暂不可见不能用于绕过全局目标存储预算。runtime 完整传送满足预算的集合，无需为知识正文再设计一套 Agent 分页接口。

拒绝无效 UTF-8/NUL；不更改用户已审阅的正文再沿用旧 hash。HTTP 层限制编码长度，业务层限制解码长度。展示名不进入身份判断，其显示长度也须有界。验证证据只保存来源引用和明确的 `unverified|observed|verified` 声明，不因批准动作就把未验证事实改成 verified。

## 3. 状态与人类授权矩阵

所有人类动作先用已验证 Principal 排除 restricted、AgentActor、task/plugin/IM/termination/daemon 凭据。JWT/PAT 的人类语义复用 `collaboration.Principal.Human()`；不凭显示名或 X-Actor 头构造授权。

| 动作 | 状态 | 当前权限 |
| --- | --- | --- |
| 提出/提出纠错 | 新 pending | source Read + 该协作单当前 creator 或 handler；目标按下述可见性规则选择，不无条件要求 target membership，也不要求批准角色 |
| 查看 source 候选正文/版本链 | 任意 | 当前 source Read 且该版本的 required sources 仍可按原版本还原；purge/delete 时仅保留允许的审计/控制元数据 |
| 批准 | pending、未过期、来源版本可用 | source Read + target membership；project 为 human member lead 或 workspace owner/admin；workspace 为 owner/admin |
| 拒绝 | pending | 与批准相同；必须有说明 |
| 撤回 pending | pending | 当前 source creator/handler，或能批准该目标的人；仅收缩状态，不产生生效内容 |
| 撤回 approved | approved | 当前 source creator/handler 且 source Read，或当前 target 确认者；target 确认者即使失去 source Read 仍可执行**内容无关的撤回**，但不能取回正文 |
| 恢复已撤回/拒绝版本 | 终态 | 不支持原地恢复；提出新版本并重新人工确认 |

选择撤回的双侧管理权，是为了让来源管理者能停止发布自己的候选，也让目标管理员在来源后来不可读时仍能移除目标贡献。它不增加阅读或批准权限，普通评论者仍不能撤回。目标管理界面可为无 source Read 的目标确认者提供不含 source ID/title/summary/body 的 restricted contribution 控制行，仅携带 candidate ID、目标内状态和撤回 affordance。

没有“任务必须仍开放”的条件：完成后的协作单仍可提炼知识。不得使用较宽的 Comment/ControlAI，也不得借用 SubmitCommand 的开放任务条件。作者退出不重写历史署名；账号被禁用的人不能再写，已经合法批准的历史由当前状态和消费授权判断。

### 3.1 提案权与目标可见性分别判断

外部当前 handler 可以是有效参与者而没有来源 workspace 的 member 行；已批准的 creator/handler 提案权不能被额外的 membership 条件取消。目标选择采用以下顺序，并在提交事务内重验：

1. **来源 workspace**：从受权协作单投影取得的 `source_workspace_id` 可直接作为候选目标。creator/handler 有当前 source Read 即可提出，即使没有该 workspace membership。只能复用已经合法可见的来源身份/名称，不能借此读取 workspace context、成员或项目目录。
2. **当前来源 project**：仅当该协作单当前 project 关系/引用已经通过受权 source 投影合法可见时可选择；锁后必须仍是当前关系。非成员界面只复用该引用及已有合法显示信息，没有名称时用“当前项目”，不能额外查询私有项目目录、description 或 resources。
3. **其他目标**：必须按提出者的普通 workspace/project 可见性解析，复用已有正常选择器和授权。不可见与不存在返回一致的 `knowledge_target_not_found`；不能通过任意 ID、候选数量或目录查询探测隐藏目标。

proposal 只创建 source-owned pending 候选及其回执/审计，不改目标 base context、managed contribution 集合或 `knowledge_revision`。为验证已合法可见的当前 source target，内部只允许必要的 ID/归属/存活性锁读，不扩大为私有 catalog 读取。批准阶段仍要求 source Read + 当前 target membership + 确认者权限；可提出不等于可批准、可读目标上下文或可消费其内容。

## 4. HTTP 与返回合同

source 集合沿用已存在的 `/api/collaboration/issues/{id}` 人类入口；单候选动作通过 ID 加载 source/target 后检查，避免 target 撤回为了拿 URL 而泄露 source ID。

| 接口 | 必要性 |
| --- | --- |
| `GET/POST /api/collaboration/issues/{id}/knowledge-candidates` | source 内分页与提出/纠错 |
| `GET /api/collaboration/knowledge-candidates/{candidateId}` | 受权正文、准确版本、同 lineage 历史元数据、当前动作能力 |
| `POST /api/collaboration/knowledge-candidates/{candidateId}/approve` | 批准准确审阅的版本 |
| `POST /api/collaboration/knowledge-candidates/{candidateId}/reject` | 明确拒绝 |
| `POST /api/collaboration/knowledge-candidates/{candidateId}/withdraw` | 双侧当前管理者的内容无关撤回 |
| `GET /api/collaboration/knowledge-candidates/{candidateId}/operations/{requestId}` | 原调用者按预先保存的 candidate ID + request ID 恢复内容无关 receipt；source 由服务器内部解析 |
| `GET /api/projects/{id}/knowledge`、`GET /api/workspaces/{id}/knowledge` | 目标页面展示 managed contributions；不能把正文加入普通 project/workspace GET 的全量文本字段 |

POST create 示例（整数版本在线上统一十进制字符串）：

```json
{
  "protocol_version": 1,
  "candidate_id": "client-preselected-uuid",
  "request_id": "client-stable-key",
  "target": {"kind": "project", "workspace_id": "uuid", "project_id": "uuid"},
  "summary": "回放任务须使用持久锁",
  "content": "...",
  "sources": [{"kind": "memory", "issue_id": "uuid", "version": "7", "sha256": "64hex"}],
  "evidence": [{"kind": "run_record", "run_id": "uuid", "record_id": "stable-id", "verification": "observed"}],
  "applicability": {"surfaces": ["issue", "chat", "quick_create", "autopilot", "collaboration"]},
  "expires_at": null,
  "supersedes_candidate_id": null,
  "derived_from_candidate_id": null
}
```

`candidate_id` 与 request_id 均由客户端在发送前保存，服务器验证 UUID/原键绑定，避免创建响应丢失后没有可恢复地址。sources 必须非空；全部 source/evidence 属于同一个原 source issue，引用的版本、hash 和可用状态由服务器核对。commit/测试文字引用只能作为带 source anchor 的 evidence 数据，服务器不读取任意磁盘路径或外部 URL，也不声称验证了其内容。客户端的 verification 值是有作者/依据的声明；未经服务端独立验证不能标成 server-verified。由旧候选派生必须原 source 相同且旧候选当前可读；派生不会绕过原 source ACL。

approve body：`{protocol_version:1,request_id,expected_state_revision:"1",expected_version:"2",expected_content_sha256:"..."}`。reject/withdraw 也要求准确的 `expected_state_revision`，说明为 `reason`。每次新意图用新 request_id；不确定结果重试复用原 key。

receipt 固定为 `{protocol_version,request_id,action,code,candidate_id,version,state_revision,state,target_knowledge_revision?}`；没有正文、summary、reason 或 source refs。列表/详情的 capabilities 来自当前服务器计算，缺失字段代表 unsupported/unknown，不能默认可批准。

recovery endpoint 只要求当前可认证的原 actor；不以当前 source/target 角色为前置。服务器从 candidate ID 查内部 source scope，再按 `(actor,request_id,scope)` 查 operation，并严格核对 kind、receipt.candidate_id/action/key。不存在或不属于该 actor 均返回 `operation_not_found`；不返回 source ID、隐藏目标身份或任何正文。已经撤权的写调用不获得新权限，只能读取本人原内容无关结果。candidate 行和 operation 在历史保留期内不可物理删除，否则恢复合同必须先有等价的内部索引替代。

主要错误码：

| HTTP / code | 条件 |
| --- | --- |
| 400 `invalid_knowledge_request` | 非法 JSON/未知字段、版本格式、scope/ref 组合、UTF-8/NUL |
| 413 `knowledge_candidate_too_large` | 单版本/元数据/HTTP 字节预算超限 |
| 404 `knowledge_candidate_not_found` | 不存在或调用者无权获知；不能区分私有存在性 |
| 404 `knowledge_target_not_found` | 任意目标不可见/不存在，或非成员提交的 source project 不再是当前合法可见引用；不区分隐藏目标存在性 |
| 403 `knowledge_permission_denied` | 已能看到目标动作但当前角色不允许；机器写入同样拒绝 |
| 428 `knowledge_revision_required` | 状态动作缺 expected revision/hash |
| 409 `knowledge_revision_conflict` / `knowledge_content_changed` | 不是实际审阅版本 |
| 409 `knowledge_state_conflict` | 非法状态转换或旧版已 superseded |
| 409 `knowledge_source_unavailable` | source 引用已清除/删除/不能按其原版本还原 |
| 409 `knowledge_expired` | pending 已过期，不能批准 |
| 409 `knowledge_target_budget_exceeded` | 批准后的目标集合超过数量/材料预算 |
| 409 `idempotency_conflict` | 同 key，不同 canonical payload/action |
| 503 `knowledge_busy` / `knowledge_unavailable` | 锁/序列化重试耗尽或依赖读取失败；复用原 key，不装作空集合 |

`expires_at` 可空表示无自动过期；非空必须是未来 UTC 时间。修改有效期也是新内容版本，不能在已批准行上延长 TTL。自动过期无需后台任务才能生效。

## 5. 一个写事务及锁序

确认 source 和 target 都在同一 PostgreSQL 事务中，不串联两个各自 commit 的服务。READ COMMITTED + 显式锁后重读；有限重试沿用现有 40001/40P01/55P03 处理，失败不发布外部效果。

固定顺序：

1. 已验证 actor 的 user row（`LockCollaborationUsers`），核对账户/password epoch。
2. source 与 target workspace 去重后按 UUID 排序取 `FOR KEY SHARE`，阻止删除；按相同 workspace 顺序取 actor 的 subscriber/member fence。仅在该动作/目标路径要求 membership 时才将缺 member 行判为拒绝；来源 workspace 或合法可见当前 source project 的 pending 提案允许外部 handler 无 member 行。批准的 target 角色必须在 `LockActiveMember` 的 `FOR SHARE` 后读，不能只用 `FOR KEY SHARE` 保护非 key 的 role 更新。
3. source status catalog 共享锁；目标 project 取所需的 NOWAIT 行锁，pending 提案仅锁读合法可见 ID/归属，不能多读私有目录。只有批准/撤回等生效集合变更才取得目标知识 revision 更新需要的锁；proposal 不读取或改写目标上下文集合。所有锁升级争用均 NOWAIT 后整体回退。
4. source issue → issue_collaboration → collaboration_session，复用 source writer 顺序；随后重新解析当前 creator/handler、source Read 和全部 target facts。
5. lineage root 及涉及的新/旧 candidate 行按 UUID 顺序锁；读取原 operation，再校验 request/hash/revision/source versions，写 candidate/state/target revision 和内容无关 event/receipt，一次 commit。

理由：project 删除先锁 project 再锁关联 issue；既有 source 结构写在持有 issue 后使用 `LockProjectForAssociationNowait`，不会无限等待反向 project 锁。本草案同样要求 NOWAIT + 完整事务回退，必须用两种锁序测试确认，不通过局部重试“修复”已经失去的快照。

source 与 target 取自数据库/已验证 proposal，不能信任请求 workspace header 替换 source。participant、memory/comment source 的重新校验在上述 source 锁后；新事件只写引用/版本。没有权限的 replay 只可通过 candidate ID + 原 request ID 的本人 receipt 恢复接口取得其原内容无关结果；不用调用者已不可见的 source ID 构造 URL。

## 6. 受控贡献的消费与不扩权

新增统一 `ResolveApprovedKnowledge`，输入必须来自服务端已解析的 task/runtime/source/target，不接收客户端自报 user 或 audience。输出为完整有界 typed bundle、eligible manifest hash、目标知识修订和不注入原因。按 target/source/current audience 过滤，不把 whole target 文本或 raw workspace context 当候选来源。

| 消费表面 | 必须走的路径与授权主体 |
| --- | --- |
| 第一方 ordinary issue / mention / assign / rerun | 从 DB task 的 `OriginatorUserID`、已准入 runtime.OwnerID 和目标 issue 所属 workspace 解析；两个人均须当前能读 source。输出对 workspace 可见时，按 ACL 策略证明 target 读者包含于 source 读者；同 source workspace 是可证明的直接路径，不能只因跨 workspace 的当前成员碰巧相同就宣称长期包含 |
| 第一方 Web/Desktop Chat | 仍走同一 resolver；另核对 immutable ChatSession.CreatorID、当前 private-agent gate 和选择的 project。creator/真实 originator/runtime owner 均必须满足 source 当前权限；不能丢掉 chat claim 或用 session installer 代替真实发送者 |
| quick-create | 在产生普通 issue 之前按将要发布的 workspace 输出范围判断；不能用“尚无 issue ID”跳过 source/target 检查 |
| autopilot/manual/schedule | 同一 resolver；使用真正的 OriginatorUserID（包括现有 trigger-owner 授权），不使用 AccountableUserID。NULL originator、不可证明的输出目的地不注入私有贡献 |
| collaboration run | 接入已稳定的 handoff/snapshot producer；按当前 issue 的受权读者/已受理输入来源和 actual runtime resource owner 判定，不继承来源 workspace/project 的全部能力 |
| 外部频道转发/未知 audience | 不从 installer/显示作者推断全部受众；无法证明 source 可见范围包含该受众时返回明确 `audience_unavailable`，不携带私有正文。第一方 Chat 仍必须实现 |

可信字段：`AgentTaskQueue.OriginatorUserID` 是授权人；`AccountableUserID` / `OriginatorSource` 是审计数据。runtime owner 只限定执行凭据，不授予 initiator 更宽的权限。当前 human GET 使用 request Principal；后台 claim 用服务端构造的 typed consumer facts，不在 HTTP context 中伪造一个人类登录来复用 helper。

不注入原因固定枚举：`unsupported_client`、`missing_originator`、`missing_runtime_owner`、`source_access_denied`、`source_unavailable`、`target_not_applicable`、`audience_unavailable`、`expired`。未获权的一项不能泄漏 candidate ID/summary/source ID；详细拒绝原因只保留在有权限的服务端审计。暂时 DB/依赖错误返回 503，而不是把它当合法的无贡献。

### 6.1 所有人类投影共享 source availability（K4）

先执行统一的 `KnowledgeSourceAvailability`，再序列化任何候选正文/摘要/可能包含引文的 evidence 说明。required sources 指提案 `sources` 数组的全部引用，不能由客户端事后降为 optional；用来派生正文的 evidence 也必须列入 sources。它同时供 source list、candidate detail、每个历史版本、target active/history projection 和 runtime resolver 使用：

- 调用者无当前 source Read：普通正文接口隐藏该对象；target-only 管理者仅得到允许的 restricted 控制元数据。
- 任一 required memory/comment/run source 被 purge/delete，或指定版本无法依法还原：候选不可变副本的 body、summary 及派生正文说明全部省略，返回 `source_unavailable` 和允许的作者/版本/状态/时间等审计元数据；不能因 source membership 仍在就把副本吐回。
- 普通 withdraw、expiry、supersede 只使未来生效集合不可用；若 source Read 与 required source availability 仍成立，合法的人类历史正文可以保留。它们不等于 source purge，不新增物理擦除操作。
- target knowledge_revision 不能替代 source ACL/purge/expiry 变化。缓存收到 source access/content 变化时须取消旧请求并清除不再可读正文；重新聚焦/轮询也走当前服务端 policy，不让过期 cache 或晚到响应恢复隐藏正文。

## 7. 普通 claim、独立披露、启动与精确 session 续作

当前真实 seam 是 `agent.ExecOptions.Lifecycle` 和 `Daemon.executeAndDrain`（`daemon.go:8434`），不是 `agent.Options` / `executeWithBackend`。`StartTask` 在 `daemon.go:7569` 已把任务置为 running，然后才构造/执行最终输入。`RequeueTaskAfterClaimFailure` 仅适用于未交付、未 StartTask 的 dispatched claim，本设计不复用它处理启动知识拒绝。

### 7.1 Finalized claim 不披露私有知识

新 daemon 按实际 runtime/backend/mode 协商 `knowledge_context_version=1`。普通单次与批量 claim 的 response 只返回 source-free descriptor：支持版本、task/runtime/claim generation、有效 provider/mode、是否需要后续 preparation（由协议/backend 支持决定，不取决于私有候选是否存在）。它不含 candidate/source ID、summary、正文、hash 或私有集合数量。

`FinalizeTaskClaim` 沿用现有 password/runtime→task→execution admission/token/receipt 顺序，仅在原 task 锁内写 source-free negotiation/claimed 阶段。**不在这个事务增加 workspace/source/candidate 锁，不在 claim builder 预取私有正文**。finalization 失败、bundle 还未准备或 claim response 丢失均没有知识正文泄漏。旧/不支持 backend 继续普通任务的原有 base 执行，不调用私有 preparation，不因知识功能全局存在而失败。

### 7.2 私有正文通过独立 preparation 事务披露（K2）

新增 `POST /api/daemon/tasks/{taskId}/knowledge-context/prepare`，请求为 `{protocol_version:1,request_id,claim_generation,dispatched_at,resume_intent}`。resume_intent 只能是该 finalized claim 已给出的 exact session descriptor，或 fresh；source/target/consumer 身份不接受客户端自报。server 从实际 task、runtime、agent、Chat/issue scope 解析，并核对 effective provider/mode。new preparation 发生在成功 finalized claim 之后、最终 BuildPrompt 之前；允许当前 task 已为 running，但要求尚无本 logical prompt 的物理 launch。

事务前的 task/target/candidate **元数据**读取只用于确定有界锁集合，不披露正文。事务采用 READ COMMITTED，下列锁均须 NOWAIT 或同 namespace 的 try-lock；竞争最多复用原 key 做既有有界传输/事务重试，耗尽返回 503，不持锁等待另一个相反图：

1. 有序的实际 originator/runtime owner/第一方固定读者 user 与 credential rows，重验 active/password/binding 事实。
2. 所有涉及的 source/target workspace 按 UUID 取 `FOR SHARE NOWAIT`；它先于任何 chat/task 锁，且与 workspace 删除的 UPDATE 冲突。member/subscriber fence 使用现有 namespace 的 nonblocking try-lock，member rows `FOR SHARE NOWAIT`，角色锁后重读。
3. target project（如有）`FOR SHARE NOWAIT`；Chat row（如有）`FOR SHARE NOWAIT`；source issue→issue_collaboration→collaboration_session 按稳定 source ID 顺序取共享 NOWAIT 锁；required source version 和候选行共享 NOWAIT 锁。全部 source/purge/expiry/target revision/audience 在锁后重新检查；hint 集合不再一致就整体回退，不能补拿逆序锁。
4. 最后才取实际 managed runtime/binding 与 task 行的 NOWAIT 锁，重新核对 task/runtime/agent/scope、dispatched_at、claim_generation、negotiation mode 与合法 status。只更新专用 metadata 列，不改业务输入；在这里不再调用会隐式等待 workspace/source 的 helper。
5. 组装完整有界 bundle，存 `prepare_id + original key/hash + exact manifest + disclosure_authorized` 元数据并 commit；**commit 后**才写正文响应。此事务是向 daemon 披露正文的授权线性化点，不能称为“尚未交付任何私有数据”。

这个顺序有意独立于 FinalizeTaskClaim：如果 finalizer 已持 runtime/task 并等待 source execution row，preparation 在最后拿 runtime/task 时 NOWAIT 失败并释放 source；如果 workspace-delete 已持 workspace，则 preparation 在最早的 workspace 步骤即失败，绝不先持 task 再等 workspace。需新增或复用准确的 NOWAIT/try-lock 查询，不能把现有 blocking 封装直接放进此顺序。

lost preparation response 重试同 request key：原 manifest 不漂移，**再次披露前仍重验当前资格**；withdraw/revoke/purge/expiry 后只返回 refusal，不能凭旧 receipt 再发送正文。重新 claim generation 不继承旧 preparation 权限。响应写出失败保留 disclosure-authorized 审计，保守按可能已送达 daemon 处理；不伪称有网络 ACK 或 Provider 理解证明。

### 7.3 per-launch freshness 与明确终态拒绝（K3）

新增 `POST /api/daemon/tasks/{taskId}/knowledge-context/authorize`：`{protocol_version:1,request_id,prepare_id,manifest_sha256,prompt_sha256,claim_generation,attempt_seq,attempt_nonce,session_binding}`。它用 7.2 的独立顺序和当前 actual task/runtime/consumer/source/audience 重新核对，不返回正文，只写内容无关授权结果。任何重试必须绑定同一 identity/hash；不创建新的业务 input。

支持该能力的 effective backend 用 `agent.ExecOptions.Lifecycle` / `LaunchObserver`，在每次 provider 物理 start 前检查；preparation/helper launch 不消费知识。Callbacks 记录真实 Started/termination，普通 task token 不由 observer 换成更宽凭据。当前 HostLifecycleSupported 仅覆盖允许的 local provider 组合；不支持的组合从 negotiation 就不披露私有贡献，普通任务继续原路径。unsupported effective backend 不复用任何已知 managed exposure 或损坏 negotiated history 的 exact session，先选 fresh ordinary session；只有已证明无 managed exposure 的 legacy 会话保持旧 resume 行为，不能通过历史上下文重新取得本次未获支持的私有贡献。

选定失败策略是**有界终态拒绝**：`knowledge_context_changed` / `knowledge_authorization_denied` 从 Backend.Execute 或 session result 经 `executeAndDrain` 返回普通 task failure 路径，保留当前 task ID/claim generation、记录、usage、取消与会话事实。新增准确、非自动重试的 failure classification，且排除 `shouldRetryWithFreshSession` 和后台 MaybeRetryFailedTask 对该拒绝的自动重放；不能调用 dispatched-only RequeueTaskAfterClaimFailure。

- 第一物理 launch 前拒绝：标明 `physical_launches=0`、知识授权已变化；任务正常失败，用户可用既有显式 retry/rerun 流程重新取得当前上下文。retryable 指允许明确的重试意图，不表示系统自动重排 running task。
- 已有物理 launch 后、adapter 后续 launch 被拒：停止后续 start，保留已发生的副作用/输出/usage/termination。不能把它当从未执行而重新发送整个业务输入，也不能为过关自动换成 base-only prompt。
- 原有 backend 在非知识错误下的 retry/fallback 行为不被重新授权；每个实际新 launch 都仍需当前知识检查。首次拒绝与后续拒绝必须有真实 ordinary-entry fake executable 测试。

preparation 与 launch authorization 是两个不同的线性化点。撤回在 disclosure 前提交则没有正文响应；disclosure 已获权、但 launch 前撤回则 daemon 可能已合法收到文本而旧 prompt 不能启动。已经合法交付的历史输出和私人 Provider 历史不会被本功能擦除。

### 7.4 按 exact session 的累计 exposure 选续作（K1）

`GetLastTaskSession` 的实际返回 session、manual rerun 的精确 source session、Chat 的直接指针与 `GetLastChatTaskSession` fallback 都必须转成同一种 `KnowledgeSessionBinding`，再验证累计 exposure；不能先验证最新 task、随后恢复另一个 session。

- 绑定包括 provider、runtime、native session ID、scope kind/ID、generation。generation 不能从任意最近 task 猜出；新 fresh logical session 由 server 分配，后续只有匹配的同 scope/session 可以继承。
- 累计集合保留该 session 全部已披露/可能已投入的 candidate version/hash 与授权来源。至少覆盖 same-session 多 task、cancelled/failed task、没有回传新 session ID 但原 Chat 指针保留的 turn、历史较早 turn 中的贡献。metadata-only pending resume binding 也要参与查询。
- 新 fresh session 在 native ID 尚未知时只记录受控 generation，不能供 resume；native ID 首次被 pin/complete/fail 回传时，pointer 与该 session 的 exposure binding 同事务持久化。内部 fallback 改成另一 native session 时，两者的关系必须可观察并保留，不能把旧 S 的 manifest 自动贴到新 T。
- 在接收私有 preparation 前验证所选 session 全部历史 exposure 的当前 source/target/expiry/audience 资格。任何一个已失效或历史无法证明，就 retire/refuse 该 exact session；在当前业务输入尚未物理执行时走既有 fresh-session 选择，绝不复用其私有历史。launch 时才发现变化则走 7.3 终态拒绝，不进行自动 fresh replay。
- legacy **从未协商且已证明没有 managed exposure** 可继续既有会话；version=1 的已披露阶段缺失/损坏 metadata 则 fail closed。不能只把 NULL 一律当空集合或一律当坏历史。
- 累计 metadata 必须有界；超过现有约定的 32 项/64 KiB，不能截断证明链。下一次输入尚未执行时选择新 generation；已运行中的知识拒绝仍终态收尾。

实现仍需证明：各 provider/native-session 分支能提供上述 exact binding，所有会写 session 指针的 pin/complete/fail/chat fallback 路径能原子维护 exposure。当前 LaunchInfo 不能凭其 Provider/Kind 自动推导 native session；若 adapter 的内部 session 切换无法观察，该组合不得宣称支持私有 managed knowledge。此项保留为 G2 contract test 门，不假称仅新增 per-task JSON 就已解决。

## 8. UI、迁移与回退

- core 新增 typed schema/API/Query；源列表 key 含 issue ID，目标列表 key 含 target kind/workspace/project 与分页参数；服务器状态不放入 Zustand。
- source 详情提供“提出候选”、版本链、确认/拒绝/撤回及冲突重读；权限由 capabilities 控制且服务器兜底。target 页面独立展示 managed contributions，base context editor 不拼入这些正文。
- 所有新增边界经 parseWithFallback / 严格 Go decode；旧字段缺失为 unsupported，声明支持但响应畸形必须明确失败。双端复用 views，en/zh-Hans 同步，自动 skill 写入不存在。
- migration 首文件创建 candidate 表和新增列，不内联 PRIMARY KEY/UNIQUE 自动建索引；id 唯一、lineage/version 唯一、lineage approved 唯一、source 分页、approved target 查询等索引分别单语句 CONCURRENTLY 创建。不要猜 migration 编号。
- project/workspace 删除显式处理 target contributions，不改写 source 审计；workspace 已有 collaboration records 删除拒绝要保持。已持有候选历史或 delivery metadata 时 down migration 拒绝丢数据。
- 回退先停新 writer/新知识 producer，保留 reader 与已保存来源/状态；不能回退到把新内部事件当错误的 reader，也不能把批准内容复制进 base strings 作为兼容 fallback。

## 9. Main-review checkpoints / caveats

- 源/目标权限不扩张、人工确认、skills 后置均已决定；本文没有待用户重新确认的扩权选项。
- 撤回双侧管理、目标集合预算、普通 claim negotiated lifecycle 属于本文给主会话复核的技术选择。必须通过矩阵和实际 stdin 测试后才可以升级为正式实施合同。
- 状态机/预算常量及接口数量均服务于具名要求：候选不可变、审批、撤回、原键恢复、目标展示和真正启动检查；不引入通用 KB/API 引擎。
- Related code/spec evidence: 同目录 preflight；`internal/collaboration/access.go`、`collaboration_memory_operation.go`、`handler/project_write_fence.go`、`queries/subscriber.sql:113`、`handler/daemon.go:1739/1909/2150/3506`、`attribution/attribution.go:147`、`agent/launch_lifecycle.go:83`；相关 `.trellis/spec/server` 文档同 preflight。
- External references: 本草案不选择外部 SDK/服务；未进行外部检索。
