# Archive readiness, 2026-11-20

The user asked whether this existing full-regression task could be archived,
then requested continued handling of the identified blockers. Reuse this task;
no new task or full-regression rerun is needed for the archive review.

## Ownership

- R3 execution, repairs, production builds, reports and cleanup remain owned by
  the active regression session `01a1f7e5-4ff1-7242-a508-e02c52e55190`.
- Archive session `01a1f80a-1bd5-72a2-a31c-30806bebd632` owns the allocation
  guard repair, its independent review, final evidence review and archive move.
- Do not stop another session's processes, rebuild its clients, modify its
  private allocations or include unrelated working-tree changes in commits.

## Observed blockers

1. Task metadata still said completed from R2 while `implement.md` introduced
   R3. R3 has live browser/static/backend runners and no final report yet.
   Its initial core receipt has 91 expected, two unexpected and one skipped
   case. Keep original evidence and wait for verified repairs/replays.
2. `e2e/fixtures/collaboration-runtime-output-allocation.mjs` admits only the
   literal active task path and requires that directory to exist. A normal
   Trellis archive moves it, so future explicit output allocations cannot be
   admitted. Its Node test explicitly asserts the directory requirement.

## Smallest behavior change

Keep the full-regression task identity fixed. Permit the caller to explicitly
select its canonical active path or its normal Trellis archive path
`.trellis/tasks/archive/YYYY-MM/11-20-full-e2e-regression-20261120`.
Verify the selected real task directory and regular, nonsymlink `task.json`
identify this exact task. Continue requiring the allocation record to pin the
same explicitly selected path. Do not scan for tasks or silently redirect old
allocations after a move. Retained historical evidence is immutable; a later
run must create and pin a new allocation using the actual archived path.

The guard and its existing Node tests own this behavior. Only those two code
files and the Controlled Agent output allocation section of the E2E spec need
changes. Keep the legacy no-explicit-fields path and all database, source,
storage, ownership, origin, deployment and digest checks intact. This is not a
general task-discovery feature or a product change.

## Verification and completion

- [x] Capture a failing archived-task regression before implementing the guard.
- [x] Admit exact active and archived task identities with newly pinned records.
- [x] Refuse missing/foreign/malformed metadata, traversal/symlink paths and
      allocation/task disagreement; retain existing boundary tests.
- [x] Run Node guard tests, relevant syntax/static checks and independent review.
- [x] Verify R3 final per-case reconciliation, repaired failures, source bindings,
      checks, explicit coverage limits and task-owned resource cleanup.
- [x] Commit only task-owned work with Lore context and verification trailers.
- [x] Archive the existing task, repair moved task-local relative report links,
      verify archive admission against the actual new directory and journal.

R1/R2 passes remain historical. Archive completion cannot be inferred from
their counts or the old completed status.

## Allocation repair evidence

Commit `963461f67` contains only the guard, its Node tests and the E2E spec.
Original guard against the new tests: 22 pass, six fail. Implementer and
independent reviewer each obtained 28 passes, zero failure/skip. Both scripts
pass syntax and targeted repository-base ESLint; guard-only checkJs passes.
The real-filesystem regression proves a stale allocation fails after the
move and an explicitly repinned archived allocation succeeds. Exact commands,
hashes, red/green logs and independent review are retained under
`artifacts/qa/full-e2e-20261120-archive/`.

Running checkJs against the entire untyped JS test harness also reports four
pre-existing errors (filesystem mock shape and undocumented pg connection
parameters); clean HEAD copies reproduce the same errors. No suppressions
were introduced. R3's complete acceptance still precedes archival; the changed
output fixture was replayed successfully as recorded below.

The separate detail-journey permit accepts active-form task references without
checking the task directory. Archiving does not invalidate those retained
permits; this repair adds archived-path admission only to the output fixture.

## Archive preflight

The actual task-directory probe passed with current guard/task metadata hashes
and verified removal of its private temporary allocation. It opened no network
or database connection. After the final move, rerun
`artifacts/qa/full-e2e-20261120-archive/probe-task-allocation.mjs` with the explicit
archived path and a new receipt name. Do not overwrite the active-path receipt.

`task.py archive` itself does not verify acceptance or repair links. Its normal
move will clear both the R3 and archive sessions' active pointers, so only run
it after R3 hands off completed acceptance and cleanup. Branch metadata and both
four-entry context manifests validate. No child or parent task needs rewriting.
The two relative report links in `research/final-results.md` need two more
parent segments after moving; the exact changes are recorded in
`artifacts/qa/full-e2e-20261120-archive/task-link-move-plan.json`.

A read-only aggregation audit checked the 16:36 R3 snapshot: all 514 identities
and 613 included executions matched exactly, all 52 admitted reports were new
R3 Chromium runs, and historical backups/diagnostic results were not counted.
The snapshot correctly reported 465 pass, 11 fail, one timeout and 37 skip.
This confirms aggregation behavior only, not final acceptance; regenerate and
review after the remaining runs, repairs and production rebuild finish.

## Final output replay owned by the archive session

The R3 owner explicitly handed off `output-final/` in its `ownership.md`.
On production build `hkdVBnDw48prvVOiZ5KxE`, both original Web and Desktop
controlled output journeys passed (2/2, no skip/retry, 182 seconds) through guard
commit `963461f67`. All 16 pinned source/artifact files matched before and after.
This exercises actual local daemon/provider/CLI processes with deterministic
provider output, not real-model or packaged-Main acceptance.

