# 实施与验收计划：知识候选逐层沉淀

- Query: 把同目录正式设计拆成可验证的最小实施批次。
- Scope: internal；K1–K5 已审阅，本次 F1–F4 合同修订已通过主会话独立复核；产品实现、隔离数据库与分层测试已有版本化证据；最终确定性整合门已通过。
- Date: 2026-10-31
- Revision: 2026-11-01；保留既定权限与 K1–K5，落实 F1–F4 及 G0/G1 确定性证据。
- Status: **各实施批次已落地，确定性质量门已通过，已固定提交并合入协作主分支**。G0/G1 已按提交证据冻结；本次 F1–F4 已通过主会话独立复核，G3 环境与所有权已全部落实。G2 是消费/会话代码的启用前验收，不是阻止开始实施的循环前置条件。

## 0. 入口与分工

- [x] 固定 runtime 产品提交 `0e1ee8e1e957b46a5bbd5d91a84772d69dd4d64b`、证据提交 `2b861347e62573ca765157ebf14629712793e7df`。G0/G1 只涵盖 `../10-30-collaboration-context-handoff-runtime/verification-integration-2026-11-01.md` 的来源/清除、handoff/实际输入绑定与确定性及 Web/macOS Electron 检查；真实模型、Windows/安装包/额度矩阵仍单列开放。
- [x] K1–K5 及外部处理人提案边界已通过独立设计复核，PRD 已补齐可观察 AC；本次 F1–F4 代码证据、正式合同与测试名已写入，独立设计复核见 `research/main-contract-review-2026-11-01.json`。
- [x] 主会话独立复核 `research/contract-revision-2026-11-01.md` 与 F1–F4：父行 KEY SHARE、独立修订行、source-only intent/无反向锁 fanout、有限缓存可见期、共享 request UUID 命名空间。
- [x] 主会话确认 existing knowledge task 的 planning artifacts/context manifests 齐备；沿用既有范围/权限授权，不新建重复任务。
- [x] 独立 worktree `/Volumes/artisan/code/2026/mc-knowledge-promotion-20261101` 基于 `765ab186a` 建立，branch `codex/collaboration-knowledge-promotion-20261101`；实际实现仍须分批验证。
- [x] Owner A 已建立专用 DB `multica_knowledge_20261101_b3c35f9c`；私有环境文件只存本机，默认测试禁止真实 Provider。所有权见 `research/execution-ownership-2026-11-01.json`。
- [x] 首批 `knowledge_schema` 为唯一 migration/sqlc/protocol/纯策略 owner；后续 handler/router/daemon/core 已按批次交接，最终 sqlc 生成由主会话负责。

建议分批所有权：A = schema/service/SQL；B = HTTP/core/views；C = ordinary/collaboration 消费与启动；D = 独立检查。A 的纯 protocol/helper 测试与 B 的 schema 测试可并行；共享 DB service/handler 全套顺序运行；C 等 A 的 typed resolver 和 runtime handoff 完成后再接入。

## 1. 先固定数据与纯策略（候选暂不生效）

Owner A；已由 `71379546c` 交付，证据见 `research/schema-implementation-2026-11-01/handoff.md`。本批实际文件：新 `server/pkg/protocol/collaboration_knowledge.go`、纯 policy/helper tests、新 migrations、`server/pkg/db/queries/collaboration_knowledge.sql`，必要 project/workspace/agent queries、失效 outbox queries 与 sqlc 输出；沿用 realtime relay，不新增依赖。

- [x] 写失败测试：新协议 version/UUID/decimal/hash、source/target 不一致、AgentActor/机器主体拒绝、source manager 与 target confirmer 的独立策略。
- [x] 落一张 candidate 版本实体，immutable source/target/content/hash/refs、lineage/version/supersedes/derived_from、状态 CAS 与 approval 事实。
- [x] 加入独立 `collaboration_knowledge_target` 计数表和专用 `collaboration_knowledge_invalidation` scan/target_hint 表；不在 workspace/project 热点行新增计数。首次计数行初始化的唯一键争用用 25ms lock_timeout 整事务回退，不能伪称 INSERT 自带 NOWAIT；普通 task 加 knowledge_context_version 和 metadata-only knowledge_delivery；区分 never-negotiated、claimed/no-disclosure、disclosed 与损坏 negotiated history，不用 base strings 或 user task.context 储存生效正文。
- [x] 落 CONCURRENTLY 索引单文件矩阵；fresh/upgrade/reapply/down 保护和 sqlc 漂移先通过。
- [x] 单独验证所有确定预算：64 KiB 正文、2 KiB 摘要、32 refs、16 KiB decoded metadata、HTTP 转义边界、16 项/128 KiB 每目标完整材料、2 MiB runtime wire。

