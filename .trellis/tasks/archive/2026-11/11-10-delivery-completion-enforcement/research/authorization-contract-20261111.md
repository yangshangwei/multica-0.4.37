# Research: P1 明确完成授权与跨 writer 合同

- Query: 冻结可实施的授权 API、明确类型绑定、持久状态不变量与 P2/P3 原事务接口。
- Scope: internal；只写本文件，不改 P0 或其他产品/研究文件。
- Date: 2026-11-11（Asia/Shanghai）
- Worktree: `/Volumes/artisan/code/2026/mc-delivery-completion-enforcement-20261110`
- 基础：当前 design/implement、D1–D7 与主会话冻结的 D6 wire；用户已委托后续选项按推荐执行。以下建议供主会话直接采纳/派发，不是已实现验收。

## Findings

### 1. 唯一新授权模型

合同版本固定 `delivery-completion-enforcement-v1`。一次授权由当前源方人类明确签发，绑定一个目标、一个 exact completion operation UUID、一个指定消费者、一个当前政策修订及一组实时验证通过的精确证据。原 private evaluation 不变、不能传给消费者；新授权不接受 evaluation_key 作为证据选择器。

采用 **immutable authorization + 一个 append-only terminal fact**：授权绑定永不 UPDATE；terminal 只能是 consumed 或 revoked，每个 authorization 最多一条。Prepare 持授权行锁即为本事务暂时占用，不持久化独立 claim、不引入已领取但状态尚未完成的中间状态。过期/政策失效是当前适用性，不把授权行改成另一版；历史仍可在当前读取权限下恢复。

v1 `skip_supported=false`、`waivable_checks=[]`：无 skip 表或“通用 skip”分支。明确请求 skip 返回不支持。任何 hold/unknown/缺失/过期/失权/绑定变化仍拒绝。D2 仅保留未来治理合同。

### 2. 严格 HTTP 边界

采用同一组 **Auth-only API**（放在 RequireWorkspaceMember 外），每个操作由 service 重新核对 source/target 与人类/机器当前权利，不能因为路由在外就省略成员锁。这样协作外部发起人/R38 可读自己的最小回执，无需假装是源成员。

| 方法/路径 | 请求与权限 |
| --- | --- |
| `POST /api/delivery-completion-authorizations` | 仅真实 human；一个 canonical nonzero UUID `Idempotency-Key`；严格下述 body；源 workspace 当前成员 + 源制品 reader + 目标意图当前可读。签发需当前 enforce 修订及观察 pass。 |
| `GET /api/delivery-completion-authorizations/{authorizationId}` | 原 issuer、被指定 human/task 消费者或指定 R38 admin 的当前权限；仅最小身份/状态/原消费事实，不公开私有 observation/凭据/子制品内容。webhook 没有通用读身份，由原 issuer 读取。 |
| `GET /api/delivery-completion-authorizations/by-request-key/{requestKey}?workspace_id={UUID}` | 仅原 human issuer，以 source workspace+当前本人+create key 查原行，用于 POST 丢响应。其他人相同 key 返回404。 |
| `POST /api/delivery-completion-authorizations/{authorizationId}/revocations` | 真实当前 human；原 issuer 或源 workspace 当前 owner/admin；UUID `Idempotency-Key`；严格 `{"reason":"..."}`。原 issuer仍须当前源/目标访问权；管理员权限不授予私有正文读取。 |

非空冲突的 `X-Workspace-ID/Slug` 不得把 body 的 source 另解释成其他 workspace；account API 采用实际 target 的 source 身份，忽略 header 必须与现有 account adapter 一致并在 body/load 后严格比对。建议统一在适配器去掉这两种选择 header，再以 body 的 canonical source UUID核对真实 target；不改认证上下文。

