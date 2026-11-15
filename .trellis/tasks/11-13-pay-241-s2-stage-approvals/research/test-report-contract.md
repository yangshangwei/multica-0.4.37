# Research: F05 类型化运行测试报告合同

- Query: 为 S2 自动关联原 run/attempt、失败 case 和不可变日志选择最小生产入口；保持现有身份、原请求恢复、当前权限及 QA 人工签字边界。
- Scope: internal；静态研究，未修改代码、数据库、运行模式或 Git。
- Date: 2026-11-13
- Product boundary: 用户已确认人类手建 child 保留、AI 建议物化须指定人审批、启用/重开明确选择沿用旧 child/代码，以及显式兼任可、实现责任人不得自评、QA 与发布审批分离；配置权与最终验收按本任务 PRD 的已确认规则。本文定义报告证据入口，不代替任何真人签字。

## Findings

### 1. 选定方案

新增独立的 **run-local typed test-report**，复用当前 `mcb_` 业务 capability。生产路径是已获准运行中的测试生产者把结构化结果交给 Multica CLI，CLI 调窄报告 endpoint，服务端从 capability 和冻结运行上下文自动归因。人不再手工挑一个 run 并填写失败 case，服务端也不从自然语言猜 case。

推荐新入口及符号：

| 层 | 入口 / 推荐符号 | 现成复用点 |
| --- | --- | --- |
| CLI | `multica collaboration test-report submit <issue-uuid> --request-id <uuid> --report-stdin --output json`；`newCollaborationTestReportSubmitCmd` / `runCollaborationTestReportSubmit` | `cmd/multica/cmd_collaboration.go:15–79` 的命令/参数 allowlist；`internal/cli/collaboration.go:46` 的完整 run context 校验、`:185–205` 的无重定向窄客户端。 |
| 窄读取 | `multica collaboration test-report subject <issue-uuid>` → `GET /api/collaboration/runtime/runs/{runId}/test-subject` | 当前 `lockRuntimeBusiness`，只读这个 run 已冻结的 subject，不能返回“最新候选”。 |
| 写 HTTP | `POST /api/collaboration/runtime/runs/{runId}/test-reports` → `RecordCollaborationRuntimeTestReport` | 新文件 `internal/handler/collaboration_runtime_test_report.go`；用 `collaborationNarrowBearer(..., CollaborationBusinessTokenPrefix)`、严格 body decoder、`collaborationRuntimeRunID`；router 放在现有 runtime 窄路由块 `cmd/server/router.go:1413–1434`。 |
| 原结果确认 | `GET /api/collaboration/runtime/runs/{runId}/test-reports/by-request/{requestId}` | 只确认原 capability/attempt 的已提交报告，返回 content-free receipt；不借此读取正文。CLI 可提供 `test-report receipt ... --request-id`，无自动重试/改键。 |
| service | `RecordRuntimeTestReport` / `RuntimeTestSubject` / `RuntimeTestReportReceipt`，新增 `internal/service/collaboration_test_report.go` | `collaborationRuntimeMutation`、`lockRuntimeBusiness`、现有 run event ledger、执行变化 hint。 |
| 人类读取 | `GET /api/collaboration/issues/{id}/runs/{runId}/test-reports[/{reportId}]` | 与 `RunRecords` 相同的可信人类 + 当前 issue read snapshot，另核对 S2 candidate/证据可读性；不能由 mcb 读取其他人的报告。 |

不选择扩展 `squad activity`：`service/collaboration_squad.go:392–432` 限定 squad，写 `business_outcome` 并完成业务 turn；提交一份报告不应结束运行、产生 continuation 或改变成功/失败事实。不选择随意往旧 outcome 加字段：`protocol.CollaborationRunOutcome` 在 `pkg/protocol/collaboration.go:109–119`、`handler/collaboration_runtime.go:243–261` 是严格旧合同，没有逐 case 集合。

报告不自动签 QA、不自动 advance、不写普通评论、不创建 child、不调用新测试执行器或模型。适用于当前明确受控的 namespace/host run；既有 ordinary task token (`mat_`)、daemon token、`mct_` termination token、人类 JWT/PAT 都不能写此 endpoint。

### 2. 运行身份与 attempt 由服务器确定

当前 CLI 的五个环境变量只有 `mcb_`、Server 和完整 `CollaborationRunIdentity`；**没有 attempt nonce**，见 `internal/cli/collaboration.go:19–33`、`:227–249`。不要扩散 supervisor 凭据或要求 CLI 自报/猜 nonce。

