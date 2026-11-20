# Ordinary UI numeric revision product implementation — 2026-11-19

Owner: `/root/ordinary_ui_product_implement`, the dispatched native `trellis-implement` child. Active task: `.trellis/tasks/11-13-pay-241-s3-recovery`.

Current result: **eight product paths and ten test paths implemented; scoped static verification passes on coherent WORK R02.** Core 64/64 and Views 216/216 named tests pass, with no failures, skips, duplicate identities or unhandled runtime errors. Whole Core/Views typechecks pass. Scoped lint has zero errors; the one existing table warning is independently matched to frozen BASE. Independent review and the genuine Web/Desktop UI journey remain Main's separate acceptance steps.

Current implementation and static verification are in `/Volumes/artisan/.multica-acceptance/pay241-s3-ui-cas-20261119-u691amwe/source/client-ui-product-work-r02`. R01 is retained as history. ROOT product, specs, task state, Git, runtime, configuration and databases are read-only for this lane. Other agents' edits and every original failure are retained.

## Change boundary

Ordinary priority/assignee UI callers currently omit the numeric revision supplied by their rendered Issue. Core already forwards an explicit `expected_revision`, excludes it from optimistic entity fields, and owns rollback/invalidation. The smallest change is to attach the supplied observation at the shared action hook, board card, table row and SubIssueRow, then preserve it through the existing run-confirm `issueRevision` slot. A selected rendered batch receives the existing common token only when every selected Issue has the same known positive safe integer revision.

The reviewed eight product paths and ten test paths are recorded in `ordinary-ui-cas-allocation-20261119-r01.json` and `docs/plans/2026-11-19-pay-241-ordinary-ui-cas.md`. No cache lookup renews an observation. Existing transport, optimistic cancellation, collaboration classification/authority, decimal collaboration versions, status/completion/drag/drop, partial/unknown results and versionless API behavior remain with their current owners. There is no protocol, dependency, migration, fan-out or retry-system addition.

The new Node helper suite owns the observation matrix. Existing Core mutation tests own real QueryClient transport/optimistic/rollback controls. Views tests assert actual caller, gate and confirmation wiring instead of repeating the helper matrix. Tests are written and recorded against unchanged product bytes before implementation; subsequent runs use new labels under the closed `configs/ui-static-r01.json` recorder.

## Initial binding

ROOT HEAD is `89740ed14d1abf1bdc7ba7da26144984884c7cb5`; the index is empty at this lane's initial read. All eight product paths and nine existing test paths match the frozen BASE bytes. The new `packages/core/issues/field-revision.test.ts` is absent from both trees. Core/Views workspace links resolve into the product WORK, and the existing Vitest, TypeScript and ESLint tools are available. Static configuration has no database URL or model credentials.

## Verification and handoff

The first unchanged-product RED is retained under `verification/ui-product-core-red-r01` (38 PASS / 26 FAIL) and `verification/ui-product-views-red-r01` (188 PASS / 28 FAIL), with zero skips and no source/config/tool drift. Core's 26 failures identify the missing new helper export; all 38 mutation controls, including the new transport/optimistic/rollback control, pass. The actual Views caller/gate/modal requests omit their required observed token.

One first-round modal conflict assertion used a plain error object, but the real `errorCode` boundary intentionally recognizes `ApiError` only. `superpowers:systematic-debugging` traced the failure to that fixture at `api/client.ts:errorCode`; only the owned test input was changed to a real `ApiError` with the original structured 409 code. The original failed round remains immutable. A new unchanged-product Views RED label records the corrected input before product changes.

The corrected unchanged-product `ui-product-views-red-r02` remains 188 PASS / 28 FAIL, zero skips; its modal conflict now reaches the expected localized feedback and fails on the actually omitted token. Whole Views baseline `ui-product-views-type-baseline-r01` exits 2 without source/config/tool drift. It reproduces the unrelated `issue-detail.tsx(2430,333)` iteration `open`/`onOpenChange` props mismatch. Main identified a hybrid assembly: product WORK's old `iteration-participation.tsx` SHA is `5fdf978786569b8af7dc1fcc877741bd89233501de308334d82c2d3fd3705c78`, while current ROOT has the matching newer callee. This lane changes no iteration product. Three additional errors are owned test-only `findByRole` options; removing unsupported `exact: true` preserves their exact string-name matching. The failed typecheck is retained. All eight product paths are still unchanged; Main is allocating coherent BASE/WORK R02 and carrying only the ten owned tests before further implementation.

