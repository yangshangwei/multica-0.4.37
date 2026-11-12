# Capture correction — complete packet revision 2

Ready for a **full independent review of all 30 images**, covering every one of the 18 required platform/state/viewport groups. The packet contains 26 fresh images from `recapture4` plus 4 unchanged Web purge/restricted images from `final3`. It supersedes 23 prior images; all originals and both failed recapture attempts remain intact. `review-gallery.md` lists the entire current set. `final-screenshots.json` has exact paths, hashes, PNG dimensions, logical viewports and coverage. `capture-packet.json` records retained and superseded groups.

The two affected journeys passed together in **33.532 seconds**, zero failures/skips/flaky outcomes/automatic retries. They retain real TestApiClient setup, external-handler proposal, human exact-version approval, correction, workspace derivation, separate base context and managed lists, native authenticated account WS, and native purge/target-hint assertions. The six unrelated journeys were not rerun; the earlier 8/8 functional evidence remains scoped to its original run.

## Disposition of the three capture findings

- **VR-E01:** Web project at 640×960 now opens the real responsive properties sheet after the media-query commit/effect boundary and restores the selected review. Two opaque, unobscured overlapping images include the managed list, full review, final actions, Version history and panel bottom.
- **VR-E02:** Native project at 640×960 uses the same real control and has two settled opaque captures. The helper checks finite animations, stable geometry, sheet state/opacity, clipping and hit-testing before and after each actual compositor capture. Background blur belongs to the incumbent sheet backdrop; the sheet text is opaque and does not overlap it.
- **VR-E03:** Native workspace at 640×960 now covers the current panel height through its bottom, with the visible Version history summary fully inside the clip rectangle. Coverage uses viewport plus every clipping ancestor, refreshes dimensions for each image and checks the final panel height, rather than the initial height. Hidden history-entry buttons are excluded from the last-visible-control proof.

## Files and evidence

Only `e2e/fixtures/collaboration-knowledge.ts`, the three capture-call arguments in `e2e/collaboration-knowledge.spec.ts`, and this evidence directory changed. The helper enters targets wide before the deliberate wide-to-narrow capture cycle, waits for the actual panel/sheet branch, restores reviewed selection, waits for settled content and captures with overlap until the current bottom and last visible control are covered. No product edit, detector, model, agent CLI discovery, dependency/lockfile change or commit was made.

The product remains the exact frozen QA snapshot: base `53a1662586555d14ea04efb3b951744a4993fa60` plus the unchanged 117-file server overlay. All overlay hashes match, and the only tracked frontend difference is Next's known generated route import. Production Web was rebuilt from those same bytes on restart; its new build ID and API listener identity are in `recapture-source-verification.json` and the recapture running records. The native renderer/preload hashes remain pinned. Strict TypeScript, scoped ESLint and whitespace checks pass.

`recapture1` ran two journeys: native passed, Web correctly failed the new opacity guard. `recapture2` preserved a Web reopen failure caused by carrying a previously closed mobile portal into the capture sequence. Entering each target at wide size before its explicit responsive cycle resolved that fixture precondition. `recapture3` passed Web. A final metadata correction excluded hidden collapsed-history entries; `recapture4` then passed both journeys. These are 6 actual executions of 2 existing tests: 4 passes and 2 failures, not 6 new independent cases. All failures and traces are retained. The implementer opened the three corrected final captures at original resolution; this is evidence validation, not an independent design verdict.

The current full-packet review is still pending. No product defect or fidelity/ship approval is inferred from the recapture result, and no G2/runtime injection or Provider understanding claim is added.

## Environment

Owned API 18324, production Web 13244 and renderer 5418 are stopped. No native process/profile remains. The dedicated database and QA checkout are retained, all three collaboration gates are false, and ordinary execution task count is zero. `recapture-cleanup.json` and `recapture-gate-safety.json` record this. `commands.md` has restart/cleanup commands with the retained CORS fix.

At main's request, `dependency-links.py` documents the safe local dependency reuse used by QA. It is provided for main to run in its own implementation checkout; this lane did not alter implementation dependencies.
