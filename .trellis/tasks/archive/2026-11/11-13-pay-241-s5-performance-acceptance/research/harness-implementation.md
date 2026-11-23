# S5 采样工具实现记录

- 日期：2026-11-14。实施者：原生 `trellis-implement`（由主会话为 S5 派发）。
- 源码基线：HEAD `fb9f4c070`（工作树另有他人未提交条目，未触碰）。
- 范围：implement.md 第 1 节 1.1–1.5。只新增四个文件，没有修改任何已有文件，没有执行 git 写操作。
- 结论：四个文件已完成并通过反例测试、定向 tsc/ESLint 与未启用时的 skip 核对。**没有任何一步连接过真实 API、hub、数据库、daemon 或模型**；本文件不是 P01–P07 的通过证据，TC-24 与父任务状态不变。

## 1. 文件与哈希

| 文件 | 行数 | SHA-256 |
| --- | --- | --- |
| `e2e/collaboration-performance.spec.ts` | 68 | `87ba4d0916c11102be385e811981062315d9c640aa3eb7740bb391665647d175` |
| `e2e/fixtures/collaboration-performance.ts` | 1247 | `1b043991fa58fcd4a2e406577ea40a2339325941de740dd5cad14f6ea6cb3579` |
| `e2e/fixtures/collaboration-performance-stats.ts` | 404 | `9b8724218c3ccbd6f6a60715c7fcbed08ebabb0c1ccd63577d34377308171683` |
| `e2e/fixtures/collaboration-performance-stats.node-test.mjs` | 353 | `f8369bd56dd3da57892f28a10656be8eea15dd59505294c30845272bc77f764e` |

只读导入的既有文件：`e2e/fixtures.ts`（`TestApiClient`）、`e2e/fixtures/collaboration-host-runtime.ts`（`adoptHostTarget`、`enrollHostRuntime`、`hostAuthority`、`hostMode`、`issueRoot`、`openHostAcceptanceGate`、`startHostDaemon`、`versioned`、`waitForHostRuntime`）。没有导入 `sendHostComment`、`seedHostRuntime`、`hostEnvironment`。没有新增依赖（Node 原生 `fetch`/`WebSocket`/`process.hrtime`，既有 `pg` 与 `@playwright/test`）。

## 2. 执行过的命令与退出码

日志目录 `L=/Users/artisan/.multica/acceptance/pay241-20261113-01/logs`，静态检查目录 `S=/Users/artisan/.multica/acceptance/pay241-20261113-01/s5-harness-static`。Node 均为 `/Users/artisan/.nvm/versions/node/v22.23.2/bin/node`（v22.23.2），cwd 为仓库根。

| # | 命令 | 退出码 | 日志 |
| --- | --- | --- | --- |
| 1 | `node --experimental-strip-types --test e2e/fixtures/collaboration-performance-stats.node-test.mjs`（函数为抛出 `not implemented` 的骨架） | 1（20 项全部失败） | `$L/s5-harness-stats-red-1.log`，骨架副本 `$L/s5-harness-stats-red.skeleton.ts.txt` |
| 2 | 同上（收紧一条反例后仍为骨架） | 1（20/20 失败） | `$L/s5-harness-stats-red-2.log` |
| 3 | 同上（实现后首次） | 1（19 通过，1 失败） | `$L/s5-harness-stats-green-attempt1.log` |
| 4 | 同上（修正该条反例自身的错误后） | 0（20/20） | `$L/s5-harness-stats-green-attempt2.log` |
| 5 | `node node_modules/typescript/bin/tsc --project $S/tsconfig-e2e.json --pretty false`（首次） | 2（5 个错误） | `$S/typecheck-attempt1.log` |
| 6 | 同上（修正后） | 0 | `$S/typecheck-attempt2.log` |
| 7 | `node node_modules/eslint/bin/eslint.js --config $S/eslint.config.mjs --max-warnings 0 --no-cache <四个文件>`（首次） | 0，无输出 | `$S/eslint-attempt1.log` |
| 8 | `node $S/run-static.mjs final`：tsc `--listFiles`、tsc `--showConfig`、ESLint、四次 `--print-config`、反例测试，并核对检查前后四个文件哈希未变 | 0 | `$S/commands-final.json`（完整 argv、生效配置、退出码、日志哈希） |
| 9 | `env -u COLLABORATION_PERF_ACCEPTANCE -u COLLABORATION_PERF_RUN_DIR -u COLLABORATION_PERF_MANIFEST pnpm exec playwright test e2e/collaboration-performance.spec.ts --list` | 0（3 tests in 1 file） | `$L/s5-harness-playwright-list.log` |
| 10 | 同样的环境，`pnpm exec playwright test e2e/collaboration-performance.spec.ts --workers=1 --retries=0 --reporter=list` | 0（3 skipped，0 运行） | `$L/s5-harness-playwright-not-enabled.log` |