At the R01 checkpoint, implementation and actual GREEN/static evidence were pending. Those original failure records remain retained. This report does not accept live UI CAS, parent TC-16, full S3, or later S4/S5.

## Coherent R02 continuation

Before this report was updated, its R01 Markdown and JSON were copied byte-for-byte to campaign `intake/ordinary-ui-product-r01-report.md/json`, retaining mode `0644`. Their SHA-256 values are `a1345927f17927cd62b3f047b694809970284324efeb18a36f8086dd4e29efdd` and `c9b4db71508358222439772e7830d5e81aae574fbfa4375e1c2da724d8ec7078` respectively.

Main's authoritative `intake/ui-base-coherence-correction-r02.json` (SHA `e134e442886c996d5cb221107ba801264380239da3f5e3c139bb3486426c7d28`) binds ten unowned ROOT companions and the coherent BASE manifest `d0595fd5abd8ca962bfcc1b2404155bf7c4edb40bdcfad7886ea77e75f312b98`. This child changes none of those companions. The eight product files remain equal to coherent BASE/current ROOT; all ten tests match the frozen final R01 test bytes/modes. Consumed Core/UI workspace links resolve into WORK R02. New closed `configs/ui-static-r02.json` SHA is `d4e2f01d9a675773755d914874165bfca6432a6a41236448e058680cdabbf2c8`; body gates remain zero and there is no database URL or model credential. Fresh R02 RED precedes product edits.

Fresh R02 `ui-product-r02-core-red-r01` is 38 PASS / 26 missing-helper FAIL. `ui-product-r02-views-red-r01` is 183 PASS / 33 FAIL: the same 28 intended field/confirmation failures plus five existing route controls whose partial navigation mock lacks the newly carried `useIterationParticipationStoreApi` export. The actual route/context and Core store were read; only the owned `issue-detail.test.tsx` mock now supplies a stable real Core StoreApi for each test. No route, navigation, iteration or other unowned product changes were made. The original 33-failure run remains retained. Corrected unchanged-product `ui-product-r02-views-red-r02` is 188 PASS / 28 intended FAIL before any of the eight product edits.

## Final implementation and verification

The shared pure helper attaches only the supplied safe positive numeric observation for ordinary priority/assignee intent. Detail/menu, board, table and SubIssueRow capture that observation before their existing delegation. Board dependencies track the rendered Issue. SubIssueRow reuses the existing localized conflict toast. The gate echoes only an already supplied token into `issueRevision`; modal single/batch Go, handoff and suppress submissions preserve it. The toolbar uses a common token only for equal known selected rendered observations. Current cache classification, transport, synchronous optimistic cancellation, rollback, status/completion/drag/drop and partial/unknown selection behavior were not refactored. No dependency, protocol, backend, migration, fan-out or retry system was added.

All commands use campaign `helpers/record-run.py` and the hash-bound closed R02 static configuration. Vitest runs use JSON and no file parallelism; typechecks use `--noEmit --incremental false`; lint runs from each owning package with the explicit changed paths. Each command retains its exact argv, raw stdout/stderr, source-before/after, config/tool hashes and result.

| Final check label | Observed result |
| --- | --- |
| `ui-product-r02-core-green-r01` | 64 named PASS, 0 FAIL/SKIP; 26 helper cases plus 38 mutation controls |
| `ui-product-r02-views-green-r01` | 216 named PASS across eight suites, 0 FAIL/SKIP |
| `ui-product-r02-core-type-r01` | Whole Core exit 0, stdout/stderr empty |
| `ui-product-r02-views-type-r01` | Whole Views exit 0, stdout/stderr empty |
| `ui-product-r02-core-lint-r01` | Three owned files, 0 errors / 0 warnings |
| `ui-product-r02-views-lint-r01` | Fifteen owned files, 0 errors / 1 existing warning |
| `ui-product-r02-views-base-lint-r01` | Frozen BASE independently reproduces that exact existing warning |

The table warning is `react-hooks/exhaustive-deps` for `resolveStatusLabel`, at BASE line 1848 / WORK line 1849. The added import accounts for the line offset; this unchanged callback is outside the field-intent edit. No unrelated dependency fix was made. Core and Views RED/GREEN named sets match exactly. Every one of the fourteen retained commands has zero recorded source/config/tool drift; final current tool/config hashes also match their recorded values.

