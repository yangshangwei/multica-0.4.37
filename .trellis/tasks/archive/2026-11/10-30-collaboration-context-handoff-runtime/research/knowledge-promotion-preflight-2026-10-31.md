# Research: 知识候选逐层沉淀的前置契约与最小设计

- Query: 在可靠交接契约稳定后，如何复用现有项目/空间上下文、协作权限与审计设施，实现候选→人工确认→受控生效→纠错/过期/撤回？
- Scope: internal；只读代码研究及本文件写入，没有执行测试、数据库操作、真实 Provider 或 git 操作。
- Date: 2026-10-31
- Target task: `10-30-collaboration-knowledge-promotion`；当前仅向 runtime 集成主会话提供前置研究，不代表该子任务已开始实施。
- Snapshot caveat: 主工作区的 runtime handoff/recall/checkpoint 实现仍在变化；本文把既有稳定路径和新的未验收契约分开说明。

## Findings

### 1. 已确认的产品边界

依据 sibling `prd.md` 和父任务 R5/AC8：

- 发起人或当前处理人可以提出；普通参与者有评论权不代表可以提出。
- 项目候选由目标项目负责人/管理员确认；空间候选由目标 workspace owner/admin 确认。
- 确认必须是人，并同时具有来源可见权与目标权限；Agent 担任项目 lead 不等于人工确认。
- 主会话已再次明确：确认不扩大来源可见范围；消费时仍须重新检查来源可见权与目标适用性/权限。这是既定产品边界，不是待用户重新选择的授权方案。
- 未确认不得影响实际上下文；候选与生效内容分离，保留来源、版本、适用范围、验证证据及纠错/撤回链。
- 撤回、过期和来源权限变化必须作用于后续注入，不能只改 UI 状态。
- 无自动 skill 写入、通用知识库、向量检索、跨 Server、服务端 LLM 总结或新依赖。

### 2. 文件地图与可复用代码

