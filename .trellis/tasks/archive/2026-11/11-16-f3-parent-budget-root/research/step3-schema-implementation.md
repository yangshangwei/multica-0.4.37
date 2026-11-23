# F3 step 3 — schema implementation

Date: 2026-11-17. Worktree `/Volumes/artisan/code/2026/mc-f3-budget-root-20261116`
(branch `codex/f3-budget-root-20261116`, base `86fea651c`). Nothing is committed
or staged. `server/internal/budget/` was not edited (tests import it read-only).

This report describes the working tree after round 3 (the five schema decisions
D1–D5 from the step 3 check, recorded as S1–S5 in `design.md` §8). Earlier
rounds are kept only where their evidence still applies.

## Verdict

Step 3 is implemented and its checks pass on the round 3 tree: six tables,
thirteen concurrent indexes, a verification migration, ten sqlc queries, the
teardown manifest registration, and tests B51 and B56. D1–D5 are applied, ten
table-level CHECK constraints are named, and the two column comments are in the
migration. Every one of the 55 CHECK constraints is pinned: dropping any single
one fails the schema tests (55 killed, 0 survived). No enforcement behaviour and
no wiring into issue create/update/delete or finalize was added.

## Migrations

| No. | File stem | Statement |
| --- | --- | --- |
| 866 | `866_issue_budget` | six `CREATE TABLE`, no key, no index; two `COMMENT ON COLUMN` |
| 867 | `867_issue_budget_root_id` | `issue_budget_root_id_key` UNIQUE (id) |
| 868 | `868_issue_budget_root_issue` | `issue_budget_root_issue_key` UNIQUE (workspace_id, root_issue_id) |
| 869 | `869_issue_budget_member_issue` | `issue_budget_member_issue_key` UNIQUE (workspace_id, issue_id) |
| 870 | `870_issue_budget_member_root` | `issue_budget_member_root_idx` (budget_root_id) |
| 871 | `871_issue_budget_attempt_id` | `issue_budget_attempt_id_key` UNIQUE (id) |
| 872 | `872_issue_budget_attempt_claim` | `issue_budget_attempt_claim_key` UNIQUE (task_id, claim_generation) |
| 873 | `873_issue_budget_attempt_root` | `issue_budget_attempt_root_idx` (budget_root_id) |
| 874 | `874_issue_budget_entry_seq` | `issue_budget_entry_seq_key` UNIQUE (budget_root_id, seq) |
| 875 | `875_issue_budget_entry_identity` | `issue_budget_entry_identity_key` UNIQUE (budget_root_id, entry_id) |
| 876 | `876_issue_budget_entry_attempt` | `issue_budget_entry_attempt_idx` (attempt_id) |
| 877 | `877_issue_budget_operation_id` | `issue_budget_operation_id_key` UNIQUE (id) — new in round 3 |
| 878 | `878_issue_budget_operation_scope` | `issue_budget_operation_scope_key` UNIQUE (workspace_id, scope_kind, scope_id, operation_key) — was 877 |
| 879 | `879_issue_budget_observation_root` | `issue_budget_observation_root_idx` (budget_root_id) — was 878 |
| 880 | `880_issue_budget_ready` | DO block: fails unless all thirteen indexes are valid, ready, plain btree, with the expected uniqueness and columns — was 879 |

867–879 are each one statement, `CREATE [UNIQUE] INDEX CONCURRENTLY IF NOT EXISTS`
up and `DROP INDEX CONCURRENTLY IF EXISTS` down, and each is registered in
`concurrentIndexCleanups` (`server/cmd/migrate/main.go`, +13 lines). The ready
migration is still the last one.

Down path (unchanged): `866` down and `880` down first lock the six tables
(`ACCESS EXCLUSIVE ... NOWAIT`) and raise
`cannot roll back retained budget records: <table> has rows` if any row exists;
`866` down then drops the six tables with `DROP TABLE IF EXISTS`.

## Tables

No table has a foreign key, `REFERENCES`, cascade, trigger, enum type, primary
key constraint or unique constraint. All have `workspace_id uuid NOT NULL`. IDs
have no default (the application issues them). Amounts are `bigint` ticks with
`CHECK (x >= 0)`.