回退点：只有 schema/reader/pure policy，没有批准 producer 或实际注入。

## 2. 人类提案、批准、撤回与原键恢复

Owner A；已实施，A2 证据见 `research/service-implementation-2026-11-01/handoff.md`；实际边界包括 `service/collaboration_knowledge.go` 中的候选/决定/原键回执逻辑，以及 `collaboration_knowledge_sources.go` / `collaboration_knowledge_view.go`；handler 接口可交 B，但业务权限必须在 service 重验。

- [x] 用 `dbfx` 构建 source/target 两个 workspace、creator、current handler、普通参与者、project human/agent lead、owner/admin、失权用户。
- [x] 提案权仅由当前 source Read + creator/handler 决定；目标可见性独立判断：允许无 source workspace membership 的外部 handler 向来源 workspace 提出，也允许已经合法可见且仍为当前关系的 source project；其他目标走正常可见性。完成态 source 仍可提炼，普通评论者和 ControlAI-only participant 不可借道。
- [x] 提案选择器只复用合法 source workspace/current-project 引用与已有正常可见目标，不新增私有目录查询。pending 创建不改目标 base context、managed 集合或 knowledge_revision；批准仍要求 source Read + target membership/确认者权限。
- [x] 新增 `TestKnowledgeExternalHandlerCanProposeToSourceWorkspaceWithoutMembership`、`TestKnowledgeExternalHandlerCanProposeToVisibleCurrentSourceProject`：真实 external handler 无 member 行仍可建 pending，目标上下文/修订不变，无额外私有 catalog 读取。
- [x] 新增 `TestKnowledgeProposeRejectsInvisibleArbitraryDestination`、`TestKnowledgeSourceProjectProposalRevalidatesCurrentRelationship`、`TestKnowledgeApprovalStillRequiresTargetMembershipAndAuthority`：隐藏任意目标与不存在目标同样拒绝；旧 source project 引用不能继续使用；获准提出不授予批准权。
- [x] 原事务中执行设计锁序：actor → 有序 workspace KEY SHARE NOWAIT/fence/member → status/project 存活 KEY SHARE NOWAIT（只在读取 lead 授权时 SHARE NOWAIT）→ 独立 target revision UPDATE NOWAIT → source issue/collaboration/session → candidate lineage。按动作区分必须存在的 target member 行和来源目标提案允许的缺失行；只锁读已可见 target 的必要 ID/归属，锁后重验 source 当前 project 关系及其他 target 可见性/角色/ref/version/hash。
- [x] pending 候选在 material 不可用/过期时仍可由 sourceRead + target confirmer 内容无关拒绝；不增加 hidden body 权限，Core capability/动作准备保持一致。
- [x] pending/rejected/withdrawn 不注入；批准/已批准撤回/纠错 supersede 原子更新状态、独立 target revision、内容无关 source event/receipt 和确定 revision 的 target_hint。proposal/reject/pending-withdraw 不给目标房间发待审活动提示。
- [x] 纠错的新 pending 版本不提前替换旧 approved；确认时原子 supersede，lineage 最多一个 approved；跨层 derived candidate 保持原 source 和独立人工确认。
- [x] 客户端发送前持久化 candidate_id + request_id；同 key/hash 回放返回同一 receipt；不同 payload/action/kind 拒绝；知识与 memory/command 共用 `(scope_kind,scope_id,actor_user_id,request_id)` 唯一命名空间，每个新意图用新 UUID，不新增 kind 隔离；unknown commit 不创建重复候选/批准；正文不进入 operation result 或 outbox。恢复地址使用 candidate ID + request ID，source 仅在服务器内部解析。
- [x] target confirmer source Read 丢失后的 withdraw 仍只返回内容无关回执；不能读取 candidate body/source identity。source 当前管理者也可撤回，不能增加批准或读取范围。

