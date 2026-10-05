# P1 技术设计

状态：2026-10-06 已完成本轮 P1 实施验收的技术合同；不是已上线事实。规划经 Architect → Critic 两轮评审，实施后的独立前后端复审已通过。来源为 [后端研究](research/backend-map.md)、[前端研究](research/frontend-map.md) 与原 PRD。具体DTO/证据版本/hash/响应以 [api-contract.md](api-contract.md) 为唯一合同。实际迁移为 536—549；执行结果与未验范围见 [验收记录](verification.md)。

## 1. RALPLAN-DR

原则：① 单项目身份、单描述事实源；② T1 正式准入白名单决定所有实时集合；③ 统计、人工判断、验收和执行互不代替；④ 事务与版本明确表达冲突；⑤ 新客户端增强不偷换旧字段含义。

三个驱动：历史客户端兼容与防止描述丢写；健康数字可解释且能准确下钻；无数据库级联时仍能原子发布、撤权和删除。

| 方案 | 优点 | 成本与风险 |
| --- | --- | --- |
| A：实时聚合 + 请求内数据库快照 + 内容版本指纹；只持久化进展中的统计 | 当前事实直接来自已有任务，清楚区分实时与历史；无快照清理任务，首期结构少 | 概览／下钻需重算；请求间发生变化必须明确刷新，需实测 10,000 任务 |
| B：持久化每次健康快照及任务成员表，下钻读取快照 | 跨请求可完整复现点击时集合，聚合后分页便宜 | 大量短命记录、权限与删除回收更复杂；旧风险任务会与实时状态分离，仍需刷新 |
| C：前端从任务列表聚合 | 起步快，复用现有视图状态 | 分页和个人过滤导致漏项、无法同事务授权、不适用于历史验收；不满足 P1 |

选择 A。B 是未来实测读负载要求出现后可评审的替代方案；不在 P1 同时维护两种统计来源。

## 2. 实施前现状、边界与落地模块

- 原入口在 `server/cmd/server/router.go:2132`，描述字段和完整响应在 `server/internal/handler/project.go:24`，现有更新缺 CAS 且 nullable 参数可能回写旧值（`:529`、`server/pkg/db/queries/project.sql:46`）。沿用 Project，不建立专项表。
- 当前 `done_count` 包含 done/cancelled；正式统计 SQL 已有 T1 白名单（`server/pkg/db/queries/project.sql:69`）。现有统计吞错路径（`project.go:80`）只作为旧兼容路径，不用于新健康。
- 实施前工作空间没有规划时区，项目没有描述 revision、状态历史和进展实体；不从 `updated_at` 虚构进入进行中的历史时间（[研究 §2—5](research/backend-map.md)）。
- 新增 `server/internal/projecthealth/` 纯计算与统一聚合入口；handler 分 `project_health.go`、`project_update.go`、`project_timezone.go`。共享表/锁/生成 SQL 由 foundation 统一整合，业务 handler 各子任务持有。
- 服务端状态归 Query；Web/Desktop 共用 views，移动端独立读兼容。遵守 `CLAUDE.md:31`、`:49`、`:72`、`:132`；无新依赖。

## 3. 数据模型与不可变历史

所有新表使用单数表名、UUID 身份与 `workspace_id`；时间戳为 timestamptz，项目/任务日期仍为 DATE。下面所有 NOT NULL 与 CHECK 由迁移实现；没有外键、级联或新增隐式索引。

