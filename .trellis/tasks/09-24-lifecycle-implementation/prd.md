# 研发交付与 Agent 质量双闭环继续实施

## Goal

承接 RCA 验收，实施事故学习、发布后验证、可靠性与 Agent 评测能力，并按工作量和治理门禁完成两条质量闭环。

## Requirements

- 保留已完成的 RCA 路由和两项核心角色，不重新拆分 frontend/backend/mobile 角色。
- 将 `multica-incident-learning` 接入事故恢复后的独立后续阶段，将 `multica-rollout-and-canary-verification` 接入发布审批后的观察阶段；事故必须先止血和确认恢复，学习不阻塞止血。
- 让可靠性工程师输出 SLI/SLO、错误预算、容量/降级、恢复和 RPO/RTO 证据；让 Agent 评测工程师输出可复现的版本化评测、门禁建议和漂移信号。两者不得执行未经批准的生产操作。
- 明确可复用的 Incident Learning / Canary、Reliability Review、Agent Quality Gate、Migration Review、Experience Validation 小队组合；没有独立交付物的主题不得新增协调员角色。
- 对契约兼容、威胁建模、供应链、产品结果、灾备五类治理能力，先以既有角色和可组合 skill 路由覆盖；体验验证和迁移审查只有在 workload gate 有证据时才进入 listed roster。
- 重新审计 MCP：继续只提供无凭据的浏览器和推理模板；Git/CI、Observability、Database MCP 只记录准入条件，不在本任务引入未经版本固定、最小权限和脱敏设计的外部连接。
- 所有模板更新必须保留 workspace 副本；不读取生产凭据、不执行生产发布/回滚/迁移/通知，不在默认测试中运行真实 Agent CLI。

## Acceptance Criteria

- [x] 事故和发布小队的路由文档明确输入、输出、停止条件、审批边界和同一 artifact digest 约束；模板回归测试覆盖缺证据和顺序错误。
- [x] 两个新增核心角色可从模板创建，skill 自动附着，权限、并发、本地化、说明和 workspace copy protection 有测试。
- [x] 至少一份脱敏事故样例能产出带 owner/验收信号的预防任务，缺证据显式为 `unknown`；至少一份发布样例能产出 baseline/window/signals/decision/rollback outcome。
- [x] Agent 评测样例覆盖成功、工具失败、越权、成本/延迟超阈值和版本漂移五类用例；可靠性样例覆盖 SLO 退化、队列/依赖故障和恢复验证。
- [x] 生命周期矩阵和 MCP 取舍文档与实现一致，并明确体验/迁移专项角色的 workload gate 及未达到阈值时的处理。
- [x] 相关 Go 模板/handler 定向测试、skill presentation 测试、`pnpm typecheck` 和浏览器模板 E2E 通过；真实模型 smoke 未获显式授权，已记录为未测试。

## Verification Notes

- `go test ./internal/service -count=1` 通过；模板/handler 定向测试通过。
- `pnpm typecheck`（9 个包）通过；`e2e/agent-role-template.spec.ts` 2/2 通过；Views 全量测试 443 files / 5385 tests 通过。
- 完整 `server/internal/handler` suite 仍受本地既有 project/context fixture 失败影响（项目创建和 project context 大量返回 500），与本次模板改动无直接关联；定向模板覆盖不受影响。
- 真实模型 RCA smoke 未运行：需要显式 `agentintegration` 与 `MULTICA_RUN_REAL_AGENT_SMOKE=1`，可能访问认证账号并消耗配额。

## Notes

- Keep `prd.md` focused on requirements, constraints, and acceptance criteria.
- Lightweight tasks can remain PRD-only.
- For complex tasks, add `design.md` for technical design and `implement.md` for execution planning before `task.py start`.
