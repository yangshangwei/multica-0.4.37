# 性能测量设计

可执行协议以本文件为准；`research/performance-harness.md` 是原始研究，其中与本文件或 `research/plan-review.md` 不一致之处（过期行号、帧内版本语义、回写样本结构）以本文件为准。行号对应审查时的工作树（HEAD `fb9f4c070`），S2正在修改服务端文件，实施时按符号定位。

## 1. 已核实的产品事实

| 能力 | 符号 / 文件 | 对工具的约束 |
| --- | --- | --- |
| 注册、登录 | `TestApiClient.registerPassword` / `loginPassword`（`e2e/fixtures.ts`）；`PasswordRegister`、`PasswordLogin`（`server/internal/handler/auth_password.go`） | 注册每IP 30次/5分钟且要求 `ALLOW_SIGNUP` 未关闭；登录每IP 300次/分钟、每账号10次/5分钟。token默认30天 |
| 客户端基址 | `e2e/fixtures.ts` 模块加载时读取 `NEXT_PUBLIC_API_URL`，`DATABASE_URL` 缺省回落到本机 `multica` 库；`e2e/env.ts` 会从cwd加载 `.env` | 必须由私有runner显式给出两者，工具自身再拒绝默认库与非loopback |
| 邀请、转交 | POST `…/participants`、POST `…/handoffs`（`InviteParticipant`、`Transfer`，`service/collaboration_workflow.go`） | 普通workflow body只认四个 `expected_*` 字段加 `Idempotency-Key`；协作邀请未发现速率限制或人数上限；一次一个revision，必须串行 |
| 状态写 | GET `…/status-options?action=change_status`；PATCH `/api/collaboration/issues/{id}`（`ChangeStatus`、`finishStatusChange`，`service/collaboration_lifecycle.go`） | 仅当前发起人或处理人有 `Edit`（`collaboration/access.go` `CapabilitiesFor`）。一次状态变化恰好产生一条事件、无通知；回执只有 `code` 和 `collaboration_revision`，没有 `event_seq` |
| 权威读取 | GET `/api/collaboration/issues/{id}?execution_view_version=6` | `realtime.event_seq` 是当前会话事件序号；版本均为十进制字符串，用BigInt比较 |
| WebSocket | `HandleWebSocket`、`handleSubscribe`、`writePump`（`server/internal/realtime/hub.go`）；`outboundScopeCurrent`、`PublishWithID`（`realtime/collaboration.go`） | `/ws?scope=account`，首帧 `{"type":"auth","payload":{"token"}}`，10秒内未认证即关闭；每连接发送缓冲256帧，满则被驱逐；每帧发送前重新认证并重查scope（数据库查询）；无断线重放 |
| 事件中继 | `CollaborationRealtime.Run` / `deliverNext`（`service/collaboration_realtime.go`）；`LockNextCollaborationRealtimeEvent`（`queries/collaboration_realtime.sql`） | 500ms定时扫描，每条事件先向每个参与账号发空的 `collaboration:workbench_changed`，最后发 `collaboration:changed`。帧内 `event_seq` 属于该事件，`collaboration_revision` 是**中继时的当前版本**而非事件发生时的版本；`event_id` 为 `<事件UUID>:issue` |
| Node客户端 | Node 22.23.2 原生 `WebSocket` | 已实测不发送 `Origin` 头，满足 `checkOrigin`；自动应答ping |
| 受控回写 | `startHostDaemon`、`act`、`release`、`enrollHostRuntime`、`adoptHostTarget`、`openHostAcceptanceGate`（`e2e/fixtures/collaboration-host-runtime.ts`）；`collaboration-host-provider.go` | `act` 让Provider经run-local CLI写评论，评论ID等于 `request_id`（`CreateRuntimeComment`）；`release` 后Provider输出 `Host fixture result for <runId>`。测试poller `PollInterval` 50ms、并发1 |
| API触发运行 | `e2e/collaboration-installed-clients.spec.ts` 的 `controlInput` / `addInput` 模式 | Resume与带 `target_versions` 的提及评论均可纯API完成；`sendHostComment`、`seedHostRuntime`、`hostEnvironment` 需要浏览器或UI环境键，本工具不调用 |

三个Provider夹具的 `--fixture-kind` 输出相同，不能据此区分。回写必须使用由 `collaboration-host-provider.go` 构建的二进制：closure Provider在回调前已写完评论，会得到近零的假耗时。manifest绑定该二进制的源文件与产物SHA-256，预热run再做行为核对（写指令前以 `request_id` 读评论必须404，`act` 必须返回回执）。

