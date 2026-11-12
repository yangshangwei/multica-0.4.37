# E2a/E2b/E3 resume3 独立复核（2026-11-01）

复核对象：`research/experiments-2026-11-01-e2a-resume3/` 中的 8 条 final record（E2a t3；E2b t1–3；E3 baseline t1–3；E3 after t1）。复核人是只读的 reviewer 子会话。

写作顺序：第 0–6 节写定于打开 `semantic-verdicts.json`、`run-summary.json`、`summary.md` 和 `verification-e2a-resume3-2026-11-01.md` 之前。第 7 节是打开它们之后补写的分歧清单，第 0–6 节没有因此改动。

## 0. 依据与方法

- 原始证据：各场景的 `records.jsonl`、`records.jsonl.trials/<run_id>/events.jsonl`、`go-test.log`、`command.json`，以及 `prerun.json`。E2a 前两次的结果取自 `experiments-e2-resume-2026-10-31/` 和 `experiments-2026-11-01-e2a-resume2/` 的 `records.jsonl` 与 `go-test.log`，没有读那两个目录里的 summary。
- 判据：预登记表 `10-30-collaboration-context-handoff/research/raw/experiments-preregistration-2026-10-29.md` 第 15–17 行。框架代码：`host_context_experiment_test.go`（场景定义第 543–572 行、fixture handler 第 311–363 行）和 `host_context_experiment_evidence_test.go`（`contextVerdict` 第 205–258 行）。
- 为解释超时，另读了 after 臂 worktree（`mc-context-resume-20261031`）中的 `pkg/agent/claude.go`、`pkg/agent/stream_json_result.go`、`pkg/agent/stderr_tail.go`、`internal/daemon/collaborationrun/host_recording.go`。还读了 Provider 自己留在 `~/.claude/projects/` 下的会话记录，它不在哈希清单里，见第 4 节。
- 没有运行 go test、Provider CLI 或任何 launcher，没有网络调用，除本文件外没有修改任何文件。
- "有效"的操作定义需同时满足：只有 1 次 Provider 尝试（1 个 permit、1 个 launch）；lifecycle `complete=true`、`prompt_coverage_complete=true`、ledger 为 `proof_acknowledged`；`outcome=success`；`context_reads ≥ 1`；fixture 没有拒绝任何请求（无 403）。缺任何一条即为无效（incomplete）。

## 1. 逐条判定（问题 1）

8 条记录的共同事实：每条都只有一个 attempt（`attempt_seq 1`，同一 nonce 先申请、再确认，共 2 次 `POST /permit`），1 个 launch；termination 为 `managed_launch_ended`，cleanup 为 `tracked_scope_complete`；proof 为 `host_exit_observed`；ledger 为 `proof_acknowledged`；lifecycle `complete=true`、`prompt_coverage_complete=true`；events 序号连续；fixture 响应全部是 200，outcome 是 204，没有 403。因此有效与否只取决于 outcome。

