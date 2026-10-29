# 实现进度

## 2026-10-30：接收人选择器交付

本子任务已完成实现范围：

- server：新增分页/姓名搜索/最近协作 API，支持目录开关、related scope、members-only、来源成员隐私裁剪、available 条件、独立限流和查询前后权限复核。
- core：新增严格的搜索响应 schema、ApiClient、React Query 无限查询、稳定分组去重，并保留旧 resolver 与旧草稿 `recipientQuery` 兼容。
- views：新增共享 `CollaborationAccountPicker`，接入新建协作任务、邀请/转交/请求补充信息和成员属性三个入口；保留精确查找兜底，支持本人/已选/最近/全部、分页、键盘、Escape、外部标记和成员-only。
- docs/config：增加 `MULTICA_COLLABORATION_DIRECTORY_ENABLED` 的环境变量转发、双语文档和启动警告。
- tests：补充 core、Go service/handler、views picker/integration、Web E2E 发现规格。

## 验证状态

通过：

- core 全量：316 files / 4224 tests
- views collaboration：24 files / 120 tests
- recipient picker + 入口回归：4 files / 16 tests
- views/core typecheck、core/views lint
- 全仓 TypeScript typecheck：9/9 packages
- recipient handler/service race tests（完整迁移隔离库）
- Go compile/vet、sqlc regenerate、git diff --check
- Playwright E2E discovery：1 条规格
- 新增 `recipientPicker` 双语 key parity；全文件旧有 `runPreparation.attachments_one` 缺失仍保留为既有问题

已知验证边界：

- server/internal/service 与 server/internal/handler 的全包 race 回归中，handler 全包通过；service 全包在 10 分钟处因既有 `TestCollaborationSharedSkillWritersInvalidateEnabledDependents` 超时而终止。接收人相关 service race 测试单独通过，超时没有落在本子任务文件或测试上。
- E2E 已完成规格发现和静态契约检查；完整生产 Web/API 浏览器流程需要单独启动隔离 API 和构建环境，未在本轮重复跑全站验收。
- 全局 locale 文件存在父任务遗留的单个 key 差异，本子任务新增的 `recipientPicker` 命名空间已完全一致。

全仓 `pnpm test` 已执行；核心/桌面并发压力下有两个既有 `core-provider` 认证 handoff 测试失败，分别是 deferred device auth 超时和 StrictMode identity 请求数不稳定。接收人相关测试及独立 package 回归通过，失败没有落在本子任务改动上。
