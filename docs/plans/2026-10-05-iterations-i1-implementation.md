# 迭代 I1 工程方案入口

> 当前实施分支为 `codex/projects-p1`。I1 迁移已顺延为 550–566，并复用 P1 的共享时区。2026-10-07：FG/LG/HG/CG/UG/FCG 均已通过本地验收，仅 VG 的远端正式 CI 待办，发布开关关闭、未推送或合并。最终状态见[最终本地验收](../../.trellis/tasks/10-05-iterations-i1-verification/verification.md)，基础层整合过程见[分支整合记录](../../.trellis/tasks/10-05-iterations-i1/branch-integration.md)。

2026-10-05，仅规划，产品实施未开始。

采用任务唯一当前归属、固定原始承诺、追加范围事件和冻结结束快照；结束/结转/交接/禁用使用整体事务与持久幂等结果。Web/Desktop 首期完整，Mobile 编辑后置但兼容与服务器门槛首期验证。P1 共用规划时区，不新建第二套项目实体或时区设置。

- [任务需求](../../.trellis/tasks/10-05-iterations-i1/prd.md)
- [技术设计与 ADR](../../.trellis/tasks/10-05-iterations-i1/design.md)
- [API / 数据合同](../../.trellis/tasks/10-05-iterations-i1/api-contract.md)
- [六阶段实施与门槛](../../.trellis/tasks/10-05-iterations-i1/implement.md)
- [29 项 I1 验收及工程测试](../../.trellis/tasks/10-05-iterations-i1/test-spec.md)
- [规划核验](../../.trellis/tasks/10-05-iterations-i1/planning-verification.md)

按 foundation → lifecycle/history → closure → clients 集成 → verification 推进；FG/HG 等为内部里程碑，不能把父子目录关系误当完成依赖。尚未运行产品测试，尚未部署或启用迭代。

## Implementation update — 2026-10-06

The user authorized implementation. Foundation schema, SQL generation, transaction/operation helpers and pure statistics/calendar/canonical preparation are in the working copy. They do not implement the user-facing I1 workflow. Sandbox denial of local PostgreSQL/Redis TCP blocks real database verification; FG is not passed and dependent lifecycle/closure/client execution is not released. The detailed state and next steps are in the [implementation verification](../../.trellis/tasks/10-05-iterations-i1/verification.md). Git metadata is read-only in this session; no commit, push, migration or feature enablement has occurred.
