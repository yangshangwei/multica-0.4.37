# Research: Runtime scheduling and termination-proof recovery

- Query: 失败前缀是否能阻塞调度和停止证明恢复？核对当前排序、截断、错误处理、既有修复与验收条件。
- Scope: internal — server candidate/promotion queries, daemon poller, retained ledger and Runner; planning only.
- Date: 2026-10-20
- Active task: `.trellis/tasks/10-20-collaboration-gap-recovery`.
- Authorization: 本轮仅研究与规划；未修改产品源码、规范或测试，未执行 git 操作，未启动真实 provider。

## Findings

### 1. 当前结论与已有修复的边界

当前代码中仍存在四条独立的进度风险：服务端待执行目标固定前 16 条、wake/promotion 的非预期错误提前返回、daemon 每轮第一候选失败后不轮转、停止证明恢复固定尝试前 8 个可证明记录。它们不是同一个队列问题，修复与回归应分别覆盖。

当前工作区已有两项相关顺序修复，不能归为本研究的实现成果：

- `server/pkg/db/queries/collaboration_runtime.sql:32`–`47`：候选优先返回仍持 namespace reservation 的 run（`reserved DESC`），再区分 continuation，最后按 `created_at,id` 排序。持 reservation 的根 run 与更老的无 reservation 成员并存时，前者优先，避免成员不断收到 `runtime_busy`、而占位者永远未被领取。
- `server/internal/service/collaboration_runtime.go:617`–`630`：promotion 查询同一个候选集；仍有 `!candidate.Reserved` 的派生 run 时，以 `derived_run_pending` 延后新根，防止新 lane 根反复抢占成员/continuation 所需 namespace。被撤权、held、forced、settled 的派生 run按查询过滤，不能仅凭“存在过成员”永久延后新根。
- 对应现有测试：`server/internal/service/collaboration_runtime_admission_test.go:104`、`:137`、`:161`。本轮没有执行这三个 DB 测试；源码中的测试存在不等于本轮通过。

这两项修复处理合法占位者与派生执行的优先级；它们没有提供失败项轮转、分页游标或 proof 恢复公平性。

### 2. 服务端固定 16 条：哪些前缀会退出，哪些会保留

文件与调用链：

- `server/pkg/db/queries/collaboration_runtime.sql:49`–`56`：`ListCollaborationRuntimeWaitingTargets` 以 `s.issue_id,t.id` 排序，固定 `LIMIT 16`，没有 offset、after-key 或 cursor 参数。查询只要求当前 grant、无 target hold、session/channel active、存在尚未入组的 waiting 输入；完整 promotion 谓词仍在服务层。
- `server/internal/service/collaboration_runtime.go:467`–`487`：列表中的每个 target 各自执行 promotion 事务；`CollaborationError` 且 `Status < 500` 才跳过继续。
- `server/internal/service/collaboration_runtime.go:522`–`568`：gate、issue/target/channel 状态、未结 group/run、选批、运行端支持检查均可能拒绝 promotion。
- `server/internal/service/collaboration_runtime.go:604`–`629`：installation stopped、runtime offline、namespace occupied、derived run pending 都拒绝 promotion。
- `server/internal/service/collaboration_runtime.go:519`–`521`：普通 `deny` 只返回 `RefusalCode`；不能假定每次拒绝都修改目标使它退出下一轮列表。

**充分触发条件**：同一 runtime/binding 有至少 17 个仍匹配 waiting 查询的目标；按 `issue_id,t.id` 排在最前的 16 个持续无法完成 promotion，且该失败不会使它们退出查询。每轮仍得到原 16 个，第 17 个即便独立满足 promotion 条件也不被扫描。UUID 排序不是 FIFO 入队时间，也不保证跨单公平。

**健康前缀**：成功 promotion 后 waiting 输入变 in-flight、channel 持有 group，最终 proof/group 结算会让前缀变化；持久 target hold 也可能让目标退出查询。这种变化使后续目标逐轮被看到，不能用它推论持续拒绝前缀也能推进。

