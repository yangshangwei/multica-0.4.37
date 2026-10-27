# 技术设计：协作单共享的 AI 工作上下文

> 2026-10-26，planning。需求见 [PRD](prd.md)（C01–C30、CA01–CA18）；现状依据见 `research/` 下四份调研。本文只是设计，未修改产品代码。

## 设计选择

新增一张只追加的版本表承载协作上下文，人和 run 走两套接口，共用一个服务层。版本号按单独立递增，不推进单据 `collaboration_revision`，不进入运行时能力声明。daemon 在每次领取 run 时声明支持，取得业务凭据后、组装提示词前拉取内容放进 run 的输入。实时通知、活动流、run 记录全部沿用现有通道，只做最小的排除和可选字段扩展。

不新建通用"文档"或"知识库"抽象，不复用 `mutateCollaborationWorkflow`（它要求单据 revision，会让上下文写入与标题/正文编辑互相冲突），不把内容塞进 claim 响应或准备清单（前者上限 2 MiB、后者 98,304 字节且不可变）。

## 复用与职责

| 层 | 复用 | 新增 |
| --- | --- | --- |
| 存储 | 无外键、CHECK、并发建索引单文件迁移、拒绝回滚的 down 文件 | `collaboration_memory_version` 表；`collaboration_run` 两个可空列 |
| 人的写入 | `mutateCollaborationContent` 的幂等账本（`collaboration_operation`）、锁内重新解析权限、序列化冲突重试 | memory 读写、恢复、清除服务方法与路由 |
| 管理员代管 | `admin_collaboration.go` 的授权、双账本、审计与原键恢复 | 第四种代管动作 `purge_memory` |
| run 写入 | `lockRuntimeBusiness` + `currentAdmission` + `requireReservation` 门禁、mcb_ 令牌 | runtime memory 路由；按 (run, request) 幂等 |
| daemon | host_runner 的 claim/permit/prompt 流程、固定 brief 注入 | claim 声明、拉取内容、prompt/brief 文案 |
| CLI | `guardCollaborationCommand`、命令与参数白名单、`CollaborationError` 重试提示 | `multica collaboration memory show/update` |
| 实时 | `collaboration_event` 发件箱、`collaboration:changed` | 新事件类型，跳过工作台广播与活动流 |

## 存储

迁移编号在实施时按当时最新号顺延（调研时最新为 738）。M1 在 worktree 中使用 739–743：739 建表（含"只允许一次清除"的不可变触发器），740–742 三个并发唯一索引，743 为 `collaboration_run` 加列；提交前须再核对主线是否已占用这些编号。

`collaboration_memory_version`（`CREATE TABLE IF NOT EXISTS`，无外键）：

| 列 | 说明 |
| --- | --- |
| `id uuid` | 主键语义，由单独的并发唯一索引保证 |
| `issue_id uuid` | 协作单 |
| `version bigint` | 从 1 起按单递增；CHECK `version >= 1` |
| `content text` | CHECK `octet_length(content) <= 524288`（512 KB 硬顶，部署上限在应用层校验） |
| `bytes integer` | 写入时的 UTF-8 字节数；清除后保留，供历史显示 |
| `author_kind text` | CHECK IN (`member`, `agent`) |
| `author_user_id uuid` | 人写入时的账号 |
| `agent_id`、`run_id`、`resource_owner_user_id uuid` | AI 写入时的智能体、来源 run 与资源拥有者 |
| `request_id text` | run 写入的幂等键（人的写入幂等走 `collaboration_operation`） |
| `restored_from_version bigint` | 恢复操作的来源版本 |
| `purged_at timestamptz`、`purged_by_user_id uuid`、`purged_by_admin bool`、`purge_reason text` | 清除元数据；清除时 `content` 置空 |
| `created_at timestamptz` | |

每个索引单独一个迁移文件，使用 `CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS`，并登记到 `server/cmd/migrate/main.go` 的 `concurrentIndexCleanups`：

- `(id)` 唯一。
- `(issue_id, version)` 唯一：写入并发的最终仲裁，唯一冲突映射为版本过期。
- `(run_id, request_id)` 唯一、`WHERE run_id IS NOT NULL`：run 写入幂等，同时服务"本 run 写入的最新版本"查询。

`collaboration_run` 增加 `memory_mode text`（CHECK IN (`supported`, `unsupported`)，NULL 表示功能上线前的 run）和 `memory_base_version bigint`（首次注入时记录，0 表示空文档）。

