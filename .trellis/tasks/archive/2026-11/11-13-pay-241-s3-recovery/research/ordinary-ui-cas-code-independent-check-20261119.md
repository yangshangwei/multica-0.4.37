# Ordinary UI CAS code independent check — 2026-11-19

**PASS for the allocated UI product, owning-layer functional evidence and independent scoped static check.** All eight product paths implement the reviewed supplied-observation contract. Accepted unchanged functional evidence is **Core64 + Views216 =280 named PASS**, with no failed, skipped, pending, todo or duplicate case. Independent whole Core/Views TypeScript and scoped lint commands pass with zero source/config/tool drift. Views retains exactly one reproduced BASE warning. No product or test fix was required by this reviewer.

This is a code/static disposition, **not actual Web/Desktop UI CAS acceptance, TC-16 promotion, final R09 or full S3 acceptance**. Main owns the fresh whole client round, production artifacts, integration, executable spec sync and directed Lore commit.

## Ownership and source

Reviewer: `/root/root_output_integration_check`, existing native `trellis-check`; no agent was spawned. Active task: `.trellis/tasks/11-13-pay-241-s3-recovery`. The check manifest, current PRD → design → implement, root contracts, Core/Views quality/state/batch guidelines, reviewed UI plan/intake, coherent R02 allocation, implementation report and actual raw records were consulted.

Campaign root is `/Volumes/artisan/.multica-acceptance/pay241-s3-ui-cas-20261119-u691amwe` (`C`). Checker source is `C/source/client-ui-code-check-work-r01`, copied from frozen `client-ui-product-work-r02`. The authoritative `C/intake/ui-code-check-allocation-r01.json` pins this source, its **3,645-record manifest** SHA `63cb4a9e211115167320645ac2c6e4a6c50a2918eb1fd537b79754858886d46f`, and the new closed checker config.

Source write ownership was limited to the same eight product and ten test paths in that checker copy. No source write was needed. Reviewer ROOT writes are only this report and its JSON companion; independent command artifacts are under the assigned campaign verification labels. ROOT product/spec/task/config/DB/runtime/Git and all other sources, WIP and failures remain untouched.

The coherent BASE has3,644 records. Its comparison with implementation/checker WORK has exactly **8 product +10 test deltas**, zero unowned deltas and no changed mode; the sole new path is `packages/core/issues/field-revision.test.ts`. Every checker path matches the frozen implementation bytes/modes and the existing candidate manifest. All ten unowned coherence companions remain unchanged.

The broader current workspace-link inventory records31 visible `@multica` symlinks: **30 consumed nonmobile edges resolve into checker WORK**. The remaining root `@multica/mobile` edge resolves into original ROOT. No checked consuming code imports that package; Core/Views configs and the repository's nonmobile command scope exclude it. It is explicitly retained as an unconsumed edge, not silently counted as an owned resolution. `node_modules` is excluded from recursive source snapshots, so link resolution is a separately inspected current binding.

## Behavior verified against actual paths

| Owner | Actual final behavior and regression evidence |
| --- | --- |
| Core `issues/mutations.ts` | `withObservedIssueFieldRevision` uses only its supplied Issue, never QueryClient/current-highest cache. It preserves any already supplied token and attaches only a safe positive ordinary number for priority or any assignee field. Missing/invalid observations, collaboration projections and unrelated status/title/position/date/parent writes keep existing semantics. The hook/transport/optimistic code is unchanged. Core's26-case Node helper matrix and38 mutation controls pass. |
| `use-issue-actions.ts` | Captures the supplied rendered/captured-menu Issue before existing run-confirm or surface/direct delegation. The stale-menu control sends renderedR11 despite a detail cache atR12. Priority/member/clear and Agent/squad intent controls retainR11. Current source classification and completion-hint routing remain with their existing owners. |
| `board-card.tsx` | Real priority/assignee callbacks send the card observationR17; callback dependencies include the Issue, so same-ID props atR18 sendR18. Missing-version controls preserve omission. Source collaboration edit guard and navigation are unchanged. |
| `table-view.tsx` | The actual row Issue passes into the update callback; revision attaches before the existing run-confirm/surface action. Real pickers send rowR23 despite detail cacheR24; Agent/squad intent carriesR23. No latest-cache lookup is added. |
| `issue-detail.tsx` — SubIssueRow | The direct leaf uses childR8, independently of parentR3; missing child revision keeps omission. A structured conflict shows the existing localized revision message and keeps one original mutation, with no automatic resend. Existing status/completion/source-child guards and callback dependencies remain. Main-detail/menu callers use the shared action hook. |
| `run-confirm-gate.ts` | Assign and promote intent types carry optional `issueRevision` only from an already supplied `updates.expected_revision`. GateIssue gets no new observation field. Existing category/backlog/owner rules remain; no token is manufactured for status-only promotion. |
| `modals/run-confirm.tsx` | Existing optional `issueRevision` is appended in `applyTo` and preserved through single/batch Go, suppress and handoff writes. The token comes from the original modal data, not later query data. Conflict keeps the original note/token and allows explicit user continuation; cancel writes nothing. |
| `batch-action-toolbar.tsx` | Uses the existing selected rendered intersection, not raw IDs or newest-cache revisions. One/all-equal safe positive observations get the existing common token; empty, missing, invalid or heterogeneous observations omit it. Agent/squad confirmation retains originalR7 afterR8 rerender; all-backlog bypass retains the same token. Partial/unknown outcomes, selection, status and collaboration refusal remain. |

