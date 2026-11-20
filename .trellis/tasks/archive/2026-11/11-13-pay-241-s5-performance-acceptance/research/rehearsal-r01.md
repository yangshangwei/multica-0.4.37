# S5 工具演练 r01（2026-11-14）

**这是采样工具在真实API上的首次演练，不是TC-24或任何验收条目的通过证据。** 源码为S2之前的 `fb9f4c070` 加四个工具文件，只有API，没有production Web与Desktop；正式验收须在S2–S4稳定后的最终候选上以新编号重跑。

## 环境

- 运行根：`/Users/artisan/.multica/acceptance/pay241-s5-20261114-r01`（仓库外，0700）。源码为 `git clone --shared` 后检出 `fb9f4c070`，overlay四个工具文件，哈希见该目录 `source-overlay.sha256`。
- 构建：`server`（`-X main.commit=fb9f4c070…`）、`migrate`、`multica`、`host-daemon.test`（`go test -c ./internal/daemon`）、`host-provider`（`collaboration-host-provider.go`，`--fixture-kind` 输出 `multica-host-e2e-provider-v1`）；哈希见 `bin.sha256`。
- 数据库：新库 `multica_pay241_s5_r01_api`，792条迁移到804。共享容器 `multica-postgres-1` 为 **amd64镜像运行在arm64主机上（模拟执行）**，PostgreSQL 17.11，`max_connections` 100，分配时已有47条连接。
- API：pid 20501，`http://127.0.0.1:18995`，password模式，`MULTICA_PASSWORD_LIMITER_MODE=single`，无Redis（内存hub），未设置连接池变量，启动日志记录 `max_conns=25 min_conns=5`。`/health` 的commit与冻结源码一致。
- 门禁：迁移后原值 `enabled=false, creation_enabled=true, host_execution_enabled=false, epoch=2`；分配时打开为 `enabled=true`（epoch 3）。回写阶段由夹具打开宿主执行并恢复，中止后只读核对为 `enabled=true, creation_enabled=true, host_execution_enabled=false, epoch=5`。收尾时已恢复迁移原值，见下文"收尾"。
- 主机：10核、16GB。分配时负载约11，传播阶段前后负载 3.6 → 17.0，回写阶段约4.7。同机有S2的Go与Vitest测试在跑。

## 结果

| 阶段 | 结果 | 数据 |
| --- | --- | --- |
| 统计反例测试（冻结源码内） | 20/20通过 | `logs/stats-node-test.log` |
| prepare | 通过，11.2分钟 | 100次注册全部201，99次邀请全部200，三次窗口等待，无429；100个不同user_id |
| propagation | 通过，2.6分钟 | 正式10000行全部收敛，缺失0；pooled p50 511ms / p95 783ms / p99 969ms / 最大1154ms；每事件最慢观察者 p95 846ms；100个不同观察者、socket、写入者，每人恰好写一次；关联异常0、序号异常0、屏障超时0、重复帧0、首读过期0；读取次数全部为1 |
| writeback | **中止，未通过** | 预热1个与正式71个run收敛（业务结果 p50 82ms / 最大178ms；终结 p50 306ms / 最大499ms；排队/启动 p50 1.5秒，仅报告）；第72个正式run `no_provider_launch`，其后28个NOT_RUN；分母保持100，p95为无界 |

分段（仅供参考，不参与判定）：写响应 p95 73ms；事件到达 p50 397ms / p95 641ms（中继每500ms扫描一次）；观察者权威读取 p95 223ms。

主会话用独立脚本从 `propagation-samples.jsonl`（sha256 `3c0ecf5e…93f1a5`）重算：行数、阶段、10000个不同的（事件, 观察者）对、nearest-rank分位数、由原始时间戳重算的耗时、状态/序号/版本逐行一致、抖动在 [8,493] 内且五个100ms桶为 19/22/18/18/23。与工具输出逐位相同。

## 回写中止的原因

`logs/api-runtime.log` 第16291行：`08:08:52.887 INF runtime sweeper: marked stale runtimes offline count=1 workspaces=1`，与第72个run的触发同一秒，距回写阶段登记runtime约3分钟。阈值为 `service.RuntimeClaimFreshnessSeconds`=150秒（`server/cmd/server/runtime_sweeper.go`）。既有host夹具不发送runtime心跳，此前的规范用例都在约30秒内结束，从未越过阈值。这是测试夹具/采样工具的缺口，不是产品派发缺陷：真实daemon会发送心跳。

处理：退回给工具实现者在四个工具文件内补上与真实daemon相同的心跳并记录在案，不修改夹具、不调整sweeper、不缩短阶段。r01的回写样本文件原样保留；修正后以新编号、新运行目录并重新prepare后重跑。

## 其他观察

- 传播阶段API记录了11次 `db pool pressure`（连接池25，100条socket每帧一次授权查询），`empty_acquire_delta` 很大但平均等待约3ms。未调整连接池。
- API启动日志有 `auth rate limiting disabled: REDIS_URL not configured`；注册限流走的是password limiter的单进程模式，实际生效（三次窗口等待）。
- Playwright报告为通过/失败与工具summary一致；回写阶段命令退出码为1。

## 收尾（2026-11-14T02:20Z）

- 门禁：`gate.mjs read` 记录恢复前 `enabled=true, creation_enabled=true, host_execution_enabled=false, epoch=5`（`gate-read-pre-restore.json`）；`gate.mjs restore` 按 `gate-allocation.json` 的原值写回（`gate-restore.json`）；只读复核为 `enabled=false, creation_enabled=true, host_execution_enabled=false, epoch=6`（`gate-read-post-restore.json`）。三个开关回到迁移原值，epoch单调递增。复核时共享集群45条连接。
- API：核对pid 20501的命令行确为本运行根的 `bin/server` 后发送SIGTERM，同一秒退出，日志末行 `server stopped`；18995端口无监听（`api-stop.json`）。数据库、运行目录与全部日志保留。

## 未覆盖

production Web与Desktop、S2阶段门禁开启后的workflow事务、真实模型、与正式验收相同的最终源码。r01不改变父矩阵TC-24的 NOT_RUN 状态。