退出码均由 `echo $?` 直接取得，没有经过 tail/head 管道。命令 3 的失败是反例测试自己写错（`pickWritebackReaders(0,0,3)` 实际合法），实现未改；命令 5 的 5 个错误是 `invite` 可能为 undefined（3 处）、未使用的解构变量、对象字面量重复键，均已修正。两份首次失败日志原样保留。

`run-static.mjs final` 的结果：tsc 0 个错误，检查了 249 个源文件，其中 e2e 下 7 个（`env.ts`、`fixtures.ts`、`collaboration-disclosures.ts`、`collaboration-host-runtime.ts` 与三个新 TypeScript 文件）；ESLint 四个文件 0 错误 0 警告（生效规则数 91/91/91/84）；工具版本 TypeScript 5.9.3、ESLint 9.39.4。方法与归档的 S3 `targeted-static-check.json` 相同：`extends packages/tsconfig/react-library.json`、显式 `files`、`types: ["node"]`；ESLint 复用 `packages/core/eslint.config.mjs` 并把 `import-x/no-extraneous-dependencies` 的 `packageDir` 指向仓库根。没有运行全仓 `pnpm typecheck`/`pnpm lint`/`pnpm test`：e2e 不在其范围内，且没有改动任何已有文件。

## 3. 命名测试清单

### 3.1 反例测试（`node:test`，20 项：通过 20，失败 0，跳过 0）

1. nearest-rank uses the integer rank ceil(N*95/100) for N=100, 10000 and 20
2. a missing sample enters the percentile as +Infinity instead of shrinking the denominator
3. more than five percent missing makes p95 unbounded
4. any missing sample fails the sync gate even when every percentile is finite and fast
5. a duplicate event_id is counted and never yields a second arrival
6. only collaboration:changed with exactly sequence S+1 for this issue matches
7. a frame that arrives before the write response keeps its original arrival time
8. without the matching frame an authoritative read alone is never a converged sample
9. warm-up rows are retained but never enter or replace the formal denominator
10. an aborted run keeps the 10,000 denominator and reports the remainder as NOT_RUN
11. a failed write or failed preparation marks all 100 samples of that event non-converged
12. pooled p95 and per-event slowest-observer p95 are judged separately
13. accounting refuses duplicated pairs, impossible indices and relabelled late samples
14. the two write-back gates are independent and each needs a complete denominator
15. phase jitter is seeded, reproducible, inside [0,500) and not phase locked
16. registration pacing never plans more than 30 attempts in one server window
17. write-back readers are never the triggering account or the resource owner
18. the quiescence barrier accepts a delivered frame or a snapshot that already covers the sequence
19. the relay-time revision in a frame is recorded as an anomaly, never used to match
20. a manifest that changes the pre-registered protocol is rejected

### 3.2 Playwright 套件（未启用：跳过 3，运行 0）

- `PAY-241 S5 collaboration sampling › prepare: 100 password accounts join one collaboration issue through public routes`
- `… › propagation: 100 observers each see 100 status changes on their own socket and token`
- `… › writeback: 100 controlled runs each return one business result and one finalization`

### 3.3 额外的工具自检（仓库之外，对进程内假 hub；不是产品证据）

为了不让 1247 行夹具第一次执行就发生在协调者的演练里，我在 `/Users/artisan/.multica/acceptance/pay241-20261113-01/s5-harness-selftest/` 写了一个进程内假服务（`fake-hub.ts`，Node `http` 加仓库已有的 `ws`，监听 127.0.0.1:39412/39413，测试结束即关闭）和两份草稿 spec。假服务按我对服务端源码的阅读实现注册限流（对齐墙钟的 30 次/5 分钟）、首帧认证、issue 订阅、500ms 中继、`workbench_changed` 加 `collaboration:changed`、状态写与转交。**它证明的是工具自身的节流、关联、分母与落盘逻辑，不证明产品行为，也不产生任何性能结论。** 这些文件不在仓库内，也不属于交付物。