**issue_budget_root** — `id`, `workspace_id`, `root_issue_id`,
`admission_state` (open/closed), `settled_as_of_revision`, `mode` (off/observe),
`policy_revision`, `policy_epoch`, `ledger_revision`, `limit_ticks`,
`actual_known_ticks`, `estimated_ticks`, `reserved_ticks`, `unknown_hold_ticks`,
`overrun_ticks`, `unknown_count`, `activated_at`, `created_at`, `updated_at`.
- `settled_as_of_revision`: NULL means unsettled; otherwise it must lie in
  `0..ledger_revision`. This is the whole settlement axis.
- `mode`: `enforce` cannot be stored in this slice.
- `limit_ticks`: nullable; NULL means no limit configured.
- `overrun_ticks`: the metering `Projection` has it, so the stored projection does too.
- No `tombstoned_at` (D3): whether the root is deleted is read from the root
  issue's member row.

**issue_budget_member** — `workspace_id`, `issue_id`, `budget_root_id`,
`bound_parent_issue_id`, `bind_source` (root/activation/create/reparent),
`binding_version`, `bound_at`, `tombstoned_at`.
- No `id`: `(workspace_id, issue_id)` identifies the row.
- `bound_parent_issue_id` is NULL exactly when `bind_source = 'root'` (CHECK).
- `binding_version`: version of the binding rule that wrote the row.

**issue_budget_attempt** — `id`, `workspace_id`, `budget_root_id`, `task_id`,
`claim_generation`, `runtime_id`, `agent_id`, `issue_id`,
`context_parent_issue_id`, `state`, `reserved_ticks`, `price_snapshot`,
`price_digest`, `protocol_version`, `policy_revision`, `policy_epoch`,
`created_at`, `updated_at`.
- `state`: open, the five hold reasons of the metering package, or settled.
- `issue_id` / `context_parent_issue_id`: at least one must be set (quick-create
  tasks have no issue).
- `price_snapshot bytea` (2..65536 bytes) + `price_digest`: both or neither; the
  database recomputes the digest over the bytes with the package's domain prefix.
- `protocol_version`: NULL for a daemon that does not speak the budget protocol.
- `policy_revision`, `policy_epoch` (D5): nullable `bigint`, each `> 0`, both
  set or both NULL. They are not compared with the root's values by the database.
- No `would_refuse_reason` (D4): a would-refuse fact is an observation row.

**issue_budget_entry** — `workspace_id`, `budget_root_id`, `seq`, `entry_id`,
`kind`, `attempt_id`, `invocation_id`, `scope_id`, `entry_json`, `entry_digest`,
`receipt`, then the envelope columns `source`, `schema_version`,
`adapter_version`, `normalizer_version`, `producer_epoch`, `payload_digest`,
`observed_at`, `reported_at`, `received_at`, `completeness`, then
`operation_id`, and `created_at`.
- `entry_json bytea`: the exact bytes the metering package marshals, 2..65536
  bytes. `jsonb` would reorder keys and break the digest.
- `entry_digest`: CHECK equals `sha256("budget-entry-v1\n" || entry_json)` in hex.
- `receipt jsonb`: the committed outcome, returned again on a resend.
- `attempt_id`: its own column, because the stored form only has a reservation ID.
- `source`, `schema_version`: NOT NULL. `source` is format-checked only.
  `schema_version` is the version of the stored entry form in `entry_json`
  (column comment).
- All other envelope columns are nullable: an entry the server writes itself
  has none of them.
- `received_at`: when the reporting request arrived at the server; NULL on an
  entry the server wrote itself. `created_at` is the commit time of the row
  (column comment).
- `payload_digest`: 64 lowercase hex characters.
- `completeness`: `partial` or `final`.
- `operation_id uuid` (D2): nullable; the `id` of the operation (for reported
  usage, the batch) that appended the entry. It replaces the three
  `operation_scope_kind` / `operation_scope_id` / `operation_key` columns. It
  has no index and, like every other ID copy, no database check that the
  operation row exists.
- CHECKs: reserve/hold/settle need `attempt_id`; `invocation_id` and `scope_id`
  are set exactly for `usage`.

