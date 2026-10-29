# 修复注册、窄屏推荐和收件箱回归失败

## Goal

复现并修复 QA-01 注册首次点击丢失、QA-02 375px 推荐挤压描述、QA-03 收件箱旧定位器，复跑六项失败及相关回归。

## Background and authorization

The 2026-10-16 run at main@5049e0f6a had 322 passing and six failing unique cases. The affected implementation/tests still match that baseline. Evidence: `.omx/reports/main-full-e2e-20261016/REPORT.md`. The user explicitly requested task creation, test reruns and repairs on 2026-10-17; the three existing acceptance contracts define the scope.

## Requirements and acceptance

- **QA-01 / AC1:** With focus in mismatched password confirmation, one submit-button click shows validation and focuses confirmation without sending registration. Correcting it allows exactly one successful registration; independent password visibility remains intact. Canonical regression: `e2e/password-registration.spec.ts:84`.
- **QA-02 / AC2:** With three recommendations at 375×812, English and Chinese quick creation keep at least half of the description editor and the complete Create button in view, without horizontal overflow. Every suggestion remains reachable and adoption explicit. Canonical cases: `e2e/quick-create-actor-picker-phase2.spec.ts:67,115`.
- **QA-03 / AC3:** Update three formal inbox tests to the current accessible native controls, preserving notification content, read/archive, persistence and narrow-screen return assertions. Canonical cases: `e2e/core-business-journeys.spec.ts:256` and `e2e/upstream-selected-fixes.spec.ts:131,200`.
- **AC4:** Capture fresh baseline failures, then pass all six original identities with zero skips/retries in correctly configured production environments. Run related checks and record exact results and server provenance.
- **AC5:** Preserve unrelated local edits, add no dependencies, and clean up task-owned services/data after verification.

## Out of scope

No backend/auth contract changes, ranking changes, broad redesign, deployment or unrelated cleanup. Rerun affected regressions and related suites; the previous 328-case report remains historical evidence.
