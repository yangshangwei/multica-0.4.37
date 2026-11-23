# 执行计划

1. [x] 并行只读核对协作、工作管理和智能体资源域，记录实际入口、权限和限制。
2. [x] Trellis implement 子代理按页面所有权补充指南；主会话整合首页与目录及覆盖报告。
3. [x] 串行运行 node scripts/generate-docs-bundle.mjs，比较临时输出逐字节一致性。
4. [x] 执行 pnpm --filter @multica/docs test、typecheck、build；go test ./internal/docs/...；数据库可用时使用隔离库运行 Docs handler 测试。
5. [x] 检查所有内部页面链接、锚点、图片，独立 Trellis check 审核语义与覆盖；修正后仅重跑受影响检查。
6. [x] 记录证据、剩余部署限制；按实际修改清单交付，不混入用户既有改动。

边界：文档内容调整，不做业务代码重构。不新增行为镜像测试；复用现有文档与嵌入校验。docs 包无 lint 脚本，以编译/类型检查/链接校验与 diff --check 静态验证。
