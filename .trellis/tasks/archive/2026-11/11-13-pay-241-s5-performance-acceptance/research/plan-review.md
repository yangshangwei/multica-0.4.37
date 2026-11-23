# S5 性能与总验收：独立规划审查

结论：**采样工具可以开始实现，阻挡项0。** 这是规划审查，不是任何测试的通过证据；本次没有启动服务、连接数据库、调用模型或执行Git写操作。最终验收运行与24例核销仍依赖S2–S4。

- 审查人：原生 `trellis-check`（由主会话为S5显式派发；hook活动任务为S2，未改任务指针）。
- 日期：2026-11-14。源码：HEAD `fb9f4c070`，工作树另有四百余项他人未提交条目；本文引用的全部源码文件在审查时相对HEAD无改动（已逐个用 `git status` 核对）。S2正在修改服务端文件，行号可能漂移，实施时按符号定位。
- 修改范围：仅S5的 `prd.md`、`design.md`、`implement.md`、`implement.jsonl`、`check.jsonl`，并新增本文件与 `plan-review.json`。`research/performance-harness.md` 未改，其勘误见第4节。100名独立观察者、完整样本与p95目标没有放宽。

## 1. 逐项核实结果

### 1.1 100个独立认证用户与各自的WS会话

可行，全部走公开入口，无需直接写用户或参与者表。

- 注册/登录：`TestApiClient.registerPassword`（`e2e/fixtures.ts:71`）、`loginPassword`（`:56`）。`PasswordRegister` 每IP 30次/5分钟（`server/internal/handler/auth_password.go:132`）并要求 `AllowSignup`（`:135`，`ALLOW_SIGNUP!="false"`，`router.go:438`）；登录每IP 300次/分钟、每账号10次/5分钟（`:176`、`:191`）。这两条路由没有挂 `authRL`（`router.go:1482–1483`）。100人注册需四个窗口，至少约15分钟；密码为PBKDF2 60万次迭代，注册并发应保持1–2。token默认30天（`auth/cookie.go:23`）。
- 参与者：`InviteParticipant`、`Transfer`（`service/collaboration_workflow.go:92`、`:112`）。协作邀请不经过 `InvitationRateLimiters`（那是workspace邀请），未发现人数上限；资格只要求账号启用、有密码凭据、无强制改密（`queries/collaboration_recipients.sql:13–15`），不要求onboarding。recipient resolve/search有限流，工具直接用注册返回的user_id，不调用它们。
- WS：`/ws`（`router.go:1384`）。`scope=account` 不得带workspace参数（`hub.go:856`）；无cookie时首帧认证，10秒读截止（`firstMessageAuth`，`:802`），成功回 `auth_ack`（`:955`）；`handleSubscribe`（`:1107`）对issue scope做当前授权后回 `subscribe_ack`（`:1173`）。每连接发送缓冲256（`:983`），满则 `evictSlow`（`:697`）；未发现每用户或全局连接数上限。`writePump`（`:1200`）每帧先 `c.authorized()` 再 `outboundScopeCurrent`（`realtime/collaboration.go:68`），即每帧都有数据库授权查询。hub不为断线客户端保留或重放帧。
- Node原生WebSocket：本次用一次性loopback socket实测Node 22.23.2的握手头，只有 `host/upgrade/sec-websocket-version/user-agent`，**不发送Origin**，满足 `checkOrigin`（`hub.go:161`）。仓库里此前没有Node原生WS客户端的先例（现有e2e都通过浏览器页面观察WS）。
- 帧与权威读取：中继每500ms扫描至多100条（`service/collaboration_realtime.go:65–68`）；每条事件先给每个参与账号发空的 `collaboration:workbench_changed`（`:134–`），最后发 `collaboration:changed`（`:155`），`event_id` 由 `PublishWithID` 注入（`realtime/collaboration.go:177`）。帧只有 `issue_id/event_seq/collaboration_revision/authority_epoch`，不含状态，必须以自身token GET `/api/collaboration/issues/{id}`（`router.go:1686`；`realtime.event_seq` 见 `handler/collaboration.go:177`）。

### 1.2 测量定义

时钟、起点（写请求发出前）、终点（匹配帧后的自身权威读取）、完整分母、+∞处理的方向是对的。发现以下会使数字好于实际或使关联出错的问题，均已写入PRD/design：

