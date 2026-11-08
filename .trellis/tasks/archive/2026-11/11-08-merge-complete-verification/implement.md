# 执行计划：合并后完整测试与近期业务影响验收

## 0. 规划与提交身份

- [ ] 固化当前 HEAD、候选 SHA、产品路径差异和根 worktree dirty 边界。
- [ ] 记录本任务不修改产品代码，只写任务证据。

## 1. 快速完整性与静态门禁

- [ ] `git diff --check`（候选与当前 HEAD）。
- [ ] `pnpm exec turbo run lint typecheck --filter='!@multica/mobile' --concurrency=1 --force`。
- [ ] `pnpm check:ui-exports`。
- [ ] 针对近期 UI/views/web 改动运行相关 Vitest 文件，保存摘要。
- [ ] `pnpm exec turbo run test --filter='!@multica/mobile' --concurrency=1 --force -- --maxWorkers=2`。

## 2. 后端 targeted 验证

- [ ] `go test -race`：`internal/auth`、`internal/handler`、`internal/service`、
      `internal/daemon`、`internal/daemon/collaborationrun`、`cmd/server`。
- [ ] 运行 Context experiment fake-provider/continuation tests；不启用真实 provider gate。
- [ ] `go vet -p 2 ./...`，并保留数据库迁移/测试隔离结果。

## 3. 受影响业务 E2E

- [ ] 在任务独占环境执行 Projects lifecycle 与 global-scope navigation 目标用例。
- [ ] 执行 Collaboration/Knowledge/agent execution 相关稳定 Web/native 用例，保留条件
      skip 和证据文件。
- [ ] 确认生产 Web、API 和 built Desktop renderer 的 commit/port/DB 身份。

## 4. 完整跨层门禁

- [ ] 在 clean candidate checkout 执行 `make check`，使用新分配的数据库/端口和一 worker、
      zero retries。
- [ ] 保存完整日志、退出码、log SHA256、通过/失败/跳过计数和清理状态。
- [ ] 失败时先分类为产品、测试契约或环境问题；不得把旧报告或退出 0 的进程当作业务成功。

## 5. 报告与收尾

- [ ] 写 `final-report.md`：按五个业务域列出结果、证据链接、遗留风险和未覆盖项。
- [ ] 更新 `progress.md` / `task.json`，完成 Trellis quality check；不改产品代码、不推送。

## 主要命令

```sh
git rev-parse HEAD
git diff --quiet 52162f19a HEAD -- ':!*.trellis/tasks/archive/2026-11/11-08-merge-unmerged-worktrees/**'
git diff --check 52162f19a HEAD
pnpm exec turbo run lint typecheck --filter='!@multica/mobile' --concurrency=1 --force
pnpm check:ui-exports
pnpm exec turbo run test --filter='!@multica/mobile' --concurrency=1 --force -- --maxWorkers=2
cd server && go test -race ./internal/auth ./internal/handler ./internal/service ./internal/daemon ./internal/daemon/collaborationrun ./cmd/server
cd server && go vet -p 2 ./...
make check
```

## 回滚点

- 静态/targeted 失败：保留日志，继续独立检查，不修改源码。
- `make check` 失败：保留任务环境和日志，停止重复重跑同一失败；只有在根因明确且不改产品
  的情况下重试环境步骤。
- 收尾只写 `.trellis/tasks/11-08-merge-complete-verification/`，不触碰现有 dirty 任务。
