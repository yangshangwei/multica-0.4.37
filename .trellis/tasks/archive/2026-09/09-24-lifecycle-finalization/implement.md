# 实施计划

1. 激活任务后读取既有 RCA、事故学习、发布验证、角色、治理和 workload gate 证据，建立 requirement-to-evidence 差距表。
2. 补三条 RCA 路由的可复核 fixture/测试，确认 known-cause bypass、unknown diagnosis、incident mitigation-before-RCA 和修复任务交接。
3. 扩展事故学习、同产物发布观察和 Agent 质量 fixture，覆盖重复事故、缺 baseline、digest/window 错误、缺轨迹和漂移门禁。
4. 重新统计两个周期的体验验证与迁移审查 workload；证据达到 gate 后实现两个专项 role，并保留未来 gate 失守时的降级条件。
5. 补五类治理能力的失败样例和 squad/role 责任矩阵，审计 MCP catalog 的准入边界。
6. 运行 Go service/handler 定向测试、前端 typecheck/skill 测试、模板 E2E 和 Trellis validate；分离既有 fixture 故障。
7. 更新研究证据和父任务验收状态；完成 Lore commit，明确真实模型、生产 observability 和生产动作的未测试边界。

## 验证命令

```bash
cd server && go test ./internal/service -count=1
cd server && go test ./internal/handler -run 'Test(ListAgentRoleTemplates|CreateAgentFromTemplate|ListSquadTemplates|CreateSquadFromTemplate)' -count=1
pnpm typecheck
pnpm exec playwright test e2e/agent-role-template.spec.ts
python3 ./.trellis/scripts/task.py validate .trellis/tasks/09-24-lifecycle-finalization
git diff --check
```
