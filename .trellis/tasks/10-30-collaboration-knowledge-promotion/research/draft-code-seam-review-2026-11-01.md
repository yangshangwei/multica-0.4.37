# 知识草案代码接缝补充审查 — 2026-11-01

- 审查对象：`../10-30-collaboration-context-handoff-runtime/research/knowledge-design-draft-2026-10-31.md`（SHA-256 `beb30149…f6b5`，2026-11-01 00:11 版）及同目录实施草案（`e0a07fa6…73e1`）。
- 范围：草案的数据模型、§5 写事务锁序、§7.2 preparation 锁序、§4 回执、§8 缓存失效。K1–K5 及其复核（`knowledge-plan-review-2026-10-31.md`）已覆盖的 resume、claim 披露、启动拒绝、purge 投影、目标侧恢复不再重复。
- 方法：只读核对主工作区 `e3620721e` 上的 SQL、迁移和 handler/service 源码。未改源码、草案或任务状态；未运行测试、数据库或 Provider。
- 结论：**F1、F3 需在提升为正式合同前修订；F2 建议修订；F4 是实施注意事项**。都属于工程合同，不涉及已确认的权限政策，不需要再问用户。

## F1 — preparation 对 workspace 取 `FOR SHARE NOWAIT` 会和建 issue 争锁（P1，可用性）

草案 §7.2 第 2 步（设计草案第 218 行）要求每次 preparation 对 source/target workspace 取 `FOR SHARE NOWAIT`。§7.1 又规定，只要 backend 支持，就要 preparation，不看是否真有私有候选。所以这把锁会出现在每个支持该能力的普通任务上。

现有代码里，每次建 issue 都先执行 `AllocateIssueNumber`（`server/internal/service/issue_limit.go:82-88`），也就是 `UPDATE workspace SET issue_counter = issue_counter + 1`（`server/pkg/db/queries/workspace.sql:60-62`），并在整个建单事务里一直持有 workspace 行的 `FOR NO KEY UPDATE` 锁。PostgreSQL 中 `FOR SHARE` 与 `FOR NO KEY UPDATE` 冲突，所以只要同一空间有建单事务没提交，preparation 就拿到 55P03。autopilot 或批量建单时，大量普通任务的 preparation 会反复遇到 55P03，按草案在重试用尽后返回 503 `knowledge_busy`。草案没有定义 daemon 收到这个 503 后怎么办，结果是普通任务被延迟或失败，而这些任务多数根本没有私有候选。

草案给的理由是“与 workspace 删除的 UPDATE 冲突”。但删除取的是 `FOR UPDATE`（`workspace.sql:80` `LockWorkspaceForDelete`），`FOR KEY SHARE` 已经足以和它冲突，而且不与 issue_counter 更新冲突。仓库在同类协作路径里已经这么用：`collaboration_authority.sql:55` 和 `collaboration_reconciliation.sql:84` 都是 `FOR KEY SHARE NOWAIT`。

**最小修正：** preparation 和 §5 写事务对 workspace 行一律取 `FOR KEY SHARE NOWAIT`。角色仍按草案通过 `LockActiveMember`（`FOR SHARE` 的 member 行）保护，不依赖 workspace 行锁。preparation 的正确性来自对已知 candidate 行的共享锁（草案第 219 行）：已知贡献的撤回会被它阻断；锁后才批准的新贡献没进本次集合，相当于在批准之前线性化，可以接受。因此不需要用 workspace 行锁稳定修订号。如果实现选择在 manifest 里写入稳定的目标修订号，就要配合 F2，否则又得对 workspace 取 `FOR SHARE`。

**测试：** 打开一个已执行 `AllocateIssueNumber` 但未提交的建单事务，同时运行 preparation，必须成功；打开 `LockWorkspaceForDelete` 的事务时，preparation 必须 55P03 回退。

## F2 — `knowledge_revision` 放在 workspace/project 行上会与热点写路径争锁（P3，可用性）

草案 §2（第 41 行）在 `workspace` 和 `project` 上各加 `knowledge_revision`，approve/withdraw/supersede 时 UPDATE 递增。递增需要该行的 `FOR NO KEY UPDATE`，会与这些现有锁冲突：

