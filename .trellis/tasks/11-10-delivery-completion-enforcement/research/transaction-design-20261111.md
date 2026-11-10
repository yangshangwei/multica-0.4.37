# Research: 完成门禁的事务复用、主体与分包设计

- Query: 为全部实际 writer 选择最小可审查的 borrowed-transaction 复用方案；明确原人类授权、机器消费者、协作源制品以及锁与持久回执边界。
- Scope: internal；规划研究，仅写本文件，不修改产品代码、spec、原审计或任务状态。
- Date: 2026-11-11（Asia/Shanghai）
- Worktree: `/Volumes/artisan/code/2026/mc-delivery-completion-enforcement-20261110`
- 输入：主会话确认 D1–D4 已批准，D5 新批准“源项目政策 + 精确协作交付并行成立；普通协作仍限当前人类发起人；R38 保留真实管理员并另需源授权”。用户授权主会话选择其余推荐项。主会话选择最终合同，本报告给出技术建议。

## Findings

### 1. 推荐：把唯一观察与制品授权核心移入 service，实际 writer 直接借用事务

将现有代码下沉到 `server/internal/service/` 的少量专属文件，而不是让 service 回调整个 Handler 或复制第二观察器。现有 observer 已经接收 `pgx.Tx`，但其 `http.Request`/Handler 耦合需要先拆开：

- `handler/delivery_evidence.go:157` 的 `evaluateDeliveryEvidence` 只有一次观察算法；它调用 `handler/agent_artifact.go:237` 的 `artifactAuthority`、已有 RuntimeLookup 和纯制品校验。
- `artifactAuthority` 从 request 解析实际 principal、workspace、task token，并从 Handler 取 Metrics；其他业务依赖都是 service 已可访问的 auth/artifact/util/db 类型。
- `service/agent_runtime_lookup.go:25` 的 RuntimeLookup 已在 service；若另建导入 service 的下层 delivery 包，又让 service 导入 delivery，会形成循环。直接放在 service 不增加依赖，也保留唯一的 runtime 读归因。
- `handler/delivery_policy.go:81` 的政策读取和 schema readiness；`delivery_evaluation.go:201` 的恢复逻辑继续使用同一个下沉 observer。原私有评估命名空间、JSON 合同和不可变历史不改。
- HTTP 边界仍负责严格解码、真实 request header 的 deny-only 检查、路径解析和响应映射；service 不持有 `http.Request`，不发 HTTP、不打包、不排队。

生产源码构造点复查：`NewIssueService` 调用 1 处（`handler/handler.go:614`）；`NewAdminCollaborationService` 调用 1 处（`handler/admin_collaboration.go:14`）；`NewCollaborationService` 调用 **91 处、36 个文件**，含 `cmd/server/main.go:601` 的只读 authorizer（Issues=nil）和大量 handler 临时实例。全局可选 callback 注入很容易在某一实例遗漏；不能约定 nil=允许。

建议事务准入使用 service 包级函数或没有授权性可选依赖的值对象。Metrics 可为 nil（仅可观测性），政策/授权/schema 校验不得为 nil。这样原构造器无需全仓改签名，所有 writer 的直接调用由编译与入口清单检查约束。VCS 签名适配可使用单次请求必需的窄凭据校验回调，但它只验证该真实 webhook，不负责或替代交付观察算法。

### 2. 具体接口形状

以下是建议类型和函数名，不是已实现 API。精确 DTO 字段和错误码由主会话最终合同确定。

