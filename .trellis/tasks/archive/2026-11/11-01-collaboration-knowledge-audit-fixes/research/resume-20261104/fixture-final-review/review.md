# Fixture final independent review

Verdict: **pass-with-notes**, zero blocking findings. This review covers the retained `collaboration-native.ts` and `collaboration-records.ts` delta against `d0b132fff`. No source was changed.

The Relations helper now uses Base UI's actual hover opening and waits for the unchanged exact actionable submenu item before clicking. Existing child detach/reattach, server state, notification and replay assertions remain in their unmodified callers.

Native capture keeps all seven original bounds/coverage assertions and the original scroll expression. It waits for two frame-spanning measurements, freezes target/root dimensions once, records actual scroll/clamping/full rectangles, and checks stability before and after the native compositor capture. It permits the intended next-tile vertical scroll and unrelated below-target content growth while rejecting target, root, viewport or capture-part drift. The one-pixel geometry allowance and final full coverage threshold are unchanged.

Both current source hashes match the actual latest pre-run snapshot. Raw log/report hashes match the recorded exit0; the report has exactly 17 unique passed terminals, one attempt each, no skip/retry/flaky/error. All 26 retained stability geometry files and 29 PNG tiles meet the frozen/post-capture/full-coverage checks.

Independent verification: strict no-emit compilation for both fixtures exited0; ten controls extracted from the actual source predicates exited0. Pinned candidate pnpm ran canonical views ESLint against the two explicit fixture paths with --no-cache: exit0, zero errors, two “File ignored because outside of base path” warnings. The canonical config does not cover either fixture, so this is an explicit lint scope gap rather than a fixture lint pass. No broad package tests/lint, browser, DB, service or Provider run was performed.

The old 274-versus238 capture did not save enough geometry to prove its exact cause. The successful one-case diagnostic and this hardening do not claim reproduction. The targeted17 result does not replace the required new full make check, final identity audit, named inventories, CCG gates or task7 final acceptance.

Source hashes: native `b4038d1ba4eeb3e47c3d9730326d2dcb7f41af07e929bd84013712a5775373f1`; records `752090f080a3eb2600f3d0609b9b19bfde29c011d12a94a6ceb86ddb90cb666a`. Full evidence and operation boundaries are in `review.json`.
