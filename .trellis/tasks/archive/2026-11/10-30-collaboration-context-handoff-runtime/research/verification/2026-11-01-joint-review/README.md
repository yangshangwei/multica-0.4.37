# 65cd0599b 独立评审：handoff/checkpoint 失败路径的原始终止事实断言

- 评审对象：`65cd0599b`（test(collaboration): preserve original termination facts during combined failures），只看 `server/` 与 `.trellis/spec/` 部分。
- 评审开始时 HEAD：`2294654a2`。评审期间同伴提交了 `071f763e1`、`d53f1026f`、`d2fe54ab0`，HEAD 变为 `d2fe54ab0`。`git diff --quiet 65cd0599b HEAD -- server/` 和对 spec 文件的同一检查均为 exit 0，被评审代码没有变化。
- 该提交在 `server/` 下只改了 `host_handoff_checkpoint_test.go`（+13），没有改生产代码。evidence 目录下的文件不在本次评审范围。
- 环境：本机 `go1.27.0 darwin/arm64`（CI 用 Go 1.26），没有数据库，没有真实 Provider。所有测试都经 `scripts/go-test-with-agent-cli-guard.sh` 运行。
- 总体结论：**pass-with-notes**。断言与生产路径一致，不是空断言，没有数据竞争。注意事项见末尾，都不是缺陷。

## 1. 期望值是否与生产代码一致：一致

三个子测试都按生产路径追踪过，没有从测试反推。

**snapshot_refused**（fixture 返回 403 `business_capability_required`，`host_handoff_checkpoint_test.go:30-31`）
- `runHost` 在 `host_runner.go:101` 授权 attempt 1，`requestHostPermit` 在 `host_ledger.go:255` 写入 `AttemptSeq = len+1 = "1"`，`authorizeHost` 在 `host_ledger.go:283` 把阶段置为 `phaseAuthorized`。
- `hostHandoff`（`host_runner.go:185`）调用 `retryHandoff`。403 不属于 transport/408/429/5xx，`host_handoff.go:143-144` 直接返回，不重试。
- `host_runner.go:186-188` 执行 `stop()` 后进入 `recoverHost`。`host_runner.go:500-513` 对没有 termination 且处于 PermitPending/Authorized 的 launch 写入 `"not_started"`，cleanup 为 `unverified`。随后 `flushLaunches` 发送，fixture 回 `Authorized: true`（`host_runner_test.go:311,315`）。
- `sealHost` 的默认值是 `never_launched_proven`，只有存在 `managed_launch_ended` 时才改为 `host_exit_observed`（`host_ledger.go:448-453`）。期望 `not_started` / `never_launched_proven` 正确。

**binding_refused**（fixture 返回 409 `handoff_source_unavailable`，`:32-33`）
- 进入 `execute` 后，`BeforeLaunch` 依次执行 `authorize`（重放同一 permit，`host_runner.go:298-323`）和 `bindHandoff`（`host_runner.go:347`）。409 不重试（`host_handoff.go:143`），在 `burnHost`（`host_runner.go:371`）之前返回。
- `launch_lifecycle.go:342-349`：`BeforeLaunch` 出错时只执行 `l.fail` 和 `s.forget`，不调用 `Terminated`，也没有启动进程。launch 停在 `phaseAuthorized`，`retainedHostRun` 不把它标为 Unconfirmed（`host_ledger.go:193-196`），因此最终走 `host_runner.go:277` 的 `recoverHost`，写入 `not_started`。期望正确。

**fallback_snapshot_drift**（`reject_resume` 加上 `seedHostResume`，`:24-26,34-40`）
- 第一次 `execute` 带 `--resume`。fixture provider 以 exit 1 退出（`host_runner_test.go:57-60`）。`Terminated` 回调要求 `LaunchManagedEnded && WaitObserved`（`host_runner.go:388`），在 `host_runner.go:403` 以 `end.Kind` 写入 `managed_launch_ended`，ExitCode 为 nil，并立即 flush。
- `host_runner.go:224` 进入 fresh fallback，`host_runner.go:237` 第二次调用 `hostHandoff(..., handoff)`。fixture 先 append 快照再执行 mutate（`host_handoff_launch_test.go:92,98`），所以第二个快照的 `HandlerID` 被改写。`host_handoff.go:70` 的 `!reflect.DeepEqual(*previous, out)` 返回 `ErrProtocol`，第二次 execute 和第二个 permit 都不会发生。
- `recoverHost` 看到阶段已是 Reaped，跳过 not_started 分支，`sealHost` 得到 `host_exit_observed`。期望 `managed_launch_ended` / `host_exit_observed` 正确，`errors.Is(err, ErrProtocol)` 也成立（`host_runner.go:282`）。

