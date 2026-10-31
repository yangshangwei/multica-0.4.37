# 正式合同修订：F1–F4 与 runtime 依赖冻结

- Date: 2026-11-01。
- Scope: 只修订本知识任务的 PRD、design、implement、两个 context manifest 和 task.json；保留原草案审查及 runtime 证据，不改产品代码、父/runtime 账本，不提交。
- Current disposition: **F1–F4 已落实到正式计划，等待主会话独立复核**。K1–K5 的已有设计级通过仍成立；本文不是新功能的动态验收记录。

## 1. 修订边界与证据

原问题是草案把知识准备/批准放到 workspace/project 热点行，并只要求“缓存失效”，没有给目标管理员一条可恢复的失效路径。修订落在正式设计的模型、锁序、失效与缓存合同，以及对应具名实施测试；没有改变来源读权、目标确认角色或消费受众。

本次读取 CLAUDE.md、trellis-before-dev、server/realtime/startup-handoff 规范、正式三份计划、原 `draft-code-seam-review-2026-11-01.md` 和 runtime 的 `knowledge-plan-review-2026-10-31.md`。关键源码依据：

| 现有证据 | 对合同的约束 |
| --- | --- |
| `server/pkg/db/queries/workspace.sql` 的 IncrementIssueCounter / LockWorkspaceForDelete | 建单更新 workspace 非 key 字段；删除显式 FOR UPDATE。父 KEY SHARE 足以防删除且兼容计数更新 |
| `server/pkg/db/queries/project.sql` 的 UpdateProject / LockProjectForAssociationNowait | description/revision 更新与 SHARE 冲突；普通关联用 SHARE。只有 lead 授权确需稳定时才用 project SHARE |
| `server/pkg/db/queries/collaboration_authority.sql` / `collaboration_reconciliation.sql` | 已有 workspace KEY SHARE NOWAIT 和 late-lock 全事务回退模式，不能为知识引入不合作的权限锁 |
| `server/internal/service/collaboration_memory_operation.go` | 人类来源写按 user→workspace/member→status→issue/collaboration/session，再查原操作；40001/40P01/55P03 最多三次全事务重试 |
| `server/internal/service/collaboration_memory.go` 的 purgeCollaborationMemoryVersion / recordCollaborationMemoryChange | purge 在原来源事务清正文，保留历史并追加 source event；目标失效不能在这个位置反向获取 target 锁 |
| `server/internal/service/admin_collaboration_memory.go` | admin purge 复用同一个清除函数；不能漏掉非人类入口的目标失效 |
| `server/internal/service/collaboration_realtime.go` / `server/pkg/db/queries/collaboration_realtime.sql` | 现有 outbox 依赖当前 issue_collaboration JOIN；memory_changed 只给 issue 房间，不能覆盖无 source Read 的目标管理员；publish 固定 ID，失败保留 pending |
| `server/migrations/571_collaboration_schema.up.sql` / `598_collaboration_operation_request.up.sql` | event.issue_id 非空；operation 唯一键是 scope/actor/request，没有 kind |
| `server/internal/handler/workspace_revoke.go` / `collaboration_account_reconciliation.go` | 成员及账号变更有自己的权威事务/持久 reconciliation；新增失效应在原变更事务记 intent，不等待逐单处理完成 |

## 2. F1–F4 的正式决定

| Finding | 修订 | 具名实施证据入口 |
| --- | --- | --- |
| F1 | human/preparation 的父 workspace/project 存活性锁统一 KEY SHARE NOWAIT；member role 保持 SHARE，只有 project lead 授权才用 SHARE；明确 knowledge_busy 有界重试后终态，不绕过 preparation | `TestKnowledgePrepareDoesNotContendWithIssueNumberAllocation`、`TestKnowledgePrepareConflictsWithWorkspaceDelete`、`TestKnowledgePrepareDoesNotContendWithProjectDescriptionUpdate` |
| F2 | 独立 `collaboration_knowledge_target` 行维护 revision，不更新热点 parent 计数；writer UPDATE NOWAIT、preparation SHARE NOWAIT。首次唯一键插入有真实 25ms lock_timeout，不能把 INSERT ON CONFLICT 误当 NOWAIT | `TestKnowledgeApprovalDoesNotBlockUnrelatedIssueWrites`、`TestKnowledgeTargetInitializationRaceIsBounded`、`TestKnowledgeApprovalVersusProjectDeleteUsesParentFence` |
| F3 | 专用 content-free scan/target_hint 基础设施复用现有 relay；direct 批准/撤回/纠错原子写 revision+hint；purge/delete/ACL 原事务只写 scan，worker 独立分页反查曾批准历史/去重/推进目标（pending-only 不发提示）；有限 UI 可见期与请求代次兜底 | `TestKnowledgeSourcePurgeAppendsIntentWithoutTargetLocks`、`TestKnowledgeFanoutCrashAndReplayPreserveCursorAndRevision`、`TestKnowledgeAuthorityChangesFanOutWithoutCurrentMembershipJoin`、`TestKnowledgeTargetHintRelayFailureRetainsStableID`，以及 core 清除/到期/晚响应组 |
| F4 | 客户端每个新意图用新 UUID，不确定结果保持原 UUID/payload；保留全协作 kind 共用 scope/actor/request 唯一键；跨 kind 冲突 409，recovery 严格验证原 kind | `TestKnowledgeRequestIDConflictsAcrossOperationKinds`、`TestKnowledgeRecoveryRejectsOtherOperationKind` |

