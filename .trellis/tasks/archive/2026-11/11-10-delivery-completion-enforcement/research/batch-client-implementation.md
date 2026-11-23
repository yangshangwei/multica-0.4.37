# D6 batch client implementation

Date: 2026-11-11. Owner: native trellis-implement / implement_batch_client.
Scope: the 13 client/type/test files below; no backend, migrations, dependency, lockfile, task-state or spec edits. This is client protocol and cache behavior, not a complete delivery gate or M1 acceptance.

## Behavior

- Existing two-argument batch calls keep their wire body. Optional completion_bindings carries exact per-issue authorization/operation/revision; shared updates.completion, malformed/duplicate/extra bindings refuse before transport. MoveIssueRequest exposes the same completion/project-change controls as ordinary update.
- One small zod/parseWithFallback helper validates count, per-input index/ID/order, states, revisions and committed-prefix ordering for success and error bodies. Legacy full count and explicit no-mutation zero stay compatible; short counts, malformed/truncated results and failed item states cannot resolve as all-success. IDs are copied before sending so a caller changing its selection cannot change response correlation.
- BatchUpdateError remains an ApiError with original status/body, dynamic stale-auth check and retry-after metadata. Its optional parsed outcome preserves committed facts; outcomeUnknown distinguishes missing/uncertain outcomes. There is no write retry, replacement operation key or automatic resume. Redirects are errors to prevent a POST replay.
- Mutation control fields stay out of optimistic Issue snapshots. Failure restores only values still owned by this mutation, preserving newer WS/refetch values; then affected workspace lists/myAll/flat/table/details/children, inbox and requested issue timeline/attachments/tasks are invalidated for real reads. Legitimate full success keeps existing surgical board behavior.
- Surface actions return the original batch result. Toolbar success uses actual updated count and emits no update success for zero; errors retain selection and use existing failure feedback. Run-confirm already kept its dialog open on a rejected batch; the new regression protects this behavior.

## Verification

Final focused core inventory: **63 passed, 0 failed, 0 excluded** (33 API protocol/request tests + 30 existing and new mutation tests). Views inventory: **91 passed, 0 failed, 0 excluded** across toolbar, surface/controller, collaboration-boundary and run-confirm tests. These are frontend unit/component tests, not real DB or browser acceptance.

Core and Views full typecheck passed after fixing the new onSettled return-path error. Core full lint passed with no diagnostics; the final changed-core lint also passed. Views full lint exited 0 with 27 warnings, all in untouched files (existing hooks/unused disables); no warning-suppression or unrelated cleanup was added. Git diff whitespace check on the owned paths passed. New files were checked for UTF-8, trailing whitespace and single final newline.

The final request-ID freeze changed only ApiClient and its canonical protocol tests after the earlier Views run; core final tests, changed-core lint and final core typecheck cover that small change. The unchanged Views code/test hashes remain identical to its passing run. The parent still owns integrated frontend/backend and real authenticated route verification.

## Retained failures and run ledger

- client-contract-red01: 27 actual protocol behavior failures plus two test-authoring failures: new batch regressions were accidentally inserted under the single-update describe, so their mock was unavailable. Kept the record; moved them to the correct existing batch describe.
- client-contract-red02: 27 API behavior failures and one cache overwrite failure. The partial-refetch fixture initially lacked exact loaded detail facts and happened to trigger the old coordinator's conservative fallback. It was strengthened with both actual cached rows/details before classifying the specific no-refetch defect.
- client-cache-red03: both targeted cache regressions failed behaviorally (zero server refetch, newer server snapshot overwritten); 27 other tests were excluded by the explicit filter, not counted as covered.
- client-views-red01: five actual behavior failures (surface discards result, no-change reports success in two placements, selected count used instead of returned count in two placements).
- client-core-typecheck01 / client-views-typecheck01: the new error onSettled returned a Promise but success omitted an explicit return under noImplicitReturns. Added explicit undefined; both corrected package checks passed. This was an implementation typecheck failure, not behavioral red.
- Two initial read commands guessed nonexistent api/iteration-confirmation.test.ts and api/personal-access-token.test.ts. The tool refused the commands; rg --files located actual filenames. These were read-only lookup failures, not product tests.

