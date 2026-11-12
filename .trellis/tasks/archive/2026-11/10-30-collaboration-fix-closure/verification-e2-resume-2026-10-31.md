# 首轮修复交付：E2a 续作

本次从 E2a 第 1 轮接续，实际读取了场景上下文，但在上游 HTTP 429 持续出现后五分钟超时，没有最终日期答案。**本轮无有效行为样本，后续轮次和场景已停止。** 任务保持 `in_progress`。

## 本次结果

| 项目 | 结果 |
| --- | --- |
| 场景 | E2a：200 条评论，关键冻结日期位于最新一条，目标日期为 2026-11-14 |
| 实际执行 | 仅 trial 1，1 次 Provider 执行 |
| 观察 | context read 为 1；初次 CLI 输出过长而保存到临时文件，随后读取该文件的工具返回包含最新的 2026-11-14 决策；没有最终答复 |
| 运行结果 | `claude timed out after 5m0s`，300.904 秒；原始判定保留 `provider_failure_unclassified`，补充诊断与原始记录分开保存 |
| 后续 | E2a trial 2、3 及 E2b–E6 均未运行，没有附加健康探测或替换样本 |

原始工具结果证明关键日期已经暴露给 Provider，但它没有产生最终日期答案，因此仍不评分。本次超时归入环境无效尝试，不计作模型答错；这一次读取临时输出的行为也不能证明通用分页可靠性。

## 验证与来源

本轮继续使用已经复核的实验框架 `9673ab3bf`，SHA-256 为 `f6f62e55888fa2d3e733ff12dec6054e07f877828363ad0d503ada6db07aa0bd`。运行前核对了两个隔离源码目录及上轮 37 个证据 hash，框架没有变化，所以未重复假 Provider、Go、前端或迁移测试。本次只新增实验记录和任务进度。

E2a 的源码保持预登记 baseline `c58f79874cfe2e6318e3dce067d7b6bac24e943f`；CLI 仍为 2.1.316，请求模型仍为 `claude-opus-5-5`，thinking/service tier 保持 `provider_default`。主工作区正在进行的 runtime 产品改动不进入本轮实验。

测试独占 Provider 会话实际观察到 `claude-opus-5-5` 及 4 次 HTTP 429。3,017 个测试 HTTP 请求/响应按 ID 完整配对，各有一次 permit、termination 和 proof，最终账本为 `proof_acknowledged`；这证明执行已收尾，不代表行为验收通过。

运行前登记、显式环境与命令、原始 prompt/HTTP/lifecycle、Provider 诊断和汇总均保存于 [本轮证据汇总](research/experiments-e2-resume-2026-10-31/summary.md) 所在目录。原始 verdict 不因补充诊断而改写。

## 剩余验收

- 正式 E0 保留已通过的历史证据。
- E1 的三个槽位已消耗：1 个有效样本、2 次无效尝试；有效样本事实正确但未回写 memory。
- E2a trial 1 已消耗且无效。下一次从 **E2a trial 2** 继续，不重跑 trial 1；其后仍按 E2b → E3 baseline/after → E4 → E5 → E6 的顺序运行。
- 总预算余下 **23 次 Provider 调用**。需要上游恢复后再续作；不延长超时、不增加轮数、不替换模型或挑选成功样本。
