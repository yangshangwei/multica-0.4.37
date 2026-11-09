# 场景可行性调研（2026-11-09，主会话直接核实）

两个调研子代理先后因网关 ECONNRESET 中断，没有留下结论文件。以下结论由主会话直接读代码和测试核实，均附出处。

## 共用设施

- 真实进程测试设施：`e2e/fixtures/collaboration-host-runtime.ts`。
  - `createHostActor`:407 创建 password 账号。
  - `enrollHostRuntime`:176 和 `startHostDaemon`:218 跑真实 poller 和 Provider 子进程。
  - `release`:351 放行 Provider，`freeze`/`thaw`:361 暂停/恢复 daemon，`act`:370 让 Provider 通过 run-local CLI 发评论、记 squad_activity、写文件。
  - `sendHostComment`:652 走界面提及、预览、发送；`hostControl`:639 走 Resume/Stop；`waitHostSettlement`:698 等运行结算（`stopped=true` 时不检查 business_outcome）。
- 外部拥有者提供资源：Codex 的 `offerClosureResource`（`e2e/fixtures/collaboration-multi-account-closure.ts:39`，已提交 `6e103fd4a`）只支持 agent。小队要仿照 `adoptHostTarget`（`collaboration-host-runtime.ts:430`）补齐 roster 角色。
- 账号工作台：`GET /api/me/workbench?view=current|participating`（`server/internal/handler/collaboration_workbench.go:43`，service 只接受这两个 view）。
- 通知：`GET /api/me/collaboration-notifications`。
  - 创建时处理人收到 `assigned`（`server/internal/service/collaboration.go:357`）。
  - 每个流程动作，除操作人外的参与者收到 kind 等于回执 code 的通知（`collaboration_workflow.go:336-348`）。
  - 被移除的人另收一条不含内容的 `removed`（`:351`）；读取时 `removed` 不可打开，也不带标题（`collaboration_notifications.go:185`）。
  - AI 运行的通知为 `ai_completed`/`ai_failed`/`ai_stopped`（`:187`）。
- 收件人资格：只按账号目录检查，password 模式下要求有 password 凭据（`collaboration_recipient_eligibility.go:21`），不要求有工作空间。没有工作空间的账号可以做参与者和处理人。

## 场景①：外包交付、乙方小队、知识沉淀

- 小队委派（`e2e/collaboration-host-squad.spec.ts:57-142`）：
  1. 提及小队 S 后，先启动 Leader 运行（`step_kind=leader`）。
  2. Leader 用 `act` 发一条 `[@成员](mention://agent/<id>)` 评论，回执里 `run_ids=[成员运行]`（:74-82），放行 Leader。
  3. 成员运行启动，`dispatch_comment` 等于派活原文（:89-92）。成员用 `act` 回帖，回执里出现 `continuation_run_id`（:119-120）。
  4. Leader 续跑（`step_kind=continuation`）用 `act` 发总结评论并记 `squad_activity`（outcome=action），结果为 `squad_activity_recorded`（:133-138），组状态变为 `succeeded`（:142）。
  5. 宿主容量为 1，Leader、成员、续跑依次执行。
- 外部拥有者的小队：offer、adopt、consent 的接口与 agent 相同（`router.go:1709-1717`）。roster 的每个资源都要拥有者分别授权（`adoptHostTarget`:452-461）。乙方经理 B 拥有小队，当前处理人或发起人采纳，B 逐个资源授权。没有找到拦截这一流程的服务端规则。
- 跨拥有者小队（一个小队里有不同拥有者的 Agent）：本次不设计。现有设施和文档只给出“分别检查各拥有者授权”的说法，需要先另行核实小队成员跨账号是否可构造。列为未覆盖项，不冒充覆盖。
- 知识（`apps/docs/content/docs/collaboration-knowledge.zh.mdx`）：
  - 由发起人或当前处理人提出，目标为来源工作空间或项目；目标工作空间的所有者或管理员确认。
  - 运行时注入需要 API 启动时设置 `MULTICA_KNOWLEDGE_CONTEXT_ENABLED=true`（`server/cmd/server/router.go:431`），默认关闭。
  - `e2e/collaboration-knowledge-deployed.spec.ts:39-75` 证明：开关打开时，同一任务的 host 运行在 `knowledge_observation.contributions` 中收到 `{candidate_id, version, source_issue_id, content_sha256}`。
  - 界面 helper 可复用：`fillKnowledge`/`submitKnowledge`/`chooseCandidate`/`approveKnowledge`（`e2e/fixtures/collaboration-knowledge.ts:105-163`），但 `openKnowledge` 用 legacy token 登录，要换成 cookie 登录。
  - 待运行验证：同一来源工作空间里的另一条新任务能否收到这条知识。如果收不到，要如实记录，并改为在同一任务上验证复用。