| 场景 | 结果 | 日志 |
| --- | --- | --- |
| guard 拒绝矩阵：缺 opt-in、opt-in 不是 `1`、缺 `DATABASE_URL`、默认库 `multica`/`postgres`、非 loopback 的库与 API、库名/API/部署与 manifest 不符、运行目录在仓库内、目录权限 0755、协议被改（观察者 50、截止 60 秒）、连接池 100、Redis、manifest 在目录外、write-back 缺二进制；未 prepare 时测量拒绝 | 通过（首次因假服务 `ws` 导入写法失败，见 attempt1） | `$L/s5-harness-selftest-guard-attempt1.log`、`-attempt2.log`、`$L/s5-harness-selftest-run2.log` |
| prepare：100 个账号，假服务从未返回 429（工具自己把每窗口控制在 30 次内），状态文件 0600，身份清单与日志不含密码，重复执行不再注册 | 通过，11.8 分钟 | `$L/s5-harness-selftest-run2.log` |
| 干净的 propagation：10500 行，正式 10000 全部收敛，三项条件为真，100 个不同写入者与 100 个不同 socket，抖动覆盖整个 500ms 周期，重复启动被 `EEXIST` 拒绝 | 通过，2.4 分钟 | 同上 |
| 故障注入：丢 1 帧、拒绝 1 次写（409）、断 1 条 socket、1 次中继延迟 2.6 秒 | 通过：收敛 9898，缺失 102（`no_matching_frame` 1、`write_http_409` 100、`socket_closed` 1），慢事件 100 个样本以真实耗时保留，断线者在轮次边界重连且样本仍为缺失，判定为不通过 | 同上 |
| 写结果不明（连接被掐断且无操作记录） | 通过：`aborted_after_formal_3_state_unknown`，收敛 300、缺失 100、NOT_RUN 9600，分母仍为 10000，p95 为无界，没有换键重发 | `$L/s5-harness-selftest-abort-attempt1.log` |

第一次后台启动（`s5-harness-selftest-run1-cancelled-by-author.log`）是我自己在 3 秒后取消并改为脱离终端运行的，不是失败。自检用独立脚本重算了干净场景的 pooled 与每事件最慢观察者分位数，与工具输出逐位一致。自检未覆盖 `runWriteback`、`withPhaseEvidence`、`verifyLiveEnvironment`、`readGate`、`captureResources`（它们需要真实数据库或 daemon）。

## 4. PRD 条目在代码里的落点