| 被锁的行 | 现有持锁方 | 冲突结果 |
| --- | --- | --- |
| workspace | `AllocateIssueNumber` 的 UPDATE（每次建单） | 建单进行时 approve 的 NOWAIT 拿到 55P03；approve 未提交时建单的 UPDATE 阻塞等待 |
| workspace | `iteration.sql:126`、`iteration_history.sql:33` 的 `FOR SHARE NOWAIT` | approve 未提交时，迭代写入 55P03 |
| project | `LockProjectForAssociationNowait`（`project.sql:85-86`，`FOR SHARE NOWAIT`），调用方 `handler/issue.go:3858`、`handler/triage_actions.go:49/742`、`iteration/history_capture.go:151` | approve 未提交时，带 project 的 issue 写入、triage 和迭代历史 55P03 |
| project | preparation 的 `FOR SHARE NOWAIT`（草案第 219 行） | 普通项目编辑（`project.sql:57-58` 递增 `revision`）进行时，preparation 55P03 |

approve 很少发生，且现有调用方都有整事务重试：issue 更新经 `service.RetryProjectAssociationTransaction`（`service/issue.go:249`，最多 4 次，退避合计约 60ms），triage 有 `triage.go:108`，迭代历史有 `iteration/transaction.go:75`。所以大多数冲突会被吸收。剩下的风险是 approve 事务（多把锁加 candidate、event、outbox 写入）持锁超过重试窗口时，issue 写入会因无关的知识审批而失败；把计数放在业务热点行上，也会让以后每条新写路径都得考虑它。

**最小修正：** 把修订号移到专用计数行，例如 `collaboration_knowledge_target(target_kind, target_id, workspace_id, knowledge_revision)`。它是目标集合的版本计数，不是新的业务实体，也不是通用知识库。

- approve/withdraw/supersede：父 workspace/project 取 `FOR KEY SHARE NOWAIT`（挡住删除），project 批准需读 lead 时取 `FOR SHARE NOWAIT`（与 issue 关联写兼容）；首次使用时 `INSERT … ON CONFLICT DO NOTHING`，然后对计数行取 `FOR UPDATE NOWAIT` 并递增。
- preparation：父行只取 `FOR KEY SHARE NOWAIT`，计数行取 `FOR SHARE NOWAIT`。这样只会和正在进行的知识审批冲突。
- project/workspace 删除路径显式删除计数行（无外键，符合 CLAUDE.md 迁移规则）；计数行的唯一索引单独一个 `CREATE UNIQUE INDEX CONCURRENTLY` 迁移。

备选方案是用 `pg_advisory_xact_lock(target)` 串行化审批，再从 candidate 行推导修订号。它不加表，但 withdraw/supersede 也要递增，推导会变复杂，不推荐。

**测试：** approve 未提交时，带 project 的 issue 更新、triage 动作和建单都应成功；project 删除与 approve 两种顺序都能正确回退。

## F3 — 目标侧页面没有失效通道（P2，正确性/可见性）

草案 §8 只定义了目标列表的 Query key，§6.1（第 199 行）要求 source ACL/purge 变化时清除缓存，但没有说明目标页面怎样得知变化。

现有通道无法覆盖：`collaboration_event.issue_id` 是 NOT NULL（`migrations/571_collaboration_schema.up.sql:138` 起），事件只投递给 source issue 范围；source workspace 只收到空的 `collaboration:workbench_changed`（`.trellis/spec/server/collaboration-realtime.md:40-44`）。没有 source Read 的目标项目负责人或 workspace 管理员，在另一个协作参与者撤回、source purge 或 source 权限变化后收不到任何提示，项目/空间知识页面会继续显示旧的生效集合，直到重新聚焦或刷新。注入时 resolver 会重新检查，所以这不是注入泄漏；但它违背 §6.1 “不让过期 cache 恢复隐藏正文”的要求，并且会让管理员基于过期状态操作。

**最小修正：** 生效集合或其可用性变化时，在同一事务里写一条无内容的目标提示，按 workbench_changed 的方式经 outbox 投递到目标 workspace 房间。payload 只含 `target_kind`、`target_id` 和新的 `knowledge_revision`，不含 candidate/source ID。source purge、删除和来源权限变化需通过 `source_issue_id` 索引反查受影响的目标并同样发提示；这一步依赖 runtime 的 purge 语义（G0）。核心层收到提示后让目标 Query 失效并重新拉取；不以提示内容构造数据。

**测试：** 服务层断言撤回、supersede、source purge 各产生一条目标提示且 payload 无私有字段；core 测试断言提示使目标 key 失效、非空 payload 被拒绝。

## F4 — 回执幂等键与其他协作操作共用命名空间（P3，实施注意）