建议 envelope 携带既有完整 `identity`，但不接受 `actor_id`、`attempt_seq`、`attempt_nonce`、`verified`。服务器以 `(runId, hash(mcb))` 查 `GetCollaborationBusinessCapabilityByHash`，得到本 token 的 attempt，再读不可变 `collaboration_attempt` 取得 nonce、claim_generation、containment、authorized_at。写入 payload 的实际 Agent/run/attempt/nonce 全从这些持久事实产生；对 identity 的每个字段匹配 `collaborationFrozenIdentity` 与当前 run。

新报告事务流程：

1. 严格解析与大小限制；查出原始 issued business capability。不存在/错误 token 返回 403，不试 ordinary Auth。
2. 查该 run/attempt 的原 test-report。若原 request ID、原 request digest、原 identity 和原 capability attempt 完全匹配，返回最小 ACK。此分支不读取 source/log/case 正文给调用者，也不创建新事实。
3. 新写必须通过 `lockRuntimeBusiness`（`service/collaboration_squad.go:85–106`），其内部为 `lockCollaborationRuntimeRun` → 当前 mcb 未撤销且 attempt 等于 `run.authorized_attempt_seq` → `currentAdmission` → `requireReservation`。
4. `currentAdmission` 在 `collaboration_runtime.go:336–427` 重核 Stop/settled/manual、source/participant、冻结 inputs、target/grant/config/deployment 代际、runtime/installation。不可用将阻止新报告；历史失败不会被改成恢复成功。
5. 等待 source/group/run 锁后再次检查原 key/报告存在，处理并发相同提交；不得先做 source 写入再发现重复。
6. 验证冻结 subject、case 和日志关联，追加报告 event + 一个 content-free issue execution change hint，在同事务提交。commit unknown 保留原 request；GET 404 是未确认，不自动创建第二个 request。

参照 `RecordRuntimeOutcome` 的原 key ACK 分支（`collaboration_runtime_capabilities.go:434–473`、`:531–568`）和 checkpoint 锁后二次查 key（`collaboration_checkpoints.go:157–197`）。**不能直接复用 outcome ACK 而忽略 type/request digest，也不能先调用 `lockRuntimeBusiness` 再做历史 ACK**，否则 settled/revoked 后原回执无法确认。

### 3. 必須在测试前冻结代码候选，不能在上传时挑当前版本

现有 `PromptSnapshot` 只冻结实际 `CollaborationRunInput`，其字段是 command/author/epochs/body/attachments；没有 S2 candidate 身份。`PreparationSnapshot` 的 `Git {object_format, commit_oid, bundle_digest}` 是已明确配置的不可变 Git bundle，见 `pkg/protocol/collaboration_preparation.go:46–52`、`:161–176`。单凭报告里填写一个候选 UUID 不能证明运行以它为输入。

**最小可靠绑定点**：在 S2 测试动作创建既有 typed Agent 评论输入时，增加可选、严格的 `test_subject` 引用，包含 `candidate_id / candidate_version / candidate_digest`。正常评论不带该字段保持原行为；这是具体数据合同，不从 comment 正文/角色名/代码块推导。

- 人类既有生产入口是 `CollaborationService.CreateComment`（`collaboration_comments_create.go:16`）；它在同事务经 `AppendCollaborationInputInTx`（`:98–100`）创建每个实际 command，并拿到原 `CommandID`（`:108–115`）。这里验证 S2 当前 candidate、code-review 前置和调用者当前 SubmitCommand 权限，再将**实际创建成功的 command ID**绑定该 subject。target 被拒绝就不建立对应绑定；原 comment request 恢复不得重复绑定。
- 内部 DTO 是 `CollaborationCreateCommentParams`（`collaboration_comments.go:39`）。建议将 test_subject 放在 create-only params，不允许 edit comment 改已冻结 subject。API 解析和 Core 创建请求 schema须同步；评论修改不改变原运行输入。
- 用一张窄的 append-only `collaboration_test_subject` 保存 command→candidate 绑定及源 issue、lifecycle、冻结 code/artifact 引用、实际人类及其 participant/account 代际。不要改写现有 command 的 immutable text 触发器；这一张表只为严格绑定服务。
- 首次 claim 的 `claimPromptSnapshot`（`collaboration_runtime_capabilities.go:293–328`）取得真正被冻结的 command IDs。读取这些 command 的 subject；只能有一个精确一致的 subject。随后在同 claim 事务追加一次 `test_subject` run event，冻结 subject + preparation manifest digest + Git commit/bundle + prompt command IDs。旧 run 不能补贴新候选；多个不同 subject 不猜“最新”，明确拒绝 subject ambiguity。
- report 要求 envelope 中的 expected subject 与 run 的 `test_subject` event 完全一致。`readRuntimePreparation` / `protocol.ValidateCollaborationPreparation` 校验 run identity、CommandIDs、manifest digest；有 Git bundle时其完整 `CommitOID` 必须等于代码候选 SHA。制品测试还须等于候选冻结的 existing artifact ID/digest。
- 已选候选在运行期间被新版替换，不应抹掉旧运行的失败：只要业务写与源证据的当前权限仍然有效，可以继续向**原冻结候选**追加该attempt的唯一报告，并把当前适用性标为stale；不能绑定新版或因此通过当前QA。撤权/Stop则按现有新内容写入规则拒绝。报告保存权与当前阶段可消费性必须分开。
- v1 **没有冻结 Git 来源的旧 run/裸 host 目录运行不提供可用于 QA 的 typed test-report**，返回 `test_subject_unavailable` / `test_source_unbound`。它们继续使用已有 outcome/records 报告一般故障，不回填或假造 case。运行自行 checkout、修改代码、切换构建后不能仍声明原 candidate 被验证；需要新的明确 subject/受控运行。

