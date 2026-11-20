# Research: S2 服务端门禁与版本审批复用

- Query: 核对现有完成门禁、建议准入、职业审批、当前授权与原请求恢复；提出不重复建设的 S2 最小落点。
- Scope: internal，静态源码研究；未运行测试、数据库、Provider 或 Git 操作。
- Date: 2026-11-13
- Baseline: 主会话报告 HEAD `feb87aee75fa6b9dd051a4e35bf7df8b101edb75`；已有其他任务 dirty 文件不属于本研究。

## Findings

### 1. 已有 M1 是真实完成事务门禁，S2 不应再造完成令牌

`service.PrepareDeliveryMutationInTx` / `FinishDeliveryMutationInTx` 已经覆盖实际新完成事务，接受调用者的 `pgx.Tx`，不得由阶段 helper 另起事务、提交或重试。`DeliveryPreparedMutation` 只由成功准备产生，并绑定原事务；finish 再核对实际持久化 after、政策、证据及当前 issuer/consumer。

| 实际写入口 | 现有接入及需要保留的行为 |
| --- | --- |
| 普通 Issue PUT | `server/internal/handler/issue.go:3936` 的 `updateIssueAtomicallyOnce`；`4055` 在 completion 存在或状态改变时进入 Prepare，`4134` 在原事务内 Finish。`4402` 是普通 Update 调用点。 |
| 普通 Move | `server/internal/handler/issue_move.go:117`、`149` 最终委托 `UpdateIssue`，不是第二个独立完成实现。 |
| 批量更新 | `server/internal/handler/issue.go:5599` 每个 item 调同一 atomic writer；保留 committed/rejected/not_attempted/unknown 与已提交前缀。 |
| 创建为 done / comment-to-child | `server/internal/service/issue.go:477` 的 `CheckDeliveryCreateInTx` 在解析实际 parent/project 后检查；`delivery_completion.go:145` 按当前状态 category 拒绝 enforce 项目 create-as-done。 |
| GitHub / VCS merge 自动推进 | GitHub `server/internal/handler/github.go:1925` 的 `advanceIssueToDone`；VCS `vcs_webhook.go:302` 共用。实际 `service/issue_status_write.go:26` 的 `WriteIssueStatus` 在 `82–89` 匹配确切 webhook authorization 并 Prepare、`131` Finish；`63–72` 排除 collaboration Issue。 |
| 协作发起人验收与自完成 | `server/internal/service/collaboration_lifecycle.go:341` 的 `ChangeLifecycle`，先 `prepareCompletionMode(..., false)` 做只读结果检查，`378` Prepare，`385` 才正式绑定交付，`414` Finish。 |
| R38 失能发起人管理处置 | `server/internal/service/admin_collaboration.go:250` Prepare、`265` Finish；保持真实超级管理员主体，不伪造发起人的 accepted。 |

精确实现合同：`server/internal/service/delivery_completion.go:178` 开始；`215–232` 已 consumed 同意图返回原回执，`261–278` 核对新消费的 issue/status/project/policy/evidence，`294–304` 在无 authorization 的新完成分支只对 M1 enforce 返回 428。`306–369` Finish 重新校验并写唯一 consumed terminal。来源/制品仍只是已有受控 `local-synthetic` tar + 数据库观察到的 PR head，不证明 CI、实际部署、支付或模型质量。

项目 M1 默认 off，`delivery_policy.go` 处理既有 off/observe/enforce；现有 hint/evaluation 永远不是签字。人类授权请求定义在 `delivery_completion_types.go:311`，绑定 task/project/policy、确切证据 issue/approval/source/spec/artifact 和 consumer。`delivery_completion_authorization.go:40–92` 只接受真实 human issuer，先做当前凭据/读取授权后才恢复原 key。撤销和消费写同一 append-only terminal，不能延长或重签旧令牌。

### 2. 协作状态、主体与恢复合同必须复用