## 2. 账号与会话准备（不计时）

分为两个独立阶段，状态保存在仓库之外的私有运行目录（0600），报告只含 `{actor_index, user_id, username}`。

1. **prepare**：生成本次运行的后缀与 `p000`–`p099` 用户名（3–32位字母数字下划线）。按每5分钟窗口至多30个、并发1–2串行注册，四个固定窗口通常约10–15分钟，取决于首窗起跑位置，另计请求处理时间；记录实际注册时间，不将估算作为最短时长约束。注册直接调用公开的 POST `/auth/register`（与 `TestApiClient.registerPassword` 同一路由），以便读取429的 `Retry-After` 头（实施偏离，2026-11-14采纳）。遇429按 `Retry-After` 等待；结果不明时用同一用户名密码登录确认，不换用户名、不重启API来清空进程内限流。actor 0 完成 onboarding 并建立来源workspace，以公开API创建一条协作任务，再串行邀请其余99人（每次先读当前版本，新 `Idempotency-Key`，未知结果按原键 GET `…/operations/{requestId}` 恢复）。
2. **measure 开始时**：100人逐个真实登录（不超过登录限流）并调用 `/api/me` 核对身份；以并发不超过10建立100条socket并完成auth与issue订阅；逐人权威GET；读取参与者清单。任一 `subscribe_error`、认证错误、关闭或ACK超时都使该观察者未就绪，整场NOT_RUN，不悄悄剔除。

## 3. 状态传播协议

**轮次**：5次预热由actor 0执行；正式第k轮（k=0…99）由actor k执行。k≥1时先由当前有权者经公开 `handoffs`（`purpose=transfer`）把处理人转给actor k，转交回执单独记录，不进入状态分母。状态在 `todo` / `in_progress` 间交替，每次取写入者自己的 `status-options` 版本。

**静止屏障**：每轮计时写之前，必须等100个观察者都收到上一条事件（转交或上一轮状态）的 `collaboration:changed`，或到10秒截止并记录。然后写入者做一次权威GET，记录 `before_revision` 与 `before_event_seq = S`。因为本工具是该任务唯一写入者且已静止，本轮状态事件序号必为 `S+1`。

**相位抖动**：屏障总在某次中继扫描之后不久结束，若立即写入，写入相位会被锁定在500ms周期的固定位置，使分布人为变窄。因此计时写之前等待一个取自带种子伪随机序列的 `[0,500)` 毫秒延迟，种子与每轮延迟写入manifest和样本。

**关联**：先挂好观察等待再发写；事件可能早于写响应到达，缓冲并保留其原始时刻。只有 `type==="collaboration:changed"`、`payload.issue_id` 相符、`payload.event_seq === S+1`、`event_id` 以 `:issue` 结尾的帧才启动该观察者的读取。`workbench_changed`、其他序号的帧、未知帧都不启动判定。帧内 `collaboration_revision` 记录并与回执比较，不一致标记 `correlation_anomaly`，不作匹配条件。重复帧按 `event_id` 去重并计数。

**终点**：该观察者以自身token GET，读到 `status===目标状态`、`collaboration_revision>=回执版本`、`realtime.event_seq>=S+1`。读到旧值时可在截止前重读，`read_attempts` 与首读是否陈旧都记录。没有匹配帧就没有成功样本：不以纯轮询代替；断线重连后读到的状态另记 `recovered_by_reconnect_read_ms`，样本仍为缺失。

**时钟与截止**：全部时间差来自同一进程的 `process.hrtime.bigint()`，UTC只用于关联日志。每轮截止10秒；2–10秒的样本记实际耗时并判超预算。断线只在轮次边界重连（重新auth与订阅），断线期间的样本为缺失。工具以 `perf_hooks.monitorEventLoopDelay` 记录自身事件循环延迟，只报告，不从样本中扣除。

**样本字段**：`run_id, phase(warmup|formal), event_index, writer_user_id, request_id, issue_id, before_revision, before_event_seq, expected_event_seq, intended_status, pre_write_jitter_ms, request_start_ms, write_ack_ms, write_http_status, receipt_revision, event_id, frame_revision, observer_user_id, socket_id, first_event_ms, read_start_ms, read_end_ms, observed_revision, observed_event_seq, observed_status, latency_ms, event_latency_ms, read_attempts, first_read_stale, duplicate_frames, result, error_code, deadline_ms`。

**统计**：`rank = ceil(N×95/100)`（整数运算），取升序第rank个；未收敛、写失败、准备失败、NOT_RUN一律按+∞进入排序。报告pooled p50/p95/p99/max、每事件最慢观察者的分布、每观察者分布、去掉写入者本人的9900样本子集，以及写响应、事件到达、读取三段耗时。判定只按PRD P03三项。