Both daemon processes exited zero with no errors; all ten runtime-cleanup
owners fulfilled. A separately reviewed cleanup script checked the full
PostgreSQL container ID and task/owner labels, restored original execution gate
booleans with a monotonic epoch, stopped only this lane's API/Web/renderer,
required zero other client connections across its database cluster, then
stopped that exact container. Independent terminal checks confirmed four
recorded application PIDs gone, all four ports (18598, 13518, 5692, 59189) free,
PostgreSQL `Running:false` and no source drift.

Evidence: `artifacts/qa/full-e2e-20261120-r3/output-final/browser.json`,
`archive-owner-browser-command.json`, `archive-owner-before-cleanup.json`,
`archive-owner-cleanup-command.json`, `archive-owner-final.json` and
`archive-owner.exit` (0). The R3 owner has received the terminal handoff.

This output replay preceded the later ledger fix. A separate
`output-ledger-final/` allocation subsequently passed both original cases with
the final daemon/CLI bytes and 20 unchanged source/artifact hashes. Browser
execution began only after the R3 owner's explicit `browser-clear` window.

Final output evidence completed at 18:04 CST: 2/2 passes in 111 seconds, no
retry/skip, diagnostics disabled, both daemons exit zero with empty stderr and
all ten runtime-cleanup owners fulfilled. The final ledger hash matches the
independently reviewed `ff96546a…47c69` source below. Exact container ID and
task/owner labels were checked; original gates were restored, cluster clients
were zero before stop, and PostgreSQL was verified stopped. Application ports
18601/13521/5695 and PostgreSQL port 51951 were free; all four recorded process
IDs were absent. Its `archive-owner-final.json` and `archive-owner.exit` (0)
have been handed back to R3 for final case reconciliation. The earlier output
generation remains immutable evidence of its own source version.

## Additional independent R3 review

The archive-side reviewer found a database-provisioning regression: delegation
re-sourced a stale env file after the registry had selected its authoritative
connection. A harmless transport simulation proved the wrong endpoint, and the
R3 implementation owner fixed it with explicit `--current-environment` handling.
The same independent probe then proved the selected endpoint/credentials were
retained; the full fake-transport registry suite passed, as did all three shell
syntax checks. Original and follow-up reviews remain separate under
`artifacts/qa/full-e2e-20261120-archive/database-script-independent-{review,recheck}.md`.

Additional independent reviews retained in the same evidence directory:

- `test-repairs-independent-review.md` validates preparation ordering, exact
  completion approval selection and view-switch targeting without weaker
  business assertions. Category/discovery were unchanged-source replays.
- `host-diagnostics-independent-{review,recheck}.md` records the initial
  stderr compatibility defect and its verified explicit-opt-in correction.
  Default stderr/stdout and default absence of extra ledger reads are preserved.
- `ledger-open-rename-proof.*` proves the local atomic-replacement mechanism,
  not that a particular original browser failure experienced it.
- `ledger-allocation-independent-{review,recheck}.md` records and closes a
  security regression found in the first retry implementation: disappearance
  after an observed existing inode must not become fresh initialization.
  The unchanged independent upper-layer test moves from red to 2/2 green;
  the final private-file matrix independently passes 13 named race tests.
  Both private overlay directories were removed after their source and output
  were copied to evidence. The implementation remains owned by R3.

Final ledger source SHA-256:
`ff96546a3b9081317daee1e8dba642f9bd194746065a7d7b59d5c234efc47c69`.
Final browser acceptance must bind this version, preserve intermediate results
and distinguish sampled performance on earlier bytes from final-runtime checks.

## Final handoff and archive gate

R3 owner committed final task records as `fa1867259` and handed off all task
ownership. Final verification is 514/514 canonical E2E latest passes, 14,718 TS
passes, 23,691 default Go named passes plus nine explicit skips, 24,155 named
passes including supplements, and no missing or unresolved cases. The public
skills test passed after its original rate-limit window; its actual named PASS
and package PASS were independently checked.

Root cleanup independently verified 44 free ports, eight stopped PostgreSQL
containers, 109 absent runtime PIDs, removed final Main containers and restored
gates. Backend cleanup separately verified exact stopped PostgreSQL/Redis IDs,
ports 61608/61645 free, the reset runner absent and five owner0600 dump hashes.
All output-ledger-final source/runtime/cleanup gates above passed.

`final-archive-gate.json` under the archive evidence directory binds the final
reports and completed checks. Native mobile UI, Windows/macOS installation or
updates and real commercial model behavior remain outside demonstrated coverage.
Performance measurements remain bound to their earlier frozen binary; final
ledger functional acceptance is separately recorded.

## Archive completed

Moved to `.trellis/tasks/archive/2026-11/11-20-full-e2e-regression-20261120`.
All 11 task files were preserved; repository-root context manifests validate.
The old active directory is absent, both sessions' task pointers are cleared,
metadata is completed, moved report links resolve, and
`allocation-real-archived-probe.json` confirms actual archive-path admission
with temporary-allocation cleanup. The probe made no network or database
connection. `archive-move-verification.json` retains the move checks.