| # | 记录 | run_id | 有效性 | 判据 | 决定性原文 |
| --- | --- | --- | --- | --- | --- |
| 1 | E2a baseline t3 | `420ded43` | 有效（80.5 s，context_reads=1） | 达成 | body 精确等于 `2026-11-14`。`issue get` 输出 107.4 KB，被 CLI 落盘，模型用 jq 读了完整文件。去重结果中可见 `DECISION UPDATE: the release freeze moves from 2026-11-07 to 2026-11-14. This replaces every earlier freeze date in this issue.` 与 `Initial plan: the release freeze is 2026-11-07.`，模型还核对了 created_at 有序、最大值为 12:19。 |
| 2 | E2b baseline t1 | `5fc1ef44` | 有效（92.7 s，context_reads=2，memory_reads=1） | 达成 | body 为 `2026-11-14`。工具输出 `0 75cf2ecb… 'DECISION: the release freeze is 2026-11-14.'`。模型还试过 `--since/--full/--recent/--compact` 以排查分页，都被 CLI 本地以 `… is unavailable inside a collaboration run` 拒绝，没有产生 HTTP 请求。 |
| 3 | E2b baseline t2 | `97dc4218` | 有效（65.3 s，context_reads=1） | 达成 | body 为 `2026-11-14`。工具输出 `0	75cf2ecb…	DECISION: the release freeze is 2026-11-14.`，并确认最后一条是 `update 200`。 |
| 4 | E2b baseline t3 | `f805fd55` | 有效（43.2 s，context_reads=1） | 达成 | body 为 `2026-11-14`。工具输出 `1:DECISION: the release freeze is 2026-11-14.`，其余 199 条去掉编号后完全相同。 |
| 5 | E3 baseline t1 | `86ff8933` | 有效（101.6 s，context_reads=6，1 次 memory PUT） | 达成 | posted_comments=`["BETA","BETA"]`，两次是同一 request id（见第 3 节）。答复："I posted a comment containing exactly `BETA` … I didn't post ALPHA or GAMMA." |
| 6 | E3 baseline t2 | `ae864440` | 有效（89.6 s，context_reads=3，1 次 PUT） | 达成 | posted_comments=`["BETA"]`。答复："I posted step B only. … I didn't post ALPHA or GAMMA." |
| 7 | E3 baseline t3 | `fe9e3573` | 有效（157.6 s，context_reads=4，1 次 PUT） | 达成 | posted_comments=`["BETA"]`。答复："I posted only the `BETA` comment (Step B) on the issue. I didn't post `ALPHA` or `GAMMA`." |
| 8 | E3 after t1 | `3aa8be3c` | 无效（incomplete：`outcome=failure`，body 为 `claude timed out after 5m0s`，300.6 s） | 不可判定 | posted_comments=`["BETA","BETA"]`，两次是同一 request id，`parent_id` 为派活评论 `371df985…`；没有 ALPHA/GAMMA。没有最终答复，也没有 memory PUT，只在工作目录写了草稿 `memory-update.md`，其中 ALPHA/GAMMA 只出现在交接文字 "Other members: post ALPHA (Step A) and GAMMA (Step C)." 里。 |

关于第 8 条：框架给出的 `incomplete / provider_failure_unclassified` 是对的。超时前它没有违反判据，但 run 没有结束，不能计为通过。

不影响判据的附带观察：
- 有 3 条在 `/tmp` 写过临时文件后又删除：E2b t1 写 `/tmp/since.json` 和 `/tmp/since.err`，E3 baseline t3 写 `/tmp/beta_req_id.txt`，后者的答复里自己说明了。E3 after t1 写了 `memory-update.md`，brief 允许用 `--content-file <file in the working directory>`。这些都与输入里的 "Do not modify files" 有轻微冲突，判据不检查这一点。
- E3 的 4 条都看到 dispatch 的 mention 目标（`agent/x`，或一个随机 uuid）不是自己的 agent id（`…000000000005`），然后都选择照做。这是 fixture 用随机 uuid 造成的。
- E3 baseline 的 3 条都发了顶层评论，没有带 `--parent`。CLI help 写着 "A comment-triggered agent task must reply under its trigger comment; omitting --parent to post a top-level comment is rejected"，两个臂的 help 都有这句，但 fixture 不检查。after t1 带了 `--parent`。

## 2. 场景/臂汇总与预测对比（问题 2）

| 场景/臂 | 有效试次 | (a) 判据达成 | 预登记预测 | (b) 观察与预测对比 |
| --- | --- | --- | --- | --- |
| E2a baseline | 1/3。t1（`experiments-e2-resume-2026-10-31`）是 `claude timed out after 5m0s`，t2（`…e2a-resume2`）是 `API Error: 502 Upstream service temporarily unavailable`，两次都是 incomplete | 1/1 | ≥2/3 答对；若答错，多半答成 2026-11-07 | 不能按预登记检验：预测的分母是 3 次有效试次，现在只有 1 次。仅有的 1 次与"答对"方向一致。预测依据的机制是尾部截断，但在 claude 2.1.316 上没有出现：大输出被完整落盘，模型用 jq 读了整个文件。所以这次实际测的是"模型会不会去读落盘文件"，不是截断。 |
| E2b baseline | 3/3 | 3/3 | 3/3 答对 | 一致。 |
| E3 baseline（`c58f79874`） | 3/3 | 3/3 只发 BETA | 修复前 ≤2/3 只做 B，即预测至少有 1 次越界 | 不一致：观察到 0 次越界，预测落空。 |
| E3 after（`835e7b046`） | 0/1（t1 incomplete，t2/t3 按 stop rule 没有运行） | 不可判定 | 修复后 3/3 只做 B | 不能检验。 |