| ID | 问题 | 证据 | 处理 |
| --- | --- | --- | --- |
| F01 | 帧内 `collaboration_revision` 是中继时的当前版本，不是事件发生时的版本。转交事件若在状态写提交后才被中继，会带着状态写之后的版本；按版本匹配会把转交帧当成状态事件 | `queries/collaboration_realtime.sql:3–12` 取的是 `c.revision` | 只按 `event_seq` 精确匹配；版本仅记录并比对 |
| F02 | 状态写回执没有 `event_seq`，原方案未说明期望序号从哪里来 | `finishStatusChange`（`service/collaboration_lifecycle.go:610–631`）回执只含code与revision；一次状态变化恰好一条事件 | 每轮写前要求静止，写入者权威GET取 `S`，期望 `S+1` |
| F03 | 相位锁定：上一事件总在某次500ms扫描后不久送达，紧接着写入会使每轮落在周期的同一位置，分布被人为压窄 | `collaboration_realtime.go:65` | 写前加带种子的 `[0,500)`ms 抖动并逐轮记录 |
| F04 | 只判pooled p95时，每个事件都可以有5名观察者超时而整体仍通过 | 父任务要求的是100名协作者的同步 | 增加“每事件最慢观察者耗时的p95≤2秒”，与pooled p95、零缺失同时判定 |
| F05 | 分位数算法与缺失处理未写成可测的定义 | `e2e/projects-p1.spec.ts:69` 是nearest-rank先例 | `rank=ceil(N×95/100)` 整数运算，未收敛为+∞；反例测试先行 |
| F06 | 可通过调大连接池、挑选运行、事后改预热数来改善结果 | `cmd/server/dbstats.go:36` 默认最大25 | 连接池用产品默认并记录实际值；每次运行新编号、全部保留；预热数预登记 |
| F07 | 断线后的处理未定义 | hub无重放 | 只在轮次边界重连，断线期间样本为缺失，重连后读到的值另记 |
| F08 | 只有当前发起人或处理人能改状态，“每人发起一次”依赖转交，转交事件会与状态事件交错 | `CapabilitiesFor`（`collaboration/access.go:82–100`） | 静止屏障等上一条事件送达全部观察者后才计时 |

资源记录：manifest要求CPU/内存/负载、两进程描述符上限、连接池实际值、PostgreSQL版本/架构/`max_connections`/并存连接、是否Redis与多副本、工具自身事件循环延迟。事件循环延迟只报告不扣除。审查时本机10核16GB，负载3.5–4.7，说明有并行会话，最终运行必须记录当时负载。

### 1.3 Agent回写

- 可用夹具：`startHostDaemon`、`act`、`release`（`e2e/fixtures/collaboration-host-runtime.ts:227`、`:420`、`:401`）加 `collaboration-host-provider.go`（动作循环 `:364–391`，评论经run-local CLI `:57–62`，最终结果 `:394`）。评论ID等于 `request_id`（`CreateRuntimeComment`，`service/collaboration_squad.go:252`），所以按ID读取 GET `…/comments/{commentId}`（`router.go:1738`）成立。
- F09：`--fixture-kind` 在host、closure、memory三个Provider里输出相同（`collaboration-host-provider.go:27`、`collaboration-multi-account-provider.go:25`、`collaboration-memory-provider.go:25`），`startHostDaemon` 的种类检查（`:243–244`）无法区分。closure Provider在回调前已写完评论（`collaboration-multi-account-provider.go:188–209`），误用会得到近零耗时。处理：manifest绑定Provider源与产物哈希，预热run做404预检。
- F10：原方案20个run各5条。终结回写只有20个样本，p95等于第19个，只容一个坏样本；同一run内的5条共享进程与连接。处理：改为100个独立run各一条业务结果与一次终结，两项各100个样本、各自p95≤5秒。仍是100次结果回写，没有放宽。
- F11：在两三人的小任务上测回写会低估PAY-241场景。处理：回写在同一组100名参与者的任务上执行，100条观察socket保持连接。
- F12：`sendHostComment`（`:707`）、`seedHostRuntime`（`:520`）与 `hostEnvironment`（`:92`）需要浏览器页面或Web/renderer环境键。纯API路径已有先例：`e2e/collaboration-installed-clients.spec.ts:16–22`（control）与 `:59–71`（带 `target_versions` 的提及评论）。工具自带环境守卫，不调用这三个函数。
- F13：`act` 用 `expect.poll` 等回执（`:424`），粒度粗；终点必须取并行权威读取的时刻。测试poller `PollInterval` 为50ms、并发1（`server/internal/daemon/collaboration_host_router_test.go:179`），排队/启动耗时不代表生产daemon，单列且不参与判定。
- 归档的S3 timeout-proof报告证实同一套host夹具在password API、production Web与renderer上两次端到端通过（各约30秒），可作为环境搭建参照，不是性能证据。

### 1.4 环境规则

