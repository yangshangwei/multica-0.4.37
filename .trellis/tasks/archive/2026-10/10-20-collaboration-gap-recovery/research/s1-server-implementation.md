# S1a 服务端扫描实施记录

日期：2026-10-21。范围：AC-I01 的服务端 waiting / wake / candidate 扫描、局部错误隔离和相邻容量/FIFO 回归。未提交、未改变发布 gate、未启动真实 provider。

## 变更与边界

- 新增 `collaboration_runtime_scan_cursor`，以 runtime / binding / scan kind 隔离持久位置。短事务按 runtime → 游标顺序持锁并在 runtime 锁内重验身份，提交有界扫描机会后才执行单对象事务；进程重建和多个 poller 共用进度。位置不表示授权或业务成功。
- waiting 以 issue / target，wake 以 group created-at / ID 作 keyset；各至多 16 项，尾端至多回绕一次。候选以当前优先等级内的 created-at / run ID 轮转，每次只返回 1 项，匹配 daemon 的一次实际尝试。
- 每次候选选择先在完整作用域判定 `reserved`、`reserved_continuation`、`unreserved`、`unreserved_continuation` 的最高当前等级，取得游标锁后在页查询快照再次核对。低等级不填补优先页；优先级和派生存在性不取决于游标或 LIMIT。优先级竞争产生空页时保留原等级游标，不能因暂时无可返回项抹掉之前的进度。
- 保留既有 reservation / 派生顺序改动；新根通过完整过滤的 `EXISTS` 延后于待领取派生 run。真实 namespace、队列 FIFO、group、hold、proof 和 nonce 不变。
- 仅隔离明确的对象账本哨兵、已回滚且重试耗尽的 `collaboration_busy`，以及列举的对象业务拒绝；日志只保留对象 ID / 固定 code。daemon 身份、context、未知/数据库错误仍返回。提交错误另有包装，不能因内含局部 code 被吞掉或自动重放。
- wire 继续只有 `run_ids`。与协调者复核后移除了暂拟的可选候选元数据：单候选页与服务端持久游标已足够兼容旧 daemon。
- runtime 和 workspace 删除语句在原子 CTE 中删除对应扫描元数据；历史 run / reservation / proof 仍留存。managed fixture 增加对应清理。

## 文件

- `server/internal/service/collaboration_runtime.go`
- `server/internal/service/collaboration_runtime_scan_test.go`（新）
- `server/internal/service/collaboration_runtime_scan_delete_test.go`（P2 删除竞争与锁内身份重验）
- `server/internal/service/collaboration_runtime_admission_test.go`（仅“返回最高等级”的 incidental 长度断言，既有 claim/capacity 断言保留）
- `server/internal/service/collaboration_runtime_test.go`（fixture 清理）
- `server/internal/service/collaboration_fifo_test.go`（仅将 commit 屏障绑定真实 source 锁）
- `server/pkg/db/queries/collaboration_runtime.sql`
- `server/pkg/db/queries/runtime.sql` / `workspace_delete.sql`（仅删除语句）
- migrations 730 / 731 的 up / down：游标表，以及独立 `CREATE UNIQUE INDEX CONCURRENTLY idx_collaboration_runtime_scan_cursor_identity`；down 在 NOWAIT 表锁下拒绝移除已用位置。
- SQL generation、旧 wake query 删除、migrate 无效索引清理注册由协调者完成。

## 已执行证据

隔离环境：协调者创建的 pgvector / PostgreSQL 17 `service_s1` 数据库；通过专用 helper 注入连接及 `GOTOOLCHAIN=go1.26.6`。新迁移已由协调者应用至 731。没有输出连接凭据。默认测试通过 agent CLI guard，仅使用合成身份、服务调用和 DB 事实。

所有命令从 `server/` 执行：