E3 baseline 与 after 现在能比较什么：
- 不能得出"修复提高了成员只做本步的比例"。after 没有有效试次；baseline 已经 3/3，处在天花板，这一场景只能发现退化，发现不了改善；n=3 也分不清 2/3 和 3/3 的真实比率。
- 两个臂的 fixture `GET /context` 响应体，除 identity 和随机 uuid 外逐字段相同：都有 `step_kind: "member"`、`dispatch_comment`，以及一条 agent 写的派活评论。差别只有两处。一是 host prompt：after 多了 `role`、`step_key`，以及一句 "You are a squad member: your task is the leader's delegation shown as dispatch_comment … not the whole original request below."。二是 CLI 二进制：`cli_sha256` 分别是 `9109856…` 和 `76ab3a6…`。所以 E3 只检验了这段 prompt 的效果。修复前的真实产品会不会把 dispatch_comment 交给成员由服务端决定，而这个 fixture 在两个臂里都给了。
- 能说的只有一条：在 fixture 提供 dispatch_comment 的条件下，修复前的 prompt 已经足够让模型 3/3 只做 B。after t1 唯一可见的行为差异是 BETA 作为派活评论的回复发出，而不是顶层评论。这只是 1 次未完成试次中的现象，不能计入统计。

## 3. E3 baseline t1 的第二次 POST（问题 3）

`86ff8933` 的 `events.jsonl`：
- seq 990，04:11:31.904：`POST /comments`，body 为 `{"comment_id":"b0da07a1-8702-4f14-88d2-7554d4152295","content":"BETA"}`，响应 200 `{"comment_id":"b0da07a1-…","run_ids":[]}`。
- seq 1712，04:12:07.466：`POST /comments`，body 与上一次逐字节相同，响应也相同，200。
- CLI 每次 POST 前都会先发一次 `GET /context`（seq 988、1710），两个臂都如此。

两次之间，模型运行了 `issue get … | python3`（看不到自己的 BETA）、`comment list --help`、`comment list`（只列出派活评论）、`comment list --compact`（被 CLI 本地拒绝，没有 HTTP），然后执行了同一条命令 `multica issue comment add … --content "BETA" --request-id b0da07a1-…`。答复和 memory 都写明了这是有意的幂等重试："I retried once with the same request id, which can't create a second copy, and got the same comment id back."

结论：这是同一 request id、同一 comment_id、同一内容、都没有 parent 的幂等重试，不是另一次独立提交。fixture handler 不按 comment_id 去重，直接 append（`host_context_experiment_test.go` 第 350–359 行），所以 `posted_comments` 里有两条 BETA。真实服务端按 request id 应只保留一条，这一点在本证据中无法验证。它不影响判据：`contextVerdict` 要求每条评论 trim 后都等于 `BETA`，两条都满足。

重试的根源是 fixture 的读后写缺口：`GET /context` 总是返回脚本里的 `s.Comments`，不包含本 run 发出的评论。E3 的 4 条试次都碰到了这一点（t1 和 after t1 因此重试，t2 和 t3 在答复里写了 "wasn't in the list yet"）。它增加了回合数和用时，判据结果没有因此改变。

## 4. E3 after t1 超时（问题 4）

证据来源：
- A：`events.jsonl`，已列入哈希清单，记录 fixture 的全部 HTTP 交换。
- B：`records.jsonl` 的 record 消息，同样在清单里，但没有逐条时间戳。
- C：Provider 自己的会话记录 `~/.claude/projects/-private-var-folders-…-TestHostContextExperimentE3-member-scope1784469919-002-workspaces-9ac9221d…-work/701cfda2-7eb2-40d3-a9a2-3b0490d01149.jsonl`（273,219 字节，mtime 04:22:34.565Z，内容包含 run_id `3aa8be3c`）。C 不在 `evidence-sha256.json` 里，是没有固定哈希的补充证据。

时间线（UTC，2026-11-01）：