`collaboration_operation` 的唯一索引是 `(scope_kind, scope_id, actor_user_id, request_id)`（`migrations/598_collaboration_operation_request.up.sql:1`），`scope_kind` 只允许 `create|issue`（`571_collaboration_schema.up.sql:125`），不含 `kind`。知识操作按草案挂在 source issue 上，就与同一人在同一单上的 memory/command 操作共用 request_id 空间。客户端若复用了其他操作的 request_id，会得到 `idempotency_conflict`，而不是新操作。

**处理：** 不需要改表。合同中写明客户端每个意图都用新 UUID，跨 kind 的同 key 返回 `idempotency_conflict`；恢复接口按草案核对 `kind`。补一个跨 kind 同 key 的 handler 测试。

## 核对后无问题的草案前提

- `LockCollaborationUsers`（`pkg/db/generated/collaboration.sql.go:1073`）、`Principal.Human()`（`internal/collaboration/access.go:23`）、`LockActiveMember`（`FOR SHARE`）、`LockSubscriberWrites`（advisory xact lock）、`LockProjectForAssociationNowait` 都存在，语义与草案引用一致。
- `OriginatorUserID` 与 `AccountableUserID` 的区分与 `internal/attribution/attribution.go:153-168` 一致。
- 40001/40P01/55P03 有多处各自的重试判断（`project_write_fence.go:66`、`iteration/transaction.go:75`、`collaboration_account_reconciliation.go:129` 等），没有统一 helper。知识服务应复用其中一种模式，不再新造。
- 迁移编号：主工作区和集成 worktree 最大都是 746，runtime WIP 只新增 queries/generated，未新增迁移。知识迁移编号在 runtime 发布后再确定（G3）。
- `knowledge_revision`、`knowledge_candidate` 在代码中都还不存在，没有重名。

## 交接

F1–F3 应由草案所有者（Codex 协调会话 `01a1920d-…` 的 `knowledge_preflight`）写入设计与实施草案，再由主会话复核后提升。本文件不修改那两份草案，避免与正在进行的修订冲突。

## 合入后抽查 — 2026-11-01 10:40–10:58

由另一会话独立执行，只是抽查，不替代 `verification-integration-2026-11-01.md` 的正式验收。源码为 `629f0a079`（合入提交 `8cb04e329` 之后只有文档改动和下面这条测试修正）；macOS arm64，Go 1.27.0，Node 22.23.2。

- 代码落点：F1/F2 见 `server/pkg/db/queries/collaboration_knowledge.sql:6/9/12`（父行 `FOR KEY SHARE NOWAIT`，lead 读 `FOR SHARE NOWAIT`）和 `:38/41`（独立的 `collaboration_knowledge_target` 计数行）；F3 见 `protocol.EventCollaborationKnowledgeChanged` 与 `packages/core/realtime/knowledge.ts`；F4 见 `TestKnowledgeRequestIDConflictsAcrossOperationKinds`。
- DB 测试：从 `mem_template` 克隆新库，迁移到 `760_collaboration_knowledge_hint_pending`，`-p 1 -v` 串行运行。handler 的 `TestKnowledgePreparationWhileRealOrdinaryWriterIsUncommitted`（真实 `IssueService.Create` 已分配编号且停在提交前）和 `TestKnowledgeHeldOwnerApprovalAllowsActualUnrelatedWriters`，service 的 `TestKnowledgeRequestIDConflictsAcrossOperationKinds`、`TestKnowledgeFanoutNeverWaitsOnSourceWriter`、`TestKnowledgeSourceDeletionRetainsInvalidationRouting`、`TestKnowledgeTargetHintHasOnlyPublicTargetFields`，realtime 的 `TestKnowledgeQueuedTargetHintReauthorizesAtPhysicalSend`：7 个顶层、10 个子测试通过，0 跳过，exit 0。两条 handler 测试在 `-race` 下复跑也通过。克隆库已删除。
- 无 DB 测试：`go test ./pkg/protocol -run Knowledge` 15 个通过；core 的 knowledge 套件 8 个文件、60 个测试通过。
- 偶发失败：`packages/views/collaboration/knowledge-panel.test.tsx:125` 六次运行里失败一次。原因是下拉框点开后同步查询 option，当时弹层还没挂载。`629f0a079` 改为 `findByRole`；改后串行 3 次、并行 4 次都是 17/17 通过。原问题无法按需复现，所以这只说明改后没再出现，不能证明竞争已经消除。
- 本次没有运行全量 Go/前端、Web/Desktop e2e 或任何真实 Provider。
