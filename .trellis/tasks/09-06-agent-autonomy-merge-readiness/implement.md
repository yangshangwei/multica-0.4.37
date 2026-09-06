# Implementation and verification

All planned repair lanes are complete. The existing branch value is preserved: declared autonomy is enforced on the covered API writes, and unresolved task conflicts remain recoverable instead of being delivered.

## Repairs

- Squad reuse authorization uses the existing transaction for invocation-target and membership reads; ordinary ACL semantics remain intact.
- Repeated batch update objects merge field presence in input order, including case aliases, duplicate keys and outer null values.
- Finalize checks unmerged entries before staging, stages once, validates the final index, then commits without restaging. It reads Git blobs and marker-size attributes instead of parsing diagnostic prose.
- Conflict comparison preserves binary/non-regular handling, unusual paths, renames, unchanged examples and EOF line-ending changes; added duplicate groups are still refused.
- E2E cleanup uses the workspace API and asserts no status/rule-version residue. A Coordinator task token exercises the actual level ceiling, with equal/lower and legacy compatibility cases.
- Built-in guidance now names the covered API checks and states the existing process/credential isolation boundary accurately.

## Verification evidence

- New handler regressions failed on the reviewed implementation, then passed; 35 related handler cases passed.
- Cleanup assertion failed with 7 issue_status rows and 1 autopilot_rule_version row, then passed after API cleanup.
- Final rebuilt backend: 25 API cases passed; a fresh database had zero orphan status and rule-version rows afterward.
- Full guarded Go race suite: 64 packages passed. After the final EOF correction, the test dependency graph identified cmd/multica, internal/daemon, internal/daemon/execenv and internal/handler; all four passed race tests again.
- pnpm typecheck, lint and test passed through valid Turbo cache entries; the E2E file also passed uncached strict TypeScript and ESLint checks.
- Final Go vet, server/CLI build, gofmt and git diff checks passed.
- Independent server/E2E review: APPROVE. Independent execenv review: APPROVE after its EOF regression was fixed and rechecked.

## Work commits

- 665a2c05f: transaction-bound squad reuse authorization.
- 2560b3b1d: deterministic batch update fields.
- 6e4256ae3: final snapshot and conflict-group validation.
- 066665951: complete API test cleanup and real ceiling coverage.

## Limits and operational scope

These gates are not a process/credential isolation facility. Undeclared agents retain their existing behavior. Full browser UI flows and native Windows execution were not part of this change's verification; the POSIX fault-injection fixture is skipped on Windows. No migration or dependency was added.

The development backend was not the test target. Verification used temporary local databases and a separately built server; evidence logs are in /tmp/multica-autonomy-fix-6804a151/. Merge and remote publication are separate from this readiness task.
