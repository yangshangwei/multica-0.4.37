# 实施与验收顺序

状态：2026-11-23 用户明确回复“开始”，已实施、完成本地验证并带回38个文件。用户已确认本地提交，5组工作提交完成，第6组记录交付；最终结果及远端/图片预算边界见 `READINESS.md`。

## 0. Planning and handoff

- [x] 用户授权创建任务，记录另一会话仍有提交的要求。
- [x] 创建 PRD、设计、执行计划和真实 context manifests。
- [x] 最终规划摘要获之后的明确实施批准。
- [x] `task.py validate`、上下文检查后 `task.py start`；只激活本会话任务。

## 1. Recheck and select a candidate

- [x] `git fetch origin main codex/cross-workspace-collaboration`；保存 source/base/remote SHA、分支关系、工作区和暂存路径。
- [x] 对比任务记录的上次 HEAD；纳入其他会话新提交，确认哪些本地改动尚不在候选中。
- [x] 在任务私有 checkout 固定候选；不改变根工作区和其他会话文件。若 main 已分叉，检查实际集成候选。
- [x] 确认测试依赖、现有生产构建流程、任务数据库及无真实 Provider 的运行前提。

## 2. Repair image CI and prepare material disposition

- [x] 派发限定脚本实现角色；先让大差异回归在原实现上失败，再修读取机制。
- [x] 运行大列表、预算规则、显式豁免和失败传播测试；执行 `node scripts/check-image-budget.mjs --base <base-sha>`。
- [x] 生成精简图片/归档清单，逐项提出用途、压缩可行性或现有规则下的豁免理由。保留原始证据及历史。
- [x] 独立检查脚本行为与 CI 测试接线；预算仍失败则保留阻塞，不提前放行。

## 3. Close five recorded core cases

- [x] 核对最新代码与历史报告，找到原始用例和对应夹具，补齐任务私有环境配置。
- [x] 生产构建运行 `e2e/localized-template-defaults.spec.ts` 的两项记录失败。
- [x] 运行 `e2e/collaboration-business-scenarios.spec.ts` 中 S1 outsourcing。
- [x] 运行 `e2e/collaboration-live-revoke-web.spec.ts` 与 `e2e/collaboration-terminal-reopen-web.spec.ts`，保留实际 managed/password gate 和 runtime 身份语义。
- [x] 按 owning layer 最小修复真实复现的问题，建立必要回归并重放原用例。每个问题分别保留原因及结果，不以新测试替代旧断言。

## 4. Final validation

- [x] 再次检查 HEAD/origin/main，按设计处理任何新提交后确定最终候选。
- [x] 在私有 checkout 分层完成 `make check` 对应的 lint/typecheck、TS、脚本、隔离 Go race/vet、生产构建和完整默认 E2E，并补充条件用例。为共享机器资源和并行提交控制，采用完整包/受影响文件重放；不声称一次原样 `make check` 全绿。
- [x] 对照 `.github/workflows/ci.yml` 补核图片检查、生成的 sqlc/docs/reserved-slug 文件一致性、UI exports、适用静态分析及其他脚本契约。生成命令只在私有 checkout 执行。
- [x] `pnpm knip` 按现有 CI 的 warning 级别如实报告，不把历史警告擅自提升为本次新增阻塞。
- [x] 检查平台/版本相关 CI；未运行远端 CI、Windows/macOS 或移动端时明确未覆盖，禁止声称全部 CI 通过。
- [x] 独立审查最终差异、原失败闭环、证据归属和环境清理；修复后按影响范围重验，结果见独立审计文档。

## 5. Final candidate refresh and report

- [x] 最后 fetch 并记录 HEAD、origin/main、origin 功能分支。发现新提交则回到步骤 1/4 做影响检查；不会把等了一段时间当作另一会话已完成。
- [x] 确认根工作区和其他会话工作保留；只清理本任务拥有的进程/环境。
- [x] 交付固定 SHA 的合并建议，分别说明 Git 可合并性、质量阻塞、遗留债务和缺失的外部验证。
- [x] 用户在完整本地提交清单后回复“执行”；按清单完成工作提交及任务记录提交。
- [ ] PR 中的图片预算说明与最终远端 CI 待后续交付；推送、PR 发布和 main 合并不在本次本地提交授权中。

## Parallelism and ownership

候选固定后，图片检查与五项回归的只读定位可独立进行；产品修复只有明确文件归属时才能并行。实现与独立检查分开，所有角色知道存在其他会话，不覆盖他人差异。共享源码集成、数据库迁移、构建和最终验收按依赖顺序执行。
