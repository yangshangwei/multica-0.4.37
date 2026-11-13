# S3 超时证明增量：独立规划审查

结论：**可开始实施，规划审查通过。** 主会话已在 design.md 的最小实现第3项纳入同目录重启与ACK故障barrier，并增加Stop两种顺序的独立评审补充。无新增产品待决项。本增量没有改变生产超时、授权或恢复策略，不依赖 S2 的职业选择。T01–T06 保留，完成本增量不关闭 S3 或 PAY-241。

- 审查人：原生 `trellis-check`，`/root/s2_plan_check`。
- 日期：2026-11-13。
- 范围：本增量PRD/design/implement及6项curated context、实际Provider/runner/poller/E2E fixture路径。只新增本审查文件，不修改产品、规划、其他任务或Git。
- 结论绑定 `plan-review.json` 内的文件摘要；这是规划审查，不是已运行测试的通过证据。

## 已核实可行性

1. `server/pkg/agent/claude.go:64–65` 将 `ExecOptions.Timeout` 交给 `runContext`；`agent.go:142` 的正值是 `context.WithTimeout`，确实是执行wall-clock期限。`stream_json_result.go:130–133` 将DeadlineExceeded表示为timeout；`collaborationrun/host_recording.go:210–230` 保留failure与原错误，不需要新增运行状态。
2. `newHostRunFixture` 使用当前测试二进制作为受控Provider，wait模式先写真实启动记录再等待最多8秒（`host_runner_test.go:82`、`:132–157`）；改本用例的ExecOptions.Timeout即可触发真实超时。应先观察启动，再断言timeout来源及真实managed termination，不能仅看耗时或EOF。
3. 测试poller硬编码3分钟（`server/internal/daemon/collaboration_host_router_test.go:136`），closure Provider callback期限150秒（`e2e/fixtures/collaboration-multi-account-provider.go:241`）；当前“不release”先得到回调失败的判断正确。仅测试输入新增有界期限可以修复该验证缺口。
4. 现有fixture有lost outcome/proof reply与原payload比较（`host_runner_test.go:310–370`）。`Runner.recoverHost` 为phaseProof优先补proof，phaseAcknowledged不再Claim/Permit/启动Provider（`host_runner.go:481–553`）；可重开同一磁盘Ledger，并记录恢复前后的所有请求与启动计数。
5. 超时不属于自动fresh-session fallback条件；`host_runner.go:225–248`只允许明确resume rejection触发单次新session。测试应断言该既有策略保持。

## 已纳入设计的两项夹具合同

### G01 / 已解决于设计：重启必须保留原目录、账本和身份

当前 `startHostDaemon` 每次都 `mkdtemp`（`e2e/fixtures/collaboration-host-runtime.ts:218–219`）；`close()`读取安全摘要后删除整个目录（`:320–335`）。直接close再start测试不到旧账本恢复，既可能空跑成功，也会丢失原证据。

最小修订：在同一test-owned fixture对象内分开“停止这个poller进程”“以同目录/同安装运行身份重建poller”“最终关闭callback并清理目录”。重启保持原Ledger目录、Provider/CLI字节、callback、runtime/installation与已准入配置；不要重新enroll或伪造新的nonce。只有最终cleanup执行一次目录删除。保留每一代poller的安全HTTP摘要、原账本指纹与启动计数；旧版本 `close()` 的默认生命周期保持。

单测同样要调用 `OpenLedger` 重开原路径，构造新的Runner/Client；不能只复用旧Runner对象。恢复后的第二轮应继续无新Claim/Permit/启动，确认不是依赖原内存状态。

### G02 / 已解决于设计：ACK丢失须停在明确的待恢复账本

E2E poller持续执行恢复轮，单次丢回复可能在重启之前自行补齐。现有host transport只观察claim/permit/termination/proof，尚未记录outcome（`collaboration_host_router_test.go:56–60`）；如果不补可见性和故障barrier，T02可能只是一次正常完成后重启。