**失败处理**：写请求非200或结果不明时，按原键GET恢复其结果；该事件100个样本记未收敛，不在同一事件序号上换键重发。状态已知则继续下一轮，否则中止并把余项记NOT_RUN。

本协议度量的是串行写入下API状态收敛，不含并发写入负载、React Query或像素绘制。最终环境可另开少量真实Web/Electron页面测DOM可见，单列且不外推为100个页面。

## 4. 结果回写协议

在第3节同一任务（或参与者清单完全相同的另一条任务，manifest注明）上执行，100条观察socket保持连接。actor 0 持有执行workspace、受控runtime与Agent，用 `enrollHostRuntime`、`startHostDaemon`、`adoptHostTarget` 完成公开登记、采纳与同意，再以公开 `control` 接口Resume。仅此阶段调用 `openHostAcceptanceGate`，结束时恢复。

1个预热run加100个正式run，run k由actor k以带提及与当前 `target_versions` 的评论触发，上一个run终结后才发下一条。每个run记录：

- **排队/启动**：评论POST发出前到工具观察到该run的Provider回调（以自身时钟细粒度观察 `visits`）。单列，不参与5秒判定；说明测试poller间隔50ms不代表生产daemon。
- **业务结果回写**：先以 `request_id` 读评论确认404。`t0` 在调用 `act(visit,[{kind:"comment",request_id,content}])` 之前；同时由另一名参与者（既非触发者也非资源所有者）每25ms读 GET `…/comments/{request_id}`。终点是第一次返回200且内容摘要、`author_type==="agent"`、`author_agent.agent_id` 均相符的读取完成时刻，同时记录上一次读取的开始时刻以给出可见区间。`act` 自身的resolve时刻不作终点；回执的 `request_id` 事后核对，run记录复核该评论归属此run。
- **终结回写**：`t1` 在 `release(visit)` 之前；终点是同一轮读取中任务详情显示该run `progress_state==="settled"`、`proof_state==="host_exit_observed"`、`launch_count==="1"`、`business_outcome==="success"`，且 `…/runs/{runId}/records` 出现 `Host fixture result for <runId>`，取两者中较晚的完成时刻。
- **提示路径（次要）**：一名已订阅观察者从 `t0` 到收到该评论事件帧并权威读到评论的耗时，单列。

每项样本截止30秒，预算5秒；超时、拒绝、重复启动都保留为未收敛。`act` 或daemon夹具报错时中止本阶段，余项NOT_RUN。统计方法同第3节，判定按PRD P04。daemon内部的 `started_ms` 等时刻来自另一个时钟原点，不与工具时钟相减。

**runtime存活（r01演练后补充）**：既有host夹具不发送runtime心跳，而API的runtime清扫器在 `service.RuntimeClaimFreshnessSeconds`（150秒，`server/cmd/server/runtime_sweeper.go`）后把runtime标为离线。100个串行run持续数分钟，r01在登记约3分钟后的第72个正式run处得到201回执却无Provider启动（见 `research/rehearsal-r01.md`）。工具必须按生产daemon的方式、以远低于阈值的间隔为该runtime发送心跳，不得改夹具、改清扫器、中途重新登记runtime或缩短阶段；每次心跳的时刻与状态落盘，每个run触发前记录runtime在线状态，离线时触发的run以专门错误码（如 `runtime_offline`）记为未收敛，而非笼统的 `no_provider_launch`。心跳由actor 0（runtime所有者）发出，不在任何计时区间的关键路径内，也不由读取者账号发出。其他随时间到期的凭证、租约与状态同样逐项核对并在报告中列出。

## 5. 环境

遵循 `.trellis/spec/server/database-test-isolation.md`、`verification-environment.md` 与 `.trellis/spec/web/frontend/e2e-run-environment.md` 的password启动段。

