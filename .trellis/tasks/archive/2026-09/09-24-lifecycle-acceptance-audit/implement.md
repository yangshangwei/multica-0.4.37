# 执行计划

1. 运行 `task.py validate`，启动本任务并确认当前任务指针。
2. 运行 RCA 相关 Go 单元/handler/template 测试，记录通过数和跳过数。
3. 启动或复用本地 API/Web，运行真实 API Playwright 模板 E2E；记录实际命令、URL、结果和环境。
4. 检查真实模型 smoke 的显式授权条件；无授权时写入 blocker evidence，不调用模型 CLI。
5. 审计父任务与历史子任务 JSON/PRD/evidence，输出状态矩阵并标出仍未完成的验收项。
6. 运行 `pnpm typecheck`、`go vet`、`git diff --check`、Trellis validation；更新任务验收和 evidence。
7. 只提交本任务新增文件或明确归属本任务的测试改动，归档本任务；保留其他工作区修改。

## 验证命令

```bash
go test ./internal/handler -run 'Test(CreateLifecycleHandoff|.*Template|.*Squad)' -count=1
go test ./internal/service -run 'Test.*(Agent|Squad|Skill|Autonomy)' -count=1
PLAYWRIGHT_BASE_URL=http://localhost:13492 NEXT_PUBLIC_API_URL=http://localhost:18572 pnpm exec playwright test e2e/localized-template-defaults.spec.ts --workers=1
pnpm typecheck
go vet ./internal/handler ./internal/service ./cmd/server
git diff --check
python3 ./.trellis/scripts/task.py validate .trellis/tasks/09-24-lifecycle-acceptance-audit
```
