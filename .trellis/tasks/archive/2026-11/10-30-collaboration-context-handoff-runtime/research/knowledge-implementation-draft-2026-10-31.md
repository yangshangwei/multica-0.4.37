# Implementation draft: 知识候选逐层沉淀

- Query: 把同目录知识设计草案拆成可验证的最小实施批次。
- Scope: internal；规划草案，没有实施、测试或数据库操作。
- Date: 2026-10-31
- Revision: 2026-11-01；补齐非成员外部 handler 的来源目标提案与隐藏任意目标拒绝。
- Status: **DRAFT / 不代表知识子任务已激活**。主会话在 runtime G0–G2 达成、设计复核后，才将本文提升为 sibling `implement.md`。

## 0. 入口与分工

- [ ] 固定 runtime 的最终集成 HEAD/dirty tree hash、source/provenance DTO、purge/availability、ordinary 与 collaboration 实际启动 seam。
- [ ] 将 `knowledge-design-draft-2026-10-31.md` 的选择复核为正式合同，补入知识任务 PRD 的可观察 AC。无新增扩权决策；若技术合同必须变更，主会话记录理由和影响。
- [ ] 主会话确认 existing knowledge task 的 planning artifacts/context manifests 齐备并按既有授权推进；不新建重复任务。
- [ ] 准备独立 worktree 与专用测试 DB；记录版本/命令/数据库标识，禁止真实 Provider 自动混进默认测试。
- [ ] 指定唯一 migration/sqlc owner。普通 daemon.go、handler/daemon.go、router.go、protocol 与 core API client 修改须顺序交接，不能多个 agent 同时写。

建议分批所有权：A = schema/service/SQL；B = HTTP/core/views；C = ordinary/collaboration 消费与启动；D = 独立检查。A 的纯 protocol/helper 测试与 B 的 schema 测试可并行；共享 DB service/handler 全套顺序运行；C 等 A 的 typed resolver 和 runtime handoff 完成后再接入。

## 1. 先固定数据与纯策略（候选暂不生效）

Owner A；预计文件：新 `server/pkg/protocol/collaboration_knowledge.go`、纯 policy/helper tests、新 migrations、`server/pkg/db/queries/collaboration_knowledge.sql`，必要 project/workspace/agent queries 与 sqlc 输出。

- [ ] 写失败测试：新协议 version/UUID/decimal/hash、source/target 不一致、AgentActor/机器主体拒绝、source manager 与 target confirmer 的独立策略。
- [ ] 落一张 candidate 版本实体，immutable source/target/content/hash/refs、lineage/version/supersedes/derived_from、状态 CAS 与 approval 事实。
- [ ] 加入 workspace/project 独立 knowledge_revision、普通 task 的 knowledge_context_version 和 metadata-only knowledge_delivery；区分 never-negotiated、claimed/no-disclosure、disclosed 与损坏 negotiated history，不用 base strings 或 user task.context 储存生效正文。
- [ ] 落 CONCURRENTLY 索引单文件矩阵；fresh/upgrade/reapply/down 保护和 sqlc 漂移先通过。
- [ ] 单独验证所有确定预算：64 KiB 正文、2 KiB 摘要、32 refs、16 KiB decoded metadata、HTTP 转义边界、16 项/128 KiB 每目标完整材料、2 MiB runtime wire。

回退点：只有 schema/reader/pure policy，没有批准 producer 或实际注入。

## 2. 人类提案、批准、撤回与原键恢复

Owner A；预计 `service/collaboration_knowledge.go`、`collaboration_knowledge_operation.go`、必要 source helper；handler 接口可交 B，但业务权限必须在 service 重验。