| 位置 | 字段、唯一性和写入规则 |
| --- | --- |
| `project` 增列 | `revision bigint NOT NULL DEFAULT 1`（属性真实变动递增），`description_revision bigint NOT NULL DEFAULT 1`（仅正文真实改变递增），`in_progress_since timestamptz NULL`，`in_progress_since_source text NULL`（transition/migration） |
| `workspace` 增列 | `planning_timezone text NULL`；不放入可整体替换的 settings JSON；NULL 表示未配置且 effective UTC |
| `project_state_change` | `id, workspace_id, project_id, actor_type, actor_id, from_status, to_status, reason NULL, created_at, project_revision`；一次实际状态转换一行，未填 reason 返回明确标记；同 revision 不重复 |
| `project_update` | `id, workspace_id, project_id, author_user_id, published_at, current_revision bigint`；固定作者／首发时间，不因更正改变最近发布时刻 |
| `project_update_revision` | `id, workspace_id, project_id, update_id, revision, editor_user_id, created_at, kind, body, health_judgment NULL, correction_reason NULL, evidence jsonb, statistics_snapshot jsonb NULL, acceptance jsonb NULL`；只 INSERT，唯一 `(update_id, revision)` |
| `project_update_request` | `workspace_id, project_id, actor_user_id, request_id, operation, payload_hash, update_id, result_revision, created_at`；唯一 `(workspace_id, actor_user_id, request_id)`；operation 为 create/correct |
| `project_update_notification` | `id, workspace_id, project_id, update_id, recipient_user_id, source_revision, status(pending/delivered/cancelled/dead_letter), delivered_at NULL, attempts, next_attempt_at NULL, last_error_code NULL`；唯一 `(workspace_id, update_id, recipient_user_id)`，稳定 id 同时用作 inbox id |

`kind=progress|risk|acceptance`，`health_judgment=on_track|attention|risk`。`acceptance` 为 `{conclusion: passed|partial|failed, scope, explanation, description_revision, description_snapshot}`，保存文本快照而非另一个可编辑目标；验收修订不改变适用描述和快照。针对新描述验收必须另建记录。非验收 kind 不带 acceptance；更正不能改变 kind。

`evidence`保存api-contract §4的EvidenceInput、分kind的observed_version与collected_at；issue用revision，execution用state_version+结果digest，url无验证版本。读取时逐项授权/补标签，不永久复制保护正文；附件继续原下载权限，不新增公开下载。历史署名按合同§3批量最小展示字段，离开/删除仍有稳定署名状态。

`statistics_snapshot`保存合同§2的非递归ProjectStatisticsSnapshot，绝不嵌入进展/验收对象；拒收客户端数字。不完整时允许不附快照发布，显式请求附快照但不可计算时返回503。更正默认保留原快照；显式重新采集生成新修订的快照且保留旧版。当前/最近验收按稳定发布时间和ID选择，规则见合同§2，不用更正时间覆盖最新验收。

新表唯一 ID、业务唯一约束和查询索引全部以独立单语句 `CREATE [UNIQUE] INDEX CONCURRENTLY` 文件建立；如需主键，在后续迁移 `ADD CONSTRAINT ... PRIMARY KEY USING INDEX` 复用已建唯一索引。不得在 CREATE TABLE 内用 PK/UNIQUE 隐式创建普通索引。按项目查询时间序列索引 `(workspace_id,project_id,published_at,id)`、修订 `(update_id,revision)`、待投递 `(next_attempt_at)` partial；不提前添加未经 EXPLAIN 证明的健康组合索引。

历史描述保持原值并设 revision=1。历史 `in_progress` 项目从迁移时刻建立 migration 基线，界面说明“历史开始时间未知，从升级后计时”；不得借 `created_at/updated_at` 声称历史进入时间。其他状态 since=NULL。后续真实进入 in_progress 改为 transition，退出保留历史审计但当前 since=NULL。此项产品建议冻结为 P1 默认，评审可显式修订。

## 4. 更新、CAS、日期与能力检测

沿用 `PUT /api/projects/{id}`，新增 `expected_revision?`、`expected_description_revision?`、`status_reason?`。先授权并锁当前项目，再依据请求实际存在的键合并；不把预先读取的旧 nullable 值传回 SQL。WHERE 同时限定 workspace/id。空缺不改变，null 的既有清空语义保留。