**issue_budget_operation** — `id`, `workspace_id`, `budget_root_id`,
`scope_kind` (issue/root/attempt), `scope_id`, `operation_key`,
`operation_kind`, `intent_digest`, `actor_type` (member/runtime), `actor_id`,
`result`, `created_at`.
- `id uuid NOT NULL` (D2) with its own unique index. The four-column scope key
  stays unique as before; it is still what makes a retried operation resolve to
  the first one.
- `intent_digest`: 64 lowercase hex characters.

**issue_budget_observation** — `id`, `workspace_id`, `budget_root_id`, `kind`
(would_refuse/attribution_difference/coverage_gap), `reason`, `issue_id`,
`task_id`, `claim_generation`, `attempt_id`, `detail`, `created_at`.
- `claim_generation` (D1): nullable `bigint`, `>= 0`. No new index.
- No unique index at all: one attempt may have any number of rows (V17).

### Named constraints

Ten table-level CHECKs carry explicit names, so no CHECK in these tables has a
positional name (`<table>_check`, `<table>_check1`, ...). The catalog test
asserts both the ten names and the absence of any positional name.

| Table | Constraint | Rule |
| --- | --- | --- |
| root | `issue_budget_root_settlement_check` | `settled_as_of_revision` NULL or in `0..ledger_revision` |
| member | `issue_budget_member_root_parent_check` | parent NULL exactly for `bind_source = 'root'` |
| attempt | `issue_budget_attempt_attribution_check` | `issue_id` or `context_parent_issue_id` set |
| attempt | `issue_budget_attempt_price_pair_check` | snapshot and digest both or neither |
| attempt | `issue_budget_attempt_price_digest_check` | digest recomputed over the snapshot bytes |
| attempt | `issue_budget_attempt_policy_pair_check` | policy revision and epoch both or neither |
| entry | `issue_budget_entry_digest_check` | digest recomputed over `entry_json` |
| entry | `issue_budget_entry_kind_attempt_check` | reserve/hold/settle need `attempt_id` |
| entry | `issue_budget_entry_usage_invocation_check` | `invocation_id` set exactly for `usage` |
| entry | `issue_budget_entry_usage_scope_check` | `scope_id` set exactly for `usage` |

The other 45 CHECKs are single-column checks with PostgreSQL's
`<table>_<column>_check` names, which are already stable.

## Queries and generated code

`server/pkg/db/queries/issue_budget.sql`: `InsertIssueBudgetRoot`,
`GetIssueBudgetRootByIssue`, `LockIssueBudgetRoot` (`FOR UPDATE`),
`InsertIssueBudgetMember`, `GetIssueBudgetMember`, `InsertIssueBudgetAttempt`
(now with nullable `policy_revision`, `policy_epoch`), `InsertIssueBudgetEntry`
(now with nullable `operation_id`), `ListIssueBudgetEntries` (one root,
`ORDER BY seq`), `InsertIssueBudgetOperation` (now with `id`),
`InsertIssueBudgetObservation` (now with nullable `claim_generation`).
Generated with `go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate`
(exit 0, no output): `pkg/db/generated/issue_budget.sql.go`, `models.go`
+111/-0 against the base. No generated file was edited by hand;
`sqlc diff` on the final tree exits 0 with no output (`r3/sqlc-diff.txt`).
sqlc copies the two column comments into `models.go` as field comments.

## Teardown and other guards

- `internal/handler/workspace_delete_manifest_test.go`: the six tables are
  registered as `workspaceRetainAudit` (+9 lines).
- Teardown handler: unchanged. No non-test code refers to the retain class, so a
  retained table needs no handler code. `TestWorkspaceDeletionLeavesBudgetRowsUntouched`
  shows teardown neither writes nor locks a budget row.
- Other enumerating guards found: only the `cmd/migrate` cleanup-registration
  tests. They pass.
- The brief said the cleanup-registration check already reports eight
  `report_snapshot*` omissions. That did not reproduce in this worktree: the map
  lists 795–803 and the three registration tests passed before my change
  (`baseline-cleanup.txt`) and after it.

## Inputs from the step 2 check