| 条目 | 文件 › 符号 | 做法 |
| --- | --- | --- |
| P01 | `collaboration-performance.ts` › `prepareRun`、`openObserverSessions`、`connectObserver`、`verifyParticipants` | 逐人 POST 公开 `/auth/register`；测量期逐人 `TestApiClient.loginPassword` 加 `/api/me` 核对；各自一条 `/ws?scope=account`，首帧 auth → `auth_ack` → `subscribe` → 同一 issue 的 `subscribe_ack` → 自身 token 的权威 GET 且 `capabilities.read===true`。断言 100 个不同 user_id、token、socket_id，参与者清单恰为这 100 人且 `account_active===true`。任何一项不满足即抛出 `NOT_RUN`，此时样本文件尚未创建。工具里没有 INSERT/UPDATE/DELETE，数据库只读（门禁、`current_database()`、连接计数）。 |
| P02 | `-stats.ts` › `PERFORMANCE_PROTOCOL`、`accountPropagation`、`failedEventRows`、`notRunEventRows`；`runPropagation` 的 `plan` | 5 次预热由 actor 0 写，正式第 k 轮由 actor k 写；每轮追加 100 行并 fsync。预热保留但不进正式分母；把正式样本改标为预热只会留下按 NOT_RUN 计的空洞；预热行超过 5 个事件即报错。中止后余项显式写 NOT_RUN，文件截断时由核算补齐。 |
| P03 | `-stats.ts` › `propagationVerdict`、`classifyObserverSample`、`nearestRankIndex`；`runPropagation`、`observeEvent`、`writeStatus` | 起点 `requestStart` 取在发出 PATCH 之前；终点是该观察者匹配帧之后、自身 token 权威 GET 读到目标状态、`realtime.event_seq>=S+1`、版本不低于回执的那次读取完成时刻。三项条件同时成立才通过。写响应、事件到达、读取分段只进 `segments_informational`。每个观察者的分布与超预算者清单写入 summary。 |
| P04 | `runWriteback`、`pollUntil`、`resumeExecution`；`-stats.ts` › `accountWriteback`、`writebackVerdict`、`pickWritebackReaders` | 1 个预热 run 加 100 个正式 run，在同一条 100 人任务上、100 条 socket 连接着执行。业务结果 `t0` 在 `host.act` 之前，终点是另一名参与者（非触发者、非资源所有者）首次读到该评论且正文、`author_type`、`author_agent.agent_id` 相符；并要求该评论出现在此 run 的 records 里。终结 `t1` 在 `release` 之前，终点是同一轮读取里任务详情四项状态与 records 的结果正文同时成立。两项各自 100 个样本、各自判定。排队/启动耗时与提示路径只报告。 |
| P05 | `withPhaseEvidence`、`captureResources`、`runPropagation`/`runWriteback` 的 summary 与 failures；`openLineLog`、`writeExclusive` | 全部非收敛样本进 `failures-*.json`；种子在 manifest，逐轮延迟在每行 `pre_write_jitter_ms`；资源快照含 CPU/内存/负载、工具自身描述符上限、API 进程（Linux 读 `/proc`，其余取 manifest 声明）、连接池声明值、PostgreSQL 版本/`max_connections`/按库连接数、`/health/realtime`、工具事件循环延迟（只报告）。样本与 summary 以独占方式创建，已开始的测量不能在同一目录重跑。 |
| P08 | `-stats.node-test.mjs`（20 项）与第 3.3 节 | 反例先失败后通过；演练结果只标注源码版本。 |

## 5. 审查陷阱 F01–F14 的落点

| ID | 文件 › 符号 | 处理 |
| --- | --- | --- |
| F01 | `-stats.ts` › `recordFrame`、`matchExpected`、`revisionAnomaly` | 只按 `event_seq` 匹配；帧内版本记为 `frame_revision`，与回执不同则置 `correlation_anomaly`，不参与匹配。 |
| F02 | `-stats.ts` › `expectedEventSeq`；`runPropagation` | 写入者在静止后的权威 GET 取 `S`，期望 `S+1`；写后再读，若不等于 `S+1` 记 `event_sequence_anomaly` 并计入 summary。 |
| F03 | `-stats.ts` › `phaseJitterMs` | 由 manifest 种子与轮次序号决定的 `[0,500)` 整数延迟，逐行记录。 |
| F04 | `-stats.ts` › `propagationVerdict` | 每事件最慢观察者的 p95 是独立条件。 |
| F05 | `-stats.ts` › `nearestRankIndex`、`nearestRank`、`summarize`、`reportQuantiles` | 整数 rank；缺失为 +Infinity；JSON 里无界分位数写 `null` 加 `p95_finite:false`，不写 0。 |
| F06 | `guardEnvironment`、`-stats.ts` › `preregistrationMismatches`、`writeExclusive` | manifest 里的协议常量与工具内登记值不同即拒绝；连接池声明值大于 25、Redis 或多副本即拒绝；每次测量独占创建文件。 |
| F07 | `reconnectClosed`、`classifyObserverSample` | 只在轮次边界重连；断线期间样本为 `socket_closed`；重连后的读取另记 `recovered_by_reconnect_read_ms`。 |
| F08 | `runPropagation`（转交与静止屏障）、`-stats.ts` › `barrierSatisfied` | 创建者经公开 `handoffs` 把处理人转给本轮写入者，转交回执写入 rounds 文件、不进分母；100 个观察者都收到上一条事件（或其订阅快照已覆盖）后才计时，超时记录在案。 |
| F09 | `guardEnvironment`、`runWriteback` | manifest 绑定 Provider 源文件与二进制、daemon、CLI 的 SHA-256，并检查源文件含 host Provider 独有的动作协议标记；每个 run 在写指令前按 `request_id` 与 run id 读评论必须 404，预热 run 另查任务上没有任何 Agent 评论且回调不含 `result_comment_id`，否则 `provider_wrote_before_instruction` 并中止；`act` 必须返回同一 `request_id` 的回执。 |
| F10 | `PERFORMANCE_PROTOCOL.writebackFormalRuns` | 100 个独立 run，各一条。 |
| F11 | `runWriteback` | 使用 `SessionSet.issueId` 即 prepare 的那条任务；每个 run 记录 `open_sockets`。 |
| F12 | `resumeExecution`、`runWriteback` | Resume 与带 `target_versions` 的提及评论全部走 API。 |
| F13 | `pollUntil`、`runWriteback` | 终点取并行权威读取的完成时刻；访问回调用工具自己的 2ms 循环观察，不用 `waitForVisit`；不使用 daemon 内部时钟。 |
| F14 | spec 的 `test.skip`、`performanceOptIn`、`guardEnvironment`、`verifyLiveEnvironment` | 没有 opt-in 三个测试全部 skip；有 opt-in 时在任何请求之前核对 loopback、非默认库、库名/API/部署与 manifest 一致，再核对 `/api/config`、`/health` 的 pid 与 commit、`current_database()`；每个测试自设超时（60/90/120 分钟）。实测确认仓库根的 `.env` 会给 Playwright 进程注入一百多个变量，包括缺省时的 `DATABASE_URL` 与 `NEXT_PUBLIC_API_URL`，所以库名与 API 必须和 manifest 逐项相符才放行。 |