| 文件/位置 | 已确认行为与用途 |
| --- | --- |
| `server/internal/collaboration/access.go:18`、`:23`、`:79` | 从已认证 context 建 Principal；Human 排除 restricted/AgentActor/机器凭据；Read、Comment、ControlAI 与 creator/handler 管理权分别计算 |
| `server/internal/service/collaboration.go:68` | `ResolveIssueAccess(ctx,q,issueID)` 读取当前数据库权限，校验账户状态/密码版本，不因持有 UUID 而允许读取；返回原 issue、版本、来源空间与 capabilities |
| `server/pkg/db/queries/collaboration.sql:1` | `GetCollaborationAccess` 区分 source member 与未撤销 participant；原作者退出与当前消费者权限不是同一事实 |
| `server/internal/service/collaboration_comments.go:101` | `readCollaborationContent` 在授权的 repeatable-read read-only 事务内做内容投影 |
| `server/internal/service/collaboration_memory_operation.go:29`、`:82` | 人类写入的原 request key/hash、锁后重新授权、冲突重试、内容无关 receipt 模式；不能照搬其中较宽的 ControlAI 写权限来批准知识 |
| `server/pkg/db/queries/collaboration.sql:32`、`:62` | 可复用 `collaboration_operation` 原键回执以及 issue event sequence / `collaboration_event` 审计 |
| `server/migrations/571_collaboration_schema.up.sql:123`、`:138` | operation scope 仅允许 `create/issue`；kind 为 text。event kind 也是 text；可加入知识动作种类，不必另造通用 workflow 引擎 |
| `server/internal/service/collaboration_memory.go:85`、`:147` | memory 版本追加，UTF-8/字节上限、stale version；purge 清正文而保留字节、作者与时间历史 |
| `server/pkg/db/queries/collaboration_memory.sql:13`、`:23`、`:44` | 指定版本读取、仅元数据历史页、purged_at 事实；候选引用必须携带明确 memory version 并检查 purge |
| `server/internal/service/source_context.go:263` | `BuildSourceIssueSnapshot` 在组装正文/附件前拒绝 collaboration source（`:275–280`）。现有普通任务 capture 不是知识沉淀入口，不能为方便而解除该边界 |
| `server/internal/handler/project.go:609`、`:744` | 项目修改在事务内锁项目；description 改动必须带 `expected_description_revision`，缺失 428、冲突 409 |
| `server/pkg/db/queries/project.sql:46` | 项目 `revision` 每次修改递增；`description_revision` 只随 description 实际变化递增；现有项目原文已有 CAS，但没有知识候选/审批服务 |
| `server/internal/handler/project_write_fence.go:49`、`:74`、`:109` | 人类 actor 校验、workspace/member 写入 fence、事务重试；是目标授权锁的模式，不应与 source 写事务各自提交 |
| `server/internal/handler/project.go:849` | `requireProjectAdministrator` 只允许 workspace owner/admin，且错误文案专用于删除项目；不包含已批准需求中的 member lead，应实现准确的确认策略而非直接借用此函数 |
| `server/internal/handler/workspace.go:323`、`:374`；`server/pkg/db/queries/workspace.sql:47` | workspace context 是直接更新的 text，没有 expected revision/CAS；不能假设与 project description 等价 |
| `server/cmd/server/router.go:1905` | workspace 更新路由使用 owner/admin middleware；候选写入还需服务层重新校验来源权限和目标角色 |
| `server/internal/handler/project_resource.go:917`、`:965` | `resolveClaimProjectContext` 为 issue/chat/autopilot/quick-create 共用项目解析；项目和资源按 workspace 过滤，DB 错误与无项目明确区分 |
| `server/internal/handler/daemon.go:3259` | workspace.context 面向每种普通 task claim 整体注入；它没有逐贡献来源可见性过滤 |
| `server/internal/handler/agent.go:397`、`:434` | 普通 claim DTO 的 WorkspaceContext、ProjectDescription、ProjectResources；新 typed 受控贡献需要经过 claim→daemon 的完整边界 |
| `server/internal/daemon/daemon.go:6966`、`:6986` | 把 claim 上的项目/空间上下文搬到 execenv；这里不应自行猜授权 |
| `server/internal/daemon/execenv/runtime_config_sections.go:184`、`:418`、`:932` | 空间和项目原文进入 runtime brief；per-turn 状态原则上应使用请求输入而不是污染稳定 brief 前缀 |
| `server/internal/daemon/execenv/context.go:288` | project description 还会写入 `.multica/project/resources.json`。直接拼入 base description 会产生第二个可恢复的正文副本 |
| `server/internal/handler/daemon.go:2394`、`:2412`、`:2707` | runtime owner、真实 initiator、Provider resume 是不同身份/生命周期；新贡献的消费者策略必须区分这些角色 |
| `server/internal/handler/daemon.go:1875`、`:1909`、`:3506` | 普通 task token 的实际用户绑定是经服务端解析的 runtime.OwnerID，缺 owner 明确拒绝 claim；不能接受客户端自报 consumer user |
| `server/internal/attribution/attribution.go:147`、`server/pkg/db/generated/models.go:271` | `OriginatorUserID` 才是授权人；`AccountableUserID` 是审计身份，即便非空也不能作为授权回退 |
| `server/internal/handler/chat.go:252`、`:281` | 第一方 Chat transcript 由 CreatorID 和当前 private-agent access 共同保护；CreatorID 不等于外部群聊的全部收件人 |
| `server/internal/service/task.go:1181`、`:2131`；`server/internal/handler/daemon.go:2756`、`:2850` | 已有 force-fresh / channel context revision 的新会话路径，可研究用于失效知识曾进入历史会话后的安全续作 |

### 3. 对现有数据流的结论

普通运行当前是：`workspace.context / project.description+resources → claim DTO → daemon TaskContextForEnv → runtime brief / project sidecar`。项目选择、仓库优先级与 workspace 边界已有规范，但不存在“按来源协作单权限过滤每段生效知识”的层。

因此，**不建议把批准后的候选正文直接追加到 workspace.context 或 project.description**：