Received after the first report; all three are in the working tree.

1. **Envelope columns.** The `envelope jsonb` placeholder was replaced by
   separate columns, listed under `issue_budget_entry`. No new index.
2. **Identifier length.** Every caller-supplied identifier is
   `CHECK (octet_length(x) BETWEEN 1 AND 200)`: entry `entry_id`,
   `invocation_id`, `scope_id`, `adapter_version`, `normalizer_version`,
   `producer_epoch`, and operation `operation_key` (seven columns after round 3;
   the entry's own `operation_key` copy is gone). The catalog test reads each
   constraint back and fails if one column is written differently; a
   101-character, 202-byte value is refused, so the limit is in bytes.
3. **Ledger load.** `ListIssueBudgetEntries` returns one root's entries by
   `seq`. No ledger-state column was added.
   - `TestIssueBudgetLedgerLoadIsOrderedByItsKey` explains the statement taken
     from the sqlc source file with whole-table and bitmap scans disabled: the
     plan is `Index Scan using issue_budget_entry_seq_key`, no Sort. With that
     index dropped the test fails (`r2/mutation-no-seq-key.txt`).
   - What the planner chooses on its own depends on the data
     (`r2/ledger-load-plan.txt`). On an empty table it uses the seq key in
     order. With 20,000 rows where one root owns 400 scattered rows, it fetches
     that root's rows by a bitmap scan on a `budget_root_id`-leading index and
     sorts those 400 rows. It never reads another root's entries. I did not
     force the plan.

## Round 3: the five decisions and what else changed

| Item | Change |
| --- | --- |
| D1 | `issue_budget_observation.claim_generation bigint`, nullable, `CHECK (claim_generation >= 0)`; insert argument added |
| D2 | `issue_budget_operation.id uuid NOT NULL` + migration 877 (unique index, registered, in the ready check); entry's three batch columns replaced by `operation_id uuid` |
| D3 | `issue_budget_root.tombstoned_at` dropped |
| D4 | `issue_budget_attempt.would_refuse_reason` dropped |
| D5 | `issue_budget_attempt.policy_revision`, `policy_epoch`: nullable `bigint`, each `> 0`, both or neither |
| names | ten constraints named (table above) |
| comments | `COMMENT ON COLUMN` for `issue_budget_entry.received_at` and `.schema_version` |

Choices I made inside that brief:

1. **Numbering.** The new operation `id` index is 877, next to the operation
   table's other index, so 877–879 of round 2 became 878–880. All maps and tests
   that name a stem were updated.
2. **Attempt price size check split out.** Naming the digest constraint meant
   rewriting the combined "size and digest" CHECK. The size bound is now a
   column check on `price_snapshot`, the digest rule is
   `issue_budget_attempt_price_digest_check`. Two refusal cases (snapshot
   shorter than an object, snapshot over 64 KiB) pin the size half, which no
   case pinned on its own before.
3. **Comments as catalog comments**, not only SQL comments, so they are visible
   in the database and in the generated models. The repository already has
   `COMMENT ON COLUMN` in fourteen migration files.
4. **Refusal cases.** Of the 23 cases the checker added, 22 are kept (those on
   the operation table now insert with a fresh `id`). One went with its column:
   "malformed would-refuse reason". Four of my own round 2 cases went with the
   entry's batch columns (key without scope, scope without key, unknown scope,
   key over the byte limit). Added: reused operation id (23505), operation
   without an id (23502), price snapshot too short / too long, policy revision
   zero, policy epoch zero, revision without epoch, epoch without revision,
   negative observation claim generation, and negative `estimated_ticks`,
   `reserved_ticks`, `unknown_hold_ticks`, `overrun_ticks` on the root (see
   "Do the tests bite").
5. **Not added:** an index on `issue_budget_entry.operation_id` (finding the
   entries of one batch is not a step 3 query), and any CHECK tying
   `operation_id` to an entry kind.

Not changed, as instructed: the guarded downs, `source` as an open pattern, the
three-state envelope nullability, `actor_type` values, no ledger-state column.

## Decisions to confirm

Items 1–14 and 16 are as confirmed after round 1 and 2 unless marked.

