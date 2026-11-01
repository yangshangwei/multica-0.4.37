# 知识交付审计缺陷修复

## Goal

修复 2026-11-01 只读审计对知识交付（`765ab186a..6aed33aad`）确认的全部缺陷，同时不削弱现有的安全性质：fail closed、exposure 不重标、人工审批、撤销即时生效。

## Source

- 审计范围：`765ab186a..HEAD`（审计时 HEAD 为 `93afc3cb0`，修复基线为 `6aed33aad`，两者之间只有文档提交）。
- 每条缺陷均由主会话对照代码逐行核对。用户在会话 `cb052b51-bfb5-4d87-8a6e-aed56671c077` 中要求全部修复，并确保测试通过。

## In scope

| ID | 严重度 | 缺陷 | 位置 |
| --- | --- | --- | --- |
| H1 | 高 | 开关关闭时，`mdt_` daemon 上的 claude 任务只要 custom_args 或 `MULTICA_CLAUDE_ARGS` 含白名单外参数，就会以 `knowledge_invocation_unsupported` 终止 | `service/task_knowledge.go:350-360` |
| H2 | 高 | 上一次工作目录不可达时 daemon 清空 `PriorSessionID`，与服务端绑定的 session 不一致，之后同一 issue/chat 每一轮都以 `knowledge_context_changed` 失败 | `daemon/daemon.go:5908`、`daemon/knowledge_context.go:226` |
| H3 | 高 | 128 行历史上限把只 claim 过、空清单的行也算进去，正常增长就会导致永久拒绝 | `service/task_knowledge.go:414,476`、`queries/task_knowledge.sql` |
| M1 | 中 | claim 时知识相关的非业务错误（含数据库瞬时错误）一律记为 `knowledge_unavailable` 并终止任务 | `handler/daemon.go:3234-3241` |
| M2 | 中 | prepare/authorize 对 installation 命名空间用 try-lock，与 claim/finalize 的阻塞锁冲突，返回 `knowledge_busy` 后任务终止 | `service/task_knowledge_prepare.go:270`、`daemon/knowledge_context.go:144` |
| M3 | 中 | `ListTaskKnowledgeRuntimeHistory` 没有可用索引，默认配置下每次 claim 都扫 `agent_task_queue` | `queries/task_knowledge.sql:54-65` |
| M4 | 中 | 评审时输入的理由和"已审阅"勾选会被刷新、焦点切换、hint 擦除 | `core/collaboration/knowledge-cache.ts:31,64`、`views/collaboration/knowledge-review.tsx:34` |
| M5 | 中 | 任一决策或恢复成功都会关闭正在编辑的提议草稿 | `views/collaboration/knowledge-panel.tsx:59` |
| L1 | 低 | 审批刚好撞上过期时返回 404，应为 409 | `service/collaboration_knowledge.go:557` |
| L2 | 低 | 提议撞上过期 CHECK（23514）时返回 503，应为 400 | `service/collaboration_knowledge.go:457,511` |
| L3 | 低 | knowledge 帧的两次鉴权查询发生在写 deadline 设置之后 | `realtime/hub.go:1197`、`realtime/collaboration.go:100` |
| L4 | 低 | 超时的 hint 不记录尝试，一直占住队首，卡住其后的 hint | `service/collaboration_knowledge_invalidation.go:240-266` |
| L5 | 低 | PAT 的 `FOR SHARE NOWAIT` 与异步 `last_used_at` 更新冲突，偶发 503 | `service/collaboration_knowledge_credentials.go:30` |
| L6 | 低 | knowledge 线协议严格拒绝未知字段，新服务端加字段会使整批 claim 解析失败 | `protocol/task_knowledge.go:233` |
| L7 | 低 | Started 回调失败后不上报 Terminated，取消确认收不到进程退出 | `pkg/agent/launch_lifecycle.go:431-461` |
| L8 | 低 | 列表刷新期间提交提议会误报"来源不可用" | `views/collaboration/knowledge-proposal.tsx:56-68` |
| L9 | 低 | 输入框按字符限长，schema 按 UTF-8 字节限长；超长错误被归因为来源不可用 | `knowledge-proposal.tsx:68-87`、`knowledge-review.tsx:91-101` |
| L10 | 低 | 第 2 页租约到期后显示通用加载失败，轮询也持续失败 | `knowledge-lease.ts:31`、`knowledge-panel.tsx:101,112` |
| L11 | 低 | 后端无知识接口时持续轮询，读屏器反复朗读"不可用" | `core/collaboration/knowledge-queries.ts:20-23` |
| L12 | 低 | 未解决的请求出现在所有知识面板上，可在无关面板打开别处候选，且无法删除 | `knowledge-recovery.ts:11,31-41`、`knowledge-panel.tsx:44-47` |
| L13 | 低 | zh-Hans 用了"空间""协作单"，不符合术语规范 | `views/locales/zh-Hans/collaboration.json:760,770,771,826-828` |

## Out of scope

- 真实 Provider 验收、E2a 实验（由会话 multica-0-4-37-54 独占）；
- 打开 `MULTICA_KNOWLEDGE_CONTEXT_ENABLED`；
- 未经用户同意合入 `codex/cross-workspace-collaboration`；
- 完整的 `make check`（需要与独占真实模型窗口的会话协调时间）。

## Acceptance criteria

| ID | 可观察验收 |
| --- | --- |
| AC1 | 每个缺陷 ID 都有一个具名测试：在 `6aed33aad` 上失败，修复后通过。红绿两次运行的输出都要保留。L13 例外，用 locale parity 测试和术语核对代替。 |
| AC2 | 开关关闭时，安全参数下的普通 claim 返回 nil descriptor 并正常执行；不安全的会话选择参数按设计结论处理，绝不静默剥离。 |
| AC3 | 历史只会正常增长时，任何长度都不会造成永久拒绝；损坏的行和真实 exposure，无论处在多深的历史里，仍然拒绝。 |
| AC4 | 瞬时数据库错误和锁竞争都不会终止任务：任务重新入队或进入有界重试，并留下可观察的原因。 |
| AC5 | 工作目录不可达时启动新会话，并给出与旧链路一致的提示；新会话仍要经过授权、绑定和观测。 |
| AC6 | 人工输入（理由、勾选、草稿）在刷新、租约擦除和其他操作成功后仍然保留；评审对象的身份变化时，决策控件重置；服务端提供的私有正文仍按规定即时擦除。 |
| AC7 | 新 migration 遵守无 FK、单语句 `CREATE INDEX CONCURRENTLY`；down 文件沿用 748–760 的约定；执行 `make sqlc` 后，除预期改动外没有其他差异。 |
| AC8 | Go：受影响包的 build、vet 通过；所有 DB-backed 包各用一份独立克隆库，测试全绿；改动过的并发路径加 `-race`。TS：core、views 的 typecheck、lint 和 vitest 全绿。 |
| AC9 | spec、内置 skill 文档中凡是描述了被改动行为的地方，都在同一批提交里同步更新。 |

## Constraints

- 所有改动都在 worktree `/Volumes/artisan/code/2026/mc-knowledge-fixes-20261101` 的分支 `codex/collaboration-knowledge-fixes-20261101` 上进行，不改动共享 checkout。
- 测试只使用 `multica-postgres-1` 中从 `kfix_template_20261101`（760）克隆的数据库，每个包一份。
- 不调用真实 Provider，不使用 `agentintegration` tag。
- 先写失败的测试（TDD）。每个缺陷组一个原子提交，提交信息使用 conventional 前缀。
