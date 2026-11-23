# E2a 第 2 次续作 — 2026-11-01

只执行 E2a baseline trial 2。Provider 在 195.744 秒后返回上游 502；context/memory read 和 memory write 均为 0，没有事实答案。原始 `provider_failure_unclassified` 不修改，语义复核归为环境无效尝试。

沿用原场景、固定 baseline `c58f79874cfe2e6318e3dce067d7b6bac24e943f`、框架 SHA-256 `f6f62e55888fa2d3e733ff12dec6054e07f877828363ad0d503ada6db07aa0bd`、Claude CLI 2.1.316、请求模型 `claude-opus-5-5` 和 provider_default thinking/tier。实际生效模型/tier 未观察到，不以请求值冒充。

已核对 1,964 对测试 HTTP 请求/响应、连续事件序号、相同 run 的一次 permit/termination/proof、最终 proof_acknowledged 及 prompt hash。执行收尾成立，行为验收未通过。错误正文在原始记录中；未为诊断增加 Provider 请求或复制私人会话。

首个不完整结果后停止，trial 3 和所有后续场景未运行。E1 已消耗三个槽位（1 有效、2 无效）；E2a 已消耗两个槽位（均无效）。剩余上限 **22 次 Provider 执行**，下一入口为 **E2a trial 3**，其后 E2b → E3 baseline/after → E4 → E5 → E6。不补旧编号、不扩预算。

本证据只属于原固定产品实验，不覆盖新 runtime `0e1ee8e1e`。本窗口没有代码改动或并行编码/审查调用。第一次本地 preflight 对已登记框架补丁的检查过严，在 Provider 启动前退出；随后只豁免哈希完全匹配的登记文件，产品源码仍无漂移。

证据入口：`prerun.json`、`framework-manifest.json`、场景目录的 `command.json`、`go-test.log`、`records.jsonl`、`records.jsonl.trials/…/events.jsonl`、`semantic-verdicts.json` 和 `run-summary.json`。`evidence-sha256.json` 记录其完整性。
