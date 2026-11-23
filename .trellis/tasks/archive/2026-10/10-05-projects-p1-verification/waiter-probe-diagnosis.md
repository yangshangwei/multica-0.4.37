# Waiter negative control：隔离复测与阻塞归因

2026-10-05。未修改 `agent_builder_test.go`、waiter helper、实际锁等待断言或生产代码。

## 独立复测

- health私库，原 negative control `-race -count=20`：20/20通过，13.901s；`.omx/p1-waiter-isolated.log`。
- 同库，两个真实 send/rebind 正向交错测试加 negative control，`-race -count=5`：15/15通过，6.552s；`.omx/p1-waiter-related.log`。
- 归档 `0af59c5d5` 原server及原schema私库，原 negative control `-race -count=5`：5/5通过，5.266s；`.omx/p1-waiter-baseline.log`。
- waiter helper 和该测试相对 `0af59c5d5` 无源码差异。

原失败不能在独占数据库的正常执行中重现；没有证据表明P1改变了该锁探测算法。

## 可复现机制

`waitForWaiterBlockedBy` 精确判断 `holderPID = ANY(pg_blocking_pids(pid))`。它的语义是“确实有backend被这个holder阻塞”，不是“这个backend一定是当前测试准备的那一个”。negative control额外假设 ours holder 没有阻塞任何外部操作。

通过仅测试的 Go overlay，在原 negative control中加入一个最终ROLLBACK的外部清理事务 `DELETE FROM agent WHERE id=mine.BuilderAgentID`。这是套件清理的一种子集；删除carrier需要处理其被holder锁住的chat_session。记录到真实PG关系：

```text
ours holder PID 41220
unrelated holder PID 41221
cleanup waiter PID 41224: blockers=[41220], query="DELETE FROM agent WHERE id=$1"
ordinary unrelated waiter PID 41223: blockers=[41221], query="SELECT id FROM chat_session WHERE id = $1 FOR UPDATE"
```

此时相同negative control稳定报原错误（`.omx/p1-waiter-overlap-red.log`，1.332s）。helper没有把41221误认成41220；它发现了真的被41220挡住的另一个清理backend。“ours blocks nobody”前提已经被外部清理打破，原错误文字的“blocked by another backend”不足以描述这种情形。

overlay只用于诊断，保存在 `.omx/p1-waiter-overlap-overlay.{json,go}`；实际清理事务回滚、waiter均结束，仓库测试源码未改。没有真实agent CLI执行。

## 结论与边界

父任务报告当时同一verification库可能有两个handler测试进程重叠；它们共享固定fixture命名和清理范围。上述机制与此相符。原全仓日志未保存当时waiter PID/query，所以不能把那个具体进程认定为已经查实的唯一原因。

正确处理是维持独占测试数据库／避免两个handler TestMain同库重叠，并由父任务完整复跑。当前不应删除或放宽锁等待断言，也不应把negative control改成无条件通过。若独占重跑仍出现失败，应在失败时采集pg_stat_activity和pg_blocking_pids关系后继续定位；目前没有需要修改生产或helper逻辑的证据。