**条件限制**：同一个 runtime namespace 被真实存活或未知进程占用时，下一个根本就不具备准入条件；此时保留屏障是正确结果，不是可以靠分页绕过的饥饿。需要用某些目标特有的持久拒绝，与后继确实可运行的目标来验收失败公平性。不得把 `runtime_busy`、offline 或数据库错误改成永久 target hold 来“清前缀”；现有注释明确限定 hold 的范围（`server/internal/service/collaboration_runtime.go:685`–`709`）。

候选 run 查询还有另一处 `LIMIT 16`：`server/pkg/db/queries/collaboration_runtime.sql:35`–`47`。它按 reservation、step kind、创建时间、ID 排序，无游标。仍匹配该查询但不可成功 claim 的前缀可以限制后继可见性；是否某个具体错误能形成这样的持久候选，必须沿真实 admission/invalidator 验证，不能仅凭 query 断言所有拒绝均有此结果。

### 3. 非预期错误可阻止整次 List 返回既有候选

`ListRuntimeRuns` 的顺序是 authorize daemon → squad wakeups → waiting promotion → candidates：

- `server/internal/service/collaboration_runtime.go:454`–`465`：`drainSquadWakeup` 遇到 `CollaborationError` 且 `<500` 跳过；其他错误直接返回。
- `server/internal/service/collaboration_runtime.go:471`–`485`：promotion 同样仅跳过上述业务拒绝；普通 error、数据库错误、typed 5xx 直接返回。
- `server/internal/service/collaboration_runtime.go:488`–`495`：只有前两阶段完整结束，才读取并返回已经存在的候选 run。
- `server/internal/daemon/collaboration.go:202`–`205`：List error 使 dispatch 直接失败。

因此一个单独 wake 或 promotion 项若每轮稳定产生未归为 `<500` 的错误，就能使同一 runtime 的所有后续 wake、promotion 和既有候选返回均停滞；不需要 16 个失败项。这里是提前返回路径，和“遍历完前 16 个仍没有看到第 17 个”不同。

**设计约束**：全局 daemon 身份失效、请求取消、整个 DB 不可用与单对象损坏/冲突不能一律吞掉。每个候选事务已经独立，应只隔离可归于该对象的失败，并报告它；unknown commit 不得重发 mutation。全局失败保留显式错误，其他已独立验证的工作仍应有机会被观测。

### 4. daemon 第一候选重试没有公平轮转

- `server/internal/daemon/collaboration.go:186`–`215`：先取得 machine slot，再列候选；从 `RunIDs` 中选第一个不在 `blocked` 的 ID，立刻 break。一次 dispatch 只选择一项，没有 cursor 或上次失败位置。
- `server/internal/daemon/collaboration.go:219`–`235`：Runner 返回 `ErrUnconfirmed` 时把该 run 写入 `blocked` 并保留容量；其他错误释放 slot 和 activeTasks，但不写失败轮转状态。下一轮按相同顺序可再次选择相同 ID。
- `server/internal/daemon/collaborationrun/runner.go:63`–`72`：launcher Probe 或 Claim 失败在注册/烧 nonce 前返回，可能是这种非 `ErrUnconfirmed` 错误。
- `server/internal/daemon/collaborationrun/runner.go:49`–`60`：已 ACK tombstone 返回成功；可证明历史记录走 finish/abandon；burned/recorded 未证明记录保持 `ErrUnconfirmed`。

**充分触发条件**：第一未 blocked ID 持续保留在候选结果中，其 Runner 每次失败且错误不包含 `ErrUnconfirmed`，同时候选之后存在实际可领取的 run。此时一个失败项就能反复被选；释放 slot 本身不意味着后继得到机会。若列表全部前 16 项都是本地 blocked，后继也可能因 server 截断不可见，但这些 blocked 项的机器/namespace屏障是否允许后继必须另验。

**健康前缀**：成功收尾的 run 在服务器 settled 后退出候选；后继自然晋升。需要候选持久不退出的失败场景，才能验收轮转。

**不应弱化的边界**：未知 launch 仍须保留 machine slot 与 update barrier；公平性只能给其他已满足真实容量/namespace条件的工作提供扫描机会，不能“跳过未知”后释放它占用的资源。