"版本 0"不落库：一张单没有任何行时视为空文档，已有协作单无须回填（C01）。

## 写入语义

所有写入（人编辑、恢复、run 写入）在一个事务里完成：

1. 锁内重新解析权限与单据状态（人走 `ResolveIssueAccess`，run 走 runtime 门禁）。
2. 先查幂等：同一请求已成功则原样返回原回执，载荷不同则 409。这一步必须在版本校验之前，否则合法重放会被误判为版本过期。
3. 校验内容：UTF-8 字节数不超过部署上限（`memory_too_large`，返回 `bytes` 与 `max_bytes`）；含 NUL 拒绝（`invalid_memory_content`），不做静默清洗，保证重放哈希一致。
4. 读当前最大版本，不等于 `expected_version` 则 409 `stale_memory_version` 并返回当前版本号。
5. 插入 `expected_version + 1`。并发插入撞 `(issue_id, version)` 唯一索引时同样映射为 `stale_memory_version`。40001/40P01/55P03 按现有做法重试三次后 503 `collaboration_busy`。
6. 同事务写一条 `memory_changed` 发件箱事件（见"实时与副作用隔离"）。

不推进 `issue_collaboration.revision`、不写 `issue_edited`、不调用任何通知函数。

恢复：读取来源版本，已清除则 409 `memory_version_purged`；否则按上面的流程以来源内容写一个新版本，并记 `restored_from_version`。

清除：目标版本等于当前最大版本时 409 `memory_version_current`（当前最大版本只会增加，所以这个判断在并发下仍然安全）。用原 `Idempotency-Key` 重放返回原成功回执；换新键再清除已清除的版本返回 409 `memory_version_purged`，客户端按"已完成"处理并刷新。清除只置空 `content` 并写清除元数据，保留 `bytes` 与署名。清除原因只存在版本行与管理审计里，普通读者看不到；管理员代管清除对读者显示为"管理员已清除"（`purged_by: null, purged_by_admin: true`），真实管理员只记在审计中。（M2 实施时确定。）

AI 写入的内容在服务端经 `redact.Text`（`server/pkg/redact/redact.go`）处理后再存储与计算幂等哈希，与 host 已对所有共享执行记录做的脱敏一致（`server/internal/daemon/collaborationrun/host_recording.go:36`）。人的写入不做处理（C30，D-Q1 已确认）。字节上限按脱敏后的内容判断。

## 上限配置

- 环境变量 `MULTICA_COLLABORATION_MEMORY_MAX_BYTES`，默认 131072，合法范围 32768–524288。仿照 `parseLLMMaxRetries`（`server/cmd/server/main.go:152-182`）解析，非法值启动失败退出。
- 值注入 `CollaborationService` 与 `Handler`；`AdminReadConfig` 增加 `collaboration_memory_max_bytes`，管理后台设置页只读显示。客户端设置 schema 不是严格对象，旧客户端会忽略新字段。
- 所有 memory 读接口返回 `max_bytes`，Web 与 Desktop 的用量显示只读这个值。上限调低后已超限的当前版本照常返回（`bytes > max_bytes`），由客户端标"需要精简"。
- 写入请求体的读取上限按 `6 × max_bytes + 64 KiB` 设置，覆盖 JSON 转义的最坏膨胀；真正的限制以解码后的字节数为准。
- 文档同步：`.env.example`、`docker-compose.selfhost.yml`（显式传递）、`apps/docs/content/docs/environment-variables{,.zh}.mdx` 及 `pnpm generate:docs-bundle` 生成物、`docs/offline-upgrade.zh-CN.md`。

## 人的接口

挂在现有人类协作路由组下（JWT/PAT，`server/cmd/server/router.go` 1640–1700 一带），写接口用 `Idempotency-Key` 头：

| 方法与路径 | 权限 | 说明 |
| --- | --- | --- |
| `GET /api/collaboration/issues/{id}/memory` | Read | 当前版本：`version`、`content`、`bytes`、`max_bytes`、写入者摘要、`created_at`；能力位 `can_edit`、`can_purge` |
| `GET .../memory/versions?cursor=` | Read | 按版本倒序分页（每页 50），只含元数据与清除标记 |
| `GET .../memory/versions/{version}` | Read | 单个版本；已清除时只返回元数据与 `purged` |
| `PUT .../memory` | ControlAI，非终态 | `{expected_version, content}` |
| `POST .../memory/versions/{version}/restore` | ControlAI，非终态 | `{expected_version}` |
| `POST .../memory/versions/{version}/purge` | 有效发起人，终态也可 | `{reason}`，原因 1–1000 字节、无 NUL |
| `GET .../memory/operations/{requestId}` | 原操作者 | 原键恢复，沿用内容类操作的恢复接口 |

