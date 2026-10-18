# 普通任务与协作运行的共享容量边界

## 缺口与范围

普通 `TaskService.claimTask` 已在 agent 锁前调用 `LockManagedRuntime`，取得 `installationNamespace(workspace, daemon)` 的事务锁。但 `CountRunningTasks` 只计普通 `agent_task_queue`，不知道协作运行已持久保留的容量。daemon 的内存 semaphore 不能阻止重启或另一进程再次领取。

§64 M09 先完成单智能体闭环，没有承诺多目标并行。服务端目前也没有可信的持久机器总容量；`max_tasks` 是调用方当次空槽请求，不能当机器上限。因此本切片明确采用协作运行独占 namespace：promote 在同一 namespace 锁内要求没有普通 active task 和未释放协作预约，普通 claim 在协作预约存在时保持 queued。其他 namespace 的普通行为不变。

授权/调度线新增 `collaboration_runtime_reservation`，其 `released_at IS NULL` 表示持续占用。未知证明、runtime/grant/task 行清理均不得释放或删除预约；释放只在从未许可的安全结清或真实 sealed proof 接收事务内发生。

主会话只负责 `agent.sql` 的容量统计/最终普通领取谓词、窄 namespace 查询、`task.go` 领取边界和对应测试。不改普通重试/业务状态/自动化，不新增机器总容量注册或依赖。sqlc 统一生成。

## 回归证明

- 同 agent 未释放预约计入容量；已释放预约不计入。
- 同 workspace/daemon 的其他 agent、重绑后的新 runtime 仍须等待；另一个 namespace 可领取。
- 预约没有可连接的 runtime/run/grant 行时仍计数，不能因清理失去证明边界。
- 事务屏障验证普通 claim 等待已持有的 namespace 锁，预约提交后 claim 重新检查并拒绝；不靠任意 sleep 决定竞态。
- 证明释放预约后，一次正常 claim 成功；普通单任务/批量领取已有回归继续通过。

未来多目标阶段可用经过验证的机器容量注册替代独占策略，不能仅删除本边界或依赖进程内 semaphore。

## 已验证的改动

新增测试先复现容量计数、agent/runtime/batch/SQL 领取和等待 namespace 锁后的重验六处缺口。实现仅修改 `agent.sql` 的现有 `CountRunningTasks` 和 `ClaimAgentTask`：保留预约计入 agent 容量，同 namespace 的预约阻止普通任务派发。沿用 `LockManagedRuntime`，无需修改 `task.go` 或新增服务抽象。

sqlc 对算术聚合默认推断为 int32，已给 SQL 加最终 `::bigint`，保留原有 Go int64 接口；没有通过改窄调用方计数规避问题。

Go 1.26.6 `-race` 的新增容量回归和既有并发领取、批量领取、runtime 权限回归共 17 项通过，无失败/跳过。证据为本轮临时目录中的 `ordinary-capacity-red.jsonl`、`ordinary-capacity-race.jsonl` 和 `sqlc-capacity-int64.log`。这证明普通领取侧；协作 promote 的反向竞争及完整运行链由后续集成验证负责。