## 6. 协调者需要提供的环境

### 6.1 环境变量

| 键 | 阶段 | 要求 |
| --- | --- | --- |
| `COLLABORATION_PERF_ACCEPTANCE=1` | 全部 | 显式启用；其他值一律 skip |
| `COLLABORATION_PERF_RUN_DIR` | 全部 | 已存在的绝对目录，权限 0700，不在任何 Git 工作树内 |
| `COLLABORATION_PERF_MANIFEST` | 全部 | 上述目录内的 manifest 文件 |
| `DATABASE_URL` | 全部 | loopback；库名不是 `multica`/`postgres`/`template0`/`template1`，且等于 manifest 的 `database.name` |
| `NEXT_PUBLIC_API_URL` | 全部 | loopback；origin 等于 manifest 的 `api.url`（`TestApiClient` 在模块加载时读取它） |
| `COLLABORATION_E2E_API_URL` | 可选 | 若设置，origin 必须相同 |
| `MULTICA_DEPLOYMENT_ID` | 全部 | 等于 manifest 的 `api.deployment_id` |
| `COLLABORATION_HOST_DAEMON`、`COLLABORATION_HOST_PROVIDER`、`COLLABORATION_HOST_CLI` | writeback | 绝对路径的已构建文件，SHA-256 等于 manifest 的 `binaries` |
| `COLLABORATION_PERF_PROVIDER_SOURCE` | writeback | 冻结副本里的 `e2e/fixtures/collaboration-host-provider.go`，SHA-256 等于 `binaries.provider_source_sha256` |

API 侧前提（工具只核对、不设置）：password 模式；`ALLOW_SIGNUP` 不为 false；不设 `REDIS_URL`；`DATABASE_MAX_CONNS`/`DATABASE_MIN_CONNS` 不设置或不高于默认；write-back 需要 managed installation 可用。门禁：prepare 与 propagation 要求分配时已是 `enabled=true, creation_enabled=true`，工具在这两个阶段不改门禁，结束后核对布尔值与 epoch 都未变；write-back 由既有的 `openHostAcceptanceGate` 打开并恢复，结束后核对布尔值相同、epoch 不回退。

### 6.2 manifest（由协调者在分配时写入，工具只读并校验）

