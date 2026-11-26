# Research: M1 ordinary completion backend contract

- Query: 普通单事项如何取得可审阅证据，签署明确授权，提交完成，并按原操作恢复结果；哪些能力已存在，哪些最小读接口仍缺失？
- Scope: internal；仅源码与既有测试检查。仅在本文件记录研究，不修改产品代码，不运行测试、数据库、Provider、CLI 执行或发布。
- Date: 2026-11-12
- Task: `.trellis/tasks/11-12-agent-squad-followup-roadmap/`，保持 planning。

## Findings

### 1. 总结：不需要再造授权服务，缺口在发现、准备和消费端

已实现完整的授权创建、原创建 key 查询、按授权 ID 查询、撤销和实际 writer 事务消费。现有非授权 hint、私有 evaluation 和人工签发是不同概念。首包应复用 `EvaluateDeliveryEvidenceInTx`、`CreateDeliveryCompletionAuthorizationInTx`、`PrepareDeliveryMutationInTx` 与 `FinishDeliveryMutationInTx`。

目前没有 issue-scoped 交付证据候选列表，也没有将当前 target/project/policy、选定制品绑定和凭据有效期限整理给人类审阅的 preparation DTO。现有一般审批列表和 artifact-package 详情只能作为低层来源，不能充当新的完成授权流程。

### 2. 已有 HTTP 合同（字段名必须精确）

路由：`server/cmd/server/router.go:2128`、`:2243`、`:2388`。

| 操作 | 当前合同 |
| --- | --- |
| 创建授权 | `POST /api/delivery-completion-authorizations`，human-only；请求头 `Idempotency-Key: <canonical nonzero UUID>`。不是 body `request_id`，也不是 body `request_key`。 |
| 创建结果恢复 | `GET /api/delivery-completion-authorizations/by-request-key/{requestKey}?workspace_id={sourceWorkspaceUUID}`；仅原 human issuer，重新核对当前访问权限。 |
| 授权/终结事实查询 | `GET /api/delivery-completion-authorizations/{authorizationId}`；原 issuer 或精确指定 consumer 按当前权限访问；不扩展为普通管理员任意浏览。 |
| 撤销 | `POST /api/delivery-completion-authorizations/{authorizationId}/revocations`，human-only，新的独立 `Idempotency-Key`，body 恰好 `{"reason":"…"}`。 |
| 实际完成 | `PUT /api/issues/{id}`，body 的属性叫 `completion`，不是 `delivery_completion`。 |

创建请求完整 ordinary/human 形状见 `server/internal/service/delivery_completion_types.go:61`、`:198`、`:298`：

```json
{
  "contract_version": "delivery-completion-enforcement-v1",
  "source_workspace_id": "<workspace UUID>",
  "operation_id": "<original completion operation UUID>",
  "target": {
    "kind": "ordinary",
    "issue_id": "<issue UUID>",
    "expected_issue_revision": 7,
    "expected_status": "in_progress",
    "target_status": "done",
    "target_position": null
  },
  "project": {
    "id": "<project UUID>",
    "expected_revision": 3,
    "expected_policy_revision": 2
  },
  "evidence": {
    "issue_id": "<same issue UUID>",
    "expected_issue_revision": 7,
    "approval_id": "<approval UUID>",
    "expected_source_revision": "<40/64 lowercase hex>",
    "expected_spec_digest": "<64 lowercase hex>",
    "expected_artifact_digest": "<64 lowercase hex>"
  },
  "consumer": {"kind": "human", "user_id": "<current human UUID>"},
  "expires_at": "<UTC RFC3339 Z timestamp at microsecond precision>",
  "reason": "<explicit acceptance reason>"
}
```

占位符仅解释形状，不能直接发送。所有字段必填，ordinary 的 `target_position` 必须显式 null 或具体有限数字；首包 status-only 用 null。目标、consumer 是封闭 variant，禁止混入其他 variant 字段。未知/重复/大小写别名字段拒绝。reason 非空、trimmed、UTF-8、至多 1000 字符。目标 status 必须规范化后的当前 active key。ordinary evidence issue/revision 必须等于 target issue/revision（`:365`）。

实际提交最小 body：