必须完成的 DB 并发矩阵：

| 并发对 | 两种顺序的断言 |
| --- | --- |
| approve ↔ source revoke / disabled actor | 先撤权则拒绝；先批准保留历史，但之后消费仍重新检查 |
| approve ↔ target membership/role/lead change | FOR SHARE 保护角色读取；旧角色不能在更新已提交后批准 |
| approve ↔ source memory purge/comment edit/delete | 不接收错误或已不能还原的来源版本；不能用候选副本绕过 purge |
| approve ↔ withdraw / correction approval | CAS/唯一 approved lineage，无半发布、无丢失撤回 |
| 同 lineage 多个纠错提案 | version 分配稳定、无重复版本，原内容不覆盖 |
| source 编辑/project 删除 ↔ 知识写入 | NOWAIT + 全事务回退；无锁反转死等和跨 workspace 错关联 |
| issue_counter 建单 / project description 编辑 ↔ preparation | 父 KEY SHARE 兼容普通 NO KEY UPDATE；无无关 knowledge_busy |
| approve 未提交 ↔ 建单 / issue project association / triage / iteration | 只更新独立 revision；普通路径不争知识计数，无不必要 55P03 |
| source purge 持 source 锁 ↔ target approval 持目标锁 | purge 的新增 intent INSERT 无 target/candidate/revision 访问；能在释放审批目标锁前完成原 source 操作 |

### F1–F4 的具名回归（实现名与证据以 inventory 为准）

| Finding | 必须新增的测试名与核心断言 |
| --- | --- |
| F1 | `TestKnowledgePrepareDoesNotContendWithIssueNumberAllocation`：真实建单事务已递增 issue_counter 未提交，preparation 成功；`TestKnowledgePrepareConflictsWithWorkspaceDelete`：既有删除行锁持有时按 NOWAIT 有界拒绝，无正文 |
| F1/F2 | `TestKnowledgePrepareDoesNotContendWithProjectDescriptionUpdate`：普通 description/revision 修改未提交仍可读取独立知识快照；`TestKnowledgePrepareBusyExhaustionNeverLaunchesOrReplays`：真实短暂知识争用耗尽后零新 launch/自动重放，不伪造空集 |
| F2 | `TestKnowledgeApprovalDoesNotBlockUnrelatedIssueWrites`：approval 已持独立 revision，建单、project 关联、triage/iteration 真实事务仍完成；`TestKnowledgeTargetInitializationRaceIsBounded`：两个首次初始化不会无限等待，只有一行，不以缺行掩盖 approved 历史 |
| F2 | `TestKnowledgeApprovalVersusProjectDeleteUsesParentFence`：两种顺序无孤立活跃目标；`TestKnowledgeDeletedTargetIsNotRecreatedByPendingFanout`：旧 intent 到达已删除目标不重建计数行、不改 source 审计 |
| F3 | `TestKnowledgeApprovedChangesAppendOneTargetHint`：批准、撤回、纠错各一次有效修订/提示，原 key 重放不重复；`TestKnowledgePendingChangesDoNotSignalTarget`：提案/拒绝及 source scan 命中 pending-only 候选都不能泄漏待审 source 活动 |
| F3 | `TestKnowledgeSourcePurgeAppendsIntentWithoutTargetLocks`：目标 revision 被审批占用时 purge 及 intent 仍提交，worker 后续重试；覆盖 human/admin purge；`TestKnowledgeFanoutNeverWaitsOnSourceWriter`：worker 持目标时不取 source/candidate，逆序调度无死等 |
| F3 | `TestKnowledgeSourceDeletionRetainsInvalidationRouting`：删除后没有 source JOIN 仍完成 hint；`TestKnowledgeAuthorityChangesFanOutWithoutCurrentMembershipJoin`：participant/空间成员撤权、账号禁用分别覆盖历史目标，目标管理员不需 source Read |
| F3 | `TestKnowledgeFanoutCrashAndReplayPreserveCursorAndRevision`：页中断、数据库回滚、忙目标、两个 worker、扫描中新批准/纠错不漏提示，同 cause/target 只增一次；`TestKnowledgeTargetHintRelayFailureRetainsStableID`：publish/commit 丢失、重复/乱序、重启后原 hint ID 保留 |
| F3 | `TestKnowledgeTargetHintHasOnlyPublicTargetFields`：实际帧仅 target_kind/target_id/knowledge_revision，无任何 source/candidate/body sentinel，最终发送重新授权；core `rejectsUnknownKnowledgeHintFields` 也保持旧 workbench 非空 payload 拒绝 |
| F3 | core `clearsAllKnowledgePagesBeforeRefetch`、`doesNotRestoreUnavailableKnowledgeFromLateResponse`、`expiresKnowledgeWhileOfflineOrFanoutStopped`、`expiryDeadlineWinsOverDelayedResponse`：代次先行、分页清除、30 秒可见期/15 秒重读、慢时钟/晚响应/后台恢复不延长权限 |
| F4 | `TestKnowledgeRequestIDConflictsAcrossOperationKinds`：同 source/actor UUID 先 memory/command 后 knowledge 及反向均 409，无第二操作/候选/hint；`TestKnowledgeRecoveryRejectsOtherOperationKind`：candidate-only recovery 不能误返回别的 kind 的回执 |


