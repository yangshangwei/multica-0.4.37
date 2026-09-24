# RCA 小队接入验收与真实模型评测

## Goal

Prove that the landed diagnostician routing behaves correctly for defects, maintenance failures, and incidents, then complete browser template creation and real-model RCA evaluation.

## Requirements

- Verify `bug-fix`: after reproduction, unknown causes go to diagnosis; a direct fix records why diagnosis was skipped; diagnosis does not submit the formal fix.
- Verify `incident`: request human intervention and mitigate first; after recovery confirmation create a separate root-cause task; the incident does not wait for RCA.
- Verify `maintenance`: unexplained flaky tests and upgrade failures use diagnosis and return to bug-fix or implementation when classified.
- Reuse `09-23-debug-rca-e2e-model-eval` for browser creation and real-model behavior, preserving raw evidence and environment limits.

## Acceptance Criteria

- [x] Squad roster, squad instructions, leader instructions, creation API, and docs agree on the handoff and copy-preservation rules.
- [x] Targeted Go template tests and the real creation path pass, with actual counts and skips reported.
- [x] Browser E2E creates a diagnostician from the template and verifies default skill, autonomy, presentation, and existing-copy preservation.
- [ ] Real-model evaluation covers reproducible, intermittent, or evidence-insufficient failures and reports causal evidence, tool calls, stop conditions, and authority boundaries. (deferred: authorization-gated, recorded as a documented skip)
- [x] Any unavailable environment is recorded with an exact blocker and repeatable follow-up; independent work continues.

## 验收状态（2026-09-24 收尾）

RCA 路由与浏览器/后端契约验收完成；真实模型评测按显式决定暂缓（authorization-gated）。证据见 `research/evidence.md` 与子任务 `09-23-debug-rca-e2e-model-eval`。

- 路由：bug-fix / maintenance / incident 模板与 leader instructions 均含诊断交接；incident 先缓解、后独立 RCA；diagnostician 默认 Contributor、并发 1、唯一 skill `multica-debugging`、不提交正式修复；新增 reliability / agent-evaluator 角色未改变 RCA 路由或 Operator 数量。
- 浏览器/后端：子任务 `e2e/agent-role-template.spec.ts` 2/2 与定向 Go 模板测试通过。
- deferred：AC#4 真实模型三类失败评测需显式授权，记为文档化跳过。

## Dependency

This is the first child. RCA routing landed in `bfc218434` and `3cef3556d`; this task primarily verifies it and fixes findings. The existing browser/model child owns that evaluation scope.
