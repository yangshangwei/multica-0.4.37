# 技术设计

在现有 `CreateLifecycleHandoff` API 上扩展 RCA evidence，不新增 endpoint 或数据库表。

请求保留现有 route/cause_state/follow-up 字段，并增加可选的脱敏字段：

- `diagnosis_ref`：来源 issue/comment 或外部测试报告的稳定引用；
- `regression_test`：修复任务应保留的最小回归测试标识；
- `conclusion`、`evidence`、`unknowns`：区分确认结论、事实证据和未确认项。

handler 在创建或复用下游 issue 后，将这些字段写入 `lifecycle_*` metadata，并在源 issue 的 handoff evidence 中保留同一份内容。缺失字段不把未知 RCA 误判为已确认；已有 route validator 继续决定 `diagnosis`、`direct-repair` 和 `post-recovery-rca`。

浏览器验收复用 `localized-template-defaults.spec.ts` 的真实登录、隔离 workspace 和 `seedProjectRuntime`，新增 diagnostician 创建与副本保护断言。真实模型 smoke 不在本任务执行。
