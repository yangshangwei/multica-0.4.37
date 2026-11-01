# 知识交付审计缺陷修复：设计

缺陷编号、位置和验收标准见 `prd.md`。基线提交为 `6aed33aad`；本任务的首个提交为 `4e3b61500`。

## 实施分道

| 分道 | worktree / 分支 | 缺陷 | 主要文件 |
| --- | --- | --- | --- |
| claim | `mc-kfix-lane-claim` / `codex/kfix-lane-claim-20261101` | H1、M1、H3、M3 | `service/task_knowledge.go`（claim、exposure、runtime exposure）、`handler/daemon.go`（claim 错误映射）、`queries/task_knowledge.sql`、migration 761+ |
| launch | `mc-kfix-lane-launch` / `codex/kfix-lane-launch-20261101` | H2、M2、L6、L7 | `internal/daemon/*`、`pkg/agent/*`、`pkg/protocol/task_knowledge.go`、`service/task_knowledge_prepare.go`、`service/task_knowledge_observe.go` |
| write | `mc-kfix-lane-write` / `codex/kfix-lane-write-20261101` | L1–L5 | `service/collaboration_knowledge*.go`、`internal/realtime/*`、PAT 相关查询 |
| web | `mc-kfix-lane-web` / `codex/kfix-lane-web-20261101` | M4、M5、L8–L13 | `packages/core/collaboration/knowledge-*`、`packages/core/realtime/knowledge.ts`、`packages/views/collaboration/knowledge-*`、locales |

- 各分道只修改自己负责的文件。如果必须改动其他分道负责的函数，改动要尽量小，并写进分道报告。
- `service/task.go` 的 `retryableReasons` 可能同时被 M1 和 M2 改动，冲突在合并时解决。
- 执行 spec 由 claim 和 launch 两个分道共同修改，两者改的是不同段落。

## 写入与实时（write 分道，L1–L5）

### L1 审批撞上过期

- `changeKnowledgeCandidate` 在锁定并重新读取候选行之后，用 Go 时钟判断过期，而 `ApproveKnowledgeCandidate` 的 SQL 用 `clock_timestamp()`。
- 候选行已经 `FOR UPDATE NOWAIT` 锁住，状态、revision、版本和 hash 也都对照锁住的行检查过。因此 UPDATE 不命中任何行时，唯一可能的原因是过期。
- 要求：
  - 在任何写操作（`AdvanceKnowledgeTarget`、`SupersedeKnowledgeCandidate`）之前，先按数据库时钟判断过期。已过期时返回已封存的 `knowledge_expired` 回执（409）。
  - 如果预检之后、UPDATE 之前恰好过期，返回可重试的冲突，让外层事务回滚并重试。重试时会走到预检，得到 409。不得出现 404，也不得在部分写入之后提交拒绝回执。
- 测试：
  - 需要让应用时钟与数据库时钟错开，才能稳定复现。没有现成接缝时，先单独加一个最小的时钟接缝，证明"加了接缝、未修复"时测试失败，然后再修复。
  - 断言审批得到 409 `knowledge_expired`、回执已封存、没有部分写入。

### L2 提议撞上过期 CHECK

- `createKnowledgeCandidate` 用应用时间校验 `expires_at`，INSERT 时数据库用 `clock_timestamp()` 填 `proposed_at`。违反 `CHECK (expires_at > proposed_at)` 时，23514 落到 `503 knowledge_unavailable`。
- 要求：
  - 只把这一条 CHECK 的 23514 映射为 `400 invalid_knowledge_request`，依据 `pgconn.PgError` 的 constraint 名或等价的可靠判别。
  - 其他 23514 属于服务端缺陷，保持 5xx。
  - 可以额外用数据库时间预检来缩小窗口，但不能替代上面的映射。
- 测试：用时钟接缝或直接构造边界值，断言返回 400。

### L3 knowledge 帧鉴权在写 deadline 之后

