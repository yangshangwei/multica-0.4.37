# 安全与一致性修复完成记录

任务：`10-07-audit-security-consistency-fixes`。状态：**已完成并归档**。六项审计问题均已修复，独立复核无遗留功能问题。

## 改动与提交

### d5e193977 · fix(desktop): prevent preview scripts from reading local files

- `apps/desktop/src/main/renderer-file-access.ts`
- `apps/desktop/src/main/renderer-file-access.test.ts`
- `apps/desktop/src/main/index.ts`
- `apps/desktop/src/main/renderer-web-preferences.ts`
- `apps/desktop/scripts/verify-renderer-file-access.mjs`
- `.trellis/spec/desktop/frontend/renderer-file-access.md`
- `.trellis/spec/desktop/frontend/index.md`

### 5dd1abe52 · fix(plugins): prevent machine tokens from acquiring member authority

- `server/cmd/server/router.go`
- `server/cmd/server/plugin_action_routes_test.go`
- `server/internal/handler/plugin_action.go`
- `server/internal/handler/plugin_action_test.go`
- `.trellis/spec/server/security-boundaries.md`

### 2dff34698 · fix(projects): preserve resource invariants across concurrent edits

- `server/internal/handler/project_resource.go`
- `server/internal/handler/project_resource_concurrency_test.go`
- `.trellis/spec/server/project-execution-squad.md`

### f883bb187 · fix(iterations): preserve drafts across remote edits and refresh failures

- `packages/views/iterations/iteration-form.tsx`
- `packages/views/iterations/iteration-form.test.tsx`
- `packages/views/iterations/iteration-navigation.test.tsx`
- `packages/views/iterations/iteration-page.tsx`
- `packages/views/locales/en/projects.json`
- `packages/views/locales/zh-Hans/projects.json`
- `.trellis/spec/core/frontend/iteration-operations.md`

### b9d7066da · fix(projects): keep cached navigation from mixing progress drafts

- `packages/views/projects/components/project-detail.tsx`
- `packages/views/projects/components/project-detail.test.tsx`
- `.trellis/spec/views/frontend/component-guidelines.md`

## 简化与兼容

复用现有权限分类、事务与锁、命令恢复和草稿存储。新增桌面会话级文件请求边界；项目切换只在现有概览边界增加身份 key。没有新增依赖、数据库迁移或公共 API 字段。

## 验证

- 全量 TypeScript 检查点：876 个文件、10,245 个测试通过（仅 docs 使用缓存）。
- 独立复核追加修复 408/429 后：51 个相关测试、views 类型检查及改动文件 lint 通过。
- 仓库静态检查：15/15 任务成功，lint 零错误、30 条已有警告。
- 真实 Electron 39.8.7：16/16 安全与正常功能检查通过，并独立重跑。
- 后端新增回归及相邻模块 race 测试通过，新增回归 117 个测试/子测试通过事件、零跳过；全仓 Go vet 通过。
- 新回归记录了修复前失败、修复后通过。详见 `research/final-review.md` 与 `research/remediation-verification.json`。

## 边界与清理

- 两个无关 Redis 测试因未配置 REDIS_TEST_URL 跳过；未运行全仓 Go 测试。
- 原生验证在 macOS 完成；未验证 Windows/Linux、安装包/ASAR 或完整生产启动。
- webSecurity 仍关闭；本次关闭已确认的本地文件读取入口，不声称提供完整来源隔离。
- 既有重复资源数据未自动修复。
- 本次创建的独立数据库已在确认零连接后删除，临时 Electron 验证目录由脚本清理。
- 五组业务修改已本地提交；未推送、发布或部署。原有无关工作区改动保留。