```go
// Pure transport-independent values; no pool, HTTP request or transaction owner.
type DeliveryEvidenceRequest struct { /* existing exact evidence fields */ }
type DeliveryObservation struct { /* existing observation fields */ }
type DeliveryConsumer struct { /* exactly one typed human/task/webhook variant */ }
type DeliveryMutation struct {
    Before db.Issue
    Candidate db.Issue       // derived from locked Before, never a JSON snapshot
    TargetStatus string
    GrantID pgtype.UUID
    OperationID pgtype.UUID
    Consumer DeliveryConsumer
    ProjectChangeReason string
    Collaboration *DeliveryCollaborationBinding
}

// These borrow tx, build db.New(tx), and never Begin/Commit/Rollback/retry.
func CheckDeliveryCreateInTx(ctx context.Context, tx pgx.Tx,
    workspace, resolvedProject pgtype.UUID, targetStatus string) error
func PrepareDeliveryMutationInTx(ctx context.Context, tx pgx.Tx,
    mutation DeliveryMutation) (*DeliveryPreparedMutation, error)
func FinishDeliveryMutationInTx(ctx context.Context, tx pgx.Tx,
    prepared *DeliveryPreparedMutation, actualAfter db.Issue,
    originalReceipt DeliveryWriterReceipt) (DeliveryCompletionReceipt, error)
func CheckDeliveryProjectDeleteInTx(ctx context.Context, tx pgx.Tx,
    lockedProject db.Project) error
```

`DeliveryPreparedMutation` 内部字段不导出，不能从 HTTP 反序列化或在另一事务复用。Prepare 在原 writer 已持有 catalog/issue 等基础锁后，取得当前来源/目标项目、grant、issuer、consumer、制品与 PR 锁，完整检查身份/绑定。Finish 对实际 after、grant 状态、期限和当前凭据作最终复核，再持久化消费及完成回执；它不再打开第二事务。可以保留实际 tx identity 并拒绝把 Prepare 的值交给另一 tx。

消费必须与最终状态同提交：Finish 失败返回 error，原 owner 回滚。已经发生业务写入后，不能把 gate failure 转成可提交的普通协作业务拒绝回执。Prepare 本身不改变业务状态；拒绝可以由原 workflow owner 记录其现有不成功回执。原键重放在新消费之前返回原结果，不能重新消费或再触发 Stop/通知。

新授权 API 应在当前 enforce 政策下明确接受一次提交，并调用同一个实时 observer；不要求用户把 observe 私有评估变成公开票据。可引用原评估作人类界面背景，但旧 evaluation/matching 本身从不授予完成权。新授权记录保存本次验收事实及精确绑定。

### 3. 原验收人与当前消费者分离，不模拟人类会话

将原 `artifactCredential`/`artifactAuthority` 拆成共享的三个部分，原 artifact/hint/evaluation 适配器继续调用它们：

1. **实际请求凭据快照。** 只能从认证上下文读取 `AuthenticatedPrincipal`、`PasswordSession`、VerifiedTaskToken；保留 AgentActor/原 header 的 deny-only 语义。类型不由请求 body 构造。
2. **显式主体当前权限。** workspace、user/member、agent/runtime/task、原审批人、原发起人、originator、claimant 等当前数据库事实的校验接收明确的 subject ID；不依赖“当前 request 恰巧是该人”。调用人和 retained issuer 使用不同类型，避免混淆。
3. **显式凭据再验证。** 将目前 `artifactCredential` 依赖的上下文材料作为已验证输入值传给唯一核心。原 HTTP 调用从真实上下文提取；消费 grant 时只从锁定的服务器持久 grant 读取原 issuer 的保留快照。不得调用 `WithAuthenticatedPrincipal`/`WithPasswordSession` 冒充该 issuer，不得构造 X-User-ID/X-Agent-ID/X-Task-ID。

服务端保留 issuer 快照至少包含：issuer user ID、credential kind、JWT expiry 或 PAT 原 token row/hash 标识及到期、password session version/kind/setup/change（后两项必须 false）、managed binding ID/epoch/workspace/daemon、适用的当前 account fence。**不存 bearer/JWT/PAT 明文。** PAT hash 是服务器内部凭据定位值，不放公共 DTO、日志或回执。

建议 grant 到期取请求到期、政策上限、证据 expiry、原 issuer 有限凭据 expiry 的最早值；最长 24 小时只是上界，当前制品通常受原 15 分钟期限约束。PAT 无到期仍受其他上界限制。原 credential 撤销/password version 变化/managed binding 变化、issuer 禁用或退出 source workspace、原制品读取/管理资格消失，均拒绝消费；renewed credential 不能偷偷改写旧 grant 的 issuer snapshot。

