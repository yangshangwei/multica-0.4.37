# 研发交付与 Agent 质量双闭环继续接线

## Goal

把上一轮已经注册的 RCA、事故学习、发布后验证和 Agent 评测能力接入现有 issue/comment/task queue 运行时，证明下游成员能消费上游证据；同时重新审计常见研发场景，避免通过增加静态角色数量掩盖交接缺口。

## Requirements

1. 缺陷修复、维护和事故三类入口必须记录 RCA 路由：已知原因直达修复时留下 bypass reason，未知原因进入诊断；事故必须先缓解，RCA 作为独立 follow-up，不阻塞止血。
2. 事故恢复后必须能创建独立 incident-learning 后续任务，携带 facts、inferences、unknowns、owner、验收信号和重复事故/既有预防任务关联；重复工作必须复用既有 issue。
3. 发布后验证必须绑定获批 artifact digest，并记录 baseline、观察窗口、signals、decision 和 rollback outcome；digest 不一致、窗口未完成或 baseline 缺失只能得到 `unknown`，不得执行未经批准的回滚。
4. Agent/Skill/MCP 变更必须能把版本化 case manifest、脱敏轨迹、正确性/安全/成本/延迟/工具失败/漂移结论交给 review-gate/release；缺少轨迹或越权 case 时阻塞。
5. 重新审计契约兼容、威胁建模、供应链、产品结果和灾备治理场景，优先复用现有角色/skill/squad。只有存在独立交付物、稳定工作量和不可合并的权限边界时才新增角色或 MCP；凭据型 MCP 继续延后。
6. 保留 workspace copy semantics、人工审批边界、脱敏要求和默认测试不调用本地 Agent CLI 的约束。

## Acceptance Criteria

- [ ] 通过实际 issue/comment/task queue 测试证明 bug-fix、maintenance、incident 三条 RCA 路由及其 unknown/known/mitigation-before-RCA 分支。
- [ ] 通过实际 issue 关联或幂等复用测试证明 incident-learning 的 owner/验收信号和重复任务链接可被下游读取。
- [ ] 通过发布验证函数和失败矩阵测试证明同 digest、完整窗口、baseline 缺失/窗口开放/digest 不一致的明确结论及审批阻断。
- [ ] 通过 Agent evaluator case manifest 测试证明版本来源、轨迹、越权拒绝、成本/延迟/工具失败/漂移缺口会产生 hold 或 unknown。
- [ ] 交付一份生命周期场景审计，列出已覆盖、仍为人工交接和明确不新增的能力；新增角色不超过已有 workload-backed 的体验验证工程师和迁移审查员。
- [ ] 定向 Go 测试、`pnpm typecheck`、相关 Playwright、Trellis validate 和 `git diff --check` 通过；任何既有环境失败单独记录，不伪装成通过。

## Constraints

- 不读取生产凭据或客户原始数据，不执行生产发布、回滚、迁移、恢复、通知或付费模型调用。
- 不引入数据库迁移或凭据型 MCP；不按 frontend/backend/mobile 拆分角色，不新增泛化 coordinator。
- 现有未提交改动和用户未跟踪文件必须保留。