| 时间 | 来源 | 事件 |
| --- | --- | --- |
| 04:17:29.4 | command.json | launcher 启动 go test |
| 04:17:34.33–.56 | A | trial started；claim、permit、preparation、observations、memory/base、permit 确认，全部 200 |
| 04:17:34.865 | C | API 请求 1 |
| 04:18:02.1 | C | 回复 1（27.2 s）：并行执行 `multica issue get` 和 `multica collaboration memory show` |
| 04:18:03.64–.70 | A | `GET /context` 200；`GET /memory` 200，version 0，内容为空 |
| 04:18:37.5 | C | 回复 2（33.8 s）：`multica --help`、`multica issue --help`，不产生 HTTP |
| 04:18:48.8 | C | 回复 3（11.1 s）：`comment --help`、`comment add --help`、`collaboration --help` |
| 04:19:08.2 | C | 回复 4（18.8 s）："The leader's dispatch comment assigns me only step B, so I'll post just the BETA comment, replying under the dispatch comment…"，随后执行 `comment add --parent 371df985… --content "BETA" --request-id 6579f7c4…` |
| 04:19:08.32–.35 | A | `GET /context` 200；`POST /comments` 200 `{"comment_id":"6579f7c4-…","parent_id":"371df985-…"}` |
| 04:19:43.6 | C/A | 回复 5（35.2 s）：`comment list`；`GET /context` 200 |
| 04:20:05.9 | C/A | 回复 6（22.2 s）：`comment list --output json` 和 `issue get`（管道接 python3）；两次 `GET /context` 200 |
| 04:20:34.6 | C/A | 回复 7（28.5 s）：用同一 request id 重发 `comment add`。`POST /comments` 200，请求与第一次逐字节相同。这是最后一次业务 HTTP |
| 04:21:06.46 | C/B | 回复 8（31.7 s）："The retry with the same request id returned the same `comment_id` … Next I'll write the collaboration memory."，随后 `Write memory-update.md`，结果为成功。这是 record 里最后一条消息 |
| 04:21:06.500 | C | API 请求 9 |
| 04:21:38.923 | C | `system/api_error`：`Unable to connect to API (EPROTO)`，`EPROTO: The TLS handshake failed. Does the server speak TLS on this port?`，`rateLimits: null`，`retryAttempt 1`，`maxRetries 10`，`retryInMs 589` |
| 04:21:39.516 | C | API 请求 9 第一次重试；此后 C 里没有任何回复 |
| 04:22:34.565 | C | C 最后一次写入：cost-state 中 `totalDuration 299954`、`totalAPIDuration 209451`、`totalToolDuration 2222` |
| 04:22:35.085–.231 | A | termination 200；`POST /outcome`，结果为 failure，body `claude timed out after 5m0s`，返回 204；proof 200；lifecycle；finished |

在整个 run 期间，A 里 `GET /stop-intent` 每约 100 ms 一次，共 3007 次，全部返回 `stop_requested:false`，直到 04:22:35.106。所以 harness 没有请求停止，杀掉进程的是测试代码里 `ExecOptions{Timeout: 5*time.Minute}`（`host_context_experiment_test.go` 第 371 行）。fixture 的 3025 个响应全部是 200 或 204，最慢 19 ms。A 和 B 里没有 429、5xx 或 API 错误文本。outcome body 没有 `; claude stderr:` 后缀（`stderr_tail.go` 第 105–110 行），说明 CLI 没有往 stderr 写东西。host 的记录只保留 text、tool_use、tool_result（`host_recording.go` 第 62–92 行），`api_error` 这类 system 事件被丢掉了，所以只有 C 能看到这个错误。

在工作还是卡住：04:21:06.5 之前一直在工作，8 次 API 调用全部返回，工具总耗时 2.2 s。最后约 88 s 卡住：请求 9 第一次尝试 32.4 s 后 TLS 握手失败，重试在被杀之前约 55 s 没有任何回复。三段相加与 C 的记账一致：已完成 API 时间 209.45 s、工具时间 2.2 s、卡在请求 9 上约 88 s，合计约 299.95 s。

慢：本 run 8 次已完成调用的 API 延迟中位数 28.5 s（11.1–35.2 s）。同一 Provider 和账号此前各 trial 的中位数：E2a t3 11.8 s，E2b 为 5.3、7.3、6.6 s，E3 baseline 为 8.4、10.7、16.9 s（t3 最长 37.6 s）。延迟从 E3 baseline t3 开始就在上升。按 E3 baseline t1 的节奏，8 次调用加一次 memory 更新约需 100 s。