- `writePump` 先 `SetWriteDeadline(10s)`，然后才执行 `authorized()`、`outboundScopeCurrent()`、`outboundKnowledgeFrame()`。最后一个函数里有两次不走缓存的数据库查询，各有 5s 超时。
- 要求：
  - 所有鉴权都在设置 deadline 之前完成。每一次 `WriteMessage` 紧挨着设置自己的 deadline，包括 `outboundScopeCurrent` 里直接写 `collaboration:access_changed` 的那一处。
  - spec 要求 knowledge 帧每次都做不走缓存的凭据校验，这一点保持不变。
  - 整个工作区所有连接同时查询的放大效应受房间规模约束，在 spec 中注明即可，本次不改。
- 测试：
  - 用一个耗时超过写超时的鉴权桩，断言帧仍然送达、连接不断开。
  - 现有的 realtime 测试必须继续通过。

### L4 hint 队首阻塞

- `deliverNext` 的 defer 只在 `ctx.Err() == nil` 时记录这次尝试。超过 5s 才超时的 hint 不更新 `last_attempted_at`，下一轮仍然排在第一。
- `DeliverPending` 遇到第一个错误就返回。
- 要求：
  - 单条 hint 超时、但父 ctx 仍然存活时，用一个新的、有上限的 ctx 记录这次尝试。
  - `DeliverPending` 跳过失败的 hint，继续处理后面的 hint，把错误合并后返回。设置连续失败上限（例如连续 3 次），避免 Redis 宕机时一个 tick 卡住几百秒。
  - 把单条 hint 的超时改成结构体字段，默认 5s，测试里可以改短。
- 测试：
  - 发布桩在 hint A 上阻塞到超时。断言同一次（或下一次）`DeliverPending` 能送达 hint B，并且 A 的 `last_attempted_at` 已更新。
  - 重复投递仍按 event_id 去重。

### L5 PAT 锁与 last_used 更新冲突

- `LockKnowledgePAT ... FOR SHARE NOWAIT` 与 middleware 异步执行的 `UpdatePersonalAccessTokenLastUsed` 冲突。当 UPDATE 先拿到行锁时，NOWAIT 立即得到 55P03，重试两次后返回 503。
- 要求：
  - 这把锁仍然必须和 PAT revoke 串行。修复前先确认 revoke 是怎么取锁的：如果 revoke 用的是 `FOR UPDATE`，那么改成 `FOR KEY SHARE` 就不会阻止 revoke 生效，这条路不行。
  - 可选方案：给 PAT 锁设置短的 `lock_timeout`，让它在 `deadlock_timeout` 之内短暂等待；或其他能证明与 revoke 串行的方案。实现时说明理由。
- 测试：
  - 在另一个事务里持有 `last_used_at` 的行锁，同时发起知识写入。断言写入在锁释放后成功，不返回 503。
  - revoke 与知识写入并发时，仍然串行。

## 前端小项（web 分道，L10、L11、L13）

### L10 第 2 页租约到期

- spec 规定后续页沿用第 1 页的授权期限，不续期，这一点不改。
- 要求：
  - 后续页因父 boundary 失效得到 AbortError 时，面板自动回到第 1 页，并给出可见的状态提示，例如"列表已刷新，显示最新内容"。不显示通用的加载失败。
  - 不在第 2 页上反复轮询失败。
- 测试：
  - 推进假时钟越过第 1 页的 deadline，断言回到第 1 页、显示状态提示。
  - 也可以触发一次 hint，断言同样的结果。

### L11 后端没有知识接口

- 要求：
  - 当错误是 `CollaborationUnavailableError` 时，`refetchInterval` 返回 false。窗口焦点和重连引起的重新请求，也不要让同一条 alert 反复被移除再插入。
  - 不可用提示保持稳定，不在 pending 和 alert 之间来回切换。
- 测试：
  - 断言第一次失败后，在多个 15s 周期内不再发请求。
  - 断言 alert 节点只挂载一次。

### L13 zh-Hans 术语