回退点：API 可管理候选和目标集合，但消费仍为 disabled/unsupported；未确认不影响任何真实 prompt。

## 3. 当前可见贡献解析与 human API

Owner A+B；实现覆盖 `service/collaboration_knowledge_view.go`、`handler/collaboration_knowledge.go`、router、typed core schema/API/query。

- [x] 实现 source collection、candidate detail/actions、candidate-only 原 receipt 恢复、project/workspace managed list；总数、摘要和详情都在各自授权范围内计算。
- [x] 元数据控制行与正文读取分开；source 不可读的 target confirmer 只能获得其目标内撤回所需的 restricted ID/state，不能得到 source ID/title/summary/hash。
- [x] 共享 KnowledgeSourceAvailability 先于 source list、detail、每个历史版本、target view 和 resolver 的正文/摘要序列化；required source purge/delete 时任何候选副本都不得绕回正文。withdraw/expiry 只阻止未来生效，不清掉 source 仍可用时的合法历史。resolver 另外检查 approved/expiry/supersede/withdraw/source ACL/target applicability，expiry 不依赖 sweeper。
- [x] 以服务端 DB 数据构造 OriginatorUserID、runtime.OwnerID 和 output audience；AccountableUserID/OriginatorSource 不用于授权。
- [x] 第一方 issue、private Chat、quick-create、autopilot、collaboration claim 都调用同一明确策略。跨 workspace 公共受众无法证明包含于 source ACL 时不注入；不能把这种不注入伪称整个 claim path 未实现。
- [x] 缺 principal/audience 返回明确不注入原因；DB 错误是重试错误，不返回上次正文或伪造 empty success。
- [x] 同 source workspace audience 证明、private Chat creator+agent gate、NULL originator、真实 sender 不同于 installer、跨 workspace 不扩权，均有具名测试。
- [x] malformed 新响应拒绝；旧响应无字段为 unsupported；所有能力由服务端给出，core 不从字段存在推断授权。

回退点：source/target 页面可正确呈现，runtime typed payload 尚未实际投喂。

## 4. 独立披露、真正启动与 exact-session 安全续作

实施切分遵循 `research/launch-seams-2026-11-01/pointer-retention-and-execution-handoff.md`：C1 先落 explicit ordinary env policy、per-command intent/fingerprint 与 Claude native-session callback；C2 在 A resolver/锁接口稳定后接普通 task 元数据/SQL；C3 接 HTTP/WS/managed claims、daemon 与实际输入。其他 adapter 初期验证安全 unsupported，不声称已有完整支持。

新增必验回归：ordinary 原 task 凭据保留而 collaboration 过滤不变；raw native-ID 与 attempt 绑定；unknown fresh 不可提前 pin；名义成功但 session 改变仍拒绝；configured args 无法覆盖会话选择；claim reclaim/runtime merge 不把旧 exposure 变成 legacy。不得新增未验证的厂商 CLI 参数。

Owner C；共享文件必须由 runtime owner 先交接。实现覆盖 `handler/daemon.go`、`handler/agent.go`、`service/task*.go`、相关 session selector SQL、daemon types/client/daemon.go/prompt.go、新 knowledge observer/helper。现有具体 seam 是 `agent.ExecOptions` 与 `Daemon.executeAndDrain`；collaboration handoff 只复用稳定材料 seam，不放宽其 business capability。

