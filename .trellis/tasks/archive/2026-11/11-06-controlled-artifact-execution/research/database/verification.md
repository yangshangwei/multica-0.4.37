# Database lane verification

Implemented migrations 766–770, typed sqlc execution/approval/authority locks, retained bound approval cleanup, and migration runner cleanup mappings. The table has no foreign keys, inline primary/unique constraints or implicit indexes; three unique indexes are separate concurrent migrations. Highest down770 refuses any retained row before index removal; down766 repeats NOWAIT/empty/drop under one block. Public-schema migration remains coordinator-owned.

Verified actual PostgreSQL behavior in the coordinator's `multica_artifact_20261110_migrations` database using test-owned private schemas; parent pool and other databases were not changed. No paid Provider, service or user CLI calls.

- db-static-01: 1 pass, no failures/skips (new migration contract).
- db-live-01: 8 named passes, no failures/skips (real migration/rollback, partial/wrong index readiness, spec/nonce CAS, terminal replay/expiry, concurrent idempotency/claim and busy/retained rollback).
- db-race-01: 9 named passes, no failures/skips; adds invalid state and NULL-boundary constraints.
- db-retry-01: 3 passes, no failures/skips; actual failed concurrent index retained INVALID, registered retry cleanup reconstructs it after duplicate data removal; cleanup registry checks pass.
- db-migration-lint-01: 4 passes, no failures/skips; matching directions and unique migration versions.
- sqlc regeneration is byte-idempotent across131 generated files; scoped go vet exits0; whitespace checks pass including new files.

Each JSON record links its actual command and retained output. `verification.json` records final owned source hashes. An initial missing-migration static test was intentionally written before schema creation, but its tool output was blocked as a missing-file/setup error; it is not counted as retained red evidence. No historical test was relaxed.

SQL race tests prove one durable claim winner, not physical packaging. Actual adapter invocation counts, human/agent credential authority and workspace-deletion integration still require the API lane's tests. Workspace cleanup now retains any approval matched by execution approval_id, irrespective of damaged execution workspace identity; unbound cleanup is unchanged. Admission and finalization callers must use the documented transaction/locks and never invoke after ambiguous commit.