1. 普通 workspace/project 读者及运行会得到整体字符串，而候选的 source ACL 可能更窄。
2. 文本被后续人工编辑后，无法可靠按 candidate ID 撤回其中一段；复制进 resources.json 又形成额外副本。
3. workspace 没有同等 revision，简单读-拼接-写可能覆盖其他修改。
4. 过滤下一次 payload 不会让已恢复的 Provider 历史遗忘旧正文；resume 策略也要纳入验收。

推荐保留原有人写的 base context，并把人工确认的候选作为**独立、受控的上下文贡献**。项目/空间“生效”指它进入对应目标的可授权注入集合，而不是改写整个用户原文。目标页面需把这些贡献和 base 文本分开展示，避免把受控正文复制回无来源过滤的自由文本编辑器。

## Proposed minimal design

以下为待主会话合入 design.md 的建议，不是已实现的接口。

### A. 最小存储与状态

优先新增一张 issue-owned `collaboration_knowledge_candidate`，复用现有 operation/event 账本，不另建通用审批引擎：

- 身份/目标：`id, source_issue_id, source_workspace_id, target_kind(project|workspace), target_workspace_id, target_project_id`。
- 不可变提案内容：`content, summary, content_sha256, sources, evidence, applicability, proposed_by_user_id, proposed_at`。
- 来源：typed、有限数量的引用，至少含 source kind、issue/run/comment ID、memory/comment version、hash、验证状态。服务端验证全部引用属于同一受权来源，不能照单全收客户端声称的版本和验证结论。
- 状态：`revision, state(pending|approved|rejected|withdrawn|superseded), approved_by_user_id, approved_at, expires_at, supersedes_candidate_id`。过期与 source unavailable 的实际可用性在读取/注入时计算，后台任务不是安全前提。
- 纠错创建新候选，不原地改已确认正文。新候选被确认时，原候选的失效和新候选的生效在同一事务完成；未批准的纠错不提前替换现有有效版本。
- 事件只记录 candidate/target/version/动作/操作者等必要元数据，不在事件中重复正文；原 operation receipt 同样不携带正文。

目标集合需要可审计版本。可在 workspace/project 增加独立 `knowledge_revision`（与人工 base description/context 修订分开），在 approve/retract/supersede 时原子递增；返回每次实际投影的 `target_knowledge_revision + candidate IDs/revisions + source hashes`。不要谎称现有 workspace 已有可复用 context revision，也不要让知识状态变更伪装为 project description 修改。

候选输入建议沿用已有 UTF-8/NUL/解码后字节约束和独立 HTTP 编码上限，边界常量在正式设计中集中确定；不要把 JSON 转义字节与正文预算混算。候选不是大文件仓库，附件/日志只保留授权引用。

迁移遵守仓库硬规则：无外键/cascade；每个索引单独 `CREATE [UNIQUE] INDEX CONCURRENTLY` 文件；编号在实施时选择空闲前缀。候选 source 列表与已批准 target 查询需要专用索引。删除项目/空间时由应用显式处理受控贡献；有历史记录时的 down migration 不得静默丢审计。

### B. 写入接口与授权

建议保留 issue 为候选的主入口，复用现有 collaboration 路由风格：

```text
GET  /api/collaboration/issues/{issueId}/knowledge-candidates
POST /api/collaboration/issues/{issueId}/knowledge-candidates
GET  /api/collaboration/issues/{issueId}/knowledge-candidates/{candidateId}
POST /api/collaboration/issues/{issueId}/knowledge-candidates/{candidateId}/approve
POST /api/collaboration/issues/{issueId}/knowledge-candidates/{candidateId}/reject
POST /api/collaboration/issues/{issueId}/knowledge-candidates/{candidateId}/withdraw
```

实际 API 前缀应在实施时与当前 router 核对；这里不新增顶层页面或不透明的通用 action 分发器。每个写请求携带 `request_id` 与准确的 `expected_candidate_revision`，审批还携带用户实际审阅的内容 hash/版本；重复原 key/hash 返回同一 receipt，不重复发布。