Original raw logs and numbered JSON records are retained under research/runs and the task's private log root. The common launcher counts Go JSON events; these frontend counts are taken from Vitest's actual final JSON assertionResults, not its zero Go named_counts field. Each referenced raw log hash was rechecked before this report.

| Run | Exit | Observed result |
| --- | ---: | --- |
| client-contract-red01 | 1 | 29 pass / 29 fail / 0 excluded/pending |
| client-contract-red02 | 1 | 30 pass / 28 fail / 0 excluded/pending |
| client-cache-red03 | 1 | 0 pass / 2 fail / 27 excluded/pending |
| client-contract-green01 | 0 | 58 pass / 0 fail / 0 excluded/pending |
| client-views-red01 | 1 | 12 pass / 5 fail / 0 excluded/pending |
| client-contract-green02 | 0 | 61 pass / 0 fail / 0 excluded/pending |
| client-views-green01 | 0 | 91 pass / 0 fail / 0 excluded/pending |
| client-core-typecheck01 | 2 | non-test command |
| client-views-typecheck01 | 2 | non-test command |
| client-core-lint01 | 0 | non-test command |
| client-views-lint01 | 0 | non-test command |
| client-core-typecheck02 | 0 | non-test command |
| client-views-typecheck02 | 0 | non-test command |
| client-contract-final01 | 0 | 62 pass / 0 fail / 0 excluded/pending |
| client-contract-final02 | 0 | 63 pass / 0 fail / 0 excluded/pending |
| client-core-lint-final02 | 0 | non-test command |
| client-core-typecheck-final03 | 0 | non-test command |

## Owned source hashes

The parent may use this exact inventory for independent review; no files were staged or committed by this lane.

| File | SHA-256 |
| --- | --- |
| packages/core/types/api.ts | a4df60b596af6185721c4f624f68a63104845ac7877d2498592973964c96d609 |
| packages/core/api/client.ts | 178e5b139a3d34e7bfa372549860b6458aaba02479fd2e8bf229eb5bdbe03087 |
| packages/core/api/issue-batch-update.ts | 597e92d8f30fcf97d60e6bb01df86f4207caf3bfad14206c67b9426cf98347ed |
| packages/core/api/issue-batch-update.test.ts | 300e5170f048eb9ee7ad15cd44a42cb393f76efcc9ce232d32923e1321083d47 |
| packages/core/issues/mutations.ts | 9b80961cb0414d0a2bc06647abe6a2230ee4a90d858792bbff5144c9006b4342 |
| packages/core/issues/mutations.test.tsx | c340869b288f3fb45626f91289d0cf84b30b8ad82ff9e49f0700d9ab481968a7 |
| packages/views/issues/components/batch-action-toolbar.tsx | d8ae26c591674a8d999d4747492aba9a7a0d84f82616f6cad05138bb136963c9 |
| packages/views/issues/components/batch-action-toolbar.confirm.test.tsx | 2e574865976e4a0026c66a139a884b1f3862e5c3786110d5563565119c27c0f8 |
| packages/views/issues/surface/actions-context.tsx | a411a737118c546f49a3d58bd41c7733b5ea1ddc4019ca9c31a9086ac784673a |
| packages/views/issues/surface/use-issue-surface-actions.ts | 7c238bcc957a9722fa26c677258c63e6599cf90f23044778caa1bdb85bdda48c |
| packages/views/issues/surface/use-issue-surface-actions.collaboration.test.tsx | a4adef8cb82299a457adabb974a449ab4298e25d4a5dba0fbaa00dc4cc307cba |
| packages/views/issues/surface/use-issue-surface-controller.test.tsx | 4737eb2cbd8f8312556f578673707779255147aae55da2916c885a7ef2c78128 |
| packages/views/modals/run-confirm.test.tsx | c9990bba6552676e590f7cea873dccaebde48a6c7ffd3908f879cb0762f85545 |

## Remaining integration work

Backend must emit the frozen D6 shape and enforce per-item bindings. The client change deliberately supplies no new completion authorization UI and no whole-batch recovery engine. Parent-owned e2e/core-business-workflow.spec.ts currently expects exact old {updated:3}; update it alongside backend activation with real per-item state assertions. No DB, provider, browser, external message, deployment, push or commit was performed by this lane. Existing types/index.ts already re-exports all api types, so no barrel edit was necessary.
