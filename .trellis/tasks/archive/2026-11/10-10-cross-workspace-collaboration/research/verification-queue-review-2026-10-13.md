# 协作队列内部基础独立复核（2026-10-13）

## 范围与证据来源

本轮先独立只读复核，执行代理限流后由主会话明确授权本代理接管队列代码收尾与验证。审阅 `server/internal/service/collaboration_queue.go`、对应测试和 `server/pkg/db/queries/collaboration.sql` 队列段，对照当前 execution-contract E01–E08、design D06/D08、数据契约及 `.trellis/spec/server/security-boundaries.md` 全文。还定点检查协作授权器、密码版本检查、迁移约束和路由注册。

接管后本代理修改队列 go/test 两个文件及本报告，保留其他代理的编辑，没有修改 SQL、迁移或权限测试。先确认无遗留 go test 进程，再取得专属 service DB lane。最终执行全部 `TestCollaboration` 服务测试（包含权限代理新增用例），不以此前代理限流、未结束调用或旧版本通过作为最终证据。

初审基线 HEAD 为 `332e8ebda10f416e2bcfb807cb6f0800f796a527`；队列文件仍为工作区新增文件，不代表该提交已经包含本功能。

## 初审发现与交接

| ID | 发现 | 风险及建议 | 状态 |
| --- | --- | --- | --- |
| QR01 | `CollaborationQueueReceipt.CommandSequence` 原为 `json:"command_sequence,string"`，无 command 的 Stop/Resume 回执也输出 `"0"` | 这是序号缺席与有效正数身份的契约混淆。应省略未提供的序号，并验证超过 JS 安全整数的持久 JSON 重放。当前尚无该回执的公开 HTTP/UI 消费者。 | 已修复并通过回归；缺席序号 omitempty，command/control 的大整数均保留字符串 |
| QR02 | Stop 原先先把 run.progress_state 改为 ending，再仅根据 attempts=0、started=nil、proof=unknown 决定 never-launched | 若存储出现 `running` 或 success 与零许可的矛盾，原进度证据会先被覆盖，随后被结算。当前无许可/launch 入口可制造此窗口，但在缺失/矛盾身份下应保守保留屏障。 | 已修复；写 ending 前检查原始准备阶段、零许可、无 started/success；running/success/未知进度负例保留 run 与 active_run 屏障 |
| QR03 | Resume 的 active_run 指针和未结 run 不一致原先返回内部错误，并回滚操作记录 | 原行为已经拒绝恢复，没有释放屏障；但修复该不一致后，相同拒绝 key 不会保留原拒绝。应持久化 stop_unconfirmed，保持“被拒绝的 Resume 不成为延后意图”。 | 已修复；任一屏障存在均持久拒绝，测试修复 fixture 后原 key 仍重放拒绝 |

上述问题先发送主会话与执行代理；主会话移交写入责任后本代理完成修复。未新增抽象层或依赖，复用原 SQL 和事务锁。

## 已成立的代码边界

1. **没有隐式执行路径。** Submit 仅创建 collaboration_command 和 event/operation；没有普通 task enqueue。Resume 固定拒绝并且没有 active 写入。路由只注册工作台/详情/创建结果 GET，未公开队列写入、claim、permit 或 proof。
2. **当前权限在锁后重读。** 服务入口仅接受明确 JWT/PAT 人类主体；账号锁后持来源 workspace 存在性锁、issue、collaboration、session 和当前 participant，再重读账号/参与/角色。来源普通成员不能只凭成员身份执行队列操作；密码模式另验真实密码会话版本。
3. **请求身份不可由恢复状态悄悄升级。** 新请求匹配 lifecycle、participant、account epoch；已有操作先验证当前访问，再按 issue/actor/request key 查 kind+digest。重放已有收据不执行原回调；改 payload 拒绝。同一 actor/issue 的操作受锁串行化。
4. **取消仍然只取消 waiting。** command 按 session+UUID 加锁，作者/creator/当前 handler 重新判断；状态或协作版本不符拒绝。成功取消不改 control_version、队列状态、原作者/输入/seq；事件保存真实取消者与时间。它不暗中升级为 Stop。
5. **旧 Stop 不作用于新 run。** 当前存在 run 时同时核对 run UUID、fence、claim generation，之后再核对 control version；错误先返回，尚未写 pause 或 forced disposition。已结束旧 run 且没有新 run 时可以按当前控制版本暂停空闲队列，不反写旧 outcome。
6. **已许可事实维持屏障。** authorized_attempt_seq>0 时 Stop 只请求停止；started 报告缺失和业务 success 都不构成终结证明。run fence/launch nonce 保留，claim generation 推进使旧领取身份失效。SQL 的 never-launched 更新重复检查零许可、没有 started、未 settled 和 unknown proof。
7. **提交与重试边界合理。** event、operation、command/session 共享事务。SQL 锁/序列化失败整笔回滚后最多三次重试；Commit 返回任何错误都不自动重做，原 key 可查/重放。Rollback 使用独立有界清理上下文，避免请求取消后遗留事务。
8. **gate 锁序没有被正向路径绕开。** Resume 在 source 锁之前持 gate FOR SHARE；当前无成功 Resume，因此不能把“持锁并拒绝”的测试解释成已证明完整 E06 freeze/resume 竞争。