- [x] 新普通单次/批量 claim 按 effective provider/runtime/mode 协商；response 只有 source-free descriptor。FinalizeTaskClaim 仅保存 negotiation/claim identity，不读私有正文，也不把新 workspace/source 锁嵌入其 runtime/task-first 图。
- [x] 实现独立 finalized-claim preparation endpoint；准备前只取内部 hints，按设计 7.2 的 users→ordered workspaces KEY SHARE/member fences→project KEY SHARE→独立 revision SHARE→Chat/source/candidate→runtime/task NOWAIT 顺序锁后重验。commit 披露授权 metadata 后才发送正文；原 receipt 重试再次披露前也重验。
- [x] 对实际 writer 验证 NOWAIT/try-lock 查询而非假设已有 blocking helper 适用；明确 40001/40P01/55P03 的整个事务回退和有限次数，不能持 task 再等 workspace。
- [x] 单次/批量 finalized claim、claim response 丢失/重派、authority 在 build/finalize 之间变化、workspace 删除、旧 claim generation，全部证明未授权正文不披露。
- [x] 所有 BuildPrompt 分支都有来源块：ordinary issue、comment、Chat、quick-create、autopilot；base context/稳定 brief/project sidecar 不复制受控正文。
- [x] launch authorize endpoint 重验 prepare ID、exact manifest/prompt hash、claim generation、task/runtime/session binding、candidate/source/consumer/audience；不成为任意 source 读取 API。
- [x] 使用 `agent.ExecOptions.Lifecycle` / LaunchObserver；provider 物理 start 前检查，preparation 不消费；现有内部 retry 的每个实际 start 同样检查。unsupported effective backend 不得到私有 contribution；仅已证明安全的 legacy 工作继续，保留协商/私有历史风险拒绝交付。
- [x] 选定知识拒绝的精确非自动重试 failure reason，经 `executeAndDrain` / 普通 FailTask 终态收尾；禁止 dispatched-only RequeueTaskAfterClaimFailure，排除 shouldRetryWithFreshSession/MaybeRetryFailedTask 自动重放。第一 launch 的零执行事实和后来拒绝的已有副作用、usage、记录分别保留。
- [x] 任何 claim，包括旧客户端省略 knowledge_context_version 的分支，必须在序列化 PriorSessionID 之前检查实际 session。unsupported 客户端/backend 面对当前 task 保留协商或 provider 共享历史风险时拒绝交付；不能承诺自动 fresh，当前 task 的 NULL 不证明历史为空。单独解析 direct Chat pointer、GetLastTaskSession、GetLastChatTaskSession fallback 和显式 rerun 最终实际选择的 `(provider,runtime,session,scope,generation)`；不检查一个 task 后恢复另一个 session。
- [x] 查询/维护该 exact session 的累计 exposure，包含较早 turn、cancelled/failed turn，以及新任务回传 NULL session 但原指针保留的情况。metadata 中的 pending resume binding 也参与证明；AccountableUserID 不作为授权回退。
- [x] pin/complete/fail/chat pointer 写入与对应 exposure binding 原子化；unknown fresh native ID 只记录 generation、不可复用。实际 fallback 换 session 的关系需可观察，否则该 adapter 不得宣称支持私有 managed knowledge。
- [x] proven legacy/no-managed-exposure 不无故清空旧会话；negotiated disclosed history 缺失/损坏则拒绝该 exact session。历史贡献过期/撤回/失权或累计证明超过预算，当前输入尚未执行时选 fresh generation；launch 才发现变化则终态拒绝，不自动重发输入。
- [x] disclosure_authorized、launch_authorized、Started/termination 与 session_bound 是不同事实；保留网络丢响应的保守记录，不宣称擦除已合法交付的历史输出或 Provider 私人历史。

### K1–K5 的具名回归（实现名与证据以 inventory 为准）

