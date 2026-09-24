# RCA 路由验收证据（2026-09-24）

当前基线测试与浏览器证据见子任务 `09-23-debug-rca-e2e-model-eval/research/evidence.md`。已确认：

- `bug-fix`、`maintenance`、`incident` 的模板和 leader instructions 包含诊断工程师交接；incident 保持先缓解、后独立 RCA。
- diagnostician 默认 Contributor、并发 1、唯一 role skill 为 `multica-debugging`，不提交正式修复。
- 新增可靠性和 Agent 评测角色的模板改动没有改变 RCA 路由或 Operator 数量。

未完成项：真实模型评测仍需显式 agentintegration 授权；在授权前不得宣称三类模型行为已通过。