若实现选择在 claim 当下抓“当前候选”而不绑定原 command，则旧队列可能被重标为新需求/代码，不能关闭 F05。若希望以后支持非 Git bundle 的 host 工作目录，需独立设计可信 runner source observation；本包不能把 caller SHA 字符串当作这一能力。

上述 source binding 证明的是**已授权输入与已准备代码快照的关联**。mcb 是业务能力，不是宿主文件系统/测试执行防伪证明；运行生产者仍可能错误报告测试。来源必须明确 `runtime_reported`，QA 仍审查测试充分性，不能标为 `remote_ci_verified`。

### 4. 确定的 v1 JSON 合同

`--report-stdin` 读取 report 对象，CLI 构造外层 identity 和 request_id，不接收外部 profile/server/workspace override。示例中的 ID/hash 为格式占位，不是实际证据：

```json
{
  "version": 1,
  "identity": { "...": "existing complete CollaborationRunIdentity" },
  "request_id": "<canonical UUID>",
  "report": {
    "subject": {
      "candidate_id": "<UUID>",
      "candidate_version": "1",
      "candidate_digest": "<64 lowercase hex>",
      "binding_digest": "<64 lowercase hex>",
      "source_revision": "<40 or 64 lowercase hex>",
      "artifact_digest": null
    },
    "suite_id": "payment-regression",
    "cases": [
      {"id":"callback-duplicate","outcome":"passed","log_index":0},
      {"id":"callback-delay","outcome":"failed","log_index":1}
    ],
    "logs": [
      {"kind":"record","event_id":"<retained event UUID>","message_index":2,"body_sha256":"<64 lowercase hex>"},
      {"kind":"inline","body":"expected paid, observed pending"}
    ]
  }
}
```

固定边界：

- 顶层及嵌套均拒绝 unknown/duplicate/case-alias/trailing JSON、null必填与 NUL/invalid UTF-8；所有 UUID canonical 非零。版本/代际为规范十进制 string，避免 JS 精度损失。
- 每个 run/attempt 最多 **一份**报告，多个测试 suite 先合并为一份 capture；最大 256 cases。每个 case ID 在该报告内唯一、1–256 UTF-8 bytes；`suite_id` 1–128 bytes；outcome 是 `passed | failed | error | skipped`。至少一个 case；不是从 business outcome 生成占位 case。
- 每 case 必须指向一个实际有效 log_index；logs 1–64，inline 总 decoded body ≤64 KiB，每个引用指向的 body ≤既有64 KiB limit；整份规范化报告 ≤256 KiB，HTTP外层允许最多2 MiB以兼容转义，CLI读取同样有界。超限显式拒绝，不截断到看似完整。
- `source_revision`、artifact digest 只是 expected bindings，必须与已冻结 subject 相同；没有 artifact 的 source-only tests 显式 null。要求构建证据的后续 QA/发布阶段不能把 null 当作已验证构建。
- 客户端不提交 overall passed/failed。服务端从 case集计算 `reported_result`：任一 failed/error → failed；全 passed → passed；包含 skipped → incomplete。这个结果仅总结报告声明，不证明完整业务矩阵或远端流水线通过。
- 服务器存额外 `source_kind=runtime_reported`、actual agent、run/attempt/nonce/generation、received_at、source binding、canonical request digest、redacted content digest。body/ID中的自由文本使用既有 redact 规则处理；实际秘密不应进入持久正文。用于原请求比较的 digest不公开原始私密文本。

