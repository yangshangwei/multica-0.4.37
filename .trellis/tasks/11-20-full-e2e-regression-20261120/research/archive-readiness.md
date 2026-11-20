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
- [ ] Verify R3 final per-case reconciliation, repaired failures, source bindings,
      checks, explicit coverage limits and task-owned resource cleanup.
- [ ] Commit only task-owned work with Lore context and verification trailers.
- [ ] Archive the existing task, repair moved task-local relative report links,
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
were introduced. R3 must still bind/replay the changed output fixture and
finish its own acceptance before archive.

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