F3 没有把 source event 的私有 payload 广播到 workspace。外发 `collaboration:knowledge_changed` 精确只有 target_kind、target_id、knowledge_revision；旧 workbench_changed 仍只接受空 payload。source/candidate/cause/actor ID、正文、摘要、hash/refs 均不外发；最终发送仍核对目标范围可见性。

采用两个专用基础设施表（target revision、invalidation），仍只有一个候选业务实体。单靠原 event.outbox_state 无法同时表达 source 投递完成、可分页目标扫描及每个目标重试，也无法在 source 已删除时依靠当前 JOIN 恢复。没有另建通用消息平台、审批引擎、正文副本或第三方依赖。

## 3. 锁、删除及隐私清除的具体界线

- human/preparation：parent → independent target revision → source → candidate → late runtime/task NOWAIT；现有 source 授权读写线性化保留。
- scan：own scan job → parent KEY SHARE NOWAIT → target revision UPDATE NOWAIT；每事务一条候选/一个目标，无 source/candidate/task 行锁。目标 revision、唯一 cause/target hint 与游标一起提交；失败保留原游标并调度其他工作。
- delivery：own hint → 原 relay，固定 hint ID；发布失败或未知 commit 不提前标 delivered，允许同 ID 重试。
- purge：既有 source 行事务 + 一个新 scan INSERT；**不增加 target/candidate/revision 等待**。它仍可能等待已经获权的 source 行事务，这来自原授权线性化，不能为了通知而增加审批目标锁/异步 fanout 等待。
- source 历史、路由 tombstone、candidate 和回执保留；source JOIN 不存在仍能扫描。target delete 显式移除计数行，保留待投递 hint，旧 scan 不能复活目标。既有禁止删除 collaboration records 的规则不放宽。
- 当前服务端 body 读取/实际消费立即检查 source availability/ACL，不等 worker。客户端收到提示先清正文再读；丢提示/离线时已授权投影最多可见 30 秒、可见页面至多 15 秒重读，并受更早 expiry 约束。晚响应、时钟偏差、分页或回前台不得续期，失败不恢复旧 body。

最后一条是对已合法披露文本的有限缓存承诺，不是声称能在数据库提交瞬间抹除远端文字。K4 的 purge/delete 抑制仍覆盖所有 body/summary/派生引文；普通 withdraw/expiry 仍允许当前有权且来源可用的历史阅读。

## 4. K1–K5 和依赖状态

K1 的 exact `(provider,runtime,native_session,scope,generation)` 累计 exposure、cancelled/failed/NULL turn、原子 pointer 绑定和预算不截断保持；本次把既有复核 K1-D 的四个测试名及“省略版本的旧客户端在 PriorSessionID 之前过滤”补入正式 plan。K2 独立准备披露与每次重放重授权、K3 启动拒绝不自动重放、K4 来源清除、K5 candidate+request 恢复均未降级。外部 current handler 的 source target 提案、human confirmation、source ACL 不扩张、实际 originator/runtime owner/audience 检查保持。

G0/G1 的冻结依据是产品 `0e1ee8e1e957b46a5bbd5d91a84772d69dd4d64b` 与证据 `2b861347e62573ca765157ebf14629712793e7df` 中的 `../10-30-collaboration-context-handoff-runtime/verification-integration-2026-11-01.md`。报告记录 1,380 个顶层 Go 测试、2,573 个子用例、12,982 个 TS 测试、六个生产 Web/原生 macOS Electron 用例及 sqlc/static/源码 hash 证据。本次读取该提交中的报告，没有重跑、另算或扩大这些结果。

这只冻结来源/版本/清除与 handoff/实际应用输入合同。真实 runtime 模型行为、更广平台矩阵仍开放；当前实验窗口以 upstream 502 环境结果结束，不由本任务重开或改写。G2 是知识消费/adapter/session/pointer/授权代码完成后、启用前必须通过的实现门，不能作为开始实施的循环阻塞。G3 的 owner/迁移编号/隔离环境在主会话复核后分配。

## 5. 本次验证与剩余工作

- 校验 JSON/JSONL 可解析、context 路径存在且不重复，检查正式文档没有残留 parent knowledge_revision/parent SHARE 或“runtime 全部模型通过”断言；对本次所有文档执行空白差异检查。
- 未运行产品 lint/typecheck/Go/TS/DB/Provider；本次没有产品改动，具名回归全部是待实现/运行的验收要求。
- 剩余：主会话独立复核本次 F1–F4；分配 G3 后可实施。G2 和产品完整验收仍有实际代码/测试工作，不把 planning 状态改成 implemented/completed。
