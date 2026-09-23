# 实施计划

1. 扩展服务证据结构和 handler 请求/metadata，加入观察窗口与 Agent artifact digest 的 fail-closed 校验。
2. 调整 incident-learning 的既有任务引用路径，并强化复用 follow-up 的 parent/workspace 关系校验。
3. 先写/补运行时负向和幂等测试，再运行定向 Go tests；发现 fixture 环境问题时记录而不绕过。
4. 运行 `go vet`、Trellis validate、`git diff --check`，检查现有模板和文档改动未被覆盖。
5. 更新本任务 evidence，完成 spec/提交前检查；不执行生产发布、回滚或真实模型评测。