```bash
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s1 bash ../scripts/go-test-with-agent-cli-guard.sh go test ./internal/service -run '^TestCollaborationRuntime(WaitingScan|CandidateScan|ScanIsolates|WakeFailure|ScanPropagates)' -count=1 -v
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s1 bash ../scripts/go-test-with-agent-cli-guard.sh go test ./internal/service -run '^TestCollaborationRuntimeCandidateScanAdvancesPersistentRefusals$' -count=1 -v
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s1 bash ../scripts/go-test-with-agent-cli-guard.sh go test -race ./internal/service -run '^TestCollaboration(Runtime.*|PendingDerivedRunDefersOtherLaneRoot|ReservedCandidateIsOfferedBeforeOlderDerivedRun|RevokedDerivedRunDoesNotDeferPromotion|FIFO.*|Batch.*|ManagedRuntimeBatchProofAndExactReplay|QueueConcurrentSubmissionKeepsImmutableFIFO|TerminationSurvivesOwnerAndRuntimeRemoval)$' -count=1 -v
GOTOOLCHAIN=go1.26.6 go vet ./internal/service
GOTOOLCHAIN=go1.26.6 go vet -tests=false ./internal/service
```

- 原生产代码 RED：17 个持续对象拒绝后的第 18 个 waiting / candidate 无法推进，局部 ledger / typed 503 遮蔽既有候选，身份错误被吞掉，并发 poller 重复前缀。完整日志：`/tmp/multica-gap-recovery-20261021-h6phxwo9/s1a-baseline-red.log`。
- 组合 RED：中间的持久 16 候选页仍在“一次 List 只尝试首项”时饥饿，第 18 项 18 轮内不可见。日志：`/tmp/multica-gap-recovery-20261021-h6phxwo9/s1a-composed-red.log`。随后收窄候选页为 1。
- 删除清理 RED：runtime / workspace 两条旧删除语句均留下游标，随后补同事务清理。
- `go vet ./internal/service` 在 S2 并行新测试进入前退出 0；最后一次完整 vet 和 `-tests=false` 尝试均被正在实施的 `collaboration_resource_offers_test.go:85` 的 `undefined: CollaborationOfferResourceParams` 阻断。该文件属 S2，未修改或绕过其断言；协调者将在 S2 稳定后重跑包级 vet。
- 第一轮相邻 race：33 个顶层 PASS，1 个顶层 FAIL，0 SKIP（43.240s）。12 个新增顶层回归全部 PASS，含 18 个真实小队 wake、one-offer 的实际 Claim、priority 变更、未知提交和删除清理。日志：`/tmp/multica-gap-recovery-20261021-h6phxwo9/s1a-race.log`。
- 唯一失败是 `TestCollaborationFIFOCancellationAndPromotionSerialize/promote_first` 的同步夹具：旧 `recipientCommitStarter` 暂停每个 Commit，现在先暂停不持 source 锁的游标事务。经协调者同意，测试屏障已绑定真实 `LockCollaborationIssue`，保留原生产 List 调用和所有 FIFO 断言。该测试两种先后顺序的独立 race 复测均 PASS（0.93s）。随后完整同批 34 项 PASS（41.761s，`s1a-race-final.log`）。
- 最后自查补入“空优先页不擦除低等级旧位置”的确定性断言；先实际 FAIL，随后仅在有返回项时推进游标。最终同批 race **34 顶层 PASS / 0 FAIL / 0 SKIP，42.151s**；日志 `/tmp/multica-gap-recovery-20261021-h6phxwo9/s1a-race-verified.log`。包括全部 12 个新增顶层测试。
- 最终 scoped `git diff --check` 退出 0，Go 文件已经 gofmt。

初次新测试有两处测试编写问题（Body 字段、Runtime fixture 默认字段），以及一次游标表尚未应用的环境失败；均已更正，不计为产品 RED 或通过证据。

## 验证范围与后续

证据仅覆盖 task-owned PostgreSQL / Go service 的准入、扫描及恢复事实，不代表 Linux kernel containment、真实 provider、完整 daemon / CLI、其他 OS、Web / Electron 或父 97 项验收。协调者继续负责迁移/删除 inventory、完整集成和独立检查；不以本记录关闭父任务。

S1a 实施与自测完成，待协调者独立检查、迁移 / 删除 inventory 检查及跨切片集成。最终服务端主要文件 SHA-256：