- 写入者展示复用评论的作者投影：外部账号只给 R54 最小摘要；AI 写入显示智能体名与来源 run。
- `can_edit`/`can_purge` 只是界面提示，服务端每次写入都重新判断。
- 终态写入返回 409 `issue_closed`（与现有协作写入一致）；无权限按现有能力错误码。
- 操作账本 `collaboration_operation` 的 kind 为 `memory_update`/`memory_restore`/`memory_purge`。载荷哈希对整个请求计算，账本中不重复保存内容。

管理员代管：`admin_collaboration.go` 允许的动作加上 `purge_memory`，沿用其授权（密码模式、JWT 会话、同组织平台管理员）、`creator_not_disabled` 判断、原因校验、双账本（`admin_operation` 与 kind 为 `admin_purge_memory` 的 `collaboration_operation`）和审计。管理后台只给版本元数据列表（版本号、大小、写入者、时间、清除状态），不展示正文；由报告人说明要清除哪个版本。

## 运行时接入

### 声明与登记（C28、C29）

- `CollaborationRunClaimRequest`（`server/pkg/protocol/collaboration.go:46-57`）增加 `memory_version int`（`omitempty`）。daemon 只在 host 模式（protocol 3）的 claim 中填 `1`；Linux v2 claim 不填。
- 服务端在首次 claim 冻结 prompt 的同一事务里写 `collaboration_run.memory_mode`：host 且 `memory_version=1` 为 `supported`，其余为 `unsupported`。后续 claim 不改写这个值。
- 不写入 `agent_runtime.collaboration_capabilities`，因此不触发迁移 737/704 的配置纪元推进、授权撤回与目标挂起。
- claim 响应结构不变。旧 daemon 的严格解码不受影响；旧服务端会拒绝新字段，所以必须先升级服务端（C29 前提）。

### 开工注入（C08）

- 新增 runtime 路由 `POST /api/collaboration/runtime/runs/{runId}/memory/base`（mcb_）。第一次调用把当前最大版本记入 `memory_base_version` 并返回该版本；之后的调用都返回同一个基线版本。如果基线版本在此期间被清除，就返回当前版本并改记基线。
- host_runner 在取得许可之后、组装 prompt 之前调用它（`server/internal/daemon/collaborationrun/host_runner.go:92-160` 之间）。同一 run 内 fresh-session 重试再取一次，拿到的仍是同一基线。
- 拉取最终失败（门禁拒绝或重试耗尽）时不启动 provider，按现有 `not_started` 终态路径收尾，不在缺失上下文的情况下开工。
- 内容只进 prompt：`hostPrompt` 的 JSON 增加 `memory: {version, content}`（空文档为 `{version: 0, content: ""}`）。claude/codex/opencode 都经 stdin 接收 prompt，不受命令行长度限制。内容不写入 brief 文件，也不写入本地账本。
- 这比 C08 原文"与冻结指令输入同一时刻"稍晚：发生在许可之后，会话锁等待之后。这样更符合 C08 的本意（排队和等待期间别人可能已经改过），也避免内容进入 claim 与账本。PRD 的 C08 已据此改写。

### 运行中读写（C09–C13）

| 路由（mcb_） | 说明 |
| --- | --- |
| `GET .../runs/{runId}/memory` | 当前最新版本，供 `memory show` |
| `PUT .../runs/{runId}/memory` | `{request_id, base_version, content}` |

- 门禁：`lockRuntimeBusiness` → `currentAdmission` → `requireReservation`，与 runtime 评论相同。已结算、停止、整单终态、目标挂起、授权撤回分别得到现有的 `execution_stopped`/`issue_closed`/`target_held`/`grant_required`/`stale_authority`。停止会撤销该 run 的全部 mcb_，迟到写入得 403。额外要求 `memory_mode = supported`，否则 409 `memory_unsupported`。
- 幂等：`(run_id, request_id)` 唯一。重放比较 `base_version` 与脱敏后内容的哈希，相同返回原版本，不同返回 409 `memory_write_conflict`。
- runtime 处理器通用请求体上限是 96 KiB（`server/internal/handler/collaboration_runtime.go:47-74`），memory PUT 单独使用上限配置一节的读取上限。
- 小队成员与续作各是独立的 run 行，有自己的 claim、基线与 mcb_，自然满足 C12。