The helper's early guard leaves pure status/completion/drag and other field owners unchanged. Core still excludes expected revision from optimistic entity patches, cancels reads synchronously and owns rollback/invalidation. Its real QueryClient control forwards explicitR5 despite cacheR6, rolls back the refused optimistic priority, invalidates, then accepts a separate explicitR6 write. No auto-retry, cache-derived renewal, new protocol/backend/migration/dependency, per-item batch map or fan-out appears in the complete source delta.

## Functional evidence: complete GREEN and retained RED

The reviewer re-read all **14 retained command/result/source/config/tool/raw bindings** in the implementation JSON. Every raw hash and before/after source/config/tool comparison matches; each actual child has a closed environment. This does **not** say all fourteen commands passed: the six original RED bodies and one failed baseline typecheck remain failed; seven final GREEN/static/base-warning checks passed.

| Exact coherent R02 body | Raw names | PASS / FAIL / SKIP | Disposition |
| --- | ---: | --- | --- |
| `ui-product-r02-core-red-r01` |64|38 /26 /0|Unchanged product: missing helper; retained RED |
| `ui-product-r02-views-red-r01` |216|183 /33 /0|28 field failures plus five partial-navigation-mock failures; retained |
| `ui-product-r02-views-red-r02` |216|188 /28 /0|Corrected stable real StoreApi mock, product still unchanged; intended RED |
| `ui-product-r02-core-green-r01` |64|64 /0 /0|Accepted original functional GREEN |
| `ui-product-r02-views-green-r01` |216|216 /0 /0|Accepted original functional GREEN |

The corrected Core/Views RED and GREEN name sets match exactly by actual `(file,fullName)` identity. Both GREEN JSONs contain every expected assertion once, terminal `passed`, empty failure messages, all physical files passed with empty file messages, `success:true`, no pending/todo test, and actual command exit0/empty stderr. Core has two physical files and Views eight; their `numTotalTestSuites` also counts nested describe groups and is not a physical-file or business-case denominator. The installed Vitest JSON does not emit the Jest `numRuntimeErrorTestSuites` field; no zero count is inferred from that absent property.

All3,645 final implementation source paths equal checker WORK bytes/modes. The final GREEN before/after manifests match that same current normalized source map. No new test body is necessary just to relocate the unchanged tree to checker WORK. The reviewer ran only the independent static commands below; none of the280 functional cases was replayed.

| Owning test | Accepted named cases |
| --- | ---: |
| Core `issues/field-revision.test.ts` |26|
| Core `issues/mutations.test.tsx` |38|
| Views `issues/actions/__tests__/use-issue-actions.test.tsx` |25|
| Views `issues/components/board-card-assignee-picker.test.tsx` |4|
| Views `issues/components/table-view-editing.test.tsx` |13|
| Views `issues/components/issue-detail.test.tsx` |90|
| Views `issues/actions/run-confirm-gate.test.ts` |27|
| Views `modals/run-confirm.test.tsx` |27|
| Views `issues/components/batch-action-toolbar.test.tsx` |13|
| Views `issues/components/batch-action-toolbar.confirm.test.tsx` |17|

The initial R01 modal-error input and unsupported test-only role options, the R01 hybrid type/build failure, coherent R02 partial-navigation-mock failures, and all previous evidence remain immutable. Their corrections are attributed to the implementation/Main owners, not this reviewer.

## Independent static execution

All new commands use `C/helpers/record-run.py`, checker WORK as source/CWD, and `C/configs/ui-code-check-static-r01.json` SHA `40299642b20a4c0ce9c0d282223c2a8a76cb4aecaa12fe302b2075587ce7c212`. The config has no DATABASE_URL/task DB/JWT value and empty model credentials; the child inherits no ambient environment. Trusted HOME/TMP paths belong to the dedicated checker state. No service/body/API/DB command runs.

