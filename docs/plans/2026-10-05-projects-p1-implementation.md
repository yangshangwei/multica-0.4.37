# 项目增强 P1 技术设计与开发拆分

日期：2026-10-05。范围：完成实施方案与任务拆分；Architect → Critic 顺序复审已通过，产品代码尚未实施。交付证据见[规划核验](../../.trellis/tasks/10-05-projects-p1/planning-verification.md)。

沿用现有项目，交付目标模板、完成／取消统计、健康概览、手动进展及目标验收留痕。P2/P3、I1 实现和生产部署不属于本轮范围。

## 方案与验收

- [任务需求与边界](../../.trellis/tasks/10-05-projects-p1/prd.md)
- [技术设计、方案取舍与并发契约](../../.trellis/tasks/10-05-projects-p1/design.md)
- [API 与数据契约](../../.trellis/tasks/10-05-projects-p1/api-contract.md)
- [开发顺序、文件所有权与验证命令](../../.trellis/tasks/10-05-projects-p1/implement.md)
- [14 项需求、27 项验收及补充边界测试](../../.trellis/tasks/10-05-projects-p1/test-spec.md)

统计只包含同工作空间、实际归属该项目且准入为 `not_required`／`accepted` 的正式任务。旧 `done_count` 继续表示完成＋取消；新增分项区分实际完成与取消。没有正式任务时比例不适用，未知状态或读取失败不能被表示为健康。

模板复用现有描述；验收记录保存适用描述版本。描述真实改写要求版本条件，无版本的旧客户端写入返回 428，其他既有属性仍按兼容契约处理。进展提及只通知成员，不启动智能体。

## 五个子任务

1. [基础契约与生命周期](../../.trellis/tasks/10-05-projects-p1-foundation/prd.md)：迁移、描述版本、规划时区、状态审计与删除一致性。
2. [正式任务统计与健康](../../.trellis/tasks/10-05-projects-p1-health/prd.md)：统一集合、统计拆分、健康快照和精确下钻。
3. [手动进展与目标验收](../../.trellis/tasks/10-05-projects-p1-progress/prd.md)：发布、更正、不可变验收、幂等与站内通知。
4. [共享概览与编辑体验](../../.trellis/tasks/10-05-projects-p1-ui/prd.md)：core/views、Web/Desktop 接线及移动读取兼容。
5. [集成验收与交付](../../.trellis/tasks/10-05-projects-p1-verification/prd.md)：跨端、迁移、并发、性能和文档验证。

基础契约先行；健康与进展按冻结接口并行；界面接入后完成集成验收。启动条件、集成条件与共享文件整合责任以实施计划为准。所有任务保持 `planning`，规划就绪不代表产品已经完成。
