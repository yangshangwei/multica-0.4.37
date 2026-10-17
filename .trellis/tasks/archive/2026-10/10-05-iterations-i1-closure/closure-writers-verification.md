# CG real-writer and recovery verification

Scope: new `server/internal/handler/iteration_closure_writers_test.go`. This lane owns tests only; related product fixes remain with the CG owner.

## Final run

2026-10-07, exclusive migrated database `multica_i1_notify_20261006_2310`, agent CLI guard enabled:

```sh
DATABASE_URL='<exclusive notification database>' bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -json -race ./internal/handler -run '^TestIterationClosure(Real|Catalog|HTTPRecovers|PlannedStart)' -count=1
```

Exit 0, package pass 6.456s. **28 leaf cases passed**, represented by 32 pass events including parent tests. No failures/skips. Do not add this count to overlapping full-regression counts.

Raw log: `/tmp/i1-cg-closure-writers-final.jsonl`.
SHA-256: `35a6454e115be13a45baa97fb62020d0a04eee3feb33ed8adac09099f36d793e`.
Exact names and package completion are in `closure-writers-verification.json`. `git diff --check` passed.

## Covered behavior

- One real HTTP flow closes a completed issue, then reopens/renames it, reassigns its project, deletes both original/replacement projects, deletes the issue, and reads closed statistics. The entire persisted snapshot digest stays unchanged after every writer; original completion remains frozen.
- Sixteen end/disable versus issue deletion, project deletion, ordinary status completion, and confirmed membership move cases exercise both lock acquisition orders. Requests use actual handlers and real transactions. `pg_blocking_pids` observes both writers waiting before releasing the shared parent fence. Closure-first membership move rejects with stale409 after end, or iteration_disabled422 after disable; status edits remain allowed. Writer-first invalidates the old closure preview409 with no snapshot or disabled settings.
- Four end/disable versus actual member removal cases exercise both recipient-fence queue orders. Revoke-first rejects closure403; closure-first commits before revocation. Subsequent protected history reads by the removed member return403.
- Four catalog-lock cases exercise guarded category maintenance versus end/disable in both orders. Category-first returns preview-stale409; closure-first preserves frozen category statistics after the later catalog change.
- One real HTTP recovery case commits closure successfully but injects an error from Commit afterward. The original request returns503; GET by the original request ID retrieves the durable result, and replay keeps the same operation ID and one snapshot.
- Two planned-start versus disable cases begin with two valid previews. Start-first commits and makes disable stale409; disable-first cancels/releases the plan, leaves no baseline, and rejects start422. Settings/lifecycle state match the single winning operation.

## Findings and limits

The catalog-first fixture initially returned503 because full history projection ran before stale-hash rejection. The CG owner moved stale-preview detection ahead of projection; the final run above verifies409.

**There is no production category-reclassification API.** `UpdateIssueStatus` edits presentation fields only. The catalog test deliberately uses a transaction with the existing exclusive catalog fence and a SQL category mutation to verify the lower-level locking invariant; it must not be counted as a handler category-edit workflow.

An independent review also found that date-edit notifications originally collected unvalidated coordinator/assignee IDs after writing. The CG owner added current-member reference locking before the business clock/write and owns its dedicated regression evidence.

These 28 cases complement the notification suite and CG service daemon/management-preservation tests. They do not establish the entire engineering scenario 6 Cartesian product or full CG/VG acceptance. Full shared regression, migration exercises, dual-client E2E, performance and final gate evidence remain with their existing lanes.
