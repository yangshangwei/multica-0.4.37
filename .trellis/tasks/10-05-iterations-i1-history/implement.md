# 承诺、范围事件与历史统计实施清单

- 启动门槛：FG。前置整个任务：无；按里程碑核对。
- 读取父 design/api-contract/test-spec 与本任务 manifests。
- 先写对应失败测试，再按父 implement.md 所有权执行；不得跨写其他任务。
- 运行父 test-spec 对应 canonical 检查及受影响回归。
- 在本目录 verification.md 记录 commit/命令/结果/限制，满足 HG 后交父集成。
- 当前业务任务保持 planning；纯统计/时间/canonical 准备已实现并整合。FG 后负责持久事实投影、图表、原始承诺及冻结 payload；recorder/writer 接线归 foundation。
- HG 用真实 DB fixture 验证冻结 payload，实际 end/handoff 后不漂移属于 CG/VG，不能反向等待 closure 完成才验收 HG。