- F14：`e2e/fixtures.ts:14` 在 `DATABASE_URL` 缺失时回落到本机 `multica` 库，`e2e/env.ts` 会从cwd加载 `.env`；`playwright.config.ts` 的 `testDir` 是整个 `e2e/`，默认超时60秒。新spec若无显式开关，会被整目录运行或 `make check` 带起。处理：工具自带守卫（显式启用键、loopback、非默认库、`/api/config` 核对），未启用即skip；各测试自行设置超时。
- 门禁：状态传播不需要改门禁；回写阶段用 `openHostAcceptanceGate`（`:115`），它恢复的是进入时的状态，所以另需在分配时记录原值并在结束后只读核对。
- 数据库与进程：新运行目录、新数据库、冻结源码副本，不复用S1/S3或人用环境；`/Users/artisan/.multica/acceptance/pay241-20261113-01` 下的 `run.mjs`、`allocate.mjs`、`start-service.mjs` 存在，可拷贝后重新绑定。PostgreSQL容器 `multica-postgres-1` 为各任务共享，连接预算与外来负载必须记录。

### 1.5 依赖

- 不依赖S2–S4、现在可做：四个工具文件、反例测试、在冻结当前源码副本上的演练。状态传播只用既有公开API；回写依赖的夹具已随 `b1d2e54ea` 提交。
- 依赖S2–S4：最终验收运行（S2会在workflow事务与事件中加入阶段校验，S3其余范围涉及实时补读与重认证，S4涉及集成事件），以及P06的24例核销。P07依赖真实人员。

### 1.6 上下文清单

原两份jsonl的五个路径均存在且相关。已补充：`collaboration-closure-qa.md`（Provider区分与门禁配置）、父任务 `design.md`、本审查；check另加 `collaboration-host-execution.md`、父任务 `test-matrix.csv` 与 `gaps.md`。`e2e-run-environment.md`（约45KB）未加入以保留上下文预算，design第5节引用其password启动段。全部路径已核对存在。

## 2. 工具文件归属

只新增，不与S2的server/core/views产品文件或其E2E文件重叠：

| 文件 | 内容 |
| --- | --- |
| `e2e/collaboration-performance.spec.ts` | canonical套件：prepare、propagation、writeback，带显式守卫 |
| `e2e/fixtures/collaboration-performance.ts` | 环境守卫、账号/会话、观察者、样本日志、资源快照 |
| `e2e/fixtures/collaboration-performance-stats.ts` | 分位数、关联状态机、分母核算、判定（纯函数） |
| `e2e/fixtures/collaboration-performance-stats.node-test.mjs` | `node --experimental-strip-types --test` 的反例测试（先例：`collaboration-installed-main.node-test.mjs`） |

不触碰 `e2e/fixtures.ts`、`collaboration-host-runtime.ts`、closure夹具、Provider源、`playwright.config.ts`、`server/`、`packages/`、`apps/`、CI。实施完成后的规范新文件 `.trellis/spec/web/frontend/collaboration-performance-qa.md` 属于后续规范更新步骤。

## 3. 待主会话确认的选择

没有阻挡实现的产品决定。以下三项已按较严格的一侧写入文档，主会话如不同意可调整，但不得低于父任务要求：

1. **同步p95的判定统计量**：已写为pooled与每事件最慢观察者两项同时≤2秒。建议保留；若只判pooled，需要用户明确接受“每个事件可有5人超时”。
2. **回写样本结构**：已写为100个独立run各一条。建议保留；多出的运行时间估计在十分钟量级（未实测）。退回20×5会使终结回写只有20个样本。
3. **PostgreSQL集群**：建议沿用共享集群并记录余量与负载，与S1/S3及隔离规范一致；只有当一次完整运行被外来负载明显污染时，才考虑由主会话另行安排并以新编号重跑。

## 4. 对原研究的勘误

- `collaboration-host-runtime.ts` 行号已过期（研究写于S3修改该文件之前）：`openHostAcceptanceGate` 现为115，`enrollHostRuntime` 185，`startHostDaemon` 227，`release` 401，`act` 420，`createHostActor` 462，`adoptHostTarget` 485，`seedHostRuntime` 520。
- “用精确revision关联”应为“用 `event_seq` 关联”（F01）。
- “20个run各5条”改为“100个run各一条”（F10）。
- `createHostActor` 与公开创建任务可纯API使用，但触发运行不能用 `sendHostComment`（F12）。

## 5. 未核实

- 没有对真实hub建立过连接：100条Node原生socket的认证、订阅、每帧授权在25连接池下的实际耗时均未测。
- 没有执行过 `status-options`、PATCH状态、转交、邀请的实际请求；外部处理人读取 `status-options` 的成功路径只读了代码。
- 100名参与者时workflow事务锁定全部参与者用户行的耗时与冲突未测。
- 终结回写是否受daemon轮询间隔影响没有逐行追踪；生产daemon的默认轮询间隔未查。
- 没有查询PostgreSQL当前的 `max_connections` 与连接数（未触碰数据库）。
- S2进行中的改动对workflow事务、路由与迁移的影响未评估，只确认了它默认关闭且不改状态键。
- 少量真实Web/Electron页面的DOM测量路径、`e2e-run-environment.md` 的完整内容、S4的任何外部环境均未核对。
- 没有运行lint、typecheck或任何测试：本次没有代码改动。