### 5. 停止证明恢复固定前 8 个，失败/锁忙都占计数

- `server/internal/daemon/collaborationrun/ledger.go:68`–`118`：`Retained()` 从 `os.ReadDir` 枚举 `.json`，文件名是 `sha256(deployment/installation/run).json`；名称顺序固定，既不是运行时间也不是重试时间。ACK记录被跳过，其余保留。
- `server/internal/daemon/collaborationrun/ledger.go:105`–`115`：reaped、never-launched、sealed、proof，以及所有 pre-burn阶段为 `CanProve`；burned/recorded 为 `Unconfirmed`，不具备证明恢复资格。
- `server/internal/daemon/collaboration.go:131`–`150`：每轮只尝试前 8 个 `CanProve`；在调用 Runner 前执行 `recovered++`。证明上传失败与 `ErrBusy` 都消耗这一额度。错误仅写日志，函数仍返回 nil，没有 last-attempt cursor。
- `server/internal/daemon/collaborationrun/runner.go:131`–`142`：每项 proof 请求超时 10 秒；失败加入 `ErrProofPending`，不 ACK。后续仍保留原身份/receipt待重放。
- `server/internal/daemon/daemon.go:4840`–`4844`：proof 恢复位于每轮普通/协作 machine claim 之前。8 个接近超时的请求能使一次调度周期延长约 80 秒；这是吞吐延迟，独立于第 9 个 proof 的长期饥饿。

**充分触发条件**：至少 9 个 `CanProve` 记录；按摘要文件名排序，前 8 个每轮都仍未 ACK，例如持续上传失败、拒绝、或被另一 supervisor 锁住。它们消耗每轮全部额度，第 9 个即使 proof endpoint 可立即 ACK也永远不被此恢复循环尝试。一个失败项不会在预算 8 下单独饿死紧随其后的项，但能减低每轮有效进度；要永久占满前缀需 8 个持续占额项。

**健康前缀**：ACK完成后 `Retained()` 不再返回该项，下轮第 9 个进入前 8。接口恢复后原 8 项依次 ACK也会解除堵塞；不能把这种正常收敛当作持续失败条件已被处理。

**不可证明与损坏的区别**：

- 合法 burned/recorded 的 `Unconfirmed` 项被 proof 恢复跳过，不消耗“8个 CanProve”额度；启动时仍保留 machine 容量（`server/internal/daemon/collaboration.go:97`–`111`）。
- 任何无法读取、权限不私有、过大、身份/摘要不符、严格解码或阶段验证失败的账本，`Retained()` 返回 `ErrUnconfirmed`，不是部分列表（`server/internal/daemon/collaborationrun/ledger.go:69`–`103`、`:172`–`219`）。poller因此拒绝进一步 machine claim。fail-closed 行为必须保留；是否可以独立恢复其他有效已 sealed proof 可另作设计，但不能用忽略坏账本启动新进程。

### 6. 最小拟议设计

1. **服务端扫描完整机会**：给 waiting目标/候选 run 的有界扫描引入稳定 keyset游标或等价公平轮转；失败项保留自身历史，游标按“已尝试”推进并有限回绕，避免固定前 16。candidate优先 reservation约束仍在；派生 run是否等待用不截断的 existence查询或同等证明，不能让分页改变 R23。
2. **错误分界**：wake/promotion每项独立事务；可归到对象的错误留日志/可观测原因并继续其他项；身份、取消、DB整体不可用等全局故障仍明确拒绝。未知 commit仅查原身份，不重放 mutation。
3. **dispatch轮转**：记录尝试过的候选位置，对非 unknown错误给其他候选机会；同一 run的下一次尝试仍复用原身份/ledger，既不制造新 attempt，也不把 stale/unsupported当真实成功。reserved候选依然必须优先，不得绕开 namespace实际占用。
4. **proof恢复轮转**：按上次尝试key而非前8进行有限回绕；包括失败和锁忙都推进公平位置，保留有界预算。原proof bytes/ReceiptID/attempt nonce永久复用。可使用共享短预算避免8个请求阻塞整个poll周期；若拆为后台恢复，其幂等与跨进程ledger锁仍是权威，不让重排创建第二个fork。
5. **fail-closed不变**：不可证明/损坏账本仍保留容量和启动屏障；proof到达只结算原run，不自动Resume、解除hold或复活stopped/cancelled输入。