最小 ACK 为 `{version, issue_id, run_id, request_id, report_id, report_digest, accepted:true}`，无 case、日志、私有来源或角色快照。`report_id` 可直接使用新 run-event 的服务器 UUID。ACK 表示已保存，不表示 QA 已通过。

### 5. 日志绑定与尚未上传的处理

首选 `record`，引用**已提交**的 exact run event ID + message index + body digest，而不是传分页 cursor、时间戳、任意 URL 或本地路径。

- 仅允许本 report 的同 issue/run/attempt 下 `record_checkpoint`，以及有确切同 attempt 的既有 typed `outcome.record`；new report before outcome 时通常使用已 ACK checkpoint。原事件 payload 的 identity/attempt先通过现有 validator，再解析指定 message，比较原 retained redacted body bytes 的 SHA-256。
- `service/collaboration_checkpoints.go:55–99` 和 `collaboration_records.go:407–436` 已有 event到typed message的严格校验；以**存储 event payload**为权威，不通过人类 HTTP回环读取或从 mutable UI拷贝内容。`record:<stream>:<ordinal>` 是展示稳定ID，不单独证明 attempt：仍须回到其实际 event 校验 nonce/identity。
- legacy outcome record v1若能从其包裹的 `CollaborationRunOutcome` 验出原 attempt，可按 event+message_index 引用；没有原 attempt 的任意旧记录明确不支持。
- 引用 event 未提交、message尚未上传或消息范围不存在：返回409 `test_report_log_not_retained`，不写报告、不登记假 hash、不后台补改。生产者可等待原日志的ACK后，保留同请求再次提交；已停止/撤权后不能新补正文。
- 生产者还没有可引用的 retained checkpoint 时，可在**首次报告**提交 `inline` 有界日志。服务端 redact、计算摘要，与 cases/subject 同一 immutable event提交。这是实际报告输入日志，不冒充 daemon recording或远端CI。后续异步 transcript不会替换它。
- 不支持任意 filesystem路径/远端URL抓取，不调用附件上传、不复用用户浏览器 Cookie。附件报告若后续需要，可另加版本；v1不为此扩展动态上传/执行器。

### 6. 持久化、幂等与失败投影

已有 `collaboration_run_event` 无 kind枚举约束（migration571:112–120）；已有 `CreateCollaborationBusinessEvent`（`queries/collaboration_squad.sql:36–40`）。报告用 kind=`test_report`，固定 producer key=`test-report:<attempt_seq>`；migration596/597 已给 `(run_id, producer_event_id)` 与 `(run_id,seq)` 唯一索引。payload保存 request_id、request_digest与完整不可变服务器报告。

- 同run/attempt同request同digest返回同ACK。相同key不同body或另一个request企图再次提交同attempt，409 `test_report_conflict` / `test_report_already_recorded`；不能覆盖首轮失败。
- 真正重试在现有新run/新attempt产生新报告；不另建重试调度器。一个attempt内部反复跑测试必须在最终capture前汇总；要保留已提交的失败并再测，就明确启动新运行。该限制要进CLI帮助和UI。
- 读取列表使用当前可信人类/issue权限的 `readCollaborationContent` 模式（`collaboration_records.go:144–171`），scope精确到issue/run，分页/大小有界，no-store。撤权不可读时不得暴露case名称或日志；GET receipt仍保持原最小ACK。
- 接纳报告时通过 `recordCollaborationExecutionChange` 发布内容为空的 report hint，复用既有 outbox/invalidation。S2 evidence query按精确 subject/run/attempt读报告并自动显示失败case/log。不把新typed kind强加给旧 `records.items` 判别联合；它们继续原协议，report用专门可选资源。
- **不要用测试报告改 `run.business_outcome`**。报告可能所有case通过但Agent随后清理失败/超时；保留原reported_result，同时把可用于QA的整体适用性标为 `execution_failed`/`execution_unsettled`。没有报告的运行失败只显示执行失败，测试case结果未知。失败case报告可以提示QA审查/退回，不能自动伪造QA决定。
- QA通过只允许精确报告+候选依赖且当前所需执行/证明已具备；报告上传后尚在运行不等于最终成功。候选变化、Stop/revoke、后续失败、artifact不匹配都不能经旧QA依赖跳过当前检查。新报告不自动替换QA已选版本；失败与人工改判各自保留。