### CLI

- 新增根命令 `multica collaboration memory show <issue-id>` 与 `update <issue-id> --base-version N --request-id UUID (--content-stdin | --content-file PATH)`。只在协作 run 环境内可用：不在 run 中时报错，并加入 `collaborationCommandAllowed` 与参数白名单（`server/cmd/multica/cmd_collaboration.go:14-61`）。`<issue-id>` 必须等于已受理的单。
- 不提供 `--content`，避免大段内容经命令行转义。内容经 `guardLocalPathLinks` 检查，提示用行内代码引用路径。
- `show` 默认打印 `version N · bytes/max` 与正文，`--output json` 给结构化结果。
- 失败提示：`stale_memory_version` 提示先 `show` 再合并、用新版本号重试；`memory_too_large` 给出当前大小与上限、提示精简；结果未知时沿用 `CollaborationError` 的"用同一 request-id 重试"。

### 固定文案

- `hostPrompt` 现有的 "Read the issue's shared context before acting"（`host_runner.go:488`）改为分别指明协作上下文与评论，避免和界面名 "Shared context" 混淆。
- `hostBrief` 增加固定几行：协作上下文是本单的当前状态；开工先读输入里的 `memory`；需要时 `memory show` 刷新；结束前把整份文档改写成最新状态后 `update`（改写，不追加）；推荐骨架；不写凭据；细节写评论、文档里只留指针；遇到版本冲突先读取合并再写。
- 同步更新 `server/internal/service/builtin_skills/multica-working-on-issues/` 的 `SKILL.md`、`references/collaboration-workflow.md`、`references/working-on-issues-source-map.md`，以及 `.trellis/spec/server/collaboration-runtime-cli.md`、`collaboration-host-execution.md`。

## 实时与副作用隔离

- 新事件类型 `memory_changed`（写入、恢复、清除都发），payload 只有 `version` 与动作，不含内容和作者。
- `deliverNext`（`server/internal/service/collaboration_realtime.go:94-161`）对这个类型跳过参与者和来源空间的 `workbench_changed` 广播，只发本单范围的 `collaboration:changed`。前端沿用现有的单据失效刷新，无须改事件 schema。
- `ListCollaborationActivityEvents`（`collaboration_activity.sql:3-9`）及其他读取 `collaboration_event` 的查询排除该类型，避免出现"已记录活动"空行。实施时要全量检索 `collaboration_event` 的读取方。
- 不调用 `writeCollaborationCommentNotifications`/`notifyAddedCollaborationMentions`。搜索只查 issue 与 comment（`server/internal/handler/issue.go:643` 一带），插件桥只订阅 issue/comment/task 总线事件，新表都不会被带出（C22）。

## run 记录（C24、C28）

- 每个 run 的投影增加可选字段 `memory: {support, base_version, written_version}`。`support` 取 `supported`/`unsupported`；`written_version` 为该 run 写入的最大版本，没写过时为 null；功能上线前的 run 不带这个字段。
- 按 `collaboration-records.md` 加可选字段，不新增条目类型，也不新增 `collaboration_run_event` 类型，所以旧客户端的封闭条目联合不受影响。
- 前端 schema 里 `support` 未知值按缺失处理（不显示标记），符合"服务端枚举要有默认分支"。

## 权限矩阵

| 操作 | 允许 | 终态 |
| --- | --- | --- |
| 读当前版本、版本列表、单个版本 | Read（参与者、来源空间成员） | 可 |
| 编辑、恢复 | ControlAI（全部有效参与者） | `issue_closed` |
| 清除历史版本 | 有效发起人 | 可 |
| 代管清除 | R38 平台管理员，且发起人被禁用 | 可 |
| run 读取、写入 | 已受理、未停止、`memory_mode=supported` 的 run | 拒绝（门禁） |
| 其他机器身份、普通 agent/task、plugin | 无（`ResolveIssueAccess` 对非人类返回 404） | |

## 客户端