- 新的仓库外运行目录与新数据库（名称由分配记录给出，如 `multica_pay241_s5_<编号>_api`），迁移前核对 `current_database()`；不复用S1/S3的库、端口、配置或人用环境。S1/S3私有目录里的 `run.mjs`、`allocate.mjs`、`start-service.mjs` 只作为拷贝后重新绑定的参考。
- 冻结的源码副本（提交加明确的overlay文件清单），由它构建API、migrate、CLI、daemon测试二进制（`go test -c ./internal/daemon`）、`collaboration-host-provider.go`，最终验收再加production Web与Desktop renderer。运行中的根工作区有他人未提交改动，不直接从它构建。
- API：password模式、关闭设备认证、清空Cloud URL、单进程password limiter、不设 `REDIS_URL`（单节点内存hub）、`ALLOW_SIGNUP` 不为false。`DATABASE_MAX_CONNS` / `DATABASE_MIN_CONNS` 不设置，使用产品默认（最大25），从启动日志记录实际值；连接预算不足时先停掉本任务其他空闲API，不得调高到默认以上后仍报告为默认配置。
- 运行前后记录：`uptime` 负载、CPU/内存、两进程的描述符上限、PostgreSQL版本、容器镜像架构、`max_connections` 与按库聚合的连接数（不含SQL文本）、并行会话。共享集群上的外来负载只会使结果变差；受污染的运行保留并以新编号重跑，不删除。
- 门禁：分配时记录 `collaboration_execution_gate` 原值；状态传播阶段不改门禁，只断言 `creation_enabled`；回写阶段由夹具打开并恢复；结束后以只读核对与分配原值一致、epoch单调。
- 工具自带环境守卫：要求显式 `COLLABORATION_PERF_ACCEPTANCE=1`、运行目录与manifest路径、loopback的API与数据库、非默认库、`/api/config` 为password且部署ID相符、三个受控二进制为绝对路径。未设置启用键时三个测试全部skip，保证 `make check` 或整目录Playwright运行不会触发它；**已显式启用但其余任一项不合格时测试失败而不是skip**（实施偏离，主会话2026-11-14采纳：显式要求运行却静默跳过会把环境错误伪装成"未执行"）。
- 连接池与部署形态由manifest声明并在运行时核对：声明的连接池上限超过产品默认25、配置了Redis或多于一个副本时工具拒绝运行（实施偏离，同日采纳），保证结果只描述本节规定的单节点默认形态。

## 6. 产物

运行目录内：`manifest.json`（预登记的分母、预热数、截止、种子、并发、源码/构建/配置哈希、环境规格）、增量写入并flush的 `propagation-samples.jsonl` 与 `writeback-samples.jsonl`、`setup-identities.json`（无密钥）、按阶段分开的 `summary-<phase>.json` 与 `failures-<phase>.json`（实施偏离，2026-11-14采纳：三个阶段分别运行、分别失败，单一summary会被后一阶段覆盖）、门禁与资源前后快照。任务目录只收summary、索引与哈希；token、密码、含凭证的daemon账本不入库。结束顺序：关闭socket → 等待受控run终结 → 关闭daemon → 恢复门禁 → 停止本任务自有进程，保留数据库与失败证据。

## 7. 文件归属

仅新增，不修改他人文件：

- `e2e/collaboration-performance.spec.ts`：canonical套件，含 prepare、propagation、writeback 三个带守卫的测试，各自设置足够的超时。
- `e2e/fixtures/collaboration-performance.ts`：环境守卫、账号/会话、观察者、样本日志；只导入 `TestApiClient` 与host夹具的既有导出。
- `e2e/fixtures/collaboration-performance-stats.ts`：纯函数（分位数、关联状态机、分母核算、判定），不依赖Playwright，只用可擦除的类型语法。
- `e2e/fixtures/collaboration-performance-stats.node-test.mjs`：`node --experimental-strip-types --test` 运行的反例测试。
- 实施后的规范新文件 `.trellis/spec/web/frontend/collaboration-performance-qa.md` 由实施/规范更新步骤添加。

不触碰 `e2e/fixtures.ts`、`e2e/fixtures/collaboration-host-runtime.ts`、closure夹具、Provider源文件、`playwright.config.ts`、`server/`、`packages/`、`apps/` 与CI配置。若确实缺少必要的测量钩子，先向主会话报告，不为计时给产品或共享夹具加开关。e2e不在 `pnpm typecheck` 范围内，静态检查沿用S3 timeout-proof记录的定向tsc/ESLint方式。

## 8. 依赖

- 现在可做：四个工具文件、反例测试、在冻结当前源码副本上的演练（结果标注源码版本，仅证明工具可执行）。状态传播只用既有公开API；回写依赖的host夹具已随 `b1d2e54ea` 提交。
- 须等待：最终验收运行要在S2（阶段门禁会进入workflow事务与事件）、S3其余范围（实时补读与重认证）、S4所需变更稳定后重跑，旧样本不继承到新源码。P06依赖S2–S4各自证据，P07依赖真实人员。

## 9. 限制

受控Provider与测试账号不代表真实模型或业务人员。同机loopback、单节点内存hub的结果不外推到跨地域、多副本或生产容量。API收敛不等于100个页面的渲染性能。