- [ ] 用 `dbfx` 构建 source/target 两个 workspace、creator、current handler、普通参与者、project human/agent lead、owner/admin、失权用户。
- [ ] 提案权仅由当前 source Read + creator/handler 决定；目标可见性独立判断：允许无 source workspace membership 的外部 handler 向来源 workspace 提出，也允许已经合法可见且仍为当前关系的 source project；其他目标走正常可见性。完成态 source 仍可提炼，普通评论者和 ControlAI-only participant 不可借道。
- [ ] 提案选择器只复用合法 source workspace/current-project 引用与已有正常可见目标，不新增私有目录查询。pending 创建不改目标 base context、managed 集合或 knowledge_revision；批准仍要求 source Read + target membership/确认者权限。
- [ ] 新增 `TestKnowledgeExternalHandlerCanProposeToSourceWorkspaceWithoutMembership`、`TestKnowledgeExternalHandlerCanProposeToVisibleCurrentSourceProject`：真实 external handler 无 member 行仍可建 pending，目标上下文/修订不变，无额外私有 catalog 读取。
- [ ] 新增 `TestKnowledgeProposeRejectsInvisibleArbitraryDestination`、`TestKnowledgeSourceProjectProposalRevalidatesCurrentRelationship`、`TestKnowledgeApprovalStillRequiresTargetMembershipAndAuthority`：隐藏任意目标与不存在目标同样拒绝；旧 source project 引用不能继续使用；获准提出不授予批准权。
- [ ] 原事务中执行设计锁序：actor → 有序 workspace/fence/member → status/target row NOWAIT → source issue/collaboration/session → candidate lineage。按动作区分必须存在的 target member 行和来源目标提案允许的缺失行；只锁读已可见 target 的必要 ID/归属，锁后重验 source 当前 project 关系及其他 target 可见性/角色/ref/version/hash。
- [ ] pending/rejected/withdrawn 不注入；批准原子更新状态、target revision 和内容无关 event/receipt。
- [ ] 纠错的新 pending 版本不提前替换旧 approved；确认时原子 supersede，lineage 最多一个 approved；跨层 derived candidate 保持原 source 和独立人工确认。
- [ ] 客户端发送前持久化 candidate_id + request_id；同 key/hash 回放返回同一 receipt；不同 payload/action 拒绝；unknown commit 不创建重复候选/批准；正文不进入 operation result 或 outbox。恢复地址使用 candidate ID + request ID，source 仅在服务器内部解析。
- [ ] target confirmer source Read 丢失后的 withdraw 仍只返回内容无关回执；不能读取 candidate body/source identity。source 当前管理者也可撤回，不能增加批准或读取范围。

必须完成的 DB 并发矩阵：

| 并发对 | 两种顺序的断言 |
| --- | --- |
| approve ↔ source revoke / disabled actor | 先撤权则拒绝；先批准保留历史，但之后消费仍重新检查 |
| approve ↔ target membership/role/lead change | FOR SHARE 保护角色读取；旧角色不能在更新已提交后批准 |
| approve ↔ source memory purge/comment edit/delete | 不接收错误或已不能还原的来源版本；不能用候选副本绕过 purge |
| approve ↔ withdraw / correction approval | CAS/唯一 approved lineage，无半发布、无丢失撤回 |
| 同 lineage 多个纠错提案 | version 分配稳定、无重复版本，原内容不覆盖 |
| source 编辑/project 删除 ↔ 知识写入 | NOWAIT + 全事务回退；无锁反转死等和跨 workspace 错关联 |

回退点：API 可管理候选和目标集合，但消费仍为 disabled/unsupported；未确认不影响任何真实 prompt。

## 3. 当前可见贡献解析与 human API

Owner A+B；预计 `service/collaboration_knowledge_view.go`、`handler/collaboration_knowledge.go`、router、typed core schema/API/query。

- [ ] 实现 source collection、candidate detail/actions、candidate-only 原 receipt 恢复、project/workspace managed list；总数、摘要和详情都在各自授权范围内计算。
- [ ] 元数据控制行与正文读取分开；source 不可读的 target confirmer 只能获得其目标内撤回所需的 restricted ID/state，不能得到 source ID/title/summary/hash。
- [ ] 共享 KnowledgeSourceAvailability 先于 source list、detail、每个历史版本、target view 和 resolver 的正文/摘要序列化；required source purge/delete 时任何候选副本都不得绕回正文。withdraw/expiry 只阻止未来生效，不清掉 source 仍可用时的合法历史。resolver 另外检查 approved/expiry/supersede/withdraw/source ACL/target applicability，expiry 不依赖 sweeper。
- [ ] 以服务端 DB 数据构造 OriginatorUserID、runtime.OwnerID 和 output audience；AccountableUserID/OriginatorSource 不用于授权。
- [ ] 第一方 issue、private Chat、quick-create、autopilot、collaboration claim 都调用同一明确策略。跨 workspace 公共受众无法证明包含于 source ACL 时不注入；不能把这种不注入伪称整个 claim path 未实现。
- [ ] 缺 principal/audience 返回明确不注入原因；DB 错误是重试错误，不返回上次正文或伪造 empty success。
- [ ] 同 source workspace audience 证明、private Chat creator+agent gate、NULL originator、真实 sender 不同于 installer、跨 workspace 不扩权，均有具名测试。
- [ ] malformed 新响应拒绝；旧响应无字段为 unsupported；所有能力由服务端给出，core 不从字段存在推断授权。