- `server/internal/collaboration/access.go:23`：Human 要同时满足未受限、非 AgentActor、JWT/PAT；持有人类 PAT 的 AgentActor 仍不能签字。
- `access.go:82–114`：当前 active account + source member/participant 决定读取；执行/协作动作还要有效 participant。creator/handler ID 不复活撤销的参与关系。`Complete/Cancel` 只属于 creator；职业名称没有作用。
- `server/internal/service/collaboration_workflow.go:388–420`：共用原请求 digest 与事务 owner；未知 commit 不自动重试。`440–530` 锁当前用户、workspace、catalog/W01/iteration、issue、session 和当前授权；`534–584` 按 scope+issue+actual human+request key 恢复，检查 payload hash，并持久保存拒绝与成功回执。
- `workflowRoleAllowed` 在 `collaboration_workflow.go:360` 是封闭动作集合。S2 新的人类动作应显式加入，并在实际动作中再核对固定角色；不能用 `Edit` 或 `ControlAI` 代替 QA/发布权。
- `SubmitAcceptance` 在 `collaboration_handoff.go:274`，正式提交保存不可变结果/依据并转给 creator；`ReturnAcceptance:351` 必须指向当前未决 SubmissionID。`in_review` 仅用于这条正式交付链。
- `delivery_completion_collaboration.go:158–180` 对普通 collaboration 完成仍要求当前真人 creator；`183–239` 绑定当前 revision/lifecycle/participant/account、确切 submission 或 self-result。来源 artifact 必须是同 workspace、同 project、确切 parent 下的普通 child，见 `77–95`；不能把 collaboration runtime run 包装成普通任务 artifact。
- 既有 R38 是 creator disabled 后的管理处置，不是一般验收继任。`collaboration_handoff.go:298` 明确禁止把 disabled creator 的验收自动改派他人。若 S2 要新增最终验收继任，属于产品改变，不能悄悄复用 handler transfer。

### 3. 建议审批已有可借用的部件，但没有可直接等同的合同

**Triage** 有接受、修改后接受、拒绝、版本 CAS 和原 request digest：`server/internal/handler/triage_actions.go:109` 定义动作，`126` 仅 accept 可带 fields，`350–399` 恢复同原 actor/key/hash，`452` 调实际 acceptance，`537` 持久化 before/after/action/执行意图。其对象本身已是 `admission_status=pending` 的 Issue，不是协作中的独立建议；`triage.go:77` intake 没有 parent ID；`triageHumanActor:177` 允许当前 workspace 的人类成员审阅，不是固定审批人。不能把 triage 原入口称为 TC05 已满足。

`accept_and_execute` 是独立明确动作，plain accept 不执行。现有 pending admission fences 可作为规则/测试参考，但把 S2 建议塞进该全工作区分诊队列会同时改变可见性、职业权限及原产品语义。

**Knowledge candidate** 已有不可变内容、state revision/version/hash、approve/reject/withdraw 与当前授权读取：`server/internal/service/collaboration_knowledge.go:109–151`、`548–657`。但这是知识发布，不是子任务/研发审批；而且 `151–154` 连 propose 也 human-only。可以借用版本/回执模式，不要给它增加建子任务副作用或把 knowledge approval 当作研发批准。

**一次性创建普通子任务** 推荐借用 `IssueService.CreateInTx`：`server/internal/service/issue.go:375–382` 明确不 commit、不 enqueue、不 publish；`444–477` 重新校验 parent/project 并继承项目；`499` 分配编号；`518–560` 使用真实创建代码。批准决定、候选终结/child 映射和所有 child 应在同一事务提交；并发以候选行锁/CAS及唯一 candidate-version+child-ordinal 约束收口，原 key 返回同组 child ID。标题去重 `490` 是相似业务条目的保护，不能充当请求幂等。

不要调用 `IssueService.Create` 作为审批内部步骤：它拥有/提交自己的事务，并在 `issue.go:368–369` 走 `maybeEnqueueOnAssign`。不要批准前创建一个可执行 agent-assigned todo，再靠 UI 把它藏起来。批准后若仅授权拆解，子任务创建应保持明确的不派发语义；普通自动分派规则不等于产品已批准开始编码。拟建 child 在 source workspace，需要原始真人有当前 source 创建/成员权限；外部 participant 的协作审批权不自动授予 source workspace membership。

### 4. 可用的 PR / CI / run / artifact 绑定

