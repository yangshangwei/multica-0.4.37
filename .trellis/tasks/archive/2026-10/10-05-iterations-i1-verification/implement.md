# 集成验收与发布准备实施清单

- 启动门槛：FG+LG+HG+CG+UG最终验收。前置整个任务：无；按里程碑核对。
- 读取父 design/api-contract/test-spec 与本任务 manifests。
- 先写对应失败测试，再按父 implement.md 所有权执行；不得跨写其他任务。
- 运行父 test-spec 对应 canonical 检查及受影响回归。
- 在本目录 verification.md 记录 commit/命令/结果/限制，满足 VG 后交父集成。
- 2026-10-07：本地29项、原PRD界面、兼容／迁移／性能／视觉已验收，UG/FCG关单；VG正式远端CI未执行，保持in_progress与关闭开关。下一步仅在未来授权发布阶段处理远端CI，见verification.md。