当前消费者再独立验证自己的真实 credential 与原 writer 动作权限；grant 不使人类变 owner/admin，不使 agent 获得人类协作能力，也不使平台管理员变 source member。私有 evaluation GET 保持 workspace+原 evaluator+key，任何消费者接口都不能借 grant 读取其私有 observation 全文。

证据校验的被授权读取主体是 issuer；消费者只得到自身动作所需的最小 completion receipt（grant/operation/issue/版本/结果/责任人，按既有可见范围投影）。机器或跨账号协作人不能借失败细节枚举原制品、源子任务正文或私有评估。

### 4. 变更绑定与 D4

推荐 enforce 完成使用专门的状态变更；允许 position/order 作为不影响验收内容的附带排序。若同次请求使 title/description/assignee/parent/stage/date/priority/属性/标签/附件或项目等其他实际字段改变，先保存这些变更，再取证/授权；当前完成请求返回 binding conflict，不默默接受未审过的候选内容。

判断基于锁定 Before 和服务端计算 Candidate 的**实际差异**，不能仅依据 JSON 字段是否出现。原 `updateIssueAtomicallyOnce` 会补齐 nullable 字段、合并描述晚到媒体并关联附件（`handler/issue.go:3840` 起、`:3892`）；Prepare 必须在这些输入解析后检查。若允许排序同时完成，记录 resulting revision；原验收绑定 Before 内容版本，不能把日后编辑投射为新通过。

非 done→done 触发授权；done→done/自定义 done 互换和已 done 非项目编辑不触发、不消费。严格解析原持久状态类别使用 `service.IssueStatusCategory`；新目标使用受 catalog 锁保护的 Resolve。

项目变更必须读取**来源和目标**当前政策，来源来自锁定 Before；按 UUID 排序取得两个 project SHARE NOWAIT 锁，兼顾无项目。任一 enforce 时已 done 必须先重开，迁移不能同请求进入 done；移出 enforce 要真实当前 human owner/admin、理由和持久调整记录。无实际 project 变化不算迁移。project 删除原本先锁 project 再 issue；在其已有 project 锁内拒绝当前 enforce，保留已有含协作 issue 拒绝（`handler/project.go:963`）及 off/observe 普通 detach（`:978`、`queries/project.sql:122`）。

### 5. VCS 的真实服务身份不能只有 system 字符串

当前 `WriteIssueStatus` 的 `source=pull_request_close_aggregate` 和 actor `system/user_id=null` 不是一个可精确指定的消费者身份（`service/issue_status_write.go:26`、`:31`）。GitHub 与 VCS 最终都丢掉连接信息调用 `advanceIssueToDone`（`handler/github.go:1898`、`vcs_webhook.go:274`）。

建议为这一次真实 webhook 建立类型化 proof，并贯穿 mirror→advance→WriteIssueStatus：

- GitHub：source workspace、**本地 installation row UUID**、numeric installation ID、安装信任修订、repo owner/name、精确 PR ID/number/head、已验证事件类别与原 delivery/event identity。
- VCS：source workspace、connection UUID、provider、规范化 instance URL、**连接凭据信任修订**、repo/PR/head、原事件 identity。
- SHA 与 PR 身份来自实际已验证 webhook 及本事务锁定的持久 PR/link，不能来自 caller 随意字段。当前 PR aggregate 仍是独立必要条件；grant 不覆盖仍 open 的其他 PR。

连接 ABA 不能只绑 UUID：`queries/vcs.sql:14` 的 reconnect 原地 upsert，`:58` secret rotation 原地 update；GitHub reconnect 的 `CreateGitHubInstallation` 也是 ON CONFLICT 原地更新（`queries/github.sql:21`）。推荐明确的单调 trust revision，重连/换 provider/令牌或 secret 轮换递增；纯 display metadata 刷新不递增。删除后重建 UUID 变化同样使授权失效。信任修订属于新门禁凭据绑定，不改历史评估。