严格 JSON：拒绝重复/未知/错误 variant 字段、null 必填值、大小写别名、尾随 JSON；UUID canonical nonzero；revision 为非负 int64 数字，协作正 epoch 按现有十进制字符串；时间只收 UTC RFC3339Nano 并归一至 DB 微秒，必须未来且不超过 cap；body 最大 1 MiB（self-result 的 JSON 转义需容纳既有两份 65,536 字节 prose）；其他 variant 可更小，但同 decoder 仍逐字段限长。

POST 公共顶层字段恰为：

```json
{
  "contract_version": "delivery-completion-enforcement-v1",
  "source_workspace_id": "UUID",
  "operation_id": "UUID",
  "target": {"kind": "ordinary", "issue_id": "UUID", "expected_issue_revision": 4, "expected_status": "in_review", "target_status": "done", "target_position": null},
  "project": {"id": "UUID", "expected_revision": 2, "expected_policy_revision": 3},
  "evidence": {"issue_id": "UUID", "expected_issue_revision": 4, "approval_id": "UUID", "expected_source_revision": "hex", "expected_spec_digest": "hex64", "expected_artifact_digest": "hex64"},
  "consumer": {"kind": "human", "user_id": "UUID"},
  "expires_at": "2026-11-11T00:00:00Z",
  "reason": "明确验收理由"
}
```

`reason` trim 后 1–1000 Unicode 字符、有效 UTF-8、无 NUL。`project` 必填非空，因为授权仅用于实际 enforce 项目；source 和 project 来自锁定 target，再与输入比较。`evidence` 的 policy version 由服务端固定为现有 `delivery-artifact-observe-v1`，不允许请求重新定义观察算法。普通 evidence.issue_id 必须等于 target；协作必须是下述 exact ordinary child。授权创建用**新实时观察**，保存该 observation 的确切 execution/receipt 等身份及原到期，不把历史 pass/current/matching 升级为授权。

创建返回201，新请求同键同正文在当前读取权限下返回200原行；同键不同规范请求返回409。先查询原键再判断新准入，可恢复已过期/已消费/后来政策变化的原事实，绝不续期或重观察覆盖原行。

### 3. Target 与 Consumer 明确 union

每个 Target 都包含表中公共 issue/revision/expected_status/target_status 字段，其他字段只在指定 variant 出现。exact target_status 必须当前 active 且 Category=done；保存标准化 key，不能任意换成另一个 done。

| target.kind | 额外严格字段与规则 | consumer.kind |
| --- | --- | --- |
| `ordinary` | `target_position: number|null` 必填；null 表示不能改变排序，number 是本次允许的 exact finite position；其他实际字段改变先保存再取证。 | human/task/github/vcs |
| `collaboration_submission` | `expected_collaboration_revision`, `expected_lifecycle_epoch`, `expected_participant_epoch`, `expected_account_epoch`（皆十进制字符串）；`status_options_version`；`submission_id`；`acceptance_note`（可空，但有效 UTF-8且≤65,536字节）。exact current pending available submission，由服务器冻结其 event/lifecycle与结果/依据/附件摘要。 | human（必须当前发起人） |
| `collaboration_self` | 同上4个 epoch/version和status_options；`result`,`basis`,`draft_id:UUID|null`,`attachment_ids:UUID[]`,`source_comment_id:UUID|null`；result/basis 非空且分别≤65,536字节；附件用已有数量限制、排序去重的规范集合。必须不存在 pending submission，且当前发起人承担本工作。 | human（必须当前发起人） |
| `collaboration_admin` | `expected_collaboration_revision`,`expected_lifecycle_epoch`,`status_options_version`,`submission_id:UUID|null`,`administration_reason`；operation_id 等于原 admin Idempotency-Key；action 固定 complete，不接受 successor。submission null 是签发时明确不存在，不是任意匹配。 | admin |

普通/协作所有额外语义字段在规范 request digest 内。self prose 不要求事先写一个假 submission；服务器计算 `self_result_digest`，最终 consumer 提交原结果内容并在同 tx 内生成真实 submission，再把真实 ID写入消费事实。可以保留 typed request_json作原键冲突核对，但它不是执行时万能 scope；后面的 scalar列和明确 typed digest 才是可索引/可核对合同。

