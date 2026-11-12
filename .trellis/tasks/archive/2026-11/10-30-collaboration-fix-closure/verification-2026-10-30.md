# 首轮收尾验证记录（2026-10-30）

固定实施 HEAD：5491d1b22（实施 worktree codex/collab-context-plan）。

## 代码与迁移

- 集成历史保留 85b98c3bb 与 8 个修复提交。
- 746_collaboration_owner_reconsent.*.sql 替代未发布的重复前缀 745；旧 745_drop_user_cloud_waitlist 保留。
- Fresh migration：make migrate-up，隔离数据库 multica_mc_context_plan_20261030_220，执行到 746。
- Reapply：go -C server run ./cmd/migrate up，745/746 均 skip already applied，exit 0。
- make sqlc 后 git diff --exit-code -- server/pkg/db/generated server/pkg/db/queries，无漂移。
- A1 选择矩阵：8 个纯 helper 测试通过，覆盖引用必保、引用超 FileCount/PerFile/Total/Git budget、缺失引用、可选历史跳过。
- Legacy compatibility：unsupported files、late delivery、issued claim、settlement boundary 4 个回归通过。

原始日志：

- research/raw/migration-prefix-lint-final.log
- research/raw/migration-reapply-final.log
- research/raw/attachment-budget-final.log
- research/raw/service-collaboration-final.log
- research/raw/handler-collaboration-final.log
- research/raw/go-vet-final.log
- research/raw/go-build-final.log
- research/raw/pnpm-lint-final.log
- research/raw/pnpm-typecheck-final.log
- research/raw/pnpm-test-final.log

## 测试结果

- Go vet：PASS。
- Go build：PASS。
- go test ./cmd/multica/ -count=1：PASS。
- go test ./internal/daemon/collaborationrun/ -count=1：PASS。
- DB-backed go test ./internal/service -run Collaboration -count=1 -timeout 40m -v：PASS，389.532s。
- DB-backed go test ./internal/handler -run Collaboration -count=1 -timeout 40m -v：PASS，13.975s。
- pnpm lint：PASS，现有 warnings，无 errors。
- pnpm typecheck：PASS，9 packages。
- pnpm test -- --maxWorkers=2：PASS；5 turbo tasks，views 588 files / 7072 tests，exit 0。
- pnpm exec vitest run collaboration/workflow.test.tsx：PASS，6 tests。
- 依赖通过 pnpm install --frozen-lockfile 在独立 worktree 安装；lockfile 未变更。

## 尚未完成

- 实验框架硬化仍在独立 mc-context-exp-plan-20261030 worktree；原始 mc-context-exp 未提交文件保持不动。
- 正式 E0–E6 尚未运行。E0 必须使用 TestHostProviderAccountSmoke，E0b 不计入。
- 未执行正式生产 Web/桌面 handoff e2e；组件/fixture 层已补断言。
- 未运行旧 schema upgrade/down migration 矩阵；fresh/reapply 已完成。