权限必须在**同一数据库事务**中重新校验：

- 提出：已认证 human + 当前 source Read + 当前有效 creator/handler 管理角色。不要用 Comment 或 ControlAI 代替；后者允许更多参与者。完成的协作任务仍可能产生知识，不能从 SubmitCommand 的“任务必须开放”条件继承限制。
- 确认：已认证 human + 当前 source Read + 当前目标 workspace membership + 目标 owner/admin；项目还允许 `lead_type=member && lead_id=actor`。agent lead、伪造 actor header、task/plugin/IM/termination token 均不能确认。
- 接口的 source/target UUID、项目归属、候选版本和内容 hash 都必须在锁后匹配；不得把预览时的权限/角色当作长期授权。
- 不应嵌套调用 source 与 project 的两个各自提交的事务封装。复用锁 primitives，在正式设计中核对与现有 project association/delete、source revoke 的统一锁序，加入两种争用顺序测试。
- source 读权丢失后仅允许调用者自己的内容无关 operation receipt 恢复，不能通过旧 request ID 取回正文。

### C. 生效读取与实际注入

建议新增一个明确的、纯服务端决策的 resolver，而不是在 execenv 做权限推断：

```text
ResolveApprovedKnowledge(ctx, transaction,
    targetWorkspaceID, targetProjectID,
    serverResolvedConsumerIdentity, serverResolvedOutputAudience)
  -> bounded typed items + target revision + per-item provenance/availability
```

每次消费重新检查 candidate approved 状态、expiry、supersede/withdraw、来源仍存在且相应版本未 purge、当前 source 读权、目标适用范围及已确认的消费主体/输出受众。查询失败应明确失败或明确“知识不可用”，不能复用上一次缓存的私有正文冒充当前结果。

- `resolveClaimProjectContext` 是共享项目路径，可复用其错误与跨 workspace 约束，但现签名只有 project/workspace，没有 consumer identity；不能原样拿来完成新策略。
- 普通 claims 的 issue/chat/autopilot/quick-create 都要经过同一策略；无法证明消费者/输出受众安全的表面，默认不注入该受控贡献并清楚标注范围。
- 新 payload 使用 typed `approved_knowledge`/manifest，与 base strings 分开。daemon/execenv 只渲染服务端已准入材料，保留来源、版本和验证状态；“人工确认”不能授予工具或协作控制权限。
- 回收影响下一次实际启动，而不只影响列表。claim 后、进程启动前的撤回/撤权窗口必须明确：可复用 runtime 已验证的 snapshot/bind 模式，但不得直接借用 collaboration business token 给普通任务扩权。
- 若之前会话曾收到如今不再可用的贡献，应通过已记录 delivery manifest 检查安全续作；必要时不复用旧 Provider session。仅从新的 prompt 删除文本不足以证明旧会话不再使用它。现有 `force_fresh_session`/channel context revision 可作为模式，具体跨 issue/chat 的持久化点还需实施前检查。
- 不宣称可以删除 Provider 私人历史或让已在运行的模型遗忘内容；这与父任务“不扩大清除能力”的边界一致。

**已存在的可信主体与建议映射**：