| consumer.kind | 严格请求字段 | 服务端冻结、消费时重新锁定的事实 |
| --- | --- | --- |
| human | `user_id` | exact user；普通需要 source membership与原更新权；协作仍是当前人类 creator、有效 participant/account；其当前实际认证不能来自 header 模拟。 |
| task | `agent_id`,`task_id`,`task_generation` | exact ordinary task+agent+generation、runtime/owner/originator、current task token scope/managed binding；授予对象是这一次task generation，不是该 agent 所有未来任务。真实请求 VerifiedTaskToken 必须匹配且持有当前原 writer动作权限。 |
| github | `installation_id`（本地UUID）, `expected_trust_revision`, `pull_request_id`（本地UUID） | numeric installation ID、workspace、repo owner/name/PR number/head、部署 webhook config identity；只匹配这条现有PR 的真实签名事件。 |
| vcs | `connection_id`,`expected_trust_revision`,`pull_request_id` | provider、规范 instance URL、workspace、repo/PR/head、当前凭据信任修订；原body用实际当前secret验证。 |
| admin | `user_id`,`organization_id` | exact真实管理员及源组织；消费时仍需 R38当前password session/superadmin/disabled creator，不要求或伪造源成员身份。 |

机器消费者授权创建只验证当前指定对象可存在/可执行此类动作，绝不伪造它的 token 或会话。消费时真正的当前 credential再次证明主体。future webhook event ID 在签发时未知，不放必填绑定；合同冻结 exact integration/PR/head/terminal事件种类，消费事实再保留真实 delivery ID/原body digest。若一个不同 provider 的关闭事件使聚合满足，但它不是被指定消费者，不得代用该授权。

协作 source evidence 的合法路径：普通 POST issue创建 exact child→真实 ordinary task→原受控制品 tar/receipt；签发/消费均验证 child **formal ordinary**、当前 parent=target、同 source workspace、相同实际 project、exact child revision；不需要 child 先 done。源 issuer还需目标协作意图当前可读及child制品原reader权。外部 initiator只持自身协作权，不能读child正文/私有评估。

### 4. 表结构与不可变证据

建议两个表，另有 P2 项目迁移事实表由相应 writer规划；不把迁移伪装为 completion authorization。

`delivery_completion_authorization`（一条明确授权，以下列全部不可变）：

| 列组 | 精确列（类型） |
| --- | --- |
| 身份/版本 | `id uuid`, `workspace_id uuid`, `issuer_user_id uuid`, `request_key uuid`, `operation_id uuid`, `contract_version text`, `request_digest text`, `created_at timestamptz`, `expires_at timestamptz`, `reason text` |
| target | `target_kind text`, `issue_id uuid`, `issue_revision bigint`, `expected_status text`, `target_status text`, `target_position double precision NULL`, `project_id uuid`, `project_revision bigint`, `policy_revision bigint` |
| 源证据 | `evidence_issue_id uuid`, `evidence_issue_revision bigint`, `approval_id uuid`, `execution_id uuid`, `producer_agent_id uuid`, `producer_task_id uuid`, `producer_generation bigint`, `spec_digest text`, `artifact_digest text`, `receipt_digest text`, `source_revision text`, `evidence_expires_at timestamptz`, `observation jsonb`（固定原观察schema，不公共输出） |
| consumer基本 | `consumer_kind text`, `consumer_user_id uuid NULL`, `consumer_agent_id uuid NULL`, `consumer_task_id uuid NULL`, `consumer_task_generation bigint NULL`, `consumer_runtime_id uuid NULL`, `consumer_originator_user_id uuid NULL`, `consumer_binding_id uuid NULL`, `consumer_binding_epoch bigint NULL` |
| webhook | `integration_id uuid NULL`, `integration_trust_revision bigint NULL`, `github_installation_id bigint NULL`, `vcs_provider text NULL`, `vcs_instance_url text NULL`, `webhook_config_identity text NULL`, `pull_request_id uuid NULL`, `repo_owner text NULL`, `repo_name text NULL`, `pr_number integer NULL`, `pr_head_sha text NULL` |
| 协作/管理员 | `collaboration_revision bigint NULL`, `lifecycle_epoch bigint NULL`, `participant_epoch bigint NULL`, `account_epoch bigint NULL`, `status_options_version text NULL`, `submission_id uuid NULL`, `submission_event_seq bigint NULL`, `delivery_intent_digest text NULL`, `admin_organization_id uuid NULL` |
| issuer凭据 | `issuer_credential_kind text`, `issuer_credential_expires_at timestamptz NULL`, `issuer_pat_id uuid NULL`, `issuer_pat_hash text NULL`, `issuer_password_mode boolean`, `issuer_password_version bigint NULL`, `issuer_password_kind text NULL`, `issuer_binding_id uuid NULL`, `issuer_binding_epoch bigint NULL`, `issuer_binding_workspace_id uuid NULL`, `issuer_binding_daemon_id text NULL` |