- 请求含 description 且与当前值不同：必须带 `expected_description_revision`；缺失 428 `project_description_revision_required`，过期 409 `project_description_conflict`。同值 no-op 可不带 token。合法变化同时递增两种 revision。
- 新增强客户端的状态／日期等编辑带 `expected_revision`，不匹配返回 409 `project_revision_conflict`；旧客户端缺 token 仍可改非描述字段，锁内合并保留其他人的字段。同一字段的旧写保持既有最后提交语义，不声称防止所有旧客户端丢写。
- 描述冲突返回已授权的最新正文与 revision；客户端保留本地输入并比较/合并后重试。修改图标、默认小队或资源不递增 description_revision，也不令验收过期。
- 状态变化沿用五状态任意合法修改，completed 不加业务硬阻止；所有入口记录 actor、时间及 reason 或未填写。状态无实际变化不重复审计；不创建进展、验收或执行。
- 新编辑请求涉及日期时校验最终 start<=due；不涉及日期时允许旧异常值继续读取并提示修正。日期不相互推算、不移动任务日期。

`GET/PUT /api/workspaces/{id}/planning-timezone` 返回 `{workspace_id,planning_timezone:null|string,effective_timezone,configured}`；GET/PUT 均通过 URL 工作空间成员路由，URL 与调用上下文工作空间一致；PUT handler 在事务锁内重新检查 owner/admin 人类 actor，操作权限不足返回 `403 project_permission_denied`。接受有效 IANA 名称及 null 清回 UTC，使用 `time.LoadLocation`，拒绝 Local/任意 UTC 偏移字符串。设置页明示基准，不静默取当前浏览器时区。

能力检测采用 `GET /api/workspaces/{validWorkspaceId}/project-capabilities`，放在URL工作空间成员授权组。旧`/api/projects/capabilities`会命中项目id解析而返回400，禁止使用。仅专用子资源404且当前工作空间仍可读时判unsupported；400/401/403/network/畸形响应分开处理（api-contract §1/6）。能力缓存按服务器连接身份+workspace隔离、重连失效；不支持时保留原页面，明确限制新编辑。

旧 `issue_count=N`、`done_count=F+C` 永不改义。新增 `completed_issue_count=F`、`cancelled_issue_count=C`、`open_issue_count=U` 与 `statistics_complete` 可选字段，在 list/search/detail/create/update/squad 配置响应一致扩展；真实失败不补新分项零值。新客户端收到旧字段时显示原范围闭合，新增分项显示不可用，不能用 done_count 推断完成数。

## 5. 统一健康、快照与下钻

正式全量集合 `S` 使用 SQL `i.workspace_id=$ws AND i.project_id=$project AND i.admission_status IN ('not_required','accepted')`。现有任务实体删除后不在该表结果内；若未来增加软删字段，再显式排除。不要发明当前不存在的 `deleted_at`。父子独立 ID 去重，关联有效性用 EXISTS/唯一 lookup 避免 JOIN 放大。

overview/drilldown/preview与publish/correct统一使用接收qtx的采集函数，在同一REPEATABLE READ快照读取项目、时区、含archived目录、全量正式任务、有效指派与最新进展。业务只读端点也需授权locking read，故不设PostgreSQL READ ONLY；只禁止业务INSERT/UPDATE。目录或任务失败令complete=false或503，未知项为null而不是0；未知状态留在N/U且标不完整，不复用吞错helper。

事务具体落点：现有`txStarter.Begin`（`handler.go:59`）返回pgx.Tx后，**第一条SQL**执行`SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ WRITE`，在任何SELECT/锁/查询器调用之前；失败立即rollback，不拓宽全局txStarter接口。随后workspace shared→actor撤权fence→active member `FOR SHARE`→project→source锁。RR快照可能在workspace查询建立，因此等待fence后成员必须locking read；若成员已更新/删除由PG报40001并重试，不能普通SELECT旧成员宣称当前授权。来源版本/授权细节在合同§4，同一qtx贯穿检查、统计与写入，禁止另开只读事务拼贴结果。

SQLSTATE40001、40P01或55P03仅整事务rollback后重新Begin/设隔离/授权/采集/写入，最多3次（首次+2重试，25ms/75ms加0—25ms jitter，受请求context截止约束）；最后503 `project_write_retry_exhausted`。request_id和原payload保持不变；业务409/422不自动重试。唯一请求竞争rollback后读取已提交身份，匹配hash回放，否则409；无outbox/事件在commit前外发。锁住前授权旧快照或双时点统计均为实现错误。