### 7. 验收与回归映射

| 场景 | 必须证明 | 原需求/契约 | 适合的测试层 |
| --- | --- | --- | --- |
| 17+ waiting目标，前16持续目标特有拒绝，第17合法 | 后继在有限轮次得到扫描/推进，失败前缀历史与hold语义不被改写 | E01公平容量，E05无关lane可推进；AC-G02 | service DB，真实fixture+确定性fault |
| wake/promotion单对象普通error/5xx，在其后有合法已存在候选 | 局部失败不吞掉合法候选；全局认证/取消/DB不可用仍明确拒绝 | E01、E02事务与错误边界 | service DB+route |
| reserved根与更老成员/continuation并存 | reserved先领取；真正pending派生优先于新根；撤权派生不永久挡根 | R23、E01/E03 | 已有admission测试＋分页边界回归 |
| 第一候选持续非Unconfirmed失败，后继合法 | 后继有限轮次得到尝试；原run不造新nonce；未知者仍占实际容量 | E01、E04、Q02 | daemon httptest+临时ledger+fake launcher |
| 9+ sealed/pre-burn记录，前8持续proof失败 | 第9个得到ACK；前8保留原receipt/身份，不被伪造结束 | R17、E04/E05、Q05 | daemon recoverProofs+httptest+临时ledger |
| 9+记录，前8跨supervisor锁忙 | ErrBusy不永久吃满固定前缀；不额外fork、不抢锁 | E04跨进程互斥 | daemon/ledger，测试创建子进程 |
| ACK丢失、recovery重启、快速失败项 | 重放原sealed proof/ReceiptID；ACK墓碑退出库存；公平状态重启后仍收敛 | E02/E04/E05、Q05 | Runner/ledger+poller |
| burned未知／损坏文件＋合法proof | 未知容量不被释放，坏记录不能变空库存；如支持独立proof恢复不借此开放claim | R17、E04、Q02 | ledger+machine poller |
| 所有前8proof请求都接近timeout | 恢复使用有限时间预算，普通/其他协作合法调度有进度 | E01实际容量/公平 | daemon可控transport/context，无真实provider |

### 8. 已执行验证

工作目录：`/Volumes/artisan/code/2026/multica-0.4.37/server`。

精确命令：

```bash
go test ./internal/daemon/collaborationrun -run '^(TestRunnerRestartConvergesPreBurnRecordsToNeverLaunched|TestRunnerRestartNeverRepeatsUncertainPermitOrFork|TestRunnerReplaysSameSealedProofAfterLostHTTPResponse|TestRunnerExplicitPermitRefusalProvesNeverLaunched|TestRunnerReplaysLostPermitWithSameAttempt|TestRunnerExhaustedPermitReplayProvesNeverLaunched|TestNonceBurnSurvivesRestartAndCannotLaunchAgain|TestSealAndProofSurviveRestartAndRefuseLateAttempt|TestLedgerLockExcludesAnotherProcess|TestLedgerCorruptionNeverCreatesFreshRun|TestLedgerRejectsSharedPermissionsAndSymlinks|TestUncertainPermitCannotBecomeExitProof|TestRetainedInventoryPreservesCapacityAndProofRecovery|TestUnrequestedRegistrationSealsAttemptZeroTombstone)$' -count=1 -v
```

结果：exit code `0`；`PASS`；包输出 `ok github.com/multica-ai/multica/server/internal/daemon/collaborationrun 1.558s`。14个顶层测试全部通过；pre-burn的3个子场景及unknown的2个子场景也通过；本命令没有 `SKIP`。

