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

## 启动链路：claim 分道（H1、M1、H3、M3）

方案比较全文见 `research/designs/claim-gate.json`、`research/designs/history-bound.json`。

### H1：claim 时的拒绝规则

- 根因：`96cedf485` 把用来证明"知识启动完全受控"的白名单 `ValidateKnowledgeSessionArgs` 当成了 claim 时的拒绝规则，而且在开关和支持判断之前执行。"无法证明受控"因此变成了"终止任务"。
- 结论（H1-A）：claim 时改用会话选择参数的黑名单 `KnowledgeSessionSelectorPresent`，无论开关是否打开。规则只作用于 claude runtime 上的 v1 invocation，`inv.ExtraArgs` 和 agent 的 custom_args 都检查：
  - 含选择参数，或 custom_args 解不出 `[]string`：启动前以 `knowledge_invocation_unsupported` 拒绝，绝不剥离参数。
  - 开关关闭、无选择参数：按不支持处理，交给原有的历史守卫。命名空间里有保留的协商行或 exposure 就拒绝；历史损坏返回 `knowledge_context_changed`；否则返回 nil descriptor，按普通任务执行。H1 修的就是这一格。
  - 开关打开、无选择参数、参数都在白名单内、没有 `--no-session-persistence`：照常协商。
  - 开关打开、无选择参数，但有白名单外参数或 `--no-session-persistence`：按不支持处理，走同一个历史守卫。新任务按普通任务执行，不协商；保留了协商记录的任务拒绝。
  - `inv == nil`（旧 daemon）或非 claude runtime：不变。
- 叶子层 `KnowledgeInvocationFingerprint` 在 `opts.Knowledge` 存在时仍拒绝白名单外的参数，所以获准的知识启动仍然完全受控。
- 放弃的方案：
  - 只在开关关闭时跳过白名单：开关一打开，带 `--allowedTools` 之类参数的 claude 任务全部终止。
  - 删掉这一步、只靠历史守卫：干净命名空间里 `--continue`、`--resume <id>` 会作为普通任务执行，可能接上 exposure 行已被删除的旧会话记录。
  - 扩充白名单：永远补不全，新的厂商参数照样终止任务。
- H1-followup（`6f7a601ce`）按三份复核扩大了黑名单。下面这些写法在 H1 之前都被白名单拒绝，现在恢复拒绝：
  - 远程、附着、后台会话：`--teleport`、`--cloud`、`--remote*`、`--attach*`、`--bg`/`--background`、`--desktop`、`--environment`、`--tmux`；
  - Claude Code 2.1.316 的子命令名和别名，以及旧版本的 `remote-control`；
  - 单独的 `--`，它会把 daemon 追加在最后的 `--settings <managed path>` 变成位置参数。
- 只 claim 过、没有已提交清单的会话没有披露过私有内容。spec 把"已知的托管会话"定义为历史中有一次已提交、清单非空的 preparation。命名空间守卫证明没有 exposure 时，不支持的客户端可以恢复只 claim 过的会话，由 `TestKnowledgeClaimedOnlyManagedSessionResumesForUnsupportedInvocation` 固定。

### M1：claim 时的非业务错误

- 根因：claim handler 把所有非 `CollaborationError` 的错误都当作知识领域的拒绝。数据库瞬时错误、连接错误、ctx 超时都会终止任务，而且不重试。
- 结论（M1-A）：在 handler 中分流：
  - `*service.CollaborationError`：按它的 code 终止；
  - `pgx.ErrNoRows`（runtime、agent、chat、issue、autopilot 的行在 claim 中途消失）：终止，沿用现有 reason；
  - `failClaimedTaskBeforeLaunch` 内的结算失败：保留现有的重新入队兜底；
  - 其他错误：立即重新入队，返回 500，不写 failure_reason。
- 放弃的方案：
  - 留给过期回收：chat 会一直显示"启动中"，直到超时。
  - 服务端返回带类型的瞬时错误：要审计四个函数的全部返回路径，改动过大。
  - 把 `knowledge_unavailable` 加进 `retryableReasons`：违反 spec 第 110 行，会重试真正的 fail-closed 拒绝。
- 测试：`TestKnowledgeClaimTransientHistoryErrorRequeuesTask`、`TestKnowledgeClaimErrorDisposition`。handler 的故障注入测试会替换共享的 `testHandler` 状态，不能与其他测试并行。
- 相邻问题，实现时核实：`FinalizeTaskClaim` 的错误分支（`handler/daemon.go` 约 3584–3594 行）总是重新入队，包括 32 次 claim 上限触发的 `knowledge_context_changed` 这类业务拒绝。如果会无限重新入队，就在 M1 中一并修复并加测试。

### H3 + M3：历史读取的上限与索引

- 根因：
  - H3：两处历史读取对一个只增不减的集合加了固定的 `LIMIT 129`，读满就当作"历史不完整"而拒绝。每次受支持的 claim 都写一行 `claimed`，历史从不清理，读取范围也比单个会话宽：精确会话读取覆盖 agent 在该 issue、chat 或 scope 下的全部协商行；runtime 读取覆盖整个 runtime、agent、daemon/provider，以及跨工作空间的 owner/provider 命名空间。
  - M3：runtime 读取的谓词用 OR 连接一个相关 IN 子查询和一个没有索引的 jsonb 包含条件，没有可用的索引路径；两处读取也都没有只覆盖协商行的部分索引。
