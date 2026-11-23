# Bounded failure repair — final verification

The requested 2 E2E and 4 unit failures are resolved on the isolated repair candidate. The final canonical run passed all 4 cases in 57.3 seconds, with no retries or skips: complete production Web and real Electron detail journeys, plus both platform scope/recovery regressions. Each complete detail journey restored exactly 620 and retained 19 capture records with the existing geometry checks.

## Changes

- QA001: register the three known squad creation routes in diagnostic matching while retaining unknown-route and ID masking.
- QA003: align both current progress-report descriptions with the embedded current-project default; recognize only the two verified shipped historical defaults through the existing localization branch, preserving customized text.
- QA004: disable browser scroll anchoring only on the account detail scroll root, whose explicit memento owns its offset. The original trace proves the disappearing 112px error plus32px gap moved620 to476 without a JavaScript write.
- E2E synchronization: Web reload waits for actual URL-comment focus before capturing; Electron reload restores its persisted tab without refocusing an old anchor. The recovery regression uses a failed read and the current Refresh control, holds and continues the original request, then requires its real200/completed transport, banner removal and unchanged620. No retry, timeout or geometry relaxation.

## Evidence and review

`final-results.json` indexes exact case identities, source hashes, raw results and all retained attempts. `review-final-20261107.md/.json` contains the independent source/evidence review. `scope-transport-root-cause.md/.json` retains the native transport probes: both synthetic fulfillment variants exposed status0 despite upstream200; continuing the original GET produced an actual renderer200. This was a test injection incompatibility, not evidence that a real server200 failed in production.

Original failures remain intact, including repaired-canonical1PASS/3FAIL and this session's canonical-final1 2PASS/2FAIL. Final evidence is additive. The immutable build provenance remains unchanged; revision-canonical-final2 proves the product inputs and494 renderer artifacts unchanged while separately pinning the two E2E revisions.

The original4 unit failures passed in targeted213 tests and expanded package runs. The15 supported product lint/typecheck tasks passed with30 warnings/0errors. Those product/unit diffs are byte-identical to the checked candidate. There are13,332 unique unit tests with passing evidence across runs, **not a single all-green run**: the initial Views run had3 failures in unchanged create-issue.test.tsx, whose unmodified61-test recheck passed. Their exact first-run cause remains unknown. Exploratory E2E lint using the Web config reports the same pre-existing errors asHEAD; the root E2E suite has no supported lint/typecheck project.

## Delivery boundary

Only the nine repair source/test files, two relevant specs and this repair's records are included. The generated next-env declaration is excluded after checking its single generated import difference. Main-tree source files are guarded against their baseline hashes before integration; unrelated dirty files are not staged or overwritten.

`cleanup-final.json` proves ports19793/14713/5981 closed, all five recorded service PIDs exited, their process groups empty, and zero other connections to multica_scroll_repair_20261107. Database, worktree and failed evidence remain available for diagnosis.

The broader branch acceptance task remains in_progress. Installer, other-platform, real-provider and unexecuted inventory gates retain their original limits. No new Go run was needed for unchanged backend products. No paid model call, push or deployment occurred.