通过范围：临时私有ledger、nonce单向性、跨进程锁、损坏/权限拒绝、ACK丢响应时原proof重放、pre-burn永不启动收敛、permit相同seq/nonce的有界重放。`runner_permit_test.go`的 `refusingLauncher` 永不fork（`:17`起）；Runner恢复测试只使用 `httptest`，锁测试用测试自身子进程；未发现或执行用户安装的agentCLI。

**没有执行**：service DB候选/调度测试、daemon `recoverProofs` 的9记录公平性测试（当前所读测试中没有该回归）、真实Linuxnamespace/原生provider/完整CLI链路。它们属于未运行或待新增，不能写成“跳过后通过”。上述通过证明了原有安全/幂等基础，不能证明失败前缀已修复，也没有本轮可执行的端到端饥饿复现。

### 9. 相关规范与源码清单

| 文件 | 用途 |
| --- | --- |
| `.trellis/workflow.md` | 任务获准创建仍处规划，复杂任务的PRD/design/implement合同 |
| `CLAUDE.md` | 默认测试禁止用户安装的agentCLI；Go/事务与包约束 |
| `.trellis/spec/server/collaboration-targets.md:139`、`:147`、`:225`、`:368` | containment、proof与显式Resume、失效收敛、公平性/崩溃验收 |
| `.trellis/spec/server/collaboration-runtime-cli.md` | 显式CLI、真实machine slot、未知进程保留容量、当前Linux窄实现范围 |
| `.trellis/spec/server/collaboration-switch-and-skills.md` | 旧group全部proof、资源epoch失效与历史proof不可改写 |
| `.trellis/tasks/10-10-cross-workspace-collaboration/research/execution-contract.md:5`、`:32`、`:63`、`:90`、`:112`、`:195` | E01准入与公平；E03小队；E04 nonce/containment；E05独立结果/proof；Q/M验证矩阵 |
| `server/internal/service/collaboration_runtime.go:448` | 服务端wake→promotion→候选，逐项错误与reservation/derived约束 |
| `server/pkg/db/queries/collaboration_runtime.sql:35` | 固定16候选与waiting目标、排序、namespace reservation |
| `server/internal/daemon/collaboration.go:131`、`:188` | 固定8proof恢复、第一候选dispatch与slot释放 |
| `server/internal/daemon/daemon.go:4829` | 生产machine poll loop，proof恢复先于slot/claim |
| `server/internal/daemon/collaborationrun/ledger.go:68` | 库存、摘要身份、阶段与损坏fail-closed |
| `server/internal/daemon/collaborationrun/runner.go:40` | 重启恢复、pre-burn收敛、permit replay、proof上传 |
| `server/internal/service/collaboration_runtime_admission_test.go:104` | 既有derived/reserved/revoked顺序回归 |
| `server/internal/daemon/collaborationrun/runner_test.go:45`、`:95`、`:116` | 已运行的pre-burn/unknown/ACK丢失测试 |
| `server/internal/daemon/collaborationrun/runner_permit_test.go:106`、`:124`、`:158` | 已运行的permit拒绝与同nonce重试测试 |
| `server/internal/daemon/collaborationrun/ledger_test.go:48`、`:203` | 已运行的nonce/库存/私有ledger安全测试 |

External references: 未联网；仅用当前仓库与Go标准库行为，没有供应商/API文档或跨平台运行证据。

## Caveats / Not Found

- 不允许新写回归测试，故真实缺陷结论以明确控制流的充分条件为依据；具体生产数据是否存在持久可见的拒绝候选、局部错误分类和可领取后继，仍需实施时的确定性DB/poller回归证明。
- 服务端候选/派生顺序已存在未提交相关改动；本研究没有查看git差异或将其归因给本任务，不声称已提交、已发布或本轮修复。
- 本轮只运行不调用真实provider的现有确定性Go测试。DB、Linuxkernel隔离、真实provider、多平台与完整父任务验收没有执行。
- 为遵守主会话的限时收敛要求，在拿到定向测试结果后直接落盘；不继续扩大源码读取或测试范围。
- 实现必须保留reservation优先、派生准入、未知容量、原proof/nonce/receipt身份和显式Resume边界；不能用降低R17停止证明要求换取表面进度。