1. **Guarded down plus a verification migration** (now 880). An empty schema
   still rolls back cleanly.
2. Settlement is one nullable column, `settled_as_of_revision`.
3. `mode` allows only `off` and `observe`.
4. `limit_ticks` is nullable.
5. The root stores `overrun_ticks` and a single `unknown_count`; it stores no
   serialized ledger state (step 5 decides that).
6. The root has `activated_at` but no actor column; the actor is on the
   operation row.
7. Member: no `id`; `binding_version` is a rule version, not a per-root counter;
   `bind_source` values as listed.
8. Attempt: `state` folds open, the five hold reasons and settled into one
   column; the issue check is "at least one", not "exactly one"; `runtime_id`
   and `agent_id` are NOT NULL; `protocol_version` is a nullable integer.
   Round 3: the policy pair is nullable so an attempt written without a policy
   judgment can exist.
9. Entry: canonical form stored as `bytea` with a database CHECK on the digest.
   A change of the digest domain in the package then needs a migration (the
   named constraint is what that migration would replace). Stored forms are
   capped at 64 KiB and identifiers at 200 bytes. `attempt_id` is a `uuid`,
   which assumes the ledger uses the attempt UUID as `ReservationID`. The
   `kind`, `attempt_id`, `scope_id`, `invocation_id` and `operation_id` columns
   are not checked against the bytes or against other tables; the writer must
   keep them equal.
10. Operation: `id` added in round 3; `budget_root_id NOT NULL` (only committed
    operations are stored); `operation_kind` is format-checked only.
11. Observation: `budget_root_id NOT NULL`; `reason` is format-checked only; no
    mutable "merged" column.
12. `sqlc.yaml` gains `json:"-"` for all six tables so ticks cannot leave as
    JSON numbers through a generated struct. Not asked for; easy to remove.
13. `LockIssueBudgetRoot` filters on `workspace_id` and `id`, not `id` alone.
14. `CREATE TABLE` is written without `IF NOT EXISTS`.
15. Envelope readings. Settled in round 3: `schema_version` is the version of
    the stored entry form; `received_at` is request arrival time; the batch
    reference is one `operation_id`. Still for step 6 to confirm:
    - `producer_epoch` is bounded text, not a number.
    - `payload_digest` assumes SHA-256 in hex.
    - `completeness` is text (`partial`/`final`), not a boolean.
16. Step 4 will change B56's expectation: the member row will get
    `tombstoned_at`, so the "unchanged" snapshot for the child member must
    become "tombstoned, otherwise unchanged".

## Verification

All commands ran from `/Volumes/artisan/code/2026/mc-f3-budget-root-20261116/server`.
`$DB(name)` stands for
`DATABASE_URL='postgres://multica:multica@127.0.0.1:5432/<name>?sslmode=disable'`.
Round 3 output is in `/tmp/f3-step3/r3/`; rounds 1 and 2 are in `/tmp/f3-step3/`
and `/tmp/f3-step3/r2/` and are superseded except where cited.

### Red, round 1 (before implementation)

| Command | Exit | File | Result |
| --- | --- | --- | --- |
| `env -u DATABASE_URL go test ./internal/migrations/ -run 'TestIssueBudget' -count=1 -v` | 1 | `red-migrations-text.txt` | 3 FAIL: migration files do not exist |
| `$DB(f3_budget_s3_red_migrate) go test ./cmd/migrate/ -run 'TestIssueBudget' -count=1 -v` | 1 | `red-cmd-migrate.txt` | 3 FAIL: `stat ... 866_issue_budget.up.sql: no such file` |
| `$DB(f3_budget_s3_red_handler) go test ./internal/handler/ -run 'TestIssueBudgetSchemaCatalog\|TestIssueBudgetSchemaConstraints\|TestDeleteMemberIssueKeepsBudgetRows\|TestWorkspaceDeletionManifestCoversPublicSchema' -count=1 -v` | 1 | `red-handler.txt` | 3 FAIL: `found 0 of 6 budget tables`, SQLSTATE 42P01 twice |
| same command after the migrations, before the manifest edit | 1 | `red-handler-manifest.txt` | `TestWorkspaceDeletionManifestCoversPublicSchema` FAIL: `unclassified=[issue_budget_attempt ... issue_budget_root]` |
| `go vet ./internal/handler/` before sqlc generate | 1 | `red-queries-compile.txt` | `q.InsertIssueBudgetRoot undefined` |