```json
{
  "status": "<exact authorized target status>",
  "completion": {
    "authorization_id": "<authorization UUID>",
    "operation_id": "<original completion operation UUID>",
    "expected_issue_revision": 7
  }
}
```

`expected_revision` 可同值提供，但若与 `completion.expected_issue_revision` 不同则 400。见 `server/internal/handler/issue.go:3508`、`:3541`。普通 status-only 不应附带内容、附件、指派或结构更新。

创建响应成功为 201；同原 key、完全相同语义 body 的已保存请求为 200/replayed。请求 key 的数据库范围是 workspace + issuer；`operation_id` 的唯一范围是 workspace。相同 key 不可换 reason/expiry/artifact/body，原 operation 也不可换语义（`server/pkg/db/queries/delivery_completion.sql:6`、`:29`、`:36`）。

### 3. 授权/receipt 的最小响应

`server/internal/service/delivery_completion_authorization.go:21`：

```text
authorization_id, operation_id, issue_id, issue_revision, target_status,
contract_version, created_at, expires_at, state, applicability, replayed,
completion_receipt?
```

`state`: `issued | consumed | revoked`。普通 issued 为 `applicability: not_checked`；到期后为 `expired`。终结事实为 consumed/revoked 时 applicability 同名。`issued/not_checked` 不是当前可消费证明；它没有重跑完整 completion admission。GET 返回的 `replayed` 默认 false，不能用它判断是否查到了旧操作。

ordinary `completion_receipt` 精确公开字段（`server/internal/service/delivery_completion.go:70`）：

```text
authorization_id, operation_id, issue_id, completed_revision,
target_status, completed_at, replayed
```

实际 actor、before revision、mutation digest 等在服务端 immutable terminal 留存并验证；**当前响应不公开 actor/before 字段**，不要规划消费不存在的字段。普通 PUT 成功在 IssueResponse 附此 receipt；回放返回当前 issue + 原 receipt，绕过二次通知/副作用（`server/internal/handler/issue.go:4081`、`:4436`）。

终结 receipt 是历史事实；另一请求已 reopen/edit 时当前 issue 可以是 todo、不同标题和更高 revision。查询恢复必须单独 GET issue 当前状态，展示原操作结果与当前状态，不能把历史 `target_status=done` 投影成当前 done（`server/internal/service/delivery_completion_authorization.go:417`、`server/internal/handler/delivery_completion_authorization_test.go:128`）。

### 4. 当前证据来源与缺失的发现合同

| 来源 | 能取得什么 | 限制 |
| --- | --- | --- |
| `GET /api/issues/{id}` | issue UUID、revision、status/category、project 等 | 不提供受控 artifact candidates；issue revision 位于 `server/internal/handler/issue.go:90`。 |
| `GET /api/projects/{id}` | project revision | `server/internal/handler/project.go:30`；与单独 issue GET 非原子。 |
| `GET /api/projects/{id}/delivery-policy` | policy mode/revision、contract、skip_supported=false、waivable_checks=[] | `server/internal/service/delivery_policy.go:16`、`server/cmd/server/router.go:2342`；policy revision 不等于 project revision。 |
| `GET /api/agent-approvals?status=executed&limit=…` | 审批 ID、workspace、agent/task/issue、summary/plan、执行状态/时间 | `server/internal/handler/agent_approval.go:216` 只有 status/limit，没有 issue 过滤、cursor 或 artifact 类型专用筛选；人类拿到一般审批不等于可读 artifact。 |
| `GET /api/agent-approvals/{approvalId}/artifact-package` | approval、execution_id、spec、spec_digest、revision_provenance、status/result_code、receipt、receipt_digest | `server/internal/handler/agent_artifact.go:134`、`:319`。spec 中 source revision/expiry；成功 receipt 中 output_digest/output_bytes/artifact_id。需精确 artifact authority。 |
| `POST /api/issues/{id}/delivery-evidence/evaluate` | 当次锁定 observation、verdict/reason、实际绑定、DB 时间、valid_until | 调用方须预先知道全部精确期望字段，没有发现能力，也没有授权能力。 |

没有找到受控 artifact bytes 的专用 HTTP 下载路由。首包只规划元数据、摘要、已链接 PR、digest 与观察结论，不把通用附件下载冒充该 artifact 下载，不自动执行打包、审批或生成制品。