## 场景②：客服工单

- 手动和 AI 辅助的切换：`POST /ai-policy`，body 为 `{ai_policy:"assisted"|"manual"}`，回执 `ai_policy_changed`（`collaboration_ai_policy.go:43-79`）。
  - 只有发起人能切换（`collaboration_workflow.go:371`）。
  - 手动模式下，提交指令、恢复、登记、提供、采纳、授权、切换都会被拒绝，拒绝原因 `task_manual`（`:30-37`）。
  - 切到手动会暂停并停止全部目标；切回 AI 辅助“不改变执行意图”，仍需显式恢复（`:41-42`）。
- 文件流转：
  - 补充信息的请求和返回只带 Note，不带文件（`collaboration_workflow.go:36-39`、`collaboration_handoff.go:30-45`）。
  - 文件可以随评论共享（`collaboration_comments_create.go:37,88`），或用 `POST /attachments/deliver` 共享（`collaboration_attachments.go:590`）。提交结果可以附文件（`collaboration_handoff.go:36-45`）。
  - Agent 运行会收到这条任务全部已共享的文件，指令引用的排在最前（`collaboration_runtime_preparation.go:43-80`、`pkg/db/queries/collaboration_preparation.sql:13-20`），Provider 上报 `attachment_hashes`。
- 重开：只有发起人能操作（`collaboration_workflow.go:371`）。条件是终态且队列 closed（`collaboration_lifecycle.go:347-349`）。重开后每个目标都会被暂停（hold `issue_reopened`，`:485-495`），需显式恢复；service 测试中重开后 `queue_state=paused`（`collaboration_lifecycle_test.go:168-170`）。

## 场景③：异常治理

- 撤回授权（`collaboration_targets.go:35-71`）：只撤回操作者本人的授权，并停止依赖该授权的在途工作，不取消其他人等待中的指令。
  - 现有 `e2e/collaboration-live-revoke-web.spec.ts` 用模拟 daemon，运行停在 `launch_authorized`，终止一直未确认（:34-125）。
  - 期望值：回执 `grant_revoked`；通道 `stop_requested`；hold `grant_required`；在途运行 `forced_disposition=stopped`；排队输入保持 `waiting`、`group_id=null`；重新授权后 hold 仍在；终止未确认前 Resume 返回 `stop_unconfirmed`。
  - 用真实进程验证“撤权 → 进程真实终止并取得证明 → 重新授权 → 显式恢复 → 排队输入继续执行”，这一段此前没有用例覆盖。
- 移除参与者（`collaboration_workflow.go:147-153,262-300`）：
  - 不能移除发起人。如果被移除的人是当前处理人，处理责任退回发起人。
  - 撤销他的参与资格，撤回他在本任务上的全部授权，取消他发出的等待指令，停止受影响的在途工作。回执 `participant_removed`。
- 取消（`collaboration_lifecycle.go:497-521`）：只有发起人能操作。关闭会话，取消全部等待指令，停止全部目标，回执 `cancelled`。之后可以重开，规则同上。

## 结论

三个场景都可以在 password + 真实 host 运行环境里实现，不需要改产品代码，统一用 host Provider。场景①的知识注入需要 API 以知识开关打开的配置单独跑一批。跨拥有者小队不在本次范围内。