### Red, round 2 (step 2 inputs)

| Command | Exit | File | Result |
| --- | --- | --- | --- |
| `go vet ./internal/handler/` | 1 | `r2/red-queries-compile.txt` | `unknown field Source in struct literal of type db.InsertIssueBudgetEntryParams` |
| `$DB(f3_budget_s3_r2_red) go test -overlay r2/overlay-no-queries-test.json ./internal/handler/ -run 'TestIssueBudgetSchemaCatalog\|TestIssueBudgetSchemaConstraints' -count=1 -v` | 1 | `r2/red-handler-schema.txt` | 2 FAIL: envelope columns absent, length constraints absent, SQLSTATE 42703 |

### Red, round 3 (tests changed first, schema still round 2)

| Command | Exit | File | Result |
| --- | --- | --- | --- |
| `go vet ./internal/handler/` | 1 | `r3/red-queries-compile.txt` | `unknown field PolicyRevision in struct literal of type db.InsertIssueBudgetAttemptParams` |
| the three schema-dependent handler tests on `f3_budget_s3_r3_red`, with `-overlay r3/overlay-no-queries-test.json` | 1 | `r3/red-handler-schema.txt` | `TestIssueBudgetSchemaCatalog`, `TestIssueBudgetSchemaConstraints`, `TestWorkspaceDeletionLeavesBudgetRowsUntouched` FAIL: new columns, named checks, comments and the operation id index absent; SQLSTATE 42703 on `operation.id` |
| the `TestIssueBudget` tests of `./internal/migrations/` | 1 | `r3/red-migrations-text.txt` | `TestIssueBudgetIndexMigrationContract`, `TestIssueBudgetRollbackKeepsLedgerRows` FAIL: `877_issue_budget_operation_id` and `880_issue_budget_ready` files absent |
| the `TestIssueBudget` tests of `./cmd/migrate/` on `f3_budget_s3_r3_red` | 1 | `r3/red-cmd-migrate.txt` | all three FAIL |

The overlay only removes the queries test from that one run, so the schema
tests could run against the old schema while the queries test did not compile.

### Green, round 3 (final working tree)

Every run below is newer than the last source edit.

| Command | Exit | File | Top-level PASS / FAIL / SKIP |
| --- | --- | --- | --- |
| `$DB(f3_budget_s3_r3_handler) go test ./internal/handler/ -run 'TestIssueBudget\|TestDeleteMemberIssueKeepsBudgetRows\|TestWorkspaceDeletion\|TestDeleteIssue\|TestBatchDeleteIssues' -count=1 -v` | 0 | `green-handler.txt` | 17 / 0 / 0 |
| `$DB(f3_budget_s3_r3_migrate) go test ./cmd/migrate/... -count=1 -v` | 0 | `green-cmd-migrate.txt` | 67 / 0 / 1 |
| `$DB(f3_budget_s3_r3_migrations) go test ./internal/migrations/... -count=1 -v` | 0 | `green-migrations.txt` | 103 / 0 / 0 |
| `$DB(f3_budget_s3_r3_race_handler) go test -race ./internal/handler/ -run 'TestIssueBudget\|TestDeleteMemberIssueKeepsBudgetRows\|TestWorkspaceDeletion' -count=1 -v` | 0 | `race-handler.txt` | 9 / 0 / 0, no DATA RACE |
| `$DB(f3_budget_s3_r3_race_migrate) go test -race ./cmd/migrate/... -run 'TestIssueBudget\|TestEveryConcurrent\|TestConcurrentIndexCleanupsMatchTheirMigrations' -count=1 -v` | 0 | `race-cmd-migrate.txt` | 6 / 0 / 0, no DATA RACE |
| `go build ./...` | 0 | `final-build.txt` | |
| `go vet ./...` | 0 | `final-vet.txt` | |
| `gofmt -l cmd/migrate internal/handler internal/migrations pkg/db/generated internal/budget` | 0 | `final-gofmt.txt` | empty |
| `env -u DATABASE_URL go test ./internal/budget/... -count=1 -v` | 0 | `final-budget-pkg.txt` | 70 / 0 / 0 |
| `go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 diff` | 0 | `sqlc-diff.txt` | empty |

