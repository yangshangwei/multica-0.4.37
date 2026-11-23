# 技术设计

## 来源

参见父S3 research/recovery-gaps.md，以及S2 research/verification-plan.md。现有HostRunFixture使用当前测试二进制作为进程并有原permit/outcome/proof账本；HostRouter测试poller默认AgentTimeout=3分钟，closure Provider回调150秒先失败，不能据其不release宣称执行超时。

## 最小实现

1. 在collaborationrun包新增定向测试，复用newHostRunFixture(wait)。显式较短ExecOptions.Timeout，先确认capture有实际启动，再等待超时。检查outcome原因/终结证明及原ledger；模拟首次结果或proof响应丢失后重建Runner并恢复同一原请求，数实际启动、每种结果、业务comment入口分别断言。
2. 测试专用collaborationHostRouterInput增加可选agent_timeout_ms；仅允许有界正整数（建议1000至180000毫秒），缺省维持180000；非法值在创建工作状态前失败。该字段只存在_test.go和E2E fixture，不能影响生产CLI/配置。测试说明真实起止点，避免机器负载导致在启动前超时。
3. startHostDaemon或等价fixture增加显式option向测试poller传递，旧调用保持原值。E2E使用足够完成启动的短deadline（例如5秒），远小于callback150秒；先收到visit且不release，验证进程执行超时。显式重启daemon继续原ledger，仅补原回执，不创建新command。现有startHostDaemon每次创建目录且close删除ledger；本增量新增测试专用的stopPreservingState/restart边界，复用同一目录、runtime和管理身份，最终只清理一次。丢回执使用明确fault barrier：实际API已经提交才拦截响应，旧poller持续看不到ACK，待旧进程退出且pending ledger存在后恢复网络、启动新进程；不能把同进程内部重试冒充重启恢复。
4. 集成新canonical e2e/collaboration-timeout-recovery.spec.ts，复用已有账号/管理凭据/host gate/bootstrap/helper，不复制整套fixture。保留一条控制结果comment并单独计数；禁止将outcome当作comment或支付副作用。必要恢复操作调用已有API，不新增产品接口。

## 归属

Implement独占server/internal/daemon/collaborationrun/host_timeout_recovery_test.go、host_runner_test.go必要fixture钩子、server/internal/daemon/collaboration_host_router_test.go及其边界测试、e2e/fixtures/collaboration-host-runtime.ts、e2e/collaboration-timeout-recovery.spec.ts。其他产品文件未经具体必要性复核不修改；发现产品bug报告主会话。其他代理/任务共享工作区，不得回退其改动。

## 验证

纯受控进程包无需DB；测试进程HOME/TMP在私有仓库外目录，不能调用用户已登录CLI。实际API测试另分新库，password API+production Web和对应Desktop构建按S2环境规范记录，fixture开启/恢复host gate。编译当前daemon测试二进制和closure Provider，不复用旧版本。用例未覆盖Desktop实际界面时明确标注，不因共享代码就宣称已验证。

## 独立评审补充

Stop竞争分超时先与Stop先两个受控顺序。Stop先时允许协议规定的无business outcome终结，只断言同一run/permit/proof、无重复写及终结一致；不得把所有取消伪称执行超时。缺少结果的合法分支与记录了timeout failure的分支分别报告。