### 7. 最小迁移与文件归属

**报告本体无需新表/索引**，复用 run event ledger及现有唯一索引；新增 bounded查询到 `server/pkg/db/queries/collaboration_test_report.sql` 并重新生成 sqlc。

冻结测试subject需要一个窄表：`collaboration_test_subject(issue_id, command_id, candidate_id, candidate_version, candidate_digest, subject jsonb, bound_by_user_id, participant_epoch, account_epoch, lifecycle_epoch, created_at)`。subject 是服务端生成的规范化不可变代码/构建引用，不是任意传入 JSON。表和CHECK/immutability trigger一份迁移，`UNIQUE(command_id)` 使用另一份 `CREATE UNIQUE INDEX CONCURRENTLY` 单语句迁移；无FK、不额外加通用workflow层。降级不得通过删这些已审输入来绕过；沿用阶段包对历史保留的既定策略。

单一服务端 owner负责新 `collaboration_test_report.go`、comment create-only subject接线、first-claim freeze、service查询及报告投影。Protocol/CLI owner负责 `pkg/protocol/collaboration_test_report.go`、`internal/cli/collaboration_test_report.go`、`cmd/multica/cmd_collaboration_test_report.go`、全命令guard和帮助；HTTP/router与Core共享请求schema由对应owner同步。builtin技能 `server/internal/service/builtin_skills/multica-working-on-issues/` 及其 source-map应记录report只是证据、不能签QA；host有效提示从 `internal/daemon/collaborationrun/host_runner.go:553` 接入可用命令说明，不改普通task提示为全局强制。

生产者使用现有测试命令/脚本输出结构化结果，CLI只做有界解析、归因和传输，不新增测试runner、不执行 report文本内命令、不自动安装依赖、不调用真实模型。S2本地证据必须由测试构建的受控Provider实际经CLI→handler→service提交，不能只seed report表就宣称生产入口通过。

### 8. 必需验证与不支持分支

1. Protocol：重复字段、畸形identity/case/日志、空集、重复ID、unknown状态、skip、Unicode字节和总量边界；报告结果由服务端派生。
2. CLI：真实root命令注册/allowlist；错issue、partial context、profile override、subject格式畸形在HTTP前拒绝；report-stdin有界；无ambient human/daemon凭据；失去响应保留原request。格式合法但与run冻结subject不一致的报告由服务端拒绝且不写event；现有CLI环境只有run identity，不能在未读取subject前自行知道服务器的candidate绑定。
3. Service/handler/router：mcb真claim/permit流程正例，wrong-run/mct/mat/daemon/human否定；token绑定attempt/nonce不可伪造；Stop/revoke先后顺序、removed/rejoined authority、settled后仅原ACK；并发同报告、改payload/换key均无第二个event。
4. Subject：真实human评论→实际command→first claim冻结，candidate换版/多个subject冲突、旧run、missing Git、wrong commit/artifact、改live代码候选不重标原subject、derived member/continuation依原冻结inputs决定，不偷继承另一run的candidate。
5. Logs：实际checkpoint ACK后引用；未ACK/别run/别attempt/错hash/越界拒绝；inline真实保留/redaction/限制；异步record不改旧报告；不可读case/log不泄露。
6. QA：失败cases自动成为原candidate的未审证据；明确真人选择报告退回/改判，新Agent报告不覆盖；进程失败但无typed报告保持case未知；all-passed报告后进程失败保留报告但不可通过QA阶段。
7. 保留既有outcome/checkpoint/seal/proof、三Agent闭环和old CLI不可越权回归。正常/竞争测试用任务自有独立DB，受控Provider不证明远端CI、模型质量或真实支付。

## Caveats / Not Found

- 现有业务run上下文没有S2 code-candidate ID，也没有向CLI暴露attempt nonce；这是具体需要补的绑定，不能用说明文字替代。
- 现有记录是typed对话/tool消息，不是测试结果格式；自然语言、exit code、business_outcome都不能恢复未提交的失败case集合。
- v1不支持普通task token、未冻结source的host目录、跨run日志引用、任意附件/URL导入、同attempt多次改写报告或签字后自动换版。对应分支必须明确unavailable，不能用synthetic override放行。
- 此研究尚未实施/测试。方案用现有业务写权限归因和保存证据，不能宣称它独立验证了Agent究竟运行了什么或远端CI的真实结果。
