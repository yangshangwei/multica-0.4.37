# RCA 验收证据（2026-09-24）

## 已运行

- `/opt/homebrew/bin/go test ./internal/service ./internal/handler -run 'Test(ListSkillTemplates|ListAgentRoleTemplates|AgentRoleTemplates|RoleSkillTemplates)' -count=1`
  - 结果：通过。
- `/opt/homebrew/bin/go test ./internal/service -count=1`
  - 结果：通过。
- `pnpm exec vitest run packages/views/skills/lib/skill-presentation.test.ts --exclude '.omx/**'`
  - 结果：57/57 通过。
- `pnpm exec playwright test e2e/agent-role-template.spec.ts --workers=1`
  - 结果：2/2 通过；覆盖通用模板创建和 diagnostician 展示/提交字段边界。

目标包的完整 handler suite 也被尝试运行，但因本地测试数据库中与本改动无关的 project/context fixture 错误失败（例如 `project not found in this workspace`、`failed to load project context`）。模板相关的定向 service/handler 测试仍通过；该环境失败不能作为全仓 handler 通过证据。

## 真实模型 blocker

仓库默认测试禁止解析或执行用户安装的 Agent CLI。真实 smoke test 必须同时使用
`agentintegration` build tag 和 `MULTICA_RUN_REAL_AGENT_SMOKE=1`，并可能访问认证账号和消耗配额。本轮没有获得该显式授权，因此没有运行真实模型，也没有读取生产凭据或客户数据。

可复跑命令（获得授权后仅在隔离测试环境执行）：

```bash
(cd server && MULTICA_RUN_REAL_AGENT_SMOKE=1 go test -tags=agentintegration ./pkg/agent -run '<test-name>' -count=1 -v)
```

该 blocker 不影响已完成的模板、路由、权限和浏览器契约验证；真实模型的三类故障样例仍是 RCA 子任务的未完成验收项。
