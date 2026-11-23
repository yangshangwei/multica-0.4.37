# 正式任务统计与健康：设计边界

当前状态：该范围已落地，本轮P1实施验收完成（2026-10-06）。实际证据：[健康与HG](verification.md)及[ADR-05 C性能](../10-05-projects-p1-verification/performance-c-report.md)。

本子任务采用[父技术设计](../10-05-projects-p1/design.md)及[唯一API合同](../10-05-projects-p1/api-contract.md)的统一数据/API/权限/兼容/迁移契约；不得独立更改字段或创建并列实现。

基于实际项目归属和 T1 正式准入计算全量指标、健康原因和版本化风险下钻；保留旧 done_count 范围闭合含义，新增完成与取消数。

- 保持 T1 正式准入 `not_required` / `accepted`，以实际项目关联计数。
- 不将旧 `done_count` 改成仅 done；不以默认值掩盖统计失败或未知类别。
- 新索引均 CONCURRENTLY、各占单语句迁移，不新增 FK/cascade 或依赖。
- 模板、统计、进展、状态与通知不得隐式启动执行。
- 具体文件所有权和跨子任务接线由[实施计划](../10-05-projects-p1/implement.md)统一安排。

需要改变父契约时先报告父任务并补充审查，不能在局部实现中悄悄偏离。
