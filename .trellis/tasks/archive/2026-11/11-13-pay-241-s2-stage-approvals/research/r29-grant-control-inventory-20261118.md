# Research: R29 grant-control inventory and package startup preparation

- Query: 补齐 R29diag 原 handler 五 top / conditional router 三 top 的完整源码 named/leaf inventory；界定 GetByID-only 修复的实际重跑与保留证据，并静态核对同 donor 的包 fixture 前提。
- Scope: internal；只读冻结源码与已有 allocation/clone audit；仅本新 Markdown/JSON 对可写。
- Date: 2026-11-18 (Asia/Shanghai).
- Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`.

## Findings

### 冻结来源与计数规则

`S` = `/Volumes/artisan/.multica-acceptance/pay241-r29-receipt-read-20261118-6dhi01s2/source-red-r01`；所读的原产品、测试、TestMain、router、query 及 fixture 文件与接受的 R28 `/Volumes/artisan/.multica-acceptance/pay241-r28-preview-read-20261117-sz4vj59y/source-green-r01` **bytes/modes 全部相同**。JSON 保留逐文件 SHA256 与逐函数 body SHA256，供最终 GREEN 的 caller/default 等价审计。研究时尚未收到真实 R29 RED 结果；本报告不预批产品 delta。

计数来自 filesystem-only 源码函数与静态字符串循环展开；未执行 Go、test-list 或 TestMain。**named** 包含 top 及每个实际 `t.Run` 父/子名；**leaf** 是没有子 `t.Run` 的名字，无子测试的 top 本身也是 leaf。以下选集没有深层中间父名。普通 assertion/private-field/header/request loops 不新增 named。

### 原 handler 五 top：5 top /12 named /11 leaf

来源均为 `S/server/internal/handler/delivery_completion_authorization_test.go`。其中只有 Refusals 的 `:160–161` 将七个 mutation 字符串传给 `t.Run`；`:153` 私密字段循环只是断言。

| Top | 函数起始行 | Named | Leaf |
| --- | --- | ---: | ---: |
| `TestDeliveryCompletionAuthorizationActualArtifactAtomicConsumeAndReplay` | 91 | 1 | 1 |
| `TestDeliveryCompletionAuthorizationRefusalsPreserveStatusAndGrant` | 159 | 8 | 7 |
| `TestDeliveryCompletionAuthorizationRollbackRevokeAndTransactionIdentity` | 215 | 1 | 1 |
| `TestDeliveryCompletionAuthorizationTaskAndConsumerIsolation` | 316 | 1 | 1 |
| `TestDeliveryCompletionAuthorizationConsumeRevokeAndEvidenceLocks` | 360 | 1 | 1 |

完整 named（包含全部父名和静态循环子名）：

```text
TestDeliveryCompletionAuthorizationActualArtifactAtomicConsumeAndReplay
TestDeliveryCompletionAuthorizationRefusalsPreserveStatusAndGrant
TestDeliveryCompletionAuthorizationRefusalsPreserveStatusAndGrant/policy
TestDeliveryCompletionAuthorizationRefusalsPreserveStatusAndGrant/head
TestDeliveryCompletionAuthorizationRefusalsPreserveStatusAndGrant/issuer_revoked
TestDeliveryCompletionAuthorizationRefusalsPreserveStatusAndGrant/expired
TestDeliveryCompletionAuthorizationRefusalsPreserveStatusAndGrant/content
TestDeliveryCompletionAuthorizationRefusalsPreserveStatusAndGrant/wrong_consumer
TestDeliveryCompletionAuthorizationRefusalsPreserveStatusAndGrant/wrong_operation
TestDeliveryCompletionAuthorizationRollbackRevokeAndTransactionIdentity
TestDeliveryCompletionAuthorizationTaskAndConsumerIsolation
TestDeliveryCompletionAuthorizationConsumeRevokeAndEvidenceLocks
```

完整 leaf 是上表四个无子 top 加 Refusals 的七个 `/mutation` 子名；JSON 的 `leaf_inventory` 逐项列出全部11个，不把 Refusals 父名计成 leaf。实际 GetByID 是 `:72–77` helper，由 AtomicConsumeAndReplay 的 `:148` 调用。

### 条件 router 三 top：3 top /6 named /5 leaf

CurrentCredential 与 ConflictingWorkspaceSelectors 均没有 `t.Run`；`:224` 两组 selector header 仍只计一个 top/leaf。ActualSourceAndRecovery 在 `:378–379` 展开 self/submission/admin，保留 top 父名。

| Top | 来源 | Named | Leaf |
| --- | --- | ---: | ---: |
| `TestDeliveryCompletionAuthorizationRouterCurrentCredentialRecheck` | `server/cmd/server/delivery_completion_authorization_integration_test.go`:240 | 1 | 1 |
| `TestDeliveryCompletionAuthorizationRouterConflictingWorkspaceSelectors` | `server/cmd/server/delivery_completion_authorization_integration_test.go`:220 | 1 | 1 |
| `TestDeliveryCompletionCollaborationRouterActualSourceAndRecovery` | `server/cmd/server/delivery_completion_collaboration_integration_test.go`:377 | 4 | 3 |

完整 named：

```text
TestDeliveryCompletionAuthorizationRouterCurrentCredentialRecheck
TestDeliveryCompletionAuthorizationRouterConflictingWorkspaceSelectors
TestDeliveryCompletionCollaborationRouterActualSourceAndRecovery
TestDeliveryCompletionCollaborationRouterActualSourceAndRecovery/self
TestDeliveryCompletionCollaborationRouterActualSourceAndRecovery/submission
TestDeliveryCompletionCollaborationRouterActualSourceAndRecovery/admin
```

完整 leaf 是前两个 top 及后一个 top 的 `/self`、`/submission`、`/admin`，JSON 逐项列出5个。三个条件 top 不包括下面提到的 retained restart/designated/revocation tops，勿混算成同一个 selector。

### GetByID-only 的重跑/保留边界

**先由 main 接受真正 RED，再按实际 GREEN delta 定界。** 初始产品仍限定 `S/server/internal/service/delivery_completion_authorization.go` 的 GetByID 入口；不让本研究把 by-request、Create、Prepare/Finish、Revoke 或其他真实 writer 加入修复。

| 路径/证据 | GetByID-only 后的处理与精确依据 |
| --- | --- |
| 新 R29 service3 + 已有 service4/63 | owning read/read、writer fence、current authority 要真实 fresh GREEN/race；已有 service63 不重复搜索/列举，引用 R29diag JSON。若 main 组合执行为7 top/76 named/54 leaf。 |
| handler AtomicConsumeAndReplay | 必须真实重跑；真实 consume 后 `:128–146` reopen+edit，原 Prepare replay 不重写今日状态，`:148–156` GetByID200/consumed/minimal privacy。它也保留 issued Create-key replay，但没有 by-key GET。 |
| handler 另外四个原 top | 4 top/11 named/10 leaf；只走真实 Prepare/Finish、Revoke、task consumer 或 evidence writer。若这些 caller/default/helper 的 reached path 完全等价，可保留原 passing evidence，勿仅因同文件修改自动重跑全部 writer 矩阵。 |
| GetByID 的独立 consumer/variant | 若新 grant/root fence 作用于全部 GetByID，既有 `TestDeliveryCompletionCollaborationActualSourceAndOriginalRecovery`（self/submission/admin，1/4/3，`server/internal/handler/delivery_completion_collaboration_test.go:273,315`）及 `TestDeliveryCompletionR38AuthorizationHonorsCurrentConfiguration`（1/1/1，`:493,505,508`）也真实重跑；它们经过已变外层 fence，保留 outside consumer/no private evidence 与 admin-disabled consumer403/issuer200。 |
| by-request GET / Create-key replay / Revoke | 原 service by-request `:224–239` 必须继续调用原 `completionReadAuthority`；query `delivery_completion.sql:30–31` 仍 UPDATE NOWAIT。Create replay `:79,87` 与 Revoke `:254,277` 保留原 issuer helper/exclusive locks。兼容读 helper 只能由 GetByID 调用；改变公用 authority 而未给旧 caller 明确原默认值，就失去保留依据。 |
| 原 writer defaults | `completionCollaborationAccess -> false` (`delivery_completion_collaboration.go:40–41`)、`EvaluateDeliveryEvidenceInTx -> false` (`delivery_evidence.go:79–80`)、`lockDeliveryArtifactInTx -> false` (`:234–235`) 保留；已有 read helpers 的 true 入口仅供明确纯读。Prepare 仍 root `delivery_completion.go:196` /grant UPDATE `:214`。 |
| 当前 consumer 与 source | 保留 `completionConsumerAuthority(...false)` (`authorization.go:352`) 及 admin gate`:349`；该 false 是 **newAdmission=false**，不是 shared/unlocked 模式。`completionSourceIssue(...false)` (`:369`) 的 false 是 **exact=false**，仍会锁 child；不可把这两个参数误当读锁开关。所有当前 user/task generation/binding/invocation/admin/password 谓词仍需保留。 |
| 当前 credentials / retained receipt | setup`:312–325`、`completionView/Receipt/StoredRequest` 与 handler final credential`:225`/commit`:231` 保留。`TestDeliveryCompletionExplicitIssuerCredentialIsolation`（1/8/7，issuer credential test`:49`）是原独立 helper 证据，可凭 `CheckArtifactCredentialInTx`/caller 精确等价保留；它不是 GetByID endpoint negative。新 R29 authority negatives 才实际重跑改过的读入口。 |
| router 三 top | **仅 actual HTTP authentication、workspace selector、account-scoped consumer routing 或 handler finalization/wiring 改变时为必需**。handler dispatch 明确 GetByID=(false,false)、byKey=(true,false)、revoke=(false,true)，`handler/authorization.go:145–152,206–212`；router`:2158–2162` 的 account-scoped路由及 human guards 不变即可保留对应原 transport evidence。 |

**建议最小 handler fresh cut**：当 GetByID 新 grant/root fence 作用于全部 variant 时，选 AtomicConsumeAndReplay + 上述两个 direct GetByID handler top，**3 top/6 named/5 leaf**，normal/race 各一个新 `./internal/handler` profile。若 main 希望同一个 finite run 一并再确认原四个 writer controls，则组合原五+direct二为 **7/17/15**；这是可选的更强检查，不是强制整个 handler suite。实际较窄 delta 只有在未变 variant 明确仍调用 untouched old path 时，才可按 reached branch 再缩减。

### 不虚报 by-key / credentials / consumer GET 覆盖

原 handler 五 top **没有任何 `GetDeliveryCompletionAuthorizationByRequestKey` 调用**；TaskAndConsumerIsolation 是真实 task **消费写入**，不是 task GetByID。不能声称原5独自覆盖这些 GET。

对应原源码证据保留如下，main 使用前必须绑定实际旧 passing named terminals 与最终 source/default/caller 等价；这里没有证明它们曾 PASS：

- `server/cmd/server/delivery_completion_authorization_integration_test.go:84`，`TestDeliveryCompletionAuthorizationRouterRealArtifactAndRestart`：legacy/password 父子共1/3/2；重建 actual router 后 issued GetByID、own by-key GET、同 Create-key POST/current token。`:115–128` request loop 只含断言，无额外 named。
- 同文件`:161`，`TestDeliveryCompletionAuthorizationRouterDesignatedHumanKeepsEvaluationPrivate`：designated human GetByID200、consumer by-key/private evaluation404、other current member GetByID404；1/1/1。
- 同文件`:200`，`TestDeliveryCompletionAuthorizationRouterDesignatedTaskReadsOriginalGrant`：同 owner UserID 的 actual task 仍走指定 task GET；不能冒充 human revoke；1/1/1。
- 同文件`:307`，`TestDeliveryCompletionAuthorizationRouterRevocationRecovery`：原 revoke key/reason、retained single terminal、指定 consumer revoked GetByID、tar/state unchanged；1/1/1。
- `server/internal/handler/delivery_completion_issuer_credential_test.go:49` 的原1/8/7确切 children/parents在JSON单独保留，与新 service current-authority controls职责分开。

by-key、旧 credential helper、旧 consumer helpers 若实际发生变化，或缺少可绑定的旧 PASS，需要 main 明确重新定界适当的 owning/transport controls；不可把静态 source inventory 替代测试通过证据，也不可悄悄扩大产品。

### Recorded donor/clone shape 与两个 TestMain

仅读取 `/Volumes/artisan/.multica-acceptance/pay241-r29-receipt-read-20261118-6dhi01s2/database/profiles-r01/three-profile-allocation-r01.json` 及 donor-before、donor-after-clones、red/green/race-clone-before 五份 JSON。allocation SHA256=`0805d51e38e0a81a6966c5e9f84ab2a4e949d05dab60d1979581f90e8c100274`；这些是 allocation 时的记录，不是当前连接/执行状态。

五份 audit 的 `ledger`、`ledger_sha256`、全部 `indexes` 和 `table_counts` **逐项严格相同**。ledger853项，最后 `865_knowledge_automation_origin`；与 `S/server/migrations/*.up.sql` 的853个 stem **排序及集合完全一致，source-only/ledger-only均空**。共同ledger SHA=`d648c6e258f50d6abbeb3f025475ea6b3bb6ff02501c57713897fc1be606ec3e`。663个 index 全部 live/ready/valid，211个表。Fixture 的 user/workspace/member/runtime/agent/invocation target、completion grant/terminal、project policy、password credential等表均存在且计数0。

数据库并非完全空：organization1、schema_migrations853、execution_gate1、task_usage_hourly_rollup_state1、config_invalidation21、knowledge_invalidation18；本研究不读取行内容、singleton值/epoch或 credentials。JSON保留全部211表的数值及必要 index定义，未复制账号/secret字段。

| 包启动前提 | 源码与静态结论 |
| --- | --- |
| handler TestMain | `handler/handler_test.go:44–105`：非空显式 DATABASE_URL、5秒 Ping；setup先cleanup`:108–110`，再插 user/workspace(issue_prefix)/owner member/online owner runtime/workspace+public_to agent/workspace invocation target（`:113–169`）。相关7个表（含optional client_usage_daily cleanup）在 recorded shape 内。 |
| router TestMain | `cmd/server/integration_test.go:43–100`：非空显式 URL、diagnostic-or-ordinary pool/5秒 Ping；setup先cleanup`:103–106`，再user/workspace（省略issue_prefix）/member/runtime/agent（省略permission_mode），然后actual NewRouter/JWT，均在m.Run前。省略列的最终default属于待核点。 |
| 权限默认差异 | 历史migration130`:31–33`默认private；handler seed显式public_to并建target，router seed省略permission_mode仍private。当前录制相关unique indexes有效。两种fixture不同是原源码事实；勿为启动通过而改seed。 |
| 同 donor 可以建议，但未完成物理前提证明 | 没有recorded missing table或migration gap；可提名同donor作新包clone来源。audit没有列types/nullability/default、UUID function/extension、CHECK/FK/trigger/function状态、singleton gate row值，ledger相同不能证明条件DDL已执行。这些需main在新profile启动前核实，或以真实setup/target terminals补足。 |
| 包 fixture/temp/environment | handler `agent_artifact_test.go:68,95` 与 router `agent_artifact_integration_test.go:101,131` 用实际t.TempDir/packager。已录service HOME/TEMP chain leaf0700、UID501、ancestor无违规group/other writable；不能把这些服务profile当成新handler/router配置证明。GOTMPDIR（本地Go1.27的t.TempDir亦使用）与可信ancestor、capacity及API配置去泄漏须在真实新profile保持。 |

两个TestMain都不运行migrator；测试名称filter照样执行fixture cleanup/setup。当前三个 R29 DB permit都只写 `package=./internal/service`；**不能用于 handler/router**。最小另配两handler normal/race profile；确实改HTTP wiring时再另配两router normal/race profile。normal/race源、selector与named/leaf集合必须匹配，fresh不复用前一次实际fixture写过的DB，保留原失败。PAY并发上限1/global2来自原allocation，不擅自放宽。

JSON提供精确selector及建议argv形状，均未执行。main需使用reviewed runner、其既定deadline/toolchain和新profile，读取actual每name终态；不以exit0、test-list或无named的TestMain退出认作通过。

## Files found / patterns / related specs / external references

核心files与一行职责、SHA/mode、上述caller/default/function行数在JSON `sources` / `function_source_bindings`；完整handler/router named和leaf在对应 inventory 节，service63仅引用既有R29diag。需遵循 `CLAUDE.md`、`.trellis/workflow.md`、server `delivery-completion-enforcement.md`（historical receipt/current access）、`collaboration-stage-approvals.md`（shared read/exclusive writer）、`database-test-isolation.md`（包DB/TestMain/secure temp）与`verification-environment.md`（真实argv/source/config/named identity）。

无外部网络查询。冻结`server/go.mod`声明Go1.26.6、Chi v5.3.0、pgx/v5 v5.9.2；这不是installed toolchain验证。旧R29diag两文件未写，SHA仍为 Markdown `780cb85add6cbf815f58f7906fedd0066b78a50c0419cf0a902ebf16c1325479` /JSON `43d62d177883296f701c475ebe042ea233d84c07507f1f79db5adbbb24779ab9`。

## Caveats / Not Found

- 所有inventory为静态源码集合；没有运行SQL/Go/test/test-list/TestMain、连DB、迁移、API/browser/build、process signal、Git或另派代理，也没有读取credential环境或编辑产品/测试/spec/旧报告。
- 尚无本研究可据以改变repair scope的真实R29 RED/final delta；本报告只准备main后续有界验证。by-key/consumer/current-credential证据不能错算到原handler5中。
- recorded schema摘要不包含列/default/constraint/trigger/function/singleton值；历史connection25/max100和free-space数值不代表当前新profile。
- 原R28 completed/consumed写入与3PASS/1FAIL immutable；失败请求的query/SQLSTATE/physical holder缺口保持UNPROVED。full S2/A01–A12、parent24、S3–S5仍open；local synthetic控制不代表S4外部集成或业务sign-off。