回退点：source/target 页面可正确呈现，runtime typed payload 尚未实际投喂。

## 4. 独立披露、真正启动与 exact-session 安全续作

Owner C；共享文件必须由 runtime owner 先交接。预计 `handler/daemon.go`、`handler/agent.go`、`service/task*.go`、相关 session selector SQL、daemon types/client/daemon.go/prompt.go、新 knowledge observer/helper。现有具体 seam 是 `agent.ExecOptions` 与 `Daemon.executeAndDrain`；collaboration handoff 只复用稳定材料 seam，不放宽其 business capability。

- [ ] 新普通单次/批量 claim 按 effective provider/runtime/mode 协商；response 只有 source-free descriptor。FinalizeTaskClaim 仅保存 negotiation/claim identity，不读私有正文，也不把新 workspace/source 锁嵌入其 runtime/task-first 图。
- [ ] 实现独立 finalized-claim preparation endpoint；准备前只取内部 hints，按设计 7.2 的 users→ordered workspaces/member fences→target/Chat/source/candidate→runtime/task NOWAIT 顺序锁后重验。commit 披露授权 metadata 后才发送正文；原 receipt 重试再次披露前也重验。
- [ ] 对实际 writer 验证 NOWAIT/try-lock 查询而非假设已有 blocking helper 适用；明确 40001/40P01/55P03 的整个事务回退和有限次数，不能持 task 再等 workspace。
- [ ] 单次/批量 finalized claim、claim response 丢失/重派、authority 在 build/finalize 之间变化、workspace 删除、旧 claim generation，全部证明未授权正文不披露。
- [ ] 所有 BuildPrompt 分支都有来源块：ordinary issue、comment、Chat、quick-create、autopilot；base context/稳定 brief/project sidecar 不复制受控正文。
- [ ] launch authorize endpoint 重验 prepare ID、exact manifest/prompt hash、claim generation、task/runtime/session binding、candidate/source/consumer/audience；不成为任意 source 读取 API。
- [ ] 使用 `agent.ExecOptions.Lifecycle` / LaunchObserver；provider 物理 start 前检查，preparation 不消费；现有内部 retry 的每个实际 start 同样检查。unsupported effective backend 不得到私有 contribution，但继续原普通 task 执行。
- [ ] 选定知识拒绝的精确非自动重试 failure reason，经 `executeAndDrain` / 普通 FailTask 终态收尾；禁止 dispatched-only RequeueTaskAfterClaimFailure，排除 shouldRetryWithFreshSession/MaybeRetryFailedTask 自动重放。第一 launch 的零执行事实和后来拒绝的已有副作用、usage、记录分别保留。
- [ ] 单独解析 direct Chat pointer、GetLastTaskSession、GetLastChatTaskSession fallback 和显式 rerun 最终实际选择的 `(provider,runtime,session,scope,generation)`；不检查一个 task 后恢复另一个 session。
- [ ] 查询/维护该 exact session 的累计 exposure，包含较早 turn、cancelled/failed turn，以及新任务回传 NULL session 但原指针保留的情况。metadata 中的 pending resume binding 也参与证明；AccountableUserID 不作为授权回退。
- [ ] pin/complete/fail/chat pointer 写入与对应 exposure binding 原子化；unknown fresh native ID 只记录 generation、不可复用。实际 fallback 换 session 的关系需可观察，否则该 adapter 不得宣称支持私有 managed knowledge。
- [ ] proven legacy/no-managed-exposure 不无故清空旧会话；negotiated disclosed history 缺失/损坏则拒绝该 exact session。历史贡献过期/撤回/失权或累计证明超过预算，当前输入尚未执行时选 fresh generation；launch 才发现变化则终态拒绝，不自动重发输入。
- [ ] disclosure_authorized、launch_authorized、Started/termination 与 session_bound 是不同事实；保留网络丢响应的保守记录，不宣称擦除已合法交付的历史输出或 Provider 私人历史。