- 依据 `apps/docs/content/docs/developers/conventions.zh.mdx`：
  - workspace 统一写"工作空间"；
  - 来源 issue 沿用文件里已有的"协作任务"，不新造"协作单"。
- 涉及 key：`derive`、`workspaceTarget`、`sourceWorkspace`、`historyTruncated`、`origin`、`openSource`。
- 验收：
  - locale parity 测试通过。
  - grep 确认 `collaboration.json` 的 `knowledge` 段里，没有单独作为 workspace 名称出现的"空间"，也没有"协作单"。

## 前端人工输入（web 分道，M4、M5、L8）

结论：只改 views，不改 core 的 `knowledge-cache` / `knowledge-lease`。刷新时擦除正文是 core spec 要求的安全行为，`knowledge-cache.test.ts:52-53` 保持不变。

### M4：评审输入和数据门控解耦

- 输入状态仍放在 React 组件 state 里，不进 Query、Zustand，也不进 StorageAdapter。core spec 禁止保留持久副本。
- `KnowledgeReview` 计算 identity，取 `[candidate_id, state_revision, availability, 可用时的 content_sha256, 非 restricted 时的 version]`；candidate 不存在时 identity 为 `null`。
- 用 `decisionKey` state 记住上一次出现的 identity：
  - identity 不为 null，且与 `decisionKey` 不同，就在 render 阶段 `setDecisionKey(identity)`。
  - `KnowledgeDecision` 移出 `candidate &&` 分支，渲染为 `key={decisionKey}`，只在 `identity === decisionKey` 时才传入 candidate。
- 这样有三种情况：
  - 数据被擦除或正在加载：不渲染任何私有内容，也不显示操作按钮，但组件实例和其中的理由、勾选都保留。
  - 同一个 identity 重新出现：理由和勾选按原样恢复。
  - 任何 identity 字段变化：组件重新挂载，理由、勾选和错误一起重置，符合 views spec 第 88 行。
- 理由文本在 identity 变化时**不保留**：`state_revision` 变化意味着原来的 `expected_state_revision` 已经过期。
- 只有 capability 变化、identity 不变时，按钮会隐藏后再显示，但输入不重置。服务端仍会校验授权和 expected 元组。
- 显示顺序不变：正文、决策区、历史。历史块单独加 `candidate && availability !== "restricted"` 守卫。

### M5 + L12：只处理本面板自己的请求

`settled(receipt, owned)` 的规则合并了 M5 和 L12 两份设计：
- 成功且 `owned`，才执行 `setSelectedId(receipt.candidate_id)`。
- 成功、`owned`、且 `receipt.action === "propose"`，才执行 `setProposal(null)`。
- 成功但不是 `owned`：设置 `openedElsewhere`，提示"请到该候选所在的知识列表中打开"，不打开也不关闭任何东西。

`owned` 的判定：
- `submit`：`receipt.request_id === next.requestId`，成功路径和 `KnowledgeReceiptCleanupError` 路径都按这个判定。
- `recover`：在 await **之前**计算，满足任一条件即为 owned：`intent?.requestId === identity.requestId`；或者当前列表 `current?.value.items` 中包含 `identity.candidateId`。

### L8：刷新期间提交提议

- `SourceKnowledge` 给 `KnowledgeProposalForm` 传入 `refreshView = async () => (await query.refetch({ cancelRefetch: false })).data?.value`。
- 提交按钮的禁用条件，以及 onSubmit 的守卫，统一改为 `locked || building || (view !== undefined && view.capabilities.can_propose !== true)`，刷新期间不再把按钮置灰。
- 把提交过程拆成几段，每段的错误分开归类：
  1. 构建来源失败 → `sourceChanged`。
  2. 取当前 view：先用 `alive.current.view`，没有就调用 `refreshView()`，最多等 `KNOWLEDGE_REFRESH_MS`。拿不到 view、失去授权或 scope 改变 → 新 key `proposalUnavailable`（"当前无法提出知识候选，草稿已保留。"）。
  3. `prepareKnowledgeProposal`：抛出 `CollaborationUnavailableError` → `proposalUnavailable`；其余错误 → `invalid`。
  4. 调用 `onSubmit`。