VCS webhook 当前在事务外以旧 connection 解密/验签（`handler/vcs_webhook.go:113`、`:134`）。完成事务内必须 SHARE NOWAIT 锁当前连接并验证该 exact trust revision，必要时对原 body/headers 再验当前 secret；断开、轮换、映射变化不能沿用旧认证。GitHub 相应锁 installation+workspace 绑定，并保留实际 HMAC 验证，当前配置关闭/签名验证失败拒绝。若签名算法留在 handler，可传“只验证本次原 body/headers 的 required callback”，返回封闭 proof；nil、过期 proof 或 identity 不匹配均拒绝。不要把此 callback 扩成任意业务 gate。

grant 由源 human 明确指定这一 integration/PR 消费；外部 webhook 不携带可信任的人类 evaluation key。服务端在锁定当前 issue 后只匹配同 issue/revision/policy/消费者/精确候选的未消费 grant；0 个拒绝，多个不明确匹配也拒绝，禁止取“最新任意 grant”。消费记录绑定原 webhook operation identity；重投只恢复确实提交的原结果，不重复执行。HTTP 202 只表示 webhook 接受处理，不能作为 issue done 成功证据。

### 6. 协作的制品可达性与最小源适配

**不能直接用同 issue 普通制品要求覆盖协作。** 现有 `artifactAuthority` 要求 `agent_task_queue.task.issue_id == resource_id`（`handler/agent_artifact.go:278`），observer 再核对该绑定（`handler/delivery_evidence.go:246`）。`service/issue_admission.go:13`→`:42` 明确禁止 ordinary task 以 collaboration issue 为执行源；当前 `queries/collaboration_runtime.sql:136` / `service/collaboration_runtime.go:848` 创建的协作 run 没有普通 task 身份。R51 spec 也明确现代协作 run 无 `agent_task_queue` identity。

因而只修改 observer 的路由拒绝不会创造合法正路径；测试直接 INSERT 一个指向协作的普通 task 会伪造生产可达性。新增受控制品的协作运行 producer 类型会扩大冻结规格、领取与执行合同，超出最小门禁复用范围。

推荐采用主会话提出的**明确普通子任务源适配**，不改旧制品资源归属：

1. 源工作区具权人类通过已有普通 `POST /api/issues` 创建该协作 issue 下的普通 child，实际项目与协作 issue 相同。`handler/issue.go:3070` 接受 parent ID，`:3194` 调 `IssueService.Create`；`service/issue.go:443` 校验同 workspace 父并可继承实际项目。R51 的 create/attach/detach 是普通 child 操作，不授予协作执行权限。
2. child 在自己的原 ordinary task/真实受控制品链生成 tar/receipt；actor 和 producer 当前权限照旧。child 无需先 done，避免“完成 child 必须完成 parent”的依赖环。
3. 源 human 通过新协作完成授权入口显式绑定**目标协作 + 这一个确切 ordinary child + 这一个实际制品 + 精确交付意图**；内部以 child 调唯一 observer，并另核对目标协作关系。不是把 child artifact 改称 parent artifact，也不能接受任意别的 issue/父 issue 制品。

精确 grant 合同至少包含：

| 绑定域 | 必须冻结且消费时复核 |
| --- | --- |
| 目标协作 | source workspace、目标 issue ID/revision、project ID/revision/policy revision、collaboration revision/lifecycle epoch、当前创建者及实际消费者身份；协作/账号/参与者相关 epoch 按消费原动作复核 |
| 普通源 child | child ID/revision、formal ordinary 身份、当前 `parent_issue_id == target`、同 source workspace、**相同实际 project**；不是靠历史 parent hint 或 title 认定 |
| 制品 | child 的 approval/execution/task/generation/spec/artifact/receipt/source revision/原期限；当前 producer/approver/issuer 全部有效 |
| 外部交付 | exact current submission ID/lifecycle/event，当前 pending 与 available；绑定正文/依据/附件冻结摘要，原 initiator acceptance 条件继续成立 |
| 自完成 | `self_result` 明确 variant，exact note/basis/attachment IDs/source_comment/draft selection 的规范化 digest + 指定人类 initiator + lifecycle；不存在 submission ID 时不得伪造一个 |
| R38 | `admin_completion` 明确 variant，真实 admin user/organization、原操作 key、action/target/expected status/reason digest、当前协作版本；当前 submission 若存在则绑定其 ID 或显式 absence，但不写 accepted decision |

