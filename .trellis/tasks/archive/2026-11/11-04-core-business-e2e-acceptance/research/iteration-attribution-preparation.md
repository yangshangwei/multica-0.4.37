# 单次千任务结转归因探针：编译完成，未执行

新增文件仅位于任务私有目录 `.omx/private/core-e2e-20261104/conditional/iteration-attribution-1/`，没有产品、SQL、冻结 P95 或依赖修改。候选保持 `1a32ca36c2c4f4455a01bdd39d0e0ab4db20cccc`。

`iteration_attribution_test.go` 通过 Go overlay 仅增加虚拟 `server/internal/handler/iteration_attribution_diagnostic_test.go`；实际 checkout 中该文件不存在。原有 TestMain 和所有产品实现不替换。`go test -c -p2 -overlay ...` 已 exit0，未执行 TestMain；源码与 Go/Node/Bash 身份前后相同，二进制没有 race。

## 实际探针合同

- 唯一测试名 `TestIterationClosureAttributionDiagnostic`，运行时精确筛选、count1、parallel1、5分钟包上限；测一次真正的 Apply end，不调用原35周期P95。
- 实际负载：1,000个todo、10项目、owner负责人、5优先级、每任务3/6标签；初始move/start/preview真实执行但不计入Apply span。
- 仅在Apply请求context带marker，pgx Query/Batch/Acquire tracer因此不计入准备、observer和提交后断言。Query完成事件包括驱动消费；完整batch一次计时，batchstatement只记数量。
- Commit使用真实产品路径；标记请求内要求一次真实commit，记录begin/commit实际trace项。已有原1msLock-onlyobserver在本测量期间启用，原限制仍在：它不证明无短锁或其他等待。
- 真实断言：千任务指针转向目标且rollover=1，source无任务指针，sourcecompleted/targetplanned，单个snapshot全部千项保留priority和3labels，exactrequest/operation只落库一次，agent_task_queue数量不变。
- 要求实际1,000活动task读、2,000participation读、7个千条membership batch/7,000条写；若实际不同诊断失败，不能拿源码推导替代实际计数。
- 保存每类query次数/时长、每batch时长、acquire、poolwait、process级GC/allocatedbytes；residual只叫未归因耗时，不叫CPU。错误归原事务owner回滚。提交后才做持久化断言，未伪造Commit→Rollback。
- 没有加入actualpreparedplan/statistics捕获或产品内部人工phase标记。先据现有驱动计时识别主导部分，再决定是否需要另一次有界计划探针。

## 隔离和入口

`run.py`复用已审 `backend/run-v4.py` 的冻结source/toolhash和具名事件核对，绑定该文件哈希。HOME/TMP固定在 Git 外 `/Volumes/artisan/e2e-private/core1a32-20261105/iteration-attribution-1/`，每次创建新子目录，拒绝Git祖先；Go caches显式，不继承数据库/发布/agent账号环境。运行前只读核验root分配库名称、752迁移和issue0；DBURL只在0600permit/childenv，不进命令或公开结果。所有DB创建/迁移由root负责，本runner无DDL。

Root审阅 `review-identity.json` 后，在该私有目录写0600 `root-permit.json`：`approved_by=root`、`candidate`、`quiet_ready=true`、`run_id=execution-1`、`review_identity_sha256`、`migration_count=752`、`database:{database,database_url,ready:true}`。运行前需明确资源窗口；现在未签发也未执行。

```text
python3 .omx/private/core-e2e-20261104/conditional/iteration-attribution-1/run.py run /Volumes/artisan/code/2026/multica-0.4.37/.omx/private/core-e2e-20261104/conditional/iteration-attribution-1/root-permit.json
```

输出只到新的 `execution-1`，存在就拒绝覆盖。`metrics.json`是一次有跟踪开销的closure分解，不是P95通过；原2,155.025ms失败永久保留。最后读取DB剩余连接，root仍负责最终资源清理。完整SHA和精确binary argv见同名JSON。