| 来源 | 当前权威事实与适用限制 |
| --- | --- |
| PR 与 issue 关联 | `github_pull_request` + `issue_pull_request`，或 `vcs_pull_request` + `issue_vcs_pull_request`；排除 reference_only。`service/delivery_evidence.go:118–125` 保留四张 PR/link 表的 SHARE NOWAIT fence；观察不是 remote live fetch。S2 应显式选择具体 provider/PR ID，不根据客户端 URL 或“最新 PR”猜候选。 |
| GitHub CI | `models.go:1583–1632`：PR 的 head_sha、snapshot_head_sha、snapshot_fetched_at、checks_rollup_state，与 per-check 的 pr_id/head_sha/name/status/conclusion。`queries/github_snapshot.sql:19–43` 按当前 head 原子写 snapshot 与替换 checks。`handler/github.go:286–291` 只有 snapshot head==current head 才 available；`1322–1326` webhook 仅触发 API refresh，payload 自报 checks 不作权威显示。 |
| VCS CI | `queries/vcs.sql:107–139` 按 current PR head + connection 聚合 commit status；`171–185` 单调时间 guard，旧 head 不参与。未知状态或 total=0 不能 passed。该镜像按 connection+sha+context，无独立 repository/check attempt 身份；跨仓相同 SHA 的严格验真不可从现有表推断，需保留边界或由 S4 补足。 |
| 集成权威 | Github installation / VCS connection 的 trust_revision 要绑定并重检；只保存 repo 文本/URL 不够。签名/连接撤换、PR 关联变化、head、检查集合及 observation 时间改变要使候选适用性失效。原 webhook/current config 校验复用 M1 `delivery_completion_webhook.go`。 |
| 协作运行 | `models.go:1088–1132` 的 CollaborationRun 包含真实 session/command/target、agent、grant/epoch、generation、attempt/proof、recording_incomplete、business_outcome。先从 run→session→issue 验证同任务及当前可读身份；仅 supplied run_id、摘要文字或 process exit=0 不能证明测试通过。失败日志保留原 run/attempt，与后续成功是不同记录。 |
| 普通制品 | `service/delivery_evidence.go:79` 的 `EvaluateDeliveryEvidenceInTx` / `220` 的 artifact 当前授权核对；实际 artifact execution 与 approval 关联、source revision、spec/artifact/receipt digest 和 producer task generation。复用此观察，不能仅接受客户端 artifact SHA 字符串。 |

阶段候选至少冻结：root issue ID + lifecycle/候选版本、source child ID/revision（如适用）、真实 PR provider/ID/集成+trust revision/repository/current SHA、服务器读取的 CI snapshot/check-set digest、制品 approval/execution/spec/artifact/receipt digest、相关测试 run/attempt/log 引用。审批只接受 expected IDs/versions/digests，由服务器重新读取事实，不接收 `passed=true` 作为技术通过依据。

GitHub snapshot 展示层有 30 分钟 stale 标识（`handler/github.go:226`），这是 UI 时效，不是自动取得强门禁 30 分钟授权的产品决定。S2 必须明确门禁用 freshness horizon，缺失/过期/未知均不通过；不能复制 UI fallback，把 unfamiliar rollup 的粗略展示当作合规验真。

人类可以登记测试说明、上传原始日志并对指定版本作业务决定；这证明的是 `human_attested`。它不能冒充已经由连接器观测的 CI、实际部署、实际回滚。发布批准与执行结果必须分开；S4 缺真实环境时，部署/回滚执行仍 NOT_RUN/BLOCKED，S2 的合成 fixture 仅证明平台拒绝/接受合同。

### 5. 建议最小模型与事务落点（建议，待主会话产品收敛）

**范围**：项目显式启用一份固定版本 S2 策略；实际业务阶段只附着在受该策略管理的 collaboration root，使用固定 `development / code_review / qa / release / verification`，不建立可配置任意工作流引擎，也不改 `issue.stage`（后者已是父子任务并行屏障分组，见 `issue.go:89`）。普通子任务保留本来工作流；其交付证据通过确切 parent/version 关联 root。

最小持久事实可用四类表加必要唯一索引；职责不要混进可随意编辑的 issue.metadata/properties：

