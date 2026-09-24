# QA remediation

Implement the approved repair plan in docs/plans/2026-09-24-qa-remediation-plan.md.

## Requirements
- Production Web build and verified isolated API/Web identity for E2E.
- Lifecycle provenance, note, evidence, audit and queue writes must be authorized and atomic; compatible queue reuse remains idempotent.
- Correctness failures must never pass evaluation.
- Fix builtin catalog/localization and Observer instructions without rewriting existing custom copies.
- Repair confirmed test contracts/races and complete full regression plus dedicated Electron/publication fixtures.

## Acceptance
All F1-F7 and T1-T6 requirements and section 6 acceptance criteria in the approved plan are required. Preserve original failures and prove final results on current sources. No new dependencies, schema expansion or foreign keys.
