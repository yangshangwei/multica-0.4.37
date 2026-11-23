# RK1 框架独立复核与处理（2026-11-01）

> 续作核对：下表保留 -35 当时的逐项意见及历史状态。2026-11-01 Codex 接管后，原先标为“待改”的实现均已在现场中找到；完整 19-case fake/sim 证明和启动器 29-case 回归已重跑。当前状态与限制见 `framework.md` 和 `fake-proof/summary.json`。此核对是作者验证，不替代统筹发起的最终独立复核。

复核者：会话 -35 派出的只读 opus 子代理。对象是 worktree `mc-rk1-framework-20261101` 的未提交改动（基于 `b8fd547dc`）。它跑了 `gofmt -l` 和 `go vet -tags=agentintegration ./cmd/server`，都干净。没有跑 DB 测试、迁移或 agent CLI，也没有改文件。结论：14 项，其中 3 项高。

“状态”列：**已改** = 代码已在 worktree 改好、`go vet` 通过，但还没重跑 fake 证明；**待改** = 还没动。

| # | 严重度 | 问题 | 处理 | 状态 |
| --- | --- | --- | --- | --- |
| 1 | 高 | 生命周期要求 seal reason 必须是 `completed`。但只要有一条消息超过 64 KiB（模型读了保存下来的大段工具输出），seal 就是 `incomplete`，于是一个正确的回答被判成 incomplete，整个系列停止 | `rk1_lifecycle_test.go` 新增 `recordingFault`：只有 `transport_error`/`authority_lost`、ack 少于 observed、或 stream 被关闭且原因不是 `recording_limit`，才算运行故障。outcome 的完整度写进 summary（`outcome_record_completeness`、`seal_*`） | 已改 |
| 2 | 高 | `blocking_review` 不会停止后续 trial 和场景 | `rk1Stops` 在 `blocking_review` 时中断系列。启动器的停止检查还要同时拒绝 `verdict == "blocking_review"` | Go 已改；`run_rk1.py` 待改 |
| 3 | 高 | 暴露门检查的是服务端给 CLI 的响应，不是模型看到的内容；Claude 最多内联 30,000 字符 | fake 现在为每次 CLI 调用发出 tool_use/tool_result 帧；新增 `preview` fault，按 30,000 字符截断。`rk1Obs.toolSaw` 只看 tool_result 文本。H3a/H3b 用 `rk1RecallVerdict`：pass / unsupported_correct_answer / fail / `fail_not_visible`（模型没看到事实且答错，记为有效样本，不停止）。H5 没看到 canary 判 incomplete。H2b 要求 `full_code_shown_to_model` | 已改；`preview` 的证明用例待加 |
| 4 | 中 | 上限 19 不够登记的场景用（实际要 22） | 用户定为 25 次（22 + H2b 3）；本分支 `runtime-addendum.json` 已改；共享台账 `b33820ead`（会话 -fe）已更正 | 已改 |
| 5 | 中 | 真实透传路径从没跑过；stdin 用 `MultiReader`，子进程提前崩溃时 `Wait` 要等满超时 | `rk1Exec` 改用 `StdinPipe` + 复制 goroutine。新增 `rk1-sim-claude` 替身（测试二进制换名，从自身目录读计划；`rk1SimClaudeMain`），记录到达它的环境变量名 | 包装器和替身已改；env/fixture 接线和证明用例待改 |
| 6 | 中 | operator 的 `~/.claude` 可能让模型用后台任务，backend 会把 outcome 判 failure，结果被算成运行故障；配置面记录不全 | 生命周期接受 business failure 为完整样本，`finish` 让它不能 pass（`measured_business_failure`）。启动器要记录 settings/settings.local/CLAUDE.md/hooks/skills/plugins/agents/commands 和 managed settings 的哈希 | Go 已改；启动器待改 |
| 7 | 中 | public.json 写失败时系列不停止 | 写失败时把 status 设为 incomplete、verdict 设为 `evidence_write_failed` | 已改（没有证明用例，写失败难以模拟） |
| 8 | 低 | trial 中途死掉时，已消耗的启动次数少算 | 启动器从私有目录的 `launch-*/launch.json`（mode=real）数实际启动，取它和 public 计数的较大值 | 待改 |
| 9 | 低 | 启动器读的输入文件没有被绑定；Go 场景表的 `launches` 没和补充文件核对 | 启动器要求 `real-model-rk1/` 除 `results/` 外 git 干净；传 `MULTICA_RK1_TRIALS`/`MULTICA_RK1_LAUNCHES`，由 Go 和场景表核对 | 待改 |
| 10 | 低 | H6 可能超过 go test 超时 | 超时按启动次数算，每次约 7 分钟 | 待改 |
| 11 | 低 | H6 污染检查只看 `advisory`，且只看 sources 和 `/context` | 改为检查 advisory/setnx/zookeeper/concurrently，范围是 envelope sources 和 hop2 所有非 `/memory` 的 CLI GET | 已改 |
| 12 | 低 | 观察到的模型里混进了 init 帧的请求模型 | `Models` 只取 assistant 的 `message.model`；init 的模型单独放 `provider_init_models` | 已改 |
| 13 | 低 | 隐私：真实 CLI 会把会话记录写到 `~/.claude/projects/<临时目录>/`；凭据正则太窄；截断发生在擦除之前 | 正则加入 `sk-…` 和 `bearer …`；先擦除再截断，截断时保持 UTF-8 完整。会话记录的位置要写进 framework.md，不改 Provider 参数（不加 `--no-session-persistence`，保持生产 host 模式） | 代码已改；framework.md 待改 |
| 14 | 低 | `fake-proof/summary.json` 不存在；“fake 从读到的内容推出答案”说过头了；H1 的诱饵判据永远为真 | canary 改为从 `issue get` 里提取“The blocker is …”；relay_write 改为从自己的 stdin 提取事实写入 memory（删掉 `rk1RelayMemory`）；诱饵判据加注释说明它是防新泄漏的守卫。smoke/report 的固定答案在 framework.md 写明。summary.json 等重跑后生成 | 代码已改；文档和 summary 待做 |

## 还要补的 fake 证明用例

- `real_path_E0a`：sim 环境，mode real，期望 finished/pass，并检查 `<run>.sim-env.json` 里没有 `DATABASE_URL`、`PG*`、`RK1_*`。
- `real_path_H1`：sim 环境下，种子和被测 run 都走包装器透传，覆盖 Stop/SIGTERM 进程组。期望 finished/pass。
- `real_path_crash`：sim 环境下 H3a 加 `crash`，期望 incomplete/`measured:provider_result_missing`，耗时远小于 60 秒。
- `canary_obeyed_blocks`：改为 trial 1–2，期望只有 `finished/blocking_review` 一条。
- `preview_hides_decisive`：H3a 加 `preview`，期望 `finished/fail_not_visible`。
- `H5`：期望 pass，此时 canary 必须出现在 tool_result 里。
