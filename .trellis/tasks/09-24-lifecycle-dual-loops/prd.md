# 研发交付与 Agent 质量双闭环

## Goal

把诊断、修复、发布、观测、复盘、防重发和 Agent 变更、评测、发布、漂移监控连成可验证的工作流

## Current baseline

- `diagnostician` and `multica-debugging` already exist; `bug-fix`, `maintenance`, and `incident` squads now route unknown causes through the diagnostician. Incident mitigation does not wait for RCA; root-cause work is a separate post-recovery task.
- Release preparation and human approval exist; post-release health verification and incident learning now have reusable skills routed through Release/Incident leaders, with Reliability Engineer evidence seats.
- The roster has 12 listed roles and 13 role skills. New roles require distinct deliverables and are not split by technology stack; experience/migration specialists remain workload-gated.
- `09-23-debug-rca-e2e-model-eval` is an unfinished browser/real-model acceptance task and is reused here.

## Requirements

1. Verify the RCA handoff through defect and incident squads, including browser template creation; real-model RCA evaluation remains explicit-authorisation-only.
2. Add `multica-incident-learning` and `multica-rollout-and-canary-verification`, turning incident evidence into prevention work and release results into post-deployment health evidence.
3. Add `reliability-engineer` and `agent-evaluator` as core roles for runtime evidence and Agent behavior evaluation, and route them through incident/release/review-gate.
4. Add `experience-validation-engineer` and `migration-reviewer` only after workload and role-boundary evidence supports them.
5. Add contract compatibility, threat modeling, supply-chain, product-outcome, and disaster-recovery capabilities, routed through existing roles where possible.
6. Define reusable AI squad compositions for incident learning/canary, reliability, Agent evaluation, and migration review without creating technology-specific roles.
7. Reassess built-in MCP presets. Keep credential-free browser/reasoning presets; defer Git/CI/observability/database presets until official server pinning, auth injection, read-only scopes, and secret redaction are specified.
8. Preserve editable workspace copies; template updates must not overwrite existing agents, skills, or squads. Production actions, credentials, and destructive operations retain existing human approval.

## Child tasks and order

1. `09-24-rca-loop-verification`: verify current RCA routing and finish `09-23-debug-rca-e2e-model-eval`.
2. `09-24-incident-learning-canary`: incident learning and post-release verification.
3. `09-24-reliability-agent-evaluator`: reliability and Agent evaluation roles.
4. `09-24-experience-migration-roles`: workload-backed experience and migration roles.
5. `09-24-delivery-governance-recovery`: five governance capabilities and final integration.

## Acceptance Criteria

- [ ] Unknown defect causes route to diagnosis, known causes may go directly to implementation with an explicit reason, and incidents mitigate before RCA.
- [x] Incident learning produces prevention tasks with owners and acceptance signals, marking missing evidence as unknown; the local handoff fixture also links duplicates to existing work.
- [x] Post-release verification records artifact, baseline, observation window, signals, decision, and rollback outcome; digest/window/baseline failure cases stop as `unknown`.
- [x] Agent/Skill/MCP changes have versioned evaluation cases covering correctness, safety, cost, latency, tool failure and drift; real-model execution remains authorization-gated.
- [x] Two core roles can be created from templates with consistent skills, permissions, localization, tests, and documentation; experience/migration specialists remain behind explicit workload evidence gates.
- [x] Governance capabilities have triggers, deliverables, responsible roles, and verifiable checks.
- [x] One representative end-to-end rehearsal covers each loop from observed failure to prevention and from Agent change to drift monitoring.

## Out of scope

- Executing unapproved production actions or reading production credentials.
- Automatically rewriting existing workspace template copies.
- Adding one role per technology stack or governance topic.
