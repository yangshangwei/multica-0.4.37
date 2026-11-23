# RKI-6 只读恢复路径预检（2026-11-02）

结论：**RKI-6 继续开放，尚未复现或修复当前候选。** 当前终段恢复和单次 Runner 框架与保留失败版本逐字节一致。最终安静环境下的 normal/race 仍须实跑；一次通过也不能宣称消除了原失败机制。

## 已核实的历史事实

保留来源 `4b508179f` 的 E0a 为 `incomplete / measured:handoff_or_recording_event_missing`，Runner error 为 `context deadline exceeded`。私有 ledger 已是 `proof_acknowledged`，outcome 已 ACK，唯一 checkpoint 的 3 条消息全部 ACK，recording 已 Drained。`completed` seal（sequence1、observed3、acknowledged3）已落盘，但没有 SealAccepted；71 条 HTTP trace 中没有 seal 请求。Provider 物理启动和授权 attempt 均为 1，业务 success/proof settled 不改变不完整判定。

这比早期 H3b 的未补完 backlog 更具体：E0a 的剩余工作是已持久化 metadata seal，不能把它解释为消息正文丢失。上述只读取现有记录；未写原 ledger/数据库，未执行恢复。原件路径和完整文件哈希仅记录在配套 JSON，不复制正文或凭据。

## 当前源码与生产路径

- `host_runner.go:476`、`host_checkpoints.go:192` 和 `rk1_run_test.go:81` 所在三个完整文件与 `4b508179f` **逐字节相同**，哈希见 JSON。合流本身没有改变这个机制。
- `recoverHost` 在 `host_runner.go:520` 创建一秒 recording context，先恢复 checkpoint，再用独立 outcome/proof 路径，最后由 defer 发送 seal（`:485–495`）。同一 context 的墙钟期限在 outcome/proof 执行期间也继续流逝。`sealHostRecording` 先落盘原 seal（`host_checkpoints.go:374–413`），再检查 `ctx.Err()`（`:441`），因此可以合法留下“seal 已落盘、未发送 HTTP”的状态。该路径在代码上仍存在；不能从 trace 精确断言历史一秒分别消耗于哪几个请求，因为 `rk1_trace_test.go:73` 仅记录 handler **完成时间**，没有请求起点／耗时／deadline。
- `Ledger.Retained` 保留 acknowledged 但 recording pending 的 v2 run（`ledger.go:114`）；`hostRecordingPending` 以未 ACK seal 判为 pending（`host_checkpoint_ledger.go:104`），`retainedHostRun` 仍给它 CanProve（`host_ledger.go:185`）。
- 生产 daemon 每轮先 `recoverProofs`（`daemon.go:4844`），按共享十秒／最多八项的恢复轮次选中该 run，以 **没有 Host 的 Runner** 调用 `Run`（`collaboration.go:178–214`）。已有 ledger 进入 `recoverHost`（`host_runner.go:35–42`）；acknowledged 分支不重 Claim、Permit、Provider 或 proof。完整恢复后的原 seal ACK 才会使 pending inventory 消失。
- RK1 的 `execute` 只有一次 `Runner.Run`（`rk1_run_test.go:112`），返回后立即 observe/lifecycle/评分（`rk1_trial_test.go:119–173`）。当前框架如实测量单次前台完成，**没有覆盖 daemon 的后续保留恢复轮次**。当前登记要求 seal 且 RunErr 为空（`rk1_lifecycle_test.go:124/138`），所以原失败正确地仍为 incomplete。

## 对下一入口的建议

先按既定计划在重型任务静止后的最终候选上执行原断言 normal/race；本报告不代替结果，也不预测 happy fake 必然失败。发生同类 incomplete 时停止付费入口，保留本轮所有原始结果，不加睡眠、延长超时、清掉 RunErr 或临时多跑一次。

最小的后续诊断是由统筹隔离复制失败数据库和私有 ledger，另建诊断输出，显式运行一轮生产式 **无 Host 的恢复**。核验只用原 seal 身份，物理启动／attempt 仍为1，没有新 Claim/Permit/handoff/业务执行/proof，seal 获 durable ACK 且 retained inventory 清空；首轮错误单独保留。该诊断不能重写原样本为 pass。

若诊断证明生产恢复可完成，后续框架可考虑在评分前明确驱动生产有界 reconciliation，并逐轮保存原错误／状态／调用数；这需要先改登记与框架、独立复核和重新绑定新候选，再做全 fake/race。不能向已经 incomplete 的 trial 事后追加恢复后改判。若隔离恢复仍失败，应依据实际失败修产品或 ledger，而不是只改评分。

此次仅新增本 Markdown 和配套 JSON；未改产品／框架源码，未运行测试、数据库、服务、Agent CLI、Provider 或外部操作。未验证最终 fake/race、真实 daemon 恢复、真实模型及原生平台。
