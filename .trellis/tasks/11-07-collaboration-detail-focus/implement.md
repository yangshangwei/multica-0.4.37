# Execution / cleanup plan

1. Capture pre-task working copies and existing visual baseline; inspect existing regression coverage.
2. Implement workflow dialog and menu in one lane, including feedback/recovery, source embedding and meaningful dialog tests.
3. Implement main hierarchy, participants/permissions and optional composer tools in a disjoint lane; preserve auxiliary attention/anchors and existing comment tests.
4. Run scoped regression suites, types and lint. Review current diffs independently and fix proven defects.
5. Batch wide/narrow visual verification with real components, keyboard/modal geometry and both locales; persist visual verdict. Make at most one consolidated visual correction pass, then confirm.
6. Update detail spec, record exact checks and limitations, finish task. Do not commit unrelated dirty work.

## Expected files
- workflow.tsx, workflow.test.tsx, source-bridge.tsx/tests: modal flow and existing source entry.
- collaboration-detail.tsx/tests, participants.tsx, comment-composer.tsx/tests: hierarchy and progressive controls.
- locales/en/collaboration.json and locales/zh-Hans/collaboration.json: only necessary labels, owned by hierarchy lane.
- Task artifacts, detail spec, bounded QA artifacts.

## Validation
pnpm --filter @multica/views exec vitest run collaboration
pnpm --filter @multica/views typecheck
pnpm --filter @multica/views exec eslint <changed files>
Impeccable detect on changed UI files; browser visual and interaction evidence with current sources.

## Completion
All six steps completed. Independent check fixes and final evidence are in verification.md. Existing workflow E2E selectors were adapted to menuitem semantics.