有效负责人：member 的 assignee_id 为 user UUID 且当前 workspace_member 有效；agent 同空间存在且未归档；squad 同空间未归档且有效 leader 存在未归档。类型/ID 不成对或未知类型为无效。此处判断引用生命周期，不调用当前查看者的执行授权。离线 agent 仍有效，另给 `execution_environment_unavailable_count`；小队机器绑定不足也独立说明，不自动改配（`issue.go:3727`、研究 §4）。lead 单独按 member/agent 验证，不代替 assignee。

计算 §9 原 PRD 的 N/F/C/U、B/O/A/R、L 与风险并集；`N=0` 比例 null；unknown 在 U 且完整性失败。D 为采集时刻在 effective_timezone 的日期，due<D 才逾期；paused 仍可逾期。七日依据本地日历日期差 >=7，锚点为最近发布时刻，无发布则最近真实进入 in_progress／迁移基线；更正不重置七日计时。

overview返回含statistics与两个验收摘要的ProjectOverview；分页/更新/错误的唯一shape见api-contract。统计与验收分层避免递归，history不会重新计算发布时统计。

`snapshot_version` 为服务端规范序列化后 SHA-256 指纹，纳入 project_revision、D/时区、状态目录、按 ID 排序的正式成员及影响健康的字段/有效指派/运行环境状态、最近发布时间；不是权限凭证。计算时间本身不进入指纹，避免无变化也刷新。health 优先 incomplete→unavailable；空任务仍单独显示项目逾期；其余按原 PRD 风险→需关注→暂无上述风险；结束状态独立展示未结束任务事实。

`GET /api/projects/{id}/health/issues?signal=blocked|overdue|unassigned|in_review&snapshot_version=...&cursor=...&limit=...` 重新使用同一服务/同一个请求内快照，返回 `{items,total,snapshot_version,refreshed,overview,next_cursor}`；limit 默认50，最大100。cursor 绑定 signal、版本与稳定排序位置。版本相同则总数与原卡片一致；不同则 `refreshed=true` 并返回当前 overview/计数。按已评审的 [ADR-05](research/pagination-adr.md)，无cursor返回新第一页，有cursor时保留last_id，在当前正式风险集合中继续其后位置，不强制回首页。页面只替换、不累计旧页；变更提示在遍历中持续，明确锚点前新增或重新入险的任务需从头刷新。只有成功的显式从头刷新才清提示；失败保留。空后缀而total>0只说明当前位置之后无结果，不宣称项目无风险。

下钻为临时项目风险上下文，显示信号和基准日，可清除回原视图。它不能继承当前 activeView、actorKind、showSubIssues、隐藏状态、个人负责人过滤；复用任务列表／表格渲染，不复用被过滤的统计集合。P1 风险结果初始只提供准确列表／表格，原任务入口继续保留五种视图；不把无日期风险任务静默丢给甘特投影。服务端仅接受规定的 signal，客户端不能用任意 SQL/条件扩权。改变列表／表格样式不改变 health scope，返回概览恢复总体统计。

## 6. 手动发布、证据复核、修订与通知

新增 `GET /api/projects/{id}/updates`（cursor 默认20、最大100），以及该项目下的 `GET /updates/{updateId}/revisions`、`POST /updates/preview`、`POST /updates`、`PUT /updates/{updateId}`。指定修订的执行证据通过 `GET /api/projects/{id}/updates/{updateId}/revisions/{revision}/executions/{taskId}` 读取真实任务和消息；要求该修订确实引用执行，并重新校验当前成员及来源访问权，复用既有执行记录对话框。所有身份均在项目工作空间内解析，列表不跨空间；发布/修订同时检查真实 member actor、机器凭据类别、有效成员及撤权 fence，不能只相信 owning user UUID（`actor_guards.go:96`、`triage.go:175`）。

