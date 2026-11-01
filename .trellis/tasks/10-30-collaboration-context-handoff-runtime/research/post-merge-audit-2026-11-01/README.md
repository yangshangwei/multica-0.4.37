# 合入后 runtime 只读审计（2026-11-01）

状态：**9 项确认缺陷全部未修复**，另有 2 项被驳回。本页只做记录，没有改产品代码，也没有运行测试或调用 Provider。

## 来源

- 执行：会话 multica-0-4-37-ef 在 13:31–14:58（+0800）运行只读 workflow，共 14 个代理、571 次工具调用。该会话此后连续遇到网关 502，结果没有写入任何台账。会话 -a8 从它的 workflow 输出整理成本目录。
- 被审源码：`codex/cross-workspace-collaboration`，server tree `192ef85a0`。`400527447`、`2b026872c`、`03469921c` 的 server/packages/apps 树都与它相同。workflow 输出没有写 HEAD；13:24 到 15:52 之间没有产品提交，所以审计读到的就是这棵树。
- 方法：envelope、recording、recall 三个维度分别对照 prd/design/spec 读代码，每项发现再交给一个对抗复核代理。全程只读，没有运行测试、构建或数据库套件，也没有写复现测试。各维度未覆盖的范围见 `findings.json` 的 `coverage.*.not_checked`。
- 原始输出不入库，只记录路径、字节数和 sha256 `a4f00538…e05a3`。`findings.json` 只保留结果字段，没有复制代理日志和提示词预览。

## 当前 HEAD 复核

在 `b8fd547dc` 上，会话 -a8 逐行复读了 REC-1、ENV-2、ENV-4、RECALL-2。前三项仍然成立；RECALL-2 已被 `c54a28ba6` 部分修正。其余各项引用的代码文件在审计后没有改动：`b8fd547dc` 与被审树相比，只多了 `ac2a24f8f`、`c54a28ba6` 的 15 个文件，host_runner.go 只多了召回指引的 3 行。

## 确认的缺陷

严重度取复核后的值；括号里是审计代理原先给的值。

| ID | 严重度 | 位置 | 问题 | 影响的验收 |
| --- | --- | --- | --- | --- |
| REC-1 | 中（原 高） | `host_checkpoints.go:139` | 一次非确定性 checkpoint 失败（超时、429、5xx、传输错误）后，本轮不再上传，之后的块只写进 ledger。结束后恢复只有约 1 秒，随后无条件提交 outcome。outcome 提交后，服务端用 409 `checkpoint_out_of_order` 拒收剩余块，daemon 把它们标为 Rejected、原因记为 `authority_lost`。用户 Stop 时则被 409 `execution_stopped` 拒收。授权一直有效，记录却永久丢失 | AC4 |
| REC-2 | 中 | `host_checkpoints.go:128` | 块绑定暂存时那次 launch 的 token 和 attempt_seq。fresh-session fallback 之后，旧 attempt 未提交的块被拒，整条流随之关闭，fallback 那次 launch 的记录也一起丢失 | AC4 |
| ENV-1 | 中 | `collaboration_runtime_handoff.go:208` | 实时 handoff 读取当前最新 memory，DTO 也没有启动基线版本字段。共享面板把这个最新版本显示成“Started from v{N}”，与同一 run 行头显示的基线版本矛盾 | AC5 |
| ENV-2 | 中 | `collaboration_runtime_handoff.go:118` | 先脱敏再按 64 KiB 截断。接近上限的合法委派如果含凭据样文本，脱敏后变长、被截断，daemon 拒绝截断的委派，这个成员 run 永远无法启动 | AC5 |
| RECALL-1 | 中 | `protocol/collaboration_recall.go:49` | runtime 评论召回只有 `author_type`、`author_id` 和权限，没有作者显示名；人工路径有名字。不符合父 PRD R4、runtime PRD R4 | AC6 |
| ENV-4 | 低 | `collaboration_handoff_wakeup.go:103` | continuation 的 `wakeup_causes.completeness` 一律为 `unknown`，新记录的完整原因也是，面板因此显示“早期续作原因未记录” | AC5 |
| REC-3 | 低 | `host_checkpoint_ledger.go:193` | 所有确定性 4xx 都记为 `authority_lost`。协议里的 `stopped` 从不产生，`transport_error` 实际不可达，接收方分不清 Stop、撤权、传输失败和被取代的 attempt | AC4、AC9 |
| REC-4 | 低 | `host_recording.go:237` | 瞬时失败后即使恢复把全部块补齐，outcome 仍在恢复前就按 partial 固定，run 永久显示 `recording_incomplete` | AC4 |
| RECALL-2 | 低 | `collaboration-runtime-cli.md:49` | `c54a28ba6` 已补上命令列表；但第 49–52 行仍写着过滤等操作不支持、读取只返回最新 200 条评论，召回契约（`budget_too_small`、`content_state`、绑定 revision 的游标）在 spec 里仍没有成文 | AC6 |

## 驳回

- ENV-3：outcome 把 `result.Error` 并入 `body`。design.md:86 只要求没有后端契约时标为 `provider_result/unknown`，现状符合；失败本身由 `outcome=failure` 表明。
- ENV-5：服务端会产生 `purged`；冻结回放在来源被清除时返回 409、不启动，这是 design.md:81 规定的行为，并有测试锁定。

## 对 RK1 的影响

- REC-1、REC-4 会改变 H1、E0a 观察到的 `recording_incomplete` 和 seal 原因。本地框架里 checkpoint 很少失败，这两个缺陷大概率不出现，所以 RK1 通过不能证明它们不存在。
- RECALL-1 使 H3 无法按名字归属评论，只能按角色归属。
- ENV-2 只在委派接近 64 KiB 且含凭据样文本时触发。RK1 夹具达不到这个条件，因此不覆盖它。

## 用户决定（2026-11-01 17:10 +0800，会话 -a8）

- 9 项全部在 RK1 之前修复。修复会改变源码绑定，因此 RK1 必须在修复合入并重新运行 `source_manifest.py` 之后才能启动。RK1 本来就要等知识审计修复合入，所以这一步不增加它的等待时间。
- 由会话 -a8 修复：在独立 worktree 中进行，每项先写失败测试，独立复核后经负责协调的会话 -14 [4a018c] 合入共享分支。会话 -35 继续构建 RK1 框架，两边不互相阻塞。