1. 项目 S2 策略修订：project/workspace、fixed contract、enabled、revision、changed_by/reason/time。与 M1 policy 独立，默认关闭；旧项目无记录时保持既有行为。
2. 任务阶段配置/当前投影：issue/workspace、stable business owner、固定各阶段 human approver IDs、role revision、phase/current candidate/lifecycle/revision。角色变更记录在既有不可变协作 event 中，不把权限只保存在客户端或职业名称。
3. 不可变版本候选：candidate ID、issue、kind（拆解或交付）、version/supersedes、规范化内容/技术证据快照及 digest、submitted_by/source run、time。上游修改产生新版本，不覆盖旧 candidate。
4. 不可变阶段决定：candidate/version/digest、role revision、step、actual human actor、decision/reason、timestamp、对应批准的撤销/取代引用；必要 current pointer/CAS 与原 request ledger 同事务。拆解接受结果可在决定的受类型约束 payload 中保存 child ID/ordinal，不一定再建通用 mapping 服务；无论物理结构如何，都必须候选单次 materialization 的 DB 唯一约束。

写接口建议保持协作域：阶段配置、候选提交、候选决定、阶段推进、精确原 key 恢复/只读摘要。新增固定 DTO/parser 不把任意 JSON patch 暴露为权限接口。采用 `mutateCollaborationWorkflow` 既有用户/成员/epoch/事务/回执与 `finishWith` 事件/通知；新增动作加入封闭 allowlist，角色核权同时服务端执行。Agent 只能走它已获授权的运行输出/候选输入协议，不能借人类 workflow endpoint 审批。

**完成接入**：在 `PrepareDeliveryMutationInTx` 新完成路径中调用一个借用事务的 S2 facts validator。带未消费 M1 binding 的分支（`delivery_completion.go:261–278`）和无 binding 的分支（`294–304`）都要执行；检查必须在 off/observe 返回 NotApplicable 之前，所以 S2 enabled + M1 off/observe 仍不能绕过阶段。随后继续既有 M1 enforce，二者是叠加条件。已 consumed 的原操作 replay 保留当前读取/主体检查和历史回执语义，不重新推进阶段、不要求历史 done 成为“当前 done”。Finish 或实际写前后锁中须保证 S2 policy/role/candidate/evidence 不被并发更换。

**交付接入**：提交正式交付进入 in_review 前，也检查当前 verification 候选的证据与阶段规则（`collaboration_handoff.go:320` 之前）；否则旧客户端可跳过业务阶段直接发起最终验收。拒绝不得绑定附件或改当前 submission。返工引用原失败/退回决定，回到 development；仍不复用 in_review 做 code_review。

**重要逃逸路径**：只改 Prepare 不足以保护 S2。当前 `CheckDeliveryProjectChangeInTx` 只处理 M1 enforce；S2 enabled + M1 off/observe 时，移动/清空 project、删除 project/Issue、候选 child 换 parent 都要确认不会逃逸规则。建议复用原 project-change/delete 事务落点检查 S2 policy；必要的策略 off 是当前 owner/admin 的显式 reason + revision CAS，不默默退出监管。自动对所有普通 task 加全流程阶段会扩大任务范围，应避免。

### 6. 需要主会话收敛的产品决定

1. 固定审批人由谁配置、哪些角色允许兼任、自评审是否禁止；推荐具体任务固定真人，代码作者不得签自己的代码评审，QA 与 release 决定分离，但兼任约束属于产品选择。
2. 最终验收是否继续 creator；推荐此次复用 creator 精确验收，R38 单独管理处置，不把项目经理任命当作隐含继任。
3. 稳定业务责任人独立于流转 handler；变更由谁发起及是否令哪些批准失效。
4. 项目启用后适用已有任务还是仅新建/显式纳管任务；关闭策略后能否直接完成、已发布操作的撤销如何表述。不能由 migration 把所有已有任务静默纳管。
5. TC07 要求 PR/CI 齐备才进入 code_review，随后人工 code review 才进入 QA；CI freshness、允许 neutral/skipped、是否需要 required-check 精确集合不能从 UI rollup 自动决定。
6. 首期 `human_attested` 是否可用于非自动技术证据、哪类阶段仍必须连接器事实；无真实部署证据时只能保留未验收，不能把签字显示为发布完成。

