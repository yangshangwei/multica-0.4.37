# Development plan

1. Capture baseline and inspect existing disclosure/recovery/anchor contracts.
2. Add optional presentation status/visibility props to existing attachments, memory and knowledge panels; retain standalone defaults and test status wiring.
3. Implement toolbar/persistent drawer and integrate detail composition, reference link and anchor handling. Add bilingual labels and meaningful interaction/lifetime tests.
4. Run collaboration tests, views types/lint and static detection; independent Trellis review of lifetime/recovery/permission handling.
5. Native Electron visual/interaction pass at wide and narrow sizes, draft close/reopen, category switch, keyboard/nested modal and main geometry. One consolidated visual correction pass if needed.
6. Update spec and evidence, scoped commit and task archive.

Expected files: collaboration-detail.tsx/tests; new detail-auxiliary.tsx/tests; panel-status.ts; attachment-panel.tsx/tests, memory-panel.tsx/tests, knowledge-panel.tsx/tests; EN/ZH collaboration locale; affected E2E helpers; task evidence/spec. Existing backend/API/delivery semantics unchanged. Baseline tests already protect current business behavior.

## Completion evidence
Implementation and independent review complete. See verification.md for final checks and limitations.