- 结论（方案 a）：不设行数上限，完整读取：
  - 精确会话读取用 keyset 分页，每页 128 行，循环体不变，原有按顺序提前返回的逻辑保持不变。
  - runtime 读取先用一条 UNION 取出全部 key，再按 id 分批取行，每批 128 行，遇到第一个 exposure 就停。key 和行分开读，是因为 GIN 分支不能按游标顺序返回行；按页做 keyset 会让每一页重跑 bitmap 和 jsonb 复查，总成本 O(M²)。
  - 某个 key 在批量取行时已经消失：按 fail-closed 拒绝。
- migration：
  - 761：`CREATE INDEX CONCURRENTLY IF NOT EXISTS collaboration_knowledge_agent_history_idx ON agent_task_queue (agent_id, created_at DESC, id DESC) WHERE knowledge_context_version IS NOT NULL`
  - 762：`collaboration_knowledge_runtime_history_idx`，列为 `(runtime_id, created_at DESC, id DESC)`，条件相同。
  - 763：`collaboration_knowledge_delivery_history_idx`，`USING gin (knowledge_delivery jsonb_path_ops)`，条件相同。
  - down 文件沿用 748–760：先 `SELECT collaboration_assert_knowledge_empty();`，再 `DROP INDEX IF EXISTS`。三个索引名都要在 `cmd/migrate/main.go` 的 `concurrentIndexCleanups` 中登记。
- 放弃的方案：
  - 只读可能带 exposure 的行：恢复的 chat 每一轮都引用同一个 native ID，集合仍然每轮多一行；被改坏、恰好丢了标记的行会漏判。
  - 维护派生标记列：标记可能和文档不一致，而带外写入正是"任何位置的损坏都要拒绝"要防的情况。
  - 超过 128 行就换新会话：上限是按范围计的，之后每一轮都会被迫换新会话；runtime 路径上也没有可以换的会话。
  - 一条不分页的语句：sqlc `:many` 会把所有行读进内存，每行最多 64 KiB，而且不能在第一个 exposure 处停下。
- 测试：`TestKnowledgeResumeReadsCompleteLongSessionHistory`、`TestKnowledgeResumeDeepExposureStillGoverns`、`TestKnowledgeResumeCorruptRowAnywhereRefuses`、`TestKnowledgeRuntimeHistoryReadsCompleteNamespace`（替换只断言缺陷的 truncated 子测试）、`TestKnowledgeRuntimeHistoryIsIndexDriven`。查询计划在迁移后的克隆库上用 EXPLAIN 核对，保存为 `research/evidence/M3-plan.txt`。
- 留给负责人确认：
  - 很长的 chat 下，每次 claim 的 O(N) 成本是否可以接受。如果上千轮是常态，再另立"按范围维护 exposure 汇总"的后续任务，并单独做 fail-closed 评审。
  - owner 命名空间是跨工作空间的。一个 owner 只要在任一处有过私有披露，他在所有工作空间的不支持 claim 都会被永久拒绝。spec 要求如此，本次保持不变。

## 启动链路：launch 分道（H2、M2、L6、L7）

方案比较全文见 `research/designs/resume-unreachable.json`、`research/designs/launch-robustness.json`。

### H2：上一次工作目录不可达

- 根因：服务端在 claim 时就把知识 descriptor 绑定到选中的 native session。daemon 要到 claim 和 StartTask 之后才知道这个会话能不能接上，接不上时可达性检查会把这一轮改成新会话。旧链路接受这种情况，知识链路却无法告知服务端，结果有三处精确相等检查把它当成过期的 claim 拒绝：daemon 的 `validateKnowledgeClaim`、服务端 prepare/authorize 的 `req.Session == descriptor`、pkg/agent 的 Resume 意图检查。被拒的尝试不发布新的 session_id，也不让旧会话退役，所以下一次 claim 还会选中同一个接不上的会话，同一 issue 或 chat 之后的每一轮都以同样的方式失败。
- 结论（方案 a）：`TaskKnowledgePrepareRequest` 增加可选字段 `resume_unavailable`，由 `PrepareTaskKnowledge` 在负责披露的同一个事务中校验，并改绑到新的 generation：
  - 只允许在 claim 还没有 preparation、没有 attempt 时改绑，所以不会移动已记录的 exposure。用同一个 request_id 重放原请求仍然接受。
  - 旧会话的行保持不变。以后如果再次选中它，仍要经过 `BuildTaskKnowledgeClaim` 的重新校验。
  - 改绑在提交后只记一条不含内容的日志（task_id、旧 generation、新 generation），不加持久字段。持久字段会在回滚到旧服务端后破坏严格解码。
- 兼容性：
  - 不带标记的请求，字节和 receipt hash 都不变。
  - 新 daemon 遇到旧服务端返回的 400：按 `knowledge_unavailable` 阻塞，不重试、不调用 authorize、不启动进程。
  - 旧 daemon 遇到新服务端：仍在每一轮报 `knowledge_context_changed`，直到升级。spec 已写明，操作文档还缺这一句（见 `progress.md`）。
