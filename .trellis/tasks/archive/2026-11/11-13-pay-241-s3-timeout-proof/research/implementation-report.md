# S3 timeout fixture implementation

本增量只修改测试与测试专用fixture；不修改生产执行、权限、超时默认值或恢复策略。实际 API/Web/Electron 验收由主会话执行，当前代码和静态检查不能替代 T05/T06。

## 交付

- `host_timeout_recovery_test.go` 使用当前测试二进制启动真实受控进程，以1.5秒执行deadline触发真实超时。outcome/proof/both丢ACK分别重开磁盘Ledger并由不带Host executor的新Runner恢复原请求；比较原事件/证明/attempt与payload，恢复不新增claim、permit或launch。另测Stop先与timeout先的两种合法终结。
- `host_runner_test.go` 仅增加fixture请求计数和“outcome已记录后发生Stop”注入点，原行为默认不变。
- `collaboration_host_router_test.go` 的可选 `agent_timeout_ms` 只作用于测试poller。缺省180000，显式整数1000–180000，null/0/负数/小数/字符串/溢出均拒绝。`hold_receipt_acks` 在真实API已返回2xx后持续丢弃outcome/proof回复，保留真实状态、原请求与响应SHA；不伪造成功或改变请求。ready回显已采用的测试选项。
- `collaboration_host_router_boundary_test.go` 覆盖期限边界及真实loopback HTTP回答后的ACK屏障，403/503保持原响应。该HTTP测试不是实际产品DB集成。
- `startHostDaemon` 新增可选fixture配置及 `stopPreservingState` / `restart`：使用同一私有目录、runtime/管理身份和原账本，重新启动独立poller进程后解除ACK屏障。原close仍负责唯一的最终销毁；原Windows/POSIX Node回归保留。
- `collaboration-timeout-recovery.spec.ts` 使用closure Provider（自动CLI comment/memory写入）和15秒执行deadline；不释放Provider、不提前断callback。要求真实API失败/证明及本地pending账本，停旧进程并恢复原身份，评论/输入附件/结果/命令/内存分别保持；Web与实际Electron renderer展示原失败；明确新command成功后旧failure仍可查。回执新旧SHA必须一致，新进程不得对旧run发claim/permit。

## 首次失败与修正

1. `s3-timeout-impl-unit-first.jsonl`：新测试的5个子用例失败（含父计数7）。原因是新断言错误要求HTTP permit只调用1次；生产已有准备许可和物理启动前的同身份再次核权。改为记录恢复前计数并要求恢复后不增，唯一permit/launch/terminal事实仍各1。没有放宽生产规则或将多HTTP请求误作多执行。
2. `s3-timeout-impl-harness-red.jsonl`：新fixture接口尚未定义的编译失败，随后测试fixture实现通过；不把它标为已有生产漏洞。
3. 首次ESLint使用core配置直接检查E2E，既有Playwright/pg测试devDependencies被误判；本次E2E检查显式允许根包已声明的devDependencies。同时修正新spec的finally直接throw，保留原body错误并完成所有清理后再报告。
4. `s3-timeout-impl-stop-final.jsonl`：一次命令误写私有config路径，在读取配置时退出，测试未执行。正确路径重跑保留为final2；未连接数据库。

## 执行与限制

命令、实际测试名、退出码、日志及SHA见同目录 `implementation-results.json`。所有Go运行通过主会话分配的no-DB runner，以仓库外HOME/TMP和显式自建程序执行；未执行agentintegration、用户安装的模型CLI、数据库创建/迁移、生产发布或资金操作。未运行全仓长测试。

本增量默认代码边界为六个测试/fixture文件。原始受控进程、HTTP fake与实际API验收是三种证据层级；只有主会话执行新E2E后，才能决定T05/T06及本增量是否关闭。输入附件保留不能推导Agent输出附件上传幂等，S3其他恢复/阻塞/通知与父任务24用例仍开放。