preview/WriteInput/Create/Correct返回采用api-contract §3—6，固定canonical hash算法与身份/nullability/排序。include_statistics时hash纳入统计version但不纳入采集时间，实际发布重算版本不同须409重新预览；成功保存写事务实际采集snapshot。正文trim后Unicode code point 1—10,000，理由1—1,000、证据最多50；验收scope必填，passed/partial需证据或可复核说明；既有description不收紧限制。

发布RR写事务在§5隔离/授权/重试协议内：workspace shared→member fence/locking read→project→request检查→source权限/版本锁→统计→INSERT update/revision/request/outbox→commit→刷新事件。描述与验收同项目锁，更正CAS；preview后变化409给可见新差异，不回传无权内容。实际snapshot与preview按合同比较，不把preview数字直接落库。

幂等 request_id 按一次用户意图生成并保留重试；规范化 payload hash 包含所有业务输入、复核版本、operation 和目标，排除纯传输字段。相同身份/hash 已成功返回原 update/result_revision（200 replay=true），不同 hash 为409 `idempotency_conflict`，不额外发通知。并发插入身份靠唯一索引加事务重试；成功响应丢失用相同请求重放查询原结果，未经授权不能查询。失效 preview 后用户确认新内容生成新 request_id。

通知由服务端解析显式member提及；首发每人一条，更正只给从未通知的新成员补发；稳定inbox身份防重，agent只引用、不用CommentCreated。P1借鉴T1持久去重，不复制其先锁队列算法（`triage_notifications.go:164`）。

outbox算法冻结：无锁SELECT最多100条due候选ID→逐条READ COMMITTED事务→workspace shared→recipient撤权fence/active member锁→project shared→outbox `FOR UPDATE SKIP LOCKED`→锁内重查pending/到期/成员/项目/修订→INSERT inbox ON CONFLICT DO NOTHING与status=delivered同事务commit。始终先project后outbox；第二worker锁不到outbox直接跳过。项目删除若先完成，查不到project则结束不重建；成员无效时在正序锁内标cancelled并仅存原因码。投递/删除/撤权不得在已持outbox时倒序阻塞。

已新增`RunProjectUpdateNotifications(ctx)`在`server/cmd/server/main.go:652`邻近T1 worker注册，共用sweepCtx/退出cancel；启动立即扫描，之后每5秒，单记录事务最长5秒，逐条提交。临时错误整笔rollback，另起同锁序事务记录attempts与下次时间：min(5s×2^(attempt-1),15m)+0—1s jitter；12次失败进入dead_letter，停自动投递、日志/指标告警，保留记录可由运维既有DB操作复位pending（无新增UI/外部通知）。40001/40P01/55P03也按此队列退避，不在持锁时sleep；进程崩溃未commit不算已投递，恢复按同ID重试。

站内进展事件只发 workspace/project/update ID 与 revision 用于 Query invalidation，不能广播正文/证据。不产生 Task/Issue/Comment 执行事件，不发邮件或外部回执。通知已读/归档不改变任何业务状态。

## 7. 错误与客户端恢复

新增错误保持现有 HTTP error envelope，并附稳定 `code`、可选 `field_errors` 与经授权的 `current`；不将调试 SQL/他空间身份返回客户端。

| HTTP | code/情况 | 恢复 |
| --- | --- | --- |
| 400 | 无效 UUID、signal、cursor 或日期格式 | 指定字段错误，保留输入 |
| 401 / scope 403 / scope 404 | 认证失效、工作空间访问撤销；scope 404 必须携 `workspace_access_denied` | 清保护 Query/证据/草稿及候选缓存，拒绝迟到响应；不重试写 |
| 403 | `project_permission_denied`、`project_evidence_forbidden`、`project_updates_disabled` | 操作、来源或能力受限；保留仍授权的工作空间内容与本人输入，来源失权单独清证据查询，不当整空间撤权 |
| 404 | 当前授权范围内项目/进展不存在 | 停止提交；已删除项目只允许复制本人未提交正文 |
| 409 | project_description_conflict、project_revision_conflict、project_update_revision_conflict、project_update_preview_stale、idempotency_conflict | 显示最新可见状态/差异，保留输入；不能自动覆盖 |
| 422 | 文本/验收/证据/IANA/最终日期顺序无效 | 逐字段提示；外链“未验证”不等于错误 |
| 428 | project_description_revision_required | 告知需支持版本编辑的客户端；保留旧客户端输入 |
| 503 | project_health_unavailable、暂时锁冲突或可重试数据库失败 | 不展示新绿色健康，保留明确时间戳旧值；写重试沿用 request_id |