Each clone was created from the template and brought to `880_issue_budget_ready`
with `go run ./cmd/migrate up` on the clone (`migrate-up-*.txt`, all exit 0).

The handler figure is 17 where round 2 reported 18: this run's pattern leaves
out `TestProjectDeletionPreservesKnowledgeAutomationHistory`, an existing test
that does not touch budget tables. The seven budget tests are the same.

Zero-skip evidence. `r3/green-handler.txt` names, all `--- PASS`:
`TestIssueBudgetSchemaCatalog`, `TestIssueBudgetSchemaConstraints`,
`TestDeleteMemberIssueKeepsBudgetRows`, `TestIssueBudgetQueriesRoundTrip`,
`TestIssueBudgetLedgerLoadIsOrderedByItsKey`,
`TestWorkspaceDeletionLeavesBudgetRowsUntouched`,
`TestWorkspaceDeletionManifestCoversPublicSchema`, plus ten existing delete and
teardown tests; `grep -c -- '--- SKIP'` is 0. The single skip in
`r3/green-cmd-migrate.txt` is `TestCommentContentBigramRequirementMatchesRealPGBigm`
("Postgres does not provide pg_bigm"), an existing test unrelated to this step.
`r3/green-migrations.txt` has `TestIssueBudgetTableMigrationContract`,
`TestIssueBudgetIndexMigrationContract`, `TestIssueBudgetRollbackKeepsLedgerRows`
as `--- PASS`.

### Do the tests bite

**CHECK mutation, round 3.** Method of the step 3 check: on one clone, drop each
CHECK in turn, run the schema tests with a prebuilt test binary, restore it;
killed means the run exits 1. The final tree has 55 CHECK constraints (54
before; four went with dropped columns, five came with D1, D5 and the price
size split). Every mutation run reports zero skipped tests.

| Pass | Tests run | Script | Killed | Survived |
| --- | --- | --- | --- | --- |
| checker's method | `TestIssueBudgetSchemaCatalog\|TestIssueBudgetSchemaConstraints` | `r3/mut-checks.sh` | 55 | 0 |
| behaviour only | `TestIssueBudgetSchemaConstraints` | `r3/mut-checks-behaviour.sh` | 55 | 0 |

Results per constraint: `r3/mut/checks-result.txt` and
`r3/mut-behaviour/checks-result.txt`; with all constraints restored both exit 0.
Both passes ran before the last edit to the test file, which was a one-space
`gofmt` fix on one line; the green runs above are after it.

Why the second pass: the catalog test now asserts the ten named constraints by
name, so the checker's method kills those ten whether or not any insert is
refused. The behaviour-only pass asks for a refused insert for every
constraint. Its first run had 51 killed and 4 survived: the root's
`estimated_ticks`, `reserved_ticks`, `unknown_hold_ticks` and `overrun_ticks`
checks were held only by the catalog test's amount-column assertion. I added
one refusal case for each; the table above is the rerun.

**Keys and locks, round 1.** On clone `f3_budget_s3_mutation` I added cascading
foreign keys to three budget tables and reran the new tests
(`mutation-fk-cascade.txt`, `mutation-fk-cascade-2.txt`, both exit 1 as
intended): the catalog test reported "3 foreign keys" and "6 triggers"; B56
reported the child member and the attempt as gone; the teardown test reported
root, member and attempt rows written or removed. `xmax-lock-marker.txt` shows a
`FOR KEY SHARE` lock changes `xmax`, so the teardown snapshot also detects a
lock that leaves the row in place. Not repeated in round 3; those assertions did
not change.

### Down / up proof

`cmd/migrate down` rolls back every migration, so it cannot stop at 866. Two
proofs:
- `TestIssueBudgetMigrationsRunUpDownUp` runs the real files up, down, up through
  the migration runner in a private schema.