- 草稿在任何错误下都保留。

### 测试（`packages/views/collaboration/knowledge-panel.test.tsx`）

- 同一 identity 刷新后，理由和勾选仍在。
- 版本或 revision 变化后，两者都重置。
- 审批另一个候选成功后，草稿保留。
- 列表刷新期间提交提议成功，不误报 `sourceChanged`。
- 恢复不属于本面板的请求时，不打开别处的候选。
- 原有的第 83 行测试保留，作为"identity 变化"的用例。

## 字节限制与恢复请求（web 分道，L9、L12）

### L9：按字节限长，错误分开归因

- core `knowledge-schema.ts` 导出常量：
  - `KNOWLEDGE_SUMMARY_MAX_BYTES = 2048`
  - `KNOWLEDGE_CONTENT_MAX_BYTES = 65_536`
  - `KNOWLEDGE_REFERENCE_MAX_BYTES = 2048`
  - `KNOWLEDGE_REASON_MAX_BYTES = 1000`
- 文件中原有的字面量改用这些常量，行为不变。
- views 新增 `KnowledgeByteLimit({ id, value, max })` 组件：用 `collaborationTextByteLength` 计算，超出时渲染 `role="alert"` 的行内错误，文案为"已超出 {{max}} 字节上限（当前 {{used}} 字节）"。
- 提议表单的 summary、content、evidence，评审表单的 reason：
  - 去掉 `maxLength`；
  - 超限时加 `aria-invalid` 和 `aria-describedby`，在输入框下方显示 `KnowledgeByteLimit`；
  - 超限时禁用提交，onSubmit 提前返回。
- 提议表单的错误归因见 L8。
- 评审表单 catch 中剩下的 `invalid`，此时只可能来自 NUL 字符、非法文本或失去授权，现在的归因是准确的。
- 来源加证据超过 32 条、元数据超出预算这两种情况，仍显示通用的 `invalid` 提示。这条提示现在归因正确，所以不再单独加文案。

### L12：恢复请求的范围和删除

- 存储里仍然只放客户端生成的 identity，不加入来源 ID。core spec 和 `knowledge-recovery.ts:17` 都禁止存来源 ID。
- 每个面板仍列出这个 server + user 下的所有未决请求，这样每一条在任何面板都找得到。列表上方加一句说明 `pendingScope`："从其他协作任务或目标范围发起的请求也会列在这里。"
- 恢复时按 M5 + L12 的 `owned` 规则处理，不在无关面板中打开别处的候选。
- core 新增 `dismissKnowledgeOperation(scope, identity, storage?)`：
  - 只删除本地的恢复指针，不发请求；
  - 不清除内存里的尝试状态，不让投影过期；
  - 删除本身不能当作"服务端未应用"的证据。
- "移除本地记录"按钮的显示规则：
  - 只在本次挂载期间对这条请求执行过一次未能结算的检查之后才显示。这包括 404 和撤销凭据之后的 403；404 本身不能说明什么，spec 第 168 行写明了这一点。
  - 面板自己正在进行的 intent 永远不显示这个按钮。
  - 旁边附说明 `dismissHint`："只会删除本设备上的恢复记录，不会取消请求。"
- zh-Hans 文案统一用"协作任务"，不用"协作单"，与 L13 一致。

### 测试

- core 测试：`dismissKnowledgeOperation` 遇到外来 scope 时拒绝，删除失败时抛出错误。
- views 测试：
  - 700 个中文字的 summary 显示字节上限错误，并禁用提交；
  - 400 个中文字的 reason 同样显示字节上限错误；
  - 恢复不属于本面板的请求时不打开该候选；
  - 一次未结算的检查之后出现删除按钮，点击后这条请求从列表中消失。

<!-- WORKFLOW_DESIGNS -->
