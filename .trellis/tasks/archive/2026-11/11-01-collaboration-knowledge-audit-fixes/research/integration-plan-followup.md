# Integrated history-index plan regression

Owner: `/root/knowledge_plan_regression`. Target worktree `mc-knowledge-fixes-20261101`, starting HEAD `0702d5039`. Only the canonical runtime-history index test/probe in `task_knowledge_history_growth_test.go` and this follow-up's evidence are owned; other writers' task records and logs stay untouched.

## Root cause and smallest correction

The source-bound full service attempt at `45efa093c` failed `TestKnowledgeRuntimeHistoryIsIndexDriven`. It set `enable_seqscan=off`, yet the optimizer legally chose `idx_agent_task_queue_runtime_id` and `agent_task_queue_pkey` rather than one of the new partial history indexes. The old test interpreted that choice as absence of an eligible negotiated-row index. The approved M3 design requires those real index paths, not a guarantee that one named index wins every cost comparison under retained test history/statistics. Earlier independent review CV-2 already limits EXPLAIN evidence to index eligibility, not a production-size performance benchmark.

Keep the production SQL, generated code, migrations and planner settings unchanged. First assert against the current task table's real PostgreSQL catalog that all three history indexes are ready/valid/live, nonunique, have the required access method/key/order/opclass and negotiated-row predicate. Missing or malformed indexes must fail explicitly. Retain the controlled no-sequential-scan EXPLAIN check, but record the chosen partial-index fact as evidence instead of requiring that choice under every table history. The correction cannot claim reduced scan cost or a selective GIN plan.

Expected code boundary: only `TestKnowledgeRuntimeHistoryIsIndexDriven` and `knowledgeHistoryPlanProbe`. No production behavior, authorization, history retention, database data cleanup or new dependency changes.

## Original failure retained

`research/integration-verification/service-full.json` binds the failed command, source, database and raw log hash. The separate `plan-original-failure.json` will point to that unchanged evidence and retain the named failure excerpt/hash. Do not rerun or modify that failed database directly.

## Database verification request — pending root allocation

No database commands or DB-backed tests have run from this child. Please allocate an explicit service-only database/permit at `research/plan-db-permission.md` pointing to its private URL/identity record. Proposed selection:

`go -C server test ./internal/service -parallel=2 -count=1 -json -timeout=10m -run '^TestKnowledgeRuntimeHistoryIsIndexDriven$'`

Run normal and race via the repository Agent CLI guard with the real gate and Git config overrides unset. If possible, make the new task database a clone of the preserved failed service database `kfix_joint_svc_20261102_0110`, rather than altering that source; this retains the table history/statistics that exposed the assertion's invalid assumption. An overlay restoring the old strict plan assertion should reproduce its failure there, while the corrected catalog/path contract passes. A second missing-index lookup overlay must fail the catalog guard without dropping or changing a real index. Root may instead provide a freshly migrated clone and a separate preserved-history clone; the command scope remains only this service selection.

Build/vet/compile-only checks can proceed without DB access. Full service and final same-candidate acceptance remain coordinator gates. Native messages lose Payload, so this file is the complete status/request handoff.

## Implementation ready for allocated verification

The canonical test now checks real catalog readiness/validity/liveness/nonuniqueness, btree/GIN access method, key/order/opclass definition and the negotiated-row predicate before the query. EXPLAIN still rejects a sequential scan under its existing local enable_seqscan=off control; it logs whether a new partial path won instead of requiring that cost choice. The original old test bytes remain available at the source-bound45efa093c commit; no unexecuted overlay is counted as evidence. Production SQL, generated code and migration bytes remain unchanged.

For a missing-index negative control, a Go overlay can add a transaction-owned temporary shadow agent_task_queue created LIKE the actual table but without indexes immediately before catalog validation. The same catalog lookup must refuse it; the transaction rollback removes the shadow table. No actual index is dropped or edited, and the original failed database remains untouched. Please include this test-owned temporary-table overlay in the explicit new-database permit.

Database permission was allocated by root before execution. Only the recorded task database was used; service compile-only and vet passed.

## ROOT allocation READY (read now)

The requested research/plan-db-permission.md isREADY andcontains bothfreshandpreserved-historyclones. Primaryhistorypermit .omx/private/joint-plan-history-root-20261102/db-permission.json. Readthefileandexecuteonlythepermittedselection/overlays. Test-ownedtemporaryshadowtableoverlayisexplicitlyallowed, no realindexdrops. OriginalDBretaineduntouched.

## Final result

Normal canonical check1/1 and race with adjacent H3 controls9/9 passed, no failure or skip. The missing-index rollback-only shadow-table overlay exited1 with the named catalog refusal; it did not alter any real index or the preserved failed database. The allocated fresh database chose a new partial index; no claim is made that its fresh plan reproduces the retained full-suite's existing-index choice. The original full-service failure remains the red provenance for the invalid old assumption.

Source and generated SQL were not edited. The test now checks real ready/valid/live/nonunique index contracts with access method, key/order/opclass and predicate, then records the actual plan choice. No default-planner or production-scale performance guarantee is inferred. Complete commands, database identity, exits, named counts and hashes are in integration-verification/plan-verification.json and its referenced records. Full service was not repeated by this child. Coordinator independent delta review and final combined acceptance remain open.

## Preserved-history allocation follow-up

Root added a new clone of the completed failed service database and requested the original strict plan assertion overlay. The source database remains unchanged. After reading the updated permit, this child is now running the same canonical normal/race selection plus the old45efa assertion overlay and the missing-index temporary-shadow overlay against `kfix_joint_plan_history_20261102_0122`. These results use separate plan-history-* records; earlier fresh-database results are preserved and not overwritten. No old-assertion replay result is claimed until its terminal record exists.

## History clone result and recorded-plan replay

On the new preserved-history clone, normal1/1 and race9/9 passed. The overlay restoring the original old assertion also passed1/1: that clone's optimizer selected a partial index at this time, so the original failure was not deterministically reproduced there. The intended-red filename/record is retained with an explicit non-reproduced classification; it is not counted as a killed mutant or red pass.

The original45efa full-service log still contains the real failing existing-index EXPLAIN. To verify the precise assertion change without manipulating table data/statistics or claiming a newly selected plan, this child will replay that recorded JSON only at the EXPLAIN response boundary through Go overlays. The old assertion must reject that same observed plan; the catalog-backed current assertion must accept it while the actual namespace query still executes. This is an assertion replay, not a new optimizer run or performance measurement. Separate missing-index TEMP-shadow red follows on the history clone.

## History and assertion replay complete

The root-created preserved-history clone passed normal1/1 and race9/9, no fail/skip. Restoring the original assertion there also passed1/1; its intended-red record explicitly says the expectation was not reproduced. That pass is retained, not relabeled. Replaying the original full-service's actual EXPLAIN JSON at the response boundary gave one expected old-assertion failure and one current-assertion pass; the canonical namespace query still executed on the owned clone. This demonstrates the changed assertion's acceptance of the already-observed legal plan, not a new optimizer result or performance measurement. The same clone's missing-index TEMP-shadow control failed1/1 at the real catalog check.

All selected normal/race checks and both expected negative controls are complete. Only the assigned test file changed; production SQL/generated/migration bytes remain untouched. No default DB, migration, DB creation or real index deletion was performed. Test-owned temporary shadow tables rolled back. Ready for a local test/evidence commit and coordinator independent delta review; final full acceptance remains open.