| Finding | 必须新增的测试名与核心断言 |
| --- | --- |
| K1 | `TestKnowledgeResumeDirectChatPointerUsesExactExposure`：更新任务 session=NULL 时仍校验实际旧 Chat 指针；`TestKnowledgeResumeOlderSessionFallbackUsesChosenKey`：校验最终被选中的较旧 session，而非最新 task |
| K1 | `TestKnowledgeResumeCancelledTaskRetainsExposure`、`TestKnowledgeResumeSameSessionUnionsAllTurns`、`TestKnowledgeExplicitRerunUsesSourceSessionExposure`：取消/多 turn/精确 rerun 不丢累计来源 |
| K1 | `TestKnowledgeWithdrawnOnlyInOlderTurnForcesFresh`、`TestKnowledgeLegacyNoneDiffersFromCorruptNegotiatedHistory`、`TestKnowledgeSessionPointerAndExposureBindAtomically`：旧 turn 独有的失效贡献不被绕过；legacy 不误重置；新 native pointer 不先于绑定可见 |
| K1-D | `TestKnowledgeLegacyClientCannotResumeManagedSession`、`TestKnowledgeUnsupportedBackendCannotResumeManagedSession`：省略版本/不支持 backend 的真实 claim，保留协商/私有历史风险均拒绝交付；安全 legacy 另有独立正向证据 |
| K1-D | `TestKnowledgeDowngradeDoesNotTreatNullCurrentTaskAsEmptyHistory`、`TestKnowledgeProvenLegacySessionRemainsResumable`：旧 task NULL 不覆盖较早 exposure 或损坏 proof；证明无 exposure 的 legacy 仍可恢复 |
| K2 | `TestKnowledgeClaimFinalizationDisclosesNoPrivateFields`、`TestKnowledgePrepareBeforeFinalizedClaimRefused`：原 claim 只有 descriptor，任何未 finalize 路径没有正文/summary/source ID |
| K2 | `TestKnowledgeAuthorityChangeBetweenClaimBuildAndFinalize`：早期 hints 后权限变化，finalize 不放正文，后续 preparation 当前拒绝；`TestKnowledgePreparationVersusWithdrawAndSourceRevoke`：两种提交顺序的独立披露线性化 |
| K2 | `TestKnowledgePreparationWorkspaceDeleteLockOrder`、`TestKnowledgePreparationFinalizerContentionUsesNowait`、`TestKnowledgePreparationLostResponseReauthorizes`、`TestKnowledgePreparationRejectsOldClaimGeneration`：实际 writer 争用无 task→workspace 死等，重发不会按旧授权吐正文 |
| K3 | `TestOrdinaryKnowledgeFirstLaunchRefusalIsTerminal`：StartTask 后、首进程前授权变化，零物理 launch、当前 task 正常 failed、没有自动 requeue/input replay；`TestOrdinaryKnowledgeLaterLaunchRefusalDoesNotReplay`：第一次已有副作用，第二次拒绝保留 usage/transcript/termination 且只执行一次业务输入 |
| K3 | `TestOrdinaryUnsupportedKnowledgeBackendRunsBaseTask`：effective backend 不支持且没有保留协商/私有历史风险时，issue/chat 等普通入口执行原 base；风险存在则拒绝 |
| K4 | `TestKnowledgeHumanListSuppressesPurgedSourceText`、`TestKnowledgeHumanDetailAndHistorySuppressDeletedSourceText`、`TestKnowledgeTargetViewSuppressesUnavailableSourceText`：每个人类投影无正文/summary/派生引文 sentinel |
| K4 | `TestKnowledgeWithdrawnOrExpiredHistoryRemainsReadableWhenSourceAvailable`：普通失效不等于 purge；core 的 `doesNotRestoreUnavailableKnowledgeFromLateResponse` 覆盖清缓存/取消旧请求后晚到响应 |
| K5 | `TestKnowledgeTargetWithdrawalRecoveryAfterSourceAndTargetRoleLoss`：响应丢失后凭 candidate ID+原 request ID 恢复自己的内容无关 receipt，不需 source ID；`TestKnowledgeRecoveryRejectsOtherActorAndCandidate`、`TestKnowledgeCreateLostResponseUsesPreselectedCandidateID` 覆盖所有者及创建恢复 |