```json
{
  "schema_version": 1,
  "run_id": "pay241-s5-r01",
  "purpose": "rehearsal",
  "seed": 20261114,
  "protocol": { "observers": 100, "warmupEvents": 5, "formalEvents": 100, "propagationDeadlineMs": 10000, "propagationBudgetMs": 2000,
    "relayPeriodMs": 500, "writebackWarmupRuns": 1, "writebackFormalRuns": 100, "writebackDeadlineMs": 30000, "writebackBudgetMs": 5000,
    "registrationWindowMs": 300000, "registrationPerWindow": 30, "registrationMarginMs": 2000 },
  "source": { "commit": "<冻结提交>", "overlay_sha256": { "e2e/collaboration-performance.spec.ts": "<…>" } },
  "api": { "url": "http://127.0.0.1:<port>", "pid": 0, "commit": "<与 /health 一致>", "deployment_id": "<…>",
    "pool_max_conns": 25, "pool_min_conns": 5, "descriptor_limit": null, "redis": false, "replicas": 1 },
  "database": { "name": "multica_pay241_s5_<编号>_api" },
  "gate_original": { "enabled": true, "creation_enabled": true, "host_execution_enabled": false, "epoch": "1" },
  "binaries": { "provider_source_sha256": "<…>", "provider_sha256": "<…>", "daemon_sha256": "<…>", "cli_sha256": "<…>" }
}
```

`protocol` 必须与工具内 `PERFORMANCE_PROTOCOL` 逐键相同，多一个、少一个或值不同都会被拒绝。`purpose` 取 `rehearsal` 或 `acceptance`，原样写入 summary。`api.pool_*` 是协调者从 API 启动日志读到的实际值；`api.pid`/`api.commit` 与 `/health` 不符即拒绝。`source` 的内容工具不解释，只原样带入 summary。`binaries` 只在 writeback 需要。

### 6.3 运行目录布局

```
$COLLABORATION_PERF_RUN_DIR/            0700，仓库之外
  manifest.json                          协调者写入
  private/prepare-state.json             0600：账号用户名与密码、注册尝试的墙钟时刻、任务创建与每次邀请的原始请求键
  prepare-log.jsonl                      注册/邀请的状态码日志，无密码与 token
  setup-identities.json                  {actor_index, user_id, username}，无密钥
  observed-<phase>-<UTC>.json            每次尝试：live 身份、协议、工具文件哈希、门禁前值
  resources-<phase>-<UTC>-before.json / -after.json
  propagation-rounds.jsonl               每轮：转交回执、屏障、抖动、原始请求键、写结果、写后序号
  propagation-samples.jsonl              只追加，每轮 fsync；10500 行
  summary-propagation.json、failures-propagation.json
  writeback-runs.jsonl、writeback-samples.jsonl（每个 run 两行）
  summary-writeback.json、failures-writeback.json
```

token 不落盘。密码只在 `private/` 里。`*-samples.jsonl` 与 `summary-*.json` 以独占方式创建：只要某阶段写出过样本文件，就必须换新的运行目录与编号重跑（连同重新 prepare）；还没写出样本的 NOT_RUN 尝试可以在同一目录重试，每次尝试的 observed/resources 文件都保留。

## 7. 运行命令与预计时长

在冻结的源码副本里、经私有 runner 注入上述环境后执行（`run.mjs` 与 `perf.json` 由协调者新建）：

```bash
node --experimental-strip-types --test e2e/fixtures/collaboration-performance-stats.node-test.mjs
node "$TASK_RUN_ROOT/run.mjs" "$TASK_RUN_ROOT/perf.json" pnpm --dir "$TASK_SOURCE_ROOT" exec playwright test e2e/collaboration-performance.spec.ts -g "prepare:" --workers=1 --retries=0
node "$TASK_RUN_ROOT/run.mjs" "$TASK_RUN_ROOT/perf.json" pnpm --dir "$TASK_SOURCE_ROOT" exec playwright test e2e/collaboration-performance.spec.ts -g "propagation:" --workers=1 --retries=0
node "$TASK_RUN_ROOT/run.mjs" "$TASK_RUN_ROOT/perf.json" pnpm --dir "$TASK_SOURCE_ROOT" exec playwright test e2e/collaboration-performance.spec.ts -g "writeback:" --workers=1 --retries=0
```

顺序为 prepare → propagation → writeback，共用一个运行目录。两个测量阶段各自重新登录 100 人并建立 100 条 socket。propagation 与 writeback 在判定不通过或中止时测试为红，此时原始样本、summary 与 failures 已经落盘。

