# Execution and verification plan

## Plan review

- [x] User authorized a Trellis task plus modifications and verification of the published audit recommendations.
- [x] Reconcile all 17 findings with acceptance criteria; keep the gated E01 appendix separate.
- [x] Record source/runtime evidence and narrow file ownership.
- [x] Curate and validate implement/check manifests; review artifacts and start task.

## Implementation

- [x] Lane A: recovery helper and catalog/error-loop tests, content-aware grids, long property dialog, catalog labels/contrast and shortcut values, retained triage settings.
- [x] Lane B: member/permission recovery, avatar rejection, name validation, pending guards, awaited repository deletion and serialized timezone preference.
- [x] Lane C: provider read/recovery states, consistent GitHub pending behavior, connected contrast and retained MCP editing.
- [x] Lane D: safe PAT boundary, protected token dismissal/revocation, desktop control names and update announcement.
- [x] Merge lane locale fragments sequentially; run English/Chinese parity checks.

## Quality gates

- [x] Meaningful failing-before/passing-after regressions in each changed package; existing protections retained.
- [x] `pnpm -C packages/views test settings` plus affected triage/common suites.
- [x] Focused core PAT and affected save/query tests.
- [x] Focused desktop settings tests.
- [x] `pnpm -C packages/views typecheck`, `pnpm -C packages/core typecheck`, desktop node/web type checks; web consumer typecheck when feasible.
- [x] ESLint on changed source/test files, static detector on changed UI, whitespace/source-boundary review.
- [x] Trellis check subagent reviews final combined diff, requirements and any new regressions.

## Native acceptance

- [x] Reuse the disposable Electron settings harness; keep real writes/agent execution blocked.
- [x] 900px and 450 CSS px catalogs have pointer-reachable menus; 12-option dialog remains usable.
- [x] Eight named controls, current shortcut values and updater live status in AX/DOM.
- [x] Light/dark warning and connected text contrast >=4.5.
- [x] Bounded status failures, truthful collection/permission errors, retained MCP/triage input nodes.
- [x] No false avatar success; clear invalid-name feedback and correction/save; one pending invitation request.
- [x] Token close/revoke protection, malformed response recovery; GitHub and timezone pending controls; failed final-repository deletion retains retry.
- [x] Record frontend-fixture versus real-service limits, source fingerprints and cleanup.

## Finish

- [x] Update recovery/accessibility specs if new executable contracts need preserving.
- [x] Write remediation report mapping F01–F17 to changed files and validation evidence.
- [x] Record task/session completion per Trellis; retain unrelated changes and do not deploy/publish.