草稿 key 包含服务器连接身份/workspace/project/kind，只有本人输入可持久化，不能把服务器证据正文持久化。撤权时清理该空间的 Query、编辑器中服务器内容和候选项，并移除该空间持久草稿；删除项目可留下单次“复制我的未提交文本”界面，但不自动恢复对象。发布成功才清草稿。服务端响应均 schema 解析，缺失或未知枚举显示降级，不能通过 fallback 假装新写成功。

客户端实现落点：`packages/core/projects/{p1-queries,p1-mutations}.ts` 提供 P1 查询与写入，既有 `queries.ts` / `mutations.ts` 保留原项目接口；overview/updates/revisions/risk keys，均带 wsId/projectId；所有 fetch 带 captured workspace、AbortSignal，按 `api/client.ts:4468` 同时检查 project/workspace 身份。发布、删除、验收、描述 CAS 使用 server-first；修正 `mutations.ts:114` 的现有乐观删除，不让新增预览字段直接混入 Project cache。普通可预测图标等属性仍可局部乐观更新。

模板复用 `ContentEditor` 的 `onUpdate(markdown,baseMarkdown)`、`insertMarkdownAtEnd`、`flushPendingUpdate`（`content-editor.tsx:122`、`:295`、`:334`）；空描述插入、非空预览勾选缺失章节，插入未就绪不能丢内容。旧自由正文不解析成强制结构；描述自动保存序列化并保存 adopted revision，复用 `RevisionConflictCompare` 展示冲突。显式采用服务端版本时清除放弃的本地草稿，并保持已授权正文与 revision，直到 canonical props 追上；旧受控值不能盖回服务端选定版本。进展另设按项目/创建或更正身份的 draft store，登记 `drafts/register-all-drafts.ts:17`，迟到成功不能清新输入；进展的预览、取消与仅复制删除文本的命令式读取统一 trim，与 ContentEditor 回调/卸载 flush 的既有归一化一致；包括提及末尾空格在内的等价正文只是确认，不取消成功预览，真实正文变化仍使预览失效。该真实浏览器问题由 `31534d844` 修复，见[回归证据](../10-05-projects-p1-ui/preview-whitespace-verification.md)。

WS 在 `realtime/use-realtime-sync.ts:795` 的 project 前缀下失效新查询；新增 `project:update_published|update_corrected|planning_timezone_changed` 只传身份/版本。任务状态、项目移动、删除、截止日期、指派、T1接受，成员离开、agent/squad归档、runtime在线状态、自定义状态目录更新与时区修改均影响健康。扩展 `issues/cache-coordinator.ts:606` 目前只处理 status/project 的依赖。概览每60秒检查工作空间规划日期，发现与统计基准日不同才失效并重新查询；窗口焦点恢复立即执行该检查，重连失效相关查询。它不是午夜精确定时器，也不采用查看者时区。

撤权事件、401、scope 403 或明确 `404 workspace_access_denied` 先取消并 remove workspace/project queries、关闭编辑器/证据预览并清理内存/持久草稿，之后由现有单 responder 处理导航；不等待 workspace 列表成功才遮蔽内容（`use-realtime-sync.ts:1253`）。操作/来源/能力限制的三类403保留仍授权的空间数据与本人输入，普通项目/进展404也不当整空间撤权。会话清理递增单调 generation，旧请求、mutation及编辑器清理不能回灌新会话；退出登录或切换服务器时清除仅供复制的删除项目文本。项目删除确认成功才清缓存/导航；事件与本端行为用 self-event guard 避免双跳转。