| Owning test path | Named PASS | Finite wiring / controls |
| --- | ---: | --- |
| `packages/core/issues/field-revision.test.ts` | 26 | Priority/all assignee shapes, safe/invalid/missing observations, explicit token, collaboration and other-field exclusion |
| `packages/core/issues/mutations.test.tsx` | 38 | Real QueryClient token forwarding, optimistic exclusion, rollback and explicit fresh token; existing status/move/completion/cache controls |
| `packages/views/issues/actions/__tests__/use-issue-actions.test.tsx` | 25 | Priority/member/clear R11; supplied menu R11 with cache R12; Agent/squad intents and old no-token/completion rules |
| `packages/views/issues/components/board-card-assignee-picker.test.tsx` | 4 | Real priority/clear R17; same-ID prop refresh captures R18; old member/Agent/squad avatar and missing-token controls |
| `packages/views/issues/components/table-view-editing.test.tsx` | 13 | Real row priority/clear R23 despite detail cache R24; real Agent/squad intent R23; old editor/status/source/navigation controls |
| `packages/views/issues/components/issue-detail.test.tsx` | 90 | Child R8 rather than parent R3; missing child version; conflict feedback/no automatic resend; old route/source/content/subscription controls |
| `packages/views/issues/actions/run-confirm-gate.test.ts` | 27 | Supplied assign/promote token only; existing category, backlog and absent-token decisions |
| `packages/views/modals/run-confirm.test.tsx` | 27 | Original R11 through single/batch Go/handoff/suppress, promotion, conflict, cancel, unknown/partial and shortcut controls |
| `packages/views/issues/components/batch-action-toolbar.test.tsx` | 13 | One/equal R7; heterogeneous/missing/invalid/empty compatibility; unselected R99 and unresolved IDs excluded |
| `packages/views/issues/components/batch-action-toolbar.confirm.test.tsx` | 17 | Surface/embedded member+clear R7, Agent/squad original intent across R8 rerender, backlog bypass, status and outcome/selection controls |

Final source audit compares 3,644 BASE records with 3,645 WORK records: exactly **8 product + 10 test deltas**, zero unowned deltas and zero changed modes. The only added path is the Node helper suite. All ten unowned coherent ROOT companions retain their allocated bytes. Thirty consumed workspace links resolve into WORK R02. Current WORK exactly matches the recorded source-after of the last whole Views typecheck. The adjacent JSON retains every named test, all fourteen raw/command/result hashes, complete eighteen-path before/after hashes, configuration/tool rechecks and the final source/link audit.

| Product path | Final SHA-256 |
| --- | --- |
| `packages/core/issues/mutations.ts` | `d1de248dfb739dfc2a372219fba7c729e302d750735f9abca636951160a10b77` |
| `packages/views/issues/actions/use-issue-actions.ts` | `314379827c1a46d08921b4a7990241cd6b8f38f38e7946a13bd3983202200406` |
| `packages/views/issues/components/board-card.tsx` | `cdfbfa10e643704d7c00419bf13be3357c12dcda9cdfdc1c93ef2085a25bd3df` |
| `packages/views/issues/components/table-view.tsx` | `84b2bb490bfe3d19d8121aafc99103a24b84f73c78a5b21c9197ec041bd62051` |
| `packages/views/issues/components/issue-detail.tsx` | `f1956a4bf2c0bb528de70bb170ea9f4c38b8d8c86799dd35e701f1b04d487434` |
| `packages/views/issues/actions/run-confirm-gate.ts` | `a117527ae793d02f9d14ef495cf583889d5ab0acc740f200d6ec1de2b07cfbaa` |
| `packages/views/modals/run-confirm.tsx` | `6974f2718b8b283a6cae8ca164703c913818b4815c21a1354da5dba50b01532d` |
| `packages/views/issues/components/batch-action-toolbar.tsx` | `190665f7d92ef027b2b12be548808c197cd779863767c005142c8ed0dead38b4` |

## Main handoff and limits

WORK R02 product and tests are frozen after the passing static checks. This child wrote only the eighteen private product/test paths and these two assigned ROOT reports, plus the explicitly requested private R01 report preservation copies. ROOT product/spec/task/Git/configuration/database/runtime ownership was not taken over. No Go, DB, live API, browser, Provider or model execution occurred in this lane.

Main should clone the frozen candidate for native independent `trellis-check`, run the separately allocated genuine Web/Desktop UI CAS journey, integrate only accepted deltas, capture the supplied-observation/confirmation/common-batch contract in specs and make the directed Lore commit. Existing missing-version and heterogeneous-batch compatibility are explicit limits. This product/static result does not itself accept live UI CAS, parent TC-16, full S3/R09 or later S4/S5. Main's failed UI admission/locator round is separate from this 280-name product proof and remains under its own retained evidence.
