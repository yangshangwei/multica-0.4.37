# 基线 StageCompletion 夹具失败诊断

状态：只读诊断结束，原始 unknown 根因未确认。未修改产品代码、夹具和断言；未调用真实智能体。

## 原始证据

完整 service 运行日志：任务外证据目录 `security-remediation-v2-evidence-20261120/logs/service_native-20261120-080051.log`。最初五个失败位于 5610–5624 行：

- `TestStageCompletionFinalClockRunsAfterWriterReceipts`
- `TestStageCompletionSubmissionRequiresItsOriginalOperation`
- `TestStageCompletionCandidateHeadLocksInOwningTransaction`
- `TestStageCompletionImmutableSubmissionLocksInOwningTransaction`
- `TestStageCompletionFinalSubmitAccountCannotChange`

均在 `newStageCompletionFixture` 初始化的 `PrepareStageExecution` 返回 `State: unknown`、`err: nil`，尚未进入各自真正要检验的行为。随后同一日志的 `AllM1ModesAndActualWriters` 与 `FinalCurrentCredentialHorizons/stage_signer` 也出现同类初始化失败。日志中的嵌套 FAIL 行不能直接当作独立失败案例数。

以下五个文件与已提交基线 `efdb597d6` 逐字节相同：`collaboration_stage_completion_test.go`、`collaboration_stage_completion_tx_test.go`、`collaboration_stage_execution_fixture_test.go`、`collaboration_stage_execution.go`、`collaboration_stage_execution_observations.go`（均在 `server/internal/service/`）。

## 已核实调用路径

`newStageCompletionFixture` → `PrepareStageExecution` → `enterStageExecution` → `RecordStageExecutionObservation`。后者失败时，`enterStageExecution` 有意记录服务端 `unknown` 观察并返回；因此外层 `err == nil` 不能证明集成观察记录曾成功，也不透露最初错误。

`stageExecutionSourceFixture.Execute` 用宿主 `time.Now().UTC()` 设置 `ObservedAt`。实际验证读取数据库 `ArtifactExecutionDatabaseTime`（`clock_timestamp()`）并拒绝未来观察；随后还会进行唯一性、权限、事务与写入检查。记录和 unknown 回退共享五秒截止时间。

每个 fixture 独立建立连接池、`NewCollaborationService`、随机工作空间/账号和观察签名密钥。`TxStarter` 是实例字段；紧邻前驱 `TestStageCompletionFinalSubmitAuthorityCannotChange` 在调用后明确还原为该实例的 pool。未找到前驱把 `TxStarter` 留到后续新实例的路径。数据库 singleton gate 与进程环境仍是共享面；fixture 自行设置并登记恢复，不能仅凭代码阅读排除整包污染。

## 定向原测试

使用根会话分配并迁移到 868 的专用 `service_diag` 库，运行同五个原测试，未启用 overlay：

```text
python3 /Volumes/artisan/code/2026/security-remediation-v2-evidence-20261120/run.py test service_diag ./internal/service -run '^TestStageCompletion(FinalClockRunsAfterWriterReceipts|SubmissionRequiresItsOriginalOperation|CandidateHeadLocksInOwningTransaction|ImmutableSubmissionLocksInOwningTransaction|FinalSubmitAccountCannotChange)$' -count=1 -v -timeout=5m
```

退出 0，RUN/PASS 5、FAIL/SKIP 0，103.746 秒。日志 `logs/service_diag-20261120-084232.log`。各测试约 8–34 秒，不能据此断言五秒观察窗口耗尽。定向通过不替代原整包失败。

## 当前假设与取证

1. 观察的宿主时间晚于记录时数据库时间。预测：实际错误为 `stage_execution_conflict`，记录 `ObservedAt > database_now`，其余有效性字段为真。当前一般时间采样落在请求往返区间，并未捕获失败瞬间，既不证明也不否定这一假设。
2. 观察记录遇到数据库/锁/上下文截止错误。预测：保留原始 Record 错误与 finalCtx 状态能直接区分；不能从耗时或 ARM64 镜像身份推断。
3. 夹具观察或共享状态与授权输入不一致。预测：观察有效性布尔值或具体记录错误指出不一致；单独通过但特定前驱序列失败可支持污染方向。

私有 logging-only overlay 位于任务外 `work/stage-diagnostic/`，只输出被吞掉的错误以及观察与数据库时间、有效性布尔值，不输出凭据，不改返回值、分支、时间或断言。原始文件 SHA256 写入 `source-before.json`，执行后再核对。


## 完整运行的后续状态

根会话报告本次完整 native service 在 45 分钟达到测试超时，RUN 2651、PASS 2614、FAIL 19、SKIP 0；这些数含嵌套测试，整包未完成。保留原日志和失败数据库，不将定向通过换算为整包通过。下一次验证拟统一为 Linux Go 1.26.6 与同 Docker 引擎的 PostgreSQL，从新迁移库跑原始完整包；这是一组环境受控的新证据，不是已证明的时钟修复。

08:48 只读进程列表还观察到其他会话目录 `/private/tmp/multica-backend-20261120-6npz3e0a/` 的 service 测试正在运行。没有终止或修改它们；存在共享负载是事实，不能据此确定原始 unknown 的原因。


## 私有日志 overlay 的实际结果

命令使用同五个原测试，额外传入 `-overlay .../work/stage-diagnostic/overlay.json`，`-count=1 -timeout=5m`。日志 `logs/service_diag-20261120-084500.log`：退出 1，运行 3、通过 2、失败断言 0、跳过 0，**五分钟超时**，301.780 秒；最后两条未开始。前两条分别在 130.17 秒和 96.32 秒通过。

没有出现任何 `SECREM_DIAG` 行，故本次没有捕获到 Record 错误或未来观察，不能把原 unknown 归因为时钟或截止时间。超时栈停在第三条初始化的 `release` → `StageCodeEvidence` → `stageSignerCurrent` → `GetCollaborationParticipant` 的数据库响应读取，尚未执行该次 `PrepareStageExecution`。这也不是原始 unknown 的复现。

08:49 对诊断库做过一次无 SQL/凭据内容的连接汇总，观察到一个 `idle in transaction / ClientRead`，没有记录到锁等待；一次采样不能排除前后其他等待。摘要在任务外 `work/stage-diagnostic/activity-0849.txt`。

私有 overlay 只加入日志；两个原文件 SHA256 前后相同（`source-before.json`、`source-after.json`）。不再追加前驱序列或更多次数，以免把大量时间耗在未复现的定向路径。`service_diag` 无活动诊断进程，交还根会话；保留库与日志至统一证据清理。

结论：原完整包失败与其未完成状态保留；原精确五条在独立库通过；日志诊断因环境下整体缓慢而超时、未捕获原故障。下一步有意义的证据是新受控环境的完整原包运行，仍须准确标明工具链与执行环境变化，不能称为已定位并修复基线问题。