### 7. 文件归属与验证建议

- 服务端实现 lane：新增固定 S2 领域类型/判定文件于 `server/internal/collaboration/`；service 文件 `collaboration_stage_*.go` 与最小 workflow/lifecycle/handoff/M1 接线；独立 `collaboration_stage.sql` + sqlc；独立迁移（无 FK、每个索引 CONCURRENTLY 单文件）。必须由一个 owner 负责所有共享 transaction/lock 协调文件。
- HTTP lane：`server/internal/handler/collaboration_stage*.go` 和实际 router 路由；strict duplicate/unknown/UUID/schema parsing；源任务权限不隐含 source artifact read。
- 客户端 lane：Core 的明确 schema/API/Query；共享 Views 阶段面板与原操作恢复。不得让客户端布尔值成为服务器授权。
- 验证依赖：fresh task-owned DB，service/handler/router 分开且 normal/race 分开；本研究未执行任何用例。
- 现有基线可复跑：`TestDeliveryCompletionEnforcementRouterOrdinaryAndMove`、`...BatchCommittedPrefix`、`...CreateAndCommentChild`、`TestDeliveryCompletionCollaborationRouterActualSourceAndRecovery`、`...CurrentIdentityAndChild`、`...SelfAttachmentRemainsUnboundUntilAccepted`、`TestDeliveryCompletionWebhookRouterTrustChangeRefuses`；再覆盖 `TestCollaborationGenericStatusesCannotManufactureReview`、`TestCollaborationDeliveryRevisionResubmissionAndExactAcceptance`、`TestCollaborationWorkflowRequiredWriteFailuresRollbackEverything` 与 `...LostCommitRecoveryAndRefusedKey`。
- S2 新矩阵必须有：所有 phase 正负路径；无 PR/无 CI/旧 head/未知 CI；候选/roles/policy/lifecycle 每一维版本变更；Agent/伪造身份/非指定人/撤权；并发 approve/revoke/advance；同原 key 与换 key 重复 materialization；child 创建故障全 rollback；off/observe/enforce 组合；正式 submit/self-complete/admin 完成与 project-detach 逃逸；QA 改判后 Agent 新结果不能覆盖旧人审；跨 workspace 来源读取与最小回执恢复。

### 8. 与其他任务边界

`11-10-git-engineering-material-workflow` 的 D1–D10 明确：Git 管定义与获批版本，Multica 管讨论/分派/运行；产品仅 Multica 账号；后台集成身份实际执行，不能冒充原生个人 review；需求 PR/MR 获批后受控自动合并只覆盖物料仓；代码自动合并未授权。见该任务 `design.md:13–40`、`155–182`。S2 只引用其稳定物料/版本边界，不实现另一个需求 Git 发布器、自动合并器、GitLab/GitHub 写入连接器或 D8 自动启动引擎。

M1 的 artifact/completion authorization 与 F2 的 report snapshot 是已有独立权威来源；不要复制 observer、生成平行完成令牌或把 F2 报告当成签字。真实 Provider/历史实验原账本仍由 `11-12-collaboration-real-runtime-acceptance` 管理；PAY-241 合成测试不得自动开启真实模型准入。

## Caveats / Not Found

- 未找到职业 code reviewer/QA/release approver 的现成服务端角色配置或阶段审批实体；现有 triage reviewer 是分诊责任，不是这组权力。
- 未找到可直接证明真实部署、回滚、支付/履约完成的现有 S2 事实；这部分必须由 S4 明确环境、集成与实际结果。
- Github/VCS 的当前镜像本身不是远端实时状态，历史 CI 表也不是不可变所有 attempt 审计。必须把候选使用的服务器事实固定下来，并保留具体来源和验证层级。
- 研究期间遇到几次过宽输出被截断与不存在路径 glob；已改用小范围具体文件读取，未据缺失命令作“功能不存在”判断。
- 本文为研究建议，不代表任何产品规则已获用户批准、代码已实现或用例已通过。