**已有合法路径证据：** `.trellis/spec/server/collaboration-records.md` 的 Ordinary children 合同；`handler/collaboration_children_test.go:135` 通过 `h.CreateIssue` 实际创建协作父下普通 child。该现有测试只作源码证据，本轮未运行；新完成验收必须使用真实认证创建 child、正常 enqueue/受控制品 API 与真实 tar，不能只复用该测试中的直接 credential context。

源授权创建人必须有 child 现有制品 reader 权限、source membership 和读取目标协作交付意图的当前权限。consumer 可以是外部发起人，只得到窄授权事实；不能因此获得 child 正文/私有制品/私有 evaluation。若用户想验收的协作没有这条源证据，enforce 必须拒绝，并提示先准备明确源 child；不能退化为文本 result 即通过制品政策。

R38 在原创建者禁用后仍可能因旧制品的 originator/approver 禁用而不可用，这种拒绝是正确的。源工作区另一个具权人类可以在已有普通 child 路径准备新的有效源制品并明确授权实际管理员；不能为行政完成跳过旧主体撤权。

### 7. 协作事务和自结果的顺序

普通协作仍先 `mutateCollaborationWorkflow` 的真实 human/initiator/action/version 检查（`collaboration_workflow.go:371`、`:388`）；R38 仍先 `AdminCollaborationService.lockLifecycle` 的当前平台身份/组织/禁用状态（`admin_collaboration.go:259`）。新授权不取代它们。

现有 self-completion 的 `prepareCompletion` 会调用 `recordSelfDelivery`，已经写 handoff/event（`collaboration_lifecycle.go:468`、`collaboration_delivery.go:177`）。新增门禁若在它后面返回可提交的业务 refusal，将留下部分 self-result。建议把“构建/核对 completion intent”和“写 self-result”拆成两个事务内局部步骤：

1. 只读核对精确 pending submission 或 prospective self-result、现有附件/评论权限，构建规范化 binding。
2. PrepareDeliveryMutationInTx 验证源 child、唯一 observer、issuer/consumer/grant/policy。此时尚未改 self-result/status/queue。
3. 对 self-result 调原 `recordSelfDelivery`，取真实生成 submission ID；原 `changeLifecycle` 继续写 status、Stop/queue、decision 与回执。
4. Finish 消费 grant，回执同时记录 prospective digest 和本事务真实 submission ID。后续任何失败必须回滚全部，不能新建 accepted 假记录。

R38 在 `admin_collaboration.go:225` 调原 changeLifecycle 前做 Prepare，之后 Finish；不调用 prepareCompletion、不制造 initiator delivery_decision。它保留自身双账本及 real admin identity。恢复优先用现有 original actor/org/key receipt，已消费 grant 不触发第二次 close。

### 8. 锁顺序与并发失效

