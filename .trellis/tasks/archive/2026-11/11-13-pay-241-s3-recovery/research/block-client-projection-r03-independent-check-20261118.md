# Blocking client R03 explicit-repair projection review

- Reviewer and correction owner: `/root/s3_check_batch_block`; active task `11-13-pay-241-s3-recovery`.
- Decision: **VERIFIED_EXPLICIT_REPAIR_CLIENT_PROJECTION_R03**. The source-confirmed visibility/readiness contradiction is fixed. This is code and mocked client evidence, not API/production Web/Desktop or full S3 acceptance.
- Main explicitly accepted this projection choice and supplied `block-client-explicit-repair-projection-admission-r03.json`. Only four admitted files in the fresh private R03 copy were edited. Frozen R01/R02, rejected combined inputs, ROOT and server wire/capabilities remain unchanged.

## Findings (fixed)

File: `packages/core/collaboration/presentation.ts` and canonical `presentation.test.ts`.

The actual server's detail projection deliberately sets generic Transfer/ChangeStatus false on managed blocked roots, including pending-submission cases. Its separate `information_workflow.actions.repair.available` can correctly be true. Core required both closed generic permissions for explicit repair, so a valid real detail had no repair action despite the mocked fixture passing.

Explicit repair now combines live write context, current creator/handler identity, existing edit authority, known managed blocked state and the specific current repair action. It does not restore generic permissions or accept a single availability flag as authority. Normal request and return continue to consume their own capabilities/current request/actionability.

Canonical tests now use parsed real server shapes with generic capabilities false. They cover standalone and pending repair; source-only missing write context, absent edit authority, a different current role, a current repair refusal and older unknown support remain denied. A complete authorized current request/return remains independent of generic transfer/status permissions. The previous pending fixture's unparsed current/latest mismatch was replaced with a valid parsed exact submission pair.

File: `packages/views/collaboration/workflow.tsx` and canonical `information-workflow.test.tsx`.

The entry filter and inner form repeated the same generic-capability contradiction. They now reuse Core's existing derived primary/secondary actions for current repair entry/readiness. No new public export, API field, state store or parallel permission logic was added. Existing repair fixtures now carry the actual closed generic capabilities; one additional pending happy-path test proves an explicit creator, exact pending submission/event, explicit in-review choice and one atomic handoff POST without B2/Resume/command/submission consumption.

## Findings (not fixed)

No remaining issue within this four-file correction scope. Full backend V2/wire/history/authority admission and actual matching runtime acceptance remain required separately. No server capability, public DTO, matrix or archive change was made.

## Verification

- Core RED:19 assertions,17 PASS/2 FAIL/0 pending; only actual standalone/pending repair visibility fails against unchanged R02 production bytes.
- Views RED:4 assertions,1 PASS/3 FAIL/0 pending; all repair cases fail at the missing Repair button, while normal managed Request passes.
- Final full focused Core profile:178/178 PASS,9 native files,0 failed/pending; all previous171 plus7 additional canonical assertions.
- Final full focused Views profile:94/94 PASS,10 native files,0 failed/pending; all previous93 plus1 pending wiring assertion. Backend/parser matrices were not duplicated in DOM tests.
- Final full Core and Views typecheck:exit0 each, noEmit/incrementalfalse/compositefalse.
- Lint:existing package configs, exact2 Core/2 Views files, no-ignore,0 errors/warnings/messages.
- Every invocation used the exact closed no-DB wrapper/config with NODE_ENV=test, actual Node22/installed tools and its private package cwd. Arguments/PIDs/config/tool/driver hashes, raw output and before/after source snapshots are retained. No database/API/browser/build/provider execution occurred.
- All3624 original R02 source/config/asset file bytes/modes remain immutable. R03 differs in exactly4 admitted files, without additions/deletions. All2010 dependency links independently reconcile to the appropriate R03 workspace/external dependencies. Every invocation's source snapshot has0 drift.

Private manifests, exact4-file patch, native assertion inventories and both unchanged RED receipts are under `independent-check/block-client-projection-r03` at `/Volumes/artisan/.multica-acceptance/pay241-s3-clients-20261118-x4c5mjep`. The adjacent JSON binds the final report and seal. The earlier report-write envelope error is retained separately and changed no source or report.

This candidate is ready for guarded source composition and fresh matching builds. Full S3 remains in progress; parent24 and all original failures remain retained.