| 运行信息 | 服务端可信来源 | 新贡献消费规则 |
| --- | --- | --- |
| 执行凭据所属人 | 已准入的 `db.AgentRuntime.OwnerID`；单次/批量 claim 均将它写入 `CreateTaskTokenParams.UserID` | 检查该人的当前 source Read 与 target membership/applicability；不能把 daemon token 自身视为 human source 读者，也不能借 resource owner 扩大发起者权限 |
| 本次业务授权人 | 已入队任务的 `OriginatorUserID`；`attribution.Result.UserID` 是明确的授权字段 | 作为发起请求侧的权限边界再次检查；NULL 时不使用 owner_fallback、rule publisher 或 AccountableUserID 补出授权，受控贡献不注入并给出明确 reason |
| 直接消息发起者 | `InitiatorUserID`；评论触发的实际作者由服务端读取 comment row，Agent 作者通过原已验证 originator 链确定授权人 | 用作来源/审计与输入作者一致性校验，不以显示名、任意 actor header 或历史作者字段取代 originator 授权 |
| 第一方私有 Chat 的读者 | `ChatSession.CreatorID` 加当前 private-agent 访问规则（`chat.go:252–296`） | 读者也必须能读 source；不可仅检查 runtime owner。频道型 Chat 还要考虑真实外部投递受众，不能用 installer/creator 代替整个群 |
| 普通 issue / 项目运行的输出读者 | 目标 issue/workspace 的当前可读集合 | 源码现有普通 claim 没有逐贡献 audience 证明；新 resolver 必须建立“目标读者集合是 source 可读集合的子集”的证明。可用集合查询/反连接验证，无法证明时不注入该贡献 |
| 审计责任人 | `AccountableUserID`、`OriginatorSource`（`attribution.go:153–185`） | 仅保留归因，不参与授予 source 读权；尤其 `originator=NULL, accountable!=NULL` 必须覆盖负向测试 |

同 source workspace 的目标可以利用现有 source-member Read 规则作受众校验的快速路径；跨 workspace 不是一次人工确认就能绕过交集。不得自行把所有跨 workspace 用例标为已支持，也不需要重新询问是否允许扩权：既定规则已经禁止扩权。缺少受众映射的外部渠道/系统任务应返回显式 `not_injected` 原因并不携带贡献正文；暂时查询失败则走可重试失败，不使用旧正文或伪造空集。

### D. 最小前端与边界文件

预计新增/修改范围（不是执行授权清单）：

| 层 | 建议文件/职责 |
| --- | --- |
| DB | 新 candidate migrations、`server/pkg/db/queries/collaboration_knowledge.sql` 及 sqlc 输出；workspace/project 的 knowledge revision；显式删除/历史保留查询 |
| Domain/service | 新 `server/internal/service/collaboration_knowledge*.go`：typed refs、状态机、源/目标授权、原键 receipt、有效贡献解析；适量复用 collaboration operation/event primitives |
| HTTP | 新 `server/internal/handler/collaboration_knowledge.go`，当前 router 注册；目标上下文的受控列表使用 human 权限接口 |
| Runtime | `handler/daemon.go`、`handler/agent.go`、`handler/project_resource.go` 的 claim 解析；`daemon/types.go`、`daemon/daemon.go` 与 `execenv`/per-turn prompt 的 typed material delivery；必要的 resume manifest 门禁 |
| Core | 新 `packages/core/collaboration/knowledge-schema.ts`、queries/API，使用 parseWithFallback，query keys 含 issue/target/workspace 身份；无需 Zustand 新 server store |
| Views | 协作详情的候选提出/审阅/版本链；项目/空间只展示符合 human ACL 的受控生效项；base 文本编辑器不吸收受控正文；en/zh-Hans 同步 |
| Guidance | 更新内置协作指导及 source map：可以提出与人工批准的区分，引用/验证语义，禁止自动 skill 发布 |

## Required meaningful tests