可另保留 `request_json jsonb` 为严格DTO规范请求存档；不能从它解析自由字段决定授权，也不能输出。self/admin/submission 的 digest 输入固定版本结构：所有公共target字段、variant专属字段、consumer ID、operation ID及源证据 IDs，UUID/时间/附件顺序标准化；domain prefix独立。self prose、接受note、admin reason以明确字段进入digest，不能只绑定目标状态。无需新增通用 JSON“授权范围”解释器。

`delivery_completion_terminal`（只 INSERT，一授权最多一条）：

| 列 | 合同 |
| --- | --- |
| `authorization_id uuid`, `kind text`, `recorded_at timestamptz` | kind只能 consumed/revoked；recorded_at DB clock |
| `actor_kind text`, `actor_user_id uuid NULL`, `actor_agent_id uuid NULL`, `integration_id uuid NULL` | 记录本次真实人类/task/integration/admin，不冒充issuer；消费需与grant精确匹配 |
| `operation_id uuid NULL`, `mutation_digest text NULL`, `issue_revision_before bigint NULL`, `issue_revision_after bigint NULL`, `status_after text NULL` | consumed必填；op等于grant；before等于grant，after是实际返回的写入revision，不假定+1；状态等于grant target |
| `collaboration_operation_id uuid NULL`, `admin_operation_id uuid NULL`, `result_submission_id uuid NULL`, `result_collaboration_revision bigint NULL` | variant对应原真实账本/生成submission；R38不得假造accepted/self ID |
| `provider_event_id text NULL`, `provider_event_digest text NULL` | webhook真实事件来源证明，不公开原payload/secret |
| `revoker_user_id uuid NULL`, `revocation_request_key uuid NULL`, `revocation_reason text NULL`, `revocation_digest text NULL` | revoked必填，consumed必须NULL；原授权列不改 |

约束：所有身份/必填字段 NOT NULL；所有 UUID 非全零；revision≥0、政策/epoch>0；hex摘要固定64小写，SHA40或64；`created_at < expires_at <= created_at + interval '24 hours'`、expires_at≤evidence_expires_at且≤有限issuer expiry；kind/variant组合 CHECK按上表使不相关列NULL，避免 nullable字段被第三种解释。ordinary evidence_issue_id=issue_id；collab evidence_issue_id<>issue_id且collab字段完整。数据库不能跨表验证实时parent/role/receipt，应由原 tx锁与代码验证。

索引每个单独 migration：authorization `UNIQUE(id)`；`UNIQUE(workspace_id,issuer_user_id,request_key)`；`UNIQUE(workspace_id,operation_id)`（防一原操作指多个授权）；terminal `UNIQUE(authorization_id)`；authorization普通检索索引 `(workspace_id,issue_id,issue_revision,policy_revision,consumer_kind,integration_id,pull_request_id)`。全部 CONCURRENTLY；无 inline PK/UNIQUE 或FK。当前读取到最高780；编号由root统一分配，禁止子代理抢号。终态唯一是最终防线，grant行锁是所有正常路径共同序列化点。