项目详情增加概览／任务切换及共享路径参数，通过 `paths.ts:41` 和 NavigationAdapter，Web `projects/[id]/page.tsx:6`、Desktop `project-detail-page.tsx:8` 保持刷新/返回/新页可恢复。风险下钻由独立 `ProjectRiskIssues` 呈现，未挂接普通 `IssueSurface` 的视图控制器，因此不会继承 activeView 或个人过滤；原任务入口继续提供五种视图。移动端只解析可选字段并维持旧 done_count，自己的 hooks/cache 适配能力限制及旧属性更新；不导入 Web hooks/store，也不提供伪成功的新进展编辑。

## 8. 删除、关联写与锁顺序

沿用管理员删除，`project.go:680` 的事务基础上完整应用层清理；旧 FK 迁移证据在 `034:19`、`065:7`、`097:2`，实施先核验 catalog，移除相关依赖时用 IF EXISTS，不依赖部署库一定存在这些 FK。

统一锁顺序：workspace shared → actor撤权fence/member → project（多项目 UUID 升序）→ issue/update/子记录。关联 writer 在最终写入事务持项目共享锁，删除持 FOR UPDATE。已先持 issue 锁的 T1/批量路径请求项目锁必须 NOWAIT，失败整事务回滚并有限重试，不阻塞形成 project→issue 环；保持现有 T1 `FOR SHARE NOWAIT` 契约（`triage_actions.go:43`）。

必须纳入的 writer：任务创建（显式、父任务继承、自动化）、单条/批量换项目、T1 接受与候选项目写入（手工/CSV共享 `triage.go:499` 的创建事务）、聊天会话、资源写、默认小队配置、自动化/触发器绑定、进展首发/更正/投递，以及工作空间删除。以统一函数/查询实现，不靠 handler 前置检查。原资源与执行快照 fence 回归继续通过（研究 §7—8）。

delete-impact 响应（`GET /api/projects/{id}/delete-impact`，仅管理员）给关联任务总数/正式数、资源/进展/自动化数量、`project_revision`，声明任务及执行历史保留。DELETE 可带 expected_revision；缺少时仍允许旧管理员入口，清理集合以锁内最新值为准。

删除事务锁住项目后：解除全部同空间issue.project_id；清理scope视图/聊天引用、资源/小队配置，不删共享实体/物理目录；自动化清project_id，非archived项设`status='paused',pause_reason='project_deleted'`，已archived维持archived但清引用，所有相关`autopilot_trigger.enabled=false`（现有语义`autopilot.sql:73`、`:89`、`:225`）；删专属进展/revision/request/outbox及项目通知保护载荷、状态审计和project行；commit发事件。保留issue/admission/assignee、评论、运行执行/历史，不授新资源权限。任一步全部回滚。

T1候选字段实际位于 `issue_triage.candidate_project_id`（`triage.go:608`），与issue真实归属不同。删除保留候选ID和既有分拣审计中的ID作为来源记录，读取显示“候选项目已删除，需要重新选择”，不返回已删项目正文；接受时必须重新校验，不能把失效候选写回issue。候选创建同样持锁，先删除则候选校验失败；先写候选再删除允许留下明确失效的来源引用，不把它当活跃孤儿。进展专属request/outbox随项目删除；任何迟到请求先校验项目存在/授权，返回404，禁止重放重建项目、进展或通知。历史执行及分拣审计不改写。

工作空间删除显式清理新表；不能仅依赖项目删除入口。所有 writer 与删除的两种顺序只允许“关联失败”或“先关联后解除”。P1 不创建迭代表；未来 I1 接入时必须保持此删除事务不写迭代归属/快照，并追加真实集成验收；当前不能把此项称为已验证。

## 9. 上线、回退、性能与 ADR

先additive schema/索引、回填/API，再能力开关/UI。预发布验证旧项目逐字段无损与旧计数。UI回退只关闭入口，schema/进展保留且兼容服务端可只读。数据down必须guard：任何新进展/revision/request/outbox/state audit数据，非空规划时区或project revision/description_revision>1即明确拒绝，不能导出后自动删除。空新增数据的维护down先停全部API/worker/writer，预检后才撤索引；数据down在显式事务中按workspace→project→新表取得排他表锁，锁内再次检查guard，非空RAISE EXCEPTION整笔回滚，再移除空表/增列。并发写先提交则guard拒绝，down先锁则writer等候后失败，不能丢已提交数据。索引down仍独立单语句，部分失败按up恢复；不承诺重建移除过的历史FK。填充库、并发writer、索引失败均隔离演练。