### `packages/core/collaboration`

- 新文件 `memory-schema.ts`、`memory-queries.ts`、`memory-actions.ts`，依次仿照 `run-records-schema.ts`、`run-records-queries.ts`、`comment-actions.ts` 与 `operation.ts`/`draft.ts`/`pending.ts`，并从 `index.ts` 导出。
- 解析：`parseWithFallback` + zod；版本号用 `version.ts` 的十进制字符串；`can_edit`/`can_purge` 用 `.catch(false)`。畸形响应返回 null，不记录私有正文。
- 查询键：`[...collaborationKeys.issue(scope, id), "memory"]`，版本列表与单个版本再往后追加。键按账号划分，不带 `wsId`，与现有协作键一致（`access.ts:19-24`）。读取经 `protectCollaborationRead`。实时 `collaboration:changed` 已按单据前缀失效，自然覆盖这些键，无须改 `realtime/collaboration.ts`。
- API：`ApiClient` 增加 memory 的读、列、取、更新、恢复、清除方法，走 `fetchCollaboration`。路由不存在（旧服务端）时返回 unavailable，面板隐藏。
- 写入：先把草稿存入存储再发请求，拿到校验过的回执后才删除草稿；请求带 `Idempotency-Key` 与 `expected_version`；可经 `/operations/{requestId}` 恢复。409 `stale_memory_version` 返回回执而不抛错，与 `operation.ts:33-36` 一致。
- 字节数用 `collaborationTextByteLength`，上限一律用服务端返回的 `max_bytes`。
- `run-records-schema.ts`/`execution-schema.ts` 的 run 增加可选 `memory`，`support` 未知值按缺失处理。
- 测试：`memory-schema.test.ts`、`memory-queries.test.ts`、`memory-actions.test.ts`，都用 `// @vitest-environment node`。

### Web/Desktop：`packages/views/collaboration`

- 新组件 `memory-panel.tsx`，放在 `CollaborationDetail`（`collaboration-detail.tsx:64-102`）里 `CollaborationExecutionPanel` 之前，和 AI 相关的区块挨在一起。Web 和 Desktop 都只渲染 `AccountWorkbenchPage`，所以不需要平台接线。
- 展示：`RichContent resourcePolicy="restricted"` 渲染当前版本，默认限高折叠，可展开全文。头部显示版本号、写入者、来源 run、时间和"已用 / 上限"；超限时加"超出当前上限，需要精简"标记；空文档显示说明文字。
- 编辑：沿用协作区现有的 `Textarea`（`workflow.tsx:160`），旁边显示实时字节数，超限时禁用保存。版本冲突沿用 `workflow.tsx` 的过期基线模式：保留用户输入，提示"已有人更新"，让用户点"使用当前版本"后自己合并，不自动重提交。
- 版本历史：面板内用 `Collapsible` 列出元数据。查看某个版本用 `Dialog` 只读渲染。"恢复为此版本"和"清除"用 `AlertDialog` 确认，清除时要求填写原因；清除入口只在 `can_purge` 为真、且不是当前版本时出现。
- 深链：`core/paths/account.ts` 的 hash 类型增加 `memory` 与 `memory-v{n}`（`collaboration-detail.tsx:46-56` 现在只认 run、comment、description）。
- run 记录：`run-records.tsx` 的 `RunRecords` 摘要 `<dl>` 增加"协作上下文"一行，显示"基于 vN 开工 · 写入 vM"、"未更新上下文"或"运行时不支持协作上下文"，版本号链接到 `#memory-v{n}`。
- 文案：`locales/{en,zh-Hans}/collaboration.json` 新增 `memory.*` 段，`parity.test.ts` 保证两种语言的键一致。
- 测试：`memory-panel.test.tsx` 覆盖主流程、权限隐藏、冲突、超限和清除确认；判断矩阵放在 core 的 `.test.ts` 里。

### 管理后台（仅 Web）

- `AdminSettingsPage`（`packages/views/admin/settings/settings-page.tsx`）增加一行只读的 `collaboration_memory_max_bytes`；`observability-schema.ts` 设置 schema 增加可选字段。
- 管理后台的协作单详情，仿照已有的 R38 生命周期代管界面增加"清除协作上下文版本"：只列元数据，填写版本号和原因后提交。Desktop 没有管理后台路由，不做。

### Mobile