不为“过期”写terminal；因此已过期但未消费的grant仍可明确撤销留证。new schema readiness核对表列/ CHECK版本/确切valid-ready-live索引形状。第一降级步骤在停准入维护期 NOWAIT锁并拒绝任何grant/terminal/enforce政策历史；不得删除记录换回滚通过。普通资源删除不清这些表；失去当前资源后公共读取拒绝。

### 5. issuer凭据模型与 P0 API

P0拟导出 `service.ArtifactCredentialSnapshot`（Principal/PasswordSession/VerifiedTaskToken显式值）、`CheckArtifactCredentialInTx`、`LockArtifactWorkspace`、`LockArtifactUsers`、`CheckArtifactAuthorityInTx`、`ReadArtifactSpec`、`DeliveryEvidenceRequest`、`DeliveryObservation`、`EvaluateDeliveryEvidenceInTx`、`ArtifactError{Status,Code}`。P1直接复用，最终签名以P0冻结结果为准。

issuer只能 JWT/PAT human，签发时snapshot从真实认证context捕获；restricted/agent hint/setup/change都拒绝。存上述私有scalar metadata（无明文JWT/PAT、password hash、webhook secret）。PAT实际 ID/hash必须从已锁定token行派生，body不得提供。password/managed绑定与P0检查同义。

消费从**已锁grant**重建显式snapshot并调用P0凭据/主体校验；这是服务器保留的授权源，不是替换当前consumer context。不得调用 WithAuthenticatedPrincipal/WithPasswordSession假装issuer发请求；P0对managed/password当前检查也不能误读consumer context。JWT有限 expiry和password session version、PAT撤销/expiry、binding epoch与issuer user/member权限均复核。

grant cap=min(显式请求期限, DB now+24h, 原evidence expiry, issuer有限credential expiry)；推荐拒绝超cap请求并返回可用上限，不静默把客户端所签意图改成不同期限。旧token过期或被撤销，未消费grant不可用；新的凭据只可读取授权历史或签发新的grant，不能重绑旧issuer snapshot。

### 6. 消费、撤销、重放的 SQL/事务不变量

1. writer原catalog/issue等锁完成后，`SELECT ... authorization WHERE id=$1 FOR UPDATE NOWAIT`；同时校验workspace/target/consumer/operation，不让不同请求靠猜ID获得内容。
2. 查询terminal：consumed且同op、same semantic mutation digest、当前caller访问权成立→返回原最小receipt，**不运行第二次状态/附件/iteration/Stop/通知**；revoked→409。此分支必须在新done→done/字段编辑分流前执行。
3. 无terminal才验证当前政策修订、before/revision、精确target intent、consumer实际writer权限、issuer snapshot、源child、唯一observer、所有hard checks和DB expiry。Prepare不INSERT、不改状态。
4. 原writer执行一次真实业务写入。Finish重新核对same tx、actualAfter、最新DB时间/凭据，再 `INSERT terminal(kind='consumed',...)`；返回error须回滚所有状态/自结果/双账本，不将late失败包装为可提交业务拒绝。
5. 若INSERT冲突，读取已存在结果仅用于证明冲突/重放；在本事务已经写了业务状态后不得把它吞成成功。正常同grant锁应使这种冲突只属于异常/错误调用，应回滚并报告。
6. 撤销同样锁grant，检查当前原issuer或owner/admin、原revocation key/digest；无terminal→INSERT revoked。并发consume与revoke只一方能提交；锁争用409、不自动重试。已consumed不能撤销成未完成；同revoker/key/reason恢复原revoked，其余冲突。
7. 任何不确定Commit/响应不明返回非成功/unknown，只 GET auth/原操作；绝不新key补偿、不接管、不把缺terminal当自动重发依据。客户端不知道事务是否仍在途时，GET用短NOWAIT读锁，争用返回busy/unknown。