最小修订：使用测试专用transport故障，在**真实API已提交响应后**阻断选定run的outcome/proof ACK；保存安全请求身份与原body摘要，保留原local pending状态，直至旧poller结束。新poller解除该故障并补同一请求。不能先拦掉请求再声称丢ACK，也不能直接写数据库/账本来构造成功终结。重启前后均需保存pending/acknowledged阶段和相同run/attempt/nonce摘要/event/receipt身份；无新command、Claim、Permit或Provider visit是分别计数的断言。

允许在最小单测中用既有httptest lost-reply机制证明接口重放；实际API E2E仍须独立记录其真实提交与丢回复边界。故障只在 `_test.go`/E2E fixture，通过显式参数启用，默认完全关闭。

## 实施与验收注意项（不新增产品范围）

- **Stop竞争按两个顺序证明。** timeout先、Stop先分开，用已存在launch/termination/outcome事件barrier固定边界。Stop先时新业务outcome可能被正确拒绝，不能要求无条件存在timeout failure：`host_runner_test.go:338–348`的fixture也会拒绝Stop后的新outcome。应断言真实Stop/终结事实与零重复写入；timeout先已提交的failure必须保持不变。需要重试成功时使用新的明确run/command，保留旧失败，不能修改旧结果或自动Resume。
- **短期限只改测试输入。** 建议 `agent_timeout_ms` 缺省180000，显式值为1000–180000的整数；0、负数、超限、非整数/错误类型及显式null应拒绝，且在Mkdir/register等工作状态创建前校验。只有缺省使用原3分钟。Node边界与Go测试poller都验证，不新增生产环境变量或CLI选项。
- **给真实启动留预算。** E2E期限可选10–15秒，仍远低于150秒callback和5分钟harness期限。必须观察Provider visit与原评论后才认定超时证据；若执行在准备/启动前已失败，用例失败，不能降格为PASS。测试外层context留足清理/证明时间，不能用外层cancel冒充执行timeout。纯fixture的期限应低于wait模式8秒自然结束。
- **用匹配的Provider协议。** closure Provider已在callback前通过run-local CLI写memory/comment，comment ID关联run（`collaboration-multi-account-provider.go:188–212`）。它等待单字节`r`（`:261`），不能调用另一种host Provider的 `daemon.act(...)` 脚本动作协议。直接核对其原评论即可，无需更改S1五次运行。
- **分开副作用与证据。** outcome不是comment；输入附件的相同ID/字节保留也不证明Agent输出附件上传。T05分别报告实际comment、memory、outcome、proof、input attachment的ID/计数/摘要；未发生的输出附件写入明确不计入覆盖。只导出allowlisted摘要，原Ledger含凭据不得附入公开报告。
- **清理也是门禁。** 故障解除/各代poller退出/最终proof/gate恢复需finally保护；失败时先保存证据。只保留实际可观测managed termination保证，不宣称所有任意子进程或模型工具副作用exactly-once。没有Desktop实际UI用例时，构建证据与实际界面证据分列。

## 文件归属与检查结果

原计划列出的现有hook文件均存在；新增 `host_timeout_recovery_test.go` 和 `collaboration-timeout-recovery.spec.ts` 是计划新增文件，当前缺少是正常状态。上述restart/fault能力可以保持在已分配的 `collaboration_host_router_test.go` 与 `collaboration-host-runtime.ts` 内，不需扩大生产边界。

- `task.py validate`：PASS；implement/check各6条，路径全部存在。
- 实际调用路径检查：PASS；确认真实timeout、fixture8秒wait、3分钟/150秒关系、local ledger恢复和清理行为。
- Lint / TypeCheck / Go normal/race / E2E：NOT_RUN；尚未新增测试代码，不宣称产品或T01–T06通过。
- Findings（fixed）：本代理没有修改实现或规划；G01/G02已由主会话纳入设计，经本轮复核解决。可启动此独立增量，无需再征求职业产品规则。最终产品验收仍须实际执行T01–T06及独立检查。