- 保留每个 owner 原始前缀：普通 workspace/member/catalog/W01/iteration/issue；协作 sorted users/account/workspace/catalog/W01/iteration/issue/collaboration/session；R38 platform fence/sorted users/workspace/org/catalog/W01/iteration/issue。不强行把不同事务改成一个新全局锁顺序。
- 在其后新增的 project、grant、issuer/credential、child、制品与 PR 锁均采用现有 NOWAIT/短 lock timeout 策略，避免与先 project 后 issue 的删除及先 user 后 issue 的协作/撤权事务互等。来源/目标 project 和额外 users/child ID 做确定排序。
- 协作先锁目标 parent 后，再锁源 child `FOR NO KEY UPDATE NOWAIT`（或明确等价能够阻挡内容/parent/project 变更的行锁）；child 观察器重复锁同一行是同事务，不开第二事务。child 所需 iteration/W01 不在本次修改，所以无需无故写它，但若读 snapshot 涉及迭代身份则需要一致规则。
- 新 grant issuance 以同样的目标→child→源项目 NOWAIT 顺序取得实际关系；不能先 observer 的 child 锁，再无界等待 parent。现有 child creation 并不锁 parent 做长期关联证明；因此**消费必须从锁定的当前 parent/child 行核对关系**。
- 保留四个 PR/link 表 SHARE NOWAIT；其锁必须持续到状态事务 commit。VCS 连接及 installation 行锁加上 revision 条件使 disconnect/rotation 串行；PR 写事务的存在性/排序变化不得绕过观察。
- Prepare 及 Finish 使用数据库 clock_timestamp；最终消费 SQL 有 expiry/revoked/consumed 条件和唯一约束。原成功授权的 expiry 不因读取/重放延长。Finish 验证失败返回 error 回滚。
- 原普通/Create 安全 55P03 retry 与协作事务内部 retry 不能吞掉“消费/commit未知”；建议新 gate contention 使用明确领域冲突，默认不自动重试；原 owner 未涉及 gate 的旧 off/observe 行为保持。
- schema readiness 在借用 tx 时检查 exact index shape；政策表/授权表缺失或未知 mode 不能默认为 off。授权记录、消费/撤销/迁移事实无 FK，索引逐文件 CONCURRENTLY；删除不清证据。

### 9. 全部 writer 接入点与所有权

| Lane | 独占产品范围（建议） | 必须证明的事实 |
| --- | --- | --- |
| A 核心 | 新 `service/delivery_*.go`、`service/artifact_authority.go`；既有 handler 三个 evidence/policy/evaluation 文件与 agent_artifact 的下沉适配；新 db queries/migrations/generated | 唯一 observer；原 off/observe/私有恢复不变；typed issuer/consumer；事务内授权/一次消费/留证 |
| B 普通 writer | `handler/issue.go`、`issue_move.go`、`source_context.go`；`service/issue.go`；相应 handler/service tests | PUT/move/batch 单项事务、创建拒绝、自定义 done、混合变更拒绝、逐项结果及实际回执 |
| C 协作与 R38 | `service/collaboration_lifecycle.go`、`collaboration_workflow.go`、`collaboration_delivery.go`、`collaboration_edit.go`、`collaboration_source_fields.go`、`admin_collaboration.go`；对应 handler/decoder/tests | exact ordinary-child adapter、外部 submission/self-result/R38 分别正负路径、源迁移规则和无部分自结果 |
| D VCS/项目 | `service/issue_status_write.go`、`handler/github.go`、`vcs_webhook.go`、`project.go`；连接 trust revision 查询及对应测试 | 原签名+integration 精确绑定、原 status tx、source/target政策、项目删除先关闭 |
| 主会话 | spec、任务/PRD/design/implement/context、router 新 API 注册、最终生成物集成、日志/源码库存/Lore 集成 | 所有 writer 清单闭合后才开放 enforce；保护既有主目录修改 |

实际只有有限并发 slot：A 先交付冻结接口；B/C/D 在不修改 A 文件的条件下分批并行。`service/issue.go`、`handler/issue.go`、db generated/migrations 是冲突热点；A 不顺手改 writer，C/D 需要 SQL 时交 A/主会话集中生成。Trellis check 独占审查与自修须暂停冲突 writer 再进行。

### 10. 最小可独立验收包及最终激活顺序

1. **P0：唯一观察/授权核心下沉。** 行为保持的重构包；不加 grant、不开放 enforce。先固化既有真实 artifact/observer/evaluation/hint 回归，再移动 transport-free 实现；验证现有 handler/router 普通/race、build/vet/sqlc/format。这是核心复用基础，不是 gate 功能完成。
2. **P1：明确完成授权、撤销与消费事务核心。** typed issuer snapshot/consumer、精确 scope、当前实时 observer、新 immutable grant/receipt、skip 明确检查清单、schema readiness/保留迁移。先保证同实际 tx 能回滚和原键恢复；公开项目 enum 仍不开放 enforce。不能把测试内插入 enforce row 当生产启用。
3. **P2：普通更新/move/batch/create/comment-child + 项目迁移/删除接入。** 每个实际 writer 有真实身份与最终状态/回执，保留逐项 batch 事实。此包可以证明普通 writer 子范围，但仍不是全 gate。
4. **P3：VCS 精确身份与协作普通 child adapter/精确交付/R38 接入。** 不扩大生产者执行种类，不伪造普通 task、私有评估或 initiator。至少一条通过真实源 child 受控制品链的正路径及全部撤权/并发拒绝。
5. **P4：启用 enforce、兼容与完整最终验收。** 所有 writer/core/schema/授权路径已接通后才扩展 policy enum/API；明确新 policy revision 使旧 grant/observe 事实失效、off 回退保留所有证据。每 DB 包/最终 normal/race 独立新库，真实认证、tar/SHA、逐项持久状态、普通/协作/VCS/R38 原回执、全部失败与具名库存、冻结源码后独审。

