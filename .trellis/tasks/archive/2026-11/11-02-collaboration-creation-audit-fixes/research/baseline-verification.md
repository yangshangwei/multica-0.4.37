# Baseline verification (before this task implementation)

- Production build uses saved pre-implementation source, shared tokens/base CSS and current official resources; immutable source hashes and build PID in research/preview/baseline-build.json.
- Fresh Chromium context + synthetic API + in-memory StorageAdapter; no real API, account or Electron operations.
- Core canonical validation: `pnpm --filter @multica/core exec vitest run collaboration/delivery.test.ts collaboration/delivery-api.test.ts collaboration/delivery-form.test.tsx collaboration/access.test.ts collaboration/draft.test.ts` — 5 files, 43 tests PASS (988ms).
- Views original baseline before inherited source-picker edits: 3 files / 15 tests PASS.

| Finding | Current reproduced evidence |
|---|---|
| R1 | {'sourceLocked': False, 'titleRetained': False, 'navigationCalls': [], 'records': 0, 'dialogRemaining': 1} |
| R2 | {'role': None, 'name': '筛选选项', 'controls': None, 'activeDescendant': None, 'activeExists': False}; role=option count 0 |
| R3 light | source: 2.623:1, title: 1.555:1, description: 1.555:1, submit: 1.555:1, close: 1.555:1, recipientTrigger: 2.623:1, recipientSearch: 1.179:1 |
| R3 dark | source: 3.445:1, title: 1.837:1, description: 1.837:1, submit: 1.837:1, close: 1.837:1, recipientTrigger: 3.445:1, recipientSearch: 1.182:1 |
| R4 | {'messages': ['已选择自己', '任务未创建，草稿和文件已保留。你可以再次提交。'], 'preserved': '额度拒绝验证', 'posts': 1, 'sourceUnlocked': True} |
| R5/R6 390 | IDs visible=False; row heights=[32, 32, 32, 32]; close=28px; parent-search=19px; stage=32px |
| R7 390 | initial submit visible=False; bottom close visible=False; bottom submit visible=True |
| R5/R6 320 | IDs visible=False; row heights=[32, 32, 32, 32]; close=28px; parent-search=19px; stage=32px |
| R7 320 | initial submit visible=False; bottom close visible=False; bottom submit visible=True |
| R8 | {'rawImmediate': '协作任务改进', 'during': ['协', '协作', '协作任', '协作任务', '协作任务改', '协作任务改进'], 'after': ['协', '协作', '协作任', '协作任务', '协作任务改', '协作任务改进'], 'priorCandidateVisible': False, 'selectedBeforeDelay': '选择父任务（可选）. 可选。选择后，这项协作任务会成为它的子任务。选择父任务（可选）'} |

Baseline observations confirm all eight gaps. Search input is now inside the inherited source-task popup and is 19px tall, replacing the former 32px inline input.

The initial harness failures were localized-name selection (English Filter options vs Chinese 筛选选项), fixed-portal clipping analysis, and a generic-dialog 95% animation frame. They are fixture-calibration issues, not product failures. The eight product baseline cases completed; the corrected generic transition sample is retained separately in research/browser-baseline/generic-picker-dialog-compatibility/results.json. No baseline check is a claim of a fix.
