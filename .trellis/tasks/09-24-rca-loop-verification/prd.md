# RCA 小队接入验收与真实模型评测

## Goal

Prove that the landed diagnostician routing behaves correctly for defects, maintenance failures, and incidents, then complete browser template creation and real-model RCA evaluation.

## Requirements

- Verify `bug-fix`: after reproduction, unknown causes go to diagnosis; a direct fix records why diagnosis was skipped; diagnosis does not submit the formal fix.
- Verify `incident`: request human intervention and mitigate first; after recovery confirmation create a separate root-cause task; the incident does not wait for RCA.
- Verify `maintenance`: unexplained flaky tests and upgrade failures use diagnosis and return to bug-fix or implementation when classified.
- Reuse `09-23-debug-rca-e2e-model-eval` for browser creation and real-model behavior, preserving raw evidence and environment limits.

## Acceptance Criteria

- [ ] Squad roster, squad instructions, leader instructions, creation API, and docs agree on the handoff and copy-preservation rules.
- [ ] Targeted Go template tests and the real creation path pass, with actual counts and skips reported.
- [ ] Browser E2E creates a diagnostician from the template and verifies default skill, autonomy, presentation, and existing-copy preservation.
- [ ] Real-model evaluation covers reproducible, intermittent, or evidence-insufficient failures and reports causal evidence, tool calls, stop conditions, and authority boundaries.
- [ ] Any unavailable environment is recorded with an exact blocker and repeatable follow-up; independent work continues.

## Dependency

This is the first child. RCA routing landed in `bfc218434` and `3cef3556d`; this task primarily verifies it and fixes findings. The existing browser/model child owns that evaluation scope.