协议校验（`pkg/protocol/collaboration_host.go:296-318,343-367`）允许以上三种组合。`never_launched_proven` 要求所有 launch 都是 `not_started`，`not_started` 要求 ExitCode 为 nil 且 cleanup 为 unverified。

## 2. Fixture 语义：key 正确，字段比较有意义

- `permits` / `terminations` 是 `map[string]...`，key 是请求体里的 `AttemptSeq`（`host_runner_test.go:106-107,249,310`）。这个值由 `host_ledger.go:255` 生成，即第 N 次 launch 为 `"N"`，所以 `"1"` 就是第一次 attempt 的序号。`proofs` 是 append 的切片（`:364`）。
- 有可能出现多条：permit 和 termination 每个新 attempt 各多一条；proof 每次 `/proof` 请求 append 一条，包括丢失回复后的重放。但新断言之前的 `:128` 已经要求三个长度都为 1，所以 `"1"` 和 `[0]` 都是唯一的条目。重放时 fixture 要求与原值完全相同，否则返回 409 并保留原值（`:244-248,305-309`），所以读到的是原始事实。
- 比较的字段都有意义。Identity 是可比较的 struct（`pkg/protocol/collaboration.go:20-33`）。AttemptSeq、AttemptNonce、Kind 可以识别出“换 attempt”和“换终止类型”两类回归。
- `reflect.DeepEqual(proof.Launches[0], termination)` 两边类型相同，都是 `protocol.CollaborationLaunchTermination`（`collaboration.go:131`，`collaboration_host.go:54-65`）。唯一的指针字段是 `ExitCode *int`，DeepEqual 比较的是指向的值，三条路径都是 nil（`host_runner.go:403,511`）。这个比较覆盖 EventID、LaunchID、Cleanup，因此能发现“恢复时重新生成了 termination 事实”这类回归（见变异 T6）。

## 3. 非空断言：可以失败（已在临时 worktree 中证明）

全部操作都在 `git -C /Volumes/artisan/code/2026/multica-0.4.37 worktree add --detach /tmp/review-65cd0599b 65cd0599b`（exit 0）中进行。每次变异前后都执行了 `git checkout -- .`（P1 对照之后执行 `git checkout HEAD -- server/`），并确认 `git status --short` 为空。

测试命令（下文记为 CMD）：
`bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race ./internal/daemon/collaborationrun -run '^TestHostHandoffCheckpointFailureCleanup$' -count=1 -timeout 3m -v`

| 编号 | 单行变异 | exit | 结果 | 是否符合预期 |
| --- | --- | --- | --- | --- |
| 基线 | 无 | 0 | 3 个子测试 PASS，`ok ... 2.522s` | 是 |
| T1 | 默认 `wantTermination` 改为 `"managed_launch_ended"` | 1 | snapshot、binding FAIL `:139 handoff failure changed the original attempt or termination fact: {... Kind:not_started ...}`；fallback PASS | 是 |
| T2 | fallback `wantTermination` 改为 `"not_started"` | 1 | 仅 fallback FAIL `:139`，实际值 `Kind:managed_launch_ended` | 是 |
| T3 | fallback `wantProof` 改为 `"never_launched_proven"` | 1 | 仅 fallback FAIL `:142 proof did not preserve the same original termination fact` | 是 |
| T4 | 默认 `wantProof` 改为 `"host_exit_observed"` | 1 | snapshot、binding FAIL `:142`；fallback PASS | 是 |
| T5 | `termination.AttemptNonce != permit.AttemptSeq`（与错误字段比较） | 1 | 三个子测试都 FAIL `:139` | 是 |
| T6 | DeepEqual 右侧改为只换了 `EventID` 的 termination 副本 | 1 | 三个子测试都 FAIL `:142` | 是 |
| T7 | `h.terminations["2"]`（不存在的 key） | 1 | 三个子测试都 FAIL `:139`，读到零值 `{ProtocolVersion:0 Identity:{DeploymentID: ...` | 是 |
| P1 | 生产代码 `host_ledger.go:448` 的 `state := "never_launched_proven"` 改为 `"host_exit_observed"` | 1 | snapshot、binding FAIL `:142`；fallback PASS | 是 |
| P1 对照 | 保留 P1，测试文件换成 `65cd0599b^` 的版本 | 0 | 3 个子测试全部 PASS，`ok ... 2.337s` | 是：旧断言漏掉了这个回归 |

P1 说明 `ValidateCollaborationHostProof` 对单个 `not_started` launch 也接受 `host_exit_observed`，因为 count 为 1，不触发 `count == 0` 的拒绝。只检查数量的旧断言发现不了这个回归，新断言可以。