若 P1/P2 可单独提交而未开放 enforce，报告只能称基础或限定 writer 接入；不得归档本 enforcement 子任务、关闭 M1 或以静态覆盖替代 P4。

## Files found

核心文件及单行用途：

- `server/internal/handler/agent_artifact.go`：原主体、凭据、制品权限、spec 与实际包装入口。
- `server/internal/handler/delivery_evidence.go`：唯一实时观察及 receipt/PR 绑定。
- `server/internal/handler/delivery_evaluation.go`：原人类私有恢复与不可变观察。
- `server/internal/service/agent_runtime_lookup.go`：必须保留的运行时读取/指标归因，影响包依赖方向。
- `server/internal/service/issue_admission.go`：ordinary task 排除协作源，证明直接协作制品方案不可达。
- `server/internal/service/issue.go`、`handler/issue.go`：合法 ordinary-child 创建、父继承项目与普通实际完成。
- `server/internal/service/collaboration_delivery.go`：自结果写入与精确 submission，决定 gate 调用顺序。
- `server/internal/service/collaboration_lifecycle.go`、`admin_collaboration.go`：两个独立协作完成 owner。
- `server/internal/handler/github.go`、`vcs_webhook.go`、`server/pkg/db/queries/github.sql`、`vcs.sql`：签名入口、消费者身份丢失与重连 ABA。
- `server/internal/auth/principal.go`、`password_session.go`、`pat_principal.go`：实际凭据与原 issuer 快照所需材料。

## Related specs

读取/复用 `.trellis/workflow.md` 的 planning/研究约束，当前 task/prd/design/implement/decision-register，及 server 的 delivery-completion-hints、delivery-policy-evaluations、delivery-evidence-observation、controlled-artifact-execution、collaboration-delivery、collaboration-administration、collaboration-records 规格。后续代码包仍须按数据库隔离规范单独加载/验收。

## External references

无。研究基于当前本地源码与任务合同；无外部 API、供应商、网络或真实模型调用。

## Caveats / Not Found

- 本文件是建议设计，不是实现或验收结论；主会话拥有最终合同选择。报告没有将 D5 的新答复回写到主会话独占决策文档。
- 没有运行 Git、测试、DB/容器或 provider。旧测试只作为合法调用链的源码证据。
- 初次只读搜索尝试了不存在的 `handler/task.go` 和 `queries/collaboration_queue.sql`，工具拒绝；已用 `rg --files` 定位真实路径。另有 `rg` 无匹配导致 exit 1，均是检索失误/无结果，不是产品 red；无状态副作用。
- 原 observer 先遇 hold 就返回 reason；若 skip 允许某个非强制检查，不能仅按 hold 字符串放行而漏检后续 receipt/绑定。需要让**同一个** observer 产生完整 hard-check 结果，再单独评估明确可豁免检查，不得复制第二算法。
- GitHub 当前 HMAC secret 来自运行配置，不自带数据库 trust epoch；最终实现须明确配置关闭/轮换时如何使正在消费的 webhook 认证失效，不能凭 system 字符串宣称已复核集成身份。
- 关系型子证据适配是新的显式授权关系，不改写旧 artifact 属于 child 的事实。缺失/已 detach/异项目/跨源/已撤权 child 都拒绝；没有这条源链时不能声称协作 enforce 有成功路径。