现有一般 approvals list 不应先取最近 N 条再按 issue 客户端过滤；这会把范围截断误当该事项没有证据。也不能静默选 newest、first 或 matching 的任意一个 artifact。需要完整分页/明确截断信息，用户选择一个确切候选。

### 5. 证据与 issuer 有效期

`server/internal/service/delivery_evidence.go:23` 的 evaluate 输入恰好：

```text
policy_version=delivery-artifact-observe-v1,
expected_issue_revision,
expected_project_id, expected_project_revision,
approval_id,
expected_source_revision, expected_spec_digest, expected_artifact_digest
```

两个 project 字段必须同为 null 或同为有效值。输出 `DeliveryObservation`（`:36`）含 issue/project revision、approval/execution、source revision、observed_head_sha、spec/artifact/receipt digests、producer agent/task/generation、observed_at、valid_until、verdict/reason、provenance。

`valid_until` 来自 artifact execution 的 `expires_at`，不是 issuer credential expiry（`:264`）。授权创建新跑一次 `EvaluateDeliveryEvidenceInTx`，只接受 pass 且有 valid_until；随后限制 `now < expires_at <= min(now+24h, evidence valid_until, finite issuer credential expiry)`，超界 409 `delivery_completion_expiry_invalid`，不会静默截短（`server/internal/service/delivery_completion_authorization.go:137`、`:151`）。

issuer expiry 从真实 JWT principal 或锁定 PAT 行读取（`:556`），PAT hash、password version、binding 等只在服务端保留。现有完成提示、observer、授权前 DTO 没有公开安全的聚合 max_expires_at；不能让客户端解码/推断凭据或固定设为 24h。建议 selected preview 返回服务端 `checked_at` 与聚合 `max_expires_at`，不暴露凭据类别/hash/ID。创建时仍重算，过期后重新审阅，不能自动延长。

### 6. 权限与私有 evaluation

观察器复用 `CheckArtifactAuthorityInTx`（`server/internal/service/delivery_evidence.go:177`），校验 retained producer task/generation、runtime、approval、artifact canonical spec/receipt、当前原发起者/审批者等；不是只校验当前 workspace membership。

`server/internal/service/artifact_authority.go:364`：人类读既有 artifact 时，若不是原 human initiator，通常要求 agent owner 或 workspace owner/admin；审批人也必须保有 artifact manager 身份。列表获取的 approval summary 权限不能放宽到 artifact metadata/spec/receipt。候选接口应按当前 artifact authority 过滤，不能返回他人无权 artifact 的 ID、digest、数量或拒绝详情；锁/数据库失败不得伪装空列表。

私有 durable evaluation 使用 original evaluator + key，响应含 evaluator/request_key/observation；不是授权请求字段（`server/internal/handler/delivery_evaluation.go:20`、`:75`）。授权 POST 根本不接收 evaluation_key，直接以 issuer 重新观察。新普通流程不必创建 durable evaluation，更不得借他人的 key；既有 hint 仍 `evaluation_key:null` 且 `enforced=false/usable_for_completion=false`。

授权读取也不是失权后的无限回查后门：原 issuer 通过 `completionIssuerRead` 复核当前普通 issue 与原 artifact 访问，指定 consumer 走自己的权限（`server/internal/service/delivery_completion_authorization.go:327`）。如被移除/停用/凭据撤销，结果可能拒绝；客户端保留未知或无权查看，不扩大查询范围。

### 7. 事务、拒绝与未知结果

`PrepareDeliveryMutationInTx`（`server/internal/service/delivery_completion.go:178`）先核对实际 consumer 和 immutable request。带原 binding 已 consumed 的请求先恢复原 receipt；不受当前 issue 已 reopen 的影响，不二次完成。未 consumed 新准入才检查当前非 done -> done、精确 issue revision/status、project/revision/policy enforce、实际 consumer 与 retained issuer、原 artifact/source/expiry。

Finish 使用同一实际 `pgx.Tx`，校验最终持久化 issue 和当前证据；授权 consumed、状态变更、iteration 等原 writer facts 原子提交。SQL terminal insertion 再用 `clock_timestamp()` 校验 grant/evidence/issuer/consumer expiry（`server/pkg/db/queries/delivery_completion.sql:41`）。

