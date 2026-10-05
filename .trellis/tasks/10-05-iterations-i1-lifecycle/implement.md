# 手动生命周期与任务归属实施清单

- 启动门槛：FG。前置整个任务：无；按里程碑核对。
- 读取父 design/api-contract/test-spec 与本任务 manifests。
- 先写对应失败测试，再按父 implement.md 所有权执行；不得跨写其他任务。
- 运行父 test-spec 对应 canonical 检查及受影响回归。
- 在本目录 verification.md 记录 commit/命令/结果/限制，满足 LG 后交父集成。
- 当前只规划，保持 planning；产品实施未开始。