Prepare对象内保存原tx identity且字段私有，Finish若不同tx拒绝。外层普通/Create的55P03和协作40001等retry不能把gate busy/unknown自动变另一消费；将门禁锁冲突转换为固定领域拒绝，且 commit failure永不retry。

### 7. P2/P3 函数与 wire

```go
type DeliveryCompletionBinding struct {
    AuthorizationID pgtype.UUID
    OperationID pgtype.UUID
    ExpectedIssueRevision int64
}
// Other union variants are concrete structs, not map[string]any.
type DeliveryMutationIntent struct {
    Before db.Issue
    Candidate db.Issue
    Completion *DeliveryCompletionBinding
    ProjectChangeReason string
    Consumer DeliveryConsumer
    Collaboration *DeliveryCollaborationIntent
}
func PrepareDeliveryMutationInTx(ctx context.Context, tx pgx.Tx,
    in DeliveryMutationIntent) (DeliveryPrepareResult, error)
func FinishDeliveryMutationInTx(ctx context.Context, tx pgx.Tx,
    prepared DeliveryPreparedMutation, after db.Issue,
    result DeliveryWriterResult) (DeliveryCompletionReceipt, error)
```

PrepareResult是明确 `not_applicable|replayed|prepared` union，不能nil默认放行；replayed附原最小receipt。纯done→done无需grant，但若请求提供completion必须进入原授权恢复/冲突分支，不能当普通编辑忽略。Finish只收prepared；服务不Begin/Commit/Rollback/retry。创建/项目删除使用前研究的专用Check函数，不虚构completion。

主会话已冻结单项 wire：`completion?: {authorization_id,operation_id,expected_issue_revision}`，`project_change_reason?: string`。batch额外 `completion_bindings?: Array<{issue_id,authorization_id,operation_id,expected_issue_revision}>`，禁止updates.completion共用；完整次序结果来自各真实tx。若旧 `expected_revision` 与completion版本同时存在且不相等，400；缺completion的enforce新完成428。目标status与position来自实际writer请求并与grant比较。

协作/R38增加completion binding，但operation_id必须等于其原Idempotency-Key（规范UUID）；已有collaboration/admin ledger继续拥有原intent digest/receipts。VCS无外部grant header，事务内按exact integration/PR/head/issue revision/policy锁定匹配待消费grant：0个拒绝、多个拒绝；使用该grant已冻结operation UUID，保存实际事件digest。不得按“最新授权”随便挑一个。

返回最小CompletionReceipt字段固定：`authorization_id, operation_id, issue_id, completed_revision, target_status, completed_at`；协作另有 `collaboration_revision` 与非R38真实`submission_id`（需要时）；`replayed`是本次响应属性。不给credential、原observation、private child details或内部ledger row ID。

普通PUT重放如需保留旧IssueResponse，可返回**当前已授权读取的issue**并附该历史completion receipt，不能重新返回旧done对象覆盖重开后的当前状态。batch重放标`committed/completion_replayed`并用原completed_revision供事实恢复，同时客户端重取当前状态；不再发parent通知。现有代码的哪些响应字段允许扩展由ordinary/client lane共同确认，不在service复制完整HTTP响应。

### 8. GET隐私与新准入分离

GET只返回：`authorization_id,operation_id,target issue/冻结revision/target_status,contract_version,created_at,expires_at,state(issued|consumed|revoked),applicability,completion_receipt?`。`applicability`最小枚举 `not_checked|expired|policy_changed|binding_changed|revoked|consumed`；不做“可完成=true”承诺。issued不意味着当前可用，消费总是全重验。