- Manual, repeated in round 3 on clone `f3_budget_s3_r3_downup`,
  `/tmp/f3-step3/r3/down-up-proof.txt`, using
  `/tmp/f3-step3/r3/down-budget-migrations.sh` (each down file through `psql`
  from 880 to 866, then its ledger row removed):

| Step | Tables | Indexes | Ledger rows | Head |
| --- | --- | --- | --- | --- |
| clone at 865 | 0 | 0 | 0 | `865_knowledge_automation_origin` |
| `migrate up` (exit 0) | 6 | 13 | 15 | `880_issue_budget_ready` |
| down with one member row (exit 1, refused at 880) | 6 | 13 | 15 | `880_issue_budget_ready` |
| down on empty tables (exit 0) | 0 | 0 | 0 | `865_knowledge_automation_origin` |
| `migrate up` again (exit 0) | 6 | 13 | 15 | `880_issue_budget_ready` |

Invalid indexes and foreign keys are 0 at every step. The refusal message was
`cannot roll back retained budget records: issue_budget_member has rows`.

## Not verified

- Only the local PostgreSQL 17 container (amd64 image, emulated on arm64) was
  used. Behaviour on the production server version is not tested.
- Concurrent index builds ran on empty tables only; build time and lock
  behaviour on a loaded database are not measured.
- Ledger load cost was not measured. The plan was read on an empty table and on
  20,000 synthetic rows (round 2); replay time for a large root is a step 5
  question.
- Round 3 mutated CHECK constraints only. The foreign-key mutation of round 1,
  the seq-key mutation of round 2 and the checker's five index and key
  mutations were not repeated. The new `issue_budget_operation_id_key` index was
  not mutated on its own; it is covered by the catalog shape assertion, the
  "reused operation id" case and the ready migration.
- The whole `internal/handler` package was not run, as instructed; only the
  patterns above.
- No frontend check was run; no TypeScript changed.

## Databases

Round 1: `f3_budget_s3_red_handler`, `f3_budget_s3_red_migrate`,
`f3_budget_s3_mutation`, `f3_budget_s3_handler`, `f3_budget_s3_migrate`,
`f3_budget_s3_migrations`, `f3_budget_s3_race_handler`,
`f3_budget_s3_race_migrate`, `f3_budget_s3_down`. Round 2:
`f3_budget_s3_r2_red`, `f3_budget_s3_r2_handler`, `f3_budget_s3_r2_mutation`,
`f3_budget_s3_r2_migrate`, `f3_budget_s3_r2_migrations`,
`f3_budget_s3_r2_race_handler`, `f3_budget_s3_r2_race_migrate`,
`f3_budget_s3_r2_down`. Round 3: `f3_budget_s3_r3_red`,
`f3_budget_s3_r3_handler`, `f3_budget_s3_r3_checks`, `f3_budget_s3_r3_checks2`,
`f3_budget_s3_r3_migrate`, `f3_budget_s3_r3_migrations`,
`f3_budget_s3_r3_race_handler`, `f3_budget_s3_r3_race_migrate`,
`f3_budget_s3_r3_downup` (the handler and the two mutation clones were dropped
and recreated for the final runs). All are dropped (`dbs-after-drop.txt`,
`r2/dbs-after-drop.txt`, `r3/dbs-after-drop.txt`). `f3_budget_base_865` remains,
still at `865_knowledge_automation_origin` with no budget table; no test or
migration ran against it.

## Files

Worktree, under `server/`:
- new `migrations/866_issue_budget.{up,down}.sql` … `migrations/880_issue_budget_ready.{up,down}.sql` (30 files)
- changed `cmd/migrate/main.go`; new `cmd/migrate/issue_budget_test.go`
- new `internal/migrations/issue_budget_test.go`
- new `internal/handler/issue_budget_schema_test.go` (B51), `issue_budget_delete_test.go` (B56), `issue_budget_teardown_test.go`, `issue_budget_queries_test.go`
- changed `internal/handler/workspace_delete_manifest_test.go`
- new `pkg/db/queries/issue_budget.sql`; changed `sqlc.yaml`
- generated `pkg/db/generated/issue_budget.sql.go`, `pkg/db/generated/models.go`