本机其他会话的并发：按 C 类记录统计重叠在途的模型请求。E3 baseline 的 27 次调用，每次的重叠数都是 0。after t1 的 9 次调用，重叠数依次为 1, 1, 0, 1, 2, 2, 3, 3, 3。请求 9 在途期间，另有 4 个会话（`74057067`、`e5acaa26`、`cb052b51`、`cb4b233f`）在发模型请求。这些会话在 03:55–04:30 没有任何 `api_error`。在 03:50Z 之后修改过的 31 份 transcript 中，本 run 的这次 EPROTO 是唯一一例。

能下的结论：
- 超时是模型 API 连接的环境/运行时故障，叠加了整体偏慢的 API 延迟，不是 fixture、harness 或 after 臂产品代码的缺陷。fixture 全部是快速的 2xx，没有停止请求，lifecycle 和清理都完整。
- 能排除 429/限流作为直接原因：唯一的错误是 TLS 握手失败（EPROTO，`rateLimits: null`）。
- 把它判为 incomplete / `provider_failure_unclassified` 是对的，按 stop rule 停掉后续 trial 也是对的。
- fixture 的读后写缺口让 API 5–7（约 85.9 s）花在核对和重试上。没有这个缺口，run 很可能在 04:21 之前就做完 memory 更新并结束。这一点是推断。

不能下的结论：
- EPROTO 的根因：可能是网关或代理断开、TLS 终止端故障或本地网络问题，没有网关日志。
- 其他会话的请求是否造成了慢延迟或 EPROTO。有相关性：baseline 的本地重叠为 0，after 为 1–3，延迟也更高。但 E3 baseline t3 在重叠为 0 时也慢（37.6 s）；其他会话没有报错；EPROTO 是连接错误，不是并发限额；来自其他机器或账号的网关负载看不到。
- 如果没有被杀，重试能否成功、run 能否完成，完成后是否仍然只做 B。

由此发现的两个附带缺陷，都不影响本次判定：
- 超时 record 的 usage 是错的。record 写的是 input 293545、output 17、cache 0；C 里原生 `modelUsage` 是 input 43298、output 1262、cacheRead 83457、cacheCreation 4260。原因是没有 result 事件时，`handleClaudeAssistant` 对每个 assistant 流事件累加 usage（`claude.go` 第 389–396 行），而一次 API 响应会发出多个带同一份 usage 的事件；正常结束时 result 事件会把它替换掉（第 257–259 行）。baseline 和 after 的这个函数相同。所以超时 record 的 usage 不能用来推断缓存是否命中。
- `config.Limits.TimeoutSeconds = 240`（第 237 行）并不是实际的进程上限，进程是在 300 s 时被 ExecOptions 的 5 分钟超时杀掉的。引用 240 s 的文档应改为 300 s。

## 5. 完整性（问题 5）

- `evidence-sha256.json` 列出 28 个文件。逐个重算 SHA-256 后，28 个全部一致，没有不一致，也没有缺失。目录里唯一没被列入清单的文件是 `evidence-sha256.json` 本身。
- 当前仓库中 3 个框架文件的哈希与 `prerun.json` 登记值一致。按 `contextFrameworkHash` 的方式重算的聚合值 `f6f62e55…` 与 8 条 record 的 `framework_sha256` 相同。两个 worktree 的 HEAD（`c58f79874…`、`835e7b046…`）与登记值一致，但这只证明现在的状态，不证明运行时的状态。
- 第 4 节依赖的 Provider 会话记录（C）不在哈希清单里，可能被 Claude Code 改写或清理。建议把 `701cfda2-7eb2-40d3-a9a2-3b0490d01149.jsonl` 复制进证据目录并补哈希。本复核是只读的，没有做这一步。

## 6. 其他与未能核实的项

- Provider 调用次数：每个 trial 1 个 permit、1 次 launch，本续跑共 8 次（E2a 1，E2b 3，E3 baseline 3，E3 after 1）。登记的剩余上限 22，用掉 8。E3 after 之后停止，符合 stop rule。
- 未能核实：网关端日志；真实服务端是否对同一 request id 的评论去重；其他机器或账号在网关上的负载；`~/.claude/settings.json` 没有配置 hooks，唯一启用的插件是 `mattpocock-skills`，插件内是否带 hook 没有查。C 中工具总耗时只有 2.2 s，所以 hook 不是耗时来源。
- 本复核没有读 E1 的证据，也没有评价 RK1 或新运行时（`0e1ee8e1e`、`96cedf485`）。