分层验收保留实际 fake executable 输入断言：准确 eligible 内容/版本/来源进入 stdin；不进入稳定配置；originator 与 owner 任一失权不泄漏；fake 单次/批量 claims、Chat、quick-create、autopilot 的输入接线；普通评论合并由实际 CreateComment/claim/preparation 的多作者用例与 daemon 输入层组合证明。命名测试是验收计划，不能把名字存在或 `[no tests to run]` 当通过。

G2 的 adapter/native-session、全部 pointer writer 与独立 disclosure/authorize 已有 C1–C4 实施和分层证据；剩余门为最后具名并发矩阵、整合 Go、当前源码 UI 复验及最终证据核对。

## 5. 持久失效、Shared Web/Desktop 与内置指导

Owner A 负责 service/SQL/outbox，B 负责 core/views；实现覆盖 `service/collaboration_knowledge_invalidation.go`、相关 source/账号/成员变更与删除 writer、realtime dispatcher、`packages/core/collaboration/knowledge-*`、core API client、`packages/views/collaboration` 的候选组件、project/workspace existing settings/context 页面 slot、en/zh-Hans。

- [x] 按 design §8.1 覆盖所有触发：批准/withdraw/supersede 原事务写 target_hint；source human/admin purge、comment/source 删除、ACL/participant 变更原事务只追加 source scan；workspace member 变更追加 source_workspace scan；账号禁用/credential 变更追加保守 all_sources scan。不得依赖事后 best-effort publish 或待完成的逐单 reconciliation。
- [x] worker 锁 own job，只扫描曾批准（published revision 非空）的历史；pending-only 目标不发提示。固定上界/candidate ID 游标，每轮最多 100 行、每短事务最多一个目标；父行 KEY SHARE NOWAIT→独立 revision UPDATE NOWAIT，绝不回头取 source/candidate/task 锁。唯一 cause/target、revision/hint/游标原子提交；source/target 删除保留必要路由/未完成 outbox，目标缺失不复活。
- [x] target_hint drainer 复用 PublishWithID/relay/final-send authorization；固定原 hint ID、只送三字段 payload 到受权目标房间，失败不标 delivered。普通 workbench 空 payload 合同保持。生命周期注册、重启恢复、pending age/重试诊断不记录 source 内容。
- [x] 当前 human GET 从一致授权投影返回 checked_at/valid_until/next_expiry；core key 含 server/viewer/scope，target 提示先递增本地读取代次、cancel/remove 全部正文页，再 GET。30 秒最长可见期、15 秒可见页面重读；expiry/离线/回前台先清除，失败不保留 stale body，分页/晚到响应/慢时钟不能延长租期。runtime 不使用 UI 租期缓存授权。
- [x] source 内提出候选、看准确 revision/hash、审核动作 awaited、409/428 后重新读取；失败不能乐观显示已生效。非成员外部 handler 的选择器能显示已合法可见的来源 workspace/current project，不借目标页面或新目录 API 扩权；其他目标仅列普通可见项。
- [x] target 独立展示 managed contributions、来源/版本/验证范围/过期；base editor 不接收复制后的合并正文。
- [x] 权限变化、source purged、restricted target 管理行和撤回路径有清楚状态；旧服务端 unsupported 不虚构“空知识”。源/目标 cache 必须随 live access/purge/expiry 变化取消和清除正文，不能只依赖 target knowledge_revision。
- [x] core 纯 schema/query policy 矩阵只测一次；共享组件测试覆盖交互/关键回归；不在 app 测试重复 shared logic。
- [x] 同步内置协作指导和 source map：Agent 可整理材料，但不能冒充 human 确认、自动生效或写 skill。
- [x] Web UI6 覆盖仅有目标权限管理员的控制/撤回；Web UI2/UI7 与原生 macOS UI8 覆盖来源清除、实际目标提示与贡献变化。Core 定时器矩阵覆盖离线/晚响应的 30 秒上限。原生目标单方管理员未单独跑同一旅程，按共享页面与 Web 行为分层证明；实际用例名见 UI inventory。
- [x] 30 张图/18 组状态完成独立视觉审查；当前合并源码 production Web 7 项、原生 macOS Electron 1 项复验通过。采用分层证明：真实页面覆盖提案/精确确认/纠错/撤回，普通与 Host 组合用实际 fake stdin 覆盖后续输入变化；没有宣称每个平台都执行了一条 UI→Provider 完整旅程。

## 6. 完整质量门与版本证据