1. **提案无副作用**：pending 创建、编辑替代、拒绝均不改变 base context 或 runtime effective contributions；没有自动知识/skill 文件写入。
2. **权限矩阵**：creator/current handler 允许提出，普通评论者不允许；人类 member lead、owner/admin 的目标权限；agent lead、机器凭据、restricted/account disabled、伪造 actor、失去目标 membership 全部拒绝。完成态 source 的允许范围单独覆盖。
3. **source/target 双边界**：外部参与者、有目标权限但无来源读权、有来源权但无目标权限、错 workspace/project/source ref、读取 candidate URL 直接绕过列表，均不能泄漏正文或审批。
4. **并发与幂等**：同 key/hash 重试一次落库，改 hash 冲突；批准与 source revoke、target role change、candidate withdraw 两种锁顺序；审批页面 stale revision/hash 拒绝；未知 commit 恢复只返回原内容无关回执。
5. **纠错/过期/撤回**：新纠错未批准时旧版保持生效；批准时原子切换；过期无需 sweeper 也立即从 resolver 消失；撤回与正在组装/启动的 claim 竞争不得继续注入失效版本；历史链仍可按权限审计。
6. **来源可清除性**：memory purge、comment delete/revision change、run evidence unavailable 后，候选不能通过自己保存的正文或快照绕回已清除来源；原作者退出不是把历史作者改成 daemon owner，也不能自动毁掉仍受权的审计事实。
7. **真实交付边界的确定性证明**：用 test-created fake executable 检查实际 stdin/必要 sidecar，不仅断言 resolver struct；相同有效 manifest 可重取，失效贡献不在下一次输入，已有 session 的 manifest 变化不会复用不安全历史。
8. **兼容与字节预算**：旧 claim 缺失字段不误称支持；已声明字段畸形或错 identity 拒绝；long Unicode/JSON escaping、元数据计入预算，截断可召回且不把部分内容标完整。
9. **共享 UI**：确认动作 awaited；冲突显示当前版本；无权限只露出允许的状态/元数据；target base context editor 不会把受控贡献复制成自由文本。核心权限矩阵留在 service/helper，组件不重复全量矩阵。
10. **迁移与回退**：fresh/upgrade/reapply、索引规范/sqlc 无漂移；有 retained candidate/event 时 rollback 不丢审计；project/workspace 删除显式处理，不依赖 FK/cascade。

## Runtime dependencies: confirmed versus provisional

| 前置 | 状态 | 知识阶段如何使用 |
| --- | --- | --- |
| source issue ACL、human principal、memory immutable version/purge、原 operation/event ledger | 既有代码已确认，本研究未重新运行其测试 | 可以直接设计复用，但所有新增读写仍需新回归 |
| handoff typed sources（kind/ID/run/version/hash/availability） | `server/pkg/protocol/collaboration_handoff.go:123` 已有工作副本实现，尚待本轮 runtime 统一验收 | 建议复用来源词汇和校验思路，先固定正式合同，再引用这些具体结构 |
| snapshot 重建核对 source hash、purge/stale 拒绝 | `server/internal/service/collaboration_handoff_snapshot.go:25`、`:99`、`:211` 已有 WIP | 不把存在代码当作已证明；知识消费的权限主体也不同，不能直接接 runtime token |
| checkpoint 记录水位、缺失原因、成员/队长结果与 wakeup 来源 | runtime 本轮集成中 | 只从服务端已持久化且获授权的记录建立证据引用；缺失/未验证必须原样保留 |

## Caveats / Not Found

- 在本次定向代码/SQL 搜索中，没有找到已实现的 collaboration knowledge candidate、approval→target contribution、supersedes/retract 服务。现有 Switch proposal 是资源同意/切换合同，不能直接把它当知识审批实体；只复用幂等/锁后重验的模式。
- 消费主体/受众仍有工程实现缺口，但产品规则已经确定，无须重新询问是否可以扩权。正式设计应落成上表的可信 DB 主体映射、集合校验、无法解析时的不注入原因以及 source/target 变更时的重验。
- 撤回动作的策略应在实现设计写明：建议 pending 可由当前提案管理者撤回，approved 的撤回使用与批准相同的目标人类权限；来源授权失效由 resolver 自动阻止消费。不能把原作者退出自动解释为撤销全部历史批准，也不能因提出者后来失权让目标管理员无法撤回。这是既定权限范围内的保守动作映射，不需要引入新的授权角色。
- 上述建议只针对尚未开始的知识阶段。runtime 集成不应等待这些问题，也不应为此在本轮增加通用审批/知识实体。
- 外部资料：未进行外部文档检索；这里没有陌生 SDK/API 选型，依据是本仓库代码与明确的父/子任务要求。
- Related specs：`CLAUDE.md`；`.trellis/spec/server/collaboration-foundation.md`、`collaboration-source-editing.md`、`collaboration-host-execution.md`、`collaboration-records.md`、`project-execution-squad.md`、`security-boundaries.md`；core/views 的 API schema、Query 状态及 package-boundary 规则。