核查过但未列为问题：来源 status 的 category 在普通更新接口中不可变，归档保留已使用状态的语义；因此没有把普通显示字段更新误判成可在 Submit 期间切换为终态的漏洞。账号 fence 读取未单独加行锁依赖约定的账号治理 user-row fence，后续推进者必须遵守该锁序。

## 测试能证明什么

| 测试组 | 实际证明 | 不等于 |
| --- | --- | --- |
| ConcurrentSubmissionKeepsImmutableFIFO | 两个人类并发保存得到唯一连续 seq，paused/维护期间保留 waiting，原 key 不重复 command、变载荷拒绝 | 多调度 worker 的实际 FIFO 执行或 provider 并发隔离 |
| CancelUsesCurrentRolesAndPreservesOrder | 当前角色矩阵、作者/内容/seq 留存、取消重放、下一条保留 | 真实参与者移除/转交服务已接线 |
| CancelRechecksStateAfterSessionLock | 利用 pg_blocking_pids 确认取消在等待锁；竞争 fixture 提交 in_flight 后取消拒绝且不 Stop | 生产 promote 服务已经实现 |
| StopPreservesBarrierAfterAnyPermit | 已发许可、业务结果、started 与未发许可的数据库边界 | 原生进程树退出或 daemon sealed proof |
| ResumeRejectionNeverTurnsIntoDeferredIntent | 旧拒绝 key 在合法结算后仍拒绝；旧 gate epoch 不可复用；新请求也不启用未实现执行 | 正向 Resume、freeze 扫描、重新 enable 的完整协议 |
| StaleStopCannotReachNextRun | 旧 run UUID/fence 请求不改变后来 run 和控制状态 | 实际 claim/launch 与 Stop 的数据库并发覆盖 |
| RejectsStaleIdentityAndMachine | 人类白名单、source-member-only、关闭/未知状态，以及显式提升 epoch 后的旧请求拒绝 | 真实 disable/reset/revoke writer 会推进协作 epoch |
| EventRollbackAndUnknownCommitRecovery | event 失败事务原子回滚，真实提交后模拟丢回复不自动重试，原 key 只恢复一条 command | daemon fork/unknown-launch 账本证明 |
| SameKeyAndLargeSequence、BarrierAsUnconfirmed、ReceiptOmitsAbsentCommandSequence（新增） | 同 key 并发、command/control 超安全整数 JSON、无 command 不输出 0 序号、双屏障不一致的持久拒绝 | 未增加正向执行能力 |

以上用例均包含在接管后执行的最终服务测试中；见下方命令与结果。

## 必须保持的后续门禁

- 当前生产代码对 collaboration_account_fence 仅见 Ensure/Get，真实账号禁用、全局撤销、重置的 epoch 推进和持久失效 fanout 尚未接线。测试直接 SQL 提升 epoch 只证明比较逻辑；不能声称真实禁用→恢复不会复活旧意图。在开放 Submit 等接口前必须完成 E07。
- 当前 run 全部来自测试 fixture；没有 promoter/claim/finalize/permit、受限业务 token、终结专用 token、监督器账本/containment 或经过认证的 proof 接收。不能声明真实停止已确认、P4 完成或允许共享执行。
- owner/grant/runtime/installation、作者撤销、close/reopen 的全部 writer 尚须接入统一锁协议。SQL helper 的零许可字段必须成为所有未来许可 writer 的事务内权威，不允许外部启动后补记或 legacy fallback。
- 持久回执的 command/control 数字以字符串输出；新 HTTP 边界仍需严格 DTO/schema，不能把数据库生成结构直接当作完整前端控制快照。

## 定点回查与静态检查

接管后先运行新增负例：QR02 三项、QR03 两项、QR01 一项均失败，证据在 `/tmp/multica-collaboration.Qp1TA6/queue-review-red.log`。修正后在主会话提供的本地专用数据库 `multica_collaboration_20261013_87773`（迁移至 612）执行：

```sh
set -a
source /tmp/multica-collaboration.Qp1TA6/env
set +a
# cwd: server/
go test -race ./internal/service -run 'TestCollaboration' -count=1 -v
```

- 最终 exit 0；19 个顶层测试、29 个子项全部通过，0 FAIL、0 SKIP、无 DATA RACE，包耗时 10.295 秒。
- 日志：`/tmp/multica-collaboration.Qp1TA6/verification-service-final.log`。
- 同次验证包含权限代理新增的双访问依据撤权、六个关键写入失败时完整回滚、unknown commit 原身份恢复。
- `gofmt -l server/internal/service/collaboration_queue.go server/internal/service/collaboration_queue_test.go`：无输出。
- `git diff --check`：通过；新增文件另以上述 gofmt 验证。
- `go vet ./internal/service`：exit 0，通过。

最终队列文件 SHA-256：

| 文件 | SHA-256 |
| --- | --- |
| collaboration_queue.go | `a20d8ade2a2f7f4f1525b9e0ff88a52594c8476b1434edca304be380bdc9bd13` |
| collaboration_queue_test.go | `5f0c4b7d442dbb05baa6f05553bcea9eb5cdfbcb9fe3ef455e304fded4a07d53` |
| collaboration.sql（只读） | `042ea2d2e1628a63e2af06c14db8a3cc6a44c03ab58108d452f503aff4a571e7` |

service DB lane 已在测试完成后归还主会话。上述通过只证明当前内部基础，不覆盖本报告列出的 P4/P5/P8 剩余能力。