| 文件 | SHA-256 |
| --- | --- |
| `server/internal/service/collaboration_runtime.go` | `5d4d7ee5bdcb8c5521421fa784678b9b5877d511b0fb3d38a2bcbb0d7751c7fc` |
| `server/internal/service/collaboration_runtime_scan_test.go` | `17c7676071346f71522436592a997721b4e79b2f43c853e11fb40b21ca0728ce` |
| `server/internal/service/collaboration_runtime_scan_delete_test.go` | `b081a504ee6838581cb414d754042bcc518b827486d463949f5601fa1e04a48d` |
| `server/pkg/db/queries/collaboration_runtime.sql` | `83fc3e2e6878b135d855b429e56ea46b5f5847b1c18e72b3847f03c3873119e4` |

## 独立检查修复：P2 删除与扫描竞争

检查者在私有 schema 复现了已授权扫描晚于 runtime 删除创建游标：`check-s1-s3a/cursor-delete-repro.json` 记录 `orphan_cursor_count=1`。仅在删除 CTE 中清理游标不足以阻止旧扫描随后创建新行。

新增 `server/internal/service/collaboration_runtime_scan_delete_test.go`，用实际 `TeardownRuntime`、`LockWorkspaceTaskOwnerRuntimes` 和删除 SQL 验证 runtime / workspace 的 scan-first、delete-first 两种顺序；另测等待 runtime 行锁期间 binding epoch 改变。测试通过实际 `pg_blocking_pids` 判断同步边界，不靠睡眠猜顺序。

失败优先结果：四个删除场景分别留下 1 / 2 / 1 / 2 个孤立游标，绑定变更场景仍以旧身份成功。均在加入行锁前真实 FAIL；日志 `/tmp/multica-gap-recovery-20261021-h6phxwo9/s1a-delete-race-red.log`。

修复仅复用已有 `LockAgentRuntime`：短游标事务先锁 runtime，再重新授权，再 Ensure / 锁游标；runtime 消失返回 `runtime_not_found`，绑定变化返回身份拒绝。事务结束后才处理 wake/promotion，不把 runtime / cursor 锁带入对象事务。授权读仅取当前快照，不增加 user / namespace 写锁或新 SQL / migration。

候选优先级竞争夹具也按 runtime → cursor 顺序持锁，在同一事务提交 Claim 使用的准确 reservation primitive，验证等待后的当前优先级、原游标和真实后续 Claim。原来“持游标 → 让扫描持 runtime → 独立 Claim”的夹具会被新的真实 runtime 锁正确拒绝，已移除该倒置顺序。

```bash
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s1 bash ../scripts/go-test-with-agent-cli-guard.sh go test -race ./internal/service -run '^TestCollaborationRuntimeScan(SerializesWithDeletion|ReauthorizesAfterRuntimeFence)$' -count=1 -v
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s1 bash ../scripts/go-test-with-agent-cli-guard.sh go test -race ./internal/service -run '^TestCollaboration(RuntimeScanSerializesWithDeletion|RuntimeScanReauthorizesAfterRuntimeFence|RuntimeCandidateScan.*|RuntimeWaitingScan.*|RuntimeScanIsolatesObjectFailure|RuntimeWakeFailureDoesNotHideCandidate|RuntimeScanUnknownCommitPreservesOriginalRun|RuntimeScanCursorCleanup|FIFOCancellationAndPromotionSerialize|FIFOConcurrentPromotionConsumesEachInputOnce)$' -count=1 -v
```

修复后复测先被并行 S2 的生成符号缺失阻断：`collaboration_resource_offers.go:83` 的 `TryLockCollaborationResourceOfferMember` / Params 尚未生成；随后又遇到 S2 新测试写入期间的缺失 import。没有修改 S2，协调者在其文件齐备后提供短暂只读窗口。

最终 focused race **13 顶层 PASS / 0 FAIL / 0 SKIP，17.286s**，覆盖两个新测试的五种交错、候选优先级竞争 / 多 poller、waiting 公平性、对象错误隔离、未知提交、删除清理，以及 FIFO 并发 / 取消竞争。日志 `/tmp/multica-gap-recovery-20261021-h6phxwo9/s1a-delete-race-green.log`。同一最终源码窗口的 `GOTOOLCHAIN=go1.26.6 go vet ./internal/service` 退出 0；gofmt 与 scoped diff-check 通过。

P2 实施与定向验证完成，没有新增 migration、SQL 或协议变更。此前 34 项通过是 P2 修复前证据；上述 13 项为本次锁与删除竞争修复后的证据。由主会话继续独立复核和稳定源码下的集成验证。
