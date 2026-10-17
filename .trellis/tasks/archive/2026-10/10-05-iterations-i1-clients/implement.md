# Web/Desktop 完整闭环与兼容实施清单

- 启动门槛：FG 后实施；随 LG/HG 稳定 API 分段联调；完整闭环联调与 UG 验收须 CG。前置整个任务：无；按里程碑核对。
- 读取父 design/api-contract/test-spec 与本任务 manifests。
- 先写对应失败测试，再按父 implement.md 所有权执行；不得跨写其他任务。
- 运行父 test-spec 对应 canonical 检查及受影响回归。
- 在本目录 verification.md 记录 commit/命令/结果/限制，满足 UG 后交父集成。
- 2026-10-07：UG已通过并关单；本地最终双端／视觉／兼容证据见verification子任务。VG正式远端CI仍未运行，不启用发布开关。