| 条件 | 当前拒绝/结果 |
| --- | --- |
| 不合法 JSON/UUID/variant | 400 `invalid_delivery_completion_request`（key 为 `invalid_delivery_completion_key`） |
| 当前权限不足 | 403 `delivery_completion_denied`，或按访问边界 404 |
| 不存在的 grant/key | 404 `delivery_completion_not_found` |
| stale/mismatch/revoked/lock contention | 通常 409 `delivery_completion_conflict`；不是 passing evidence |
| expiry 超界 | 409 `delivery_completion_expiry_invalid` |
| 内容等混合实际变更 | 409 `delivery_completion_content_changed` |
| enforce 新完成缺 binding | 428 `delivery_completion_authorization_required` |
| schema/服务不可用 | 503 `delivery_completion_schema_unavailable` / `delivery_completion_unavailable` |
| Commit 尝试后失败/不确定 | 503 `delivery_completion_outcome_unknown` |
| 显式 skip | 409 `delivery_skip_not_supported` |

handler 映射见 `server/internal/handler/delivery_completion_authorization.go:19`；PUT commit unknown 不会被原安全锁重试路径重试（`server/internal/handler/issue.go:3557`）。

恢复区别：

- 创建 POST 结果未知：保存原 request key、operation_id、请求语义；使用原 issuer/workspace 的 by-request-key GET。GET 404 是查询时未见记录，不证明原 POST 永不提交；不得因 404 自动换 key 创建。
- 完成 PUT 结果未知：用原 authorization ID GET 查看 terminal。consumed + 匹配 receipt 才证明原操作完成；revoked 证明该 grant 已撤销。issued/not_checked 或 expired 不证明原 PUT 未发送/未在途，不能自动写重试。
- 当前 GET 的 NOWAIT 锁可能返回 conflict，保持未知并允许再次查询，不能显示拒绝完成或成功完成。
- 一次查询的 absence/issued 是瞬时事实。现有 v1 **没有 durable pending claim、提交失败终结表或取消在途 PUT 的协议**；首包不能把此缺失伪装“确认未执行”。
- 撤销与消费锁同一授权行，最多一个 append-only terminal。取消授权后需检查实际终结结果；consumed 时只能展示已完成，不可擅自 reopen。撤销 key/body 重放相同才成功，已 consumed 或不同 revocation 操作冲突（`server/internal/service/delivery_completion_authorization.go:241`）。

### 8. 本包最小新增建议（设计提议，尚未实现）

1. 增加 human-only、issue-scoped candidates 只读边界，采用真实 issue/artifact authority、确定排序及分页；仅返回有权元数据。不要改变一般审批列表合同，不新建 evidence ledger。
2. 增加 selected completion-preview 只读边界。输入 current issue revision、exact target、selected approval；从受控 retained execution 取得 source/spec/artifact，复用当前 observer 和 project/status/policy 检查，返回规范化绑定、观察结果、有效期上限与当前 human consumer。仅对 pass/enforce/current/new completion 给出可用于构造请求的字段，不签发、不创建 private evaluation、不改状态。
3. Preview/候选数据都不是 completion permit；新授权 POST 和实际 PUT 按原合同分别复核。避免服务端吐出可直接提交的带 reason/operation/key 的“全自动授权”，由明确的人类确认生成这些操作标识与 reason。
4. 维持自定义 done 按 category；done-to-done 不新授权；reopen 后新的完成必须新的 snapshot、授权、operation。证据若仍通过现有 current observer 可重新选定，不强制复用或自动重打包。
5. 暂不改 artifact download、打包、审批、机器 consumer、拖拽/批量/创建即完成/混合更新/协作/R38。v1 skip_supported=false 为已验收限制。

### 9. 已有测试来源与新增验收重点

以下只确认测试源码存在，**本次未执行，不构成本次通过证据**。

