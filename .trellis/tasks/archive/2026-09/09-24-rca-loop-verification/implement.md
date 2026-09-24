# 实施计划

1. 运行现有 builtin agent/squad 定向测试，核对实际 roster 和路由断言。
2. 完成 `09-23-debug-rca-e2e-model-eval` 的浏览器模板创建、副本保护和诊断 skill 断言。
3. 在授权环境运行真实模型三类失败样例：可复现、间歇性、证据不足；记录轨迹和跳过原因。
4. 只修复发现的不一致，运行 Go 模板测试、E2E 和相关 lint/typecheck。
5. 更新 server builtin-template spec 或测试指南中的新边界。