首版不做（C25），不修改 `apps/mobile`。服务端接口按账号划分、只面向人类账号，Mobile 以后接入时可直接使用。届时须按 `apps/mobile/CLAUDE.md` 自建数据层与账号/单据范围的实时订阅，并一并处理协作单在 Mobile 进入普通详情后写操作被 409 拒绝的现存问题（`research/client-patterns.md` §5）。

## 兼容、上线与回退

上线顺序：

1. 服务端与迁移。人的接口立即可用；没有 daemon 声明支持，所有新 run 都标 `unsupported`，照常执行。
2. Web 与 Desktop 客户端。旧客户端看不到面板，也不受新增可选字段影响。
3. daemon 与 CLI（Desktop 内置 CLI 随安装包发布）。之后的 host run 开始注入。

- 旧服务端会拒绝新 daemon 的 claim（`DisallowUnknownFields`，`server/internal/handler/collaboration_runtime.go:68`），离线升级文档要写明"先升级服务端"。
- 回退：daemon 回退到旧版只会让新 run 标 `unsupported`，不丢数据。服务端回退受迁移约束，down 文件拒绝回滚，与现有协作迁移一致。
- 已安装的旧 Desktop：run 记录的新字段是可选的，`collaboration:changed` 的结构不变，管理后台设置的新字段会被非严格 schema 忽略。
- 不新增功能开关。协作执行本身已有开关，host 模式默认关闭（10-24）。

## 测试策略

| 层 | 内容 |
| --- | --- |
| Go service（克隆模板库） | 版本 CAS 与并发插入、幂等重放与冲突、上限与 NUL、恢复与清除（含当前版本、已清除、终态）、权限矩阵、R38 代管、run 门禁全分支、基线冻结与清除后改记、事件不进活动流也不触发工作台广播 |
| Go handler | 用 `testutil.Call(...).Want(...)` 驱动路由；畸形请求体、超大请求体、`Idempotency-Key` 缺失 |
| Go CLI / daemon | 白名单与参数、stdin/文件输入、错误提示；host_runner 用测试创建的假 provider 可执行文件验证 prompt 含 `memory`、brief 不含内容、拉取失败不启动 |
| 启动配置 | 环境变量默认值、边界、越界启动失败 |
| `packages/core` | schema 畸形响应、查询键、写入的待处理与冲突处理 |
| `packages/views` | 面板主流程、权限隐藏、冲突提示、用量与超限、run 记录标记；矩阵类判断放在 core 的 `.test.ts` |
| e2e（Playwright） | 换机接力主场景（两个假运行时）、人编辑与冲突、恢复、清除、终态只读 |

真实智能体冒烟测试放在 `agentintegration` 标签后，只在用户授权时运行。

## 设计阶段的用户决定

- Q25：daemon 每次 claim 时声明支持，不写入能力声明，升级不重新授权（C29）。
- D-Q1：AI 写入沿用 `redact.Text` 脱敏，人的写入不处理（C30）。
- 用户撤回 Mobile 范围：Mobile 首版不做（C25），原 D-Q2（Mobile 文案语言）随之取消。

## 源码依据

- `server/internal/service/collaboration_comments_edit.go:18-75`：`mutateCollaborationContent` 的独立 revision 与幂等，人的写入照此实现。
- `server/internal/service/collaboration_workflow.go:371-421`：`mutateCollaborationWorkflow` 要求单据 revision，不复用。
- `server/internal/service/collaboration_squad.go:84-121, 205-243`：runtime 门禁与评论幂等。
- `server/internal/service/collaboration_runtime_capabilities.go:279-315`、`collaboration_runtime_preparation.go:325-367`：首次 claim 冻结。
- `server/migrations/737_collaboration_runtime_preparation.up.sql`、`704_collaboration_config_invalidation.up.sql`：能力声明推进配置纪元并撤回授权，C29 因此不走能力声明。
- `server/internal/daemon/collaborationrun/host_runner.go:31-218, 462-493`：host 流程、brief 与 prompt。
- `server/internal/service/collaboration_realtime.go:94-161`：发件箱广播。
- `server/internal/service/collaboration_records.go:388-430`：封闭的 run 记录条目类型。
- `server/internal/service/admin_collaboration.go:66-255`：R38 代管。
- `server/pkg/redact/redact.go:66`、`server/internal/daemon/collaborationrun/host_recording.go:36`：现有脱敏。
