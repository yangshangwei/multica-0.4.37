# Final-state audit handoff

The auditor reuses the frozen v11 `verify_owned_bindings` implementation after
checking the independent review, current clearance, launcher/adapter/environment
hashes and Git HEAD/dirty inventory. It executes that implementation under a
non-main module name, so no Make command or registered launcher mutation occurs.

Only the exact generated Next import may change. The registered Git blob must
match its binding hash, the actual declaration must be a regular file at its
owned path, and all bytes must equal either that blob or its one exact import
replacement. A path-specific in-memory hash normalization leaves every other
source/tool/workspace/artifact guard intact. The report retains raw and normalized
Next hashes and product SourceIDs separately. Candidate files are never restored
or rewritten.

The audit also requires a completed run and checks its initial source, adapter,
binding, CLI, resource-proof and canonical-source fields against the final
bindings, plus the execution environment and complete log hashes. Reports use
exclusive mode0600 creation below this directory, including explicit failure
reports when required review data is absent.

## Verification

- `red.json` / `red.log`: ten expected failures before the auditor existed.
- `green-initial.json`: original ten regressions passed.
- `report-failure-red.json`: one expected failure among twelve cases exposed an
  empty-report error path when review initialization preceded error handling.
- `green-final.json` / `green-final.log`: twelve passed, zero failed/skipped;
  both Python files passed AST syntax parsing. Network was denied, Git/Node
  metadata processes were mocked, and all source/link/artifact mutations were
  confined to new temporary fixtures.

The regressions cover clean state, allowed generated import, unrelated tracked
mutation despite an empty Git status, foreign workspace target, changed desktop
artifact, extra Next declaration, Next symlink replacement, changed pinned tool,
changed canonical execution environment, incomplete run, failure-report retention
and prior-report preservation.

## Root entry after the complete command

```sh
/opt/homebrew/opt/python@3.14/bin/python3.14 \
  /Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/10-30-collaboration-fix-closure/research/continue-20261103/final-state-audit/final_state_audit.py \
  --output /Volumes/artisan/code/2026/multica-0.4.37/.trellis/tasks/10-30-collaboration-fix-closure/research/continue-20261103/final-state-audit/post-make-v11.json
```

Select another new filename for any subsequent audit; existing output is refused.
The candidate must retain its registered v11 paths and clearance, and
`make-check-v11.json` must be complete. This implementer did not execute that
actual-candidate audit, services, database, product lint/typecheck/tests or a
Provider. Root must independently review this script and run it after the real
command.

An identity audit exit0 does not turn a failed complete command into a pass:
`wholecheck_exit_code` and `wholecheck_passed` remain explicit. Root separately
verifies actual live API/Web/database/renderer/Redis ownership and all original
result inventories before acceptance; this script records
`live_resources_revalidated=false`.