### K1–K5 的具名回归（全部待实现/运行）

| Finding | 必须新增的测试名与核心断言 |
| --- | --- |
| K1 | `TestKnowledgeResumeDirectChatPointerUsesExactExposure`：更新任务 session=NULL 时仍校验实际旧 Chat 指针；`TestKnowledgeResumeOlderSessionFallbackUsesChosenKey`：校验最终被选中的较旧 session，而非最新 task |
| K1 | `TestKnowledgeResumeCancelledTaskRetainsExposure`、`TestKnowledgeResumeSameSessionUnionsAllTurns`、`TestKnowledgeExplicitRerunUsesSourceSessionExposure`：取消/多 turn/精确 rerun 不丢累计来源 |
| K1 | `TestKnowledgeWithdrawnOnlyInOlderTurnForcesFresh`、`TestKnowledgeLegacyNoneDiffersFromCorruptNegotiatedHistory`、`TestKnowledgeSessionPointerAndExposureBindAtomically`：旧 turn 独有的失效贡献不被绕过；legacy 不误重置；新 native pointer 不先于绑定可见 |
| K2 | `TestKnowledgeClaimFinalizationDisclosesNoPrivateFields`、`TestKnowledgePrepareBeforeFinalizedClaimRefused`：原 claim 只有 descriptor，任何未 finalize 路径没有正文/summary/source ID |
| K2 | `TestKnowledgeAuthorityChangeBetweenClaimBuildAndFinalize`：早期 hints 后权限变化，finalize 不放正文，后续 preparation 当前拒绝；`TestKnowledgePreparationVersusWithdrawAndSourceRevoke`：两种提交顺序的独立披露线性化 |
| K2 | `TestKnowledgePreparationWorkspaceDeleteLockOrder`、`TestKnowledgePreparationFinalizerContentionUsesNowait`、`TestKnowledgePreparationLostResponseReauthorizes`、`TestKnowledgePreparationRejectsOldClaimGeneration`：实际 writer 争用无 task→workspace 死等，重发不会按旧授权吐正文 |
| K3 | `TestOrdinaryKnowledgeFirstLaunchRefusalIsTerminal`：StartTask 后、首进程前授权变化，零物理 launch、当前 task 正常 failed、没有自动 requeue/input replay；`TestOrdinaryKnowledgeLaterLaunchRefusalDoesNotReplay`：第一次已有副作用，第二次拒绝保留 usage/transcript/termination 且只执行一次业务输入 |
| K3 | `TestOrdinaryUnsupportedKnowledgeBackendRunsBaseTask`：effective backend 不支持时不获取私有贡献，但 issue/chat 等普通入口照常执行 |
| K4 | `TestKnowledgeHumanListSuppressesPurgedSourceText`、`TestKnowledgeHumanDetailAndHistorySuppressDeletedSourceText`、`TestKnowledgeTargetViewSuppressesUnavailableSourceText`：每个人类投影无正文/summary/派生引文 sentinel |
| K4 | `TestKnowledgeWithdrawnOrExpiredHistoryRemainsReadableWhenSourceAvailable`：普通失效不等于 purge；core 的 `doesNotRestoreUnavailableKnowledgeFromLateResponse` 覆盖清缓存/取消旧请求后晚到响应 |
| K5 | `TestKnowledgeTargetWithdrawalRecoveryAfterSourceAndTargetRoleLoss`：响应丢失后凭 candidate ID+原 request ID 恢复自己的内容无关 receipt，不需 source ID；`TestKnowledgeRecoveryRejectsOtherActorAndCandidate`、`TestKnowledgeCreateLostResponseUsesPreselectedCandidateID` 覆盖所有者及创建恢复 |

另外保留实际 fake executable 输入断言：准确 eligible 内容/版本/来源进入 stdin；不进入稳定配置；originator 与 owner 任一失权不泄漏；fake 单次/批量 claims、comment coalescing、Chat、quick-create、autopilot 全路径实际接线。命名测试是验收计划，不能把名字存在或 `[no tests to run]` 当通过。

G2 的剩余实现门是 adapter/native-session 实际绑定和全部 pointer writer 的原子维护，以及独立 disclosure/authorize 锁与现有 writer 的关系；本文提出合同，不声称代码已经满足。

