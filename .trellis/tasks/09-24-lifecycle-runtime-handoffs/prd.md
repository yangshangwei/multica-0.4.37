# 生命周期运行时交接闭环

## Goal

把生命周期契约从静态 skill/fixture 变成可审计的运行时交接：诊断、事故复盘、发布后验证和 Agent 评测都能落回 issue，并通过现有 task queue 形成后续工作。

## Requirements

- 提供 workspace-scoped 的 lifecycle handoff API，要求调用者先通过现有 issue 权限检查。
- `bug-fix` 的已知原因允许直接修复，但必须记录原因和下游修复 issue；未知原因必须先创建或复用诊断后续 issue。
- `maintenance` 的未知失败必须路由到诊断后续 issue。
- `incident` 只有在 mitigation 完成后才能创建独立 RCA 后续 issue；RCA 不得阻塞止血。
- incident-learning 必须分离事实、推断、未知，并创建或幂等复用带 owner、优先级和验收信号的预防 issue。
- rollout/canary 和 Agent evaluation 只接受完整证据并写回源 issue；digest、基线、窗口、版本和必需 case 缺失时返回 `unknown`，不得宣称通过。
- 所有后续 issue 必须继承 workspace 边界和 source issue 关联；assigned agent/squad 使用现有 enqueue/dedup 语义。
- 不新增数据库表、凭据型 MCP、生产回滚执行器或真实模型评测调用。

## Acceptance Criteria

- [x] known/unknown bug-fix、maintenance 和 mitigation-first incident 均有 handler runtime 测试。
- [x] incident-learning 的重复请求复用同一 prevention issue，并保留 owner/acceptance metadata。
- [x] rollout/canary 证据写回同一 source issue，digest/window/baseline 失败进入 `unknown`，超阈值只返回建议或 hold。
- [x] Agent evaluation 的版本和六类 case 轨迹写回 source issue，缺证据阻断 pass。
- [x] 定向 Go tests、`go vet`、`git diff --check` 和 Trellis 校验通过；完整 handler suite 的既有 fixture 失败已单独记录。

## Constraints

- 复用现有 `IssueService.Create`、issue metadata 和 `TaskService`，不引入新 migration。
- 模板更新不得覆盖 workspace 已有副本；生产动作和外部凭据继续由人工审批边界保护。