## 4. 并发：没有问题

- `hostRunFixture.serve` 在入口处持有 `f.mu` 直到返回（`host_runner_test.go:182-183`）。`permits`、`terminations`、`proofs` 只在 serve 内写入（`:249,310,364`，已用 grep 确认没有其他写入点）。测试里读取时用的是同一个 `h.hostRunFixture.mu`。
- `launchHandoffFixture` 有自己的 `mu` 字段，遮蔽了嵌入结构的 `mu`（`host_handoff_launch_test.go:19-20`），测试显式写 `h.hostRunFixture.mu` 是正确的。handoff 处理器的加锁顺序是 h.mu 再 f.mu（`:73-75`），checkpoint 处理器在本地 `mu` 之后取 f.mu（`host_handoff_checkpoint_test.go:60,69`），两处都没有反向加锁，不会形成环。
- 不会有晚到的写入：`Run` 是同步的，stop 轮询协程在 `stopPolling` 中被 join（`host_runner.go:85`），checkpoint sender 在 `sender.finish()` 结束（`:247`），seal 在 `recoverHost` 的 defer 中同步执行。三个字段的写入都发生在对应响应编码之前，而客户端收到响应先于 `Run` 返回。
- 运行证据：CMD 改为 `-count=10 -timeout 5m`，exit 0，30 次子测试 PASS，0 FAIL，0 `DATA RACE`。`go -C server vet ./internal/daemon/collaborationrun` exit 0，`gofmt -l` 无输出。

## 5. Spec 文字：基本准确，一处措辞比测试宽

- “snapshot/bind refusal 保留 `not_started` 与 `never_launched_proven`”：与 T1/T4 覆盖的断言一致。
- “proof 必须包含完全相同的 termination 事实，而不只是一个 launch”：对应 `len == 1` 加 DeepEqual，准确。
- “pin 原始 permit identity、attempt、nonce”：测试比较的是 termination 与 permit 是否一致，permit 未被替换由 `permits == 1` 和 fixture 的重放检查（`host_runner_test.go:244-248`）保证。表述成立。
- 措辞较宽的一处：“fallback refusal after the first process ends” 泛指 fallback 拒绝，测试只覆盖 snapshot drift 这一种（`host_handoff_checkpoint_test.go:34-40`）。fallback 中途的 memory、brief 或 bind 失败不在这个测试里。按生产代码，这些路径都发生在第二次 permit 之前，结论应该相同，但本测试没有证明。
- 这段 spec 同时列出 `host_handoff_checkpoint_lifecycle_test.go`，而 Kind/State/DeepEqual 断言只在 `host_handoff_checkpoint_test.go` 中。lifecycle 测试（`:179-181`）只比较 identity 三元组。

## 注意事项（不是缺陷）

1. 身份断言是相对比较（termination 和 proof 对比 permit），没有直接对比 `h.claim.Identity`。生产代码中 permit 的身份来自 `local.record.Identity`（`host_ledger.go:267`），而 `Ledger.Lock` 校验 `claim.Identity == r.Identity`（`host_ledger.go:52`），所以两种比较实际等价。不过，如果 permit 和 termination 被一致地换成另一个身份，这几行发现不了。
2. Kind 断言有一部分已被 ledger 状态机约束：`observeHostTermination` 按阶段限制 kind（`host_ledger.go:328-334`）。这些断言的主要价值在于把契约钉在这个组合场景里，以及 State/DeepEqual 部分（P1 是旁证）。
3. 包内其他测试也断言了 proof State，例如 `host_knowledge_test.go:223`、`host_review_test.go:187` 的 `never_launched_proven`。P1 很可能也会被它们发现，但我没有对整个包运行 P1，这一点是根据代码推断的。
4. `:125-127` 和 `:131-133` 是两段相邻的临界区，可以合并成一段。按上面的分析，两段之间不会有写入，只是写法上的问题。

## 未检查的内容

- 提交里 `research/verification/2026-11-01-joint-current/` 下的 evidence 文件，以及 commit message 里的 “Joint race 40 items / tagged 80 items” 等计数。
- 整个包或整个 `make test` 的运行，Windows 和 Linux，CI 使用的 Go 1.26（本机是 1.27），真实 Provider。
- 没有完整阅读 `host_handoff_checkpoint_lifecycle_test.go`，只读了 `:170-250`。

## 清理

`git -C /Volumes/artisan/code/2026/multica-0.4.37 worktree remove --force /tmp/review-65cd0599b` exit 0。`worktree list` 中不再有这一项，路径已不存在。共享 checkout 中只新增了本报告，没有提交。
