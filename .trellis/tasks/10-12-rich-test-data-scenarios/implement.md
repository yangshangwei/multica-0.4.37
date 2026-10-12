# 执行计划

1. [x] 阅读 server/core/desktop 约定和现有 E2E fixture，确认列、枚举和当前工作区边界。
2. [x] 实现 `scripts/seed-rich-test-data.mjs`：参数解析、目标解析、幂等创建、依赖写入、reset、dry-run 和汇总输出。
3. [x] 增加 `Makefile` 的 `seed` 目标和 package script 入口，保持命令短且复用当前 `DATABASE_URL`。
4. [x] 当前开发数据库执行 dry-run、seed、重复 seed；reset 在独立完整 schema 测试库实跑，不清理已交付手测数据。
5. [x] Node 语法检查、13 项安全测试、全仓 lint/typecheck、31 个真实 API、9 个当前 renderer 的原生 Electron 页面检查通过。
6. [x] 更新 `.trellis/spec/server/development-fixtures.md`，完成 Trellis check 审查；进入提交与归档。
