# Independent native capture stability review

The smallest defensible decision is **bounded stable measurement plus one frozen capture snapshot and per-tile pre/post verification** inside `captureNativeSection`. This is capture-protocol hardening. The old `274 -> covered238` failure has not been reproduced with saved geometry, so this review does not call it a proven height-change bug or a completed fix.

## Findings (fixed)

None. Product, fixture, task metadata and execution files were read only. This report and its JSON companion are the only writes.

## Findings (not fixed)

**NC-S1 — fixed offsets are not bound to a stable target/root snapshot.** The helper samples `focusHeight` and `availableHeight` once, derives offsets, then reads each tile's current geometry without asserting that size/root/content still match the initial sample. Mixing initial and changed geometry can invalidate a coverage claim. This is visible in the source independently of the unknown old failure cause.

**NC-S2 — pre-capture geometry is not verified against the saved native frame.** The helper updates its coverage and awaits `nativeScreenshot`/`capturePage`, but performs no post-capture equality check. The project guide already says a pre-capture no-overflow assertion alone does not prove what the image contains. Keep the current bounds and coverage logic; verify the same frame before accepting each tile.

**The old cause remains unknown.** The failed run saved initial height274 and final covered238, but did not retain final target/root boxes or scroll. A smaller final target, clipping, and insufficient achieved scrolling are not distinguishable from those values alone. The presence of `query.isFetching` content in `OfferConfirmation` is a legitimate possible layout-change mechanism, not proof that it caused36pixels in that old run.

## Actual diagnostic evidence

The new exact one-case diagnostic passed without an algorithm/threshold change. All12 stage/size records and14 tiles retain unchanged target heights and satisfy the original coverage condition. The adoption200% record has initial/current height238, full coverage238, a384px scroll root within a450x600CSS viewport, and requested/achieved scroll1984 below max2478. It is not clipped or clamped in this observation.

Several wide captures request scrolling beyond the root's maximum, but still expose their complete target. That clamping is legal. The200% cancelled/deleted records use overlapping tiles and cover378.75pixels. Do not reject all clamping or replace the scroll formula merely because requested and achieved offsets differ.

## Minimal implementation contract

1. After current native size/zoom and font readiness, use an existing bounded `expect.poll`/RAF condition to obtain two matching frame-spanning reads. Freeze the accepted target/root snapshot before calculating the original offsets. There is no fixed sleep and no extended whole-test timeout.
2. The global stable signature needs focus horizontal position/width/height, root x/y/width/height/clientHeight, and viewport width/height. Compare target node/textContent locally if semantic snapshot consistency is needed; log equality or a summary rather than adding plaintext body/secret values.
3. Do not freeze focus vertical position or scrollTop globally: each deliberate tile scroll changes them. Do not freeze scrollHeight merely because content outside the target can change; the actual per-tile geometry and achieved coverage remain authoritative.
4. For every tile, preserve current scroll, root bounds, no-overflow, overlap and full-coverage assertions. Verify dimensions/root/viewport/content still match the frozen snapshot, and compare full focus/root rectangles plus scrollTop immediately before and after native capture. Persist pre/post observations before refusing changed geometry.
5. Keep `coveredThrough >= acceptedInitialHeight -1`. A smaller later height must not replace the accepted initial target, and the helper must not silently restart against a smaller target after capture drift. Unstable or changed frames fail with durable evidence.

This requires only local measurement and predicate code in the fixture. No product form/query policy, CSS, dependency, global screenshot helper or new architecture is justified by the present evidence. A generic check for no disabled controls would also be wrong because intentionally disabled resource states can be valid screenshot targets.

## Verification

This review parsed the actual12 JSON geometry records, verified14 tile constraints, read the current/preserved helper and the caller/form/query source, and hashes every input in `review.json`. It ran no lint, type-check, test, build, SQL, service, container or Provider command. Existing test results are parsed observations, not this review's execution claims.

After implementing the bounded protocol, preserve new source hashes and run the same exact native case/current17 directed set, then the required complete shared gate. A focused synthetic protocol control can verify drift refusal and legal tile scrolling, but cannot replace the product/native run. This report issues no final candidate clearance. Knowledge allocation/calls remain0/0 and production injection remains default off.