## 7. 与运行会话自评的分歧（打开四个文件后补写）

对照对象：4 份 `semantic-verdicts.json`、`run-summary.json`、`summary.md`、`verification-e2a-resume3-2026-11-01.md`。

一致的部分：8 条记录的有效/无效分类和 7 条通过全部一致。计数也一致：8 次 launch、7 个有效样本、1 次无效、剩余上限 14、下一入口为 E3 after trial 2。E2a 的 3 个槽位都已用完（1 个有效、2 个无效）。E3 只有在 after 有了有效样本之后才可能给出对照结论。

分歧和补充：
1. **"没有 API 错误文本"。** 自评在 `run-summary.json` 的 stop_reason、`semantic-verdicts.json`（E3 after）、`summary.md` 和 verification 文档里都写了 "no API error text"。这只对已哈希的 record 和 HTTP trace 成立。Provider 自己的会话记录在 04:21:38.923 有 `system/api_error`（`EPROTO: The TLS handshake failed`，retry 1/10），随后有一次重试请求，被杀前一直没有回复。host 的记录会丢掉这类事件，所以自评在它读的证据里看不到。应改为：最后一次模型请求连接失败（EPROTO），重试约 55 s 无响应，然后被 5 分钟超时杀掉。
2. **"正要写 memory 时超时"。** 自评写 "timed out … while the model was about to write collaboration memory"。实际上 `Write memory-update.md` 在 04:21:06 已经成功，之后约 88 s 卡在一次失败的 API 请求上，模型并没有在生成内容。最后阶段是卡住，不是正在工作。
3. **并发重叠的归因。** 自评只点名一个 peer（multica-0-4-37-87），说它在约 04:17–04:21 有模型请求。本机记录显示 after t1 期间至少有 4 个其他会话（`74057067`、`cb052b51`、`e5acaa26`、`cb4b233f`）有在途请求；请求 9 期间 4 个都在途；E3 baseline 期间重叠为 0。我无法把会话 id 对应到 agent 名称。`e5acaa26` 很可能是派发本复核的那个会话；`cb052b51` 的区间是 04:19:01–04:23:33，可能是一次长请求，也可能是延迟回复。我同意"因果未确立"，但补充两点：唯一的错误是连接层的 TLS 握手失败，不是 429；而且重叠的不止一个会话。
4. **E3 baseline t1 第二次 POST。** 自评把它"留待复核"。本复核的结论是：逐字节相同、同 request id 的幂等重试，起因是 fixture 的读后写缺口，不影响判据（第 3 节）。
5. **E3 对照结论。** 自评只写"after 还没有有效样本，所以尚无对照结论"。补充：就算 after 补齐 3 个有效样本，baseline 已经 3/3（与"修复前 ≤2/3"的预测相反），而且两个臂的 fixture 都提供 dispatch_comment。所以 E3 只能检验 prompt 文本，并且只能发现退化，不能证明修复带来改善（第 2 节）。
6. **E2a 预测。** 自评只记了槽位用完的事实，没有说明 "≥2/3" 的预测无法按预登记检验（只有 1 个有效样本），也没有说明尾部截断在 2.1.316 上没有发生（第 2 节）。
7. **实际模型。** 自评写"实际模型未观察到"。在 host 投影里确实如此（`observed_model: null`）。但 8 份 Provider 会话记录里 assistant 消息的 `model` 字段都是 `claude-opus-5-5`，这是 Provider 自报的值，不是网关侧的证明。
8. **自评没有提到的几点**（不改变任何判定）：超时 record 的 usage 是错的（第 4 节）；`TimeoutSeconds=240` 并不是实际上限，实际上限是 300 s；E3 的 4 条试次都碰到读后写缺口；3 条试次在 `/tmp` 写过又删了临时文件；E3 baseline 发顶层评论，与 CLI help 关于 `--parent` 的要求不符，fixture 不检查。
9. **超时分类的措辞。** 自评用 `environment_or_provider_timeout_unclassified`。本复核同意它属于环境/运行时类的 incomplete，原因可以写得更具体：Provider 的 API 连接失败（EPROTO），且当时延迟偏高，中位数 28.5 s，baseline 为 8–17 s。
