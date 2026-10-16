# 基础契约与写入收口实施清单

- 启动门槛：已评审计划与实施指令。前置整个任务：无；按里程碑核对。
- 读取父 design/api-contract/test-spec 与本任务 manifests。
- 先写对应失败测试，再按父 implement.md 所有权执行；不得跨写其他任务。
- 运行父 test-spec 对应 canonical 检查及受影响回归。
- 在本目录 verification.md 记录 commit/命令/结果/限制，满足内部 FG 可供其他任务使用，最终须 FCG（LG+HG+CG+UG后共享接线整合及回归）才交父归档。
- 当前为 in_progress：FG 已通过；已有 writer、授权、持久操作及性能证据保留。下一步按父 handoff §10 推进 LG/HG，共享接线仍由 foundation 整合至 FCG。
- foundation 在 FG 前拥有事件持久化、sequence/scope_revision 和全部 writer 接线；不能等待 FG 后的 history 才提供 recorder。

- FG 已通过，详见 fg-verification.md；foundation 仍为 in_progress，保留共享接线所有权至 FCG。下一步 LG/HG 可并行实施。
