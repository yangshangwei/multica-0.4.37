# 共享概览与编辑体验：设计边界

本子任务采用[父技术设计](../10-05-projects-p1/design.md)及[唯一API合同](../10-05-projects-p1/api-contract.md)的统一数据/API/权限/兼容/迁移契约；不得独立更改字段或创建并列实现。

共享 core 数据契约/Query/草稿和 views 概览、目标模板、健康下钻、进展/验收编辑、历史、完成提示；Web/Desktop平台接线和旧端/移动读取兼容。

- 保持 T1 正式准入 `not_required` / `accepted`，以实际项目关联计数。
- 不将旧 `done_count` 改成仅 done；不以默认值掩盖统计失败或未知类别。
- 新索引均 CONCURRENTLY、各占单语句迁移，不新增 FK/cascade 或依赖。
- 模板、统计、进展、状态与通知不得隐式启动执行。
- 具体文件所有权和跨子任务接线由[实施计划](../10-05-projects-p1/implement.md)统一安排。

需要改变父契约时先报告父任务并补充审查，不能在局部实现中悄悄偏离。