| 测试文件 | 已有覆盖线索 |
| --- | --- |
| `server/internal/handler/delivery_completion_authorization_test.go:91` | 实际 artifact 原子消费/replay；`:128` reopen 后原 receipt 恢复；`:159` policy/head/issuer revoked/expired/content/wrong consumer/operation；`:215` rollback/revoke/tx identity；`:268` missing binding/create policy；`:295` archived done editing；`:360` consume/revoke/evidence locks；`:402` terminal 时刻 expiry。 |
| `server/cmd/server/delivery_completion_authorization_integration_test.go:84` | router + actual artifact/restart recovery；`:146` machine/spoof；`:161` designated human 与 private evaluation 隔离；`:220` selectors；`:240` current credentials；`:264` TTL；`:285` skip/ambiguous JSON；`:307` revoke recovery。 |
| `server/internal/handler/delivery_completion_consumer_test.go:19` | issuer 与 distinct contributor consumer 分离。 |
| `server/internal/handler/delivery_completion_issuer_credential_test.go:49` | retained issuer credential 与 actual consumer credential 独立验证。 |
| `server/internal/service/delivery_completion_types_test.go`、`delivery_completion_receipt_test.go` | 封闭 JSON variant / receipt 合同。 |
| `server/cmd/server/delivery_completion_enforcement_integration_test.go` | 实际 writer 集成边界；不能被新 preview 的单元测试替代。 |

新增必须证明：候选分页不漏/越权不泄露、selected preview 所需字段不靠猜测、current authority/issue/project/policy/PR/expiry 竞争、DB commit 失败 preview 不成功、无 durable evaluation/authorization/queue 写；有限 JWT/PAT max expiry、unknown 字段 fail closed；真实路由 create -> PUT -> GET receipt -> 当前 issue；故障注入区分“未发送”“收到明确拒绝”“发送后未知”，保存原 key，GET 404/issued/conflict 都不自动 POST/PUT；撤销竞争、重复操作、custom done、reopen 新完成。需要 package-owned fresh DB 的 normal/race，但本规划阶段不申请或运行。

## Files Found

- `server/internal/handler/delivery_completion_authorization.go`：auth-only transport、严格 body/key、no-store、错误与 commit unknown。
- `server/internal/service/delivery_completion_types.go`：所有封闭请求 variant 和 canonical digest。
- `server/internal/service/delivery_completion_authorization.go`：签发、读取、撤销、存储事实检查、issuer/consumer。
- `server/internal/service/delivery_completion.go`：实际 writer borrowed-transaction prepare/finish 与 receipt。
- `server/internal/service/delivery_evidence.go`：唯一受控 artifact observer，DB PR head 与 retained artifact/current authority。
- `server/internal/service/artifact_authority.go`：artifact/credential/current principal 验证与数据可见性边界。
- `server/internal/handler/agent_artifact.go`：受控 artifact 详情与执行已有 API。
- `server/internal/handler/agent_approval.go`：一般 workspace approvals 列表，非 issue evidence catalog。
- `server/internal/handler/delivery_evaluation.go`：原 evaluator 私有历史观察。
- `server/internal/handler/issue.go`：普通完成 transport、atomic writer、回放/unknown。
- `server/pkg/db/queries/delivery_completion.sql`：immutable grant/terminal、key 唯一性、commit 时 expiry fence。
- `server/cmd/server/router.go`：当前 routes 与 human middleware。

## External References

无。研究依据当前本地 Go 源码与本仓库合同，不需要外部 SDK/API 资料。

## Related Specs

- `.trellis/spec/server/delivery-completion-enforcement.md`
- `.trellis/spec/server/delivery-completion-hints.md`
- `.trellis/spec/server/index.md`
- `.trellis/tasks/archive/2026-11/11-05-agent-squad-rnd-improvements/stage-closure-2026-11-12.md`

## Caveats / Not Found

- 只读源码研究不证明当前环境、数据库迁移或真实 Provider 正常；历史通过不得计作本次结果。
- 未找到 issue evidence candidates、selected preparation、受控 artifact 下载路由；这是本次检索范围中的实际缺口，不扩大为全部产品未实现。
- 无持久化“未提交/已失败”授权终结事实；未 consumed 不足以解除未知。若未来要求无人工等待且自动可靠重试，需要独立设计操作终结/恢复协议，不能在 UI 擅自推断。
- 当前工作区有其他并行改动；行号为本次读取时锚点，实现前重新核对。
