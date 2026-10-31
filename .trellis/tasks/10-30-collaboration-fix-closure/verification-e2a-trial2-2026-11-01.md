# E2a 第 2 次续作

本次仅执行 E2a baseline trial 2。Provider 在 195.744 秒后返回上游 502，没有读取场景上下文或给出日期答案；本次为环境无效尝试。已按首个不完整结果停止后续轮次和场景，未额外探测、换模型或延长超时。

| 项目 | 证据与结论 |
| --- | --- |
| 固定来源 | baseline `c58f79874cfe2e6318e3dce067d7b6bac24e943f`；实验框架 `f6f62e55888fa2d3e733ff12dec6054e07f877828363ad0d503ada6db07aa0bd` |
| Provider | Claude CLI 2.1.316；请求 `claude-opus-5-5`，thinking/tier 保持 provider_default；实际模型与生效配置未观察到 |
| 原始结果 | `incomplete` / `provider_failure_unclassified`；错误正文为 `API Error: 502 Upstream service temporarily unavailable` |
| 行为证据 | context/memory read 与 memory write 均为 0；不评分为正确或错误答案 |
| 执行收尾 | 1,964 对 HTTP 请求/响应完整配对，事件序号连续，同一 run 各一次 permit/termination/proof，账本最终 `proof_acknowledged` |
| 剩余 | 总上限 **22 次 Provider 执行**；下一入口 **E2a trial 3**，随后 E2b → E3 baseline/after → E4 → E5 → E6 |

E1 三个槽位已消耗（1 有效、2 无效）；E2a 两个槽位已消耗（均无效）。原编号和失败证据继续保留。生命周期完成不能代替模型验收，reported zero tokens 也不构成零费用证明。

运行前登记、命令、原始记录、独立语义判定与完整性 hash 见 [本轮证据](research/experiments-2026-11-01-e2a-resume2/summary.md)。新 runtime `0e1ee8e1e` 的真实验收尚未运行，不继承本系列的结论。