- 放弃的方案：
  - 先失败、再让服务端跳过这个会话：当前输入仍然失败；自动重试又违反"知识失败不自动换新会话、不重新入队、不重放输入"的规定。
  - claim 时上报可达的会话：daemon 不知道会领到哪个任务，只能列出本机全部会话；而且可达性取决于 claim 之后的决定。
  - 单独的改绑接口：多一条路由和一个持久的中间状态；可能出现改绑已提交、prepare 却失败的情况。
- 测试：`TestTaskKnowledgePrepareRequestCarriesResumeUnavailable`、`TestKnowledgeUnreachablePriorWorkdirRebindsFreshGeneration`、`TestKnowledgeUnreachablePriorWorkdirRefusesEchoedStaleBinding`、`TestKnowledgeUnavailableResumeRebindsFreshGeneration`、`TestKnowledgeUnavailableResumeRefusesUnsafeRebind`、`TestKnowledgeUnreachableResumeHTTPRebindsAndBreaksLoop`（issue、chat 两个子测试）。

### M2：命名空间锁竞争

- 根因：claim 和 finalize 对同一个 key 用阻塞锁，prepare/authorize 却用 try-lock，一次没拿到就当作最终结果。daemon 75ms 的重试很少能等过一个并发的 claim、finalize 或 prepare 事务，而 spec 规定 `knowledge_busy` 是终止性的。
- 结论（M2-b）：服务端对整个事务做有上限的重试，预算 1.5s，低于 daemon 的 5s `knowledgeRequestTimeout`。适用于普通的 `PrepareTaskKnowledge`、`AuthorizeTaskKnowledgeLaunch`，以及协作运行的 `PrepareRuntimeKnowledge`、`AuthorizeRuntimeKnowledge`。
  - 等待期间不持有任何锁，锁顺序不变：先 source graph，最后 namespace。
  - 重试是安全的：prepare 按 request_id/request_hash 重放，authorize 按 request_id 重放，协作 preparation ID 是确定的（`uuid.NewSHA1(runID, ...)`）。
  - 预算用完仍返回终止性的 `knowledge_busy`，符合 spec 第 155 行。
- 放弃的方案：
  - 阻塞锁加短 `lock_timeout`：prepare 会在持有 issue、daemon_token、member、chat 锁的同时等待 namespace，而 finalize 的加锁顺序相反，会形成等待环。
  - 加长 daemon 的重试：已经安装的 daemon 用不上。
  - 把 `knowledge_busy` 改为可重试：违反 spec 第 110、155 行，可能在披露之后重放输入。

### L6：线协议严格拒绝未知字段

- 根因：问题在于影响范围，严格解码本身没错。知识 descriptor 的严格解码嵌在整个 claim 信封的解码里，一个解不开的 descriptor 会让服务端已经提交的一整批任务全部作废。存储 ledger 保持严格是对的，但还没有写成版本规则。
- 结论：
  - L6(i)-1：每个任务单独解码 descriptor，仍然严格。解码失败的任务 `KnowledgeContext=nil`，在任何 prompt 和启动之前明确拒绝，不会降级成普通执行。同一批的其他任务照常执行。
  - L6(i)-3：在 spec 中写明版本规则：线协议新增字段必须同时提升 `TaskKnowledgeVersion`。
  - L6(ii)-1：存储的 ledger 仍然严格解码，并受版本约束。回滚到低于 ledger 版本的服务端时，受影响的任务返回 409 `knowledge_context_changed`，直到重新升级。
- 放弃的方案：宽松解码。会话绑定类型同时用于入站请求和存储 ledger，宽松的读取方会丢掉自己不认识的 exposure 记录，再在整行重新编码时把它们永久抹掉。
- 测试：`TestKnowledgeClaimDescriptorDecodeFailureRefusesOnlyThatTask`、`TestKnowledgeBatchClaimSurvivesOneUndecodableDescriptor`。

### L7：Started 回调失败后的 Terminated

- 设计阶段的结论：缺陷无法复现。无论 ProcessState 是否为 nil，Started 失败的两个分支都会经 `waitOwnedProcessTree` 调用 `finish()` 恰好一次，上报 `Terminated(LaunchManagedEnded, WaitObserved=true)`。daemon 的 `finishProcess(true)` 在上报终止观测之前执行。
- 要求：用测试证明 Started 失败之后 Terminated 恰好上报一次；如果测试推翻了上述推理，就带测试修复。不另加 `finish(LaunchUnknown)`：它会让 Terminated 上报两次，还会用较弱的记录覆盖真实的记录。

### M2、L6 的测试

`TestKnowledgePreparationWaitsOutSiblingNamespaceHolder`、`TestKnowledgeAuthorizationWaitsOutSiblingNamespaceHolder`、`TestKnowledgePreparationNamespaceBusyBudgetIsBounded`、`TestCollaborationKnowledgePrepareWaitsOutSiblingNamespaceHolder`，以及上面 L6 的两个测试。
