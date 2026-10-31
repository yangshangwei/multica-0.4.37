# 首轮修复交付：第二次续作

正式 E0 的通过记录仍有效。本次跑完 E1 剩余的第 2、3 轮：**第 2 轮事实回答正确，但没有回写协作上下文；第 3 轮在 6 次上游 HTTP 429 后超时。** E1 累计只有 1 个有效行为样本，E2–E6 未运行，任务保持 `in_progress`。

## 本次变更与验证

本地提交 `9673ab3bf` 修改两个 `host_context_experiment*.go` 测试文件及实验规范：续跑可指定原编号，避免重新执行已消耗的轮次；总轮数仍为 3。未设置起始轮次时从 1 开始，显式空值、非法或越界配置在 Provider 探测前拒绝。

实施及独立复核均完成假 Provider 回归、tagged 编译/`go vet`、gofmt 和 diff 检查。完整入口证明默认执行 `1,2,3`，续跑仅执行 `2,3`，不完整结果会停止后续轮次，接力编号和配对上下文正确。baseline 和 after 使用同一框架，最终 baseline 假 Provider 套件 10 个顶层测试、28 个含子测试的通过项。场景文本、评分、模型和超时未变；产品 Go/前端代码无改动，因此未重复首次续作的全量前端及数据库验收。

## E1 的三轮账本

| Trial | 有效性 | 原始证据与裁决 |
| --- | --- | --- |
| 1，首次续作 | 无效 | 6 次 429、1 次 503 后五分钟超时；旧证据保留 |
| 2，本次 | 有效，事实正确，未回写 | 经 3 次 429 后恢复；216.455 秒完成；读取 context 和 memory 各 1 次，明确答出 pg-blue-17，以及 pg-green-15 缺少 pgvector 0.8；memory PUT 为 0 |
| 3，本次 | 无效 | 读取 context 1 次；6 次 429 后报告 `claude timed out after 5m0s`；300.882 秒，无最终答复或 memory PUT |

第 2 轮的原始框架判定 `manual_review_required` 与第 3 轮的 `provider_failure_unclassified` 均保留；补充裁决另存，不覆盖原始记录。第 2 轮从短暂限流中恢复并完整结束，所以属于有效样本；不能把所有出现过 429 的尝试一律排除。有效样本中事实正确为 1/1，memory 回写为 0/1；不足以声称预登记的 3/3 事实回答及至少 2/3 回写目标达标。

两轮均保留完整 prompt/HTTP/lifecycle 证据；各有一次 permit、termination 和 proof，最终账本均为 `proof_acknowledged`。CLI 为 2.1.316，两轮测试独占 Provider session 实际观察到 `claude-opus-5-5`。本次没有继续发起 E2 或额外健康探测调用。

## 来源与证据

- 真实 E1 的产品源码：baseline `c58f79874cfe2e6318e3dce067d7b6bac24e943f`。
- after 验证源码：`835e7b046c6841e0945a59326bc4ddcbae4a0df7`；排除实验测试后的 server 产品源码与已通过 E0 的 `4c833e737` 无差异。
- 框架 SHA-256：`f6f62e55888fa2d3e733ff12dec6054e07f877828363ad0d503ada6db07aa0bd`。
- [运行前登记及框架复核](research/experiments-resume-2026-10-31/harness-review.md)；同目录保留 `prerun.json`、框架 manifest、红/绿检查及独立复核日志。
- [真实运行元数据](research/experiments-resume-2026-10-31/E1_memory_first-baseline/metadata.json)、[原始记录](research/experiments-resume-2026-10-31/E1_memory_first-baseline/records.jsonl)、[Provider 诊断](research/experiments-resume-2026-10-31/E1_memory_first-baseline/provider-diagnosis.json)、[逐项语义裁决](research/experiments-resume-2026-10-31/E1_memory_first-baseline/semantic-verdicts.json)。费用没有观测值，不作估算；未复制私人 Provider 会话正文。

## 下次入口

上游恢复后从 **E2a trial 1** 继续，随后按原顺序 E2b → E3 baseline/after → E4 → E5 → E6；最多剩余 24 次 Provider 调用。本次在第 3 轮环境失败后停止，不用后续实验绕过停止条件。E1 的三个槽位均已消耗，不重跑第 1 轮，不添加第 4 轮，不以成功替换无效尝试。E1 样本不足及已观察到的回写遗漏继续作为验收限制，后续产品改进由 runtime 子任务承接。