| 阶段 | 预计 | 依据 |
| --- | --- | --- |
| prepare | 约 12–20 分钟，另加 99 次真实邀请 | 注册限流要等过 3 个对齐墙钟的 5 分钟边界；对假服务实测 11.8 分钟（起跑时离边界约 2 分钟）。真实 PBKDF2 与邀请耗时未测。测试超时 60 分钟。 |
| propagation | 估计 4–10 分钟 | 登录间隔 250ms 即至少 25 秒；105 轮，每轮含一次转交、静止屏障、0–500ms 抖动与至多 10 秒截止。对假服务实测 2.4 分钟。全部超时的最坏情况约 40 分钟。测试超时 90 分钟。 |
| writeback | 未执行过，估计 5–15 分钟 | 101 个 run 串行，每个含触发、启动、一条结果、终结；任何一项 30 秒未收敛即中止。测试超时 120 分钟。 |

## 8. 与 design.md 的出入（请主会话确认）

1. **有 opt-in 但环境不合格时测试失败而不是 skip。** design 第 5 节写“缺任一项即 skip”。没有 opt-in 时确实全部 skip，`make check` 与整目录运行不会触发；但协调者已经显式启用后，把“默认库”“库名不符”这类拒绝显示成 skip 容易被读成无事可做，所以我让它报错并列出原因。如需严格按 design，把 spec 里的 `guardEnvironment` 调用包进 `test.skip` 即可。
2. **summary 与 failures 按阶段分文件**（`summary-propagation.json` 等），而不是单个 `summary.json`/`failures.json`。两个阶段是两次 Playwright 调用，分文件才能独占创建、互不覆盖。
3. **注册用工具自己的 `fetch` 调公开 `/auth/register`**，没有用 `TestApiClient.registerPassword`，因为后者丢弃响应头，读不到 `Retry-After`。登录仍用 `TestApiClient.loginPassword`。
4. **连接池的“实际值”取自 manifest 声明**，工具无法从外部读到 API 的池配置；它另外记录 PostgreSQL 侧按库聚合的连接数，并拒绝声明值大于 25 的运行。macOS 上 API 进程的描述符上限同样只能取声明值。
5. **提示路径（次要指标）**量的是“`t0` 之后第一个变更帧、随后该观察者读到评论”，不能保证那一帧就是评论事件的帧；我没有确认 runtime 评论是否产生自己的事件。它不参与判定。
6. 转交统一由创建者（actor 0）发起；design 写“当前有权者”，创建者始终有权。

## 9. 没有核实的事项

- **没有对真实 hub、API、数据库、daemon、Provider 或模型执行过任何一步。** `runWriteback`、`resumeExecution`、`withPhaseEvidence`、`verifyLiveEnvironment`、`readGate`、`captureResources` 从未执行，只通过了类型检查与 lint。
- 下列服务端行为只读过源码、或沿用其他夹具的用法，没有发过请求：创建者兼处理人的任务创建返回 201；邀请与转交返回 200 及 `participant_added`/`transferred`；创建者在不是处理人时仍可转交；新 workspace 的 `status-options` 含 `todo` 与 `in_progress`；一次状态变化恰好一条事件；非资源所有者的参与者发提及评论能在所有者已同意的目标上排队（`state: waiting`）；任意参与者可读 run records；`control` 的 resume 请求体；`adoptHostTarget` 对 agent 目标的 `role: "agent"`；成功终结后队列保持 active。任何一条不成立，工具会把对应事件或 run 记为未收敛或中止，不会给出虚假的通过。
- 100 名参与者时 workflow 事务、每帧授权查询在默认连接池下的耗时，以及任务详情随 100 个 run 增长后的读取耗时，均未测。终结回写的读取成本会随 run 序号上升，summary 里可按 `run_index` 查看。
- 100 条 Node 原生 socket 对真实 hub 的认证与订阅未测；假服务确认了 Node 22.23.2 的原生 WebSocket 不带 `Origin` 头。
- S2 正在修改的 workflow 事务、路由与迁移对上述假设的影响未评估。
- 没有运行全仓 typecheck、lint、Vitest、Go 测试或 `make check`。
- 自检没有覆盖：转交结果不明、订阅被拒导致的 NOT_RUN、屏障超时、注册响应丢失后的登录探测。