| New reviewer label | Scope / observed result |
| --- | --- |
| `ui-code-check-core-type-r01` | Whole Core `tsc --noEmit --incremental false`; exit0, stdout/stderr empty |
| `ui-code-check-views-type-r01` | Whole Views same flags; exit0, stdout/stderr empty |
| `ui-code-check-core-lint-r02` | Existing package config, three owned files, JSON/max-warnings0; exit0/errors0/warnings0 |
| `ui-code-check-views-lint-r01` | Existing package config, fifteen owned files, JSON/max-warnings1; exit0/errors0/exact known warning1 |

Every selected new command has identical3,645-record before/after/current source, unchanged config/tool hashes and zero drift. Exact argv, executable/config/recorder hashes, timing, stdout/stderr and source manifest hashes are in the JSON companion.

## Findings (fixed)

- File: `C/verification/ui-code-check-core-lint-r01/command.json` and its retained result/logs.
  Issue: the reviewer's first Core lint command named a nonexistent package-local ESLint entrypoint and exited1 with `MODULE_NOT_FOUND`; no lint body started.
  Fix: verified the installed WORK-root ESLint path/SHA against the original successful implementation command, then used the new `ui-code-check-core-lint-r02` label with the same source/config and exact three files. It passed. No dependency, source, config or original failed record changed.
- File: these new reviewer reports.
  Issue: an initial exploratory raw parser expected a Jest-only runtime-error count absent from this Vitest JSON.
  Fix: reconciled the actual reporter's success, physical-file messages/status, complete assertion terminals/failure messages, exit and stderr. The initial diagnostic and original raw remain unchanged; no test replay or fabricated field was used. The first malformed report-patch invocation created no file and was corrected before the report was written.

No product/test violation needed a source fix. Final checker source remains the allocated manifest, unchanged.

## Findings (not fixed)

- File: `packages/views/issues/components/table-view.tsx:1849`.
  Issue: one `react-hooks/exhaustive-deps` warning for the existing `resolveStatusLabel` dependency. The exact rule/severity/message/column/unchanged callback matches coherent BASE line1848 and the implementation's final lint; the import explains the one-line offset.
  Why left: this is a reproduced pre-existing callback outside the field-intent change. Main explicitly directed preservation. No warning suppression or unrelated callback fix was introduced. Scoped lint passes with this exact baseline qualification.
- File: current workspace dependency inventory.
  Issue: one visible root `@multica/mobile` edge resolves outside checker WORK.
  Why left: it is unconsumed by these nonmobile scopes; the30 used edges resolve into WORK. Dependency-link/config changes are Main-owned and outside the18 source paths. No mobile acceptance or hidden initial-edge snapshot is inferred.
- Scope: actual ordinary UI CAS, ROOT integration and final S3/R09.
  Issue: these remain separate acceptance work. Main reports whole UI R01 NOT_ACCEPTED: both platform tops failed on the Assignee accessible-name fixture before genuine PUTs, with a separate retained admission sequencing failure; gate remained epoch2 and connections0. That round is not a missing-revision UI RED or a product/code failure. E2E's fresh repaired source and its whole runtime proof remain Main/independent-runtime-review owned.
  Required next proof: both genuine same-R picker PUTs on both platforms and both commit orders, exact200/409 `revision_conflict`, unchanged losing field/visible conflict, explicit freshR+1 continuation, separately labelled versionless API control, matching production source/program/client/native/gate/PID/connection closure. Static/DOM/API controls cannot substitute.

## Final freeze and Main handoff

The checker candidate is frozen by its unchanged allocated manifest and complete18-path hashes in JSON. Main may copy this accepted candidate for GREEN production and exact delta integration. No further checker source writes are planned. Existing output/runtime/notification/batch/block evidence remains separately source-qualified; no body is rerun merely because this code review completed.

Main should sync the supplied-rendered revision, preserved explicit token, four actual leaf owners, original confirmation token and common selected batch compatibility contract into executable Core/Views specs. Integration must preserve all other ROOT WIP/modes; this report authorizes no unrelated iteration, backend, dependency or task-state edit. Real models, funds, production publication, installed Main and S4/S5 remain separate scopes.

## Verification

- Lint: **PASS**, errors0; Core warnings0, Views exact retained BASE warning1.
- TypeCheck: **PASS**, fresh independent whole Core/Views commands.
- Tests: **PASS by independent artifact/source reconciliation**, original Core64/Views216 named GREEN; no body replay.
- Source/config/tool/raw/name-set/workspace binding: **PASS within stated scope**;18owned deltas, no unowned/mode/source drift.
- Product/test source changes by reviewer:0. Actual UI/DB/API/Provider/Git/process controls by reviewer:0.
- Actual UI CAS / TC-16 / final R09 / full S3: **not accepted by this report**.
