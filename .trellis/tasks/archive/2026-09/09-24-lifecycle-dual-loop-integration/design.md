# 技术设计

继续以 `CreateLifecycleHandoff` 为唯一运行时写入口，不引入新表。新增 `governance` handoff 类型，输入 capability 和带状态的信号列表；service 层按 capability 定义必需信号，并返回 `pass`、`hold` 或 `unknown`。handler 只记录证据、评论和可选 follow-up，不获得生产权限。

五类治理 capability 的最低信号为：

- `contract-compatibility`: `old-client-matrix`、`plugin-boundary`、`parse-with-fallback`
- `threat-modeling`: `trust-boundaries`、`abuse-paths`、`mitigations`
- `supply-chain`: `lockfile`、`provenance`、`sbom`、`license-review`
- `product-outcome`: `baseline`、`observation-window`、`observed-result`、`owner`
- `disaster-recovery`: `backup`、`restore-drill`、`rpo-rto`、`degradation-path`

每个信号状态只允许 `pass`、`fail`、`unknown`。任一 `fail` 返回 `hold`；没有 fail 但缺失或 unknown 返回 `unknown`；全部必需信号 pass 才返回 `pass`。

现有 `lifecycle_handoff` 保留为 latest 兼容键；每次写入同时追加到 `lifecycle_handoff_history`，只保留最近 20 条、限制序列化大小，防止元数据无限增长。审计评论仍逐次写入，作为完整证据链的第二份副本。

跨阶段测试使用本地脱敏 issue：RCA handoff -> rollout evidence -> incident-learning / governance evidence，并另以 Agent evaluation 记录版本和漂移 case。真实模型、生产指标和 rollback 不在本任务执行。