foundation 提供可回退的新 UI 能力配置，关闭写入口时不能把普通旧客户端描述无 CAS 再放开，否则会破坏并发保证。后端回退目标必须是含安全 CAS 和新表只读支持的兼容版本；不能回退任意旧二进制然后声称历史仍可见。

性能由verification固定500项目/每项目最多10,000任务分布、PG/Go/机器与并发，采EXPLAIN/CPU/内存/query数；预热20次，overview/四signal并发1/10各200请求、冷5次；跨端30次due/assignee/admission变更。活跃分页另以0/1/10次每秒底层变更各10分钟、每秒翻页，记录刷新比、到第二页成功率/P95、连续重置次数。先基线后评审锁预算再最终测量；HTTP 2秒门槛已由C复测通过；跨页面5秒收敛证据单列于浏览器报告。原A已实测在1/s下第二页42.8%、10/s下0/599，按ADR-05评审改为C并保留A证据；B仍是冻结初始工作清单的有效替代，C不宣称跨页穷尽变化中的最终集合。复测使用实际ID前进而非refreshed=false作为前进判断，非空后缀必须100%前进，终止空后缀单列。相同版本统计/下钻一致性始终100%，不能第一页代全量。C三档各至少600秒实测1709/1709非空续页严格前进、第二页成功率均100%、静态最大P95约558ms，74页当前集合与独立SQL完全一致；见[性能报告](../10-05-projects-p1-verification/performance-c-report.md)。

ADR-01：选实时服务端聚合 A；驱动为正确集合/成本/解释性；拒绝前端聚合，暂缓持久健康快照。代价为重算与刷新提示；跟进是实测后选择索引，不预加缓存。

ADR-02：单描述 + 不可变验收快照；拒绝 goal/scope/criteria 双写。描述真实改写要求 CAS，旧无 token 写返回428；这是为满足并发不覆盖而对旧描述编辑的明确限制，其他原项目管理仍兼容。不能同时保证任意旧无版本写入与防丢写。跟进更新 CLI/内置 skill 契约和旧客户端提示。

ADR-03：append-only 进展修订 + 请求幂等 + 专用通知 outbox；拒绝复用任务评论发布链，因为提及可能执行 agent。代价为三类身份与投递运维；跟进记录重试/积压、不记录保护正文。

ADR-04：应用层清理 + 所有关联 writer 共享锁；拒绝依赖旧 FK 级联或静默解绑继续跑自动化。代价是 foundation 范围跨若干现有 writer；跟进强制删除并发/故障注入测试，未来 I1 追加历史保护证明。

## 10. 三项 pre-mortem 与可观测性

1. **上线后“完成数”突然下降或健康漏项**：旧 done_count 被改义、分页或 activeView 污染。预防：老响应金样、N/F/C/U矩阵、10000条含隐藏父子/非正式任务、隔离下钻上下文；指标记录 complete=false/unknown原因，禁止正文入日志。
2. **验收依据错绑或已撤权证据仍可见**：无版本写或缓存/preview复用。预防：描述锁/CAS、提交再授权、证据版本复核、撤权清缓存/草稿、机器身份矩阵；监控409/428与拒绝原因（只含ID/计数）。
3. **删除后孤儿任务/自动化继续创建或通知重复**：某个 writer 未持共享锁、投递崩溃。预防：逐入口锁清单、双顺序竞争/故障注入、事务outbox/稳定inbox身份；观察 orphan扫描、outbox积压、重试次数与删除失败。任何新增关联 writer 必须加入此协议与测试。

实施更正：删除采用 READ COMMITTED，在持有项目排他锁后枚举当前关联；发布/统计继续 RR。真实chat-create/delete屏障证明旧RR删除可能漏掉等锁期间提交的子记录，现有10阶段回滚与17入口双序测试锁定此差异。