- issuer使用本次真实credential，检查当前source membership、target访问及源证据原reader权；新credential可读原事实，但不能修改保留issuer snapshot。
- 指定ordinary human检查本次source/issue权限；指定task要求真实同task/generation/token与scope；其owner不能凭UUID自动成为另一consumer。
- 指定collaboration发起人检查当前human/participant/account/目标协作访问；指定admin按当前R38原receipt读取合同复核admin/source组织，creator恢复后可读原结果但不可新行政完成。它们不需要获得child/evaluation读权，也不收到那部分内容。
- source owner/admin可撤销不等于可读原issuer私有observation；接口不提供全文reader。错workspace、错主体、缺当前资源统一404/既有403且无嵌套证据。
- 原issuer/producer后来失权或旧grant expiry只阻断**新消费**，不把已提交的最小completion事实从有权consumer历史中改成“从未发生”；历史receipt不再次作实时验收。若目标当前访问也失去，仍拒绝读。

### 9. 可实施的真实正路径与剩余风险

P1以真实认证human→正常ordinary issue/task→真实受控制品批准/执行→实际tar字节/SHA→签发grant为基础，再借用测试owner真实状态tx证明一次消费及回滚；不得直接INSERT成功artifact/假task替代生产链。P2补真实PUT/move/batch/create/project route；P3协作使用已存在R51合法child创建，普通task只属于该child，然后真实submission/self/admin事务。VCS用本地真实router签名webhook和持久installation/connection/PR；无外部消息或真实provider调用。

P1/P2未接齐前公共policy API仍禁止enforce；如果正向基础验收需预置enforce行，只能在明确隔离测试夹具内标注**内部核心验收**，不得说完整路由启用可用。P4才用owner/admin真实政策route完成完整用户链。没有无条件nil callback或off fallback。

需要实现时明确的技术风险：

- self prepareCompletion目前会写handoff：必须先只读生成intent并gate，再原write，Finish失败全回滚。
- VCS/GitHub原地重连需单调trust revision；GitHub全局secret/config身份采用当前design的停止旧进程/排空前提，不能假称新flag保护旧实例。
- task consumer的当前auth_version/managed binding检查要与P0实际snapshot一致，不能把一个agent UUID当真实执行身份。
- 普通position附带完成必须在grant中精确冻结。move的before/after anchor是旧外层读取；最终position偏离签发值就拒绝，不重新求一个值代替原意图。
- 普通授权与协作intent的request digest均须拒绝重复字段/规范歧义；数据库JSONB文本不用于重新散列，使用固定typed canonical形式。
- schema新增列较多但为闭合union。不要为“通用性”退回任意JSON scope；若拆typed detail表，仍需同一授权ID及单terminal事务约束，不能漏掉对应variant的必需行。

## Files found / Related specs

- 当前 `.trellis/tasks/11-10-delivery-completion-enforcement/design.md`、`implement.md`、`research/decision-register.md`：已确认业务与分包。
- `server/migrations/776_delivery_policy_evaluation.up.sql`、`server/pkg/db/queries/delivery_policy.sql`：当前off/observe与私有评估schema，最高迁移读取为780。
- `server/internal/auth/principal.go`、`password_session.go`、`server/pkg/db/generated/models.go` 的PAT/task/password/connection模型：显式凭据字段来源。
- `server/internal/service/collaboration_workflow.go:24`、`collaboration_queue.go:26`、`collaboration_lifecycle.go:21`：原操作/epoch/精确target合同。
- `server/internal/handler/issue.go:3411`、`:4811`：单项/batch DTO扩展位置。
- `server/cmd/server/router.go:2187`：现有source membership组与Auth-only新读取入口需要分开。
- 相关spec：delivery-policy-evaluations、delivery-evidence-observation、delivery-completion-hints、controlled-artifact-execution、collaboration-delivery、collaboration-administration、collaboration-records、database-test-isolation；前研究transaction-design中的源码定位继续适用。

## External references

无。纯当前本地领域与代码研究，无新依赖/外部资料调用。

## Caveats / Not Found

本报告没有修改产品、运行测试/数据库/容器/provider、执行Git或派发子代理。P0接口来自主会话的实现者提案，实际导出签名仍需P0交付时核对。所有运行验收与schema迁移编号由主会话/相应实施lane负责；本文件不是P1/P2/P3完成证明。