以下为计划命令，不是通过记录；最终测试名以新增文件为准，并防止 `[no tests to run]` 被当作通过：

```sh
go -C server test ./pkg/protocol ./internal/collaboration ./internal/daemon/execenv ./internal/daemon -run 'Knowledge|ManagedContribution' -count=1 -v
go -C server test ./internal/service -run 'Knowledge|ManagedContribution' -count=1 -v
go -C server test ./internal/handler ./cmd/server -run 'Knowledge' -count=1 -v
make sqlc
go -C server vet ./...
go -C server build ./...
pnpm lint
pnpm typecheck
pnpm test
```

- [x] DB 命令带任务专用 DATABASE_URL，service 与 handler 全套按计划串行；先确认测试真的运行、零意外 skip。
- [x] changed tests 通过后再扩大到相关 collaboration、project/workspace、claim/resume 的既有套件；只有新变更/新失败才重复已通过检查。
- [x] 每次保存 HEAD、dirty diff/hash、命令、平台/DB、测试名与退出码。sqlc 前后记录生成变化并确认无非预期 drift。
- [x] 独立 check 对照 PRD/设计审查读写权限、lineage/幂等、不可变内容、预算、早期拒绝、实际 stdin、resume、旧客户端，以及删除/回退。
- [x] 更新相关 server/core/views specs、问题→修复→证据映射；符合 Lore 的分批提交，只带任务所有文件。
- [x] 本次计划修订不调用 Provider；已结束的 upstream 502 实验窗口不自动重开。未来如另有获授权真实观察，单列版本/来源/覆盖路径，并由原实验 owner 管理账本；fake 通过不证明模型理解或清偿原 E 系列。

## 7. 可标完成的条件

必须逐项有证据：未批准不改变输入；人类权限矩阵成立；批准精确版本并保留来源；source/target/audience 不扩权；纠错/expiry/withdraw 影响下一次实际输入；无授权回退/旧 session 绕过；所有第一方 claim 表面已接线；Web/Desktop 真实入口完成；历史及 rollback 保留。

开始实施后，缺任意一项则知识子任务保持 in_progress 并精确写缺口；当前确定性验收已通过；任务保留 in_progress 以追踪独立真实模型和发布门，不能把它们标作已完成。runtime 原任务、首轮真实实验、Windows/安装包/多账号额度矩阵的验收仍各归其原任务，不用知识单元测试代替。

## Caveats / Not Found

- 历史设计核对见 preflight 与 contract revision；当前执行证据见各批 handoff、`research/acceptance-audit-2026-11-01.md` 及最终补验目录。历史报告中的“未运行/待实施”只描述当时快照。
- 候选版本、独立 target revision、持久 scan/hint、有限缓存可见期、exact-session exposure 与披露/启动授权均由知识分支新增；原 runtime 的通过结论不自动覆盖这些代码。
- 主会话应将普通 Provider lifecycle contract、最终 source/target 锁序和撤回双侧角色作为技术复核重点；这些不是要求用户重新确认已经决定的 source 不扩权政策。
- External references: 无新外部技术选择；实现沿用仓库 Go/SQL/React Query/strict schema 与 fake-executable 测试模式。

## 最终验收补验跟踪

原始缺口核对见 `research/acceptance-audit-2026-11-01.md`，最新核对见 `research/final-acceptance-review-2026-11-01.md`。审批/恢复/人类删除投影的 30 个叶子场景、准备/真实写者/来源撤权/普通评论合并的 12 个叶子场景均已有 normal/race 证据；全套发现的夹具隔离、删除分类、指导篇幅和复用库残留问题均已完成修正/诊断，最终全新库 handler→service 全套通过；测试清理增量由独立顺序 normal/race 复验闭合。普通组合为 23 个叶子场景/27 个 pass 事件，协作 Host 组合为 13 个叶子场景/16 个事件，容器不重复计数。生产 Web/原生 macOS 冻结快照 8 个独立用例及 30 张视觉复核通过；最终整合源码已按 server/frontend/product 清单核对，零漂移。

最终产品提交 `96cedf4850319a06c98fdda3d67d82595f01b6b2`；确定性 G2 通过，完整证据见 `verification-integration-2026-11-01.md`。真实模型及父任务发布门保留开放。
