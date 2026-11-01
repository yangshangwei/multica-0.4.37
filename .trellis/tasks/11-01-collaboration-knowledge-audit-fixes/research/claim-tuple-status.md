# CT-1 tuple cursor regression

Owner: `/root/claim_tuple_regression`. Task recovered from the visible parent dispatch because the delivered NEW_TASK payload was empty. Worktree `mc-kfix-lane-claim`, base `a4e444bdf`; existing review and root test evidence are preserved.

Smallest gap: current deep-history tests separate the decisive old row from newer rows by a day, so they do not prove complete traversal through an equal-timestamp group. Production already uses the correct `(created_at,id)` cursor. Only the canonical `task_knowledge_history_growth_test.go` and CT1 evidence/status change; SQL/generated files, migrations and production behavior stay untouched.

Plan: create one recorded native binding plus260 claimed-only rows with the existing fixture; set all261 negotiated rows to one fixed timestamp; prove lawful resume, select row129 by the production descending tuple order, corrupt that exact metadata row, and require `knowledge_context_changed`. Then run a private Go overlay replacing only the generated tuple comparison with timestamp-only comparison, keeping all bound SQL parameters typed, and require the new test to fail. The original production file is never changed by the mutant.

Only allocated database `kfix_claim_tuple_root_20261102_0033` is permitted. Before each test invocation, load its private permission and check actual `current_database` and migration identity with read-only SQL. Normal/race and relevant H3 controls run under the repository Agent CLI guard, with real-agent/Git config overrides removed and parallelism2. No database creation/migration/deletion, Provider, installed Agent CLI, dependency or full-suite operation.

State: implementation and verification in progress; no pass claim yet.

## Live verification

The canonical tied-history test passed normally:1 named run/pass,0 fail/skip; database identity verified against the allocated763 ledger before invocation. Source and log hashes are in `evidence/CT1-normal.json`. The private generated-SQL overlay preserves UUID parameter typing while removing the UUID comparison; it never edits production SQL/generated files. Timestamp-only mutation exited1 with the new named test failing for skipped tied history, not SQL setup. Race and adjacent controls follow.

## Final result

Implemented only the canonical tied-history test. Normal1/1 and race with three adjacent H3 controls4/4 passed, no fail/skip. Vet and scoped diff whitespace check passed. The timestamp-only SQL overlay was killed (exit1, one named failure). In that observed mutant run, the original binding generation was already lost at the valid-control assertion, before corruption; this is recorded explicitly. The correct implementation reached and passed the row129 corruption refusal. No SQL/generated source was edited, and the original generated-file SHA-256 remained unchanged.

No product or executable contract changed, so no spec edit was required or permitted in this bounded slice. Full service/handler/migration suites were not repeated; their existing parent evidence is unchanged. Only the allocated task database was used, no migration or cleanup command ran, and no Provider or installed Agent CLI was accessed. Exact commands, source/test/log hashes, exit codes and inventories are in `evidence/CT1-verification.json` and its referenced records. Coordinator independent review and final merged-candidate acceptance remain open.

State: verified and ready for the owned atomic commit; other untracked review/root evidence is untouched.
