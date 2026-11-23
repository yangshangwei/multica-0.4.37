# Landing and verification gate review

Reviewed at: 2026-11-08T11:09:47.956975+08:00

Scope: read-only review of the task-private `run-full-check.py` and `land-verified.py`, repository `scripts/check.sh` / relevant `scripts/dev-env.sh` paths, current task artifacts, previous independent review, and the original root preservation archive. No source re-audit, live runtime-fix review, product edits, tests, commits, branch updates or process operations were performed. Only this report was written.

## Findings (not fixed)

### Production build's generated route import needs a recorded reconciliation

`apps/web/next-env.d.ts` in the integration checkout currently imports `./.next/dev/types/routes.d.ts`. Production Next build changes that import to `./.next/types/routes.d.ts`. `land-verified.py:14` rejects any dirty integration tree, so a successful full gate can still fail landing admission on that known generated rewrite. The root checkout already has an unrelated tracked modification to this same file; committing the generated rewrite would also introduce an avoidable root overlap.

The repository explicitly handles this exact build/dev rewrite in `scripts/dev-env.sh:checkout_source_id` and the final-verification-identity spec. After the full run ends, retain the generated diff and before/after content hashes, prove the only difference from the tested Git blob is that exact import line with unchanged type/mode, then restore only this integration file to the verified blob. Verify integration clean and unchanged HEAD before running the landing script. Preserve root's own file exactly. Any additional mutation requires diagnosis; do not apply a broad restore or turn this into a general generated-file exception.

This is an operational reconciliation recommendation, not permission to ignore a test failure. The first full run remains failed regardless of clean source identity.

### Ending HEAD alone is not the final source audit

`run-full-check.py:22` records the command's exit code and ending HEAD, but does not record ending dirty status or a final tracked-source fingerprint. `land-verified.py:12-14` correctly refuses a changed HEAD or dirty tree at landing, yet closure should retain the actual final identity evidence rather than relying only on a raw `exit_code=0` field.

At the end of the new complete run, save final HEAD, raw porcelain status, the exact generated-file reconciliation above, and post-reconciliation clean status. Bind the complete log (hash and terminal result) and registry's API/Web provenance to that same run. Read the complete log, including Go package failures, race/panic output, production build failure details if any, and Playwright summary. The terminal exit status remains decisive; earlier partial passes must not be stitched into a successful full run. Review the new runtime fix separately before committing the candidate.

No script rewrite is required merely to repeat these checks; main can record the final audit alongside the run. Candidate product files must remain frozen throughout the run.

### Execution plan step 9 is stale relative to the approved handoff

`implement.md` step 9 still asks for all tests plus another complete `make check` inside the root after merging. Root intentionally contains unrelated source/document changes, so those tests would run a different source tree from the reviewed candidate and cannot prove that the clean candidate was landed intact. The latest user instruction and handoff instead require the complete gate on the clean integration commit and an exact post-landing preservation/source verification.

Replace step 9 with: verify root HEAD equals the tested SHA; verify each landed changed path is byte-identical to that commit; compare original unrelated worktree bytes/modes and staged diff against the saved preservation record; inspect unresolved entries/status and a scoped whitespace check; refresh the branch/worktree inventory without redoing the completed source audit. State explicitly that the unrelated dirty root overlay was preserved and was not certified by the clean-candidate tests. A later docs-only task-record commit may advance HEAD; retain the product candidate identity and prove its product tree is unchanged.

## Confirmed safeguards and preservation evidence

- The runner requires a clean integration checkout, uses a unique non-overwriting run directory, invokes `make check` without test-selection arguments, and records actual command exit status. Run 01 finished with exit 2; it is not a pass.
- `scripts/check.sh` uses ordered static/TypeScript/script/Go-race-vet/API/production-Web/Playwright stages under `set -euo pipefail`, one Playwright worker and zero retries. Its cleanup preserves a nonzero result and cannot report successful completion before the final stage.
- The landing script requires full-gate exit 0, matching candidate/ending/integration HEADs, a clean integration checkout, the expected root branch and fast-forward ancestry. It refuses changed-path intersections with root dirty/untracked paths and uses `--ff-only --no-autostash`.
- Its before/after fingerprints cover bytes, filesystem mode and symlink targets for known dirty/untracked paths; staged binary diff equality is checked separately. Changed landed paths must equal the verified commit. A merge refusal does not authorize resets, automatic stash manipulation or overwriting root changes.
- Independently compared root's current `git diff --binary --full-index HEAD --` with the original 09:47 archive: byte-identical. The current staged binary/full-index diff also equals the original empty staged patch. Root remains at `e541cfb436fed9c6733174479718b651e2cf030d` with 53 tracked dirty paths.
- Original root archive contains 766 explicit untracked paths: 26 belong to this integration task, 740 are unrelated. Every unrelated original untracked path still exists with identical bytes and mode. Four original task-owned documents changed as expected (`design.md`, `implement.md`, `research/runtime-handoff-conflicts-20261108.md`, `task.json`). New task records explain the growing current untracked count and do not invalidate the original preservation evidence.
- These are point-in-time observations. Repeat overlap/preservation checks immediately before and after landing after the new fix establishes its final paths. No current root overlap was found for the examined auth/runtime-lookup fix candidates; the final diff remains authoritative.

## Closure evidence and commands

1. Preserve run 01 unchanged and finish the narrow runtime fix review plus focused regressions.
2. Commit the reviewed fix, verify clean integration state, and run exactly one new complete gate under a new run ID. No simultaneous full run.
3. Require exit 0, complete-stage/summary review and final source/provenance audit. Retain skipped opt-in/platform cases as skipped; never describe them as executed paid-provider/native acceptance.
4. Run the disjoint-path landing script after the generated import reconciliation, then retain `root-before-landing.json`, `root-landing.log`, `root-after-landing.json` and the original-state comparison. Do not overwrite a prior failed landing attempt's evidence if another attempt becomes necessary.
5. Update ACs, execution status, source disposition and limitations only to match final evidence. Keep all original failure records.
6. Archive only this task with `task.py archive <task> --no-commit`, and use `add_session.py --no-commit` for its journal entry. The existing journal/index already include unrelated dirty changes; do not auto-stage/commit those complete files. Selectively commit this task's durable records with the Lore protocol, or preserve mixed journal changes uncommitted as the handoff allows.
7. Stop only owned verification services after checking their actual identity. Keep source branches/worktrees, backup refs, archives and diagnostic databases; no push, deployment or real paid Agent execution.

## Verification

- Static review and original root preservation comparisons: completed as above.
- Lint/typecheck/tests: not run in this report-only dispatch, as requested. Existing implementation review and full-run evidence were read, not re-certified.
- Product-fix approval and full-gate approval: pending the main session's dedicated review/new run. No unconditional landing approval is claimed.
