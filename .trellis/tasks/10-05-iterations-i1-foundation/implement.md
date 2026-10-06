# 基础契约与写入收口实施清单

- 启动门槛：已评审计划与实施指令。前置整个任务：无；按里程碑核对。
- 读取父 design/api-contract/test-spec 与本任务 manifests。
- 先写对应失败测试，再按父 implement.md 所有权执行；不得跨写其他任务。
- 运行父 test-spec 对应 canonical 检查及受影响回归。
- 在本目录 verification.md 记录 commit/命令/结果/限制，满足内部 FG 可供其他任务使用，最终须 FCG（LG+HG+CG+UG后共享接线整合及回归）才交父归档。
- 当前为 in_progress：迁移/纯 helper 已整合，FG 未通过。已有 writer 切片按各 S2 验证记录保留；下一步完成父 handoff §10 的其余 FG 工作，不重做 S0/S1。
- foundation 在 FG 前拥有事件持久化、sequence/scope_revision 和全部 writer 接线；不能等待 FG 后的 history 才提供 recorder。