## 5. Shared Web/Desktop 与内置指导

Owner B；预计 `packages/core/collaboration/knowledge-*`、core API client、`packages/views/collaboration` 的候选组件、project/workspace existing settings/context 页面 slot、en/zh-Hans。

- [ ] source 内提出候选、看准确 revision/hash、审核动作 awaited、409/428 后重新读取；失败不能乐观显示已生效。非成员外部 handler 的选择器能显示已合法可见的来源 workspace/current project，不借目标页面或新目录 API 扩权；其他目标仅列普通可见项。
- [ ] target 独立展示 managed contributions、来源/版本/验证范围/过期；base editor 不接收复制后的合并正文。
- [ ] 权限变化、source purged、restricted target 管理行和撤回路径有清楚状态；旧服务端 unsupported 不虚构“空知识”。源/目标 cache 必须随 live access/purge/expiry 变化取消和清除正文，不能只依赖 target knowledge_revision。
- [ ] core 纯 schema/query policy 矩阵只测一次；共享组件测试覆盖交互/关键回归；不在 app 测试重复 shared logic。
- [ ] 同步内置协作指导和 source map：Agent 可整理材料，但不能冒充 human 确认、自动生效或写 skill。
- [ ] 按现有设计规范做视觉检查；用 production Web 和原生 Desktop 分别验证提出→人工确认→下一次输入生效→撤回后下一次不注入。

## 6. 完整质量门与版本证据

以下为计划命令，不是通过记录；最终测试名以新增文件为准，并防止 `[no tests to run]` 被当作通过：

```sh
go -C server test ./pkg/protocol ./internal/collaboration ./internal/daemon/execenv ./internal/daemon -run 'Knowledge|ManagedContribution' -count=1 -v
go -C server test ./internal/service -run 'CollaborationKnowledge' -count=1 -v
go -C server test ./internal/handler ./cmd/server -run 'Knowledge' -count=1 -v
make sqlc
go -C server vet ./...
go -C server build ./...
pnpm lint
pnpm typecheck
pnpm test
```

- [ ] DB 命令带任务专用 DATABASE_URL，service 与 handler 全套按计划串行；先确认测试真的运行、零意外 skip。
- [ ] changed tests 通过后再扩大到相关 collaboration、project/workspace、claim/resume 的既有套件；只有新变更/新失败才重复已通过检查。
- [ ] 每次保存 HEAD、dirty diff/hash、命令、平台/DB、测试名与退出码。sqlc 前后记录生成变化并确认无非预期 drift。
- [ ] 独立 check 对照 PRD/设计审查读写权限、lineage/幂等、不可变内容、预算、早期拒绝、实际 stdin、resume、旧客户端，以及删除/回退。
- [ ] 更新相关 server/core/views specs、问题→修复→证据映射；符合 Lore 的分批提交，只带任务所有文件。
- [ ] 任何真实 Provider 观察使用独立、已授权、串行的版本化运行，不消费或改写首轮 E2a–E6 的原槽位。fake 通过不证明模型理解。

## 7. 可标完成的条件

必须逐项有证据：未批准不改变输入；人类权限矩阵成立；批准精确版本并保留来源；source/target/audience 不扩权；纠错/expiry/withdraw 影响下一次实际输入；无授权回退/旧 session 绕过；所有第一方 claim 表面已接线；Web/Desktop 真实入口完成；历史及 rollback 保留。

缺任意一项则知识子任务保持 in_progress 并精确写缺口。runtime 原任务、首轮真实实验、Windows/安装包/多账号额度矩阵的验收仍各归其原任务，不用知识单元测试代替。

## Caveats / Not Found

- 本计划没有执行任何上述命令；只读研究依据见 preflight 和设计草案。
- 候选版本实体、target revision、exact-session exposure、独立 preparation/disclosure 与普通 knowledge authorize 都是待实现内容，不是现有 runtime WIP 自带能力。
- 主会话应将普通 Provider lifecycle contract、最终 source/target 锁序和撤回双侧角色作为技术复核重点；这些不是要求用户重新确认已经决定的 source 不扩权政策。
- External references: 无新外部技术选择；实现沿用仓库 Go/SQL/React Query/strict schema 与 fake-executable 测试模式。
